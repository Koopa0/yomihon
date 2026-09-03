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
