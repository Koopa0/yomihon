//go:build windows

package status

import "os"

// Windows exposes no documented directory-entry synchronization through
// os.Root. Flip refuses before filesystem access; this sentinel backstops any
// path that bypasses that preflight.
func syncDirectory(*os.Root) error { return ErrDurabilityUnsupported }
