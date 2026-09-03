package vault

import "golang.org/x/text/unicode/norm"

// NormalizeNFC returns s in Unicode Normalization Form C — the one
// normalization every vault path, policy and wikilink key passes through.
// Case folding and whitespace stay with the feature that owns them.
func NormalizeNFC(s string) string {
	return norm.NFC.String(s)
}
