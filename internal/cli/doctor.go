package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/platform"
	"github.com/cysec-env/cysec/internal/ui"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check CySec.env health",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				return err
			}
			defer app.DB.Close()

			ui.Section("CySec.env Health Report")
			fmt.Println()

			warnings := 0
			errors := 0

			// Core application
			checkResult("Core Application", true, "")

			// Platform
			checkResult("Platform", true, fmt.Sprintf("%s", app.Platform))

			// Registry
			regCount := app.Registry.Count()
			if regCount > 0 {
				checkResult("Registry", true, fmt.Sprintf("%d tools", regCount))
			} else {
				checkResult("Registry", false, "empty — run 'cysec registry update'")
				warnings++
			}

			// Database
			dbCount := app.DB.Count()
			checkResult("Database", true, fmt.Sprintf("%d installed", dbCount))

			// Check runtimes
			fmt.Println()
			fmt.Println(ui.StyleBold.Render("  Runtime Availability:"))

			runtimes := []struct {
				name string
				cmds []string
			}{
				{"Git", []string{"git"}},
				{"Python", []string{"python3", "python"}},
				{"Go", []string{"go"}},
				{"Node.js", []string{"node"}},
				{"Docker", []string{"docker"}},
			}

			for _, rt := range runtimes {
				found := false
				for _, cmd := range rt.cmds {
					if platform.HasCommand(cmd) {
						found = true
						break
					}
				}
				if found {
					checkResult(rt.name, true, "")
				} else {
					checkResult(rt.name, false, "not found")
					// Don't count missing optional runtimes as errors
				}
			}

			// Check installed tools
			installed, _ := app.DB.ListInstalled()
			if len(installed) > 0 {
				fmt.Println()
				fmt.Println(ui.StyleBold.Render("  Installed Tools:"))

				broken := 0
				for _, t := range installed {
					tool := app.Registry.Lookup(t.ID)
					if tool != nil {
						err := app.Engine.HealthCheck(tool)
						if err != nil {
							checkResult(t.Name, false, "health check failed")
							app.DB.UpdateHealth(t.ID, "broken")
							broken++
						} else {
							checkResult(t.Name, true, t.Version)
							app.DB.UpdateHealth(t.ID, "healthy")
						}
					} else {
						checkResult(t.Name, true, t.Version)
					}
				}

				if broken > 0 {
					errors += broken
				}
			}

			// Summary
			fmt.Println()
			if errors == 0 && warnings == 0 {
				ui.Success("All checks passed.")
			} else {
				if errors > 0 {
					ui.Error(fmt.Sprintf("%d errors found.", errors))
				}
				if warnings > 0 {
					ui.Warn(fmt.Sprintf("%d warnings.", warnings))
				}
			}
			fmt.Println()

			return nil
		},
	}
}

func checkResult(name string, ok bool, detail string) {
	if ok {
		if detail != "" {
			fmt.Printf("    %s %-24s %s\n", ui.StyleSuccess.Render(ui.SymbolCheck), name, ui.StyleDim.Render(detail))
		} else {
			fmt.Printf("    %s %s\n", ui.StyleSuccess.Render(ui.SymbolCheck), name)
		}
	} else {
		if detail != "" {
			fmt.Printf("    %s %-24s %s\n", ui.StyleError.Render(ui.SymbolCross), name, ui.StyleWarning.Render(detail))
		} else {
			fmt.Printf("    %s %s\n", ui.StyleError.Render(ui.SymbolCross), name)
		}
	}
}
