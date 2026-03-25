package cmd

import (
	"fmt"
	"os"

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
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/aurl/config.json)")
}
