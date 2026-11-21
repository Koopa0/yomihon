package confdir

import (
	"errors"
	"os"
	"runtime"
	"syscall"
)

// ProcessAlive says whether a process with the id exists. Signal 0 delivers
// nothing and reports only whether it could be delivered; a process that is not
// ours answers permission denied, which is still a process. Windows has no
// such signal, and there a process that cannot be opened by id is gone.
func ProcessAlive(pid int) bool {
	if pid < 1 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return process.Release() == nil
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
