package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <tool|all>",
		Short: "Update an installed tool or all tools",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			target := args[0]

			if target == "all" {
				return updateAll(app)
			}

			return updateTool(app, target)
		},
	}

	return cmd
}

func updateTool(app *App, toolName string) error {
	// Check if installed
	installed, err := app.DB.Get(toolName)
	if err != nil || installed == nil {
		ui.Error(fmt.Sprintf("Tool '%s' is not installed. Use 'cysec install %s' first.", toolName, toolName))
		return nil
	}

	// Look up registry for latest info
	tool := app.Registry.Lookup(toolName)
	if tool == nil {
		ui.Warn(fmt.Sprintf("Tool '%s' is installed but no longer in the registry.", toolName))
		return nil
	}

	ui.Section(fmt.Sprintf("Update %s", tool.DisplayName))
	ui.KeyValue("Current version:", installed.Version, 20)
	ui.KeyValue("Registry version:", tool.Version, 20)

	if installed.Version == tool.Version {
		ui.Success("Already up to date.")
		return nil
	}

	fmt.Println()
	ui.Step("Reinstalling with latest version...")

	// Uninstall old version
	_ = app.Engine.Uninstall(tool, app.DB)

	// Install new version
	result, err := app.Engine.Install(tool, app.DB, app.Shims)
	if err != nil {
		ui.Error(fmt.Sprintf("Update failed: %v", err))
		return nil
	}

	// Recreate shim
	if result.EntryPoint != "" {
		shimName := tool.EntryCommand
		if shimName == "" {
			shimName = tool.ID
		}
		_, _ = app.Shims.Create(shimName, result.EntryPoint, tool.InterfaceType)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("%s updated to %s.", tool.DisplayName, tool.Version))
	return nil
}

func updateAll(app *App) error {
	installed, err := app.DB.ListInstalled()
	if err != nil {
		return fmt.Errorf("failed to list installed tools: %w", err)
	}

	if len(installed) == 0 {
		ui.Info("No tools installed. Nothing to update.")
		return nil
	}

	ui.Section("Update All Tools")
	fmt.Printf("  Checking %d installed tools...\n\n", len(installed))

	updated := 0
	failed := 0

	for _, inst := range installed {
		tool := app.Registry.Lookup(inst.ID)
		if tool == nil {
			continue
		}

		if inst.Version == tool.Version {
			fmt.Printf("  %s %s  %s\n", ui.StyleSuccess.Render(ui.SymbolCheck), inst.Name, ui.StyleDim.Render("up to date"))
			continue
		}

		fmt.Printf("  %s %s  %s → %s\n", ui.StyleInfo.Render(ui.SymbolArrow), inst.Name, inst.Version, tool.Version)
		_ = app.Engine.Uninstall(tool, app.DB)
		result, err := app.Engine.Install(tool, app.DB, app.Shims)
		if err != nil {
			fmt.Printf("  %s %s  %s\n", ui.StyleError.Render(ui.SymbolCross), inst.Name, err.Error())
			failed++
			continue
		}

		if result.EntryPoint != "" {
			shimName := tool.EntryCommand
			if shimName == "" {
				shimName = tool.ID
			}
			_, err = app.Shims.Create(shimName, result.EntryPoint, tool.InterfaceType)
			if err != nil {
				fmt.Printf("  %s %s  failed to create shim: %v\n", ui.StyleError.Render(ui.SymbolCross), inst.Name, err)
				failed++
				continue
			}
		}
		updated++
	}

	fmt.Println()
	if updated > 0 {
		ui.Success(fmt.Sprintf("%d tools updated.", updated))
	}
	if failed > 0 {
		ui.Warn(fmt.Sprintf("%d tools failed to update.", failed))
	}
	if updated == 0 && failed == 0 {
		ui.Success("All tools are up to date.")
	}
	fmt.Println()

	return nil
}
