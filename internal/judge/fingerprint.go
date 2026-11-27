package judge

import (
	"fmt"
	"hash/fnv"
)

// fingerprintVersion is the algorithm-version prefix every fingerprint value
// carries. A baseline set-diff is meaningful only between runs that hashed the
// same way, so the reader refuses any other prefix; changing the hash — its
// constants, its separator placement, its rendering — mints the next prefix.
const fingerprintVersion = "v1:"

// fingerprint returns the stable identity of a finding: the algorithm-version
// prefix followed by a 64-bit FNV-1a hash over ruleID, path and target, with a
// 0x1f separator after each of the three so ("a", "b") differs from ("ab", ""),
// rendered as sixteen lowercase hex digits. Which path and target each rule
// feeds is part of the identity too: an alias collision feeds an empty path
// with the normalized alias, and schema findings feed the field name and the
// violating value joined by another 0x1f.
func fingerprint(ruleID RuleID, path, target string) string {
	h := fnv.New64a()
	for _, part := range [...]string{string(ruleID), path, target} {
		// A hash.Hash's Write never returns an error.
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0x1f})
	}
	return fmt.Sprintf("%s%016x", fingerprintVersion, h.Sum64())
}
