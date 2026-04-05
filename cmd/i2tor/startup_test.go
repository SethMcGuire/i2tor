package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/state"
)

func TestAssessStartupNeedsInstallWhenNothingAvailable(t *testing.T) {
	t.Parallel()

	paths, err := apppaths.Resolve(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	cfg := config.Default()
	cfg.ReuseExistingTorBrowser = false
	cfg.ReuseExistingI2P = false
	cfg.AutoCheckUpdates = false

	got := assessStartup(context.Background(), cfg, paths, state.DefaultManifest())
	if !got.NeedsInstall {
		t.Fatalf("NeedsInstall = false, want true")
	}
	if got.AutoStart {
		t.Fatalf("AutoStart = true, want false")
	}
}

func TestAssessStartupAutoStartsWhenManagedInstallsExist(t *testing.T) {
	t.Parallel()

	paths, err := apppaths.Resolve(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	cfg := config.Default()
	cfg.AutoCheckUpdates = false

	writeTestFile(t, filepath.Join(paths.JavaRuntimeDir, "bin", "java"), 0o755)
	writeTestFile(t, filepath.Join(paths.I2PRuntimeDir, "i2prouter"), 0o755)
	writeTestFile(t, filepath.Join(paths.TorBrowserRuntimeDir, "Browser", "firefox"), 0o755)

	got := assessStartup(context.Background(), cfg, paths, state.DefaultManifest())
	if !got.AutoStart {
		t.Fatalf("AutoStart = false, want true")
	}
	if got.NeedsInstall {
		t.Fatalf("NeedsInstall = true, want false")
	}
}

func writeTestFile(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("stub"), mode); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
