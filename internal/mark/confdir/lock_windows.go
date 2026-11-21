package confdir

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// allBytes is the length of the range a lock claims, in each half of its 64
// bits: the whole of the file, however long, from its first byte.
const allBytes = ^uint32(0)

// openLock opens the lock file in dir, making it if it is not there. Windows has
// no flag that refuses a link, so it is opened through a root of dir, which
// refuses one that leads out of it.
func openLock(dir Dir, name string) (*os.File, error) {
	root, err := os.OpenRoot(dir.path)
	if err != nil {
		return nil, err
	}
	file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if closeErr := root.Close(); closeErr != nil && err == nil {
		return nil, errors.Join(closeErr, file.Close())
	}
	return file, err
}

// tryLock takes the exclusive lock on file if it is free, without waiting, and
// says whether it did.
func tryLock(file *os.File) (bool, error) {
	err := control(file, func(handle windows.Handle) error {
		return windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, allBytes, allBytes, new(windows.Overlapped))
	})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}

func unlock(file *os.File) error {
	return control(file, func(handle windows.Handle) error {
		return windows.UnlockFileEx(handle, 0, allBytes, allBytes, new(windows.Overlapped))
	})
}

// control runs call on file's handle.
func control(file *os.File, call func(windows.Handle) error) error {
	conn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var callErr error
	if err := conn.Control(func(fd uintptr) { callErr = call(windows.Handle(fd)) }); err != nil {
		return err
	}
	return callErr
}
