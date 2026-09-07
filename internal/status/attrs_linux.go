//go:build linux

package status

import (
	"errors"

	"golang.org/x/sys/unix"
)

func xattrMissing(err error) bool {
	return errors.Is(err, unix.ENODATA)
}
