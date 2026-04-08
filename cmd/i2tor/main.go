package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/detect"
	"i2tor/internal/install"
	"i2tor/internal/logging"
	rt "i2tor/internal/runtime"
	"i2tor/internal/state"
	"i2tor/internal/ui"
	"i2tor/internal/util"
)

const appVersion = "0.1.0"

func main() {
	ctx := context.Background()
	os.Exit(runCLI(ctx, os.Args[1:]))
}

func runCLI(ctx context.Context, args []string) int {
	command := "run"
	if len(args) > 0 {
		command = args[0]
	}

	initialPaths, err := apppaths.Resolve(ctx, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve default paths: %v\n", err)
		return 1
	}

	lock, err := state.AcquireLock(ctx, initialPaths.LockPath)
	if err != nil && command == "run" {
		fail(os.Stderr, "acquire single-instance lock", err, initialPaths.CurrentLogFile, "Close the other i2tor instance or remove a stale lock after confirming nothing is running.")
		return 1
	}
	if lock != nil {
		defer lock.Release()
	}

	logger, err := logging.New(ctx, initialPaths.CurrentLogFile, "info")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		return 1
	}
	defer logger.Close()

	cfg, err := config.Load(ctx, initialPaths.ConfigPath)
	if err != nil {
		fail(os.Stderr, "load config", err, initialPaths.CurrentLogFile, "Fix state/config.json or remove it to regenerate defaults.")
		return 1
	}
	if cfg.DataDir == "" {
		cfg.DataDir = initialPaths.Root
	}
	paths, err := apppaths.Resolve(ctx, cfg.DataDir)
	if err != nil {
		fail(os.Stderr, "resolve app paths", err, initialPaths.CurrentLogFile, "Verify the configured data_dir is writable.")
		return 1
	}

	logger.Info("main", "starting command", map[string]any{"command": command})

	manifest, err := state.LoadManifest(ctx, paths.ManifestPath)
	if err != nil {
		fail(os.Stderr, "load manifest", err, paths.CurrentLogFile, "Check state/manifest.json permissions or remove the file to let i2tor recreate it.")
		return 1
	}
	manifest.AppVersion = appVersion
	manifest.OS = util.NormalizedOS()
	manifest.Arch = util.NormalizedArch()
	manifest.LauncherManagedI2P = rt.ReconcileManagedProcessRecord(manifest.LauncherManagedI2P)

	if err := config.Save(ctx, paths.ConfigPath, cfg); err != nil {
		fail(os.Stderr, "persist config", err, paths.CurrentLogFile, "Verify the state directory is writable.")
		return 1
	}

	switch command {
	case "run":
		if err := commandRun(ctx, logger, cfg, paths, &manifest); err != nil {
			fail(os.Stderr, "run launcher", err, paths.CurrentLogFile, "Review the log file for the failing step, then rerun `i2tor doctor`.")
			return 1
		}
	case "install":
		if err := commandInstall(ctx, logger, cfg, paths, &manifest); err != nil {
			fail(os.Stderr, "install dependencies", err, paths.CurrentLogFile, "Review the artifact URL, checksum source, and network connectivity.")
			return 1
		}
	case "status":
		if err := commandStatus(ctx, cfg, paths, manifest); err != nil {
			fail(os.Stderr, "show status", err, paths.CurrentLogFile, "Run `i2tor doctor` for more detail.")
			return 1
		}
	case "paths":
		commandPaths(paths)
	case "doctor":
		if err := commandDoctor(ctx, cfg, paths, manifest); err != nil {
			fail(os.Stderr, "run diagnostics", err, paths.CurrentLogFile, "Address the failing check reported above.")
			return 1
		}
	case "logs":
		commandLogs(paths)
	case "update":
		if err := commandUpdate(ctx, logger, cfg, paths, &manifest); err != nil {
			fail(os.Stderr, "update managed dependencies", err, paths.CurrentLogFile, "Review the log file for the failing download or verification step, then retry `i2tor update`.")
			return 1
		}
	case "gui":
		if err := commandNativeGUI(ctx, logger, cfg, paths, &manifest); err != nil {
			fail(os.Stderr, "launch native gui", err, paths.CurrentLogFile, "Review the log file for the failing step, then retry `i2tor gui`.")
			return 1
		}
	case "desktop":
		if err := commandNativeGUI(ctx, logger, cfg, paths, &manifest); err != nil {
			fail(os.Stderr, "launch desktop ui", err, paths.CurrentLogFile, "Review the log file for the failing step, then retry `i2tor desktop`.")
			return 1
		}
	case "uninstall":
		if err := commandUninstall(ctx, paths, manifest); err != nil {
			fail(os.Stderr, "uninstall managed files", err, paths.CurrentLogFile, "Stop i2tor, confirm no launcher-owned I2P process is running, then retry.")
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", command)
		return 2
	}

	if command == "uninstall" {
		return 0
	}

	if err := state.SaveManifest(ctx, paths.ManifestPath, manifest); err != nil {
		fail(os.Stderr, "save manifest", err, paths.CurrentLogFile, "Verify the state directory is writable.")
		return 1
	}
	return 0
}

