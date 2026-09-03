package judge

import "fmt"

// fingerprintVersion is the algorithm-version prefix every fingerprint value
// carries. A baseline holds fingerprints for a set difference between two
// runs, and that difference is meaningful only when both runs hashed the same
// way, so the value itself says which algorithm produced it and the baseline
// reader refuses any other prefix rather than subtracting nothing. Changing
// the hash — its constants, its separator placement, its rendering — means
// minting the next version prefix, not reusing this one.
const fingerprintVersion = "v1:"

// fingerprint returns the stable identity of a finding: the algorithm-version
// prefix followed by a 64-bit FNV-1a hash over ruleID, path, and target, with
// a 0x1f separator folded in after each of the three parts, rendered as
// sixteen lowercase hex digits. Consumers set-diff fingerprints between runs
// to see what a change introduced, and the prefix scopes that diff to values
// one algorithm wrote — a baseline from a different version is refused, never
// silently under-subtracted. The hash is written out here rather than built
// on hash/fnv to keep every pinned detail visible in one place.
//
// Which path and target each rule feeds is part of the contract too.
// An alias collision feeds an empty path with the normalized alias, and
// schema findings feed the field name and the violating value joined by
// another 0x1f, which keeps two findings on the same note distinct even
// when both of their values are empty.
func fingerprint(ruleID, path, target string) string {
	const (
		offset uint64 = 0xcbf29ce484222325
		prime  uint64 = 0x100000001b3
	)
	h := offset
	for _, part := range [...]string{ruleID, path, target} {
		for _, b := range []byte(part) {
			h = (h ^ uint64(b)) * prime
		}
		// The separator keeps ("a", "b") distinct from ("ab", "").
		h = (h ^ 0x1f) * prime
	}
	return fmt.Sprintf("%s%016x", fingerprintVersion, h)
}
