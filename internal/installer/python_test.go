package installer

import (
	"path/filepath"
	"testing"

	"github.com/cysec-env/cysec/internal/registry"
)

func TestPythonAdapter_Install(t *testing.T) {
	// Only run if python/pip is available
	if findPython() == "" {
		t.Skip("Python is not available, skipping test")
	}

	adapter := &PythonAdapter{}
	entry := &registry.ToolEntry{
		ID: "invalid-pip-package-that-does-not-exist",
		AdapterConfig: map[string]string{
			"pip_package": "invalid-pip-package-that-does-not-exist-99999",
		},
	}

	tempDir := t.TempDir()
	_, err := adapter.Install(entry, filepath.Join(tempDir, "install"), filepath.Join(tempDir, "download"))
	if err == nil {
		t.Errorf("expected installation to fail for invalid pip package")
	}
}
