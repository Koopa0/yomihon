package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql" //nolint:depguard // modernc's embedded SQLite driver is consumed through database/sql.
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

// staging is a role-bound handle to the catalog's current staging generation.
// Every mutation rechecks that role inside the same transaction as the write.
type staging struct {
	writer                  *writer
	id                      int64
	identity                generationIdentity
	policySourceFingerprint [sha256.Size]byte
	corpusFingerprint       [sha256.Size]byte
	expectedChunks          int
	resumed                 bool
}

// retryNotReadyError reports the durable earliest retry time without carrying
// note text, query text, provider bytes, or credentials.
type retryNotReadyError struct {
	At time.Time
}

func (e *retryNotReadyError) Error() string {
	return fmt.Sprintf("%s until %s", ErrRetryNotReady, e.At.UTC().Format(time.RFC3339Nano))
}

func (e *retryNotReadyError) Unwrap() error { return ErrRetryNotReady }

// prepare atomically resumes or creates the sole staging generation and
// persists its complete target manifest before any provider request. A valid,
// compatible active generation may supply exact unchanged vectors; corruption
// in that optional reuse source never prevents an explicit fresh rebuild.
func (w *writer) prepare(
	ctx context.Context,
	identity *generationIdentity,
	policySourceFingerprint [sha256.Size]byte,
	targets []ChunkTarget,
) (*staging, error) {
	if w == nil || w.db == nil {
		return nil, ErrStoreNotFound
	}
	if err := w.requireCurrentFiles(); err != nil {
		return nil, err
	}
	manifest, err := newStagingManifest(identity, policySourceFingerprint, targets)
	if err != nil {
		return nil, err
	}
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin semantic staging preparation: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	q := catalog.New(tx)
	roles, err := q.Catalog(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: missing role catalog", ErrStoreCorrupt)
	}
	if err != nil {
		return nil, fmt.Errorf("read semantic role catalog: %w", err)
	}
	resume, prepareErr := prepareExistingStaging(
		ctx,
		q,
		roles.StagingGenerationID,
		manifest,
	)
	if prepareErr != nil {
		return nil, prepareErr
	}
	if resume.resumed {
		if commitErr := w.commit(tx, "semantic staging resume"); commitErr != nil {
			return nil, commitErr
		}
		return manifest.bind(w, resume.id, true), nil
	}

	id, err := createStagingGeneration(ctx, q, roles, manifest)
	if err != nil {
		return nil, err
	}
	if commitErr := w.commit(tx, "semantic staging preparation"); commitErr != nil {
		return nil, commitErr
	}
	return manifest.bind(w, id, false), nil
}

type stagingManifest struct {
	identity                generationIdentity
	policySourceFingerprint [sha256.Size]byte
	corpusFingerprint       [sha256.Size]byte
	targets                 []ChunkTarget
}

func newStagingManifest(
	identity *generationIdentity,
	policySourceFingerprint [sha256.Size]byte,
	targets []ChunkTarget,
) (*stagingManifest, error) {
	if identity == nil {
		return nil, ErrInvalidIdentity
	}
	if err := validateIdentity(identity); err != nil {
		return nil, err
	}
	if policySourceFingerprint == ([sha256.Size]byte{}) {
		return nil, fmt.Errorf("%w: empty policy source fingerprint", ErrInvalidCorpus)
	}
	orderedTargets, fingerprint, err := normalizeTargets(targets)
	if err != nil {
		return nil, err
	}
	return &stagingManifest{
		identity:                *identity,
		policySourceFingerprint: policySourceFingerprint,
		corpusFingerprint:       fingerprint,
		targets:                 orderedTargets,
	}, nil
}

func (m *stagingManifest) bind(writer *writer, id int64, resumed bool) *staging {
	return &staging{
		writer:                  writer,
		id:                      id,
		identity:                m.identity,
		policySourceFingerprint: m.policySourceFingerprint,
		corpusFingerprint:       m.corpusFingerprint,
		expectedChunks:          len(m.targets),
		resumed:                 resumed,
	}
}

