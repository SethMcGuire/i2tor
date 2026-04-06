package install

import (
	"bufio"
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

func InstallManagedI2P(ctx context.Context, paths apppaths.AppPaths, java InstalledApp) (InstalledApp, error) {
	return installManagedI2P(ctx, paths, java, true)
}

func ReinstallManagedI2P(ctx context.Context, paths apppaths.AppPaths, java InstalledApp) (InstalledApp, error) {
	return installManagedI2P(ctx, paths, java, false)
}

func installManagedI2P(ctx context.Context, paths apppaths.AppPaths, java InstalledApp, allowReuse bool) (InstalledApp, error) {
	if allowReuse {
		if existing, err := ReuseManagedI2P(paths); err == nil {
			return existing, nil
		}
	}

	meta, err := i2pMetadata(ctx)
	if err != nil {
		return InstalledApp{}, err
	}
	artifactPath, err := downloader.Download(ctx, paths.DownloadsDir, meta)
	if err != nil {
		return InstalledApp{}, fmt.Errorf("download I2P from %s: %w", meta.ArtifactURL, err)
	}
	if err := VerifyI2PDownload(ctx, paths, artifactPath, meta); err != nil {
		return InstalledApp{}, err
	}

	parent := filepath.Dir(paths.I2PRuntimeDir)
	stageDir, err := os.MkdirTemp(parent, "i2p-install-*")
	if err != nil {
		return InstalledApp{}, fmt.Errorf("create temp I2P install dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	if err := runI2PInstaller(ctx, java, artifactPath, stageDir); err != nil {
		return InstalledApp{}, err
	}
	if err := os.RemoveAll(paths.I2PRuntimeDir); err != nil {
		return InstalledApp{}, fmt.Errorf("clear previous I2P install %q: %w", paths.I2PRuntimeDir, err)
	}
	if err := os.Rename(stageDir, paths.I2PRuntimeDir); err != nil {
		return InstalledApp{}, fmt.Errorf("move I2P install into place %q: %w", paths.I2PRuntimeDir, err)
	}
	if err := NormalizeManagedI2PPortableConfig(paths.I2PRuntimeDir, java); err != nil {
		return InstalledApp{}, err
	}

	execPath, err := InstalledApp{Name: "i2p", InstallDir: paths.I2PRuntimeDir}.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	return InstalledApp{
		Name:              "i2p",
		Source:            "managed",
		Version:           meta.Version,
		InstallDir:        paths.I2PRuntimeDir,
		ExecutablePath:    execPath,
		ArtifactURL:       meta.ArtifactURL,
		ArtifactPath:      artifactPath,
		ChecksumSHA256:    meta.ChecksumSHA256,
		SignatureVerified: meta.SignatureURL != "",
		Verified:          true,
	}, nil
}

func ReuseExistingI2P(candidate detect.InstallCandidate) InstalledApp {
	return InstalledApp{
		Name:           "i2p",
		Source:         "existing",
		Version:        candidate.Version,
		InstallDir:     candidate.RootPath,
		ExecutablePath: candidate.Executable,
	}
}

func VerifyI2PDownload(ctx context.Context, paths apppaths.AppPaths, artifactPath string, metadata downloader.ArtifactMetadata) error {
	if err := downloader.VerifyDetachedSignature(ctx, paths.DownloadsDir, artifactPath, metadata); err != nil {
		return fmt.Errorf("failed to verify I2P signature for %q: %w", artifactPath, err)
	}
	if err := downloader.VerifySHA256(ctx, artifactPath, metadata); err != nil {
		return fmt.Errorf("failed to verify I2P checksum for %q: %w", artifactPath, err)
	}
	return nil
}

func ReuseManagedI2P(paths apppaths.AppPaths) (InstalledApp, error) {
	app := InstalledApp{Name: "i2p", Source: "managed", InstallDir: paths.I2PRuntimeDir}
	execPath, err := app.ResolveExecutable()
	if err != nil {
		return InstalledApp{}, err
	}
	app.ExecutablePath = execPath
	return app, nil
}

type i2pRelease struct {
	TagName string            `json:"tag_name"`
	Assets  []i2pReleaseAsset `json:"assets"`
}

type i2pReleaseAsset struct {
	Name               string `json:"name"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func i2pMetadata(ctx context.Context) (downloader.ArtifactMetadata, error) {
	release, err := fetchLatestI2PRelease(ctx)
	if err != nil {
		return downloader.ArtifactMetadata{}, fmt.Errorf("resolve latest I2P release metadata: %w", err)
	}
	asset, err := pickI2PAsset(release, runtime.GOOS)
	if err != nil {
		return downloader.ArtifactMetadata{}, err
	}
	return downloader.ArtifactMetadata{
		Name:           "i2p",
		Version:        release.TagName,
		FileName:       asset.Name,
		ArtifactURL:    asset.BrowserDownloadURL,
		ChecksumSHA256: strings.TrimPrefix(asset.Digest, "sha256:"),
		SignatureURL:   assetSignatureURL(release, asset.Name),
		SignatureKey:   "i2p",
	}, nil
}

func LatestI2PMetadata(ctx context.Context) (downloader.ArtifactMetadata, error) {
	return i2pMetadata(ctx)
}

func assetSignatureURL(release i2pRelease, assetName string) string {
	for _, asset := range release.Assets {
		if asset.Name == assetName+".sig" {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func fetchLatestI2PRelease(ctx context.Context) (i2pRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/i2p/i2p.i2p/releases/latest", nil)
	if err != nil {
		return i2pRelease{}, fmt.Errorf("create I2P release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "i2tor")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return i2pRelease{}, fmt.Errorf("fetch I2P release metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return i2pRelease{}, fmt.Errorf("fetch I2P release metadata: unexpected status %s", resp.Status)
	}
	var release i2pRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return i2pRelease{}, fmt.Errorf("decode I2P release metadata: %w", err)
	}
	return release, nil
}

func pickI2PAsset(release i2pRelease, goos string) (i2pReleaseAsset, error) {
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".jar") && strings.HasPrefix(asset.Name, "i2pinstall_") && !strings.Contains(asset.Name, "_windows") {
			return asset, nil
		}
	}
	if goos == "windows" {
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, "_windows.exe") {
				return asset, nil
			}
		}
	}
	return i2pReleaseAsset{}, fmt.Errorf("no supported I2P installer asset found in release %s", release.TagName)
}

func runI2PInstaller(ctx context.Context, java InstalledApp, installerJarPath, installDir string) error {
	javaPath, err := java.ResolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve Java runtime for I2P installer: %w", err)
	}
	templatePath := filepath.Join(filepath.Dir(installDir), "i2p-installer-template.properties")
	propsPath := filepath.Join(filepath.Dir(installDir), "i2p-installer.properties")
	if err := exec.CommandContext(ctx, javaPath, "-jar", installerJarPath, "-options-template", templatePath).Run(); err != nil {
		return fmt.Errorf("generate I2P installer properties template: %w", err)
	}
	if err := writeInstallerProperties(templatePath, propsPath, installDir); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, javaPath, "-jar", installerJarPath, "-options", propsPath)
	cmd.Env = append(os.Environ(), "JAVA_TOOL_OPTIONS=-Djava.awt.headless=true")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run unattended I2P installer into %s: %w: %s", installDir, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func writeInstallerProperties(templatePath, destPath, installDir string) error {
	template, err := os.Open(templatePath)
	if err != nil {
		return fmt.Errorf("open I2P installer template %q: %w", templatePath, err)
	}
	defer template.Close()

	var lines []string
	found := false
	scanner := bufio.NewScanner(template)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "INSTALL_PATH=") || line == "INSTALL_PATH" {
			lines = append(lines, "INSTALL_PATH="+installDir)
			found = true
			continue
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read I2P installer template %q: %w", templatePath, err)
	}
	if !found {
		lines = append(lines, "INSTALL_PATH="+installDir)
	}
	return os.WriteFile(destPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func NormalizeManagedI2PPortableConfig(installDir string, java InstalledApp) error {
	javaPath, err := java.ResolveExecutable()
	if err != nil {
		return fmt.Errorf("resolve Java runtime for portable I2P config: %w", err)
	}
	javaHome := javaHomeFromExecutable(javaPath)

	wrapperPath := filepath.Join(installDir, "wrapper.config")
	wrapperLines, err := readLines(wrapperPath)
	if err != nil {
		return fmt.Errorf("read wrapper config %q: %w", wrapperPath, err)
	}
	wrapperLines = replaceInstallPathReferences(wrapperLines, installDir)
	wrapperLines = setConfigLine(wrapperLines, "set.JAVA_HOME=", "set.JAVA_HOME="+javaHome)
	wrapperLines = setConfigLine(wrapperLines, "wrapper.java.command=", "wrapper.java.command="+javaPath)
	wrapperLines = setConfigLine(wrapperLines, "wrapper.java.classpath.1=", filepath.Join(installDir, "lib", "*.jar"))
	wrapperLines = setConfigLine(wrapperLines, "wrapper.java.library.path.1=", installDir)
	wrapperLines = setConfigLine(wrapperLines, "wrapper.java.library.path.2=", filepath.Join(installDir, "lib"))
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.2=", fmt.Sprintf("wrapper.java.additional.2=-Di2p.dir.base=%q", installDir))
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.2.stripquotes=", "wrapper.java.additional.2.stripquotes=TRUE")
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.3=", fmt.Sprintf("wrapper.java.additional.3=-Di2p.dir.pid=%q", installDir))
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.3.stripquotes=", "wrapper.java.additional.3.stripquotes=TRUE")
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.4=", fmt.Sprintf("wrapper.java.additional.4=-Di2p.dir.temp=%q", installDir))
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.4.stripquotes=", "wrapper.java.additional.4.stripquotes=TRUE")
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.5=", fmt.Sprintf("wrapper.java.additional.5=-Di2p.dir.config=%q", installDir))
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.java.additional.5.stripquotes=", "wrapper.java.additional.5.stripquotes=TRUE")
	wrapperLines = uncommentAndSet(wrapperLines, "wrapper.logfile=", "wrapper.logfile="+filepath.Join(installDir, "wrapper.log"))
	if err := os.WriteFile(wrapperPath, []byte(strings.Join(wrapperLines, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write wrapper config %q: %w", wrapperPath, err)
	}

	routerPath := filepath.Join(installDir, "i2prouter")
	routerLines, err := readLines(routerPath)
	if err != nil {
		return fmt.Errorf("read i2prouter script %q: %w", routerPath, err)
	}
	routerLines = replaceInstallPathReferences(routerLines, installDir)
	routerLines = setConfigLine(routerLines, "I2P=", fmt.Sprintf("I2P=%q", installDir))
	routerLines = setConfigLine(routerLines, "I2P_CONFIG_DIR=", fmt.Sprintf("I2P_CONFIG_DIR=%q", installDir))
	routerLines = uncommentAndSet(routerLines, "I2PTEMP=", fmt.Sprintf("I2PTEMP=%q", installDir))
	routerLines = uncommentAndSet(routerLines, "PIDDIR=", fmt.Sprintf("PIDDIR=%q", installDir))
	routerLines = uncommentAndSet(routerLines, "LOGDIR=", fmt.Sprintf("LOGDIR=%q", installDir))
	if err := os.WriteFile(routerPath, []byte(strings.Join(routerLines, "\n")+"\n"), 0o755); err != nil {
		return fmt.Errorf("write i2prouter script %q: %w", routerPath, err)
	}

	for _, scriptName := range []string{"runplain.sh", "eepget"} {
		scriptPath := filepath.Join(installDir, scriptName)
		scriptLines, err := readLines(scriptPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read portable I2P script %q: %w", scriptPath, err)
		}
		scriptLines = replaceInstallPathReferences(scriptLines, installDir)
		switch scriptName {
		case "runplain.sh":
			scriptLines = setConfigLine(scriptLines, "I2P=", fmt.Sprintf("I2P=%q", installDir))
			scriptLines = setConfigLine(scriptLines, "I2PTEMP=", fmt.Sprintf("I2PTEMP=%q", installDir))
		case "eepget":
			scriptLines = setConfigLine(scriptLines, "I2P=", fmt.Sprintf("I2P=%q", installDir))
		}
		if err := os.WriteFile(scriptPath, []byte(strings.Join(scriptLines, "\n")+"\n"), 0o755); err != nil {
			return fmt.Errorf("write portable I2P script %q: %w", scriptPath, err)
		}
	}
	return nil
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), nil
}

func setConfigLine(lines []string, prefix, value string) []string {
	for i, line := range lines {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "#")
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = value
			return lines
		}
	}
	return append(lines, value)
}

func uncommentAndSet(lines []string, prefix, value string) []string {
	return setConfigLine(lines, prefix, value)
}

func replaceInstallPathReferences(lines []string, installDir string) []string {
	for i, line := range lines {
		lines[i] = rewriteInstallPathReference(line, installDir)
	}
	return lines
}

func rewriteInstallPathReference(line, installDir string) string {
	for _, marker := range []string{"/runtime/i2p-install-", "/runtime/i2p/"} {
		idx := strings.Index(line, marker)
		if idx == -1 {
			continue
		}
		start := strings.LastIndex(line[:idx], "/home/")
		if start == -1 {
			start = strings.LastIndex(line[:idx], "\"/home/")
			if start == -1 {
				start = strings.LastIndex(line[:idx], "='/home/")
			}
		}
		if start == -1 {
			continue
		}
		end := idx + len(marker)
		for end < len(line) {
			ch := line[end]
			if ch == '"' || ch == '\'' || ch == ' ' || ch == '\t' {
				break
			}
			end++
		}
		return line[:start] + installDir + line[end:]
	}
	return line
}
