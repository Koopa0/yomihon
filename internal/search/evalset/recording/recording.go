package recording

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"slices"
	"strings"
)

var (
	// ErrAbsent means the requested local vector artifact is missing or empty.
	ErrAbsent = errors.New("semantic eval recorded vectors are absent")
	// ErrInvalid means a vector artifact is malformed or incomplete.
	ErrInvalid = errors.New("semantic eval recording is invalid")
	// ErrIdentityMismatch means vectors are incompatible with an identity
	// independently derived by their consumer.
	ErrIdentityMismatch = errors.New("semantic eval recording identity mismatch")
)

const formatVersion = 2

// Vectors is a validated, owned set of recorded query and corpus vectors.
// Identity labels remain opaque until a consumer compares them with values it
// derives from its own current configuration.
type Vectors struct {
	dimension             int
	chunkTokenCap         int
	fullIdentity          [sha256.Size]byte
	compatibilityIdentity [sha256.Size]byte
	queries               []Query
	chunks                []Chunk
}

// Query is one recorded synthetic query vector and the digest of the exact
// provider text from which it was produced.
type Query struct {
	ID             string
	SubmissionHash [sha256.Size]byte
	Vector         []float32
}

// Chunk is one recorded synthetic corpus vector and its submitted-byte hash.
type Chunk struct {
	RelPath       string
	Ordinal       uint32
	SubmittedHash [sha256.Size]byte
	Vector        []float32
}

type vectorsWire struct {
	FormatVersion         int         `json:"format_version"`
	Dimension             int         `json:"dimension"`
	ChunkTokenCap         int         `json:"chunk_token_cap"`
	FullIdentity          string      `json:"full_cache_identity"`
	CompatibilityIdentity string      `json:"query_vector_compatibility_identity"`
	Queries               []queryWire `json:"queries"`
	Chunks                []chunkWire `json:"chunks"`
}

type queryWire struct {
	ID             string    `json:"id"`
	SubmissionHash string    `json:"submission_hash"`
	Vector         []float32 `json:"vector"`
}

type chunkWire struct {
	RelPath       string    `json:"rel_path"`
	Ordinal       uint32    `json:"ordinal"`
	SubmittedHash string    `json:"submitted_hash"`
	Vector        []float32 `json:"vector"`
}

// Load reads one local recorded-vector artifact.
func Load(name string) (*Vectors, error) {
	data, err := os.ReadFile(name) // #nosec G304 -- the explicit local CLI caller owns the fixture path.
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("read semantic eval recording: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrAbsent
	}
	return Parse(data)
}

// Parse validates the artifact's structure and takes ownership of every
// vector. It deliberately does not decide whether the recorded identity is
// current; the consumer must derive and compare its own expected identity.
func Parse(data []byte) (*Vectors, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, ErrAbsent
	}
	wire, err := decode(data)
	if err != nil {
		return nil, err
	}
	full, err := decodeRequiredHash("full cache identity", wire.FullIdentity)
	if err != nil {
		return nil, err
	}
	compatibility, err := decodeRequiredHash("query compatibility identity", wire.CompatibilityIdentity)
	if err != nil {
		return nil, err
	}
	queries, chunks, err := validateVectors(&wire)
	if err != nil {
		return nil, err
	}
	return &Vectors{
		dimension:             wire.Dimension,
		chunkTokenCap:         wire.ChunkTokenCap,
		fullIdentity:          full,
		compatibilityIdentity: compatibility,
		queries:               queries,
		chunks:                chunks,
	}, nil
}

func decode(data []byte) (vectorsWire, error) {
	var wire vectorsWire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return vectorsWire{}, fmt.Errorf("%w: decode JSON: %w", ErrInvalid, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return vectorsWire{}, err
	}
	if wire.FormatVersion != formatVersion || wire.Dimension <= 0 || wire.ChunkTokenCap <= 0 {
		return vectorsWire{}, fmt.Errorf("%w: format, dimension, or chunk token cap", ErrInvalid)
	}
	return wire, nil
}

// Dimension returns the number of float32 components in every vector.
func (v *Vectors) Dimension() int {
	if v == nil {
		return 0
	}
	return v.dimension
}

// ChunkTokenCap returns the chunk budget used to create the recorded corpus.
func (v *Vectors) ChunkTokenCap() int {
	if v == nil {
		return 0
	}
	return v.chunkTokenCap
}

// RequireFullIdentity rejects vectors not bound to the consumer's complete
// corpus-generation identity.
func (v *Vectors) RequireFullIdentity(expected [sha256.Size]byte) error {
	if v == nil || expected == ([sha256.Size]byte{}) || v.fullIdentity != expected {
		return ErrIdentityMismatch
	}
	return nil
}

