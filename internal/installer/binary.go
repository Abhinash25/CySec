package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/cysec-env/cysec/internal/registry"
	"github.com/cysec-env/cysec/internal/verify"
)

// BinaryAdapter handles installation of pre-compiled binary releases.
type BinaryAdapter struct{}

func (a *BinaryAdapter) Name() string { return "binary" }

func (a *BinaryAdapter) CanHandle(entry *registry.ToolEntry) bool {
	return entry.InstallerAdapter == "binary" || entry.InstallationMethod == "OFFICIAL_BINARY"
}

func (a *BinaryAdapter) Plan(entry *registry.ToolEntry, installDir string) (*InstallPlan, error) {
	spec := entry.GetPlatformSpec(runtime.GOOS, runtime.GOARCH)
	if spec == nil {
		return nil, fmt.Errorf("no platform spec for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	downloadURLStr := spec.DownloadURL
	if downloadURLStr == "" {
		downloadURLStr = entry.OfficialReleaseSource
	}

	return &InstallPlan{
		Steps: []string{
			fmt.Sprintf("Download from %s", downloadURLStr),
			"Verify integrity",
			fmt.Sprintf("Extract to %s", installDir),
			"Detect entry point",
			"Run health check",
		},
		DownloadURL:  downloadURLStr,
		DownloadSize: entry.EstimatedDownloadSize,
	}, nil
}

func (a *BinaryAdapter) Install(entry *registry.ToolEntry, installDir string, downloadDir string) (*InstallResult, error) {
	spec := entry.GetPlatformSpec(runtime.GOOS, runtime.GOARCH)
	if spec == nil {
		return nil, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	downloadURLStr := spec.DownloadURL
	if downloadURLStr == "" {
		downloadURLStr = entry.OfficialReleaseSource
	}
	if downloadURLStr == "" {
		return nil, fmt.Errorf("no download URL for %s on %s/%s", entry.ID, runtime.GOOS, runtime.GOARCH)
	}

	// Create install directory
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create install dir: %w", err)
	}

	parsedURL, _ := url.Parse(downloadURLStr)
	fileName := filepath.Base(parsedURL.Path)
	if fileName == "/" || fileName == "." || fileName == "download" {
		if strings.Contains(downloadURLStr, "type=jar") {
			fileName = entry.ID + ".jar"
		} else {
			fileName = entry.ID + ".bin"
		}
	}
	downloadFile := filepath.Join(downloadDir, fileName)

	// Download
	if err := downloadURL(downloadURLStr, downloadFile); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Resolve Checksum
	checksum := spec.Checksum
	if checksum == "" {
		checksum = entry.Checksum
	}
	checksumURL := spec.ChecksumURL
	if checksumURL == "" {
		checksumURL = entry.ChecksumURL
	}
	algo := spec.ChecksumAlgorithm
	if algo == "" {
		algo = entry.ChecksumAlgorithm
	}

	if checksum == "" && checksumURL != "" {
		resolved, err := verify.FetchAndExtractChecksum(checksumURL, filepath.Base(downloadURLStr), algo)
		if err != nil {
			return nil, fmt.Errorf("checksum resolution failed for %s: %w", downloadURLStr, err)
		}
		checksum = resolved
	}

	// Verify Checksum before extraction
	if checksum != "" {
		res, err := verify.VerifyChecksum(downloadFile, checksum)
		if err != nil {
			return nil, fmt.Errorf("integrity verification error: %w", err)
		}
		if !res.Passed {
			return nil, fmt.Errorf("integrity verification failed for %s: expected %s, got %s", downloadURLStr, res.Expected, res.Actual)
		}
	}

	// Extract based on file extension
	if err := extractArchive(downloadFile, installDir); err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Find entry point
	entryPoint, err := a.EntryPoint(entry, installDir)
	if err != nil {
		return nil, fmt.Errorf("entry point detection failed: %w", err)
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		_ = os.Chmod(entryPoint, 0755)
	}

	// Get installed size
	size := dirSize(installDir)

	return &InstallResult{
		Success:       true,
		EntryPoint:    entryPoint,
		Version:       entry.Version,
		InstalledSize: size,
		Checksum:      checksum,
	}, nil
}

func (a *BinaryAdapter) Uninstall(entry *registry.ToolEntry, installDir string) error {
	return os.RemoveAll(installDir)
}

func (a *BinaryAdapter) HealthCheck(entry *registry.ToolEntry, installDir string) error {
	entryPoint, err := a.EntryPoint(entry, installDir)
	if err != nil {
		return err
	}

	// Try running with --version or -version
	for _, flag := range []string{"--version", "-version", "version", "--help"} {
		cmd := exec.Command(entryPoint, flag)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// If none of the flags work, at least check the binary exists and is executable
	if _, err := os.Stat(entryPoint); err != nil {
		return fmt.Errorf("binary not found at %s", entryPoint)
	}

	return nil
}

func (a *BinaryAdapter) EntryPoint(entry *registry.ToolEntry, installDir string) (string, error) {
	cmdName := entry.EntryCommand
	if cmdName == "" {
		cmdName = entry.ID
	}

	// Try common patterns
	candidates := []string{
		filepath.Join(installDir, cmdName),
		filepath.Join(installDir, cmdName+".exe"),
		filepath.Join(installDir, "bin", cmdName),
		filepath.Join(installDir, "bin", cmdName+".exe"),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	// Walk directory looking for an executable with the tool name
	var found string
	var fallback string
	_ = filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".exe")
		if strings.EqualFold(base, cmdName) {
			found = path
			return filepath.SkipAll
		}
		
		if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(path), ".exe") {
			if strings.HasPrefix(strings.ToLower(base), strings.ToLower(cmdName)) {
				fallback = path
			}
		}
		
		return nil
	})

	if found != "" {
		return found, nil
	}
	
	if fallback != "" {
		return fallback, nil
	}

	return "", fmt.Errorf("could not find entry point %q in %s", cmdName, installDir)
}

