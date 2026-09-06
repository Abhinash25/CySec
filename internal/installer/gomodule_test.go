package installer

import (
	"path/filepath"
	"testing"

	"github.com/cysec-env/cysec/internal/registry"
)

func TestGoModuleAdapter_MissingGoModule(t *testing.T) {
	adapter := &GoModuleAdapter{}
	entry := &registry.ToolEntry{
		ID:      "invalid-tool",
		Version: "latest",
		// AdapterConfig is empty
	}

	tempDir := t.TempDir()
	_, err := adapter.Install(entry, filepath.Join(tempDir, "install"), filepath.Join(tempDir, "download"))
	if err == nil {
		t.Errorf("expected installation to fail when go_module is missing")
	}
}
