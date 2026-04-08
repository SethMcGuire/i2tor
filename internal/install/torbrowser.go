package install

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"i2tor/internal/apppaths"
	"i2tor/internal/detect"
	"i2tor/internal/downloader"
)

type torBrowserRelease struct {
	Version string `json:"version"`
	Binary  string `json:"binary"`
	Sig     string `json:"sig"`
}

type InstalledApp struct {
	Name              string
	Source            string
	Version           string
	InstallDir        string
	ExecutablePath    string
	ArtifactURL       string
	ArtifactPath      string
	ChecksumSHA256    string
	SignatureVerified  bool
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

	meta, err := torBrowserMetadata(ctx)
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
	var installDir string
	if strings.HasSuffix(artifactPath, ".exe") {
		installDir, err = runNSISInstaller(ctx, artifactPath, paths.TorBrowserRuntimeDir)
	} else {
		installDir, err = extractManaged(ctx, artifactPath, paths.TorBrowserRuntimeDir)
	}
	if err != nil {
		return InstalledApp{}, err
	}
	execPath, err := InstalledApp{Name: "tor-browser", InstallDir: installDir}.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	return InstalledApp{
		Name:             "tor-browser",
		Source:           "managed",
		Version:          meta.Version,
		InstallDir:       installDir,
		ExecutablePath:   execPath,
		ArtifactURL:      meta.ArtifactURL,
		ArtifactPath:     artifactPath,
		ChecksumSHA256:   meta.ChecksumSHA256,
		SignatureVerified: meta.SignatureURL != "",
		Verified:         true,
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
	if err := downloader.VerifyDetachedSignature(ctx, paths.DownloadsDir, artifactPath, metadata); err != nil {
		return fmt.Errorf("failed to verify Tor Browser signature for %q: %w", artifactPath, err)
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

func torBrowserPlatform(goos, goarch string) (string, error) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return "linux-x86_64", nil
		case "arm64":
			return "linux-aarch64", nil
		}
	case "windows":
		if goarch == "amd64" {
			return "windows-x86_64", nil
		}
	case "darwin":
		return "macos", nil
	}
	return "", fmt.Errorf("unsupported Tor Browser platform %s/%s", goos, goarch)
}

func fetchTorBrowserRelease(ctx context.Context, goos, goarch string) (torBrowserRelease, error) {
	platform, err := torBrowserPlatform(goos, goarch)
	if err != nil {
		return torBrowserRelease{}, err
	}
	url := fmt.Sprintf("https://aus1.torproject.org/torbrowser/update_3/release/download-%s.json", platform)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return torBrowserRelease{}, fmt.Errorf("create Tor Browser release request: %w", err)
	}
	req.Header.Set("User-Agent", "i2tor")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return torBrowserRelease{}, fmt.Errorf("fetch Tor Browser release metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return torBrowserRelease{}, fmt.Errorf("fetch Tor Browser release metadata: unexpected status %s", resp.Status)
	}
	var release torBrowserRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return torBrowserRelease{}, fmt.Errorf("decode Tor Browser release metadata: %w", err)
	}
	if release.Version == "" || release.Binary == "" {
		return torBrowserRelease{}, fmt.Errorf("incomplete Tor Browser release metadata from %s", url)
	}
	return release, nil
}

func torBrowserMetadata(ctx context.Context) (downloader.ArtifactMetadata, error) {
	release, err := fetchTorBrowserRelease(ctx, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return downloader.ArtifactMetadata{}, fmt.Errorf("resolve latest Tor Browser release metadata: %w", err)
	}
	return downloader.ArtifactMetadata{
		Name:         "tor-browser",
		Version:      release.Version,
		FileName:     filepath.Base(release.Binary),
		ArtifactURL:  release.Binary,
		SignatureURL: release.Sig,
		SignatureKey: "torbrowser",
	}, nil
}

func LatestTorBrowserMetadata(ctx context.Context) (downloader.ArtifactMetadata, error) {
	return torBrowserMetadata(ctx)
}

func runNSISInstaller(ctx context.Context, installerPath, installDir string) (string, error) {
	if err := os.RemoveAll(installDir); err != nil {
		return "", fmt.Errorf("clear previous install %q: %w", installDir, err)
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", fmt.Errorf("create install directory %q: %w", installDir, err)
	}
	cmd := exec.CommandContext(ctx, installerPath, "/S", "/D="+installDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("run Tor Browser installer: %w: %s", err, string(output))
	}
	return installDir, nil
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
