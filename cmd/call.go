package cmd

import (
	"aurl/internal/client"
	"aurl/internal/config"
	"aurl/internal/parser"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var callCmd = &cobra.Command{
	Use:   "call [api-name] [method] [path]",
	Short: "Make an HTTP request to a registered API",
	Long: `Make an HTTP request to a registered API endpoint.
Example:
  aurl petstore GET /pet/1
  aurl petstore POST /pet '{"name":"dog"}'`,
	Args: cobra.ExactArgs(3),
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

	// Create HTTP client
	httpClient := client.NewHTTPClient()

	// Build request
	req := client.Request{
		Method: method,
		URL:    fullURL,
		Body:   body,
		Header: make(map[string]string),
	}

	// Add auth header if configured
	if api.Auth.Type == "bearer" && api.Auth.Header != "" {
		req.Header[api.Auth.Header] = api.Auth.Value
	} else if api.Auth.Type == "api_key" && api.Auth.Header != "" {
		req.Header[api.Auth.Header] = api.Auth.Value
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
