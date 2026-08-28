package semantic

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestEngineAggregatesChunksBeforeDepthAndRanksDeterministically(t *testing.T) {
	t.Parallel()

	rows := []ChunkVector{
		fixtureIndexRow("b.md", 4, []float32{1, 0}),
		fixtureIndexRow("b.md", 1, []float32{1, 0}),
		fixtureIndexRow("a.md", 0, []float32{1, 0}),
		fixtureIndexRow("c.md", 0, []float32{0.8, 0.6}),
		fixtureIndexRow("d.md", 0, []float32{-1, 0}),
	}
	index, err := NewIndex(rows, 2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := index.TopNotes([]float32{1, 0}, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{RelPath: "a.md", ChunkOrdinal: 0, Score: 1},
		{RelPath: "b.md", ChunkOrdinal: 1, Score: 1},
		{RelPath: "c.md", ChunkOrdinal: 0, Score: 0.8},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateApprox(0, 1e-7)); diff != "" {
		t.Errorf("TopNotes mismatch (-want +got):\n%s", diff)
	}
}

func TestEngineAppliesAllowedPathsBeforeDepth(t *testing.T) {
	t.Parallel()

	index, err := NewIndex([]ChunkVector{
		fixtureIndexRow("a.md", 0, []float32{1, 0}),
		fixtureIndexRow("b.md", 0, []float32{0.9, 0.1}),
		fixtureIndexRow("c.md", 0, []float32{0.8, 0.2}),
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := index.TopNotes([]float32{1, 0}, map[string]struct{}{
		"b.md": {},
		"c.md": {},
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RelPath != "b.md" {
		t.Errorf("filtered TopNotes = %+v, want b.md first", got)
	}
}

func TestIndexIsImmutableFromItsInput(t *testing.T) {
	t.Parallel()

	row := fixtureIndexRow("a.md", 0, []float32{1, 0})
	index, err := NewIndex([]ChunkVector{row}, 2)
	if err != nil {
		t.Fatal(err)
	}
	row.Vector[0] = -1
	got, err := index.TopNotes([]float32{1, 0}, nil, 50)
	if err != nil || len(got) != 1 || got[0].Score != 1 {
		t.Fatalf("index row was aliased: got %+v, error %v", got, err)
	}
	if index.Len() != 1 {
		t.Fatalf("Len = %d, want 1", index.Len())
	}
}

func TestOwnedIndexTransfersVectorsOnlyAfterCompleteValidation(t *testing.T) {
	t.Parallel()

	rows := []ChunkVector{
		fixtureIndexRow("a.md", 0, []float32{1, 0}),
		fixtureIndexRow("b.md", 0, []float32{0, 1}),
	}
	index, err := newOwnedIndex(rows, 2)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Vector != nil || rows[1].Vector != nil {
		t.Fatalf("owned index retained caller vector owners: %+v", rows)
	}
	ranked, err := index.TopNotes([]float32{1, 0}, nil, 2)
	if err != nil || len(ranked) != 2 || ranked[0].RelPath != "a.md" {
		t.Fatalf("TopNotes() after ownership transfer = %+v, %v", ranked, err)
	}

	invalid := []ChunkVector{
		fixtureIndexRow("a.md", 0, []float32{1, 0}),
		fixtureIndexRow("b.md", 0, []float32{0}),
	}
	if _, err := newOwnedIndex(invalid, 2); !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("newOwnedIndex() error = %v, want ErrDimensionMismatch", err)
	}
	if invalid[0].Vector == nil || invalid[1].Vector == nil {
		t.Fatal("failed owned-index construction consumed caller vectors")
	}
}

func TestExactScanCapacityBoundsCountAndRawVectorBytes(t *testing.T) {
	t.Parallel()

	if !withinExactScanCapacity(99_999, 1_536) {
		t.Fatal("99,999 x 1,536 should remain below both exact-scan guards")
	}
	if withinExactScanCapacity(100_000, 1_536) {
		t.Fatal("100,000 chunks should open the rung-two evaluation")
	}
	if !withinExactScanCapacity(87_381, 3_072) {
		t.Fatal("87,381 x 3,072 should fit the 1 GiB raw-vector guard")
	}
	if withinExactScanCapacity(87_382, 3_072) {
		t.Fatal("87,382 x 3,072 should exceed the 1 GiB raw-vector guard")
	}
}

func TestEngineRejectsInvalidDimensionsAndZeroQuery(t *testing.T) {
	t.Parallel()

	if _, err := NewIndex(nil, 0); !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("NewIndex(nil, 0) error = %v, want ErrDimensionMismatch", err)
	}
	if _, err := NewIndex([]ChunkVector{fixtureIndexRow("a.md", 0, []float32{1})}, 2); !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("NewIndex invalid row error = %v, want ErrDimensionMismatch", err)
	}
	index, err := NewIndex(nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := index.TopNotes([]float32{0, 0}, nil, 50); !errors.Is(err, ErrZeroVector) {
		t.Fatalf("TopNotes zero query error = %v, want ErrZeroVector", err)
	}
}

func TestIndexRejectsDuplicateChunkKeys(t *testing.T) {
	t.Parallel()

	row := fixtureIndexRow("a.md", 0, []float32{1, 0})
	if _, err := NewIndex([]ChunkVector{row, row}, 2); !errors.Is(err, ErrDuplicateChunk) {
		t.Fatalf("NewIndex duplicate error = %v, want ErrDuplicateChunk", err)
	}
}

func TestNewIndexRejectsRungTwoCorpusBeforeAllocatingVectors(t *testing.T) {
	t.Parallel()

	rows := make([]ChunkVector, exactScanChunkTrigger)
	if _, err := NewIndex(rows, 1536); !errors.Is(err, ErrIndexCapacity) {
		t.Fatalf("NewIndex() error = %v, want ErrIndexCapacity", err)
	}
}

func fixtureIndexRow(relPath string, ordinal uint32, vector []float32) ChunkVector {
	row := ChunkVector{
		RelPath:       relPath,
		NoteHash:      sha256.Sum256([]byte("note:" + relPath)),
		Ordinal:       ordinal,
		SubmittedHash: sha256.Sum256([]byte("submitted:" + relPath)),
		Vector:        vector,
	}
	return bindChunkVector(&row)
}
