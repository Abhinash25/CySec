package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/preflight"
	"github.com/cysec-env/cysec/internal/registry"
	"github.com/cysec-env/cysec/internal/ui"
	"github.com/cysec-env/cysec/internal/verify"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [tool]",
		Short: "Install a tool or profile from the registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			skipConfirm, _ := cmd.Flags().GetBool("yes")
			profile, _ := cmd.Flags().GetString("profile")
			checkOnly, _ := cmd.Flags().GetBool("check")

			if profile == "" && len(args) == 0 {
				return fmt.Errorf("must specify a tool to install or use --profile")
			}
			if profile != "" && len(args) > 0 {
				return fmt.Errorf("cannot specify both a tool and a profile")
			}

			if profile != "" {
				return handleProfileInstall(app, profile, checkOnly, skipConfirm)
			}

			toolName := args[0]
			if checkOnly {
				return fmt.Errorf("--check is currently only supported with --profile")
			}

			// Check if already installed
			if app.DB.IsInstalled(toolName) {
				installed, _ := app.DB.Get(toolName)
				if installed != nil {
					ui.Info(fmt.Sprintf("%s is already installed (version %s).", installed.Name, installed.Version))
					ui.Info("Use 'cysec update " + toolName + "' to update.")
					return nil
				}
			}

			// Look up in registry
			tool := app.Registry.Lookup(toolName)
			if tool == nil {
				ui.Error(fmt.Sprintf("Tool '%s' not found in the CySec.env registry.", toolName))
				fmt.Println()
				ui.Info("CySec.env searched:")
				fmt.Printf("  %s Installed tools\n", ui.SymbolBullet)
				fmt.Printf("  %s Tool aliases\n", ui.SymbolBullet)
				fmt.Printf("  %s Official registry\n", ui.SymbolBullet)
				fmt.Println()
				ui.Info("No supported match was found.")
				ui.Info("Try 'cysec search <query>' to search for tools.")
				return nil
			}

			// Show tool info before installation
			ui.Section(fmt.Sprintf("Install %s", tool.DisplayName))
			width := 22
			ui.KeyValue("Description:", tool.Description, width)
			if len(tool.Categories) > 0 {
				ui.KeyValue("Category:", tool.Categories[0], width)
			}
			ui.KeyValue("Version:", tool.Version, width)
			ui.KeyValue("Adapter:", tool.InstallerAdapter, width)
			ui.KeyValue("Trust Level:", string(verify.EvaluateTrust(tool)), width)
			if tool.EstimatedDownloadSize != "" {
				ui.KeyValue("Download Size:", tool.EstimatedDownloadSize, width)
			}
			fmt.Println()

			// Pre-installation targeted validation
			validationErrors := registry.ValidateTool(tool)
			if len(validationErrors) > 0 {
				ui.Error(fmt.Sprintf("Cannot install %s: Registry entry is invalid.", tool.DisplayName))
				for _, errStr := range validationErrors {
					fmt.Printf("  %s %s\n", ui.StyleError.Render("✖"), errStr)
				}
				os.Exit(1)
			}

			// Get installation plan
			plan, err := app.Engine.Plan(tool)
			if err != nil {
				ui.Error(fmt.Sprintf("Cannot plan installation: %v", err))
				return nil
			}

			fmt.Println()
			fmt.Println(ui.StyleBold.Render("  Installation Steps:"))
			for _, step := range plan.Steps {
				fmt.Printf("    %s %s\n", ui.SymbolArrow, step)
			}

			if len(plan.Dependencies) > 0 {
				fmt.Println()
				fmt.Println(ui.StyleBold.Render("  Dependencies:"))
				for _, dep := range plan.Dependencies {
					fmt.Printf("    %s %s\n", ui.SymbolBullet, dep)
				}
			}

			// Confirm installation
			if !skipConfirm && app.Config.GetBool("install.confirm") {
				if !ui.Confirm("Install now?", true) {
					ui.Info("Installation cancelled.")
					return nil
				}
			}

			fmt.Println()

			// Progress stages
			stages := []ui.ProgressStage{
				{Name: "Resolving installation...", Status: "active"},
				{Name: "Downloading...", Status: "pending"},
				{Name: "Verifying integrity...", Status: "pending"},
				{Name: "Installing...", Status: "pending"},
				{Name: "Configuring environment...", Status: "pending"},
				{Name: "Running health check...", Status: "pending"},
			}

			// Execute installation
			ui.RenderProgress(stages[:1])
			result, err := app.Engine.Install(tool, app.DB, app.Shims)
			if err != nil {
				stages[0].Status = "failed"
				ui.RenderProgress(stages[:1])
				ui.Error(fmt.Sprintf("Installation failed: %v", err))
				return nil
			}

			// Create command shim
			if result.EntryPoint != "" {
				shimName := tool.EntryCommand
				if shimName == "" {
					shimName = tool.ID
				}
				shimPath, err := app.Shims.Create(shimName, result.EntryPoint, tool.InterfaceType)
				if err != nil {
					ui.Warn(fmt.Sprintf("Failed to create command shim: %v", err))
				} else {
					// Update DB with shim path
					if installed, _ := app.DB.Get(tool.ID); installed != nil {
						installed.ShimPath = shimPath
					}
				}
			}

			// Show all stages as completed
			for i := range stages {
				stages[i].Status = "done"
			}
			ui.RenderProgress(stages)

			fmt.Println()
			ui.Success(fmt.Sprintf("%s installed successfully.", tool.DisplayName))
			if tool.EntryCommand != "" {
				ui.Info(fmt.Sprintf("Run '%s' to use it.", tool.EntryCommand))
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().StringP("profile", "p", "", "Install all tools in a profile (e.g. full)")
	cmd.Flags().Bool("check", false, "Run preflight checks only, do not install")

	return cmd
}

