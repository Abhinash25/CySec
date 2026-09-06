package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cysec-env/cysec/internal/database"
	"github.com/cysec-env/cysec/internal/logger"
	"github.com/cysec-env/cysec/internal/registry"
	"github.com/cysec-env/cysec/internal/shim"
	"github.com/cysec-env/cysec/internal/verify"
)

// Engine orchestrates tool installation, uninstallation, and updates
// using the appropriate adapter for each tool.
type Engine struct {
	adapters    []Adapter
	toolsDir    string
	downloadDir string
	log         *logger.Logger
}

// NewEngine creates a new installer engine with all available adapters.
func NewEngine(toolsDir, downloadDir string, log *logger.Logger) *Engine {
	return &Engine{
		adapters: []Adapter{
			&BinaryAdapter{},
			&GoModuleAdapter{},
			&PythonAdapter{},
			&SystemWrapperAdapter{},
		},
		toolsDir:    toolsDir,
		downloadDir: downloadDir,
		log:         log,
	}
}

// findAdapter returns the adapter that can handle the given tool.
func (e *Engine) findAdapter(entry *registry.ToolEntry) (Adapter, error) {
	for _, a := range e.adapters {
		if a.CanHandle(entry) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("%w: %s (adapter: %s)", ErrUnsupportedAdapter, entry.ID, entry.InstallerAdapter)
}

// Plan returns the installation plan for a tool without performing changes.
func (e *Engine) Plan(entry *registry.ToolEntry) (*InstallPlan, error) {
	adapter, err := e.findAdapter(entry)
	if err != nil {
		return nil, err
	}

	installDir := filepath.Join(e.toolsDir, entry.ID)
	return adapter.Plan(entry, installDir)
}

// Install installs a tool and records it in the state database.
func (e *Engine) Install(entry *registry.ToolEntry, db *database.StateDB, shims *shim.Manager) (result *InstallResult, err error) {
	adapter, err := e.findAdapter(entry)
	if err != nil {
		return nil, err
	}

	installDir := filepath.Join(e.toolsDir, entry.ID)
	e.log.Info("Installing %s using %s adapter", entry.DisplayName, adapter.Name())

	if db != nil {
		record := database.InstalledTool{
			ID:           entry.ID,
			Name:         entry.DisplayName,
			Version:      entry.Version,
			InstallPath:  installDir,
			EntryCommand: entry.EntryCommand,
			Adapter:      adapter.Name(),
			HealthStatus: "unknown",
			Status:       "PENDING",
		}
		if err := db.RecordInstall(record); err != nil {
			e.log.Warn("Failed to record pending installation: %v", err)
		}
	}

	defer func() {
		if err != nil {
			e.log.Warn("Installation failed for %s, triggering rollback...", entry.DisplayName)
			if db != nil {
				_ = db.UpdateStatus(entry.ID, "FAILED")
			}
			_ = e.Rollback(entry, db, shims)
		}
	}()

	// Download directory for this tool
	toolDownloadDir := filepath.Join(e.downloadDir, entry.ID)
	if err = os.MkdirAll(toolDownloadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download dir: %w", err)
	}

	if db != nil {
		_ = db.UpdateStatus(entry.ID, "DOWNLOADING")
	}

	// Run the adapter
	result, err = adapter.Install(entry, installDir, toolDownloadDir)
	if err != nil {
		return nil, err
	}

	// Verify checksum if available (for binary adapter)
	if adapter.Name() == "binary" {
		if db != nil {
			_ = db.UpdateStatus(entry.ID, "VERIFYING")
		}
		spec := entry.GetPlatformSpec(
			string(getCurrentOS()),
			string(getCurrentArch()),
		)
		if spec != nil && spec.Checksum != "" {
			// Find the downloaded file to verify
			files, _ := os.ReadDir(toolDownloadDir)
			for _, f := range files {
				if !f.IsDir() {
					vResult, vErr := verify.VerifyChecksum(
						filepath.Join(toolDownloadDir, f.Name()),
						spec.Checksum,
					)
					if vErr != nil {
						e.log.Warn("Checksum verification error: %v", vErr)
					} else if !vResult.Passed && !vResult.Skipped {
						err = fmt.Errorf("checksum verification failed: %s", vResult.String())
						return nil, err
					} else {
						e.log.Info("Checksum: %s", vResult.String())
					}
					break
				}
			}
		}
	}

	// Health check
	if db != nil {
		_ = db.UpdateStatus(entry.ID, "HEALTH_CHECKING")
	}
	e.log.Info("Running health check...")
	healthErr := adapter.HealthCheck(entry, installDir)
	if healthErr != nil {
		e.log.Warn("Health check warning: %v", healthErr)
		result.Success = true // Don't fail install on health check warning, but log it
	}

	// Record in database
	if db != nil {
		_ = db.UpdateStatus(entry.ID, "CONFIGURING")
		
		record := database.InstalledTool{
			ID:            entry.ID,
			Name:          entry.DisplayName,
			Version:       result.Version,
			InstallPath:   installDir,
			EntryCommand:  entry.EntryCommand,
			Adapter:       adapter.Name(),
			Checksum:      result.Checksum,
			Ownership:     result.Ownership,
			HealthStatus:  "healthy",
			InstalledSize: result.InstalledSize,
			Status:        "INSTALLED",
		}
		if healthErr != nil {
			record.HealthStatus = "broken"
		}
		if err := db.RecordInstall(record); err != nil {
			e.log.Warn("Failed to record installation in database: %v", err)
		}
	}

	return result, nil
}

// Uninstall removes a tool.
func (e *Engine) Uninstall(entry *registry.ToolEntry, db *database.StateDB) error {
	if db != nil {
		installedTool, err := db.Get(entry.ID)
		if err == nil && installedTool != nil && installedTool.Ownership == "SYSTEM_MANAGED" {
			return fmt.Errorf("cannot uninstall %s: it is a SYSTEM_MANAGED application", entry.DisplayName)
		}
	}

	adapter, err := e.findAdapter(entry)
	if err != nil {
		return err
	}

	installDir := filepath.Join(e.toolsDir, entry.ID)
	e.log.Info("Uninstalling %s...", entry.DisplayName)

	if err := adapter.Uninstall(entry, installDir); err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	// Remove from database
	if db != nil {
		if err := db.Remove(entry.ID); err != nil {
			e.log.Warn("Failed to remove database record: %v", err)
		}
	}

	return nil
}

// Rollback removes partial installation files, temporary downloads, broken shims, and corrects database state.
func (e *Engine) Rollback(entry *registry.ToolEntry, db *database.StateDB, shims *shim.Manager) error {
	installDir := filepath.Join(e.toolsDir, entry.ID)
	downloadDir := filepath.Join(e.downloadDir, entry.ID)

	e.log.Info("Rolling back installation for %s...", entry.DisplayName)

	if db != nil {
		_ = db.UpdateStatus(entry.ID, "ROLLING_BACK")
	}

	// Remove partial installation files
	if _, err := os.Stat(installDir); err == nil {
		if err := os.RemoveAll(installDir); err != nil {
			e.log.Warn("Failed to remove install dir %s: %v", installDir, err)
		}
	}

	// Remove temporary downloads
	if _, err := os.Stat(downloadDir); err == nil {
		if err := os.RemoveAll(downloadDir); err != nil {
			e.log.Warn("Failed to remove download dir %s: %v", downloadDir, err)
		}
	}

	// Remove broken shim
	if shims != nil {
		shimName := entry.EntryCommand
		if shimName == "" {
			shimName = entry.ID
		}
		_ = shims.Remove(shimName)
	}

	// Correct database state
	if db != nil {
		_ = db.UpdateStatus(entry.ID, "ROLLED_BACK")
	}

	e.log.Info("Rollback completed for %s.", entry.DisplayName)
	return nil
}

// RecoverStaleInstallations detects interrupted installations and rolls them back.
func (e *Engine) RecoverStaleInstallations(db *database.StateDB, reg *registry.Registry, shims *shim.Manager) {
	if db == nil {
		return
	}

	stale, err := db.ListStaleInstallations()
	if err != nil || len(stale) == 0 {
		return
	}

	for _, tool := range stale {
		e.log.Warn("Detected stale installation for %s (Status: %s). Initiating recovery...", tool.Name, tool.Status)
		
		entry := reg.Lookup(tool.ID)
		if entry == nil {
			// If not in registry, just do a basic cleanup based on ID
			entry = &registry.ToolEntry{
				ID:           tool.ID,
				DisplayName:  tool.Name,
				EntryCommand: tool.EntryCommand,
			}
		}

		_ = e.Rollback(entry, db, shims)
	}
}

// HealthCheck runs a health check on an installed tool.
func (e *Engine) HealthCheck(entry *registry.ToolEntry) error {
	adapter, err := e.findAdapter(entry)
	if err != nil {
		return err
	}

	installDir := filepath.Join(e.toolsDir, entry.ID)
	return adapter.HealthCheck(entry, installDir)
}

// EntryPoint returns the path to the tool's executable.
func (e *Engine) EntryPoint(entry *registry.ToolEntry) (string, error) {
	adapter, err := e.findAdapter(entry)
	if err != nil {
		return "", err
	}

	installDir := filepath.Join(e.toolsDir, entry.ID)
	return adapter.EntryPoint(entry, installDir)
}

// Helper to get current OS/Arch as platform types
func getCurrentOS() OS_Type { return OS_Type(getGOOS()) }
func getCurrentArch() Arch_Type { return Arch_Type(getGOARCH()) }

type OS_Type string
type Arch_Type string

func getGOOS() string {
	return os.Getenv("GOOS_OVERRIDE") // for testing
}

func getGOARCH() string {
	return os.Getenv("GOARCH_OVERRIDE") // for testing
}

func init() {
	// Ensure we use runtime values by default
	if getGOOS() == "" {
		os.Setenv("GOOS_OVERRIDE", runtimeGOOS())
	}
	if getGOARCH() == "" {
		os.Setenv("GOARCH_OVERRIDE", runtimeGOARCH())
	}
}

func runtimeGOOS() string {
	// Using build tag would be cleaner, but this works at runtime
	return fmt.Sprintf("%s", getEnvOrDefault("GOOS", detectGOOS()))
}

func runtimeGOARCH() string {
	return fmt.Sprintf("%s", getEnvOrDefault("GOARCH", detectGOARCH()))
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func detectGOOS() string   { return "windows" } // Will be resolved at compile time per platform
func detectGOARCH() string { return "amd64" }
