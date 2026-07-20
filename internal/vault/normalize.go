package vault

import "golang.org/x/text/unicode/norm"

// NormalizeNFC returns s in Unicode Normalization Form C. It is the single
// normalization primitive shared by vault paths, schema policies, retrieval,
// and wikilink resolution. Case folding and whitespace rules remain with the
// feature that owns those semantics.
func NormalizeNFC(s string) string {
	return norm.NFC.String(s)
}
