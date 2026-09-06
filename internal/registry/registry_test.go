package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_LoadResilience(t *testing.T) {
	tempDir := t.TempDir()
	toolsDir := filepath.Join(tempDir, "tools")
	os.MkdirAll(toolsDir, 0755)

	// Valid YAML
	os.WriteFile(filepath.Join(toolsDir, "valid1.yaml"), []byte(`
id: valid1
name: Valid 1
installation_method: BINARY
installer_adapter: binary
aliases: ["v1"]
`), 0644)

	// Invalid YAML syntax
	os.WriteFile(filepath.Join(toolsDir, "invalid_syntax.yaml"), []byte(`
id: invalid
name: [ this is broken YAML
`), 0644)

	// Missing installation_method
	os.WriteFile(filepath.Join(toolsDir, "missing_method.yaml"), []byte(`
id: missing_method
name: Missing
`), 0644)

	// Duplicate ID
	os.WriteFile(filepath.Join(toolsDir, "z_duplicate.yaml"), []byte(`
id: valid1
name: Duplicate 1
installation_method: BINARY
installer_adapter: binary
`), 0644)

	// Valid YAML with duplicate alias
	os.WriteFile(filepath.Join(toolsDir, "valid2.yaml"), []byte(`
id: valid2
name: Valid 2
installation_method: BINARY
installer_adapter: binary
aliases: ["v1", "v2"]
`), 0644)

	reg := New(tempDir)
	err := reg.LoadFromDir(tempDir)
	if err != nil {
		t.Fatalf("LoadFromDir should not fail on invalid files: %v", err)
	}

	if reg.Count() != 2 {
		t.Errorf("Expected exactly 2 tools loaded, got %d", reg.Count())
	}

	if reg.Lookup("valid1") == nil {
		t.Errorf("Expected valid1 to be loaded")
	}

	if reg.Lookup("valid2") == nil {
		t.Errorf("Expected valid2 to be loaded")
	}

	// Verify alias "v1" still points to valid1 (the first one that registered it)
	if reg.Lookup("v1") == nil || reg.Lookup("v1").ID != "valid1" {
		t.Errorf("Expected alias v1 to point to valid1")
	}

	// Verify alias "v2" points to valid2
	if reg.Lookup("v2") == nil || reg.Lookup("v2").ID != "valid2" {
		t.Errorf("Expected alias v2 to point to valid2")
	}
}
