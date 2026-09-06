// Package environment manages the CySec.env managed shell — spawning a
// sub-shell with the shim directory prepended to PATH.
package environment

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cysec-env/cysec/internal/platform"
)

// Manager handles the managed environment lifecycle.
type Manager struct {
	info    *platform.Info
	shimDir string
}

// New creates a new environment manager.
func New(info *platform.Info) *Manager {
	return &Manager{
		info:    info,
		shimDir: info.ShimDir,
	}
}

// Enter spawns a managed sub-shell with the CySec.env PATH prepended.
func (m *Manager) Enter() error {
	shell := m.info.Shell()

	// Build modified PATH
	currentPath := os.Getenv("PATH")
	newPath := m.shimDir + string(os.PathListSeparator) + currentPath

	// Set environment variables
	env := os.Environ()
	env = setEnv(env, "PATH", newPath)
	env = setEnv(env, "CYSEC_ENV", "1")
	env = setEnv(env, "CYSEC_HOME", m.info.DataDir)

	// Set prompt based on shell type
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "windows" && (strings.Contains(shell, "pwsh") || strings.Contains(shell, "powershell")):
		// For PowerShell, use -NoExit with a prompt function
		promptScript := fmt.Sprintf(
			`function prompt { Write-Host "cysec" -ForegroundColor Cyan -NoNewline; Write-Host "@" -NoNewline; Write-Host "env" -ForegroundColor Green -NoNewline; Write-Host ":" -NoNewline; Write-Host (Split-Path (Get-Location) -Leaf) -ForegroundColor Blue -NoNewline; return "$ " }; Write-Host ""; Write-Host "  CySec.env active. Type 'exit' to leave." -ForegroundColor DarkGray; Write-Host ""`,
		)
		cmd = exec.Command(shell, "-NoExit", "-Command", promptScript)
	case runtime.GOOS == "windows":
		// cmd.exe
		prompt := "cysec@env:$P$G"
		env = setEnv(env, "PROMPT", prompt)
		cmd = exec.Command(shell)
	default:
		// Unix shells — set PS1
		env = setEnv(env, "PS1", `\[\033[36m\]cysec\[\033[0m\]@\[\033[32m\]env\[\033[0m\]:\[\033[34m\]\W\[\033[0m\]\$ `)
		cmd = exec.Command(shell)
	}

	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = m.info.WorkspaceDir()

	return cmd.Run()
}

// IsActive returns true if we're already inside a CySec.env managed shell.
func IsActive() bool {
	return os.Getenv("CYSEC_ENV") == "1"
}

// WorkspaceDir returns the workspace directory, ensuring it exists.
func (m *Manager) WorkspaceDir() string {
	dir := m.info.WorkspaceDir()
	os.MkdirAll(dir, 0755)
	return dir
}

// ShimDir returns the shim directory path.
func (m *Manager) ShimDir() string {
	return m.shimDir
}

// setEnv replaces or appends an environment variable in a slice.
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), strings.ToUpper(prefix)) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// EnvPath returns the modified PATH string with shims prepended.
func (m *Manager) EnvPath() string {
	return m.shimDir + string(filepath.ListSeparator) + os.Getenv("PATH")
}
