package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"i2tor/internal/apppaths"
	"i2tor/internal/install"
	"i2tor/internal/logging"
)

func StartI2P(ctx context.Context, logger *logging.Logger, i2pInstall, javaInstall install.InstalledApp, paths apppaths.AppPaths) (ManagedProcess, error) {
	if err := install.NormalizeManagedI2PPortableConfig(i2pInstall.InstallDir, javaInstall); err != nil {
		return ManagedProcess{}, fmt.Errorf("normalize managed I2P install: %w", err)
	}
	if err := ensureManagedI2PClientConfig(i2pInstall.InstallDir); err != nil {
		return ManagedProcess{}, fmt.Errorf("prepare managed I2P client config: %w", err)
	}

	commandPath, commandArgs, launcherPath, err := managedI2PLaunchCommand(i2pInstall)
	if err != nil {
		return ManagedProcess{}, err
	}

	if err := os.Remove(filepath.Join(i2pInstall.InstallDir, "i2p.pid")); err != nil && !os.IsNotExist(err) {
		return ManagedProcess{}, fmt.Errorf("clear stale I2P pid file: %w", err)
	}

	cmd := exec.CommandContext(ctx, commandPath, commandArgs...)
	cmd.Dir = i2pInstall.InstallDir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if logger != nil {
		logger.Info("i2p", "i2prouter start output", map[string]any{
			"command":  commandPath,
			"args":     commandArgs,
			"launcher": launcherPath,
			"output":   string(output),
		})
	}
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("run managed I2P launcher %q via %q %v: %w: %s", launcherPath, commandPath, commandArgs, err, string(output))
	}

	pid, err := waitForPIDFile(ctx, filepath.Join(i2pInstall.InstallDir, "i2p.pid"), 10*time.Second)
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("read managed I2P pid file after startup: %w", err)
	}

	return ManagedProcess{
		Name:      "i2p",
		Command:   launcherPath,
		Args:      []string{"start"},
		PID:       pid,
		StartedAt: time.Now().UTC(),
		Owned:     true,
	}, nil
}

func ensureManagedI2PClientConfig(installDir string) error {
	clientsConfig := filepath.Join(installDir, "clients.config")
	if _, err := os.Stat(clientsConfig); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %q: %w", clientsConfig, err)
	}
	data, err := os.ReadFile(clientsConfig)
	if err != nil {
		return fmt.Errorf("read %q: %w", clientsConfig, err)
	}
	content := string(data)
	content = strings.ReplaceAll(content, "clientApp.3.startOnLoad=true", "clientApp.3.startOnLoad=false")
	content = strings.ReplaceAll(content, "clientApp.4.startOnLoad=true", "clientApp.4.startOnLoad=false")
	if err := os.WriteFile(clientsConfig, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", clientsConfig, err)
	}
	return nil
}

func ShutdownManagedI2P(ctx context.Context, proc ManagedProcess) error {
	if !proc.Owned {
		return nil
	}
	if isI2PRouterLauncher(proc.Command) {
		stopCommand, stopArgs := managedI2PStopCommand(proc.Command)
		cmd := exec.CommandContext(ctx, stopCommand, stopArgs...)
		cmd.Dir = filepath.Dir(proc.Command)
		if output, err := cmd.CombinedOutput(); err != nil {
			if proc.PID > 0 {
				if killErr := terminatePID(proc.PID); killErr == nil {
					return nil
				}
			}
			return fmt.Errorf("stop managed I2P with %q %v: %w: %s", stopCommand, stopArgs, err, string(output))
		}
		return nil
	}
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		if proc.PID <= 0 {
			return nil
		}
		if err := terminatePID(proc.PID); err != nil {
			return fmt.Errorf("terminate I2P pid %d: %w", proc.PID, err)
		}
		return nil
	}
	if err := proc.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("terminate I2P pid %d: %w", proc.PID, err)
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

func managedI2PLaunchCommand(i2pInstall install.InstalledApp) (string, []string, string, error) {
	launcherPath, err := i2pInstall.ResolveExecutable()
	if err != nil {
		return "", nil, "", fmt.Errorf("resolve managed I2P launcher in %q: %w", i2pInstall.InstallDir, err)
	}
	if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(launcherPath), ".bat") {
		return "cmd", []string{"/C", launcherPath, "start"}, launcherPath, nil
	}
	return launcherPath, []string{"start"}, launcherPath, nil
}

func managedI2PStopCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(command), ".bat") {
		return "cmd", []string{"/C", command, "stop"}
	}
	return command, []string{"stop"}
}

func isI2PRouterLauncher(command string) bool {
	base := strings.ToLower(filepath.Base(command))
	return base == "i2prouter" || base == "i2prouter.bat"
}

func waitForPIDFile(ctx context.Context, pidFile string, timeout time.Duration) (int, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			var pid int
			if _, scanErr := fmt.Sscanf(string(data), "%d", &pid); scanErr == nil && pid > 0 {
				return pid, nil
			}
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-deadline.C:
			return 0, fmt.Errorf("pid file %s did not appear within %s", pidFile, timeout)
		case <-ticker.C:
		}
	}
}
