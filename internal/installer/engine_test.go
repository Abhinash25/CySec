package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cysec-env/cysec/internal/database"
	"github.com/cysec-env/cysec/internal/logger"
	"github.com/cysec-env/cysec/internal/registry"
	"github.com/cysec-env/cysec/internal/shim"
)

// MockAdapter is a simple adapter for testing engine orchestration
type MockAdapter struct {
	shouldFail bool
}

func (m *MockAdapter) Name() string { return "mock" }
func (m *MockAdapter) CanHandle(entry *registry.ToolEntry) bool {
	return entry.InstallerAdapter == "mock"
}
func (m *MockAdapter) Plan(entry *registry.ToolEntry, installDir string) (*InstallPlan, error) {
	return &InstallPlan{Steps: []string{"test"}}, nil
}
func (m *MockAdapter) Install(entry *registry.ToolEntry, installDir, downloadDir string) (*InstallResult, error) {
	if m.shouldFail {
		return nil, os.ErrPermission
	}
	_ = os.MkdirAll(installDir, 0755)
	_ = os.WriteFile(filepath.Join(installDir, "mock.bin"), []byte("data"), 0755)
	return &InstallResult{Success: true, EntryPoint: filepath.Join(installDir, "mock.bin"), Version: "1.0"}, nil
}
func (m *MockAdapter) Uninstall(entry *registry.ToolEntry, installDir string) error {
	return os.RemoveAll(installDir)
}
func (m *MockAdapter) HealthCheck(entry *registry.ToolEntry, installDir string) error {
	return nil
}
func (m *MockAdapter) EntryPoint(entry *registry.ToolEntry, installDir string) (string, error) {
	return filepath.Join(installDir, "mock.bin"), nil
}

func TestEngineRollback(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "engine-test-*")
	defer os.RemoveAll(tempDir) // Ignore errors on Windows due to SQLite file locks

	toolsDir := filepath.Join(tempDir, "tools")
	downloadDir := filepath.Join(tempDir, "downloads")
	shimDir := filepath.Join(tempDir, "shims")
	dbDir := filepath.Join(tempDir, "db")
	log := logger.New(logger.LevelDebug, false)

	db, err := database.Open(dbDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	shims := shim.NewManager(shimDir)
	engine := NewEngine(toolsDir, downloadDir, log)
	
	// Inject our mock adapter
	mockAdapter := &MockAdapter{shouldFail: true}
	engine.adapters = append(engine.adapters, mockAdapter)

	entry := &registry.ToolEntry{
		ID:               "mock-tool",
		DisplayName:      "Mock Tool",
		InstallerAdapter: "mock",
		Version:          "1.0",
	}

	// This should fail and trigger rollback
	_, err = engine.Install(entry, db, shims)
	if err == nil {
		t.Fatalf("expected installation to fail")
	}

	// Check that install and download dirs are cleaned up
	if _, err := os.Stat(filepath.Join(toolsDir, "mock-tool")); !os.IsNotExist(err) {
		t.Errorf("install directory was not cleaned up")
	}
	if _, err := os.Stat(filepath.Join(downloadDir, "mock-tool")); !os.IsNotExist(err) {
		t.Errorf("download directory was not cleaned up")
	}

	// Check database state is ROLLED_BACK or FAILED
	tool, _ := db.Get("mock-tool")
	if tool != nil {
		if tool.Status != "ROLLED_BACK" && tool.Status != "FAILED" {
			t.Errorf("expected status ROLLED_BACK or FAILED, got %s", tool.Status)
		}
	}
	db.Close()
}

func TestRecoverStaleInstallations(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "engine-test-recover-*")
	defer os.RemoveAll(tempDir)

	toolsDir := filepath.Join(tempDir, "tools")
	downloadDir := filepath.Join(tempDir, "downloads")
	shimDir := filepath.Join(tempDir, "shims")
	dbDir := filepath.Join(tempDir, "db")
	log := logger.New(logger.LevelDebug, false)

	db, err := database.Open(dbDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	shims := shim.NewManager(shimDir)
	engine := NewEngine(toolsDir, downloadDir, log)
	reg := registry.New(t.TempDir()) // empty registry

	// Create a stale installation
	db.RecordInstall(database.InstalledTool{
		ID:     "stale-tool",
		Name:   "Stale Tool",
		Status: "INSTALLING",
	})

	// Create dummy files to ensure they get cleaned up
	os.MkdirAll(filepath.Join(toolsDir, "stale-tool"), 0755)
	os.MkdirAll(filepath.Join(downloadDir, "stale-tool"), 0755)

	// Recover
	engine.RecoverStaleInstallations(db, reg, shims)

	// Verify cleanup
	if _, err := os.Stat(filepath.Join(toolsDir, "stale-tool")); !os.IsNotExist(err) {
		t.Errorf("install directory was not cleaned up by recovery")
	}
	
	tool, _ := db.Get("stale-tool")
	if tool == nil || tool.Status != "ROLLED_BACK" {
		t.Errorf("expected status ROLLED_BACK after recovery, got %v", tool)
	}
	db.Close()
}
