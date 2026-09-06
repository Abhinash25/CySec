package cli

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
	"github.com/cysec-env/cysec/internal/verify"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <tool>",
		Short: "Show detailed information about a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			toolName := args[0]
			tool := app.Registry.Lookup(toolName)

			if tool == nil {
				ui.Error(fmt.Sprintf("Tool '%s' not found in registry.", toolName))
				ui.Info("Use 'cysec search <query>' to find tools.")
				return nil
			}

			ui.Section(tool.DisplayName)

			width := 24
			ui.KeyValue("ID:", tool.ID, width)
			ui.KeyValue("Description:", tool.Description, width)
			ui.KeyValue("Categories:", strings.Join(tool.Categories, ", "), width)

			if len(tool.Aliases) > 0 {
				ui.KeyValue("Aliases:", strings.Join(tool.Aliases, ", "), width)
			}

			ui.KeyValue("License:", tool.License, width)
			ui.KeyValue("Version:", tool.Version, width)
			ui.KeyValue("Entry Command:", tool.EntryCommand, width)
			ui.KeyValue("Install Method:", tool.InstallationMethod, width)
			ui.KeyValue("Adapter:", tool.InstallerAdapter, width)

			if tool.OfficialWebsite != "" {
				ui.KeyValue("Website:", tool.OfficialWebsite, width)
			}
			if tool.OfficialRepository != "" {
				ui.KeyValue("Repository:", tool.OfficialRepository, width)
			}

			// Registry Status
			fmt.Println()
			fmt.Println(ui.StyleBold.Render("  Registry Metadata:"))
			ui.KeyValue("    Trust Level:", string(verify.EvaluateTrust(tool)), width)
			ui.KeyValue("    Lifecycle Status:", tool.ToolStatus, width)
			if tool.EstimatedDownloadSize != "" {
				ui.KeyValue("    Download Size:", tool.EstimatedDownloadSize, width)
			}
			if tool.EstimatedInstalledSize != "" {
				ui.KeyValue("    Installed Size:", tool.EstimatedInstalledSize, width)
			}

			// Platform support
			fmt.Println()
			fmt.Println(ui.StyleBold.Render("  Platform Support:"))
			for _, p := range tool.Platforms {
				status := p.Support
				switch status {
				case "SUPPORTED":
					status = ui.StyleSuccess.Render(status)
				case "LIMITED":
					status = ui.StyleWarning.Render(status)
				case "EXPERIMENTAL":
					status = ui.StyleWarning.Render(status)
				case "UNSUPPORTED":
					status = ui.StyleError.Render(status)
				}

				arch := p.Arch
				if arch == "" {
					arch = "all"
				}

				current := ""
				if p.OS == runtime.GOOS {
					current = " ← current"
				}

				fmt.Printf("    %s %s/%s  %s%s\n", ui.SymbolBullet, p.OS, arch, status, ui.StyleDim.Render(current))
				if p.Notes != "" {
					fmt.Printf("      %s\n", ui.StyleDim.Render(p.Notes))
				}
			}

			// Installation status
			fmt.Println()
			fmt.Println(ui.StyleBold.Render("  Installation Status:"))
			if app.DB.IsInstalled(tool.ID) {
				installed, _ := app.DB.Get(tool.ID)
				if installed != nil {
					fmt.Println(ui.StyleSuccess.Render("    ✓ Installed"))
					ui.KeyValue("    Version:", installed.Version, width)
					ui.KeyValue("    Path:", installed.InstallPath, width)
					ui.KeyValue("    Health:", installed.HealthStatus, width)
				}
			} else {
				fmt.Printf("    %s Not installed. Use 'cysec install %s' to install.\n", ui.StyleDim.Render("○"), tool.ID)
			}

			if tool.Deprecated {
				fmt.Println()
				ui.Warn(fmt.Sprintf("This tool is deprecated. Replacement: %s", tool.Replacement))
			}

			fmt.Println()
			return nil
		},
	}
}
