// Package process starts cancellable process trees on the supported Unix hosts.
package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const killDelay = 2 * time.Second

// CommandContext starts the command in its own process group. Cancellation
// reaches the adapter and every harness process it starts.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = killDelay
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		pid := command.Process.Pid
		err := syscall.Kill(-pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		if err != nil {
			return err
		}
		go func() {
			time.Sleep(killDelay)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}()
		return nil
	}
	return command
}
