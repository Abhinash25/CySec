package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
)

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <tool>",
		Short: "Uninstall a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			toolName := args[0]
			skipConfirm, _ := cmd.Flags().GetBool("yes")

			// Check if installed
			installed, err := app.DB.Get(toolName)
			if err != nil || installed == nil {
				ui.Error(fmt.Sprintf("Tool '%s' is not installed.", toolName))
				return nil
			}

			// Look up registry entry
			tool := app.Registry.Lookup(toolName)

			ui.Section(fmt.Sprintf("Uninstall %s", installed.Name))
			width := 22
			ui.KeyValue("Version:", installed.Version, width)
			ui.KeyValue("Install Path:", installed.InstallPath, width)
			if installed.InstalledSize > 0 {
				ui.KeyValue("Size:", ui.FormatSize(installed.InstalledSize), width)
			}

			if !skipConfirm {
				if !ui.Confirm("Uninstall this tool?", false) {
					ui.Info("Uninstall cancelled.")
					return nil
				}
			}

			fmt.Println()

			// Remove shim first
			shimName := installed.EntryCommand
			if shimName == "" {
				shimName = installed.ID
			}
			app.Shims.Remove(shimName)
			ui.Step("Removed command shim")

			// Uninstall
			if tool != nil {
				if err := app.Engine.Uninstall(tool, app.DB); err != nil {
					ui.Error(fmt.Sprintf("Uninstall failed: %v", err))
					return nil
				}
			} else {
				// No registry entry, just remove DB record
				app.DB.Remove(toolName)
			}

			ui.Step("Removed tool files")

			fmt.Println()
			ui.Success(fmt.Sprintf("%s has been uninstalled.", installed.Name))
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	return cmd
}
