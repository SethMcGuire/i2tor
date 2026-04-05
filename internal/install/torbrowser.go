package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"i2tor/internal/apppaths"
	"i2tor/internal/detect"
	"i2tor/internal/downloader"
)

const torBrowserVersion = "14.5.1"

type InstalledApp struct {
	Name              string
	Source            string
	Version           string
	InstallDir        string
	ExecutablePath    string
	ArtifactURL       string
	ArtifactPath      string
	ChecksumSHA256    string
	SignatureVerified bool
	Verified          bool
}

func (a InstalledApp) ResolveExecutable() (string, error) {
	if a.ExecutablePath != "" {
		return a.ExecutablePath, nil
	}
	switch a.Name {
	case "tor-browser":
		for _, candidate := range []string{
			filepath.Join(a.InstallDir, "Browser", "start-tor-browser"),
			filepath.Join(a.InstallDir, "start-tor-browser.desktop"),
			filepath.Join(a.InstallDir, "Browser", "firefox"),
			filepath.Join(a.InstallDir, "Browser", "firefox.exe"),
			filepath.Join(a.InstallDir, "Contents", "MacOS", "firefox"),
			filepath.Join(a.InstallDir, "Tor Browser.app", "Contents", "MacOS", "firefox"),
		} {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, nil
			}
		}
	case "i2pd":
		for _, candidate := range []string{
			filepath.Join(a.InstallDir, "i2pd"),
			filepath.Join(a.InstallDir, "i2pd.exe"),
			filepath.Join(a.InstallDir, "bin", "i2pd"),
			filepath.Join(a.InstallDir, "bin", "i2pd.exe"),
			filepath.Join(a.InstallDir, "usr", "sbin", "i2pd"),
			filepath.Join(a.InstallDir, "usr", "bin", "i2pd"),
		} {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, nil
			}
		}
	case "i2p":
		for _, candidate := range []string{
			filepath.Join(a.InstallDir, "i2prouter"),
			filepath.Join(a.InstallDir, "runplain.sh"),
		} {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, nil
			}
		}
	case "java":
		for _, candidate := range []string{
			filepath.Join(a.InstallDir, "bin", "java"),
			filepath.Join(a.InstallDir, "Contents", "Home", "bin", "java"),
			filepath.Join(a.InstallDir, "bin", "java.exe"),
		} {
			if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("no executable found for %s in %q", a.Name, a.InstallDir)
}

func InstallManagedTorBrowser(ctx context.Context, paths apppaths.AppPaths) (InstalledApp, error) {
	return installManagedTorBrowser(ctx, paths, true)
}

func ReinstallManagedTorBrowser(ctx context.Context, paths apppaths.AppPaths) (InstalledApp, error) {
	return installManagedTorBrowser(ctx, paths, false)
}

func installManagedTorBrowser(ctx context.Context, paths apppaths.AppPaths, allowReuse bool) (InstalledApp, error) {
	if allowReuse {
		if existing, err := ReuseManagedTorBrowser(paths); err == nil {
			return existing, nil
		}
	}

	meta, err := torBrowserMetadata()
	if err != nil {
		return InstalledApp{}, err
	}
	artifactPath, err := downloader.Download(ctx, paths.DownloadsDir, meta)
	if err != nil {
		return InstalledApp{}, fmt.Errorf("download Tor Browser from %s: %w", meta.ArtifactURL, err)
	}
	if err := VerifyTorBrowserDownload(ctx, paths, artifactPath, meta); err != nil {
		return InstalledApp{}, err
	}
	installDir, err := extractManaged(ctx, artifactPath, paths.TorBrowserRuntimeDir)
	if err != nil {
		return InstalledApp{}, err
	}
	execPath, err := InstalledApp{Name: "tor-browser", InstallDir: installDir}.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	return InstalledApp{
		Name:              "tor-browser",
		Source:            "managed",
		Version:           meta.Version,
		InstallDir:        installDir,
		ExecutablePath:    execPath,
		ArtifactURL:       meta.ArtifactURL,
		ArtifactPath:      artifactPath,
		ChecksumSHA256:    meta.ChecksumSHA256,
		SignatureVerified: meta.SignatureURL != "",
		Verified:          true,
	}, nil
}

func ReuseExistingTorBrowser(candidate detect.InstallCandidate) InstalledApp {
	return InstalledApp{
		Name:           "tor-browser",
		Source:         "existing",
		Version:        candidate.Version,
		InstallDir:     candidate.RootPath,
		ExecutablePath: candidate.Executable,
	}
}

func VerifyTorBrowserDownload(ctx context.Context, paths apppaths.AppPaths, artifactPath string, metadata downloader.ArtifactMetadata) error {
	if err := downloader.VerifySignedChecksum(ctx, paths.DownloadsDir, metadata); err != nil {
		return fmt.Errorf("failed to verify Tor Browser signed checksum file for %q: %w", artifactPath, err)
	}
	if err := downloader.VerifySHA256(ctx, artifactPath, metadata); err != nil {
		return fmt.Errorf("failed to verify Tor Browser checksum for %q: %w", artifactPath, err)
	}
	return nil
}

