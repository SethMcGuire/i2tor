package main

import (
	"context"
	"fmt"
	"strings"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/detect"
	"i2tor/internal/install"
	"i2tor/internal/state"
)

type startupAssessment struct {
	NeedsInstall     bool
	UpdateAvailable  bool
	AutoStart        bool
	PrimaryMessage   string
	SecondaryMessage string
	UpdateSummary    string
}

func assessStartup(ctx context.Context, cfg config.Config, paths apppaths.AppPaths, manifest state.Manifest) startupAssessment {
	torReady := managedOrExistingTorAvailable(ctx, cfg, paths)
	javaReady := managedOrExistingJavaAvailable(ctx, paths)
	i2pReady := managedOrExistingI2PAvailable(ctx, cfg, paths)

	if !(torReady && javaReady && i2pReady) {
		missing := make([]string, 0, 3)
		if !torReady {
			missing = append(missing, "Tor Browser")
		}
		if !javaReady {
			missing = append(missing, "Java")
		}
		if !i2pReady {
			missing = append(missing, "I2P")
		}
		return startupAssessment{
			NeedsInstall:     true,
			PrimaryMessage:   "Install required before launch",
			SecondaryMessage: "Missing dependencies: " + strings.Join(missing, ", "),
		}
	}

	if cfg.AutoCheckUpdates {
		updates := availableManagedUpdates(ctx, manifest)
		if len(updates) > 0 {
			return startupAssessment{
				UpdateAvailable:  true,
				PrimaryMessage:   "Updates are available",
				SecondaryMessage: "You can update now or start with the current managed versions.",
				UpdateSummary:    strings.Join(updates, ", "),
			}
		}
	}

	return startupAssessment{
		AutoStart:        cfg.AutoStartOnLaunch,
		PrimaryMessage:   "Everything is ready",
		SecondaryMessage: "Dependencies are ready. Start when you are ready, or enable auto-start for future launches.",
	}
}

func managedOrExistingTorAvailable(ctx context.Context, cfg config.Config, paths apppaths.AppPaths) bool {
	if _, err := install.ReuseManagedTorBrowser(paths); err == nil {
		return true
	}
	if !cfg.ReuseExistingTorBrowser {
		return false
	}
	_, err := detect.DetectExistingTorBrowser(ctx)
	return err == nil
}

func managedOrExistingJavaAvailable(ctx context.Context, paths apppaths.AppPaths) bool {
	if _, err := install.ReuseManagedJava(paths); err == nil {
		return true
	}
	_, err := detect.DetectExistingJava(ctx, 17)
	return err == nil
}

func managedOrExistingI2PAvailable(ctx context.Context, cfg config.Config, paths apppaths.AppPaths) bool {
	if _, err := install.ReuseManagedI2P(paths); err == nil {
		return true
	}
	if !cfg.ReuseExistingI2P {
		return false
	}
	_, err := detect.DetectExistingI2P(ctx)
	return err == nil
}

func availableManagedUpdates(ctx context.Context, manifest state.Manifest) []string {
	updates := make([]string, 0, 3)

	if manifest.TorBrowser.Source == "managed" {
		if meta, err := install.LatestTorBrowserMetadata(ctx); err == nil && meta.Version != "" && meta.Version != manifest.TorBrowser.Version {
			updates = append(updates, fmt.Sprintf("Tor Browser %s", meta.Version))
		}
	}
	if manifest.Java.Source == "managed" {
		if meta, err := install.LatestJavaMetadata(ctx); err == nil && meta.Version != "" && meta.Version != manifest.Java.Version {
			updates = append(updates, fmt.Sprintf("Java %s", meta.Version))
		}
	}
	if manifest.I2P.Source == "managed" {
		if meta, err := install.LatestI2PMetadata(ctx); err == nil && meta.Version != "" && meta.Version != manifest.I2P.Version {
			updates = append(updates, fmt.Sprintf("I2P %s", meta.Version))
		}
	}
	return updates
}