// QueryVectors returns owned query-vector copies only when they are
// numerically comparable with the consumer's current query protocol.
func (v *Vectors) QueryVectors(expected [sha256.Size]byte) ([][]float32, error) {
	if v == nil || expected == ([sha256.Size]byte{}) || v.compatibilityIdentity != expected {
		return nil, ErrIdentityMismatch
	}
	queries := make([][]float32, len(v.queries))
	for i := range v.queries {
		queries[i] = slices.Clone(v.queries[i].Vector)
	}
	return queries, nil
}

// Queries returns owned copies of the recorded queries.
func (v *Vectors) Queries() []Query {
	if v == nil {
		return nil
	}
	queries := make([]Query, len(v.queries))
	for i := range v.queries {
		queries[i] = v.queries[i]
		queries[i].Vector = slices.Clone(v.queries[i].Vector)
	}
	return queries
}

// Chunks returns owned copies of the recorded corpus vectors.
func (v *Vectors) Chunks() []Chunk {
	if v == nil {
		return nil
	}
	chunks := make([]Chunk, len(v.chunks))
	for i := range v.chunks {
		chunks[i] = v.chunks[i]
		chunks[i].Vector = slices.Clone(v.chunks[i].Vector)
	}
	return chunks
}

func validateVectors(wire *vectorsWire) ([]Query, []Chunk, error) {
	if len(wire.Queries) == 0 || len(wire.Chunks) == 0 {
		return nil, nil, fmt.Errorf("%w: query or corpus vectors are empty", ErrInvalid)
	}
	if len(wire.Queries) != 40 {
		return nil, nil, fmt.Errorf("%w: expected exactly 40 query vectors", ErrInvalid)
	}
	queries := make([]Query, len(wire.Queries))
	for i := range wire.Queries {
		row := &wire.Queries[i]
		submissionHash, err := decodeRequiredHash("query submission hash", row.SubmissionHash)
		if row.ID != fmt.Sprintf("q%02d", i+1) || err != nil || !validVector(row.Vector, wire.Dimension) {
			return nil, nil, fmt.Errorf("%w: query vector %d", ErrInvalid, i+1)
		}
		queries[i] = Query{ID: row.ID, SubmissionHash: submissionHash, Vector: slices.Clone(row.Vector)}
	}
	chunks := make([]Chunk, len(wire.Chunks))
	type chunkKey struct {
		path    string
		ordinal uint32
	}
	seen := make(map[chunkKey]struct{}, len(wire.Chunks))
	for i := range wire.Chunks {
		row := &wire.Chunks[i]
		key := chunkKey{path: row.RelPath, ordinal: row.Ordinal}
		if !IsSyntheticPath(row.RelPath) || !validVector(row.Vector, wire.Dimension) {
			return nil, nil, fmt.Errorf("%w: corpus vector %d", ErrInvalid, i+1)
		}
		submittedHash, err := decodeRequiredHash("submitted hash", row.SubmittedHash)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: corpus vector %d submitted hash", ErrInvalid, i+1)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, nil, fmt.Errorf("%w: duplicate corpus vector", ErrInvalid)
		}
		seen[key] = struct{}{}
		chunks[i] = Chunk{
			RelPath:       row.RelPath,
			Ordinal:       row.Ordinal,
			SubmittedHash: submittedHash,
			Vector:        slices.Clone(row.Vector),
		}
	}
	return queries, chunks, nil
}

func validVector(vector []float32, dimension int) bool {
	if len(vector) != dimension {
		return false
	}
	var nonzero bool
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
		if value != 0 {
			nonzero = true
		}
	}
	return nonzero
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing JSON value", ErrInvalid)
	}
	return nil
}

func decodeRequiredHash(label, value string) ([sha256.Size]byte, error) {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return [sha256.Size]byte{}, fmt.Errorf("%w: %s is not canonical lowercase hex", ErrInvalid, label)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("%w: %s is not hex", ErrInvalid, label)
	}
	var result [sha256.Size]byte
	copy(result[:], decoded)
	if result == ([sha256.Size]byte{}) {
		return [sha256.Size]byte{}, fmt.Errorf("%w: %s is zero", ErrInvalid, label)
	}
	return result, nil
}

// IsSyntheticPath reports whether a relative Markdown path carries the
// synthetic-evaluation marker and stays outside the private Diary boundary.
func IsSyntheticPath(relPath string) bool {
	if relPath == "" || path.IsAbs(relPath) || path.Clean(relPath) != relPath || strings.Contains(relPath, "\\") {
		return false
	}
	components := strings.Split(relPath, "/")
	if slices.Contains(components, "Diary") {
		return false
	}
	base := path.Base(relPath)
	return strings.HasPrefix(base, "__synthetic_eval_") && strings.HasSuffix(base, ".md")
}
