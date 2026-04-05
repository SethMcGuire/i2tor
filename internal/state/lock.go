package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type Lock struct {
	path string
}

func AcquireLock(ctx context.Context, path string) (*Lock, error) {
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	pid := os.Getpid()
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			defer f.Close()
			if _, err := fmt.Fprintf(f, "%d\n", pid); err != nil {
				return nil, fmt.Errorf("write lock file: %w", err)
			}
			return &Lock{path: path}, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire lock %q: %w", path, err)
		}

		stale, staleErr := isLockStale(path)
		if staleErr != nil {
			return nil, staleErr
		}
		if !stale {
			return nil, fmt.Errorf("another i2tor instance is already running")
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale lock %q: %w", path, err)
		}
	}
}

func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release lock %q: %w", l.path, err)
	}
	return nil
}

func isLockStale(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("read lock file %q: %w", path, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return true, nil
	}
	return !pidExists(pid), nil
}

func pidExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