// downloadURL downloads a file from a URL to a local path.
func downloadURL(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractArchive extracts a .zip or .tar.gz archive to a directory.
func extractArchive(archivePath, destDir string) error {
	lower := strings.ToLower(archivePath)

	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".7z"):
		return extractSevenZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath, destDir)
	default:
		// Treat as a single binary
		base := filepath.Base(archivePath)
		destFile := filepath.Join(destDir, base)
		
		// If it's a standalone .exe on Windows and doesn't exactly match the tool name, we should probably rename it to ensure it's found.
		// A simpler approach is to rename it to the expected cmdName if it's an exe or raw binary.
		if runtime.GOOS == "windows" && strings.HasSuffix(lower, ".exe") {
			// Find the tool ID from the directory path (not strictly clean, but we can't easily pass cmdName here)
			// Wait, copyFile will just copy it. EntryPoint does a prefix match? No, it does EqualFold.
		}
		
		return copyFile(archivePath, destFile)
	}
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target, err := checkArchiveBoundary(dest, f.Name)
		if err != nil {
			return fmt.Errorf("illegal path in zip: %w", err)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := checkArchiveBoundary(dest, header.Name)
		if err != nil {
			return fmt.Errorf("illegal path in tar: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
			os.Chmod(target, os.FileMode(header.Mode))
		}
	}
	return nil
}

func extractSevenZip(src, dest string) error {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target, err := checkArchiveBoundary(dest, f.Name)
		if err != nil {
			return fmt.Errorf("illegal path in 7z: %w", err)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// checkArchiveBoundary validates that an extracted file path remains within the destination directory.
func checkArchiveBoundary(destDir, fileName string) (string, error) {
	target := filepath.Join(destDir, fileName)

	// Prevent path traversal via robust relative path calculation
	rel, err := filepath.Rel(destDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("escapes destination: %s", fileName)
	}
	return target, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
