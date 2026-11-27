package schema

import (
	"path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// normalizeDeclaredDir reads one directory the contract declares and reports
// the form its policy matches against, or false when the value does not name a
// directory inside the vault; every contract directory list shares it.
//
// A parent step is refused before the cleaning pass, not after: cleaning walks
// the step away, so "Private/../Notes" would declare a folder its author never
// wrote down. Segments are read rather than the cleaned value compared, because
// "./Private" and "Private/" are the same folder spelled two legal ways.
func normalizeDeclaredDir(declared string) (string, bool) {
	if declared == "" || strings.Contains(declared, `\`) {
		return "", false
	}
	normalized := vault.NormalizeNFC(declared)
	if slices.Contains(strings.Split(normalized, "/"), "..") {
		return "", false
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", false
	}
	return cleaned, true
}

// inDeclaredDir reports whether rel, a vault-relative path, is one of dirs or
// below one, read the one way every declared directory list is matched: the
// path composed and cleaned, so a dotted spelling is judged by where it lands,
// then compared a whole component at a time with [SameDirName]. valid is false,
// and in with it, for a path that is empty, carries a backslash, or leaves the
// vault.
func inDeclaredDir(rel string, dirs []string) (in, valid bool) {
	if rel == "" || strings.Contains(rel, `\`) {
		return false, false
	}
	cleaned := path.Clean(vault.NormalizeNFC(rel))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return false, false
	}
	return slices.ContainsFunc(dirs, func(dir string) bool { return pathHasFoldedPrefix(cleaned, dir) }), true
}
