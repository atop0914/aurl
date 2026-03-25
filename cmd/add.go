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

	var baseURL string
	var specType string

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

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read spec body: %w", err)
		}
		_ = string(body) // spec content available for future parsing
		baseURL = addFlags.baseURL
		specType = "openapi3"
	} else {
		// It's a file path
		body, err := os.ReadFile(specRef)
		if err != nil {
			return fmt.Errorf("failed to read spec file: %w", err)
		}
		_ = string(body) // spec content available for future parsing
		baseURL = addFlags.baseURL
		specType = "openapi3"
	}

	if addFlags.graphql {
		specType = "graphql"
		if baseURL == "" {
			baseURL = specRef
		}
	}

	// Build auth config from headers
	auth := config.AuthConfig{Type: "none"}
	if len(addFlags.headers) > 0 {
		for _, h := range addFlags.headers {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				auth.Type = "bearer"
				auth.Header = strings.TrimSpace(parts[0])
				auth.Value = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	api := config.APIConfig{
		Name:    name,
		SpecURL: specRef,
		BaseURL: baseURL,
		Type:    specType,
		Auth:    auth,
	}

	if err := cfg.AddAPI(api); err != nil {
		return fmt.Errorf("failed to save API: %w", err)
	}

	aliasNote := ""
	if specType == "graphql" {
		aliasNote = " (GraphQL)"
	}
	fmt.Printf("Registered %q%s\n", name, aliasNote)
	if baseURL != "" {
		fmt.Printf("Base URL: %s\n", baseURL)
	}

	return nil
}
