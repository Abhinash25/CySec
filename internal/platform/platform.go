// Package platform provides operating system detection, capability checks,
// and platform-specific path resolution for CySec.env.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// OS represents a supported operating system.
type OS string

const (
	Windows OS = "windows"
	Linux   OS = "linux"
	Darwin  OS = "darwin"
)

// Arch represents a supported architecture.
type Arch string

const (
	AMD64 Arch = "amd64"
	ARM64 Arch = "arm64"
	I386  Arch = "386"
)

// ToolSupport describes a tool's compatibility on a platform.
type ToolSupport string

const (
	Supported    ToolSupport = "SUPPORTED"
	Limited      ToolSupport = "LIMITED"
	Experimental ToolSupport = "EXPERIMENTAL"
	Unsupported  ToolSupport = "UNSUPPORTED"
)

// Info holds detected platform information.
type Info struct {
	OS       OS
	Arch     Arch
	HomeDir  string
	DataDir  string // Base directory for CySec.env data
	ShimDir  string // Directory for command shims
	CacheDir string // Directory for downloads and temp files
	LogDir   string // Directory for log files
	DBDir    string // Directory for state database
}

// Detect returns the current platform information.
func Detect() (*Info, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to detect home directory: %w", err)
	}

	info := &Info{
		OS:      OS(runtime.GOOS),
		Arch:    Arch(runtime.GOARCH),
		HomeDir: homeDir,
	}

	info.DataDir = resolveDataDir(info.OS, homeDir)
	info.ShimDir = filepath.Join(info.DataDir, "shims")
	info.CacheDir = filepath.Join(info.DataDir, "cache")
	info.LogDir = filepath.Join(info.DataDir, "logs")
	info.DBDir = filepath.Join(info.DataDir, "db")

	return info, nil
}

// resolveDataDir returns the platform-specific CySec.env data directory.
func resolveDataDir(osType OS, homeDir string) string {
	if cysecHome := os.Getenv("CYSEC_HOME"); cysecHome != "" {
		return cysecHome
	}

	switch osType {
	case Windows:
		// Use %LOCALAPPDATA%\CySec if available, else fall back to home
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			return filepath.Join(localApp, "CySec")
		}
		return filepath.Join(homeDir, ".cysec")
	case Darwin:
		return filepath.Join(homeDir, "Library", "Application Support", "CySec")
	default: // Linux and other Unix
		// Respect XDG_DATA_HOME if set
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "cysec")
		}
		return filepath.Join(homeDir, ".cysec")
	}
}

// ToolsDir returns the directory where managed tools are installed.
func (i *Info) ToolsDir() string {
	return filepath.Join(i.DataDir, "tools")
}

// RuntimesDir returns the directory for shared runtimes.
func (i *Info) RuntimesDir() string {
	return filepath.Join(i.DataDir, "runtimes")
}

// RegistryDir returns the directory for registry data.
func (i *Info) RegistryDir() string {
	return filepath.Join(i.DataDir, "registry")
}

// ConfigDir returns the directory for configuration files.
func (i *Info) ConfigDir() string {
	return filepath.Join(i.DataDir, "config")
}

// WorkspaceDir returns the protected user workspace directory.
func (i *Info) WorkspaceDir() string {
	return filepath.Join(i.DataDir, "workspace")
}

// BuildsDir returns the directory for Go build artifacts.
func (i *Info) BuildsDir() string {
	return filepath.Join(i.DataDir, "builds")
}

// TempDir returns the directory for temporary extraction space.
func (i *Info) TempDir() string {
	return filepath.Join(i.DataDir, "temp")
}

// DependenciesDir returns the directory for shared dependencies.
func (i *Info) DependenciesDir() string {
	return filepath.Join(i.DataDir, "dependencies")
}

// BinDir returns the directory for the cysec binary.
func (i *Info) BinDir() string {
	return filepath.Join(i.DataDir, "bin")
}

// EnsureDirectories creates all required CySec.env directories.
func (i *Info) EnsureDirectories() error {
	dirs := []string{
		i.DataDir,
		i.BinDir(),
		i.ToolsDir(),
		i.RuntimesDir(),
		i.DependenciesDir(),
		i.RegistryDir(),
		filepath.Join(i.RegistryDir(), "tools"),
		i.ShimDir,
		i.CacheDir,
		filepath.Join(i.CacheDir, "downloads"),
		filepath.Join(i.CacheDir, "tmp"),
		i.LogDir,
		i.DBDir,
		i.ConfigDir(),
		i.WorkspaceDir(),
		i.BuildsDir(),
		i.TempDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// ShimExtension returns the file extension for shims on this platform.
func (i *Info) ShimExtension() string {
	if i.OS == Windows {
		return ".cmd"
	}
	return ""
}

// Shell returns the default shell for the current platform.
func (i *Info) Shell() string {
	switch i.OS {
	case Windows:
		if ps, err := exec.LookPath("pwsh"); err == nil {
			return ps
		}
		return "powershell.exe"
	default:
		if shell := os.Getenv("SHELL"); shell != "" {
			return shell
		}
		return "/bin/sh"
	}
}

// HasCommand checks whether a command is available on the system PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// MatchesPlatform checks if a platform string (e.g., "windows/amd64") matches this info.
func (i *Info) MatchesPlatform(platform string) bool {
	parts := strings.SplitN(platform, "/", 2)
	if len(parts) == 1 {
		return string(i.OS) == parts[0]
	}
	return string(i.OS) == parts[0] && string(i.Arch) == parts[1]
}

// String returns a human-readable platform description.
func (i *Info) String() string {
	return fmt.Sprintf("%s/%s", i.OS, i.Arch)
}
