// Package installer provides the modular installer engine and adapter system.
// Each installation method (binary download, go install, pip install, etc.)
// is implemented as an adapter conforming to the Adapter interface.
package installer

import (
	"fmt"

	"github.com/cysec-env/cysec/internal/registry"
)

// Adapter defines the interface that every installer adapter must implement.
type Adapter interface {
	// Name returns the adapter identifier (e.g., "binary", "go_module").
	Name() string

	// CanHandle returns true if this adapter can install the given tool entry.
	CanHandle(entry *registry.ToolEntry) bool

	// Plan returns a human-readable installation plan without performing changes.
	Plan(entry *registry.ToolEntry, installDir string) (*InstallPlan, error)

	// Install performs the actual installation.
	Install(entry *registry.ToolEntry, installDir string, downloadDir string) (*InstallResult, error)

	// Uninstall removes the tool. The installDir is the directory to clean up.
	Uninstall(entry *registry.ToolEntry, installDir string) error

	// HealthCheck verifies that the installed tool is functional.
	HealthCheck(entry *registry.ToolEntry, installDir string) error

	// EntryPoint returns the path to the executable entry point after installation.
	EntryPoint(entry *registry.ToolEntry, installDir string) (string, error)
}

// InstallPlan describes what an installation will do before it happens.
type InstallPlan struct {
	Steps        []string
	DownloadURL  string
	DownloadSize string
	Dependencies []string
}

// InstallResult describes the outcome of an installation.
type InstallResult struct {
	Success       bool
	EntryPoint    string // Absolute path to the executable
	Version       string
	InstalledSize int64
	Checksum      string
	Ownership     string // "CYSEC_MANAGED" or "SYSTEM_MANAGED"
}

// ErrUnsupportedAdapter is returned when no adapter can handle a tool.
var ErrUnsupportedAdapter = fmt.Errorf("no installer adapter supports this tool")

// ErrInstallFailed is returned when installation fails.
type ErrInstallFailed struct {
	Tool    string
	Adapter string
	Cause   error
}

func (e *ErrInstallFailed) Error() string {
	return fmt.Sprintf("installation of %s failed (adapter: %s): %v", e.Tool, e.Adapter, e.Cause)
}

func (e *ErrInstallFailed) Unwrap() error {
	return e.Cause
}
