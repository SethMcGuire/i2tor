package detect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type InstallCandidate struct {
	Name          string
	Source        string
	RootPath      string
	Executable    string
	Version       string
	ReadOnly      bool
	DetectionHint string
}

func DetectExistingTorBrowser(ctx context.Context) (InstallCandidate, error) {
	_ = ctx

	candidates := torBrowserCandidatePaths()
	for _, root := range candidates {
		execPath, err := resolveTorExecutableFromRoot(root)
		if err == nil {
			return InstallCandidate{
				Name:          "tor-browser",
				Source:        "existing",
				RootPath:      root,
				Executable:    execPath,
				ReadOnly:      true,
				DetectionHint: "heuristic path match",
			}, nil
		}
	}

	if pathExec, err := exec.LookPath("tor-browser"); err == nil {
		return InstallCandidate{
			Name:          "tor-browser",
			Source:        "existing",
			RootPath:      filepath.Dir(pathExec),
			Executable:    pathExec,
			ReadOnly:      true,
			DetectionHint: "PATH lookup",
		}, nil
	}

	return InstallCandidate{}, errors.New("no existing Tor Browser install detected")
}

func torBrowserCandidatePaths() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "linux":
		return []string{
			filepath.Join(home, "tor-browser"),
			filepath.Join(home, "Tor Browser"),
			filepath.Join(home, ".local", "share", "torbrowser", "tbb", "x86_64", "tor-browser"),
			"/opt/tor-browser",
		}
	case "windows":
		localAppData := os.Getenv("LocalAppData")
		programFiles := os.Getenv("ProgramFiles")
		return []string{
			filepath.Join(localAppData, "Tor Browser"),
			filepath.Join(programFiles, "Tor Browser"),
		}
	case "darwin":
		return []string{
			filepath.Join("/Applications", "Tor Browser.app"),
			filepath.Join(home, "Applications", "Tor Browser.app"),
		}
	default:
		return nil
	}
}

func resolveTorExecutableFromRoot(root string) (string, error) {
	for _, candidate := range []string{
		filepath.Join(root, "Browser", "start-tor-browser"),
		filepath.Join(root, "start-tor-browser.desktop"),
		filepath.Join(root, "Browser", "firefox"),
		filepath.Join(root, "Browser", "firefox.exe"),
		filepath.Join(root, "Tor Browser.app", "Contents", "MacOS", "firefox"),
		filepath.Join(root, "Contents", "MacOS", "firefox"),
	} {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no Tor Browser executable under %q", root)
}
