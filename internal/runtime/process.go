package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"i2tor/internal/logging"
	"i2tor/internal/state"
)

type ManagedProcess struct {
	Name      string
	Command   string
	Args      []string
	Cmd       *exec.Cmd
	PID       int
	StartedAt time.Time
	Owned     bool
}

func startCommand(ctx context.Context, logger *logging.Logger, component, name, command string, args []string) (ManagedProcess, error) {
	return startCommandWithOptions(ctx, logger, component, name, command, args, "", nil)
}

func startCommandWithOptions(ctx context.Context, logger *logging.Logger, component, name, command string, args []string, workdir string, env []string) (ManagedProcess, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("capture stdout for %s: %w", name, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ManagedProcess{}, fmt.Errorf("capture stderr for %s: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return ManagedProcess{}, fmt.Errorf("start %s: %w", name, err)
	}

	proc := ManagedProcess{
		Name:      name,
		Command:   command,
		Args:      args,
		Cmd:       cmd,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now().UTC(),
		Owned:     true,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamLogs(&wg, stdout, logger, component, name, "stdout")
	go streamLogs(&wg, stderr, logger, component, name, "stderr")
	go func() { wg.Wait() }()

	return proc, nil
}

func streamLogs(wg *sync.WaitGroup, r io.ReadCloser, logger *logging.Logger, component, name, stream string) {
	defer wg.Done()
	defer r.Close()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info(component, fmt.Sprintf("%s %s", name, stream), map[string]any{"line": scanner.Text()})
	}
}

func Wait(ctx context.Context, proc ManagedProcess) error {
	done := make(chan error, 1)
	go func() {
		done <- proc.Cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func ReconcileManagedProcessRecord(record state.ManagedProcessRecord) state.ManagedProcessRecord {
	if record.PID <= 0 || !record.Owns {
		return state.ManagedProcessRecord{}
	}
	if processExists(record.PID) {
		return record
	}
	return state.ManagedProcessRecord{}
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