func prepareExistingStaging(
	ctx context.Context,
	q *catalog.Queries,
	stagingID sql.NullInt64,
	manifest *stagingManifest,
) (stagingResume, error) {
	if !stagingID.Valid {
		return stagingResume{}, nil
	}
	id := stagingID.Int64
	admissible, err := stagingMatchesManifest(ctx, q, id, manifest)
	if err != nil {
		return stagingResume{}, err
	}
	if admissible {
		return stagingResume{id: id, resumed: true}, nil
	}
	return stagingResume{}, discardStaging(ctx, q, id)
}

type stagingResume struct {
	id      int64
	resumed bool
}

type stagingInspection struct {
	id    int64
	state StagingGenerationState
}

func (w *writer) inspectStaging(
	ctx context.Context,
	manifest *stagingManifest,
) (stagingInspection, error) {
	if w == nil || w.db == nil {
		return stagingInspection{}, ErrStoreNotFound
	}
	if err := w.requireCurrentFiles(); err != nil {
		return stagingInspection{}, err
	}
	tx, err := w.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return stagingInspection{}, fmt.Errorf("begin semantic staging inspection: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	q := catalog.New(tx)
	roles, err := q.Catalog(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return stagingInspection{}, fmt.Errorf("%w: missing role catalog", ErrStoreCorrupt)
	}
	if err != nil {
		return stagingInspection{}, fmt.Errorf("read semantic role catalog: %w", err)
	}
	inspection, err := inspectStagingGeneration(ctx, q, roles.StagingGenerationID, manifest)
	if err != nil {
		return inspection, err
	}
	if err := w.requireCurrentFiles(); err != nil {
		return stagingInspection{}, err
	}
	return inspection, nil
}

func inspectStagingGeneration(
	ctx context.Context,
	q *catalog.Queries,
	stagingID sql.NullInt64,
	manifest *stagingManifest,
) (stagingInspection, error) {
	if !stagingID.Valid {
		return stagingInspection{state: StagingGenerationAbsent}, nil
	}
	inspection := stagingInspection{id: stagingID.Int64, state: StagingGenerationIncompatible}
	metadata, err := storedGenerationByID(ctx, q, stagingID.Int64)
	if err != nil {
		return inspection, err
	}
	if metadata.vectorFormatVersion != vectorFormatVersion ||
		metadata.identity != manifest.identity ||
		metadata.policySourceFingerprint != manifest.policySourceFingerprint ||
		metadata.corpusFingerprint != manifest.corpusFingerprint ||
		metadata.expectedChunks != len(manifest.targets) {
		return inspection, nil
	}
	admissible, err := stagingGenerationAdmissible(ctx, q, &metadata, manifest.targets)
	if err != nil {
		return inspection, err
	}
	if !admissible {
		return inspection, nil
	}
	exhausted, err := exhaustedPendingAttempt(ctx, q, metadata.id)
	if err != nil {
		return inspection, err
	}
	if exhausted {
		inspection.state = StagingGenerationRequiresAuthorization
	} else {
		inspection.state = StagingGenerationResumable
	}
	return inspection, nil
}

func stagingMatchesManifest(
	ctx context.Context,
	q *catalog.Queries,
	stagingID int64,
	manifest *stagingManifest,
) (bool, error) {
	metadata, err := storedGenerationByID(ctx, q, stagingID)
	if isRecoverableGenerationError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if metadata.vectorFormatVersion != vectorFormatVersion ||
		metadata.identity != manifest.identity ||
		metadata.policySourceFingerprint != manifest.policySourceFingerprint ||
		metadata.corpusFingerprint != manifest.corpusFingerprint ||
		metadata.expectedChunks != len(manifest.targets) {
		return false, nil
	}
	admissible, err := stagingGenerationAdmissible(ctx, q, &metadata, manifest.targets)
	if isRecoverableGenerationError(err) {
		return false, nil
	}
	return admissible, err
}

func discardStaging(ctx context.Context, q *catalog.Queries, stagingID int64) error {
	affected, err := q.ClearStaging(ctx, sql.NullInt64{Int64: stagingID, Valid: true})
	if err != nil {
		return fmt.Errorf("clear stale staging role: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: clear stale staging role", ErrStoreCorrupt)
	}
	affected, err = q.DeleteGeneration(ctx, stagingID)
	if err != nil {
		return fmt.Errorf("discard stale staging generation: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: discard stale staging generation", ErrStoreCorrupt)
	}
	return nil
}

func createStagingGeneration(
	ctx context.Context,
	q *catalog.Queries,
	catalogRow catalog.CatalogRow,
	manifest *stagingManifest,
) (int64, error) {
	id, err := createGeneration(ctx, q, manifest)
	if err != nil {
		return 0, err
	}
	if err := reuseActiveVectors(ctx, q, catalogRow.ActiveGenerationID, id, &manifest.identity); err != nil {
		return 0, err
	}
	if err := publishStaging(ctx, q, id); err != nil {
		return 0, err
	}
	return id, nil
}

func (w *writer) renewStagingGeneration(
	ctx context.Context,
	manifest *stagingManifest,
	expectedID int64,
	validateCommit func(context.Context) error,
) (*staging, error) {
	if validateCommit == nil {
		return nil, ErrAttemptBudgetNotRenewable
	}
	if err := w.requireCurrentFiles(); err != nil {
		return nil, err
	}
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin semantic staging renewal: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	q := catalog.New(tx)
	completed, err := renewableStagingCompletedCount(ctx, q, manifest, expectedID)
	if err != nil {
		return nil, err
	}
	newID, err := replaceExhaustedStaging(ctx, q, manifest, expectedID, completed)
	if err != nil {
		return nil, err
	}
	if w.beforeRenewCommit != nil {
		if hookErr := w.beforeRenewCommit(); hookErr != nil {
			return nil, hookErr
		}
	}
	if err := validateCommit(ctx); err != nil {
		return nil, err
	}
	if err := w.commit(tx, "semantic staging renewal"); err != nil {
		return nil, err
	}
	return manifest.bind(w, newID, true), nil
}

func renewableStagingCompletedCount(
	ctx context.Context,
	q *catalog.Queries,
	manifest *stagingManifest,
	expectedID int64,
) (int, error) {
	roles, err := q.Catalog(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: missing role catalog", ErrStoreCorrupt)
	}
	if err != nil {
		return 0, fmt.Errorf("read semantic role catalog: %w", err)
	}
	inspection, err := inspectStagingGeneration(ctx, q, roles.StagingGenerationID, manifest)
	if err != nil {
		return 0, err
	}
	if inspection.state != StagingGenerationRequiresAuthorization || inspection.id != expectedID {
		return 0, ErrAttemptBudgetNotRenewable
	}
	oldMetadata, err := storedGenerationByID(ctx, q, expectedID)
	if err != nil {
		return 0, err
	}
	completed, err := generationRows(ctx, q, &oldMetadata)
	if err != nil {
		return 0, err
	}
	return len(completed), nil
}

func replaceExhaustedStaging(
	ctx context.Context,
	q *catalog.Queries,
	manifest *stagingManifest,
	expectedID int64,
	completed int,
) (int64, error) {
	newID, err := createGeneration(ctx, q, manifest)
	if err != nil {
		return 0, err
	}
	affected, err := q.CopyGenerationChunkVectors(ctx, catalog.CopyGenerationChunkVectorsParams{
		SourceGenerationID: expectedID,
		TargetGenerationID: newID,
	})
	if err != nil {
		return 0, fmt.Errorf("copy completed semantic staging vectors: %w", err)
	}
	if affected != int64(completed) {
		return 0, fmt.Errorf("%w: copied %d of %d completed staging vectors", ErrStoreCorrupt, affected, completed)
	}
	if publishErr := publishStaging(ctx, q, newID); publishErr != nil {
		return 0, publishErr
	}
	affected, err = q.DeleteGeneration(ctx, expectedID)
	if err != nil {
		return 0, fmt.Errorf("remove exhausted semantic staging generation: %w", err)
	}
	if affected != 1 {
		return 0, fmt.Errorf("%w: remove exhausted semantic staging generation", ErrStoreCorrupt)
	}
	return newID, nil
}

func createGeneration(ctx context.Context, q *catalog.Queries, manifest *stagingManifest) (int64, error) {
	id, err := q.CreateGeneration(ctx, catalog.CreateGenerationParams{
		VectorFormatVersion:     int64(manifest.identity.vectorFormatVersion),
		Model:                   manifest.identity.model,
		Dimension:               int64(manifest.identity.dimension),
		ProtocolEpoch:           manifest.identity.protocolEpoch[:],
		ChunkerEpoch:            manifest.identity.chunkerEpoch[:],
		VaultRoot:               manifest.identity.vaultRoot,
		CorpusPolicyFingerprint: manifest.identity.corpusPolicyFingerprint[:],
		PolicySourceFingerprint: manifest.policySourceFingerprint[:],
		TargetCorpusFingerprint: manifest.corpusFingerprint[:],
		ExpectedChunks:          int64(len(manifest.targets)),
	})
	if err != nil {
		return 0, fmt.Errorf("create semantic staging generation: %w", err)
	}
	if err := insertTargetManifest(ctx, q, id, manifest.targets); err != nil {
		return 0, err
	}
	return id, nil
}

func insertTargetManifest(ctx context.Context, q *catalog.Queries, id int64, targets []ChunkTarget) error {
	var previousPath string
	for position := range targets {
		target := &targets[position]
		if position == 0 || target.RelPath != previousPath {
			if err := insertTargetNote(ctx, q, id, target); err != nil {
				return err
			}
			previousPath = target.RelPath
		}
		affected, err := q.InsertGenerationChunkTarget(ctx, catalog.InsertGenerationChunkTargetParams{
			GenerationID:  id,
			RelPath:       target.RelPath,
			Ordinal:       int64(target.Ordinal),
			SubmittedHash: target.SubmittedHash[:],
		})
		if err != nil {
			return fmt.Errorf("write semantic chunk target: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf("%w: duplicate semantic chunk target", ErrStoreCorrupt)
		}
	}
	return nil
}

func insertTargetNote(ctx context.Context, q *catalog.Queries, id int64, target *ChunkTarget) error {
	affected, err := q.InsertGenerationNote(ctx, catalog.InsertGenerationNoteParams{
		GenerationID: id,
		RelPath:      target.RelPath,
		NoteHash:     target.NoteHash[:],
	})
	if err != nil {
		return fmt.Errorf("write semantic target note: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: duplicate semantic target note", ErrStoreCorrupt)
	}
	return nil
}

func reuseActiveVectors(
	ctx context.Context,
	q *catalog.Queries,
	activeID sql.NullInt64,
	targetID int64,
	identity *generationIdentity,
) error {
	if !activeID.Valid {
		return nil
	}
	active, err := storedGenerationByID(ctx, q, activeID.Int64)
	if isRecoverableGenerationError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if active.vectorFormatVersion != vectorFormatVersion || active.identity != *identity {
		return nil
	}
	if _, generationErr := completeGeneration(ctx, q, &active); isRecoverableGenerationError(generationErr) {
		return nil
	} else if generationErr != nil {
		return generationErr
	}
	_, err = q.ReuseGenerationChunkVectors(ctx, catalog.ReuseGenerationChunkVectorsParams{
		SourceGenerationID: active.id,
		TargetGenerationID: targetID,
	})
	if err != nil {
		return fmt.Errorf("reuse compatible semantic vectors: %w", err)
	}
	return nil
}

func publishStaging(ctx context.Context, q *catalog.Queries, id int64) error {
	affected, err := q.SetStaging(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return fmt.Errorf("publish staging role: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: publish staging role", ErrStoreCorrupt)
	}
	return nil
}

func isRecoverableGenerationError(err error) bool {
	return errors.Is(err, ErrStoreCorrupt) || errors.Is(err, ErrVectorFormatMismatch)
}

func stagingGenerationAdmissible(
	ctx context.Context,
	q *catalog.Queries,
	metadata *storedGeneration,
	targets []ChunkTarget,
) (bool, error) {
	storedTargets, err := generationTargets(ctx, q, metadata)
	if err != nil {
		return false, err
	}
	if !slices.Equal(storedTargets, targets) {
		return false, nil
	}
	completed, err := generationRows(ctx, q, metadata)
	if err != nil {
		return false, err
	}
	pendingCount, err := q.GenerationPendingCount(ctx, metadata.id)
	if err != nil {
		return false, fmt.Errorf("count pending semantic targets: %w", err)
	}
	if int64(len(completed))+pendingCount != int64(metadata.expectedChunks) {
		return false, nil
	}
	attempts, err := q.AttemptsByGeneration(ctx, metadata.id)
	if err != nil {
		return false, fmt.Errorf("read semantic staging attempts: %w", err)
	}
	attemptCount, err := q.GenerationAttemptCount(ctx, metadata.id)
	if err != nil {
		return false, fmt.Errorf("count semantic staging attempts: %w", err)
	}
	if attemptCount != int64(len(attempts)) {
		return false, nil
	}
	for position := range attempts {
		if !validStoredAttempt(&attempts[position], metadata.id) {
			return false, nil
		}
	}
	return true, nil
}

func validStoredAttempt(attempt *catalog.AttemptsByGenerationRow, generationID int64) bool {
	return attempt.GenerationID == generationID &&
		validRelPath(attempt.RelPath) &&
		attempt.Ordinal >= 0 && attempt.Ordinal <= math.MaxUint32 &&
		attempt.Attempts >= 1 && attempt.Attempts <= 5 &&
		(!attempt.RetryNotBeforeUnixMs.Valid || attempt.RetryNotBeforeUnixMs.Int64 >= 0) &&
		attempt.Pending
}

// resumedFromStore reports whether prepare admitted an existing matching staging
// generation instead of creating a new one.
func (s *staging) resumedFromStore() bool { return s != nil && s.resumed }

// rows returns a copy of every currently persisted staging row in deterministic
// key order.
func (s *staging) rows(ctx context.Context) ([]ChunkVector, error) {
	tx, q, err := s.beginStaging(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	metadata, err := storedGenerationByID(ctx, q, s.id)
	if err != nil {
		return nil, err
	}
	rows, err := generationRows(ctx, q, &metadata)
	if err != nil {
		return nil, err
	}
	if err := s.writer.commit(tx, "semantic staging read"); err != nil {
		return nil, err
	}
	return rows, nil
}

// pending returns the exact target rows that still lack a vector.
func (s *staging) pending(ctx context.Context) ([]ChunkTarget, error) {
	tx, q, err := s.beginStaging(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	stored, err := q.PendingGenerationChunks(ctx, s.id)
	if err != nil {
		return nil, fmt.Errorf("read pending semantic targets: %w", err)
	}
	targets := make([]ChunkTarget, 0, len(stored))
	for _, row := range stored {
		if row.GenerationID != s.id || row.Ordinal < 0 || row.Ordinal > math.MaxUint32 ||
			len(row.NoteHash) != sha256.Size || len(row.SubmittedHash) != sha256.Size || !validRelPath(row.RelPath) {
			return nil, fmt.Errorf("%w: malformed pending target", ErrStoreCorrupt)
		}
		target := ChunkTarget{RelPath: row.RelPath, Ordinal: uint32(row.Ordinal)}
		copy(target.NoteHash[:], row.NoteHash)
		copy(target.SubmittedHash[:], row.SubmittedHash)
		targets = append(targets, bindChunkTarget(&target))
	}
	if err := s.writer.commit(tx, "pending semantic target read"); err != nil {
		return nil, err
	}
	return targets, nil
}

func (s *staging) hasExhaustedPending(ctx context.Context) (bool, error) {
	tx, q, err := s.beginStaging(ctx, true)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	exhausted, err := exhaustedPendingAttempt(ctx, q, s.id)
	if err != nil {
		return false, err
	}
	if err := s.writer.commit(tx, "semantic staging attempt inspection"); err != nil {
		return false, err
	}
	return exhausted, nil
}

func exhaustedPendingAttempt(ctx context.Context, q *catalog.Queries, generationID int64) (bool, error) {
	attempts, err := q.AttemptsByGeneration(ctx, generationID)
	if err != nil {
		return false, fmt.Errorf("read semantic staging attempts: %w", err)
	}
	for i := range attempts {
		attempt := &attempts[i]
		if !validStoredAttempt(attempt, generationID) {
			return false, fmt.Errorf("%w: malformed semantic staging attempt", ErrStoreCorrupt)
		}
		if attempt.Attempts == 5 {
			return true, nil
		}
	}
	return false, nil
}

// put writes one validated row only to the current staging role and clears the
// row's retry ledger entry in the same transaction.
func (s *staging) put(ctx context.Context, row *ChunkVector) error {
	tx, q, err := s.beginStaging(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	if row == nil {
		return ErrInvalidChunk
	}
	if validationErr := validateChunkVector(row, s.identity.dimension); validationErr != nil {
		return validationErr
	}
	affected, err := q.CompleteGenerationChunk(ctx, catalog.CompleteGenerationChunkParams{
		Vector:              encodeStoredVector(row.Vector),
		TargetGenerationID:  s.id,
		TargetRelPath:       row.RelPath,
		TargetOrdinal:       int64(row.Ordinal),
		TargetSubmittedHash: row.SubmittedHash[:],
		TargetNoteHash:      row.NoteHash[:],
	})
	if err != nil {
		return fmt.Errorf("write semantic staging row: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: row does not match a pending target", ErrInvalidChunk)
	}
	affected, err = q.DeleteAttempt(ctx, catalog.DeleteAttemptParams{
		GenerationID: s.id,
		RelPath:      row.RelPath,
		Ordinal:      int64(row.Ordinal),
	})
	if err != nil {
		return fmt.Errorf("clear semantic staging retry: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: row has no reserved provider attempt", ErrInvalidChunk)
	}
	if err := s.writer.commit(tx, "semantic staging row"); err != nil {
		return err
	}
	return nil
}

// setTopKP95 records the generation-local exact-scan observation in integer
// microseconds. Sub-microsecond precision is intentionally discarded.
func (s *staging) setTopKP95(ctx context.Context, duration time.Duration) error {
	if duration < 0 {
		return fmt.Errorf("%w: negative top-k p95", ErrInvalidCorpus)
	}
	tx, q, err := s.beginStaging(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	affected, err := q.SetGenerationTopKP95(ctx, catalog.SetGenerationTopKP95Params{
		TopKP95Us: duration.Microseconds(),
		ID:        s.id,
	})
	if err != nil {
		return fmt.Errorf("write top-k p95: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: write top-k p95", ErrStoreCorrupt)
	}
	if err := s.writer.commit(tx, "top-k p95"); err != nil {
		return err
	}
	return nil
}

// reserveAttempt durably consumes one of the explicit build's five
// pre-transport send slots before the chunk-send capability is invoked.
func (s *staging) reserveAttempt(
	ctx context.Context,
	relPath string,
	ordinal uint32,
	now time.Time,
) (int, error) {
	if !validRelPath(relPath) {
		return 0, fmt.Errorf("%w: rel_path %q", ErrInvalidChunk, relPath)
	}
	tx, q, err := s.beginStaging(ctx, false)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	key := catalog.AttemptByKeyParams{GenerationID: s.id, RelPath: relPath, Ordinal: int64(ordinal)}
	attempt, err := q.AttemptByKey(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		affected, insertErr := q.InsertAttempt(ctx, catalog.InsertAttemptParams(key))
		if insertErr != nil {
			return 0, fmt.Errorf("reserve first semantic chunk attempt: %w", insertErr)
		}
		if affected != 1 {
			return 0, fmt.Errorf("%w: attempt target is not pending", ErrInvalidChunk)
		}
		if commitErr := s.writer.commit(tx, "semantic chunk attempt"); commitErr != nil {
			return 0, commitErr
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read semantic chunk attempt: %w", err)
	}
	if attempt.RetryNotBeforeUnixMs.Valid {
		retryAt := time.UnixMilli(attempt.RetryNotBeforeUnixMs.Int64)
		if now.Before(retryAt) {
			return 0, &retryNotReadyError{At: retryAt}
		}
	}
	if attempt.Attempts >= 5 {
		return 0, ErrAttemptLimit
	}
	affected, err := q.IncrementAttempt(ctx, catalog.IncrementAttemptParams(key))
	if err != nil {
		return 0, fmt.Errorf("reserve semantic chunk attempt: %w", err)
	}
	if affected != 1 {
		return 0, ErrAttemptLimit
	}
	if err := s.writer.commit(tx, "semantic chunk attempt"); err != nil {
		return 0, err
	}
	return int(attempt.Attempts + 1), nil
}

// deferRetry persists the earliest later instant at which the same staging row
// may reserve another request.
func (s *staging) deferRetry(ctx context.Context, relPath string, ordinal uint32, until time.Time) error {
	if !validRelPath(relPath) || until.UnixMilli() < 0 {
		return fmt.Errorf("%w: invalid retry target", ErrInvalidChunk)
	}
	tx, q, err := s.beginStaging(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	affected, err := q.SetRetryNotBefore(ctx, catalog.SetRetryNotBeforeParams{
		RetryNotBeforeUnixMs: sql.NullInt64{Int64: until.UnixMilli(), Valid: true},
		GenerationID:         s.id,
		RelPath:              relPath,
		Ordinal:              int64(ordinal),
	})
	if err != nil {
		return fmt.Errorf("defer semantic chunk retry: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: retry has no reserved attempt", ErrInvalidChunk)
	}
	if err := s.writer.commit(tx, "semantic chunk retry"); err != nil {
		return err
	}
	return nil
}

type activationObserver func([]ChunkVector) (time.Duration, error)

// activateGeneration validates the complete staging row set and retry ledger,
// flips all three catalog roles, and prunes unreferenced generations in one
// transaction. observe measures the newly activated generation's top-k P95
// before the roles flip; a nil observe skips that measurement.
func (s *staging) activateGeneration(ctx context.Context, observe activationObserver) (generationMetadata, error) {
	tx, q, err := s.beginStaging(ctx, false)
	if err != nil {
		return generationMetadata{}, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	metadata, err := storedGenerationByID(ctx, q, s.id)
	if err != nil {
		return generationMetadata{}, err
	}
	if !s.matchesMetadata(&metadata) {
		return generationMetadata{}, fmt.Errorf("%w: staging metadata changed", ErrStoreCorrupt)
	}
	rows, readyErr := activationRows(ctx, q, &metadata)
	if readyErr != nil {
		return generationMetadata{}, readyErr
	}
	observedP95, err := persistActivationObservation(ctx, q, s.id, rows, metadata.topKP95, observe)
	if err != nil {
		return generationMetadata{}, err
	}
	metadata.topKP95 = observedP95
	affected, err := q.ActivateStaging(ctx, sql.NullInt64{Int64: s.id, Valid: true})
	if err != nil {
		return generationMetadata{}, fmt.Errorf("activate semantic generation: %w", err)
	}
	if affected != 1 {
		return generationMetadata{}, ErrStagingClosed
	}
	if _, err := q.DeleteUnreferencedGenerations(ctx); err != nil {
		return generationMetadata{}, fmt.Errorf("prune semantic generations: %w", err)
	}
	if err := s.writer.commit(tx, "semantic generation activation"); err != nil {
		return generationMetadata{}, err
	}
	return generationMetadata{
		identity:          metadata.identity,
		corpusFingerprint: metadata.corpusFingerprint,
		topKP95:           metadata.topKP95,
	}, nil
}

func persistActivationObservation(
	ctx context.Context,
	q *catalog.Queries,
	generationID int64,
	rows []ChunkVector,
	current time.Duration,
	observe activationObserver,
) (time.Duration, error) {
	if observe == nil {
		return current, nil
	}
	p95, err := observe(rows)
	if err != nil {
		return 0, err
	}
	if p95 < 0 {
		return 0, fmt.Errorf("%w: negative top-k p95", ErrInvalidCorpus)
	}
	persisted := time.Duration(p95.Microseconds()) * time.Microsecond
	affected, err := q.SetGenerationTopKP95(ctx, catalog.SetGenerationTopKP95Params{
		TopKP95Us: persisted.Microseconds(),
		ID:        generationID,
	})
	if err != nil {
		return 0, fmt.Errorf("write top-k p95 before activation: %w", err)
	}
	if affected != 1 {
		return 0, fmt.Errorf("%w: write top-k p95 before activation", ErrStoreCorrupt)
	}
	return persisted, nil
}

func (s *staging) matchesMetadata(metadata *storedGeneration) bool {
	return metadata.vectorFormatVersion == vectorFormatVersion &&
		metadata.identity == s.identity &&
		metadata.policySourceFingerprint == s.policySourceFingerprint &&
		metadata.corpusFingerprint == s.corpusFingerprint &&
		metadata.expectedChunks == s.expectedChunks
}

func activationRows(ctx context.Context, q *catalog.Queries, metadata *storedGeneration) ([]ChunkVector, error) {
	rows, err := generationRows(ctx, q, metadata)
	if err != nil {
		return nil, err
	}
	if validateErr := validateCompleteGenerationRows(ctx, q, metadata, rows, ErrGenerationIncomplete); validateErr != nil {
		return nil, validateErr
	}
	attempts, err := q.GenerationAttemptCount(ctx, metadata.id)
	if err != nil {
		return nil, fmt.Errorf("read semantic staging attempts: %w", err)
	}
	if attempts != 0 {
		return nil, ErrGenerationIncomplete
	}
	return rows, nil
}

func (s *staging) beginStaging(ctx context.Context, readOnly bool) (*sql.Tx, *catalog.Queries, error) {
	if s == nil || s.writer == nil || s.writer.db == nil {
		return nil, nil, ErrStagingClosed
	}
	if err := s.writer.requireCurrentFiles(); err != nil {
		return nil, nil, err
	}
	// This is the mutable writer connection. ReadOnly documents the intended
	// query shape only; modernc does not enforce the flag as an access boundary.
	tx, err := s.writer.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
	if err != nil {
		return nil, nil, fmt.Errorf("begin semantic staging transaction: %w", err)
	}
	q := catalog.New(tx)
	roles, err := q.Catalog(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, errors.Join(
			fmt.Errorf("%w: missing role catalog", ErrStoreCorrupt),
			tx.Rollback(),
		)
	}
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("read semantic role catalog: %w", err),
			tx.Rollback(),
		)
	}
	if !roles.StagingGenerationID.Valid || roles.StagingGenerationID.Int64 != s.id {
		return nil, nil, errors.Join(ErrStagingClosed, tx.Rollback())
	}
	return tx, q, nil
}
