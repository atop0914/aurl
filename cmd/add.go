package cmd

import (
	"aurl/internal/config"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var addFlags struct {
	baseURL string
	headers []string
	graphql bool
}

var addCmd = &cobra.Command{
	Use:   "add [name] [spec]",
	Short: "Register an API by name from a URL or local file",
	Args:  cobra.ExactArgs(2),
	Example: `  aurl add petstore https://petstore3.swagger.io/api/v3/openapi.json
  aurl add --graphql linear https://api.linear.app/graphql
  aurl add myapi ./openapi.json --base-url https://api.example.com`,
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringVar(&addFlags.baseURL, "base-url", "", "Override base URL from spec")
	addCmd.Flags().StringArrayVar(&addFlags.headers, "header", []string{}, "Custom headers (key: value)")
	addCmd.Flags().BoolVar(&addFlags.graphql, "graphql", false, "Treat spec as GraphQL endpoint")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	specRef := args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if already exists
	if _, ok := cfg.GetAPI(name); ok {
		return fmt.Errorf("API %q is already registered (use 'aurl %s --help' to see details)", name, name)
	}

	var specType string
	var specContent []byte
	var resolvedBaseURL string
	var detectedAuth config.AuthConfig

	// Determine if specRef is a URL or a file path
	if _, err := url.ParseRequestURI(specRef); err == nil {
		// It's a URL
		resp, err := http.Get(specRef)
		if err != nil {
			return fmt.Errorf("failed to fetch spec from %s: %w", specRef, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to fetch spec: HTTP %d", resp.StatusCode)
		}

		specContent, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read spec body: %w", err)
		}
		specType = "openapi3"
		resolvedBaseURL = addFlags.baseURL
	} else {
		// It's a file path
		var err error
		specContent, err = os.ReadFile(specRef)
		if err != nil {
			return fmt.Errorf("failed to read spec file: %w", err)
		}
		specType = "openapi3"
		resolvedBaseURL = addFlags.baseURL
	}

	if addFlags.graphql {
		specType = "graphql"
		if resolvedBaseURL == "" {
			resolvedBaseURL = specRef
		}
	}

	// Auto-detect auth from securitySchemes if not explicitly provided
	if len(addFlags.headers) > 0 {
		for _, h := range addFlags.headers {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				detectedAuth.Type = "bearer"
				detectedAuth.Header = strings.TrimSpace(parts[0])
				detectedAuth.Value = strings.TrimSpace(parts[1])
				break
			}
		}
	} else if specType == "openapi3" && len(specContent) > 0 {
		// Try to auto-detect auth from the spec via raw JSON parsing
		detectedAuth = detectAuthFromRawSpec(specContent)
	}

	if detectedAuth.Type == "none" || detectedAuth.Header == "" {
		// No auth detected
		detectedAuth = config.AuthConfig{Type: "none"}
	}

	api := config.APIConfig{
		Name:    name,
		SpecURL: specRef,
		BaseURL: resolvedBaseURL,
		Type:    specType,
		Auth:    detectedAuth,
	}

	if err := cfg.AddAPI(api); err != nil {
		return fmt.Errorf("failed to save API: %w", err)
	}

	aliasNote := ""
	if specType == "graphql" {
		aliasNote = " (GraphQL)"
	}
	fmt.Printf("Registered %q%s\n", name, aliasNote)
	if resolvedBaseURL != "" {
		fmt.Printf("Base URL: %s\n", resolvedBaseURL)
	}
	if detectedAuth.Type != "none" && detectedAuth.Header != "" {
		fmt.Printf("Auth: %s (%s)\n", detectedAuth.Type, detectedAuth.Header)
	}

	return nil
}

