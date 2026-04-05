package state

import (
	"context"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manifest.json")
	want := Manifest{
		AppVersion: "0.1.0",
		OS:         "linux",
		Arch:       "x86_64",
		TorBrowser: InstallRecord{Source: "managed", Version: "1"},
	}
	if err := SaveManifest(context.Background(), path, want); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}
	got, err := LoadManifest(context.Background(), path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if got.AppVersion != want.AppVersion || got.TorBrowser.Source != want.TorBrowser.Source {
		t.Fatalf("manifest mismatch: %+v", got)
	}
}