func commandRun(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest) error {
	session, err := startLauncherSession(ctx, logger, cfg, paths, manifest, nil)
	if err != nil {
		return err
	}
	return waitForLauncherSession(ctx, logger, cfg, session, manifest)
}

func commandInstall(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest) error {
	_, _, _, err := ensureInstalls(ctx, logger, cfg, paths, manifest)
	return err
}

func ensureInstalls(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest) (install.InstalledApp, install.InstalledApp, install.InstalledApp, error) {
	torInstall, err := resolveTorBrowser(ctx, logger, cfg, paths)
	if err != nil {
		return install.InstalledApp{}, install.InstalledApp{}, install.InstalledApp{}, err
	}
	javaInstall, err := resolveJava(ctx, logger, paths)
	if err != nil {
		return install.InstalledApp{}, install.InstalledApp{}, install.InstalledApp{}, err
	}
	i2pInstall, err := resolveI2P(ctx, logger, cfg, paths, javaInstall)
	if err != nil {
		return install.InstalledApp{}, install.InstalledApp{}, install.InstalledApp{}, err
	}
	manifest.TorBrowser = state.InstallRecord{
		Source:              torInstall.Source,
		Version:             torInstall.Version,
		ArtifactURL:         torInstall.ArtifactURL,
		ArtifactPath:        torInstall.ArtifactPath,
		ChecksumSHA256:      torInstall.ChecksumSHA256,
		ChecksumVerified:    torInstall.Verified,
		SignatureVerified:   torInstall.SignatureVerified,
		ChecksumVerifiedAt:  time.Now().UTC(),
		SignatureVerifiedAt: time.Now().UTC(),
		InstalledAt:         time.Now().UTC(),
	}
	manifest.I2P = state.InstallRecord{
		Source:              i2pInstall.Source,
		Version:             i2pInstall.Version,
		ArtifactURL:         i2pInstall.ArtifactURL,
		ArtifactPath:        i2pInstall.ArtifactPath,
		ChecksumSHA256:      i2pInstall.ChecksumSHA256,
		ChecksumVerified:    i2pInstall.Verified,
		SignatureVerified:   i2pInstall.SignatureVerified,
		ChecksumVerifiedAt:  time.Now().UTC(),
		SignatureVerifiedAt: time.Now().UTC(),
		InstalledAt:         time.Now().UTC(),
	}
	manifest.Java = state.InstallRecord{
		Source:              javaInstall.Source,
		Version:             javaInstall.Version,
		ArtifactURL:         javaInstall.ArtifactURL,
		ArtifactPath:        javaInstall.ArtifactPath,
		ChecksumSHA256:      javaInstall.ChecksumSHA256,
		ChecksumVerified:    javaInstall.Verified,
		SignatureVerified:   javaInstall.SignatureVerified,
		ChecksumVerifiedAt:  time.Now().UTC(),
		SignatureVerifiedAt: time.Now().UTC(),
		InstalledAt:         time.Now().UTC(),
	}
	return torInstall, i2pInstall, javaInstall, nil
}

