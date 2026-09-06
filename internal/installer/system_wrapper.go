package installer

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/cysec-env/cysec/internal/registry"
)

// SystemWrapperAdapter handles tools that must be installed via OS package managers or official installers.
type SystemWrapperAdapter struct{}

func (a *SystemWrapperAdapter) Name() string { return "system_wrapper" }

func (a *SystemWrapperAdapter) CanHandle(entry *registry.ToolEntry) bool {
	return entry.InstallerAdapter == "system_wrapper"
}

func (a *SystemWrapperAdapter) Plan(entry *registry.ToolEntry, installDir string) (*InstallPlan, error) {
	return &InstallPlan{
		Steps: []string{
			"Search system for existing installation",
			"Link to system binary",
			"Run health check",
		},
	}, nil
}

func (a *SystemWrapperAdapter) Install(entry *registry.ToolEntry, installDir string, downloadDir string) (*InstallResult, error) {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}

	// Try to find it in PATH
	path, err := exec.LookPath(cmdName)
	if err != nil {
		// Specific OS lookup fallback logic could go here
		msg := fmt.Sprintf("%s not found on system. Please install it using the official method:\n%s", entry.DisplayName, entry.OfficialWebsite)
		if runtime.GOOS == "windows" {
			msg += "\nEnsure it is added to your system PATH."
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return &InstallResult{
		Success:             true,
		EntryPoint:          path,
		Version:             "system",
		InstalledSize:       0, // We don't track size of system apps
		Ownership:           "SYSTEM_MANAGED",
	}, nil
}

func (a *SystemWrapperAdapter) Uninstall(entry *registry.ToolEntry, installDir string) error {
	// cysec uninstall must never remove a system-managed application
	return fmt.Errorf("cannot uninstall %s: it is a SYSTEM_MANAGED application", entry.DisplayName)
}

func (a *SystemWrapperAdapter) HealthCheck(entry *registry.ToolEntry, installDir string) error {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}
	_, err := exec.LookPath(cmdName)
	if err != nil {
		return fmt.Errorf("system binary not found: %w", err)
	}
	return nil
}

func (a *SystemWrapperAdapter) EntryPoint(entry *registry.ToolEntry, installDir string) (string, error) {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return "", fmt.Errorf("system binary not found in PATH: %w", err)
	}
	return path, nil
}
