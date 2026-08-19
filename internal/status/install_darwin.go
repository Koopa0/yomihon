package status

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// exchangeNames swaps two entries of the directory dfd refers to, in one
// atomic step, so neither version is ever missing from the directory.
// RENAME_NOFOLLOW_ANY additionally refuses the operation if any component of
// either name turns out to be a symbolic link.
func exchangeNames(dfd int, from, to string) error {
	return unix.RenameatxNp(dfd, from, dfd, to, unix.RENAME_SWAP|unix.RENAME_NOFOLLOW_ANY)
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