func resolveTorBrowser(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths) (install.InstalledApp, error) {
	if app, err := install.ReuseManagedTorBrowser(paths); err == nil {
		logger.Info("install", "using managed Tor Browser install", map[string]any{"path": app.InstallDir})
		return app, nil
	}
	if cfg.ReuseExistingTorBrowser {
		candidate, err := detect.DetectExistingTorBrowser(ctx)
		if err == nil {
			app := install.ReuseExistingTorBrowser(candidate)
			logger.Info("detect", "reusing existing Tor Browser install", map[string]any{"path": app.ExecutablePath, "hint": candidate.DetectionHint})
			return app, nil
		}
	}
	app, err := install.InstallManagedTorBrowser(ctx, paths)
	if err != nil {
		return install.InstalledApp{}, fmt.Errorf("ensure Tor Browser exists or install it: %w", err)
	}
	logger.Info("install", "installed managed Tor Browser", map[string]any{"path": app.InstallDir})
	return app, nil
}

func resolveJava(ctx context.Context, logger *logging.Logger, paths apppaths.AppPaths) (install.InstalledApp, error) {
	if app, err := install.ReuseManagedJava(paths); err == nil {
		logger.Info("install", "using managed Java runtime", map[string]any{"path": app.InstallDir})
		return app, nil
	}
	if candidate, err := detect.DetectExistingJava(ctx, 17); err == nil {
		app := install.ReuseExistingJava(candidate)
		logger.Info("detect", "reusing existing Java runtime", map[string]any{"path": app.ExecutablePath, "hint": candidate.DetectionHint})
		return app, nil
	}
	app, err := install.InstallManagedJava(ctx, paths)
	if err != nil {
		return install.InstalledApp{}, fmt.Errorf("ensure Java 17+ exists or install it: %w", err)
	}
	logger.Info("install", "installed managed Java runtime", map[string]any{"path": app.InstallDir})
	return app, nil
}

func resolveI2P(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, java install.InstalledApp) (install.InstalledApp, error) {
	if app, err := install.ReuseManagedI2P(paths); err == nil {
		logger.Info("install", "using managed I2P install", map[string]any{"path": app.InstallDir})
		return app, nil
	}
	if cfg.ReuseExistingI2P {
		candidate, err := detect.DetectExistingI2P(ctx)
		if err == nil {
			app := install.ReuseExistingI2P(candidate)
			logger.Info("detect", "reusing existing I2P install", map[string]any{"path": app.ExecutablePath, "hint": candidate.DetectionHint})
			return app, nil
		}
	}
	app, err := install.InstallManagedI2P(ctx, paths, java)
	if err != nil {
		return install.InstalledApp{}, fmt.Errorf("ensure I2P exists or install it: %w", err)
	}
	logger.Info("install", "installed managed I2P", map[string]any{"path": app.InstallDir})
	return app, nil
}

func commandStatus(ctx context.Context, cfg config.Config, paths apppaths.AppPaths, manifest state.Manifest) error {
	i2pReady := portReady("127.0.0.1:4444")
	torProxyReady := portReady("127.0.0.1:9150")
	ui.PrintStatus(os.Stdout, "i2tor status", map[string]string{
		"os_arch":            manifest.OS + "/" + manifest.Arch,
		"config_path":        paths.ConfigPath,
		"manifest_path":      paths.ManifestPath,
		"tor_browser_source": manifest.TorBrowser.Source,
		"i2p_source":         manifest.I2P.Source,
		"java_source":        manifest.Java.Source,
		"i2p_proxy_ready":    fmt.Sprintf("%t", i2pReady),
		"tor_socks_ready":    fmt.Sprintf("%t", torProxyReady),
		"reuse_existing_tor": fmt.Sprintf("%t", cfg.ReuseExistingTorBrowser),
		"reuse_existing_i2p": fmt.Sprintf("%t", cfg.ReuseExistingI2P),
	})
	_ = ctx
	return nil
}

