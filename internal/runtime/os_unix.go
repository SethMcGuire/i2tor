//go:build !windows

package runtime

import (
	"errors"
	"syscall"
)

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func terminatePID(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}
