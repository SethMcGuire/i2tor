package install

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"i2tor/internal/apppaths"
	"i2tor/internal/detect"
	"i2tor/internal/downloader"
)

const minimumJavaMajor = 17

func InstallManagedJava(ctx context.Context, paths apppaths.AppPaths) (InstalledApp, error) {
	return installManagedJava(ctx, paths, true)
}

func ReinstallManagedJava(ctx context.Context, paths apppaths.AppPaths) (InstalledApp, error) {
	return installManagedJava(ctx, paths, false)
}

func installManagedJava(ctx context.Context, paths apppaths.AppPaths, allowReuse bool) (InstalledApp, error) {
	if allowReuse {
		if existing, err := ReuseManagedJava(paths); err == nil {
			return existing, nil
		}
	}
	meta, err := javaMetadata(ctx)
	if err != nil {
		return InstalledApp{}, err
	}
	artifactPath, err := downloader.Download(ctx, paths.DownloadsDir, meta)
	if err != nil {
		return InstalledApp{}, fmt.Errorf("download Java runtime from %s: %w", meta.ArtifactURL, err)
	}
	if err := downloader.VerifyDetachedSignature(ctx, paths.DownloadsDir, artifactPath, meta); err != nil {
		return InstalledApp{}, fmt.Errorf("failed to verify Java runtime signature for %q: %w", artifactPath, err)
	}
	if err := downloader.VerifySHA256(ctx, artifactPath, meta); err != nil {
		return InstalledApp{}, fmt.Errorf("failed to verify Java runtime checksum for %q: %w", artifactPath, err)
	}
	installDir, err := extractManaged(ctx, artifactPath, paths.JavaRuntimeDir)
	if err != nil {
		return InstalledApp{}, err
	}
	javaPath, err := InstalledApp{Name: "java", InstallDir: installDir}.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	return InstalledApp{
		Name:              "java",
		Source:            "managed",
		Version:           meta.Version,
		InstallDir:        installDir,
		ExecutablePath:    javaPath,
		ArtifactURL:       meta.ArtifactURL,
		ArtifactPath:      artifactPath,
		ChecksumSHA256:    meta.ChecksumSHA256,
		SignatureVerified: meta.SignatureURL != "",
		Verified:          true,
	}, nil
}

func ReuseManagedJava(paths apppaths.AppPaths) (InstalledApp, error) {
	app := InstalledApp{Name: "java", Source: "managed", InstallDir: paths.JavaRuntimeDir}
	execPath, err := app.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	app.ExecutablePath = execPath
	return app, nil
}

func ReuseExistingJava(candidate detect.InstallCandidate) InstalledApp {
	return InstalledApp{
		Name:           "java",
		Source:         "existing",
		Version:        candidate.Version,
		InstallDir:     filepath.Dir(filepath.Dir(candidate.Executable)),
		ExecutablePath: candidate.Executable,
	}
}

type adoptiumResponse []struct {
	Binary struct {
		Package struct {
			Checksum      string `json:"checksum"`
			Link          string `json:"link"`
			Name          string `json:"name"`
			SignatureLink string `json:"signature_link"`
		} `json:"package"`
	} `json:"binary"`
	ReleaseName string `json:"release_name"`
	Version     struct {
		Semver string `json:"semver"`
	} `json:"version"`
}

func javaMetadata(ctx context.Context) (downloader.ArtifactMetadata, error) {
	arch, err := adoptiumArch(runtime.GOARCH)
	if err != nil {
		return downloader.ArtifactMetadata{}, err
	}
	osName, err := adoptiumOS(runtime.GOOS)
	if err != nil {
		return downloader.ArtifactMetadata{}, err
	}
	url := fmt.Sprintf("https://api.adoptium.net/v3/assets/latest/17/hotspot?architecture=%s&heap_size=normal&image_type=jre&os=%s&vendor=eclipse", arch, osName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return downloader.ArtifactMetadata{}, fmt.Errorf("create Java runtime metadata request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "i2tor")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return downloader.ArtifactMetadata{}, fmt.Errorf("fetch Java runtime metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return downloader.ArtifactMetadata{}, fmt.Errorf("fetch Java runtime metadata: unexpected status %s", resp.Status)
	}
	var payload adoptiumResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return downloader.ArtifactMetadata{}, fmt.Errorf("decode Java runtime metadata: %w", err)
	}
	if len(payload) == 0 {
		return downloader.ArtifactMetadata{}, fmt.Errorf("no Java runtime assets returned for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	asset := payload[0]
	return downloader.ArtifactMetadata{
		Name:           "java",
		Version:        asset.Version.Semver,
		FileName:       asset.Binary.Package.Name,
		ArtifactURL:    asset.Binary.Package.Link,
		ChecksumSHA256: asset.Binary.Package.Checksum,
		SignatureURL:   asset.Binary.Package.SignatureLink,
		SignatureKey:   "adoptium",
	}, nil
}

func LatestJavaMetadata(ctx context.Context) (downloader.ArtifactMetadata, error) {
	return javaMetadata(ctx)
}

func adoptiumArch(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "x64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("unsupported Java runtime architecture %q", goarch)
	}
}

func adoptiumOS(goos string) (string, error) {
	switch goos {
	case "linux", "windows", "darwin":
		if goos == "darwin" {
			return "mac", nil
		}
		return goos, nil
	default:
		return "", fmt.Errorf("unsupported Java runtime OS %q", goos)
	}
}

func javaCommand(install InstalledApp) (string, error) {
	execPath, err := install.ResolveExecutable()
	if err != nil {
		return "", err
	}
	return execPath, nil
}

func javaHomeFromExecutable(path string) string {
	return filepath.Dir(filepath.Dir(path))
}

func normalizeJavaVersion(version string) string {
	return strings.TrimPrefix(version, "jdk-")
}
