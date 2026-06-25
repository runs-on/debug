package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunningPIDDetectsCurrentProcess(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "debug-action.pid")
	if err := WritePID(pidFile, os.Getpid()); err != nil {
		t.Fatalf("WritePID() error = %v", err)
	}

	pid, ok := RunningPID(pidFile)
	if !ok {
		t.Fatal("expected current process to be detected as running")
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestRunningPIDIgnoresStalePID(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "debug-action.pid")
	if err := WritePID(pidFile, 99999999); err != nil {
		t.Fatalf("WritePID() error = %v", err)
	}

	if _, ok := RunningPID(pidFile); ok {
		t.Fatal("expected stale PID to be ignored")
	}
}
