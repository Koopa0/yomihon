package semantic

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql" //nolint:depguard // modernc's embedded SQLite driver is consumed through database/sql.
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"math"
	pathpkg "path"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

const vectorFormatVersion = 1

// ChunkVector is one durable semantic row. Display evidence is deliberately
// absent and must be joined from current local chunk bytes.
type ChunkVector struct {
	RelPath       string
	NoteHash      [sha256.Size]byte
	Ordinal       uint32
	SubmittedHash [sha256.Size]byte
	Vector        []float32
	manifestSeal  [sha256.Size]byte
}

// ChunkTarget is one exact row in a staging generation's complete target
// manifest. It exists before provider work begins; only its vector is pending.
type ChunkTarget struct {
	RelPath       string
	NoteHash      [sha256.Size]byte
	Ordinal       uint32
	SubmittedHash [sha256.Size]byte
	manifestSeal  [sha256.Size]byte
}

const manifestSealVersion = "yomihon-semantic-manifest-target-v1"

// Target returns the store-bound identity of d. It recomputes the submitted
// digest from the exact provider bytes, so callers never copy hash fields into
// the generation store by hand.
func (d *CorpusChunk) Target() (ChunkTarget, error) {
	if !validRelPath(d.RelPath) || d.NoteHash == ([sha256.Size]byte{}) || len(d.Submitted) == 0 ||
		sha256.Sum256(d.Submitted) != d.SubmittedHash {
		return ChunkTarget{}, fmt.Errorf("%w: embedding chunk identity", ErrInvalidCorpus)
	}
	target := ChunkTarget{
		RelPath:       d.RelPath,
		NoteHash:      d.NoteHash,
		Ordinal:       d.Ordinal,
		SubmittedHash: d.SubmittedHash,
	}
	return bindChunkTarget(&target), nil
}

// Complete binds one validated provider vector to d's exact target identity.
// The returned value owns a copy of the vector.
func (d *CorpusChunk) Complete(result EmbeddingResult) (ChunkVector, error) {
	target, err := d.Target()
	if err != nil {
		return ChunkVector{}, err
	}
	return target.complete(result.Vector), nil
}

func (t *ChunkTarget) complete(vector []float32) ChunkVector {
	return ChunkVector{
		RelPath:       t.RelPath,
		NoteHash:      t.NoteHash,
		Ordinal:       t.Ordinal,
		SubmittedHash: t.SubmittedHash,
		Vector:        slices.Clone(vector),
		manifestSeal:  t.manifestSeal,
	}
}

func bindChunkTarget(target *ChunkTarget) ChunkTarget {
	bound := *target
	bound.manifestSeal = chunkTargetSeal(&bound)
	return bound
}

func bindChunkVector(row *ChunkVector) ChunkVector {
	bound := *row
	target := ChunkTarget{
		RelPath:       row.RelPath,
		NoteHash:      row.NoteHash,
		Ordinal:       row.Ordinal,
		SubmittedHash: row.SubmittedHash,
	}
	bound.manifestSeal = chunkTargetSeal(&target)
	return bound
}

func validateChunkTarget(target *ChunkTarget) error {
	if target == nil {
		return fmt.Errorf("%w: chunk identity is nil", ErrInvalidCorpus)
	}
	if target.NoteHash == ([sha256.Size]byte{}) || target.SubmittedHash == ([sha256.Size]byte{}) ||
		target.manifestSeal != chunkTargetSeal(target) {
		return fmt.Errorf("%w: chunk identity is not bound to source bytes", ErrInvalidCorpus)
	}
	return nil
}

func chunkTargetSeal(target *ChunkTarget) [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(manifestSealVersion))
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(target.RelPath))) // #nosec G115 -- string length is non-negative
	_, _ = h.Write(size[:])
	_, _ = h.Write([]byte(target.RelPath))
	_, _ = h.Write(target.NoteHash[:])
	var ordinal [4]byte
	binary.BigEndian.PutUint32(ordinal[:], target.Ordinal)
	_, _ = h.Write(ordinal[:])
	_, _ = h.Write(target.SubmittedHash[:])
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return result
}

