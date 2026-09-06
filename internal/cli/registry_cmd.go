package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/cysec-env/cysec/internal/ui"
)

func newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage and validate the CySec.env registry",
	}

	cmd.AddCommand(newRegistryValidateCmd())

	return cmd
}

func newRegistryValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Perform full strict validation of the tool registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			ui.Section("Registry Validation")
			ui.Info("Running strict static analysis on registry definitions...")

			result := app.Registry.ValidateAll()

			fmt.Println()
			ui.KeyValue("Total Entries Scanned:", fmt.Sprintf("%d", result.TotalEntries), 25)
			ui.KeyValue("Valid Entries:", fmt.Sprintf("%d", result.ValidEntries), 25)
			if result.InvalidEntries > 0 {
				ui.KeyValue("Invalid Entries:", ui.StyleError.Render(fmt.Sprintf("%d", result.InvalidEntries)), 25)
			}
			if result.DuplicateIDs > 0 {
				ui.KeyValue("Duplicate IDs:", ui.StyleError.Render(fmt.Sprintf("%d", result.DuplicateIDs)), 25)
			}
			if result.AliasCollisions > 0 {
				ui.KeyValue("Alias Collisions:", ui.StyleError.Render(fmt.Sprintf("%d", result.AliasCollisions)), 25)
			}
			fmt.Println()

			if result.HasErrors() {
				ui.Error("Registry validation failed with the following critical errors:")
				for _, errStr := range result.Errors {
					fmt.Printf("  %s %s\n", ui.StyleError.Render("✖"), errStr)
				}
				os.Exit(1)
			}

			ui.Success("Registry is valid. No critical errors found.")
			return nil
		},
	}
}
