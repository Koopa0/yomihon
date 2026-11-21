//go:build unix

package confdir

import (
	"errors"
	"os"
	"syscall"
)

// running is ProcessAlive for a process id that can be one.
func running(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
