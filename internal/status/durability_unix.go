//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package status

import (
	"errors"
	"os"
)

func syncDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