type storedGeneration struct {
	id                      int64
	vectorFormatVersion     int64
	identity                generationIdentity
	policySourceFingerprint [sha256.Size]byte
	corpusFingerprint       [sha256.Size]byte
	expectedChunks          int
	topKP95                 time.Duration
}

func storedGenerationByID(ctx context.Context, q *catalog.Queries, id int64) (storedGeneration, error) {
	row, err := q.GenerationByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return storedGeneration{}, fmt.Errorf("%w: missing generation %d", ErrStoreCorrupt, id)
	}
	if err != nil {
		return storedGeneration{}, fmt.Errorf("read semantic generation metadata: %w", err)
	}
	return decodeStoredGeneration(&row, id)
}

func decodeStoredGeneration(row *catalog.Generation, id int64) (storedGeneration, error) {
	if !validStoredGenerationShape(row, id) {
		return storedGeneration{}, fmt.Errorf("%w: malformed generation metadata", ErrStoreCorrupt)
	}
	dimension, err := storedDimension(row.Dimension)
	if err != nil {
		return storedGeneration{}, err
	}
	expected, err := storedExpectedChunks(row.ExpectedChunks)
	if err != nil {
		return storedGeneration{}, err
	}
	topKP95, err := storedDuration(row.TopKP95Us)
	if err != nil {
		return storedGeneration{}, err
	}
	identity := generationIdentity{
		model:               row.Model,
		dimension:           dimension,
		vectorFormatVersion: int(row.VectorFormatVersion),
		vaultRoot:           row.VaultRoot,
	}
	copy(identity.protocolEpoch[:], row.ProtocolEpoch)
	copy(identity.chunkerEpoch[:], row.ChunkerEpoch)
	copy(identity.corpusPolicyFingerprint[:], row.CorpusPolicyFingerprint)
	if err := validateIdentity(&identity); err != nil {
		return storedGeneration{}, fmt.Errorf("%w: %w", ErrStoreCorrupt, err)
	}
	stored := storedGeneration{
		id:                  id,
		vectorFormatVersion: row.VectorFormatVersion,
		identity:            identity,
		expectedChunks:      expected,
		topKP95:             topKP95,
	}
	copy(stored.policySourceFingerprint[:], row.PolicySourceFingerprint)
	copy(stored.corpusFingerprint[:], row.TargetCorpusFingerprint)
	return stored, nil
}

func validStoredGenerationShape(row *catalog.Generation, id int64) bool {
	return row != nil && row.ID == id && row.VectorFormatVersion > 0 &&
		len(row.ProtocolEpoch) == sha256.Size &&
		len(row.ChunkerEpoch) == sha256.Size &&
		len(row.CorpusPolicyFingerprint) == sha256.Size &&
		len(row.PolicySourceFingerprint) == sha256.Size &&
		len(row.TargetCorpusFingerprint) == sha256.Size
}

func storedDimension(value int64) (int, error) {
	result, ok := exactInt(value)
	if !ok || result <= 0 {
		return 0, fmt.Errorf("%w: invalid generation dimension", ErrStoreCorrupt)
	}
	return result, nil
}

func storedExpectedChunks(value int64) (int, error) {
	result, ok := exactInt(value)
	if !ok || result < 0 {
		return 0, fmt.Errorf("%w: invalid expected chunk count", ErrStoreCorrupt)
	}
	return result, nil
}

func storedDuration(microseconds int64) (time.Duration, error) {
	if microseconds < 0 || microseconds > math.MaxInt64/int64(time.Microsecond) {
		return 0, fmt.Errorf("%w: invalid top-k p95", ErrStoreCorrupt)
	}
	return time.Duration(microseconds) * time.Microsecond, nil
}

