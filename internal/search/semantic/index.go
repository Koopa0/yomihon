package semantic

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
)

const (
	exactScanChunkTrigger = 100_000
	maxExactVectorBytes   = int64(1 << 30)
	float32Bytes          = int64(4)
)

// Index errors distinguish malformed vectors from deterministic capacity
// refusal before allocation.
var (
	ErrDimensionMismatch = errors.New("semantic vector dimension mismatch")
	ErrZeroVector        = errors.New("semantic cosine vector has zero magnitude")
	ErrDuplicateChunk    = errors.New("semantic index contains a duplicate chunk")
	ErrIndexCapacity     = errors.New("semantic corpus exceeds the exact-scan capacity")
)

// Result is one note-level semantic channel result. ChunkOrdinal names
// the max-scoring current chunk that supplies heading/snippet evidence.
type Result struct {
	RelPath      string
	ChunkOrdinal uint32
	Score        float64
}

type chunkKey struct {
	relPath string
	ordinal uint32
}

type indexedChunk struct {
	key       chunkKey
	vector    []float32
	magnitude float64
}

// Index is an immutable exact-cosine index for one sealed generation. It owns
// copies of every vector and precomputes their magnitudes during construction,
// so concurrent callers need no mutation lock.
type Index struct {
	dimension int
	rows      []indexedChunk
}

// NewIndex validates and copies one complete generation. Duplicate keys,
// malformed dimensions, and non-finite or zero vectors reject the whole index.
func NewIndex(rows []ChunkVector, dimension int) (*Index, error) {
	return buildIndex(rows, dimension, false)
}

// newOwnedIndex transfers validated vector slices from rows into the immutable
// index. It is used only when the caller has exclusive ownership of a freshly
// hydrated generation; public NewIndex retains copy-in semantics.
func newOwnedIndex(rows []ChunkVector, dimension int) (*Index, error) {
	return buildIndex(rows, dimension, true)
}

func buildIndex(rows []ChunkVector, dimension int, transfer bool) (*Index, error) {
	if dimension <= 0 {
		return nil, ErrDimensionMismatch
	}
	if !withinExactScanCapacity(len(rows), dimension) {
		return nil, ErrIndexCapacity
	}
	index := &Index{dimension: dimension, rows: make([]indexedChunk, 0, len(rows))}
	seen := make(map[chunkKey]struct{}, len(rows))
	for position := range rows {
		row := &rows[position]
		target := ChunkTarget{
			RelPath:       row.RelPath,
			NoteHash:      row.NoteHash,
			Ordinal:       row.Ordinal,
			SubmittedHash: row.SubmittedHash,
			manifestSeal:  row.manifestSeal,
		}
		if err := validateChunkTarget(&target); err != nil {
			return nil, fmt.Errorf("%w: row identity", ErrInvalidChunk)
		}
		if len(row.Vector) != dimension {
			return nil, ErrDimensionMismatch
		}
		magnitude, ok := vectorMagnitude(row.Vector)
		if !ok {
			return nil, ErrZeroVector
		}
		key := chunkKey{relPath: row.RelPath, ordinal: row.Ordinal}
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrDuplicateChunk
		}
		seen[key] = struct{}{}
		vector := row.Vector
		if !transfer {
			vector = slices.Clone(vector)
		}
		index.rows = append(index.rows, indexedChunk{
			key:       key,
			vector:    vector,
			magnitude: magnitude,
		})
	}
	if transfer {
		for position := range rows {
			rows[position].Vector = nil
		}
	}
	return index, nil
}

func withinExactScanCapacity(chunks, dimension int) bool {
	if chunks < 0 || dimension <= 0 || chunks >= exactScanChunkTrigger {
		return false
	}
	dimension64 := int64(dimension)
	if dimension64 > math.MaxInt64/float32Bytes {
		return false
	}
	return int64(chunks) <= maxExactVectorBytes/(dimension64*float32Bytes)
}

// Len reports the number of indexed chunks.
func (i *Index) Len() int {
	if i == nil {
		return 0
	}
	return len(i.rows)
}

// TopNotes applies the structured-filter path set before scoring, calculates
// exact cosine for every remaining chunk, takes each note's maximum before the
// depth cut, and returns deterministic score/path order. Slice position is the
// channel-rank authority; a nil allowedPaths means all rows, while an empty
// non-nil map means none.
func (i *Index) TopNotes(query []float32, allowedPaths map[string]struct{}, depth int) ([]Result, error) {
	if i == nil || len(query) != i.dimension {
		return nil, ErrDimensionMismatch
	}
	queryMagnitude, ok := vectorMagnitude(query)
	if !ok {
		return nil, ErrZeroVector
	}
	if depth <= 0 {
		return nil, nil
	}

	best := make(map[string]Result)
	for _, row := range i.rows {
		if allowedPaths != nil {
			if _, allowed := allowedPaths[row.key.relPath]; !allowed {
				continue
			}
		}
		score := cosine(query, row.vector, queryMagnitude, row.magnitude)
		previous, found := best[row.key.relPath]
		if !found || score > previous.Score || (score == previous.Score && row.key.ordinal < previous.ChunkOrdinal) {
			best[row.key.relPath] = Result{
				RelPath:      row.key.relPath,
				ChunkOrdinal: row.key.ordinal,
				Score:        score,
			}
		}
	}

	ranked := make([]Result, 0, len(best))
	for _, result := range best {
		ranked = append(ranked, result)
	}
	slices.SortFunc(ranked, func(a, b Result) int {
		if byScore := cmp.Compare(b.Score, a.Score); byScore != 0 {
			return byScore
		}
		return cmp.Compare(a.RelPath, b.RelPath)
	})
	if len(ranked) > depth {
		ranked = ranked[:depth]
	}
	return ranked, nil
}

func vectorMagnitude(vector []float32) (float64, bool) {
	var squared float64
	for _, value := range vector {
		v := float64(value)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		squared += v * v
	}
	if squared == 0 || math.IsInf(squared, 0) {
		return 0, false
	}
	return math.Sqrt(squared), true
}

func cosine(a, b []float32, magnitudeA, magnitudeB float64) float64 {
	var dot float64
	for position, value := range a {
		dot += float64(value) * float64(b[position])
	}
	return dot / (magnitudeA * magnitudeB)
}
