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

// copyXattrsFrom copies the source's extended attributes onto the replacement
// and reports the set it copied, so the same set can be read from the source
// again before the replacement takes the note's name.
func copyXattrsFrom(parent *os.Root, srcName string, dst *os.File, list func(int) ([]string, error)) (map[string][]byte, error) {
	attrs, err := currentXattrs(parent, srcName, list)
	if err != nil {
		return nil, err
	}
	for name, value := range attrs {
		if setErr := unix.Fsetxattr(int(dst.Fd()), name, value, 0); setErr != nil && !xattrIgnorable(setErr) {
			return nil, fmt.Errorf("copy extended attributes: copy %q: %w", name, setErr)
		}
	}
	return attrs, nil
}

// currentXattrs reads the extended attributes a replacement carries: the names
// this package propagates, each paired with the value the source holds right
// now. One reader answers both the copy and the later comparison, so an
// attribute the filesystem refuses to list or read is absent from both sets
// rather than turning tolerance into a refusal.
func currentXattrs(parent *os.Root, srcName string, list func(int) ([]string, error)) (map[string][]byte, error) {
	src, err := parent.Open(srcName)
	if err != nil {
		return nil, fmt.Errorf("open source to read attributes: %w", err)
	}
	attrs, err := readXattrs(int(src.Fd()), list)
	if closeErr := src.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("read extended attributes: %w", err)
	}
	return attrs, nil
}

func readXattrs(fd int, list func(int) ([]string, error)) (map[string][]byte, error) {
	if list == nil {
		list = listXattrNames
	}
	names, err := list(fd)
	if err != nil {
		if xattrIgnorable(err) {
			return map[string][]byte{}, nil
		}
		return nil, err
	}
	attrs := make(map[string][]byte, len(names))
	for _, name := range names {
		if skipCopyXattr(name) {
			continue
		}
		value, getErr := getXattr(fd, name)
		if getErr != nil {
			if xattrIgnorable(getErr) {
				continue
			}
			return nil, fmt.Errorf("read %q: %w", name, getErr)
		}
		attrs[name] = value
	}
	return attrs, nil
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
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS)
}

func xattrIgnorable(err error) bool {
	if xattrUnsupported(err) || xattrMissing(err) {
		return true
	}
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES)
}
