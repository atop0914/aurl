package cmd

import (
	"aurl/internal/config"
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered APIs",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	apis := cfg.ListAPIs()
	if len(apis) == 0 {
		fmt.Println("No APIs registered yet. Run 'aurl add <name> <spec>' to register one.")
		return nil
	}

	fmt.Println("Registered APIs:")
	for _, name := range apis {
		api, _ := cfg.GetAPI(name)
		fmt.Printf("  %-20s %s\n", name, api.SpecURL)
	}
	return nil
}
