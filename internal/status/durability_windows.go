//go:build windows

package status

import "os"

// Windows exposes no documented directory-entry synchronization through
// os.Root. Flip refuses before filesystem access on this platform; returning
// the sentinel here is a defensive backstop against any future internal path
// that bypasses that preflight.
func syncDirectory(*os.Root) error { return ErrDurabilityUnsupported }