func handleProfileInstall(app *App, profile string, checkOnly, skipConfirm bool) error {
	tools := app.Registry.ByProfile(profile)
	if len(tools) == 0 {
		ui.Error(fmt.Sprintf("Profile '%s' not found or contains no tools.", profile))
		return nil
	}

	ui.Section(fmt.Sprintf("Profile: %s", strings.ToUpper(profile)))
	ui.Info(fmt.Sprintf("Found %d tools in profile.", len(tools)))

	if checkOnly {
		return runPreflight(app, tools)
	}

	ui.Info(fmt.Sprintf("Starting batch installation of %d tools...", len(tools)))
	fmt.Println()

	var success, failed, skipped int

	for i, tool := range tools {
		stepPrefix := fmt.Sprintf("[%02d/%02d]", i+1, len(tools))
		
		if app.DB.IsInstalled(tool.ID) {
			installed, _ := app.DB.Get(tool.ID)
			if installed != nil && installed.HealthStatus == "healthy" {
				ui.Step(fmt.Sprintf("%s Skipping %s (already installed and healthy)", stepPrefix, tool.DisplayName))
				skipped++
				continue
			}
		}

		if !tool.IsSupported(runtime.GOOS, runtime.GOARCH) {
			ui.Step(fmt.Sprintf("%s Skipping %s (unsupported on this platform)", stepPrefix, tool.DisplayName))
			skipped++
			continue
		}

		ui.Step(fmt.Sprintf("%s Installing %s...", stepPrefix, tool.DisplayName))

		result, err := app.Engine.Install(tool, app.DB, app.Shims)
		if err != nil {
			ui.Error(fmt.Sprintf("%s Failed to install %s: %v", stepPrefix, tool.DisplayName, err))
			failed++
			continue
		}

		if result.Ownership == "SYSTEM_MANAGED" {
			ui.Step(fmt.Sprintf("%s Registered System-Integrated tool: %s", stepPrefix, tool.DisplayName))
		}

		// Create command shim
		shimName := tool.EntryCommand
		if shimName == "" {
			shimName = tool.ID
		}
		shimPath, err := app.Shims.Create(shimName, result.EntryPoint, tool.InterfaceType)
		if err != nil {
			ui.Warn(fmt.Sprintf("Failed to create command shim: %v", err))
		} else {
			if installed, _ := app.DB.Get(tool.ID); installed != nil {
				installed.ShimPath = shimPath
			}
		}

		success++
	}

	fmt.Println()
	ui.Section("Batch Installation Summary")
	ui.KeyValue("Successful (CySec-Managed):", fmt.Sprintf("%d", success), 35)
	ui.KeyValue("Successful (System-Integrated):", "Included in Successful", 35)
	ui.KeyValue("Failed:", fmt.Sprintf("%d", failed), 35)
	ui.KeyValue("Skipped:", fmt.Sprintf("%d", skipped), 35)

	if failed > 0 {
		return fmt.Errorf("batch installation completed with %d failures (check missing system prerequisites)", failed)
	}

	ui.Success("Profile installation complete.")
	return nil
}

func runPreflight(app *App, tools []*registry.ToolEntry) error {
	ui.Info("Running Full Profile Preflight Checks...")
	
	// 1. Platform compatibility & validity
	var supported, unsupported []*registry.ToolEntry
	for _, tool := range tools {
		if errs := registry.ValidateTool(tool); len(errs) > 0 {
			return fmt.Errorf("registry invalid for tool %s: %v", tool.ID, errs)
		}
		if tool.IsSupported(runtime.GOOS, runtime.GOARCH) {
			supported = append(supported, tool)
		} else {
			unsupported = append(unsupported, tool)
		}
	}

	ui.KeyValue("Compatible Tools:", fmt.Sprintf("%d", len(supported)), 25)
	ui.KeyValue("Unsupported Tools:", fmt.Sprintf("%d", len(unsupported)), 25)

	// 2. Host Prerequisites
	ui.Section("Host Prerequisites")
	allReqsMet := true
	for _, tool := range supported {
		if len(tool.HostPrerequisites) > 0 {
			fmt.Printf("  %s:\n", tool.DisplayName)
			results := preflight.CheckAll(tool)
			for _, res := range results {
				if res.Present {
					fmt.Printf("    %s %s: %s\n", ui.StyleSuccess.Render("✓"), res.Prerequisite.Name, res.Output)
				} else {
					status := ui.StyleError.Render("✗ Missing")
					if !res.Prerequisite.Mandatory {
						status = ui.StyleWarning.Render("! Missing (Optional)")
					} else {
						allReqsMet = false
					}
					fmt.Printf("    %s %s - %s\n", status, res.Prerequisite.Name, res.Prerequisite.Description)
				}
			}
		}
	}

	if !allReqsMet {
		ui.Warn("Some mandatory host prerequisites are missing. Installation of dependent tools may fail.")
	} else {
		ui.Success("All mandatory host prerequisites are met.")
	}

	// 3. Size Estimates (simplified for MVP)
	ui.Section("Storage Estimates")
	ui.KeyValue("Download Size:", "TBD", 25) // Would parse sizes from yaml
	ui.KeyValue("Installed Size:", "TBD", 25)

	return nil
}
