package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"i2tor/internal/apppaths"
	"i2tor/internal/install"
	"i2tor/internal/logging"
)

func ResolveTorBrowserExecutable(installInfo install.InstalledApp) (string, error) {
	if runtime.GOOS == "linux" {
		for _, candidate := range []string{
			filepath.Join(installInfo.InstallDir, "Browser", "firefox"),
			filepath.Join(installInfo.InstallDir, "Browser", "firefox.real"),
		} {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, nil
			}
		}
	}
	return installInfo.ResolveExecutable()
}

func BuildTorBrowserArgs(profileDir string) []string {
	return []string{"-profile", profileDir}
}

func LaunchTorBrowser(ctx context.Context, logger *logging.Logger, installInfo install.InstalledApp, profileDir string, paths apppaths.AppPaths) (ManagedProcess, error) {
	_ = paths
	execPath, err := ResolveTorBrowserExecutable(installInfo)
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("resolve Tor Browser executable: %w", err)
	}
	workdir := installInfo.InstallDir
	if workdir == "" {
		workdir = "."
	}
	return startCommandWithOptions(ctx, logger, "torbrowser", "tor-browser", execPath, BuildTorBrowserArgs(profileDir), workdir, nil)
}

func StartBundledTor(ctx context.Context, logger *logging.Logger, torBrowser install.InstalledApp, paths apppaths.AppPaths) (ManagedProcess, error) {
	root := torBrowser.InstallDir
	browserDir := filepath.Join(root, "Browser")
	torDir := filepath.Join(browserDir, "TorBrowser", "Tor")
	bundleDataDir := filepath.Join(browserDir, "TorBrowser", "Data", "Tor")
	dataDir := filepath.Join(paths.RuntimeDir, "tor-data")
	torBinary, err := install.ResolveBundledTorExecutable(root)
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("resolve bundled Tor executable: %w", err)
	}
	defaultsTorrc := filepath.Join(bundleDataDir, "torrc-defaults")
	launcherTorrc := filepath.Join(paths.StateDir, "tor-launcherrc")

	if err := os.MkdirAll(filepath.Join(dataDir, "onion-auth"), 0o700); err != nil {
		return ManagedProcess{}, fmt.Errorf("create Tor onion-auth dir: %w", err)
	}
	if err := os.WriteFile(launcherTorrc, []byte(fmt.Sprintf(
		"ClientOnionAuthDir %s\nDataDirectory %s\nDisableNetwork 0\nGeoIPFile %s\nGeoIPv6File %s\nSocksPort 127.0.0.1:9150\nControlPort 127.0.0.1:9151\n",
		filepath.Join(dataDir, "onion-auth"),
		dataDir,
		filepath.Join(bundleDataDir, "geoip"),
		filepath.Join(bundleDataDir, "geoip6"),
	)), 0o600); err != nil {
		return ManagedProcess{}, fmt.Errorf("write launcher torrc %q: %w", launcherTorrc, err)
	}

	env := os.Environ()
	if runtime.GOOS == "linux" {
		env = append(env,
			"LD_LIBRARY_PATH="+strings.Join([]string{torDir, filepath.Join(torDir, "libstdc++")}, string(os.PathListSeparator)),
			"HOME="+root,
		)
	}
	args := []string{
		"--defaults-torrc", defaultsTorrc,
		"-f", launcherTorrc,
	}
	return startCommandWithOptions(ctx, logger, "tor", "tor", torBinary, args, browserDir, env)
}

func ShutdownManagedTor(ctx context.Context, proc ManagedProcess) error {
	if !proc.Owned {
		return nil
	}
	if proc.PID <= 0 {
		return nil
	}
	if err := terminatePID(proc.PID); err != nil {
		return fmt.Errorf("terminate Tor pid %d: %w", proc.PID, err)
	}
	if proc.Cmd == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- proc.Cmd.Wait() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}