func completeGeneration(ctx context.Context, q *catalog.Queries, metadata *storedGeneration) (loadedGeneration, error) {
	if metadata.vectorFormatVersion != vectorFormatVersion {
		return loadedGeneration{}, ErrVectorFormatMismatch
	}
	if !withinExactScanCapacity(metadata.expectedChunks, metadata.identity.Dimension()) {
		return loadedGeneration{}, ErrIndexCapacity
	}
	rows, err := generationRows(ctx, q, metadata)
	if err != nil {
		return loadedGeneration{}, err
	}
	if validateErr := validateCompleteGenerationRows(ctx, q, metadata, rows, fmt.Errorf("%w: generation row count", ErrStoreCorrupt)); validateErr != nil {
		return loadedGeneration{}, validateErr
	}
	attempts, err := q.GenerationAttemptCount(ctx, metadata.id)
	if err != nil {
		return loadedGeneration{}, fmt.Errorf("read immutable semantic retry state: %w", err)
	}
	if attempts != 0 {
		return loadedGeneration{}, fmt.Errorf("%w: immutable generation has retry state", ErrStoreCorrupt)
	}
	return loadedGeneration{
		identity:          metadata.identity,
		corpusFingerprint: metadata.corpusFingerprint,
		topKP95:           metadata.topKP95,
		rows:              rows,
	}, nil
}

func generationTargets(ctx context.Context, q *catalog.Queries, metadata *storedGeneration) ([]ChunkTarget, error) {
	storedTargets, err := q.GenerationTargets(ctx, metadata.id)
	if err != nil {
		return nil, fmt.Errorf("read semantic generation targets: %w", err)
	}
	if len(storedTargets) != metadata.expectedChunks {
		return nil, fmt.Errorf("%w: generation target count", ErrStoreCorrupt)
	}
	targets := make([]ChunkTarget, 0, len(storedTargets))
	uniqueNotes := 0
	var previousPath string
	for position := range storedTargets {
		target, decodeErr := decodeGenerationTarget(&storedTargets[position], metadata)
		if decodeErr != nil {
			return nil, decodeErr
		}
		targets = append(targets, target)
		if position == 0 || target.RelPath != previousPath {
			uniqueNotes++
			previousPath = target.RelPath
		}
	}
	noteCount, err := q.GenerationNoteCount(ctx, metadata.id)
	if err != nil {
		return nil, fmt.Errorf("read semantic generation note count: %w", err)
	}
	if noteCount != int64(uniqueNotes) {
		return nil, fmt.Errorf("%w: generation note count", ErrStoreCorrupt)
	}
	fingerprint, err := TargetFingerprint(targets)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid generation corpus: %w", ErrStoreCorrupt, err)
	}
	if fingerprint != metadata.corpusFingerprint {
		return nil, fmt.Errorf("%w: generation corpus fingerprint", ErrStoreCorrupt)
	}
	return targets, nil
}

func decodeGenerationTarget(
	stored *catalog.GenerationTargetsRow,
	metadata *storedGeneration,
) (ChunkTarget, error) {
	if stored.GenerationID != metadata.id || stored.Ordinal < 0 || stored.Ordinal > math.MaxUint32 ||
		len(stored.NoteHash) != sha256.Size || len(stored.SubmittedHash) != sha256.Size ||
		!validRelPath(stored.RelPath) {
		return ChunkTarget{}, fmt.Errorf("%w: malformed generation target", ErrStoreCorrupt)
	}
	target := ChunkTarget{RelPath: stored.RelPath, Ordinal: uint32(stored.Ordinal)}
	copy(target.NoteHash[:], stored.NoteHash)
	copy(target.SubmittedHash[:], stored.SubmittedHash)
	return bindChunkTarget(&target), nil
}

func validateCompleteGenerationRows(
	ctx context.Context,
	q *catalog.Queries,
	metadata *storedGeneration,
	rows []ChunkVector,
	incomplete error,
) error {
	if len(rows) != metadata.expectedChunks {
		return incomplete
	}
	pendingCount, err := q.GenerationPendingCount(ctx, metadata.id)
	if err != nil {
		return fmt.Errorf("read semantic generation pending count: %w", err)
	}
	if pendingCount != 0 {
		return incomplete
	}
	fingerprint, err := CorpusFingerprint(rows)
	if err != nil {
		return fmt.Errorf("%w: invalid generation corpus: %w", ErrStoreCorrupt, err)
	}
	if fingerprint != metadata.corpusFingerprint {
		return fmt.Errorf("%w: generation corpus fingerprint", ErrStoreCorrupt)
	}
	noteCount, err := q.GenerationNoteCount(ctx, metadata.id)
	if err != nil {
		return fmt.Errorf("read semantic generation note count: %w", err)
	}
	if noteCount != int64(countGenerationNotes(rows)) {
		return fmt.Errorf("%w: generation note count", ErrStoreCorrupt)
	}
	return nil
}

