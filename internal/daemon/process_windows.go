//go:build windows

package daemon

import "errors"

func processRunning(pid int) bool {
	return false
}

func StartDetached(executable string, args []string, logPath string) (int, error) {
	return 0, errors.New("debug log shipping is not supported on Windows")
}
