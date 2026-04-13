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
	w.Resize(fyne.NewSize(820, 760))

	title := widget.NewRichTextFromMarkdown("# i2tor")
	tagline := widget.NewLabel("Launch Tor Browser with a dedicated I2P-aware profile.")
	tagline.Wrapping = fyne.TextWrapWord
	intro := widget.NewLabel("Install missing components, update managed ones, and change launcher settings in one place.")
	intro.Wrapping = fyne.TextWrapWord

	statusLine := widget.NewLabel("Status: Assessing")
	statusLine.TextStyle = fyne.TextStyle{Bold: true}
	primaryTitle := widget.NewLabel("Preparing desktop launcher")
	primaryTitle.TextStyle = fyne.TextStyle{Bold: true}
	primaryTitle.Wrapping = fyne.TextWrapWord
	primaryBody := widget.NewLabel("Collecting status from the managed data directory.")
	primaryBody.Wrapping = fyne.TextWrapWord
	stageLine := widget.NewLabel("Assessing startup state")
	stageLine.Wrapping = fyne.TextWrapWord
	progress := widget.NewProgressBar()
	progress.Min = 0
	progress.Max = 1
	progress.SetValue(0.05)

	torStatus := widget.NewLabel("Tor Browser: checking")
	torStatus.Wrapping = fyne.TextWrapWord
	javaStatus := widget.NewLabel("Java 17+: checking")
	javaStatus.Wrapping = fyne.TextWrapWord
	i2pStatus := widget.NewLabel("I2P: checking")
	i2pStatus.Wrapping = fyne.TextWrapWord
	updateStatus := widget.NewLabel("Updates: checking")
	updateStatus.Wrapping = fyne.TextWrapWord

	installButton := widget.NewButtonWithIcon("Install Missing Components", theme.DownloadIcon(), nil)
	installButton.Importance = widget.HighImportance
	updateButton := widget.NewButtonWithIcon("Update Managed Components", theme.ViewRefreshIcon(), nil)
	startButton := widget.NewButtonWithIcon("Start Browser", theme.MediaPlayIcon(), nil)
	startButton.Importance = widget.HighImportance
	startNowButton := widget.NewButtonWithIcon("Start Without Updating", theme.MediaFastForwardIcon(), nil)
	logsButton := widget.NewButtonWithIcon("Open Logs Folder", theme.FolderOpenIcon(), func() {
		if err := openPath(paths.LogsDir); err != nil {
			showErrorDialog(w, err)
		}
	})
	if !cfg.EnableLogging {
		logsButton.Hide()
	}
	dataDirButton := widget.NewButtonWithIcon("Open Data Folder", theme.FolderOpenIcon(), func() {
		if err := openPath(paths.Root); err != nil {
			showErrorDialog(w, err)
		}
	})
	quitButton := widget.NewButtonWithIcon("Quit", theme.CancelIcon(), func() {
		w.Close()
	})

	lastSettingMessage := widget.NewLabel("No pending config changes.")
	lastSettingMessage.Wrapping = fyne.TextWrapWord
	lastSettingMessage.Importance = widget.MediumImportance
	localhostWarning := widget.NewLabel("Allowing localhost access weakens Tor Browser's default local-service isolation. Leave it off unless you need loopback services in the dedicated i2tor profile.")
	localhostWarning.Wrapping = fyne.TextWrapWord

	autoUpdateToggle := widget.NewCheck("Check for managed updates on startup", nil)
	autoUpdateToggle.Checked = cfg.AutoCheckUpdates
	autoStartToggle := widget.NewCheck("Start automatically when everything is already ready", nil)
	autoStartToggle.Checked = cfg.AutoStartOnLaunch
	reuseTorToggle := widget.NewCheck("Reuse an existing Tor Browser install when found", nil)
	reuseTorToggle.Checked = cfg.ReuseExistingTorBrowser
	reuseI2PToggle := widget.NewCheck("Reuse an existing I2P install when found", nil)
	reuseI2PToggle.Checked = cfg.ReuseExistingI2P
	localhostToggle := widget.NewCheck("Allow localhost access in the dedicated profile", nil)
	localhostToggle.Checked = cfg.AllowLocalhostAccess
	keepI2PRunningToggle := widget.NewCheck("Keep managed I2P running after the browser closes", nil)
	keepI2PRunningToggle.Checked = cfg.KeepI2PRunning
	enableLoggingToggle := widget.NewCheck("Enable diagnostic logging to disk", nil)
	enableLoggingToggle.Checked = cfg.EnableLogging

	saveConfigField := func(field string, update func(*config.Config), revert func()) func(bool) {
		return func(_ bool) {
			nextCfg := cfg
			update(&nextCfg)
			if err := config.Save(context.Background(), paths.ConfigPath, nextCfg); err != nil {
				revert()
				showErrorDialog(w, fmt.Errorf("save %s setting: %w", field, err))
				return
			}
			cfg = nextCfg
			lastSettingMessage.SetText(fmt.Sprintf("%s updated. This affects future launches.", field))
		}
	}

	autoUpdateToggle.OnChanged = saveConfigField("Auto-update checks", func(next *config.Config) {
		next.AutoCheckUpdates = autoUpdateToggle.Checked
	}, func() { autoUpdateToggle.SetChecked(cfg.AutoCheckUpdates) })
	autoStartToggle.OnChanged = saveConfigField("Auto-start on launch", func(next *config.Config) {
		next.AutoStartOnLaunch = autoStartToggle.Checked
	}, func() { autoStartToggle.SetChecked(cfg.AutoStartOnLaunch) })
	reuseTorToggle.OnChanged = saveConfigField("Tor Browser reuse", func(next *config.Config) {
		next.ReuseExistingTorBrowser = reuseTorToggle.Checked
	}, func() { reuseTorToggle.SetChecked(cfg.ReuseExistingTorBrowser) })
	reuseI2PToggle.OnChanged = saveConfigField("I2P reuse", func(next *config.Config) {
		next.ReuseExistingI2P = reuseI2PToggle.Checked
	}, func() { reuseI2PToggle.SetChecked(cfg.ReuseExistingI2P) })
	localhostToggle.OnChanged = saveConfigField("Localhost access", func(next *config.Config) {
		next.AllowLocalhostAccess = localhostToggle.Checked
	}, func() { localhostToggle.SetChecked(cfg.AllowLocalhostAccess) })
	keepI2PRunningToggle.OnChanged = saveConfigField("Keep I2P running", func(next *config.Config) {
		next.KeepI2PRunning = keepI2PRunningToggle.Checked
	}, func() { keepI2PRunningToggle.SetChecked(cfg.KeepI2PRunning) })
	enableLoggingToggle.OnChanged = saveConfigField("Diagnostic logging", func(next *config.Config) {
		next.EnableLogging = enableLoggingToggle.Checked
		if enableLoggingToggle.Checked {
			logsButton.Show()
		} else {
			logsButton.Hide()
		}
	}, func() { enableLoggingToggle.SetChecked(cfg.EnableLogging) })

	heroCard := widget.NewCard(
		"",
		"",
		container.NewVBox(title, tagline, intro),
	)
	stateCard := widget.NewCard(
		"Launch State",
		"",
		container.NewVBox(statusLine, primaryTitle, primaryBody, stageLine, progress),
	)
	dependencyCard := widget.NewCard(
		"Dependencies",
		"",
		container.NewVBox(torStatus, javaStatus, i2pStatus, updateStatus),
	)
	actionsCard := widget.NewCard(
		"Actions",
		"",
		container.NewVBox(
			installButton,
			updateButton,
			startButton,
			startNowButton,
			logsButton,
			dataDirButton,
		),
	)
	settingsCard := widget.NewCard(
		"Settings",
		"",
		container.NewVBox(
			autoUpdateToggle,
			autoStartToggle,
			reuseTorToggle,
			reuseI2PToggle,
			localhostToggle,
			keepI2PRunningToggle,
			enableLoggingToggle,
			localhostWarning,
			lastSettingMessage,
		),
	)

	mainContent := container.NewVBox(
		heroCard,
		stateCard,
		dependencyCard,
		actionsCard,
		settingsCard,
	)
	scroll := container.NewVScroll(mainContent)
	scroll.SetMinSize(fyne.NewSize(760, 640))

	footer := container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("Settings are saved immediately."),
		layout.NewSpacer(),
		quitButton,
	)
	w.SetContent(container.NewBorder(nil, footer, nil, nil, container.NewPadded(scroll)))

	setState := func(primary, secondary, badge, stage string, progressValue float64, showInstall, showUpdate, showStart, showStartNow, busy bool) {
		statusLine.SetText("Status: " + badge)
		primaryTitle.SetText(primary)
		primaryBody.SetText(secondary)
		stageLine.SetText(stage)
		progress.SetValue(progressValue)
		toggleButton(installButton, showInstall, busy)
		toggleButton(updateButton, showUpdate, busy)
		toggleButton(startButton, showStart, busy)
		toggleButton(startNowButton, showStartNow, busy)
		logsButton.Enable()
		dataDirButton.Enable()
		quitButton.Enable()
	}

	refreshDependencyStatus := func(ctx context.Context) startupAssessment {
		torReady := managedOrExistingTorAvailable(ctx, cfg, paths)
		javaReady := managedOrExistingJavaAvailable(ctx, paths)
		i2pReady := managedOrExistingI2PAvailable(ctx, cfg, paths)
		torStatus.SetText(readinessLine("Tor Browser", torReady, cfg.ReuseExistingTorBrowser))
		javaStatus.SetText(readinessLine("Java 17+", javaReady, true))
		i2pStatus.SetText(readinessLine("I2P", i2pReady, cfg.ReuseExistingI2P))

		assessment := assessStartup(ctx, cfg, paths, *manifest)
		if assessment.UpdateAvailable {
			updateStatus.SetText("Updates: " + assessment.UpdateSummary)
		} else if cfg.AutoCheckUpdates {
			updateStatus.SetText("Updates: managed components are current")
		} else {
			updateStatus.SetText("Updates: startup checks disabled in settings")
		}
		return assessment
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
			assessment := refreshDependencyStatus(context.Background())
			switch {
			case assessment.NeedsInstall:
				setState(assessment.PrimaryMessage, assessment.SecondaryMessage, "Install Needed", "Waiting for install", 0.1, true, false, false, false, false)
			case assessment.UpdateAvailable:
				setState(assessment.PrimaryMessage, assessment.SecondaryMessage, "Update Available", "Waiting for update decision", 0.45, false, true, false, true, false)
			default:
				setState(assessment.PrimaryMessage, assessment.SecondaryMessage, "Ready", "Ready to launch", 1, false, false, true, false, false)
				if triggerAutoStart && assessment.AutoStart {
					startButton.OnTapped()
				}
			}
		}()
	}

	installButton.OnTapped = func() {
		runInBackground("Installing dependencies", "Downloading, verifying, and preparing managed runtime components.", func(actionCtx context.Context) error {
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
			runningDetail := "Tor Browser is open. The launcher will stay alive in the background until the browser exits."
			if cfg.KeepI2PRunning {
				runningDetail = "Tor Browser is open. Managed I2P will stay running in the background after the browser exits."
			}
			setState("Browser launched", runningDetail, "Running", "Browser launched", 1, false, false, false, false, true)
			w.Hide()
			go func() {
				waitManifest := *manifest
				err := waitForLauncherSession(context.Background(), logger, cfg, session, &waitManifest)
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

func readinessLine(name string, ready, reuseAllowed bool) string {
	switch {
	case ready && reuseAllowed:
		return fmt.Sprintf("%s: ready", name)
	case ready:
		return fmt.Sprintf("%s: ready (managed path only)", name)
	case reuseAllowed:
		return fmt.Sprintf("%s: missing managed install and no reusable install detected", name)
	default:
		return fmt.Sprintf("%s: missing managed install", name)
	}
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
