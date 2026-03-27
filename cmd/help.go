package cmd

import (
	"aurl/internal/config"
	"aurl/internal/parser"
	"fmt"

	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help [api-name]",
	Short: "Show API endpoints grouped by tag",
	Long: `Show all endpoints for a registered API, grouped by tag.
This is equivalent to running 'aurl <name> --help'.`,
	Example: `  aurl help petstore
  aurl petstore --help`,
	Args: cobra.ExactArgs(1),
	RunE: runHelp,
}

func init() {
	rootCmd.AddCommand(helpCmd)
}

func runHelp(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	api, ok := cfg.GetAPI(name)
	if !ok {
		return fmt.Errorf("API %q is not registered. Run 'aurl add %s <spec>' to register it", name, name)
	}

	// Parse the OpenAPI spec
	p := parser.NewOpenAPI3()
	openAPI, err := p.Parse(api.SpecURL)
	if err != nil {
		return fmt.Errorf("failed to parse spec for %s: %w", name, err)
	}

	printAPIHelpFromSpec(name, api.BaseURL, openAPI)
	return nil
}
