package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cysec-env/cysec/internal/platform"
)

func TestEnvironment_PATHConstruction(t *testing.T) {
	tempDir := t.TempDir()
	info := &platform.Info{
		ShimDir: filepath.Join(tempDir, "shims"),
		DataDir: tempDir,
	}

	mgr := New(info)
	path := mgr.EnvPath()

	// ShimDir should be the very first entry in the constructed PATH
	expectedPrefix := info.ShimDir + string(os.PathListSeparator)
	if !strings.HasPrefix(path, expectedPrefix) {
		t.Errorf("Expected PATH to start with shim directory %q, got %q", expectedPrefix, path)
	}
}

func TestEnvironment_SetEnv(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/home/user", "USER=test"}

	// Update existing
	env = setEnv(env, "PATH", "/new/path")
	found := false
	for _, e := range env {
		if e == "PATH=/new/path" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("setEnv failed to update existing variable. Result: %v", env)
	}

	// Add new
	env = setEnv(env, "CYSEC_ENV", "1")
	found = false
	for _, e := range env {
		if e == "CYSEC_ENV=1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("setEnv failed to add new variable. Result: %v", env)
	}
}

func TestEnvironment_IsActive(t *testing.T) {
	os.Setenv("CYSEC_ENV", "1")
	defer os.Unsetenv("CYSEC_ENV")

	if !IsActive() {
		t.Errorf("Expected IsActive to be true when CYSEC_ENV=1 is set")
	}

	os.Setenv("CYSEC_ENV", "0")
	if IsActive() {
		t.Errorf("Expected IsActive to be false when CYSEC_ENV!=1")
	}
}
