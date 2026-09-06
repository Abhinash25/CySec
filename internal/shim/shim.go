// Package shim manages command shim generation for direct tool execution.
// On Windows, .cmd files are generated. On Unix, symlinks or shell scripts
// are used.
package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Manager handles shim creation and removal.
type Manager struct {
	shimDir string
}

// NewManager creates a new shim manager.
func NewManager(shimDir string) *Manager {
	return &Manager{shimDir: shimDir}
}

// Create creates a command shim that points to the tool's entry point.
func (m *Manager) Create(name, entryPoint string, interfaceType string) (string, error) {
	if err := os.MkdirAll(m.shimDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create shim directory: %w", err)
	}

	if runtime.GOOS == "windows" {
		return m.createWindowsShim(name, entryPoint, interfaceType)
	}
	return m.createUnixShim(name, entryPoint, interfaceType)
}

// createWindowsShim creates a .cmd wrapper on Windows.
func (m *Manager) createWindowsShim(name, entryPoint string, interfaceType string) (string, error) {
	shimPath := filepath.Join(m.shimDir, name+".cmd")
	var content, ps1Content string

	cmdPrefix := ""
	if strings.HasSuffix(strings.ToLower(entryPoint), ".jar") {
		cmdPrefix = "java -jar "
	}

	if interfaceType == "gui" {
		content = fmt.Sprintf("@echo off\r\nstart \"\" /b %s\"%s\" %%*\r\n", cmdPrefix, entryPoint)
		ps1Content = fmt.Sprintf("Start-Process -FilePath 'java' -ArgumentList '-jar', '%s', $args -NoNewWindow\n", entryPoint)
	} else {
		content = fmt.Sprintf("@echo off\r\n%s\"%s\" %%*\r\n", cmdPrefix, entryPoint)
		if cmdPrefix != "" {
			ps1Content = fmt.Sprintf("& java -jar '%s' @args\n", entryPoint)
		} else {
			ps1Content = fmt.Sprintf("& '%s' @args\n", entryPoint)
		}
	}

	if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("failed to create shim: %w", err)
	}

	// Also create a .ps1 shim for PowerShell
	ps1Path := filepath.Join(m.shimDir, name+".ps1")
	_ = os.WriteFile(ps1Path, []byte(ps1Content), 0755)

	return shimPath, nil
}

// createUnixShim creates a symlink or shell script on Unix.
func (m *Manager) createUnixShim(name, entryPoint string, interfaceType string) (string, error) {
	shimPath := filepath.Join(m.shimDir, name)

	// Remove existing shim
	_ = os.Remove(shimPath)

	// Try symlink first if not GUI
	if interfaceType != "gui" {
		if err := os.Symlink(entryPoint, shimPath); err == nil {
			return shimPath, nil
		}
	}

	// Fall back to shell script (or required for GUI detach)
	var content string
	
	cmdPrefix := ""
	if strings.HasSuffix(strings.ToLower(entryPoint), ".jar") {
		cmdPrefix = "java -jar "
	}

	if interfaceType == "gui" {
		content = fmt.Sprintf("#!/bin/sh\nnohup %s\"%s\" \"$@\" >/dev/null 2>&1 &\n", cmdPrefix, entryPoint)
	} else {
		content = fmt.Sprintf("#!/bin/sh\nexec %s\"%s\" \"$@\"\n", cmdPrefix, entryPoint)
	}
	
	if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
		return "", fmt.Errorf("failed to create shim: %w", err)
	}

	return shimPath, nil
}

// Remove removes a command shim.
func (m *Manager) Remove(name string) error {
	if runtime.GOOS == "windows" {
		_ = os.Remove(filepath.Join(m.shimDir, name+".cmd"))
		_ = os.Remove(filepath.Join(m.shimDir, name+".ps1"))
	} else {
		_ = os.Remove(filepath.Join(m.shimDir, name))
	}
	return nil
}

// Exists checks if a shim exists for the given command name.
func (m *Manager) Exists(name string) bool {
	if runtime.GOOS == "windows" {
		_, err := os.Stat(filepath.Join(m.shimDir, name+".cmd"))
		return err == nil
	}
	_, err := os.Stat(filepath.Join(m.shimDir, name))
	return err == nil
}

// List returns all shim names.
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.shimDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	seen := make(map[string]bool)
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Strip extension for dedup
		base := name
		for _, ext := range []string{".cmd", ".ps1", ".exe"} {
			if filepath.Ext(name) == ext {
				base = name[:len(name)-len(ext)]
				break
			}
		}
		if !seen[base] {
			seen[base] = true
			names = append(names, base)
		}
	}
	return names, nil
}

// Dir returns the shim directory path.
func (m *Manager) Dir() string {
	return m.shimDir
}
