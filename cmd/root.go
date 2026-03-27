package cmd

import (
	"aurl/internal/config"
	"aurl/internal/parser"
	"aurl/internal/spec"
	"fmt"
	"os"
	"sort"

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
