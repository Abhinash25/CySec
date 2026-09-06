package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
)

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search for tools in the registry",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			query := strings.Join(args, " ")
			results := app.Registry.Search(query)

			if len(results) == 0 {
				ui.Warn(fmt.Sprintf("No tools found matching '%s'", query))
				fmt.Println()
				ui.Info("Try a broader search term or run 'cysec tools' to list all tools.")
				return nil
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].DisplayName < results[j].DisplayName
			})

			ui.Section(fmt.Sprintf("Search Results for '%s'", query))
			fmt.Printf("  %d matches found\n\n", len(results))

			headers := []string{"Name", "Category", "Adapter", "Registry Status", "Install Status", "Version"}
			var rows [][]string

			for _, t := range results {
				installStatus := "Available"
				if app.DB.IsInstalled(t.ID) {
					installStatus = ui.StyleSuccess.Render("Installed")
				}

				cat := ""
				if len(t.Categories) > 0 {
					cat = t.Categories[0]
				}

				rows = append(rows, []string{
					t.DisplayName,
					cat,
					t.InstallerAdapter,
					t.ToolStatus,
					installStatus,
					t.Version,
				})
			}

			ui.Table(headers, rows)
			fmt.Println()
			return nil
		},
	}
}
