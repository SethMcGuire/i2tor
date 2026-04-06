//go:build !linux

package integration

import "context"

func EnsureDesktopEntry(ctx context.Context, executablePath, iconSourcePath string) error {
	_ = ctx
	_ = executablePath
	_ = iconSourcePath
	return nil
}

func ResolveDesktopExecutablePath() (string, error) {
	return "", nil
}
