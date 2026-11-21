package confdir

import (
	"math"

	"golang.org/x/sys/windows"
)

// running is ProcessAlive for a process id that can be one. The wait answers
// WAIT_OBJECT_0 for a process that has ended; the exit code is not asked, since
// a process can end with the code that means it has not.
func running(pid int) bool {
	if pid < 1 || int64(pid) > math.MaxUint32 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	event, err := windows.WaitForSingleObject(handle, 0)
	_ = windows.CloseHandle(handle) //nolint:errcheck // the wait has answered, and a handle that does not close changes no answer
	return err == nil && event == uint32(windows.WAIT_TIMEOUT)
}
