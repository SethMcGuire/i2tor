package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBundledTorExecutableWindows(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	torPath := filepath.Join(installDir, "Browser", "TorBrowser", "Tor", "tor.exe")
	if err := os.MkdirAll(filepath.Dir(torPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(torPath, []byte(""), 0o755); err != nil {
		t.Fatalf("WriteFile(tor.exe) error = %v", err)
	}

	got, err := ResolveBundledTorExecutable(installDir)
	if err != nil {
		t.Fatalf("ResolveBundledTorExecutable() error = %v", err)
	}
	if got != torPath {
		t.Fatalf("ResolveBundledTorExecutable() = %q, want %q", got, torPath)
	}
}