// detectAuthFromRawSpec parses spec bytes to find securitySchemes
func detectAuthFromRawSpec(data []byte) config.AuthConfig {
	// Use a lightweight JSON scan for securitySchemes
	// We look for components.securitySchemes
	var result config.AuthConfig
	result.Type = "none"

	// Simple JSON parsing without full unmarshal
	// Find securitySchemes section
	idx := findJSONKey(data, "securitySchemes")
	if idx < 0 {
		// Try top-level security
		idx = findJSONKey(data, `"security"`)
		if idx < 0 {
			return result
		}
	}

	// Try to find apiKey or http (bearer) schemes
	// Look for "type": "apiKey"
	if typeIdx := findJSONNestedValue(data, "type", "apiKey"); typeIdx > 0 {
		// Find the name of this scheme — look backwards for the key name
		name := findSchemeName(data, typeIdx)
		if name != "" {
			// Find where this scheme is defined (in securitySchemes)
			schemeStart := findContainingObject(data, typeIdx)
			if schemeStart >= 0 {
				// Check the "in" field
				in := findFieldInObject(data[schemeStart:], "in")
				if in == "header" || in == "" {
					result.Type = "api_key"
					result.Header = name
					result.Value = "<API_KEY>"
					return result
				}
			}
		}
	}

	// Look for http bearer
	if typeIdx := findJSONNestedValue(data, "type", "http"); typeIdx > 0 {
		schemeIdx := findJSONNestedValue(data, "scheme", "bearer")
		if schemeIdx > 0 && schemeIdx < typeIdx+200 {
			name := findSchemeName(data, typeIdx)
			if name != "" {
				result.Type = "bearer"
				result.Header = "Authorization"
				result.Value = "Bearer <TOKEN>"
				return result
			}
		}
	}

	return result
}

// findJSONKey finds the byte index of a JSON key (not value)
// Returns -1 if not found
func findJSONKey(data []byte, key string) int {
	pattern := []byte(`"` + key + `"`)
	for i := 0; i <= len(data)-len(pattern); i++ {
		if string(data[i:i+len(pattern)]) == string(pattern) {
			return i
		}
	}
	return -1
}

// findJSONNestedValue looks for {"type": "value"} and returns the index of "value"
func findJSONNestedValue(data []byte, field, value string) int {
	fieldPat := []byte(`"` + field + `"`)
	valuePat := []byte(`"` + value + `"`)
	for i := 0; i <= len(data)-len(fieldPat); i++ {
		if string(data[i:i+len(fieldPat)]) == string(fieldPat) {
			// Find the colon and the value string after it
			for j := i + len(fieldPat); j < len(data); j++ {
				if data[j] == ':' {
					for k := j + 1; k <= len(data)-len(valuePat); k++ {
						if data[k] == ' ' || data[k] == '\n' || data[k] == '\t' {
							continue
						}
						if string(data[k:k+len(valuePat)]) == string(valuePat) {
							return k
						}
						break
					}
					break
				}
			}
		}
	}
	return -1
}

// findSchemeName finds the name of a security scheme given an index inside it
func findSchemeName(data []byte, idx int) string {
	// Search backwards for the key name (before the {)
	depth := 0
	for i := idx - 1; i >= 0; i-- {
		c := data[i]
		if c == '}' {
			depth++
		} else if c == '{' {
			if depth > 0 {
				depth--
			} else {
				// Found the opening brace, now find the key before it
				for j := i - 1; j >= 0; j-- {
					if data[j] == '"' {
						// Extract the string before this quote
						start := j - 1
						for start >= 0 && (data[start] == ' ' || data[start] == '\n' || data[start] == '\t') {
							start--
						}
						end := j
						for start >= 0 && data[start] != '"' {
							start--
						}
						if start >= 0 && end > start+1 {
							return string(data[start+1 : end])
						}
						break
					}
				}
				break
			}
		}
	}
	return ""
}

// findFieldInObject looks for a field value within the first N bytes of data
func findFieldInObject(data []byte, field string) string {
	fieldPat := []byte(`"` + field + `"`)
	for i := 0; i <= len(data)-len(fieldPat); i++ {
		if string(data[i:i+len(fieldPat)]) == string(fieldPat) {
			// Find the colon and value
			for j := i + len(fieldPat); j < len(data); j++ {
				if data[j] == ':' {
					// Skip whitespace
					for k := j + 1; k < len(data); k++ {
						if data[k] == ' ' || data[k] == '\n' || data[k] == '\t' {
							continue
						}
						if data[k] == '"' {
							// Extract string value
							for l := k + 1; l < len(data); l++ {
								if data[l] == '"' {
									return string(data[k+1 : l])
								}
							}
						}
						break
					}
					break
				}
			}
		}
	}
	return ""
}

// findContainingObject returns the index of the '{' that opens the object containing idx
func findContainingObject(data []byte, idx int) int {
	depth := 0
	for i := idx; i >= 0; i-- {
		if data[i] == '}' {
			depth++
		} else if data[i] == '{' {
			if depth > 0 {
				depth--
			} else {
				return i
			}
		}
	}
	return -1
}
