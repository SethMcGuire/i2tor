package detect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

func DetectExistingI2P(ctx context.Context) (InstallCandidate, error) {
	_ = ctx
	home, _ := os.UserHomeDir()
	var candidates []string
	switch runtime.GOOS {
	case "linux":
		candidates = []string{
			filepath.Join(home, "i2p"),
			filepath.Join(home, "I2P"),
			"/opt/i2p",
			"/usr/share/i2p",
		}
	case "windows":
		localAppData := os.Getenv("LocalAppData")
		candidates = []string{
			filepath.Join(localAppData, "I2P"),
			filepath.Join("C:\\", "Program Files", "I2P"),
		}
	case "darwin":
		candidates = []string{
			"/Applications/I2P.app",
			filepath.Join(home, "Applications", "I2P.app"),
		}
	}
	for _, candidate := range candidates {
		execPath := filepath.Join(candidate, "i2prouter")
		if stat, err := os.Stat(execPath); err == nil && !stat.IsDir() {
			return InstallCandidate{
				Name:          "i2p",
				Source:        "existing",
				RootPath:      candidate,
				Executable:    execPath,
				ReadOnly:      true,
				DetectionHint: "heuristic path match",
			}, nil
		}
	}
	return InstallCandidate{}, errors.New("no existing I2P install detected")
}
