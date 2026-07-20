//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package status

import "os"

// These targets do not expose a proven directory durability barrier. Flip
// refuses before filesystem access; this sentinel is the defensive backstop
// if a future internal path bypasses that preflight.
func syncDirectory(*os.Root) error { return ErrDurabilityUnsupported }
