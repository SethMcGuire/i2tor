//go:build !linux

package main

import (
	"context"
	"fmt"

	"i2tor/internal/apppaths"
	"i2tor/internal/config"
	"i2tor/internal/logging"
	"i2tor/internal/state"
)

func commandNativeGUI(ctx context.Context, logger *logging.Logger, cfg config.Config, paths apppaths.AppPaths, manifest *state.Manifest) error {
	_ = ctx
	_ = logger
	_ = cfg
	_ = paths
	_ = manifest
	return fmt.Errorf("native desktop GUI is currently implemented for Linux only; use `i2tor webui` on this platform")
}
