package cli

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/diskspace"
	"github.com/cysec-env/cysec/internal/ui"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Bootstrap or update the CySec.env environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			full, _ := cmd.Flags().GetBool("full")
			core, _ := cmd.Flags().GetBool("core")
			custom, _ := cmd.Flags().GetBool("custom")
			skipConfirm, _ := cmd.Flags().GetBool("yes")
			checkOnly, _ := cmd.Flags().GetBool("check-only")

			profile := ""
			if full {
				profile = "full"
			} else if core {
				profile = "core"
			} else if custom {
				profile = "custom"
			} else {
				// Default to core if nothing specified
				profile = "core"
			}

			ui.Section(fmt.Sprintf("CySec.env Setup (%s profile)", profile))

			// 1. Gather tools based on profile
			tools := app.Registry.ByProfile(profile)
			if len(tools) == 0 {
				ui.Error(fmt.Sprintf("No tools found for profile: %s", profile))
				return nil
			}

			// 2. Disk Space Calculation
			ui.Info("Calculating disk space requirements...")
			est := diskspace.EstimateProfile(tools, runtime.GOOS, runtime.GOARCH)

			availBytes, err := diskspace.CheckAvailable(app.Platform.DataDir)
			if err != nil {
				ui.Warn(fmt.Sprintf("Could not check available disk space: %v", err))
				availBytes = 0 // unknown
			}

			fmt.Println()
			ui.KeyValue("Total Registry Tools:", fmt.Sprintf("%d", est.TotalCount), 24)
			ui.KeyValue("PREINSTALLED:", fmt.Sprintf("%d", est.PreinstalledCount), 24)
			ui.KeyValue("SYSTEM_INTEGRATED:", fmt.Sprintf("%d", est.SystemIntegratedCount), 24)
			ui.KeyValue("PLATFORM_SPECIFIC:", fmt.Sprintf("%d", est.PlatformSpecificCount), 24)
			ui.KeyValue("REGISTRY_ONLY:", fmt.Sprintf("%d", est.RegistryOnlyCount), 24)
			if est.UnsupportedCount > 0 {
				ui.KeyValue("Unsupported on Host:", fmt.Sprintf("%d", est.UnsupportedCount), 24)
			}

			fmt.Println()
			ui.KeyValue("Estimated Download:", diskspace.FormatSize(est.DownloadSize), 24)
			ui.KeyValue("Estimated Installed:", diskspace.FormatSize(est.InstalledSize), 24)
			ui.KeyValue("Temporary Space:", diskspace.FormatSize(est.TempSize), 24)
			ui.KeyValue("Runtimes (approx):", diskspace.FormatSize(est.RuntimeSize), 24)
			ui.KeyValue("Total Required:", diskspace.FormatSize(est.TotalRequired), 24)
			if availBytes > 0 {
				ui.KeyValue("Available Space:", diskspace.FormatSize(availBytes), 24)

				if availBytes < est.TotalRequired {
					fmt.Println()
					ui.Error("Insufficient disk space for installation.")
					if !checkOnly {
						return fmt.Errorf("insufficient disk space")
					}
				}
			}

			if checkOnly {
				ui.Success("Preflight checks passed.")
				return nil
			}

			// Confirm installation
			if !skipConfirm && app.Config.GetBool("install.confirm") {
				if !ui.Confirm("Proceed with setup?", true) {
					ui.Info("Setup cancelled.")
					return nil
				}
			}

			// Update state tracking
			app.DB.SetMeta("setup_profile", profile)
			app.DB.SetMeta("setup_started_at", time.Now().Format(time.RFC3339))
			app.DB.SetMeta("setup_tools_total", fmt.Sprintf("%d", est.PreinstalledCount+est.PlatformSpecificCount))

			fmt.Println()
			ui.Info(fmt.Sprintf("Beginning batch installation of %d tools...", est.PreinstalledCount+est.PlatformSpecificCount))

			// Use the existing handleProfileInstall logic
			err = handleProfileInstall(app, profile, false, true) // skip confirm because we just did it

			app.DB.SetMeta("setup_completed_at", time.Now().Format(time.RFC3339))

			return err
		},
	}

	cmd.Flags().Bool("full", false, "Install the Full security profile (~110 tools)")
	cmd.Flags().Bool("core", false, "Install the Core security profile (22 tools)")
	cmd.Flags().Bool("custom", false, "Custom installation (interactive)")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().Bool("check-only", false, "Run preflight checks only")

	return cmd
}