func ReuseManagedTorBrowser(paths apppaths.AppPaths) (InstalledApp, error) {
	app := InstalledApp{Name: "tor-browser", Source: "managed", InstallDir: paths.TorBrowserRuntimeDir}
	execPath, err := app.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	app.ExecutablePath = execPath
	return app, nil
}

func torBrowserMetadata() (downloader.ArtifactMetadata, error) {
	arch := runtime.GOARCH
	osName := runtime.GOOS
	switch osName {
	case "linux":
		switch arch {
		case "amd64":
			return downloader.ArtifactMetadata{
				Name:           "tor-browser",
				Version:        torBrowserVersion,
				FileName:       fmt.Sprintf("tor-browser-linux-x86_64-%s.tar.xz", torBrowserVersion),
				ArtifactURL:    fmt.Sprintf("https://www.torproject.org/dist/torbrowser/%s/tor-browser-linux-x86_64-%s.tar.xz", torBrowserVersion, torBrowserVersion),
				ChecksumURL:    fmt.Sprintf("https://archive.torproject.org/tor-package-archive/torbrowser/%s/sha256sums-signed-build.txt", torBrowserVersion),
				SignatureURL:   fmt.Sprintf("https://archive.torproject.org/tor-package-archive/torbrowser/%s/sha256sums-signed-build.txt.asc", torBrowserVersion),
				SignatureKey:   "torbrowser",
				ChecksumSHA256: "",
			}, nil
		case "arm64":
			return downloader.ArtifactMetadata{
				Name:         "tor-browser",
				Version:      torBrowserVersion,
				FileName:     fmt.Sprintf("tor-browser-linux-arm64-%s.tar.xz", torBrowserVersion),
				ArtifactURL:  fmt.Sprintf("https://www.torproject.org/dist/torbrowser/%s/tor-browser-linux-arm64-%s.tar.xz", torBrowserVersion, torBrowserVersion),
				ChecksumURL:  fmt.Sprintf("https://archive.torproject.org/tor-package-archive/torbrowser/%s/sha256sums-signed-build.txt", torBrowserVersion),
				SignatureURL: fmt.Sprintf("https://archive.torproject.org/tor-package-archive/torbrowser/%s/sha256sums-signed-build.txt.asc", torBrowserVersion),
				SignatureKey: "torbrowser",
			}, nil
		}
	case "windows":
		if arch == "amd64" {
			return downloader.ArtifactMetadata{
				Name:        "tor-browser",
				Version:     torBrowserVersion,
				FileName:    fmt.Sprintf("tor-browser-windows-x86_64-portable-%s.exe", torBrowserVersion),
				ArtifactURL: fmt.Sprintf("https://www.torproject.org/dist/torbrowser/%s/tor-browser-windows-x86_64-portable-%s.exe", torBrowserVersion, torBrowserVersion),
			}, nil
		}
	case "darwin":
		if arch == "amd64" || arch == "arm64" {
			return downloader.ArtifactMetadata{
				Name:        "tor-browser",
				Version:     torBrowserVersion,
				FileName:    fmt.Sprintf("TorBrowser-%s-macos_ALL.dmg", torBrowserVersion),
				ArtifactURL: fmt.Sprintf("https://www.torproject.org/dist/torbrowser/%s/TorBrowser-%s-macos_ALL.dmg", torBrowserVersion, torBrowserVersion),
			}, nil
		}
	}
	return downloader.ArtifactMetadata{}, fmt.Errorf("unsupported Tor Browser platform %s/%s", osName, arch)
}

func LatestTorBrowserMetadata() (downloader.ArtifactMetadata, error) {
	return torBrowserMetadata()
}

func extractManaged(ctx context.Context, artifactPath, targetDir string) (string, error) {
	tmpParent := filepath.Dir(targetDir)
	tempDir, err := os.MkdirTemp(tmpParent, "install-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp install dir for %q: %w", artifactPath, err)
	}
	defer os.RemoveAll(tempDir)

	if err := downloader.Extract(ctx, artifactPath, tempDir); err != nil {
		return "", fmt.Errorf("failed to extract archive safely %q: %w", artifactPath, err)
	}

	root, err := extractedRoot(tempDir)
	if err != nil {
		return "", err
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return "", fmt.Errorf("clear previous install %q: %w", targetDir, err)
	}
	if err := os.Rename(root, targetDir); err != nil {
		return "", fmt.Errorf("move install into place %q: %w", targetDir, err)
	}
	return targetDir, nil
}

func extractedRoot(tempDir string) (string, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("read extracted temp dir %q: %w", tempDir, err)
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(tempDir, entries[0].Name()), nil
	}
	return tempDir, nil
}

func installTimestamp() time.Time {
	return time.Now().UTC()
}

func trimVersionPrefix(v string) string {
	return strings.TrimPrefix(v, "v")
}
