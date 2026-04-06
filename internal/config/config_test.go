package config

import "testing"

func TestDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if !cfg.ReuseExistingTorBrowser {
		t.Fatalf("ReuseExistingTorBrowser = false, want true")
	}
	if !cfg.ReuseExistingI2P {
		t.Fatalf("ReuseExistingI2P = false, want true")
	}
	if !cfg.AutoCheckUpdates {
		t.Fatalf("AutoCheckUpdates = false, want true")
	}
	if cfg.AllowLocalhostAccess {
		t.Fatalf("AllowLocalhostAccess = true, want false")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
}
