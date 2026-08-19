package status

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// exchangeNames swaps two entries of the directory dfd refers to, in one
// atomic step, so neither version is ever missing from the directory.
func exchangeNames(dfd int, from, to string) error {
	return unix.Renameat2(dfd, from, dfd, to, unix.RENAME_EXCHANGE)
}

// deviceIdentity reports the filesystem a directory belongs to, so one probe
// result serves every directory on that filesystem.
func deviceIdentity(info os.FileInfo) (any, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, false
	}
	return stat.Dev, true
}
