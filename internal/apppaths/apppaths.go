package apppaths

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type AppPaths struct {
	Root                 string
	DownloadsDir         string
	RuntimeDir           string
	TorBrowserRuntimeDir string
	I2PRuntimeDir        string
	JavaRuntimeDir       string
	ProfileDir           string
	PACDir               string
	PACFile              string
	LogsDir              string
	CurrentLogFile       string
	StateDir             string
	ManifestPath         string
	ConfigPath           string
	LockPath             string
}

func Resolve(ctx context.Context, configuredRoot string) (AppPaths, error) {
	_ = ctx
	root, err := dataRoot(configuredRoot)
	if err != nil {
		return AppPaths{}, err
	}

	paths := AppPaths{
		Root:                 root,
		DownloadsDir:         filepath.Join(root, "downloads"),
		RuntimeDir:           filepath.Join(root, "runtime"),
		TorBrowserRuntimeDir: filepath.Join(root, "runtime", "tor-browser"),
		I2PRuntimeDir:        filepath.Join(root, "runtime", "i2p"),
		JavaRuntimeDir:       filepath.Join(root, "runtime", "java"),
		ProfileDir:           filepath.Join(root, "runtime", "profile"),
		PACDir:               filepath.Join(root, "runtime", "pac"),
		PACFile:              filepath.Join(root, "runtime", "pac", "proxy.pac"),
		LogsDir:              filepath.Join(root, "logs"),
		CurrentLogFile:       filepath.Join(root, "logs", "current.log"),
		StateDir:             filepath.Join(root, "state"),
		ManifestPath:         filepath.Join(root, "state", "manifest.json"),
		ConfigPath:           filepath.Join(root, "state", "config.json"),
		LockPath:             filepath.Join(root, "state", "launcher.lock"),
	}

	for _, dir := range []string{
		paths.Root,
		paths.DownloadsDir,
		paths.RuntimeDir,
		paths.TorBrowserRuntimeDir,
		paths.I2PRuntimeDir,
		paths.JavaRuntimeDir,
		paths.ProfileDir,
		paths.PACDir,
		paths.LogsDir,
		paths.StateDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return AppPaths{}, fmt.Errorf("create app directory %q: %w", dir, err)
		}
	}

	return paths, nil
}

func dataRoot(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".local", "share", "i2tor"), nil
	case "windows":
		if localAppData := os.Getenv("LocalAppData"); localAppData != "" {
			return filepath.Join(localAppData, "i2tor"), nil
		}
		return filepath.Join(home, "AppData", "Local", "i2tor"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "i2tor"), nil
	default:
		return "", fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
}
