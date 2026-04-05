package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/logging"
	"i2tor/internal/state"
)

func TestCommandRunGeneratesPACAndProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	paths, logger, manifest := setupManagedInstall(t)
	defer logger.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:4444")
	if err != nil {
		t.Skipf("port 4444 unavailable: %v", err)
	}
	defer ln.Close()
	torLn, err := net.Listen("tcp", "127.0.0.1:9150")
	if err != nil {
		t.Skipf("port 9150 unavailable: %v", err)
	}
	defer torLn.Close()

	if err := commandRun(ctx, logger, config.Default(), paths, &manifest); err != nil {
		t.Fatalf("commandRun() error = %v", err)
	}

	pacData, err := os.ReadFile(paths.PACFile)
	if err != nil {
		t.Fatalf("ReadFile(PAC) error = %v", err)
	}
	if !strings.Contains(string(pacData), `PROXY 127.0.0.1:4444`) {
		t.Fatalf("PAC file missing I2P proxy rule")
	}

	userJS, err := os.ReadFile(filepath.Join(paths.ProfileDir, "user.js"))
	if err != nil {
		t.Fatalf("ReadFile(user.js) error = %v", err)
	}
	userJSContent := string(userJS)
	if !strings.Contains(userJSContent, `network.proxy.autoconfig_url`) {
		t.Fatalf("user.js missing PAC pref")
	}
	for _, needle := range []string{
		`extensions.torbutton.use_nontor_proxy`,
		`extensions.torbutton.startup`,
		`extensions.torlauncher.prompt_at_startup`,
		`extensions.torlauncher.start_tor`,
		`browser.fixup.domainsuffixwhitelist.i2p`,
	} {
		if !strings.Contains(userJSContent, needle) {
			t.Fatalf("user.js missing required proxy compatibility pref %q", needle)
		}
	}
}

func TestCommandRunFailsClosedWhenI2PNotReady(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx := context.Background()
	paths, logger, manifest := setupManagedInstall(t)
	defer logger.Close()

	err := commandRun(ctx, logger, config.Default(), paths, &manifest)
	if err == nil {
		t.Fatalf("commandRun() error = nil, want readiness failure")
	}
	if _, statErr := os.Stat(paths.PACFile); statErr == nil {
		t.Fatalf("PAC file exists despite readiness failure")
	}
	if _, statErr := os.Stat(filepath.Join(paths.ProfileDir, "user.js")); statErr == nil {
		t.Fatalf("user.js exists despite readiness failure")
	}
}

func setupManagedInstall(t *testing.T) (apppaths.AppPaths, *logging.Logger, state.Manifest) {
	t.Helper()

	root := t.TempDir()
	paths, err := apppaths.Resolve(context.Background(), root)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	logger, err := logging.New(context.Background(), paths.CurrentLogFile, "info")
	if err != nil {
		t.Fatalf("logging.New() error = %v", err)
	}

	writeExecutable(t, filepath.Join(paths.I2PRuntimeDir, "i2prouter"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(paths.I2PRuntimeDir, "lib", "dummy.jar"), "")
	writeExecutable(t, filepath.Join(paths.JavaRuntimeDir, "bin", "java"), "#!/bin/sh\ntrap 'exit 0' TERM INT\nwhile true; do sleep 1; done\n")
	writeExecutable(t, filepath.Join(paths.TorBrowserRuntimeDir, "Browser", "TorBrowser", "Tor", "tor"), "#!/bin/sh\ntrap 'exit 0' TERM INT\nwhile true; do sleep 1; done\n")
	writeExecutable(t, filepath.Join(paths.TorBrowserRuntimeDir, "Browser", "TorBrowser", "Data", "Tor", "torrc-defaults"), "")
	writeExecutable(t, filepath.Join(paths.TorBrowserRuntimeDir, "Browser", "TorBrowser", "Data", "Tor", "geoip"), "")
	writeExecutable(t, filepath.Join(paths.TorBrowserRuntimeDir, "Browser", "TorBrowser", "Data", "Tor", "geoip6"), "")
	writeExecutable(t, filepath.Join(paths.TorBrowserRuntimeDir, "Browser", "firefox"), "#!/bin/sh\nexit 0\n")
	return paths, logger, state.DefaultManifest()
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
