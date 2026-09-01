package schema

import (
	"path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// normalizeDeclaredDir reads one directory the contract declares and reports
// the form its policy matches against, or false when the value does not name a
// directory inside the vault.
//
// A parent step is refused before the cleaning pass rather than after it.
// Cleaning walks the step away and leaves an ordinary local directory that
// every check below would then accept, so "Private/../Notes" declared Notes —
// a folder its author never wrote down — and nothing said so, while "", "/"
// and "../.." from the same field were refused. The test reads the value's
// segments rather than asking whether cleaning changed the value at all,
// because "./Private" and "Private/" are that same folder spelled two other
// ways and stay legal.
//
// A path being looked up is the opposite case and is not this function's
// business: collapsing a parent step there is what stops a query from reaching
// past a deny list, which is why EgressAllowed cleans and then matches.
//
// The contract's directory declarations share this so the rule cannot drift
// apart again: the egress and non-instance lists carried two byte-identical
// copies of it, and correcting one of them would have left the other.
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
