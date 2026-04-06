//go:build linux

package main

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/integration"
	"i2tor/internal/logging"
	"i2tor/internal/state"
)

func commandNativeGUI(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest) error {
	if executablePath, err := integration.ResolveDesktopExecutablePath(); err == nil {
		if err := integration.EnsureDesktopEntry(ctx, executablePath, "i2tor.png"); err != nil {
			logger.Warn("main", "failed to install per-user desktop entry", map[string]any{"error": err.Error()})
		}
	}

	a := app.NewWithID("org.i2tor.launcher")
	w := a.NewWindow("i2tor")
	w.Resize(fyne.NewSize(760, 520))

	title := widget.NewRichTextFromMarkdown("# i2tor")
	subtitle := widget.NewLabel("Checking installs, updates, and runtime readiness.")
	subtitle.Wrapping = fyne.TextWrapWord
	detail := widget.NewLabel("Starting assessment.")
	detail.Wrapping = fyne.TextWrapWord
	status := widget.NewLabel("Idle")
	status.Wrapping = fyne.TextWrapWord
	stageLabel := widget.NewLabel("Waiting")
	stageLabel.Wrapping = fyne.TextWrapWord
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1
	progress.SetValue(0)

	installButton := widget.NewButtonWithIcon("Install", theme.DownloadIcon(), nil)
	updateButton := widget.NewButtonWithIcon("Update", theme.ViewRefreshIcon(), nil)
	startButton := widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), nil)
	startNowButton := widget.NewButtonWithIcon("Start Current Version", theme.MediaPlayIcon(), nil)
	logsButton := widget.NewButtonWithIcon("Open Logs", theme.FolderOpenIcon(), func() {
		if err := openPath(paths.LogsDir); err != nil {
			showErrorDialog(w, err)
		}
	})
	quitButton := widget.NewButtonWithIcon("Quit", theme.CancelIcon(), func() {
		w.Close()
	})
	localhostWarning := widget.NewLabel("Advanced: enables access to localhost in the launcher-owned Tor Browser profile. Off by default because it weakens local-service isolation.")
	localhostWarning.Wrapping = fyne.TextWrapWord
	localhostToggle := widget.NewCheck("Allow localhost access", nil)
	localhostToggle.Checked = cfg.AllowLocalhostAccess
	localhostToggle.OnChanged = func(enabled bool) {
		nextCfg := cfg
		nextCfg.AllowLocalhostAccess = enabled
		if err := config.Save(context.Background(), paths.ConfigPath, nextCfg); err != nil {
			localhostToggle.SetChecked(cfg.AllowLocalhostAccess)
			showErrorDialog(w, fmt.Errorf("save localhost access setting: %w", err))
			return
		}
		cfg = nextCfg
		if enabled {
			detail.SetText("Localhost access enabled for future launches of the dedicated i2tor profile. Restart the browser session for it to take effect.")
		} else {
			detail.SetText("Localhost access disabled. Restart the browser session for the stricter profile settings to take effect.")
		}
	}
	advancedSection := widget.NewCard(
		"Advanced",
		"These settings apply only to the dedicated i2tor profile.",
		container.NewVBox(localhostToggle, localhostWarning),
	)

	actionRow := container.NewHBox(installButton, updateButton, startButton, startNowButton, logsButton, layout.NewSpacer(), quitButton)
	content := container.NewBorder(
		container.NewVBox(title, subtitle),
		actionRow,
		nil,
		nil,
		container.NewVBox(
			widget.NewSeparator(),
			status,
			stageLabel,
			progress,
			detail,
			widget.NewSeparator(),
			advancedSection,
		),
	)
	w.SetContent(container.NewPadded(content))

	setState := func(primary, secondary, stateText, stage string, progressValue float64, showInstall, showUpdate, showStart, showStartNow, busy bool) {
		subtitle.SetText(primary)
		detail.SetText(secondary)
		status.SetText(stateText)
		stageLabel.SetText(stage)
		progress.SetValue(progressValue)
		toggleButton(installButton, showInstall, busy)
		toggleButton(updateButton, showUpdate, busy)
		toggleButton(startButton, showStart, busy)
		toggleButton(startNowButton, showStartNow, busy)
		logsButton.Enable()
		quitButton.Enable()
	}

	runInBackground := func(busyTitle, busyDetail string, fn func(context.Context) error, onSuccess func()) {
		setState(busyTitle, busyDetail, "Working", "Processing", 0.2, false, false, false, false, true)
		go func() {
			actionCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := fn(actionCtx)
			if err != nil {
				setState("Action failed", err.Error(), "Error", "Failed", 1, true, true, true, true, false)
				showErrorDialog(w, err)
				return
			}
			if onSuccess != nil {
				onSuccess()
			}
		}()
	}

	reassess := func(triggerAutoStart bool) {
		go func() {
			assessment := assessStartup(context.Background(), cfg, paths, *manifest)
			switch {
			case assessment.NeedsInstall:
				setState(assessment.PrimaryMessage, assessment.SecondaryMessage, "Install Needed", "Waiting for install", 0, true, false, false, false, false)
			case assessment.UpdateAvailable:
				setState(assessment.PrimaryMessage, assessment.UpdateSummary, "Update Available", "Waiting for update decision", 0, false, true, false, true, false)
			default:
				setState(assessment.PrimaryMessage, assessment.SecondaryMessage, "Ready", "Ready to launch", 1, false, false, true, false, false)
				if triggerAutoStart && assessment.AutoStart {
					startButton.OnTapped()
				}
			}
		}()
	}

	installButton.OnTapped = func() {
		runInBackground("Installing dependencies", "Downloading and verifying Tor Browser, Java, and I2P if needed.", func(actionCtx context.Context) error {
			if err := commandInstall(actionCtx, logger, cfg, paths, manifest); err != nil {
				return err
			}
			return state.SaveManifest(actionCtx, paths.ManifestPath, *manifest)
		}, func() {
			setState("Install completed", "Managed dependencies are ready.", "Installed", "Ready to launch", 1, false, false, true, false, false)
			reassess(true)
		})
	}

	updateButton.OnTapped = func() {
		runInBackground("Updating managed dependencies", "Fetching release metadata, verifying signatures, and replacing managed installs.", func(actionCtx context.Context) error {
			if err := commandUpdate(actionCtx, logger, cfg, paths, manifest); err != nil {
				return err
			}
			return state.SaveManifest(actionCtx, paths.ManifestPath, *manifest)
		}, func() {
			setState("Update completed", "Managed dependencies are current.", "Updated", "Ready to launch", 1, false, false, true, false, false)
			reassess(true)
		})
	}

	startAction := func(mode string) {
		setState("Starting Tor Browser", "Mode: "+mode, "Starting", "Preparing launcher session", 0.05, false, false, false, false, true)
		go func() {
			runManifest := *manifest
			session, err := startLauncherSession(context.Background(), logger, cfg, paths, &runManifest, func(stage, detail string, value float64) {
				setState("Starting Tor Browser", detail, "Starting", stage, value, false, false, false, false, true)
			})
			if err != nil {
				setState("Action failed", err.Error(), "Error", "Failed", 1, true, true, true, true, false)
				showErrorDialog(w, err)
				return
			}
			*manifest = runManifest
			_ = state.SaveManifest(context.Background(), paths.ManifestPath, *manifest)
			setState("Browser launched", "Tor Browser is open. The launcher will keep running in the background until the browser exits.", "Running", "Browser launched", 1, false, false, false, false, true)
			w.Hide()
			go func() {
				waitManifest := *manifest
				err := waitForLauncherSession(context.Background(), logger, session, &waitManifest)
				if err == nil {
					*manifest = waitManifest
					_ = state.SaveManifest(context.Background(), paths.ManifestPath, *manifest)
				}
				a.Quit()
			}()
		}()
	}

	startButton.OnTapped = func() { startAction("normal") }
	startNowButton.OnTapped = func() { startAction("skip update") }

	w.SetCloseIntercept(func() {
		w.Close()
		a.Quit()
	})

	setState("Preparing desktop launcher", "Collecting status from the managed data directory.", "Assessing", "Assessing startup state", 0.05, false, false, false, false, true)
	go func() {
		time.Sleep(150 * time.Millisecond)
		reassess(true)
	}()

	w.ShowAndRun()
	_ = ctx
	return nil
}

func toggleButton(btn *widget.Button, visible, disabled bool) {
	if visible {
		btn.Show()
		if disabled {
			btn.Disable()
		} else {
			btn.Enable()
		}
		return
	}
	btn.Hide()
}

func showErrorDialog(w fyne.Window, err error) {
	dialog.ShowError(err, w)
}
