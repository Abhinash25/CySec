package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cysec-env/cysec/internal/registry"
)

// GoModuleAdapter handles installation via `go install`.
type GoModuleAdapter struct{}

func (a *GoModuleAdapter) Name() string { return "go_module" }

func (a *GoModuleAdapter) CanHandle(entry *registry.ToolEntry) bool {
	return entry.InstallerAdapter == "go_module" || entry.InstallationMethod == "GO_MODULE"
}

func (a *GoModuleAdapter) Plan(entry *registry.ToolEntry, installDir string) (*InstallPlan, error) {
	goModule := entry.AdapterConfig["go_module"]
	if goModule == "" {
		goModule = entry.OfficialRepository
	}

	return &InstallPlan{
		Steps: []string{
			fmt.Sprintf("go install %s@%s", goModule, entry.Version),
			"Move binary to managed directory",
			"Run health check",
		},
		Dependencies: []string{"go"},
	}, nil
}

func (a *GoModuleAdapter) Install(entry *registry.ToolEntry, installDir string, downloadDir string) (*InstallResult, error) {
	// Check Go is available
	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("Go is not installed or not in PATH: %w", err)
	}

	goModule := entry.AdapterConfig["go_module"]
	if goModule == "" {
		return nil, fmt.Errorf("go_module not specified in adapter_config")
	}

	version := entry.Version
	if version == "" || version == "latest" {
		version = "latest"
	}

	// Set GOBIN to our install directory
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create install dir: %w", err)
	}

	// Run go install
	cmd := exec.Command(goPath, "install", goModule+"@"+version)
	cmd.Env = append(os.Environ(), "GOBIN="+installDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, &ErrInstallFailed{
			Tool:    entry.ID,
			Adapter: "go_module",
			Cause:   err,
		}
	}

	// Find the built binary
	entryPoint, err := a.EntryPoint(entry, installDir)
	if err != nil {
		return nil, err
	}

	size := dirSize(installDir)

	return &InstallResult{
		Success:       true,
		EntryPoint:    entryPoint,
		Version:       version,
		InstalledSize: size,
	}, nil
}

func (a *GoModuleAdapter) Uninstall(entry *registry.ToolEntry, installDir string) error {
	return os.RemoveAll(installDir)
}

func (a *GoModuleAdapter) HealthCheck(entry *registry.ToolEntry, installDir string) error {
	entryPoint, err := a.EntryPoint(entry, installDir)
	if err != nil {
		return err
	}

	for _, flag := range []string{"--version", "-version", "version", "--help", "-h"} {
		cmd := exec.Command(entryPoint, flag)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	if _, err := os.Stat(entryPoint); err != nil {
		return fmt.Errorf("binary not found at %s", entryPoint)
	}

	return nil
}

func (a *GoModuleAdapter) EntryPoint(entry *registry.ToolEntry, installDir string) (string, error) {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	candidates := []string{
		filepath.Join(installDir, cmdName+ext),
	}

	// Also try the last segment of the go module path
	if module := entry.AdapterConfig["go_module"]; module != "" {
		parts := strings.Split(module, "/")
		binName := parts[len(parts)-1]
		// Handle paths like cmd/toolname
		if strings.Contains(module, "/cmd/") {
			idx := strings.LastIndex(module, "/cmd/")
			rest := module[idx+5:]
			if slash := strings.Index(rest, "/"); slash > 0 {
				binName = rest[:slash]
			} else {
				binName = rest
			}
		}
		candidates = append(candidates, filepath.Join(installDir, binName+ext))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	// Walk looking for any executable
	var found string
	_ = filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if runtime.GOOS == "windows" && strings.HasSuffix(path, ".exe") {
			found = path
			return filepath.SkipAll
		}
		if runtime.GOOS != "windows" && info.Mode()&0111 != 0 {
			found = path
			return filepath.SkipAll
		}
		return nil
	})

	if found != "" {
		return found, nil
	}

	return "", fmt.Errorf("could not find built binary for %s in %s", cmdName, installDir)
}