func commandUpdate(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest) error {
	updates := []string{}
	if manifest.TorBrowser.Source == "" || manifest.TorBrowser.Source == "managed" {
		meta, err := install.LatestTorBrowserMetadata(ctx)
		if err != nil {
			return fmt.Errorf("resolve latest Tor Browser metadata: %w", err)
		}
		if manifest.TorBrowser.Version != meta.Version || manifest.TorBrowser.Source == "" {
			app, err := install.ReinstallManagedTorBrowser(ctx, paths)
			if err != nil {
				return fmt.Errorf("update Tor Browser to %s: %w", meta.Version, err)
			}
			logger.Info("install", "updated managed Tor Browser", map[string]any{"version": app.Version, "path": app.InstallDir})
			manifest.TorBrowser = installRecordFromApp(app)
			updates = append(updates, "Tor Browser "+meta.Version)
		}
	}
	if manifest.Java.Source == "" || manifest.Java.Source == "managed" {
		meta, err := install.LatestJavaMetadata(ctx)
		if err != nil {
			return fmt.Errorf("resolve latest Java metadata: %w", err)
		}
		if manifest.Java.Version != meta.Version || manifest.Java.Source == "" {
			app, err := install.ReinstallManagedJava(ctx, paths)
			if err != nil {
				return fmt.Errorf("update Java runtime to %s: %w", meta.Version, err)
			}
			logger.Info("install", "updated managed Java runtime", map[string]any{"version": app.Version, "path": app.InstallDir})
			manifest.Java = installRecordFromApp(app)
			updates = append(updates, "Java "+meta.Version)
		}
	}
	if manifest.I2P.Source == "" || manifest.I2P.Source == "managed" {
		javaInstall, err := resolveJava(ctx, logger, paths)
		if err != nil {
			return err
		}
		meta, err := install.LatestI2PMetadata(ctx)
		if err != nil {
			return fmt.Errorf("resolve latest I2P metadata: %w", err)
		}
		if manifest.I2P.Version != meta.Version || manifest.I2P.Source == "" {
			app, err := install.ReinstallManagedI2P(ctx, paths, javaInstall)
			if err != nil {
				return fmt.Errorf("update I2P to %s: %w", meta.Version, err)
			}
			logger.Info("install", "updated managed I2P", map[string]any{"version": app.Version, "path": app.InstallDir})
			manifest.I2P = installRecordFromApp(app)
			updates = append(updates, "I2P "+meta.Version)
		}
	}
	if len(updates) == 0 {
		fmt.Fprintln(os.Stdout, "all managed dependencies are already current")
		return nil
	}
	fmt.Fprintf(os.Stdout, "updated: %s\n", strings.Join(updates, ", "))
	_ = cfg
	return nil
}

func commandPaths(paths apppaths.AppPaths) {
	fmt.Fprintf(os.Stdout, "root: %s\n", paths.Root)
	fmt.Fprintf(os.Stdout, "downloads: %s\n", paths.DownloadsDir)
	fmt.Fprintf(os.Stdout, "runtime: %s\n", paths.RuntimeDir)
	fmt.Fprintf(os.Stdout, "tor-browser: %s\n", paths.TorBrowserRuntimeDir)
	fmt.Fprintf(os.Stdout, "i2p: %s\n", paths.I2PRuntimeDir)
	fmt.Fprintf(os.Stdout, "java: %s\n", paths.JavaRuntimeDir)
	fmt.Fprintf(os.Stdout, "profile: %s\n", paths.ProfileDir)
	fmt.Fprintf(os.Stdout, "pac: %s\n", paths.PACFile)
	fmt.Fprintf(os.Stdout, "logs: %s\n", paths.LogsDir)
	fmt.Fprintf(os.Stdout, "state: %s\n", paths.StateDir)
}