func countGenerationNotes(rows []ChunkVector) int {
	count := 0
	var previous string
	for i := range rows {
		if i == 0 || rows[i].RelPath != previous {
			count++
			previous = rows[i].RelPath
		}
	}
	return count
}

func generationRows(ctx context.Context, q *catalog.Queries, metadata *storedGeneration) ([]ChunkVector, error) {
	const pageSize = 256

	rows := make([]ChunkVector, 0, metadata.expectedChunks)
	afterPath := ""
	afterOrdinal := int64(-1)
	for {
		storedRows, err := q.GenerationChunkPage(ctx, catalog.GenerationChunkPageParams{
			GenerationID: metadata.id,
			AfterRelPath: afterPath,
			AfterOrdinal: afterOrdinal,
			PageSize:     pageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("read semantic generation row page: %w", err)
		}
		for position := range storedRows {
			stored := &storedRows[position]
			row, decodeErr := decodeGenerationRow(stored, metadata)
			if decodeErr != nil {
				return nil, decodeErr
			}
			rows = append(rows, row)
		}
		if len(storedRows) < pageSize {
			return rows, nil
		}
		last := &storedRows[len(storedRows)-1]
		afterPath = last.RelPath
		afterOrdinal = last.Ordinal
	}
}

func decodeGenerationRow(
	stored *catalog.GenerationChunkPageRow,
	metadata *storedGeneration,
) (ChunkVector, error) {
	if stored.GenerationID != metadata.id || stored.Ordinal < 0 || stored.Ordinal > math.MaxUint32 ||
		len(stored.NoteHash) != sha256.Size || len(stored.SubmittedHash) != sha256.Size {
		return ChunkVector{}, fmt.Errorf("%w: malformed chunk metadata", ErrStoreCorrupt)
	}
	vector, err := decodeStoredVector(stored.Vector, metadata.identity.dimension)
	if err != nil {
		return ChunkVector{}, err
	}
	row := ChunkVector{RelPath: stored.RelPath, Ordinal: uint32(stored.Ordinal), Vector: vector}
	copy(row.NoteHash[:], stored.NoteHash)
	copy(row.SubmittedHash[:], stored.SubmittedHash)
	row = bindChunkVector(&row)
	if err := validateChunkVector(&row, metadata.identity.dimension); err != nil {
		return ChunkVector{}, fmt.Errorf("%w: %w", ErrStoreCorrupt, err)
	}
	return row, nil
}

// CorpusFingerprint hashes the sorted manifest tuple
// (rel_path, note_hash, ordinal, submitted_hash). Vector values are excluded:
// the manifest binds exact local/provider inputs, while response validation is
// owned by each row and the generation identity.
func CorpusFingerprint(rows []ChunkVector) ([sha256.Size]byte, error) {
	targets := make([]ChunkTarget, len(rows))
	for i := range rows {
		row := &rows[i]
		target := ChunkTarget{
			RelPath:       row.RelPath,
			NoteHash:      row.NoteHash,
			Ordinal:       row.Ordinal,
			SubmittedHash: row.SubmittedHash,
			manifestSeal:  row.manifestSeal,
		}
		if err := validateChunkTarget(&target); err != nil {
			return [sha256.Size]byte{}, err
		}
		targets[i] = target
	}
	return TargetFingerprint(targets)
}

// TargetFingerprint hashes the sorted complete target manifest tuple
// (rel_path, note_hash, ordinal, submitted_hash).
func TargetFingerprint(targets []ChunkTarget) ([sha256.Size]byte, error) {
	_, fingerprint, err := normalizeTargets(targets)
	return fingerprint, err
}

func normalizeTargets(targets []ChunkTarget) ([]ChunkTarget, [sha256.Size]byte, error) {
	ordered := slices.Clone(targets)
	slices.SortFunc(ordered, func(a, b ChunkTarget) int {
		if byPath := strings.Compare(a.RelPath, b.RelPath); byPath != 0 {
			return byPath
		}
		return cmp.Compare(a.Ordinal, b.Ordinal)
	})
	h := sha256.New()
	var previousPath string
	var previousOrdinal uint32
	var previousNoteHash [sha256.Size]byte
	for position := range ordered {
		target := &ordered[position]
		if err := validateChunkTarget(target); err != nil {
			return nil, [sha256.Size]byte{}, err
		}
		if !validRelPath(target.RelPath) {
			return nil, [sha256.Size]byte{}, fmt.Errorf("%w: rel_path %q", ErrInvalidCorpus, target.RelPath)
		}
		if position > 0 && target.RelPath == previousPath && target.Ordinal == previousOrdinal {
			return nil, [sha256.Size]byte{}, fmt.Errorf("%w: duplicate chunk %q/%d", ErrInvalidCorpus, target.RelPath, target.Ordinal)
		}
		if position > 0 && target.RelPath == previousPath && target.NoteHash != previousNoteHash {
			return nil, [sha256.Size]byte{}, fmt.Errorf("%w: inconsistent note hash for %q", ErrInvalidCorpus, target.RelPath)
		}
		writeManifestTarget(h, target)
		previousPath, previousOrdinal, previousNoteHash = target.RelPath, target.Ordinal, target.NoteHash
	}
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return ordered, result, nil
}

func writeManifestTarget(h hash.Hash, target *ChunkTarget) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(target.RelPath)))
	_, _ = h.Write(size[:])
	_, _ = h.Write([]byte(target.RelPath))
	_, _ = h.Write(target.NoteHash[:])
	var ordinal [4]byte
	binary.BigEndian.PutUint32(ordinal[:], target.Ordinal)
	_, _ = h.Write(ordinal[:])
	_, _ = h.Write(target.SubmittedHash[:])
}

