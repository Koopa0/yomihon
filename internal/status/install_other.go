//go:build !darwin && !linux

package status

import (
	"errors"
	"os"
)

// errExchangeUnavailable is what these targets report instead of an atomic
// exchange. The probe descends on it rather than treating it as a failure.
var errExchangeUnavailable = errors.New("atomic directory-entry exchange is unavailable on this platform")

func exchangeNames(int, string, string) error { return errExchangeUnavailable }

// deviceIdentity has no portable answer here, so every install probes
// again rather than reusing a result it cannot key.
func deviceIdentity(os.FileInfo) (any, bool) { return nil, false }
