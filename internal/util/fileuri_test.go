package util

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFileURI(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "proxy pac.pac")
	got, err := FileURI(path)
	if err != nil {
		t.Fatalf("FileURI() error = %v", err)
	}
	if !strings.HasPrefix(got, "file://") {
		t.Fatalf("FileURI() = %q, want file:// prefix", got)
	}
	if !strings.Contains(got, "proxy%20pac.pac") {
		t.Fatalf("FileURI() = %q, want escaped path", got)
	}
}
