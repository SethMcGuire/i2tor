package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"i2tor/internal/apppaths"
	"i2tor/internal/install"
	"i2tor/internal/logging"
)

func StartI2P(ctx context.Context, logger *logging.Logger, i2pInstall, javaInstall install.InstalledApp, paths apppaths.AppPaths) (ManagedProcess, error) {
	javaPath, err := javaInstall.ResolveExecutable()
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("resolve Java executable for I2P: %w", err)
	}
	if err := install.NormalizeManagedI2PPortableConfig(i2pInstall.InstallDir, javaInstall); err != nil {
		return ManagedProcess{}, fmt.Errorf("normalize managed I2P install: %w", err)
	}
	if err := ensureManagedI2PClientConfig(i2pInstall.InstallDir); err != nil {
		return ManagedProcess{}, fmt.Errorf("prepare managed I2P client config: %w", err)
	}

	glob := filepath.Join(i2pInstall.InstallDir, "lib", "*.jar")
	jars, err := filepath.Glob(glob)
	if err != nil || len(jars) == 0 {
		return ManagedProcess{}, fmt.Errorf("resolve I2P classpath from %s", glob)
	}
	classpath := strings.Join(jars, string(os.PathListSeparator))
	args := []string{
		"-Djava.awt.headless=true",
		"-DloggerFilenameOverride=logs/log-router-@.txt",
		"-Di2p.dir.base=" + i2pInstall.InstallDir,
		"-Di2p.dir.config=" + i2pInstall.InstallDir,
		"-Di2p.dir.pid=" + i2pInstall.InstallDir,
		"-Di2p.dir.temp=" + i2pInstall.InstallDir,
		"-cp", classpath,
		"net.i2p.router.RouterLaunch",
	}
	return startCommandWithOptions(ctx, logger, "i2p", "i2p", javaPath, args, i2pInstall.InstallDir, os.Environ())
}

func ensureManagedI2PClientConfig(installDir string) error {
	clientsConfig := filepath.Join(installDir, "clients.config")
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
	if !proc.Owned || proc.Cmd == nil || proc.Cmd.Process == nil {
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
