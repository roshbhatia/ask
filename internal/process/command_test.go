package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandContextCancelsProcessGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pidPath := filepath.Join(t.TempDir(), "child.pid")
	command := CommandContext(ctx, "/bin/sh", "-c", `sleep 30 & echo $! > "$1"; wait`, "sh", pidPath)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	childPID := waitForPID(t, pidPath)
	cancel()
	err := command.Wait()
	if err == nil || (!errors.Is(err, context.Canceled) && !isExitError(err)) {
		t.Fatalf("Wait() error = %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for processExists(childPID) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if processExists(childPID) {
		t.Fatalf("child process %d survived cancellation", childPID)
	}
}

func waitForPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child process did not write its pid")
	return 0
}

func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func isExitError(err error) bool {
	var exitError *exec.ExitError
	return errors.As(err, &exitError)
}

func TestCommandContextCompletesNormally(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := CommandContext(ctx, "/usr/bin/true").Run(); err != nil {
		t.Fatal(err)
	}
}
