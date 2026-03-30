package cmd

import (
	"aurl/internal/client"
	"aurl/internal/config"
	"aurl/internal/parser"
	"aurl/internal/spec"
	"aurl/internal/validator"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "aurl",
	Short: "aurl — turn any API into a CLI command",
	Long: `aurl is a CLI tool that turns APIs (OpenAPI 3.x, Swagger 2.0, GraphQL)
into simple CLI commands. Register an API once, then call endpoints like:
  aurl petstore GET /pet/1
  aurl linear '{ viewer { name } }'`,
	Args:             validateArgs,
	TraverseChildren: true,
	RunE:             runRoot,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/aurl/config.json)")
}

func validateArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}

	// Check if --help or -h was passed
	helpFlag, _ := cmd.Flags().GetBool("help")
	hFlag, _ := cmd.Flags().GetBool("h")
	if !helpFlag && !hFlag {
		return nil
	}

	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	api, ok := cfg.GetAPI(name)
	if !ok {
		return nil
	}

	// It's an API name with --help, show API endpoints
	p := parser.NewOpenAPI3()
	openAPI, err := p.Parse(api.SpecURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing spec for %s: %v\n", name, err)
		os.Exit(1)
	}

	printAPIHelpFromSpec(name, api.BaseURL, openAPI)
	os.Exit(0)
	return nil
}

func runRoot(cmd *cobra.Command, args []string) error {
	// Handle "aurl <name> <method> <path>" pattern
	if len(args) >= 3 {
		name := args[0]
		method := strings.ToUpper(args[1])
		path := args[2]
		body := ""

		// If there's a 4th argument, it's the request body
		if len(args) > 3 {
			body = args[3]
		}

		// Validate HTTP method
		validMethods := map[string]bool{
			"GET": true, "POST": true, "PUT": true, "DELETE": true,
			"PATCH": true, "OPTIONS": true, "HEAD": true,
		}
		if !validMethods[method] {
			return fmt.Errorf("invalid HTTP method: %s", method)
		}

		// Load config
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get the API config
		api, ok := cfg.GetAPI(name)
		if !ok {
			return fmt.Errorf("API %q is not registered. Run 'aurl add %s <spec>' to register it", name, name)
		}

		// Parse the spec to get base URL
		p := parser.NewOpenAPI3()
		openAPI, err := p.Parse(api.SpecURL)
		if err != nil {
			return fmt.Errorf("failed to parse spec for %s: %w", name, err)
		}

		// Determine base URL
		baseURL := api.BaseURL
		if baseURL == "" {
			baseURL = openAPI.BaseURL
		}
		if baseURL == "" {
			return fmt.Errorf("no base URL found for API %q", name)
		}

		// Build the full URL
		fullURL := client.BuildURL(baseURL, path)

		// Extract params for validation
		_, queryParams, _ := validator.ExtractParamsFromURL(fullURL)

		// Find endpoint for validation — use path without query params
		pathOnly := strings.SplitN(path, "?", 2)[0]
		endpoint := p.FindEndpoint(openAPI, method, pathOnly)
		if endpoint != nil {
			reqVal := &validator.RequestValidation{
				QueryParams:  queryParams,
				HeaderParams: make(map[string]string),
			}
			if err := validator.ValidateEndpoint(endpoint, reqVal); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		}

		// Create HTTP client
		httpClient := client.NewHTTPClient()

		// Build request
		req := client.Request{
			Method: method,
			URL:    fullURL,
			Body:   body,
			Header: make(map[string]string),
		}

		// Inject auth — explicit config first, then auto-detect from spec
		if api.Auth.Type != "none" && api.Auth.Header != "" {
			if api.Auth.Type == "bearer" || api.Auth.Type == "api_key" {
				req.Header[api.Auth.Header] = api.Auth.Value
			}
		} else {
			// Auto-detect auth from spec
			if endpoint != nil {
				h, v, t := validator.AutoDetectAuth(endpoint, openAPI.SecuritySchemes)
				if h != "" && v != "" {
					req.Header[h] = v
					_ = t
				}
			}
		}

		// Execute the request
		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}

		// Print response
		fmt.Fprintf(os.Stdout, "%s\n", resp.Body)
		return nil
	}

	return fmt.Errorf("unknown command. Use 'aurl --help' for usage")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printAPIHelpFromSpec(name string, baseURL string, parsed *spec.ParsedSpec) {
	// Determine base URL
	if baseURL == "" {
		baseURL = parsed.BaseURL
	}

	// Print header
	fmt.Printf("# %s\n", name)
	if parsed.Title != "" && parsed.Title != name {
		fmt.Printf("# %s\n", parsed.Title)
	}
	if parsed.Version != "" {
		fmt.Printf("# Version: %s\n", parsed.Version)
	}
	if baseURL != "" {
		fmt.Printf("# Base URL: %s\n", baseURL)
	}
	fmt.Println()
	fmt.Printf("Run API calls like: aurl %s <METHOD> <PATH>\n", name)
	fmt.Println()
	fmt.Println("## Endpoints")

	// Get sorted tags
	var sortedTags []string
	for tag := range parsed.TagGroups {
		sortedTags = append(sortedTags, tag)
	}
	sort.Strings(sortedTags)

	for _, tag := range sortedTags {
		endpoints := parsed.TagGroups[tag]
		fmt.Printf("\n### %s\n", tag)
		for _, ep := range endpoints {
			colorMethod := colorMethod(ep.Method)
			fmt.Printf("  %s %s\n", colorMethod, ep.Path)
		}
	}
}

func colorMethod(method string) string {
	switch method {
	case "GET":
		return fmt.Sprintf("\033[32m%s\033[0m", method)
	case "POST":
		return fmt.Sprintf("\033[34m%s\033[0m", method)
	case "PUT":
		return fmt.Sprintf("\033[33m%s\033[0m", method)
	case "DELETE":
		return fmt.Sprintf("\033[31m%s\033[0m", method)
	case "PATCH":
		return fmt.Sprintf("\033[35m%s\033[0m", method)
	default:
		return method
	}
}