func containsControl(value string) bool {
	for _, r := range value {
		if r <= 0x1f || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
	}
	return false
}

func validateChunkVector(row *ChunkVector, dimension int) error {
	if row == nil {
		return ErrInvalidChunk
	}
	target := ChunkTarget{
		RelPath:       row.RelPath,
		NoteHash:      row.NoteHash,
		Ordinal:       row.Ordinal,
		SubmittedHash: row.SubmittedHash,
		manifestSeal:  row.manifestSeal,
	}
	if err := validateChunkTarget(&target); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidChunk, err)
	}
	if !validRelPath(row.RelPath) {
		return fmt.Errorf("%w: rel_path %q", ErrInvalidChunk, row.RelPath)
	}
	if len(row.Vector) != dimension {
		return fmt.Errorf("%w: vector dimension %d, want %d", ErrInvalidChunk, len(row.Vector), dimension)
	}
	if _, ok := vectorMagnitude(row.Vector); !ok {
		return fmt.Errorf("%w: zero or non-finite vector", ErrInvalidChunk)
	}
	return nil
}

func validRelPath(relPath string) bool {
	if !utf8.ValidString(relPath) || relPath == "" || strings.Contains(relPath, `\`) || strings.HasPrefix(relPath, "/") ||
		pathpkg.Clean(relPath) != relPath || relPath == "." || strings.HasPrefix(relPath, "../") {
		return false
	}
	return !containsControl(relPath)
}

func encodeStoredVector(vector []float32) []byte {
	raw := make([]byte, len(vector)*4)
	for position, value := range vector {
		binary.LittleEndian.PutUint32(raw[position*4:], math.Float32bits(value))
	}
	return raw
}

func decodeStoredVector(raw []byte, dimension int) ([]float32, error) {
	if dimension <= 0 || len(raw) != dimension*4 {
		return nil, fmt.Errorf("%w: vector byte length %d, want %d", ErrStoreCorrupt, len(raw), dimension*4)
	}
	vector := make([]float32, dimension)
	for position := range vector {
		value := math.Float32frombits(binary.LittleEndian.Uint32(raw[position*4:]))
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("%w: non-finite vector", ErrStoreCorrupt)
		}
		vector[position] = value
	}
	return vector, nil
}

func exactInt(value int64) (int, bool) {
	converted := int(value)
	return converted, int64(converted) == value
}
