//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package status

import "os"

// These targets expose no proven directory durability barrier. Flip refuses
// before filesystem access; this sentinel backstops any path that bypasses it.
func syncDirectory(*os.Root) error { return ErrDurabilityUnsupported }
