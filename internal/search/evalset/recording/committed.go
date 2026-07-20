package recording

import _ "embed"

//go:embed testdata/gemini-embedding-2-1536.json
var committedJSON []byte

// Committed returns the provider recording selected for the current semantic
// protocol. Each call reparses the immutable bytes so callers never share
// mutable vector storage.
func Committed() (*Vectors, error) {
	return Parse(committedJSON)
}