func commandDoctor(ctx context.Context, cfg config.Config, paths apppaths.AppPaths, manifest state.Manifest) error {
	commandStatus(ctx, cfg, paths, manifest)
	checks := map[string]bool{
		"PAC file exists":              fileExists(paths.PACFile),
		"Dedicated profile user.js":    fileExists(filepath.Join(paths.ProfileDir, "user.js")),
		"Manifest exists":              fileExists(paths.ManifestPath),
		"I2P port 4444 reachable":      portReady("127.0.0.1:4444"),
		"Tor Browser SOCKS 9150 ready": portReady("127.0.0.1:9150"),
	}
	for label, ok := range checks {
		fmt.Fprintf(os.Stdout, "%s: %t\n", label, ok)
	}
	printI2PDiagnostics(ctx, paths, manifest)
	return nil
}

func commandLogs(paths apppaths.AppPaths) {
	fmt.Fprintf(os.Stdout, "logs directory: %s\n", paths.LogsDir)
	if data, err := os.ReadFile(paths.CurrentLogFile); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		start := 0
		if len(lines) > 20 {
			start = len(lines) - 20
		}
		for _, line := range lines[start:] {
			fmt.Fprintln(os.Stdout, line)
		}
	}
}

func commandUninstall(ctx context.Context, paths apppaths.AppPaths, manifest state.Manifest) error {
	_ = ctx
	record := rt.ReconcileManagedProcessRecord(manifest.LauncherManagedI2P)
	if record.Owns && record.PID > 0 {
		return fmt.Errorf("launcher-owned I2P process still appears active with pid %d", record.PID)
	}

	root := filepath.Clean(paths.Root)
	if root == "" || root == "/" || root == "." {
		return fmt.Errorf("refusing to remove unsafe data dir %q", root)
	}

	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove managed data dir %q: %w", root, err)
	}
	fmt.Fprintf(os.Stdout, "removed managed i2tor data: %s\n", root)
	fmt.Fprintln(os.Stdout, "reused external Tor Browser, I2P, or Java installs were not modified.")
	return nil
}

func fail(w *os.File, step string, err error, logPath, nextStep string) {
	fmt.Fprintf(w, "step: %s\nerror: %v\nnext: %s\nlog: %s\n", step, err, nextStep, logPath)
}

func portReady(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func printI2PDiagnostics(ctx context.Context, paths apppaths.AppPaths, manifest state.Manifest) {
	fmt.Fprintln(os.Stdout, "I2P diagnostics:")

	if report, err := fetchI2PConsoleReport(ctx, "http://127.0.0.1:7657/"); err == nil {
		fmt.Fprintf(os.Stdout, "  Router console reachable: true\n")
		if report.Title != "" {
			fmt.Fprintf(os.Stdout, "  Console title: %s\n", report.Title)
		}
		if len(report.Indicators) > 0 {
			fmt.Fprintf(os.Stdout, "  Console indicators: %s\n", strings.Join(report.Indicators, ", "))
		}
		for _, warning := range report.Warnings {
			fmt.Fprintf(os.Stdout, "  Warning: %s\n", warning)
		}
	} else {
		fmt.Fprintf(os.Stdout, "  Router console reachable: false (%v)\n", err)
	}

	if manifest.LauncherManagedI2P.PID > 0 {
		fmt.Fprintf(os.Stdout, "  Manifest managed I2P PID: %d (owned=%t)\n", manifest.LauncherManagedI2P.PID, manifest.LauncherManagedI2P.Owns)
	}

	netdbCount := countFiles(filepath.Join(paths.I2PRuntimeDir, "netDb"))
	peerCount := countFiles(filepath.Join(paths.I2PRuntimeDir, "peerProfiles"))
	fmt.Fprintf(os.Stdout, "  netDb file count: %d\n", netdbCount)
	fmt.Fprintf(os.Stdout, "  peerProfiles file count: %d\n", peerCount)
	if udpPort, err := i2pUDPPort(filepath.Join(paths.I2PRuntimeDir, "router.config")); err == nil && udpPort != "" {
		fmt.Fprintf(os.Stdout, "  Configured I2P UDP port: %s\n", udpPort)
	}

	if lines, err := tailLines(filepath.Join(paths.I2PRuntimeDir, "eventlog.txt"), 5); err == nil && len(lines) > 0 {
		fmt.Fprintln(os.Stdout, "  Recent eventlog entries:")
		for _, line := range lines {
			fmt.Fprintf(os.Stdout, "    %s\n", line)
		}
	}
	if lines, err := tailLines(filepath.Join(paths.I2PRuntimeDir, "wrapper.log"), 8); err == nil && len(lines) > 0 {
		fmt.Fprintln(os.Stdout, "  Recent wrapper log lines:")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(os.Stdout, "    %s\n", line)
		}
	}
}

