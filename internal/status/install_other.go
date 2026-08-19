//go:build !darwin && !linux

package status

import (
	"errors"
	"os"
)

// errExchangeUnavailable is what these targets report instead of an atomic
// exchange. Flip already refuses before touching the vault on every platform
// that cannot prove durable publication, which is the same set; the probe
// descends on this error rather than treating it as a failure.
var errExchangeUnavailable = errors.New("status: atomic directory-entry exchange is unavailable on this platform")

func exchangeNames(int, string, string) error { return errExchangeUnavailable }

// deviceIdentity has no portable answer here, so every publication probes
// again rather than reusing a result it cannot key.
func deviceIdentity(os.FileInfo) (any, bool) { return nil, false }
