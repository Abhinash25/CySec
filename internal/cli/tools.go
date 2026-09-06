package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
)

func newToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List available tools",
		Long:  "List all tools in the CySec.env registry. Use 'tools installed' to show only installed tools.",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			tools := app.Registry.All()
			if len(tools) == 0 {
				ui.Warn("No tools found in registry. Run 'cysec registry update' to refresh.")
				return nil
			}

			sort.Slice(tools, func(i, j int) bool {
				return tools[i].DisplayName < tools[j].DisplayName
			})

			ui.Section("CySec.env Tool Registry")
			fmt.Printf("  %d tools available\n\n", len(tools))

			headers := []string{"Name", "Category", "Status", "Adapter"}
			var rows [][]string

			for _, t := range tools {
				status := "available"
				if app.DB.IsInstalled(t.ID) {
					status = ui.StyleSuccess.Render("installed")
				}

				cat := ""
				if len(t.Categories) > 0 {
					cat = t.Categories[0]
				}

				rows = append(rows, []string{
					t.DisplayName,
					cat,
					status,
					t.InstallerAdapter,
				})
			}

			ui.Table(headers, rows)
			fmt.Println()
			return nil
		},
	}

	cmd.AddCommand(newToolsInstalledCmd())
	cmd.AddCommand(newToolsCategoryCmd())

	return cmd
}

func newToolsInstalledCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "installed",
		Short: "List installed tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			installed, err := app.DB.ListInstalled()
			if err != nil {
				return fmt.Errorf("failed to list installed tools: %w", err)
			}

			if len(installed) == 0 {
				ui.Info("No tools installed yet. Use 'cysec install <tool>' to install a tool.")
				return nil
			}

			ui.Section("Installed Tools")

			headers := []string{"Name", "Version", "Adapter", "Health"}
			var rows [][]string

			for _, t := range installed {
				health := t.HealthStatus
				switch health {
				case "healthy":
					health = ui.StyleSuccess.Render("healthy")
				case "broken":
					health = ui.StyleError.Render("broken")
				default:
					health = ui.StyleWarning.Render("unknown")
				}

				rows = append(rows, []string{
					t.Name,
					t.Version,
					t.Adapter,
					health,
				})
			}

			ui.Table(headers, rows)
			fmt.Println()
			return nil
		},
	}
}

func newToolsCategoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "category [category-name]",
		Short: "List tools in a category",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			category := strings.Join(args, " ")
			tools := app.Registry.ByCategory(category)

			if len(tools) == 0 {
				ui.Warn(fmt.Sprintf("No tools found in category '%s'", category))
				fmt.Println()
				ui.Info("Available categories:")
				for _, c := range app.Registry.Categories() {
					fmt.Printf("  %s %s\n", ui.SymbolBullet, c)
				}
				return nil
			}

			ui.Section(fmt.Sprintf("Category: %s", category))

			headers := []string{"Name", "Description", "Status"}
			var rows [][]string

			for _, t := range tools {
				status := "available"
				if app.DB.IsInstalled(t.ID) {
					status = ui.StyleSuccess.Render("installed")
				}

				desc := t.Description
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}

				rows = append(rows, []string{
					t.DisplayName,
					desc,
					status,
				})
			}

			ui.Table(headers, rows)
			fmt.Println()
			return nil
		},
	}
}
