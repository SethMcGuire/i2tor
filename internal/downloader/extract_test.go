package downloader

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "evil.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatalf("Create zip entry error = %v", err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatalf("Write zip entry error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip writer error = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close file error = %v", err)
	}

	err = Extract(context.Background(), archivePath, filepath.Join(dir, "out"))
	if err == nil {
		t.Fatalf("Extract() error = nil, want traversal rejection")
	}
}
