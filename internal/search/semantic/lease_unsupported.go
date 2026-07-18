//go:build android || freebsd || illumos || ios || js || netbsd || openbsd || plan9 || wasip1 || windows

package semantic

import "os"

// The store's privacy contract is proved only on supported operating systems.
// Other platforms require their own permission and locking design, so store
// entry points fail before this stub is reached.
func lockWriterFileNonblocking(*os.File) error { return ErrStoreUnsupportedPlatform }

func unlockWriterFile(*os.File) error { return nil }
