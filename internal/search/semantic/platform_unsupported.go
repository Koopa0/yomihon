//go:build android || freebsd || illumos || ios || js || netbsd || openbsd || plan9 || wasip1 || windows

package semantic

func requireSemanticStorePlatform() error { return ErrStoreUnsupportedPlatform }
