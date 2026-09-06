package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipSlip(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "malicious.zip")

	// Create a malicious zip file with path traversal
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	
	// Create a file trying to write outside the extraction dir
	w, err := zw.Create("../escaped.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	w.Write([]byte("malicious content"))
	zw.Close()
	f.Close()

	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0755)

	err = extractZip(zipPath, destDir)
	if err == nil {
		t.Errorf("expected extractZip to fail on path traversal attempt (zip slip)")
	}
}

func TestTarPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "malicious.tar.gz")

	// Create a malicious tar.gz with path traversal
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("failed to create tar: %v", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "../escaped.txt",
		Mode: 0600,
		Size: 4,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	tw.Write([]byte("test"))
	tw.Close()
	gw.Close()
	f.Close()

	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0755)

	err = extractTarGz(tarPath, destDir)
	if err == nil {
		t.Errorf("expected extractTarGz to fail on path traversal attempt")
	}
}