type i2pConsoleReport struct {
	Title      string
	Indicators []string
	Warnings   []string
}

func fetchI2PConsoleReport(ctx context.Context, url string) (i2pConsoleReport, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return i2pConsoleReport{}, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return i2pConsoleReport{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return i2pConsoleReport{}, fmt.Errorf("unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return i2pConsoleReport{}, err
	}
	return summarizeI2PConsoleHTML(string(body)), nil
}

var titlePattern = regexp.MustCompile(`(?is)<title>(.*?)</title>`)

func summarizeI2PConsoleHTML(html string) i2pConsoleReport {
	report := i2pConsoleReport{}
	if match := titlePattern.FindStringSubmatch(html); len(match) == 2 {
		report.Title = strings.TrimSpace(stripHTML(match[1]))
	}
	lower := strings.ToLower(html)
	indicators := []struct {
		needle string
		label  string
	}{
		{"clock skew", "clock-skew warning visible"},
		{"firewalled", "firewalled status mentioned"},
		{"ntcp2", "NTCP2 mentioned"},
		{"ssu2", "SSU2 mentioned"},
		{"hidden mode", "hidden mode mentioned"},
		{"floodfill", "floodfill mentioned"},
		{"reseed", "reseed mentioned"},
		{"rejecting tunnels", "rejecting tunnels mentioned"},
		{"network database", "network database section present"},
		{"peer", "peer information present"},
	}
	for _, indicator := range indicators {
		if strings.Contains(lower, indicator.needle) {
			report.Indicators = append(report.Indicators, indicator.label)
		}
	}
	if strings.Contains(lower, "firewalled") {
		report.Warnings = append(report.Warnings, "Router console reports firewalled reachability. I2P eepsite access may be unreliable until UDP reachability improves.")
	}
	if strings.Contains(lower, "clock skew") {
		report.Warnings = append(report.Warnings, "Router console reports clock skew. Incorrect system time can break I2P routing.")
	}
	return report
}

func stripHTML(s string) string {
	inTag := false
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func countFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func tailLines(path string, n int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) <= n {
		return lines, nil
	}
	return lines[len(lines)-n:], nil
}

func i2pUDPPort(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "i2np.udp.port=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "i2np.udp.port=")), nil
		}
	}
	return "", nil
}

func installRecordFromApp(app install.InstalledApp) state.InstallRecord {
	now := time.Now().UTC()
	return state.InstallRecord{
		Source:              app.Source,
		Version:             app.Version,
		ArtifactURL:         app.ArtifactURL,
		ArtifactPath:        app.ArtifactPath,
		ChecksumSHA256:      app.ChecksumSHA256,
		ChecksumVerified:    app.Verified,
		ChecksumVerifiedAt:  now,
		SignatureVerified:   app.SignatureVerified,
		SignatureVerifiedAt: now,
		InstalledAt:         now,
	}
}

func openBrowser(url string) error {
	return openPath(url)
}

func openPath(target string) error {
	var cmd *exec.Cmd
	switch util.NormalizedOS() {
	case "linux":
		cmd = exec.Command("xdg-open", target)
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return fmt.Errorf("unsupported desktop open platform %q", util.NormalizedOS())
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start desktop open command: %w", err)
	}
	return nil
}

