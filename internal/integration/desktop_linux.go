//go:build linux

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const desktopTemplate = `[Desktop Entry]
Type=Application
Version=1.0
Name=i2tor
Comment=Launch Tor Browser with I2P routing for .i2p sites
Exec=%s
Icon=%s
Terminal=false
Categories=Network;System;
StartupNotify=true
`

func EnsureDesktopEntry(ctx context.Context, executablePath, iconSourcePath string) error {
	_ = ctx
	if executablePath == "" {
		return fmt.Errorf("missing executable path for desktop integration")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory for desktop integration: %w", err)
	}

	applicationsDir := filepath.Join(home, ".local", "share", "applications")
	iconsDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "512x512", "apps")
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		return fmt.Errorf("create applications directory %q: %w", applicationsDir, err)
	}
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return fmt.Errorf("create icons directory %q: %w", iconsDir, err)
	}

	iconTargetPath := filepath.Join(iconsDir, "i2tor.png")
	if iconSourcePath != "" {
		data, err := os.ReadFile(iconSourcePath)
		if err != nil {
			return fmt.Errorf("read icon source %q: %w", iconSourcePath, err)
		}
		if err := os.WriteFile(iconTargetPath, data, 0o644); err != nil {
			return fmt.Errorf("write icon target %q: %w", iconTargetPath, err)
		}
	}

	execValue := shellQuote(executablePath)
	if strings.HasSuffix(strings.ToLower(executablePath), ".appimage") {
		execValue = execValue + " gui"
	}
	entryPath := filepath.Join(applicationsDir, "i2tor.desktop")
	entryContent := fmt.Sprintf(desktopTemplate, execValue, "i2tor")
	if err := os.WriteFile(entryPath, []byte(entryContent), 0o644); err != nil {
		return fmt.Errorf("write desktop entry %q: %w", entryPath, err)
	}

	_ = exec.Command("update-desktop-database", applicationsDir).Run()
	return nil
}

func ResolveDesktopExecutablePath() (string, error) {
	if appImage := os.Getenv("APPIMAGE"); appImage != "" {
		return appImage, nil
	}
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	return path, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
