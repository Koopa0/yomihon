package judge

import "fmt"

// fingerprintVersion is the algorithm-version prefix every fingerprint value
// carries. A baseline set-diff is meaningful only between runs that hashed the
// same way, so the reader refuses any other prefix; changing the hash — its
// constants, its separator placement, its rendering — mints the next prefix.
const fingerprintVersion = "v1:"

// fingerprint returns the stable identity of a finding: the algorithm-version
// prefix followed by a 64-bit FNV-1a hash over ruleID, path and target, with a
// 0x1f separator folded in after each of the three parts, rendered as sixteen
// lowercase hex digits. The hash is written out here rather than built on
// hash/fnv to keep every pinned detail visible in one place. Which path and
// target each rule feeds is part of the contract too: an alias collision feeds
// an empty path with the normalized alias, and schema findings feed the field
// name and the violating value joined by another 0x1f.
func fingerprint(ruleID RuleID, path, target string) string {
	const (
		offset uint64 = 0xcbf29ce484222325
		prime  uint64 = 0x100000001b3
	)
	h := offset
	for _, part := range [...]string{string(ruleID), path, target} {
		for _, b := range []byte(part) {
			h = (h ^ uint64(b)) * prime
		}
		// The separator keeps ("a", "b") distinct from ("ab", "").
		h = (h ^ 0x1f) * prime
	}
	return fmt.Sprintf("%s%016x", fingerprintVersion, h)
}
