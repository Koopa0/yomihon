package status

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// exchangeNames swaps two entries of the directory dfd refers to, in one
// atomic step, so neither version is ever missing from the directory.
//
// The flag word carries nothing beyond the swap itself. Path resolution is
// already pinned: dfd was reached by the walk that validates every component,
// both names are one entry inside it, and a rename does not follow a symbolic
// link in the final component. A flag asking for more would only give an older
// kernel a reason to refuse the whole call, which the probe would read as a
// filesystem that cannot swap — the guarantee quietly downgraded for nothing.
func exchangeNames(dfd int, from, to string) error {
	return unix.RenameatxNp(dfd, from, dfd, to, unix.RENAME_SWAP)
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
