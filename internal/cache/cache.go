// Package cache manages the CySec.env download and temporary file cache.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

// Manager provides cache inspection and cleanup operations.
type Manager struct {
	cacheDir     string
	workspaceDir string // Protected — never cleaned
}

// NewManager creates a new cache manager.
func NewManager(cacheDir, workspaceDir string) *Manager {
	return &Manager{
		cacheDir:     cacheDir,
		workspaceDir: workspaceDir,
	}
}

// Status reports the current cache state.
type Status struct {
	DownloadSize int64
	TmpSize      int64
	TotalSize    int64
	Reclaimable  int64
}

// GetStatus returns the current cache status.
func (m *Manager) GetStatus() (*Status, error) {
	downloadSize := dirSizeBytes(filepath.Join(m.cacheDir, "downloads"))
	tmpSize := dirSizeBytes(filepath.Join(m.cacheDir, "tmp"))

	total := downloadSize + tmpSize

	return &Status{
		DownloadSize: downloadSize,
		TmpSize:      tmpSize,
		TotalSize:    total,
		Reclaimable:  total,
	}, nil
}

// ClearAll removes all cached files (downloads and tmp).
func (m *Manager) ClearAll() (int64, error) {
	status, err := m.GetStatus()
	if err != nil {
		return 0, err
	}

	freed := status.TotalSize

	if err := clearDir(filepath.Join(m.cacheDir, "downloads")); err != nil {
		return 0, fmt.Errorf("failed to clear downloads cache: %w", err)
	}
	if err := clearDir(filepath.Join(m.cacheDir, "tmp")); err != nil {
		return 0, fmt.Errorf("failed to clear tmp cache: %w", err)
	}

	return freed, nil
}

// CleanVerifiedDownloads removes downloads for tools that are fully installed.
func (m *Manager) CleanVerifiedDownloads(installedTools []string) (int64, error) {
	var freed int64
	downloadsDir := filepath.Join(m.cacheDir, "downloads")
	
	for _, toolID := range installedTools {
		toolDownloadDir := filepath.Join(downloadsDir, toolID)
		if stat, err := os.Stat(toolDownloadDir); err == nil && stat.IsDir() {
			size := dirSizeBytes(toolDownloadDir)
			if err := os.RemoveAll(toolDownloadDir); err == nil {
				freed += size
			}
		}
	}
	return freed, nil
}

// CleanBuilds removes all cached build artifacts.
func (m *Manager) CleanBuilds() (int64, error) {
	buildsDir := filepath.Join(filepath.Dir(m.cacheDir), "builds") // assuming cacheDir is {DataDir}/cache
	size := dirSizeBytes(buildsDir)
	if err := clearDir(buildsDir); err != nil {
		return 0, fmt.Errorf("failed to clear builds directory: %w", err)
	}
	return size, nil
}

// CleanTemp removes all temporary extraction directories.
func (m *Manager) CleanTemp() (int64, error) {
	tempDir := filepath.Join(filepath.Dir(m.cacheDir), "temp") // assuming cacheDir is {DataDir}/cache
	size := dirSizeBytes(tempDir)
	if err := clearDir(tempDir); err != nil {
		return 0, fmt.Errorf("failed to clear temp directory: %w", err)
	}
	return size, nil
}

// ClearTool removes cached files for a specific tool.
func (m *Manager) ClearTool(toolID string) error {
	toolDownloadDir := filepath.Join(m.cacheDir, "downloads", toolID)
	if _, err := os.Stat(toolDownloadDir); err == nil {
		return os.RemoveAll(toolDownloadDir)
	}
	return nil
}

// Dir returns the cache directory path.
func (m *Manager) Dir() string {
	return m.cacheDir
}

// clearDir removes all contents of a directory without removing the directory itself.
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// dirSizeBytes calculates total size of a directory recursively.
func dirSizeBytes(dir string) int64 {
	var size int64
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
