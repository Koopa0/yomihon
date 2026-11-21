//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package confdir

import (
	"errors"
	"os"
	"syscall"
)

// openLock opens the lock file in dir, making it if it is not there, and refuses
// a link left at the name, which would have the file made wherever it points. It
// is not opened through an os.Root: on macOS, two creates of one file at once
// through openat have been seen to answer that it does not exist.
func openLock(dir Dir, name string) (*os.File, error) {
	return os.OpenFile(dir.Join(name), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600) // #nosec G304 -- a lock file of the configuration directory
}

// tryLock takes the exclusive lock on file if it is free, without waiting, and
// says whether it did. It is flock and not fcntl: a lock fcntl takes belongs to
// the process, so a second claim from the same process would be granted.
func tryLock(file *os.File) (bool, error) {
	err := flock(file, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlock(file *os.File) error { return flock(file, syscall.LOCK_UN) }

// flock applies how to file's descriptor, again if a signal interrupted it.
func flock(file *os.File, how int) error {
	conn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var lockErr error
	if err := conn.Control(func(fd uintptr) {
		for {
			lockErr = syscall.Flock(int(fd), how) // #nosec G115 -- a descriptor the system handed out fits an int
			if !errors.Is(lockErr, syscall.EINTR) {
				return
			}
		}
	}); err != nil {
		return err
	}
	return lockErr
}
