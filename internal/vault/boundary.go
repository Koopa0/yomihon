package vault

import (
	"path"
	"strings"
)

// Boundary is the part of the scan's reach a vault's own contract decides: the
// directories it keeps out of the scan on top of the dot-prefixed names every
// scan skips. A directory the boundary excludes is treated exactly as a hidden
// one is — never read, so nothing indexed, listed, served, or reported
// missing beneath it. The zero Boundary excludes nothing.
//
// A Boundary is immutable; the patterns are validated by whoever read them
// from the contract.
type Boundary struct {
	excludeDirs []string
}

// NewBoundary returns the boundary that excludes every directory whose
// vault-relative slash path matches one of patterns, in path.Match syntax, so
// a star never crosses a slash. Patterns are held in NFC, the form every vault
// path is compared in. The slice is copied.
func NewBoundary(patterns []string) Boundary {
	if len(patterns) == 0 {
		return Boundary{}
	}
	held := make([]string, len(patterns))
	for i, pattern := range patterns {
		held[i] = NormalizeNFC(pattern)
	}
	return Boundary{excludeDirs: held}
}

// excludes reports whether the directory at canonicalDir, a canonical
// vault-relative path, is one the boundary keeps out of the scan.
func (b Boundary) excludes(canonicalDir string) bool {
	for _, pattern := range b.excludeDirs {
		// A pattern that fails to parse matches nothing, so a boundary built
		// from unvalidated patterns degrades to excluding less, never more.
		if matched, err := path.Match(pattern, canonicalDir); err == nil && matched {
			return true
		}
	}
	return false
}

// OutsideScan reports whether relPath lies beyond what a scan under this
// boundary looks at: any dot-leading segment, or any directory above the last
// segment that the boundary excludes. The last segment is not tested against
// the patterns because the walk only excludes directories, and a caller's
// path may name a file.
func (b Boundary) OutsideScan(relPath string) bool {
	if OutsideScan(relPath) {
		return true
	}
	for i := range relPath {
		if relPath[i] == '/' && b.excludes(relPath[:i]) {
			return true
		}
	}
	return false
}

// ValidExcludePattern reports why pattern cannot name directories to exclude,
// or "" when it can. A pattern is relative to the vault root and already
// clean — no leading slash, no empty, "." or ".." segment, no trailing slash —
// and must parse as a path.Match pattern.
func ValidExcludePattern(pattern string) string {
	switch {
	case pattern == "":
		return "is empty"
	case strings.ContainsRune(pattern, 0):
		return "contains a NUL byte"
	case strings.HasPrefix(pattern, "/"):
		return "is absolute"
	case path.Clean(pattern) != pattern || pattern == ".":
		return "is not a clean relative path"
	}
	for segment := range strings.SplitSeq(pattern, "/") {
		if segment == ".." {
			return "climbs out of the vault"
		}
	}
	if _, err := path.Match(pattern, ""); err != nil {
		return "is not a valid pattern: " + err.Error()
	}
	return ""
}
