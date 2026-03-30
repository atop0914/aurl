package cmd

import (
	"aurl/internal/client"
	"aurl/internal/config"
	"aurl/internal/parser"
	"aurl/internal/validator"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var callCmd = &cobra.Command{
	Use:   "call [api-name] [method] [path] [body]",
	Short: "Make an HTTP request to a registered API",
	Long: `Make an HTTP request to a registered API endpoint.
Examples:
  aurl petstore GET /pet/1
  aurl petstore POST /pet '{"name":"dog"}'
  aurl petstore GET '/pet/findByStatus?status=available'
  aurl petstore PUT /pet '{"id":1,"name":"dog"}'
  aurl petstore DELETE /pet/1`,
	Args: cobra.RangeArgs(3, 4),
	RunE: runCall,
}

func init() {
	rootCmd.AddCommand(callCmd)
}

func runCall(cmd *cobra.Command, args []string) error {
	name := args[0]
	method := strings.ToUpper(args[1])
	path := args[2]
	body := ""

	// If there's a 4th argument, it's the request body
	if len(args) > 3 {
		body = args[3]
	}

	// Validate method
	validMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"PATCH":  true,
		"DELETE": true,
	}
	if !validMethods[method] {
		return fmt.Errorf("invalid method %q. Supported: GET, POST, PUT, PATCH, DELETE", method)
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

	// Parse the spec to get base URL and endpoints
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

	// Build the full URL (handles query params in path)
	fullURL := client.BuildURL(baseURL, path)

	// Extract path and query params for validation
	_, queryParams, _ := validator.ExtractParamsFromURL(fullURL)

	// Find the endpoint in the spec for validation — strip query string
	pathOnly := strings.SplitN(path, "?", 2)[0]
	endpoint := p.FindEndpoint(openAPI, method, pathOnly)
	if endpoint != nil {
		// Build request validation context
		reqVal := &validator.RequestValidation{
			QueryParams:  queryParams,
			HeaderParams: make(map[string]string),
		}

		// Validate the request
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

	// Inject auth — use explicit config first, then auto-detect from spec
	if api.Auth.Type != "none" && api.Auth.Header != "" {
		// Use configured auth
		if api.Auth.Type == "bearer" {
			req.Header[api.Auth.Header] = api.Auth.Value
		} else if api.Auth.Type == "api_key" {
			req.Header[api.Auth.Header] = api.Auth.Value
		}
	} else {
		// Auto-detect auth from spec
		if endpoint != nil {
			h, v, t := validator.AutoDetectAuth(endpoint, openAPI.SecuritySchemes)
			if h != "" && v != "" {
				if t == "bearer" && h == "Authorization" {
					// For bearer, replace <TOKEN> with empty and let user know
					// We don't have a token, but we inject a placeholder
					req.Header[h] = v
				} else {
					req.Header[h] = v
				}
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
