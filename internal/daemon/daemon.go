package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Paths struct {
	StateDir string
}

func (p Paths) PIDFile() string {
	return filepath.Join(p.StateDir, "debug-action.pid")
}

func (p Paths) LogFile() string {
	return filepath.Join(p.StateDir, "debug-action.log")
}

func EnsureStateDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func RunningPID(pidFile string) (int, bool) {
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	if processRunning(pid) {
		return pid, true
	}
	return 0, false
}

func WritePID(pidFile string, pid int) error {
	if pid <= 0 {
		return errors.New("pid must be positive")
	}
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

func RemovePID(pidFile string) {
	_ = os.Remove(pidFile)
}

func AlreadyRunningMessage(pid int, stream string) string {
	return fmt.Sprintf("RunsOn debug log shipping already running with PID %d; CloudWatch stream: %s", pid, stream)
}
