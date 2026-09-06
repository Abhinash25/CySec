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

// PythonAdapter handles installation via pip/pipx.
type PythonAdapter struct{}

func (a *PythonAdapter) Name() string { return "python" }

func (a *PythonAdapter) CanHandle(entry *registry.ToolEntry) bool {
	return entry.InstallerAdapter == "python" || entry.InstallationMethod == "PYTHON_PACKAGE"
}

func (a *PythonAdapter) Plan(entry *registry.ToolEntry, installDir string) (*InstallPlan, error) {
	pipPkg := entry.AdapterConfig["pip_package"]
	if pipPkg == "" {
		pipPkg = entry.ID
	}

	return &InstallPlan{
		Steps: []string{
			fmt.Sprintf("pip install --target=%s %s", installDir, pipPkg),
			"Detect entry point",
			"Run health check",
		},
		Dependencies: []string{"python"},
	}, nil
}

func (a *PythonAdapter) Install(entry *registry.ToolEntry, installDir string, downloadDir string) (*InstallResult, error) {
	// Find Python
	pythonPath := findPython()
	if pythonPath == "" {
		return nil, fmt.Errorf("Python is not installed or not in PATH")
	}

	pipPkg := entry.AdapterConfig["pip_package"]
	if pipPkg == "" {
		pipPkg = entry.ID
	}

	version := entry.Version
	pkgSpec := pipPkg
	if version != "" && version != "latest" {
		pkgSpec = fmt.Sprintf("%s==%s", pipPkg, version)
	}

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create install dir: %w", err)
	}

	// Try pipx first (isolates tool environments), then fall back to pip --target
	if pipxPath, err := exec.LookPath("pipx"); err == nil {
		cmd := exec.Command(pipxPath, "install", pkgSpec, "--force")
		// Point PIPX_HOME and PIPX_BIN_DIR into our managed dirs
		cmd.Env = append(os.Environ(),
			"PIPX_HOME="+installDir,
			"PIPX_BIN_DIR="+filepath.Join(installDir, "bin"),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			ep, _ := a.EntryPoint(entry, installDir)
			return &InstallResult{
				Success:       true,
				EntryPoint:    ep,
				Version:       version,
				InstalledSize: dirSize(installDir),
			}, nil
		}
		// Fall through to pip if pipx fails
	}

	// pip install --target
	cmd := exec.Command(pythonPath, "-m", "pip", "install", "--target", installDir, pkgSpec, "--no-warn-script-location")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, &ErrInstallFailed{
			Tool:    entry.ID,
			Adapter: "python",
			Cause:   err,
		}
	}

	originalEntryPoint, err := a.EntryPoint(entry, installDir)
	if err != nil {
		return nil, err
	}

	// For pip --target installs, ALWAYS create a wrapper script
	// because pip-generated .exe files do not have PYTHONPATH set to installDir.
	entryPoint, err := a.createWrapper(entry, installDir, originalEntryPoint)
	if err != nil {
		return nil, err
	}

	return &InstallResult{
		Success:       true,
		EntryPoint:    entryPoint,
		Version:       version,
		InstalledSize: dirSize(installDir),
	}, nil
}

func (a *PythonAdapter) Uninstall(entry *registry.ToolEntry, installDir string) error {
	return os.RemoveAll(installDir)
}

func (a *PythonAdapter) HealthCheck(entry *registry.ToolEntry, installDir string) error {
	entryPoint, err := a.EntryPoint(entry, installDir)
	if err != nil {
		return err
	}

	for _, flag := range []string{"--version", "-version", "--help", "-h"} {
		cmd := exec.Command(entryPoint, flag)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return nil
}

func (a *PythonAdapter) EntryPoint(entry *registry.ToolEntry, installDir string) (string, error) {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	candidates := []string{
		filepath.Join(installDir, "bin", cmdName+"_wrapper.cmd"),
		filepath.Join(installDir, "bin", cmdName+"_wrapper"),
		filepath.Join(installDir, "bin", cmdName+".cmd"),
		filepath.Join(installDir, "bin", cmdName),
		filepath.Join(installDir, "bin", cmdName+ext),
		filepath.Join(installDir, "Scripts", cmdName+".cmd"),
		filepath.Join(installDir, "Scripts", cmdName+ext),
		filepath.Join(installDir, "Scripts", cmdName),
		filepath.Join(installDir, cmdName),
		filepath.Join(installDir, cmdName+ext),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("could not find entry point for %s", cmdName)
}

// createWrapper creates a platform-specific wrapper script for Python tools
// installed with pip --target to ensure PYTHONPATH is set.
func (a *PythonAdapter) createWrapper(entry *registry.ToolEntry, installDir string, originalEntryPoint string) (string, error) {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}

	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		wrapperPath := filepath.Join(binDir, cmdName+"_wrapper.cmd")
		content := fmt.Sprintf("@echo off\r\nset PYTHONPATH=%s;%%PYTHONPATH%%\r\n\"%s\" %%*\r\n", installDir, originalEntryPoint)
		if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
			return "", err
		}
		return wrapperPath, nil
	}

	wrapperPath := filepath.Join(binDir, cmdName+"_wrapper")
	content := fmt.Sprintf("#!/bin/sh\nPYTHONPATH=\"%s:$PYTHONPATH\" exec \"%s\" \"$@\"\n", installDir, originalEntryPoint)
	if err := os.WriteFile(wrapperPath, []byte(content), 0755); err != nil {
		return "", err
	}
	return wrapperPath, nil
}

// findPython locates the Python interpreter.
func findPython() string {
	for _, name := range []string{"python3", "python", "python3.exe", "python.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// pythonVersion returns the installed Python version string.
func pythonVersion() string {
	p := findPython()
	if p == "" {
		return ""
	}
	out, err := exec.Command(p, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
