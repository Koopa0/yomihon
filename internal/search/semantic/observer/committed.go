package observer

import _ "embed"

//go:embed testdata/gemini-embedding-2-1536.workload
var committedArtifact []byte

// Committed returns the production top-k observer workload only when its
// compatibility identity matches the independently derived expected value.
// Each call reparses the immutable artifact so callers never share vectors.
func Committed(expected [identitySize]byte) (*Workload, error) {
	return parse(committedArtifact, expected)
}
