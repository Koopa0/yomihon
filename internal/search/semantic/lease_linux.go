//go:build linux

package semantic

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockWriterFileNonblocking(file *os.File) error {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) {
			return ErrWriterHeld
		}
		return err
	}
	return nil
}

func unlockWriterFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
