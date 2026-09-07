//go:build linux || darwin

package status

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// refuseHardLinked reports a note that has more than one directory entry. The
// install replaces the named entry with a new inode; every other name would
// keep the pre-flip bytes.
func refuseHardLinked(info os.FileInfo, relSlash string) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink <= 1 {
		return nil
	}
	return fmt.Errorf("%w: %s has %d names; a flip would leave the others on the pre-flip bytes", ErrHardLinked, relSlash, stat.Nlink)
}

func copyXattrsFrom(parent *os.Root, srcName string, dst *os.File) error {
	src, err := parent.Open(srcName)
	if err != nil {
		return fmt.Errorf("open source to copy attributes: %w", err)
	}
	err = copyXattrs(int(src.Fd()), int(dst.Fd()))
	if closeErr := src.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("copy extended attributes: %w", err)
	}
	return nil
}

func copyXattrs(srcFd, dstFd int) error {
	names, err := listXattrNames(srcFd)
	if err != nil {
		if xattrUnsupported(err) {
			return nil
		}
		return err
	}
	for _, name := range names {
		if skipCopyXattr(name) {
			continue
		}
		if err := copyOneXattr(srcFd, dstFd, name); err != nil {
			return err
		}
	}
	return nil
}

func copyOneXattr(srcFd, dstFd int, name string) error {
	value, err := getXattr(srcFd, name)
	if err != nil {
		if xattrIgnorable(err) {
			return nil
		}
		return fmt.Errorf("read %q: %w", name, err)
	}
	if err := unix.Fsetxattr(dstFd, name, value, 0); err != nil {
		if xattrIgnorable(err) {
			return nil
		}
		return fmt.Errorf("copy %q: %w", name, err)
	}
	return nil
}

func listXattrNames(fd int) ([]string, error) {
	size, err := unix.Flistxattr(fd, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	n, err := unix.Flistxattr(fd, buf)
	if err != nil {
		return nil, err
	}
	return splitXattrNames(buf[:n]), nil
}

func getXattr(fd int, name string) ([]byte, error) {
	size, err := unix.Fgetxattr(fd, name, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return []byte{}, nil
	}
	buf := make([]byte, size)
	n, err := unix.Fgetxattr(fd, name, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func splitXattrNames(buf []byte) []string {
	var names []string
	for len(buf) > 0 {
		name, rest, found := bytes.Cut(buf, []byte{0})
		if len(name) > 0 {
			names = append(names, string(name))
		}
		if !found {
			break
		}
		buf = rest
	}
	return names
}

func skipCopyXattr(name string) bool {
	switch name {
	case "com.apple.provenance", "com.apple.macl":
		return true
	}
	return strings.HasPrefix(name, "security.")
}

func xattrUnsupported(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}

func xattrIgnorable(err error) bool {
	if xattrUnsupported(err) || xattrMissing(err) {
		return true
	}
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES)
}
