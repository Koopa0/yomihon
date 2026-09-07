//go:build darwin

package status

import (
	"errors"

	"golang.org/x/sys/unix"
)

func xattrMissing(err error) bool {
	return errors.Is(err, unix.ENOATTR) || errors.Is(err, unix.ENODATA)
}
