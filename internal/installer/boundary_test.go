package installer

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createMaliciousZip(t *testing.T, targetPath string) string {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	
	f, err := w.Create(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte("malicious content"))
	if err != nil {
		t.Fatal(err)
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "malicious.zip")
	err = os.WriteFile(zipPath, buf.Bytes(), 0644)
	if err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestZipSlipPrevention(t *testing.T) {
	destDir := t.TempDir()
	
	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\Windows\\System32\\cmd.exe",
		"C:\\Windows\\System32\\cmd.exe",
		"../" + filepath.Base(destDir) + "-evil/payload.txt", // Sibling directory escape
		"..\\" + filepath.Base(destDir) + "-evil\\payload.exe",
	}

	for _, p := range maliciousPaths {
		t.Run(p, func(t *testing.T) {
			zipPath := createMaliciousZip(t, p)
			err := extractZip(zipPath, destDir)
			if err == nil {
				t.Errorf("Expected extractZip to fail for malicious path %q, but it succeeded", p)
			}
		})
	}

	// Paths that look absolute but are safely contained by filepath.Join
	safePaths := []string{
		"/absolute/path/file.txt",
		"\\\\server\\share\\file.txt",
	}
	for _, p := range safePaths {
		t.Run(p, func(t *testing.T) {
			zipPath := createMaliciousZip(t, p)
			err := extractZip(zipPath, destDir)
			if err != nil {
				t.Errorf("Expected extractZip to safely contain absolute path %q, but it failed: %v", p, err)
			}
		})
	}
}

func TestZipValidPath(t *testing.T) {
	destDir := t.TempDir()
	
	validPaths := []string{
		"bin/binary.exe",
		"folder/file.txt",
		"file.txt",
	}

	for _, p := range validPaths {
		t.Run(p, func(t *testing.T) {
			zipPath := createMaliciousZip(t, p)
			err := extractZip(zipPath, destDir)
			if err != nil {
				t.Errorf("Expected extractZip to succeed for valid path %q, but it failed: %v", p, err)
			}
			
			// Verify file exists
			if _, err := os.Stat(filepath.Join(destDir, p)); os.IsNotExist(err) {
				t.Errorf("Expected file %q to be extracted, but it was not found", p)
			}
		})
	}
}

// 7z boundary tests mapping to the required cases using our abstract boundary check.

func TestSevenZipTraversal(t *testing.T) {
	_, err := checkArchiveBoundary("/dest", "../../../etc/passwd")
	if err == nil {
		t.Error("Expected failure for traversal")
	}
}

func TestSevenZipAbsolutePath(t *testing.T) {
	_, err := checkArchiveBoundary("/dest", "/absolute/path/file.txt")
	if err != nil {
		t.Errorf("Expected success for absolute path, got: %v", err)
	}
}

func TestSevenZipWindowsDrivePath(t *testing.T) {
	target, err := checkArchiveBoundary("/dest", "C:\\Windows\\System32\\cmd.exe")
	if err == nil {
		if !strings.HasPrefix(filepath.ToSlash(target), "/dest/") {
			t.Errorf("Path %q was not contained in /dest", target)
		}
	}
}

func TestSevenZipUNCPath(t *testing.T) {
	_, err := checkArchiveBoundary("/dest", "\\\\server\\share\\file.txt")
	if err != nil {
		t.Errorf("Expected success (safely stripped to dest dir) for UNC path, got: %v", err)
	}
}

func TestSevenZipValidArchive(t *testing.T) {
	p, err := checkArchiveBoundary("/dest", "bin/binary.exe")
	if err != nil {
		t.Errorf("Expected success for valid archive path, got: %v", err)
	}
	expected := filepath.Join("/dest", "bin/binary.exe")
	if p != expected {
		t.Errorf("Expected %q, got %q", expected, p)
	}
}
