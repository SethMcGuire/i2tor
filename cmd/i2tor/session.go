package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/logging"
	"i2tor/internal/pac"
	"i2tor/internal/profile"
	rt "i2tor/internal/runtime"
	"i2tor/internal/state"
)

type progressReporter func(stage, detail string, value float64)

const (
	i2pReadyTimeout    = 90 * time.Second
	torReadyTimeout    = 30 * time.Second
	profileDisplayName = "i2tor Dedicated Profile"
)

type launcherSession struct {
	Browser rt.ManagedProcess
	Tor     rt.ManagedProcess
	I2P     rt.ManagedProcess
}

func startLauncherSession(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest, report progressReporter) (launcherSession, error) {
	step := func(stage, detail string, value float64) {
		if report != nil {
			report(stage, detail, value)
		}
	}

	step("Checking dependencies", "Reusing or installing Tor Browser, Java, and I2P as needed.", 0.1)
	torInstall, i2pInstall, javaInstall, err := ensureInstalls(ctx, logger, cfg, paths, manifest)
	if err != nil {
		return launcherSession{}, err
	}

	step("Starting I2P", "Launching the managed I2P router.", 0.25)
	i2pProc, err := rt.StartI2P(ctx, logger, i2pInstall, javaInstall, paths)
	if err != nil {
		return launcherSession{}, fmt.Errorf("failed to start I2P from %s: %w", i2pInstall.InstallDir, err)
	}
	manifest.LauncherManagedI2P = state.ManagedProcessRecord{
		PID:       i2pProc.PID,
		StartedAt: i2pProc.StartedAt,
		Command:   i2pProc.Command,
		Args:      i2pProc.Args,
		Owns:      true,
	}

	step("Waiting for I2P", "Checking the local I2P HTTP proxy on 127.0.0.1:4444. First startup can take a while.", 0.4)
	if err := rt.WaitForI2PReady(ctx, "127.0.0.1:4444", i2pReadyTimeout); err != nil {
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("I2P readiness check failed: %w", err)
	}

	step("Starting Tor", "Launching bundled Tor from the Tor Browser install.", 0.55)
	torProc, err := rt.StartBundledTor(ctx, logger, torInstall, paths)
	if err != nil {
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("failed to start bundled Tor from %s: %w", torInstall.InstallDir, err)
	}

	step("Waiting for Tor", "Checking the Tor SOCKS listener on 127.0.0.1:9150.", 0.7)
	if err := rt.WaitForTorReady(ctx, "127.0.0.1:9150", torReadyTimeout); err != nil {
		_ = rt.ShutdownManagedTor(context.Background(), torProc)
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("Tor readiness check failed: %w", err)
	}

	step("Writing browser profile", "Generating PAC and dedicated Tor Browser profile settings.", 0.82)
	if err := pac.WritePACFile(ctx, paths.PACFile); err != nil {
		_ = rt.ShutdownManagedTor(context.Background(), torProc)
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("failed to write PAC file at %s: %w", paths.PACFile, err)
	}
	startPagePath, err := profile.WriteStartPage(ctx, filepath.Join(paths.ProfileDir, "start-page"), profile.Options{ProfileDisplayName: profileDisplayName})
	if err != nil {
		_ = rt.ShutdownManagedTor(context.Background(), torProc)
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("failed to write managed start page in %s: %w", paths.ProfileDir, err)
	}
	if err := profile.WriteDedicatedProfilePrefs(ctx, paths.ProfileDir, paths.PACFile, profile.Options{
		AllowLocalhostAccess: cfg.AllowLocalhostAccess,
		ProfileDisplayName:   profileDisplayName,
		HomePagePath:         startPagePath,
	}); err != nil {
		_ = rt.ShutdownManagedTor(context.Background(), torProc)
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("failed to write dedicated profile prefs in %s: %w", paths.ProfileDir, err)
	}

	step("Launching browser", "Opening Tor Browser with the dedicated launcher-owned profile.", 0.95)
	browserProc, err := rt.LaunchTorBrowser(ctx, logger, torInstall, paths.ProfileDir, paths)
	if err != nil {
		_ = rt.ShutdownManagedTor(context.Background(), torProc)
		_ = rt.ShutdownManagedI2P(context.Background(), i2pProc)
		return launcherSession{}, fmt.Errorf("failed to launch Tor Browser executable %s: %w", torInstall.ExecutablePath, err)
	}

	manifest.GeneratedPACPath = paths.PACFile
	manifest.DedicatedProfilePath = paths.ProfileDir
	return launcherSession{Browser: browserProc, Tor: torProc, I2P: i2pProc}, nil
}

func waitForLauncherSession(ctx context.Context, logger *logging.Logger, session launcherSession, manifest *state.Manifest) error {
	err := rt.Wait(ctx, session.Browser)
	if err != nil {
		logger.Warn("torbrowser", "browser exited with error", map[string]any{"error": err.Error()})
	}

	if shutdownErr := rt.ShutdownManagedTor(context.Background(), session.Tor); shutdownErr != nil {
		_ = rt.ShutdownManagedI2P(context.Background(), session.I2P)
		return fmt.Errorf("shutdown managed Tor pid %d: %w", session.Tor.PID, shutdownErr)
	}
	if shutdownErr := rt.ShutdownManagedI2P(context.Background(), session.I2P); shutdownErr != nil {
		return fmt.Errorf("shutdown managed I2P pid %d: %w", session.I2P.PID, shutdownErr)
	}

	manifest.LauncherManagedI2P = state.ManagedProcessRecord{}
	manifest.LastSuccessfulLaunchAt = time.Now().UTC()
	return nil
}
