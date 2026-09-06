package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/ui"
)

func newStorageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "storage status",
		Short: "Show storage usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			ui.Section("Storage Status")

			// Installed tools size
			totalInstalled, _ := app.DB.TotalInstalledSize()
			ui.KeyValue("Installed tools:", ui.FormatSize(totalInstalled), 22)

			// Cache size
			cacheStatus, _ := app.Cache.GetStatus()
			if cacheStatus != nil {
				ui.KeyValue("Downloads cache:", ui.FormatSize(cacheStatus.DownloadSize), 22)
				ui.KeyValue("Temporary files:", ui.FormatSize(cacheStatus.TmpSize), 22)
				ui.KeyValue("Reclaimable:", ui.FormatSize(cacheStatus.Reclaimable), 22)
			}

			// Workspace (protected)
			fmt.Println()
			fmt.Printf("  %s  Workspace: %s %s\n", ui.SymbolInfo, app.Platform.WorkspaceDir(), ui.StyleDim.Render("(protected)"))

			// Data directory
			fmt.Printf("  %s  Data directory: %s\n", ui.SymbolInfo, app.Platform.DataDir)

			fmt.Println()
			return nil
		},
	}
}

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage CySec.env download and temporary cache",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Show cache usage statistics",
			RunE: func(cmd *cobra.Command, args []string) error {
				app, err := initApp()
				if err != nil {
					return err
				}
				defer app.DB.Close()

				status, err := app.Cache.GetStatus()
				if err != nil {
					return err
				}

				ui.Section("Cache Status")
				ui.KeyValue("Downloads:", ui.FormatSize(status.DownloadSize), 22)
				ui.KeyValue("Temporary data:", ui.FormatSize(status.TmpSize), 22)
				fmt.Println()
				ui.KeyValue("Total:", ui.FormatSize(status.TotalSize), 22)
				ui.KeyValue("Reclaimable:", ui.FormatSize(status.Reclaimable), 22)
				fmt.Println()
				fmt.Printf("  %s  User workspace: %s\n", ui.SymbolInfo, ui.StyleDim.Render("Protected"))
				fmt.Println()

				return nil
			},
		},
	)

	clearCmd := &cobra.Command{
		Use:   "clear [tool]",
		Short: "Clear cached data",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			if len(args) > 0 {
				// Clear specific tool cache
				toolName := args[0]
				if err := app.Cache.ClearTool(toolName); err != nil {
					return err
				}
				ui.Success(fmt.Sprintf("Cache cleared for %s.", toolName))
				return nil
			}

			verified, _ := cmd.Flags().GetBool("verified")
			if verified {
				var installed []string
				for _, t := range app.DB.GetAll() {
					if t.HealthStatus != "broken" {
						installed = append(installed, t.ID)
					}
				}
				
				freedDls, err := app.Cache.CleanVerifiedDownloads(installed)
				if err != nil {
					return err
				}
				
				freedBuilds, _ := app.Cache.CleanBuilds()
				freedTemp, _ := app.Cache.CleanTemp()
				
				totalFreed := freedDls + freedBuilds + freedTemp
				ui.Success(fmt.Sprintf("Cleaned verified downloads, builds, and temp data. Freed %s.", ui.FormatSize(totalFreed)))
				return nil
			}

			// Clear all cache
			status, _ := app.Cache.GetStatus()
			if status != nil && status.TotalSize == 0 {
				ui.Info("Cache is already empty.")
				return nil
			}

			fmt.Println()
			ui.Warn("This will remove all cached downloads and temporary files.")
			if status != nil {
				fmt.Printf("  Space to be freed: %s\n", ui.FormatSize(status.Reclaimable))
			}
			fmt.Printf("  %s User workspace will NOT be affected.\n", ui.SymbolInfo)

			if !ui.Confirm("Clear cache?", false) {
				ui.Info("Cancelled.")
				return nil
			}

			freed, err := app.Cache.ClearAll()
			if err != nil {
				return err
			}

			ui.Success(fmt.Sprintf("Cache cleared. Freed %s.", ui.FormatSize(freed)))
			return nil
		},
	}

	clearCmd.Flags().Bool("verified", false, "Clear downloads only for verified/installed tools, plus builds and temp files")
	cmd.AddCommand(clearCmd)

	return cmd
}
