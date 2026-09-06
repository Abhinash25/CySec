package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cysec-env/cysec/internal/database"
	"github.com/cysec-env/cysec/internal/registry"
)

func TestResolver(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "resolver-test-*")
	defer os.RemoveAll(tempDir) // Ignore errors on Windows due to SQLite file locks

	dbDir := filepath.Join(tempDir, "db")
	db, err := database.Open(dbDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	regDir := filepath.Join(tempDir, "registry")
	os.MkdirAll(filepath.Join(regDir, "tools"), 0755)

	// Create test registry entries
	os.WriteFile(filepath.Join(regDir, "tools", "testtool.yaml"), []byte(`
id: testtool
name: Test Tool
installation_method: MOCK
installer_adapter: mock
platforms:
  - os: windows
    support: SUPPORTED
  - os: linux
    support: SUPPORTED
`), 0644)

	os.WriteFile(filepath.Join(regDir, "tools", "unsupportedtool.yaml"), []byte(`
id: unsupportedtool
name: Unsupported Tool
installation_method: MOCK
installer_adapter: mock
platforms:
  - os: linux
    support: SUPPORTED
`), 0644)

	os.WriteFile(filepath.Join(regDir, "tools", "deprecatedtool.yaml"), []byte(`
id: deprecatedtool
name: Deprecated Tool
installation_method: MOCK
installer_adapter: mock
deprecated: true
replacement: newtool
platforms:
  - os: windows
    support: SUPPORTED
`), 0644)

	reg := registry.New(regDir)
	reg.LoadFromDir(regDir)

	res := New(db, reg, "windows", "amd64")

	// 1. Built-in command
	result := res.Resolve("install")
	if result.Status != StatusBuiltin {
		t.Errorf("Expected install to be built-in, got %v", result.Status)
	}

	// 2. Registry tool not installed
	result = res.Resolve("testtool")
	if result.Status != StatusAvailable {
		t.Errorf("Expected testtool to be available, got %v", result.Status)
	}

	// 3. Unknown command
	result = res.Resolve("unknown-magic-command")
	if result.Status != StatusUnknown {
		t.Errorf("Expected unknown command to be StatusUnknown, got %v", result.Status)
	}

	// 4. Unsupported command
	result = res.Resolve("unsupportedtool")
	if result.Status != StatusUnsupported {
		t.Errorf("Expected unsupportedtool to be unsupported on windows, got %v", result.Status)
	}

	// 5. Deprecated command
	result = res.Resolve("deprecatedtool")
	if result.Status != StatusDeprecated {
		t.Errorf("Expected deprecatedtool to be deprecated, got %v", result.Status)
	}

	// 6. Installed command
	db.RecordInstall(database.InstalledTool{
		ID:     "testtool",
		Name:   "Test Tool",
		Status: "INSTALLED",
	})

	result = res.Resolve("testtool")
	if result.Status != StatusInstalled {
		t.Errorf("Expected testtool to be installed, got %v", result.Status)
	}
	if result.Tool == nil || result.InstalledTool == nil {
		t.Errorf("Expected Tool and InstalledTool to be populated")
	}
}
