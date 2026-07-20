package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	catalogdb "github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

var compareManifestRows = cmp.AllowUnexported(ChunkTarget{}, ChunkVector{})

func TestGenerationStoreRetryLedgerSurvivesRestartAndPreventsSixthSend(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("retry")
	row := fixtureChunkVector("Writing/retry.md", 0, identity.dimension, 1)
	rows := []ChunkVector{row}
	now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)

	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	build := prepareBuild(t, writer, &identity, rows)
	if got, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, now); err != nil || got != 1 {
		t.Fatalf("ReserveAttempt() = (%d, %v), want (1, nil)", got, err)
	}
	retryAt := now.Add(30 * time.Second)
	if err := build.deferRetry(ctx, row.RelPath, row.Ordinal, retryAt); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	writer, openErr = openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	build = prepareBuild(t, writer, &identity, rows)
	if !build.resumedFromStore() {
		t.Fatal("build after restart resumed = false, want true")
	}
	if _, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, retryAt.Add(-time.Millisecond)); !errors.Is(err, ErrRetryNotReady) {
		t.Fatalf("ReserveAttempt() before retry time error = %v, want ErrRetryNotReady", err)
	} else {
		retryErr, ok := errors.AsType[*retryNotReadyError](err)
		if !ok || !retryErr.At.Equal(retryAt) {
			t.Fatalf("retry error = %#v, want retry at %v", err, retryAt)
		}
	}
	for want := 2; want <= 5; want++ {
		got, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, retryAt)
		if err != nil || got != want {
			t.Fatalf("ReserveAttempt() = (%d, %v), want (%d, nil)", got, err, want)
		}
	}
	if _, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, retryAt); !errors.Is(err, ErrAttemptLimit) {
		t.Fatalf("sixth ReserveAttempt() error = %v, want ErrAttemptLimit", err)
	}
	if err := build.put(ctx, &row); err != nil {
		t.Fatal(err)
	}
	if err := build.activate(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationStoreRenewalReadsCommittedHotWAL(t *testing.T) {
	t.Parallel()

	for _, removeSharedMemory := range []bool{false, true} {
		name := "shared memory present"
		if removeSharedMemory {
			name = "shared memory absent"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := fixtureStorePath(t)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), executable, "-test.run=^TestGenerationStoreHotWALHelper$") // #nosec G204 -- executable is this test binary and the argument is a fixed test selector
			command.Env = append(os.Environ(), "YOMIHON_TEST_HOT_WAL=1", "YOMIHON_TEST_STORE_PATH="+path)
			if output, runErr := command.CombinedOutput(); runErr != nil {
				t.Fatalf("create hot WAL: %v\n%s", runErr, output)
			}
			walInfo, err := os.Stat(path + "-wal")
			if err != nil || walInfo.Size() == 0 {
				t.Fatalf("hot WAL = (%v, %v), want non-empty", walInfo, err)
			}
			if removeSharedMemory {
				if removeErr := os.Remove(path + "-shm"); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					t.Fatal(removeErr)
				}
			}
			writer, err := openRenewalWriter(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			identity := fixtureGenerationIdentity("hot-wal")
			row := fixtureChunkVector("Writing/hot-wal.md", 0, identity.dimension, 1)
			target := ChunkTarget{
				RelPath:       row.RelPath,
				NoteHash:      row.NoteHash,
				Ordinal:       row.Ordinal,
				SubmittedHash: row.SubmittedHash,
			}
			manifest, err := newStagingManifest(
				&identity,
				fixturePolicySource(&identity),
				[]ChunkTarget{bindChunkTarget(&target)},
			)
			if err != nil {
				t.Fatal(err)
			}
			inspection, err := writer.inspectStaging(t.Context(), manifest)
			if err != nil {
				t.Fatal(err)
			}
			if inspection.state != StagingGenerationRequiresAuthorization {
				t.Fatalf("staging state = %d, want requires authorization", inspection.state)
			}
			renewed, err := writer.renewStagingGeneration(
				t.Context(),
				manifest,
				inspection.id,
				func(context.Context) error { return nil },
			)
			if err != nil {
				t.Fatal(err)
			}
			if exhausted, err := renewed.hasExhaustedPending(t.Context()); err != nil || exhausted {
				t.Fatalf("renewed staging exhausted = (%t, %v), want (false, nil)", exhausted, err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGenerationStoreRenewalIsAtomicAndPreservesPerTargetDuplicateHashVectors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	writer, openErr := openWriter(ctx, fixtureStorePath(t))
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("renewal-atomic")
	firstActive := []ChunkVector{fixtureChunkVector("Writing/active-one.md", 0, identity.dimension, 10)}
	firstBuild := prepareBuild(t, writer, &identity, firstActive)
	putBuildRows(t, firstBuild, firstActive)
	if activateErr := firstBuild.activate(ctx); activateErr != nil {
		t.Fatal(activateErr)
	}
	secondActive := []ChunkVector{fixtureChunkVector("Writing/active-two.md", 0, identity.dimension, 11)}
	secondBuild := prepareBuild(t, writer, &identity, secondActive)
	putBuildRows(t, secondBuild, secondActive)
	if activateErr := secondBuild.activate(ctx); activateErr != nil {
		t.Fatal(activateErr)
	}

	first := fixtureChunkVector("Writing/duplicate-a.md", 0, identity.dimension, 1)
	second := fixtureChunkVector("Writing/duplicate-b.md", 0, identity.dimension, 2)
	second.SubmittedHash = first.SubmittedHash
	second = bindChunkVector(&second)
	pending := fixtureChunkVector("Writing/pending.md", 0, identity.dimension, 3)
	targetRows := []ChunkVector{first, second, pending}
	exhausted := prepareBuild(t, writer, &identity, targetRows)
	for i := range targetRows[:2] {
		row := &targetRows[i]
		if _, reserveErr := exhausted.reserveAttempt(ctx, row.RelPath, row.Ordinal, time.Unix(0, 0)); reserveErr != nil {
			t.Fatal(reserveErr)
		}
		if putErr := exhausted.put(ctx, row); putErr != nil {
			t.Fatal(putErr)
		}
	}
	now := time.Date(2026, time.July, 20, 14, 0, 0, 0, time.UTC)
	for want := 1; want <= 5; want++ {
		if got, reserveErr := exhausted.reserveAttempt(ctx, pending.RelPath, pending.Ordinal, now); reserveErr != nil || got != want {
			t.Fatalf("reserve pending attempt = (%d, %v), want (%d, nil)", got, reserveErr, want)
		}
	}
	targets := make([]ChunkTarget, len(targetRows))
	for i := range targetRows {
		row := &targetRows[i]
		target := ChunkTarget{
			RelPath:       row.RelPath,
			NoteHash:      row.NoteHash,
			Ordinal:       row.Ordinal,
			SubmittedHash: row.SubmittedHash,
		}
		targets[i] = bindChunkTarget(&target)
	}
	manifest, manifestErr := newStagingManifest(&identity, fixturePolicySource(&identity), targets)
	if manifestErr != nil {
		t.Fatal(manifestErr)
	}
	rolesBefore, rolesErr := writer.q.Catalog(ctx)
	if rolesErr != nil {
		t.Fatal(rolesErr)
	}
	var generationsBefore int
	if countErr := writer.db.QueryRowContext(ctx, `SELECT count(*) FROM generations`).Scan(&generationsBefore); countErr != nil {
		t.Fatal(countErr)
	}
	injected := errors.New("injected renewal commit failure")
	writer.beforeRenewCommit = func() error { return injected }
	if _, renewErr := writer.renewStagingGeneration(
		ctx,
		manifest,
		exhausted.id,
		func(context.Context) error { return nil },
	); !errors.Is(renewErr, injected) {
		t.Fatalf("renewStagingGeneration() error = %v, want injected rollback", renewErr)
	}
	rolesAfterFailure, rolesErr := writer.q.Catalog(ctx)
	if rolesErr != nil {
		t.Fatal(rolesErr)
	}
	var generationsAfterFailure int
	if countErr := writer.db.QueryRowContext(ctx, `SELECT count(*) FROM generations`).Scan(&generationsAfterFailure); countErr != nil {
		t.Fatal(countErr)
	}
	if rolesAfterFailure != rolesBefore || generationsAfterFailure != generationsBefore {
		t.Fatalf("failed renewal changed roles/generations = (%+v, %d), want (%+v, %d)", rolesAfterFailure, generationsAfterFailure, rolesBefore, generationsBefore)
	}
	attempt, attemptErr := writer.q.AttemptByKey(ctx, catalogdb.AttemptByKeyParams{
		GenerationID: exhausted.id,
		RelPath:      pending.RelPath,
		Ordinal:      int64(pending.Ordinal),
	})
	if attemptErr != nil || attempt.Attempts != 5 {
		t.Fatalf("failed renewal attempt ledger = (%+v, %v), want five slots on old staging", attempt, attemptErr)
	}

	writer.beforeRenewCommit = nil
	renewed, renewErr := writer.renewStagingGeneration(
		ctx,
		manifest,
		exhausted.id,
		func(context.Context) error { return nil },
	)
	if renewErr != nil {
		t.Fatal(renewErr)
	}
	rolesAfterSuccess, rolesErr := writer.q.Catalog(ctx)
	if rolesErr != nil {
		t.Fatal(rolesErr)
	}
	if rolesAfterSuccess.ActiveGenerationID != rolesBefore.ActiveGenerationID ||
		rolesAfterSuccess.PreviousGenerationID != rolesBefore.PreviousGenerationID ||
		!rolesAfterSuccess.StagingGenerationID.Valid || rolesAfterSuccess.StagingGenerationID.Int64 != renewed.id ||
		renewed.id == exhausted.id {
		t.Fatalf("successful renewal roles = %+v, want unchanged active/previous and new staging %d", rolesAfterSuccess, renewed.id)
	}
	if _, lookupErr := writer.q.GenerationByID(ctx, exhausted.id); lookupErr == nil {
		t.Fatal("old exhausted generation remains addressable after renewal")
	}
	completed, rowsErr := renewed.rows(ctx)
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	wantCompleted := []ChunkVector{first, second}
	if diff := cmp.Diff(wantCompleted, completed, compareManifestRows); diff != "" {
		t.Fatalf("renewed duplicate-hash vectors differ (-want +got):\n%s", diff)
	}
	pendingRows, pendingErr := renewed.pending(ctx)
	if pendingErr != nil {
		t.Fatal(pendingErr)
	}
	if len(pendingRows) != 1 || pendingRows[0].RelPath != pending.RelPath {
		t.Fatalf("renewed pending rows = %+v, want only %q", pendingRows, pending.RelPath)
	}
	if exhausted, inspectErr := renewed.hasExhaustedPending(ctx); inspectErr != nil || exhausted {
		t.Fatalf("renewed staging exhausted = (%t, %v), want (false, nil)", exhausted, inspectErr)
	}
}

func TestGenerationStoreHotWALHelper(t *testing.T) {
	if os.Getenv("YOMIHON_TEST_HOT_WAL") != "1" {
		return
	}
	path := os.Getenv("YOMIHON_TEST_STORE_PATH")
	writer, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(t.Context(), `PRAGMA main.wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	identity := fixtureGenerationIdentity("hot-wal")
	row := fixtureChunkVector("Writing/hot-wal.md", 0, identity.dimension, 1)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	for want := 1; want <= 5; want++ {
		if got, err := build.reserveAttempt(t.Context(), row.RelPath, row.Ordinal, time.Unix(1, 0)); err != nil || got != want {
			t.Fatalf("reserve attempt = (%d, %v), want (%d, nil)", got, err, want)
		}
	}
	os.Exit(0)
}

func TestGenerationStoreActiveReadIsOldOrNewAcrossActivation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("snapshot")
	oldRows := []ChunkVector{fixtureChunkVector("Writing/old.md", 0, identity.dimension, 1)}
	newRows := []ChunkVector{fixtureChunkVector("Writing/new.md", 0, identity.dimension, 2)}

	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	oldBuild := prepareBuild(t, writer, &identity, oldRows)
	putBuildRows(t, oldBuild, oldRows)
	if err := oldBuild.activate(ctx); err != nil {
		t.Fatal(err)
	}
	newBuild := prepareBuild(t, writer, &identity, newRows)
	putBuildRows(t, newBuild, newRows)

	reader, err := openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, reader)
	metadataRead := make(chan struct{})
	continueRead := make(chan struct{})
	reader.afterGeneration = func() {
		close(metadataRead)
		<-continueRead
	}
	type activeResult struct {
		generation loadedGeneration
		err        error
	}
	result := make(chan activeResult, 1)
	go func() {
		generation, err := reader.Active(ctx)
		result <- activeResult{generation: generation, err: err}
	}()
	<-metadataRead
	if err := newBuild.activate(ctx); err != nil {
		t.Fatal(err)
	}
	close(continueRead)
	oldSnapshot := <-result
	if oldSnapshot.err != nil {
		t.Fatal(oldSnapshot.err)
	}
	if diff := cmp.Diff(oldRows, oldSnapshot.generation.rows, compareManifestRows); diff != "" {
		t.Errorf("in-flight Active() tore across activation (-want old +got):\n%s", diff)
	}
	reader.afterGeneration = nil
	assertActiveGeneration(t, reader, &identity, newRows, 0)
}

func TestGenerationStoreActivationRollsBackCatalogAndPruneFailures(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("activation-rollback")
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	firstRows := []ChunkVector{fixtureChunkVector("Writing/first.md", 0, identity.dimension, 1)}
	secondRows := []ChunkVector{fixtureChunkVector("Writing/second.md", 0, identity.dimension, 2)}
	thirdRows := []ChunkVector{fixtureChunkVector("Writing/third.md", 0, identity.dimension, 3)}

	first := prepareBuild(t, writer, &identity, firstRows)
	putBuildRows(t, first, firstRows)
	if err := first.activate(ctx); err != nil {
		t.Fatal(err)
	}
	second := prepareBuild(t, writer, &identity, secondRows)
	putBuildRows(t, second, secondRows)
	if _, err := writer.db.ExecContext(ctx, `
		CREATE TEMP TRIGGER fail_activation
		BEFORE UPDATE OF active_generation_id ON catalog
		BEGIN SELECT RAISE(FAIL, 'injected activation failure'); END;
	`); err != nil {
		t.Fatal(err)
	}
	if err := second.activate(ctx); err == nil {
		t.Fatal("Activate() with catalog trigger error = nil")
	}
	assertActiveGeneration(t, writer, &identity, firstRows, 0)
	if _, err := writer.db.ExecContext(ctx, `DROP TRIGGER fail_activation`); err != nil {
		t.Fatal(err)
	}
	if err := second.activate(ctx); err != nil {
		t.Fatal(err)
	}

	third := prepareBuild(t, writer, &identity, thirdRows)
	putBuildRows(t, third, thirdRows)
	if _, err := writer.db.ExecContext(ctx, `
		CREATE TEMP TRIGGER fail_prune
		BEFORE DELETE ON generations
		BEGIN SELECT RAISE(FAIL, 'injected prune failure'); END;
	`); err != nil {
		t.Fatal(err)
	}
	if err := third.activate(ctx); err == nil {
		t.Fatal("Activate() with prune trigger error = nil")
	}
	assertActiveGeneration(t, writer, &identity, secondRows, 0)
	catalog, err := writer.q.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.StagingGenerationID.Valid || catalog.StagingGenerationID.Int64 != third.id {
		t.Fatalf("catalog after rolled-back prune = %+v, want third still staging", catalog)
	}
	if _, err := writer.db.ExecContext(ctx, `DROP TRIGGER fail_prune`); err != nil {
		t.Fatal(err)
	}
	if err := third.activate(ctx); err != nil {
		t.Fatal(err)
	}
	assertActiveGeneration(t, writer, &identity, thirdRows, 0)
}

func TestGenerationStoreCorruptionIsScopedByRoleAndActiveNeverFallsBack(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("corruption")
	firstRows := []ChunkVector{fixtureChunkVector("Writing/first.md", 0, identity.dimension, 1)}
	secondRows := []ChunkVector{fixtureChunkVector("Writing/second.md", 0, identity.dimension, 2)}

	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	first := prepareBuild(t, writer, &identity, firstRows)
	putBuildRows(t, first, firstRows)
	if err := first.activate(ctx); err != nil {
		t.Fatal(err)
	}
	second := prepareBuild(t, writer, &identity, secondRows)
	putBuildRows(t, second, secondRows)
	if err := second.activate(ctx); err != nil {
		t.Fatal(err)
	}
	catalog, err := writer.q.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.PreviousGenerationID.Valid || !catalog.ActiveGenerationID.Valid {
		t.Fatalf("catalog = %+v, want active and previous", catalog)
	}
	corruptStoredVectorLength(t, writer, catalog.PreviousGenerationID.Int64)
	assertActiveGeneration(t, writer, &identity, secondRows, 0)

	corruptStoredVectorLength(t, writer, catalog.ActiveGenerationID.Int64)
	if _, err := writer.Active(ctx); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("Active() with corrupt active error = %v, want ErrStoreCorrupt", err)
	}
}

func TestGenerationStoreExplicitBuildRecoversFromCorruptActive(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("rebuild-corrupt-active")
	oldRows := []ChunkVector{fixtureChunkVector("Writing/old.md", 0, identity.dimension, 1)}
	newRows := []ChunkVector{fixtureChunkVector("Writing/new.md", 0, identity.dimension, 2)}

	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	oldBuild := prepareBuild(t, writer, &identity, oldRows)
	putBuildRows(t, oldBuild, oldRows)
	if err := oldBuild.activate(ctx); err != nil {
		t.Fatal(err)
	}
	catalog, err := writer.q.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	corruptStoredVectorLength(t, writer, catalog.ActiveGenerationID.Int64)

	rebuild := prepareBuild(t, writer, &identity, newRows)
	if rebuild.resumedFromStore() {
		t.Fatal("build from corrupt active resumed = true, want fresh staging")
	}
	pending, err := rebuild.pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != len(newRows) {
		t.Fatalf("fresh rebuild pending targets = %d, want %d", len(pending), len(newRows))
	}
	putBuildRows(t, rebuild, newRows)
	if err := rebuild.activate(ctx); err != nil {
		t.Fatal(err)
	}
	assertActiveGeneration(t, writer, &identity, newRows, 0)
}

func TestGenerationStoreAttemptAndPutRequireExactPendingTarget(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("exact-pending-target")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})

	if err := build.put(ctx, nil); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("Put(nil) error = %v, want ErrInvalidChunk", err)
	}
	if _, err := build.reserveAttempt(ctx, "Writing/not-a-target.md", 0, time.Now()); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("ReserveAttempt() outside manifest error = %v, want ErrInvalidChunk", err)
	}
	tampered := row
	tampered.SubmittedHash = sha256.Sum256([]byte("different submitted bytes"))
	if err := build.put(ctx, &tampered); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("Put() with changed target identity error = %v, want ErrInvalidChunk", err)
	}
	if err := build.put(ctx, &row); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("Put() without reserved attempt error = %v, want ErrInvalidChunk", err)
	}
	if _, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := build.put(ctx, &row); err != nil {
		t.Fatal(err)
	}
	if _, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, time.Now()); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("ReserveAttempt() for completed target error = %v, want ErrInvalidChunk", err)
	}
}

func TestGenerationStoreActivationRejectsOutstandingRetryLedger(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("activation-retry-ledger")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if _, err := build.reserveAttempt(ctx, row.RelPath, row.Ordinal, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(ctx,
		`UPDATE chunks SET vector = ? WHERE generation_id = ? AND rel_path = ? AND ordinal = ?`,
		encodeStoredVector(row.Vector), build.id, row.RelPath, row.Ordinal,
	); err != nil {
		t.Fatal(err)
	}
	if err := build.activate(ctx); !errors.Is(err, ErrGenerationIncomplete) {
		t.Fatalf("Activate() with retry ledger error = %v, want ErrGenerationIncomplete", err)
	}
	replacement := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if replacement.resumedFromStore() {
		t.Fatal("staging with a completed target and outstanding retry resumed = true")
	}
}

func TestGenerationStoreResumeRejectsOrphanRetryLedger(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("orphan-retry-ledger")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if _, err := writer.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(ctx, `
		INSERT INTO attempts(generation_id, rel_path, ordinal, attempts)
		VALUES (?, 'Writing/orphan.md', 0, 1)
	`, build.id); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}

	replacement := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if replacement.resumedFromStore() {
		t.Fatal("staging with an orphan retry row resumed = true")
	}
}

func TestGenerationStoreActivationRevalidatesBuildMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		update string
		value  any
	}{
		{name: "vector format", update: `UPDATE generations SET vector_format_version = 2 WHERE id = ?`},
		{name: "identity", update: `UPDATE generations SET model = 'tampered-model' WHERE id = ?`},
		{
			name:   "policy source",
			update: `UPDATE generations SET policy_source_fingerprint = ? WHERE id = ?`,
			value:  sha256.Sum256([]byte("tampered policy source")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			path := fixtureStorePath(t)
			identity := fixtureGenerationIdentity("activation-metadata-" + tt.name)
			row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
			writer, openErr := openWriter(ctx, path)
			if openErr != nil {
				t.Fatal(openErr)
			}
			closeOnCleanup(t, writer)
			build := prepareBuild(t, writer, &identity, []ChunkVector{row})
			putBuildRows(t, build, []ChunkVector{row})
			args := []any{build.id}
			if digest, ok := tt.value.([sha256.Size]byte); ok {
				args = []any{digest[:], build.id}
			}
			if _, err := writer.db.ExecContext(ctx, tt.update, args...); err != nil {
				t.Fatal(err)
			}

			if err := build.activate(ctx); !errors.Is(err, ErrStoreCorrupt) {
				t.Fatalf("Activate() after %s mutation error = %v, want ErrStoreCorrupt", tt.name, err)
			}
			if _, err := writer.Active(ctx); !errors.Is(err, ErrNoActiveGeneration) {
				t.Fatalf("Active() after rejected %s mutation error = %v, want ErrNoActiveGeneration", tt.name, err)
			}
		})
	}
}

func TestGenerationStoreDiscardsCorruptStagingBeforeResume(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("discard-corrupt-staging")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	first := prepareBuild(t, writer, &identity, []ChunkVector{row})
	corruptStoredVectorLength(t, writer, first.id)

	replacement := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if replacement.resumedFromStore() {
		t.Fatal("corrupt staging resumed = true, want replacement")
	}
	pending, err := replacement.pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].RelPath != row.RelPath || pending[0].Ordinal != row.Ordinal {
		t.Fatalf("replacement pending targets = %+v, want exact target", pending)
	}
}

func TestGenerationStoreResumeRequiresExactIdentity(t *testing.T) {
	t.Parallel()

	base := fixtureGenerationIdentity("resume-identity")
	tests := map[string]func(*generationIdentity){
		"model": func(identity *generationIdentity) { identity.model += "-changed" },
		"dimension": func(identity *generationIdentity) {
			identity.dimension++
		},
		"protocol": func(identity *generationIdentity) {
			identity.protocolEpoch = sha256.Sum256([]byte("changed protocol"))
		},
		"chunker": func(identity *generationIdentity) {
			identity.chunkerEpoch = sha256.Sum256([]byte("changed chunker"))
		},
		"vault root": func(identity *generationIdentity) { identity.vaultRoot += "-changed" },
		"policy": func(identity *generationIdentity) {
			identity.corpusPolicyFingerprint = sha256.Sum256([]byte("changed policy"))
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			path := fixtureStorePath(t)
			writer, openErr := openWriter(ctx, path)
			if openErr != nil {
				t.Fatal(openErr)
			}
			closeOnCleanup(t, writer)
			row := fixtureChunkVector("Writing/target.md", 0, base.dimension, 1)
			first := prepareBuild(t, writer, &base, []ChunkVector{row})
			changed := base
			mutate(&changed)
			target := row
			target.Vector = make([]float32, changed.dimension)
			target.Vector[0] = 1
			replacement := prepareBuild(t, writer, &changed, []ChunkVector{target})
			if replacement.resumedFromStore() {
				t.Fatalf("staging with changed %s resumed = true", name)
			}
			if err := first.put(ctx, &row); !errors.Is(err, ErrStagingClosed) {
				t.Fatalf("replaced staging put() error = %v, want ErrStagingClosed", err)
			}
		})
	}
}

func TestGenerationStoreActiveHydrationRejectsMalformedFields(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*testing.T, *writer, int64){
		"hidden pending chunk": func(t *testing.T, writer *writer, generationID int64) {
			t.Helper()
			if _, err := writer.db.ExecContext(t.Context(), `
INSERT INTO chunks(generation_id, rel_path, ordinal, submitted_hash, vector)
SELECT ?, rel_path, 1, zeroblob(32), NULL
FROM notes
WHERE generation_id = ?
LIMIT 1`, generationID, generationID); err != nil {
				t.Fatal(err)
			}
		},
		"non-finite vector": func(t *testing.T, writer *writer, generationID int64) {
			t.Helper()
			raw := make([]byte, 4*fixtureGenerationIdentity("hydrate-nan").dimension)
			binary.LittleEndian.PutUint32(raw, math.Float32bits(float32(math.NaN())))
			if _, err := writer.db.ExecContext(t.Context(),
				`UPDATE chunks SET vector = ? WHERE generation_id = ?`, raw, generationID,
			); err != nil {
				t.Fatal(err)
			}
		},
		"short hash": func(t *testing.T, writer *writer, generationID int64) {
			t.Helper()
			if _, err := writer.db.ExecContext(t.Context(), `PRAGMA ignore_check_constraints = ON`); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.db.ExecContext(t.Context(),
				`UPDATE notes SET note_hash = x'00' WHERE generation_id = ?`, generationID,
			); err != nil {
				t.Fatal(err)
			}
		},
		"control in path": func(t *testing.T, writer *writer, generationID int64) {
			t.Helper()
			if _, err := writer.db.ExecContext(t.Context(), `PRAGMA foreign_keys = OFF`); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.db.ExecContext(t.Context(), `PRAGMA ignore_check_constraints = ON`); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.db.ExecContext(t.Context(),
				`UPDATE chunks SET rel_path = char(10) WHERE generation_id = ?`, generationID,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.db.ExecContext(t.Context(),
				`UPDATE notes SET rel_path = char(10) WHERE generation_id = ?`, generationID,
			); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, corrupt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			path := fixtureStorePath(t)
			identity := fixtureGenerationIdentity("hydrate-" + name)
			row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
			writer, openErr := openWriter(ctx, path)
			if openErr != nil {
				t.Fatal(openErr)
			}
			closeOnCleanup(t, writer)
			build := prepareBuild(t, writer, &identity, []ChunkVector{row})
			putBuildRows(t, build, []ChunkVector{row})
			if err := build.activate(ctx); err != nil {
				t.Fatal(err)
			}
			catalog, err := writer.q.Catalog(ctx)
			if err != nil {
				t.Fatal(err)
			}
			corrupt(t, writer, catalog.ActiveGenerationID.Int64)
			if _, err := writer.Active(ctx); !errors.Is(err, ErrStoreCorrupt) {
				t.Fatalf("Active() error = %v, want ErrStoreCorrupt", err)
			}
		})
	}
}

func TestGenerationStoreDiscardsStagingWithWrongExpectedCount(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("wrong-expected-count")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	first := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if _, err := writer.db.ExecContext(ctx,
		`UPDATE generations SET expected_chunks = expected_chunks + 1 WHERE id = ?`, first.id,
	); err != nil {
		t.Fatal(err)
	}
	replacement := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if replacement.resumedFromStore() {
		t.Fatal("staging with wrong expected count resumed = true")
	}
}

func TestGenerationStoreZeroValuesAndDiagnosticsAreSafe(t *testing.T) {
	t.Parallel()

	var zeroStore store
	if err := zeroStore.Close(); err != nil {
		t.Fatalf("zero Store.Close() error = %v", err)
	}
	var build *staging
	row := fixtureChunkVector("Writing/target.md", 0, 4, 1)
	if err := build.put(t.Context(), &row); !errors.Is(err, ErrStagingClosed) {
		t.Fatalf("nil staging put() error = %v, want ErrStagingClosed", err)
	}
	retryAt := time.Date(2026, time.July, 14, 12, 34, 56, 0, time.UTC)
	retryErr := (&retryNotReadyError{At: retryAt}).Error()
	if !strings.Contains(retryErr, retryAt.Format(time.RFC3339Nano)) {
		t.Fatalf("RetryNotReadyError.Error() = %q, want retry instant", retryErr)
	}
	identity := fixtureGenerationIdentity("model-control")
	identity.model = "bad\nmodel"
	if err := validateIdentity(&identity); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("validateIdentity() error = %v, want ErrInvalidIdentity", err)
	}
}

func TestGenerationStoreStaleBuildCannotMutateReplacement(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("stale-handle")
	firstRows := []ChunkVector{fixtureChunkVector("Writing/first.md", 0, identity.dimension, 1)}
	secondRows := []ChunkVector{fixtureChunkVector("Writing/second.md", 0, identity.dimension, 2)}
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	stale := prepareBuild(t, writer, &identity, firstRows)
	replacement := prepareBuild(t, writer, &identity, secondRows)
	if err := stale.put(ctx, &firstRows[0]); !errors.Is(err, ErrStagingClosed) {
		t.Fatalf("stale put() error = %v, want ErrStagingClosed", err)
	}
	putBuildRows(t, replacement, secondRows)
	if err := replacement.activate(ctx); err != nil {
		t.Fatal(err)
	}
	assertActiveGeneration(t, writer, &identity, secondRows, 0)
}

func TestGenerationStoreWriterLeaseIsNonblocking(t *testing.T) {
	t.Parallel()

	path := fixtureStorePath(t)
	first, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, first)
	if _, err := openWriter(t.Context(), path); !errors.Is(err, ErrWriterHeld) {
		t.Fatalf("second openWriter() error = %v, want ErrWriterHeld", err)
	}
	assertPrivateGenerationFiles(t, filepath.Dir(path))
}

func TestGenerationStoreWriterConnectionPragmas(t *testing.T) {
	path := fixtureStorePath(t)
	writer, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	conn, err := writer.db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, conn)

	for _, tt := range []struct {
		name  string
		query string
		want  int
	}{
		{name: "foreign keys", query: `PRAGMA foreign_keys`, want: 1},
		{name: "full synchronous writes", query: `PRAGMA synchronous`, want: 2},
		{name: "five second busy timeout", query: `PRAGMA busy_timeout`, want: 5000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got int
			if err := conn.QueryRowContext(t.Context(), tt.query).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("%s = %d, want %d", tt.query, got, tt.want)
			}
		})
	}

	var journalMode string
	if err := conn.QueryRowContext(t.Context(), `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Errorf("PRAGMA journal_mode = %q, want %q", journalMode, "wal")
	}
}

func TestGenerationStoreRejectsUnsafeSQLiteSidecar(t *testing.T) {
	t.Parallel()

	path := fixtureStorePath(t)
	writer, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	wal := path + "-wal"
	if err := os.Chmod(wal, 0o644); err != nil { // #nosec G302 -- deliberately unsafe fixture must be rejected
		t.Fatal(err)
	}
	if _, err := writer.Active(t.Context()); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("Active() with unsafe WAL error = %v, want ErrStorePermissions", err)
	}
	if err := os.Chmod(wal, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationStoreReaderConnectionRefusesWrites(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, err := openWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	reader, err := openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, reader)
	conn, err := reader.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, conn)

	var queryOnly int
	if err := conn.QueryRowContext(ctx, `PRAGMA query_only`).Scan(&queryOnly); err != nil {
		t.Fatal(err)
	}
	if queryOnly != 1 {
		t.Errorf("PRAGMA query_only = %d, want 1", queryOnly)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE forbidden_reader_write (id INTEGER)`); err == nil {
		t.Fatal("read-only semantic store CREATE TABLE error = nil")
	}
}

func TestGenerationStoreRefusesUnsafeExistingPermissionsWithoutChangingThem(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	parent := filepath.Join(t.TempDir(), "shared-cache")
	if err := os.Mkdir(parent, 0o755); err != nil { // #nosec G301 -- fixture must prove an intentionally unsafe shared directory is rejected unchanged
		t.Fatal(err)
	}
	path := filepath.Join(parent, "vectors.sqlite")
	if _, err := openWriter(ctx, path); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("openWriter() error = %v, want ErrStorePermissions", err)
	}
	info, statErr := os.Stat(parent)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("parent mode after refused openWriter = %o, want unchanged 755", got)
	}

	privateParent := filepath.Join(t.TempDir(), "semantic")
	if err := os.Mkdir(privateParent, 0o700); err != nil {
		t.Fatal(err)
	}
	unsafePath := filepath.Join(privateParent, "vectors.sqlite")
	if err := os.WriteFile(unsafePath, nil, 0o644); err != nil { // #nosec G306 -- deliberately insecure fixture must be refused unchanged
		t.Fatal(err)
	}
	if _, err := openStore(ctx, unsafePath); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("openStore() error = %v, want ErrStorePermissions", err)
	}
	fileInfo, fileStatErr := os.Stat(unsafePath)
	if fileStatErr != nil {
		t.Fatal(fileStatErr)
	}
	if got := fileInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("store mode after refused openStore = %o, want unchanged 644", got)
	}

	unsafeLease := filepath.Join(privateParent, "unsafe.lock")
	if err := os.WriteFile(unsafeLease, nil, 0o644); err != nil { // #nosec G306 -- deliberately insecure fixture must be refused unchanged
		t.Fatal(err)
	}
	leaseParent, err := openStoreParent(filepath.Join(privateParent, "store", "vectors.sqlite"), false)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, leaseParent)
	if _, err := acquireWriterLease(leaseParent, filepath.Base(unsafeLease)); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("acquireWriterLease(0644) error = %v, want ErrStorePermissions", err)
	}
	leaseInfo, leaseStatErr := os.Stat(unsafeLease)
	if leaseStatErr != nil {
		t.Fatal(leaseStatErr)
	}
	if got := leaseInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("lease mode after refusal = %o, want unchanged 644", got)
	}

	leaseTarget := filepath.Join(privateParent, "target.lock")
	if err := os.WriteFile(leaseTarget, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	leaseLink := filepath.Join(privateParent, "linked.lock")
	if err := os.Symlink(leaseTarget, leaseLink); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireWriterLease(leaseParent, filepath.Base(leaseLink)); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("acquireWriterLease(symlink) error = %v, want ErrStorePermissions", err)
	}
}

func TestGenerationStoreLeaseSurvivesDirectoryReplacement(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	base := t.TempDir()
	dir := filepath.Join(base, "semantic")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vectors.sqlite")
	var secondErr error
	first, err := openGenerationWriterWithHooks(ctx, path, false, writerOpenHooks{
		afterLease: func() {
			if renameErr := os.Rename(dir, dir+"-detached"); renameErr != nil {
				t.Fatal(renameErr)
			}
			if mkdirErr := os.Mkdir(dir, 0o700); mkdirErr != nil {
				t.Fatal(mkdirErr)
			}
			second, openErr := openWriter(ctx, path)
			secondErr = openErr
			if second != nil {
				if closeErr := second.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, first)
	if !errors.Is(secondErr, ErrWriterHeld) {
		t.Fatalf("openWriter() after store-directory replacement error = %v, want ErrWriterHeld", secondErr)
	}
}

func TestGenerationStoreWriterRejectsPostLeaseSymlinkReplacement(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	dir := filepath.Join(base, "semantic")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	trap := t.TempDir()
	path := filepath.Join(dir, "vectors.sqlite")
	writer, err := openGenerationWriterWithHooks(t.Context(), path, false, writerOpenHooks{
		afterLease: func() {
			if renameErr := os.Rename(dir, dir+"-detached"); renameErr != nil {
				t.Fatal(renameErr)
			}
			if symlinkErr := os.Symlink(trap, dir); symlinkErr != nil {
				t.Fatal(symlinkErr)
			}
		},
	})
	if writer != nil {
		if closeErr := writer.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("openGenerationWriterWithHooks() error = %v, want ErrStorePermissions", err)
	}
	for _, candidate := range storeFilePaths(filepath.Join(trap, "vectors.sqlite")) {
		if _, statErr := os.Stat(candidate); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("external store artifact %q stat error = %v, want not exist", filepath.Base(candidate), statErr)
		}
	}
}

func TestGenerationStoreCreatesOnlyItsDedicatedPrivateDirectory(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	path := filepath.Join(base, "semantic", "vectors.sqlite")
	writer, openErr := openWriter(t.Context(), path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := parentInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("dedicated directory mode = %o, want 700", got)
	}
}

func TestGenerationStoreCreatesMissingCacheHierarchyWithPrivateModes(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	app := filepath.Join(cache, "yomihon")
	semanticDir := filepath.Join(app, "semantic")
	path := filepath.Join(semanticDir, "vectors.sqlite")
	writer, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	for _, dir := range []string{cache, app, semanticDir} {
		info, statErr := os.Stat(dir)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("directory %q mode = %o, want 700", filepath.Base(dir), got)
		}
	}
}

func TestGenerationStoreRejectsWritableCacheAncestorWithoutMutation(t *testing.T) {
	t.Parallel()

	ancestor := t.TempDir()
	if err := os.Chmod(ancestor, 0o777); err != nil { // #nosec G302 -- deliberately unsafe fixture must be rejected
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(ancestor, 0o700); err != nil { // #nosec G302 -- restore the owner-only mode after the deliberately unsafe fixture
			t.Errorf("restore cache ancestor permissions: %v", err)
		}
	})
	cache := filepath.Join(ancestor, "cache")
	path := filepath.Join(cache, "yomihon", "semantic", "vectors.sqlite")
	if _, err := openWriter(t.Context(), path); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("openWriter() error = %v, want ErrStorePermissions", err)
	}
	if _, err := os.Stat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cache path stat error = %v, want not exist", err)
	}
}

func TestGenerationStoreActivatesImmutableRolesAndPrunes(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "semantic", "vectors.sqlite")
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("stable")

	firstRows := []ChunkVector{fixtureChunkVector("Writing/a.md", 0, identity.dimension, 1)}
	first := prepareBuild(t, writer, &identity, firstRows)
	if first.resumedFromStore() {
		t.Fatal("first build resumed = true, want false")
	}
	putBuildRows(t, first, firstRows)
	if err := first.setTopKP95(ctx, 125*time.Microsecond); err != nil {
		t.Fatal(err)
	}
	if err := first.activate(ctx); err != nil {
		t.Fatalf("first Activate() error: %v", err)
	}
	if err := first.put(ctx, &firstRows[0]); !errors.Is(err, ErrStagingClosed) {
		t.Fatalf("put() after activate() error = %v, want ErrStagingClosed", err)
	}
	assertActiveGeneration(t, writer, &identity, firstRows, 125*time.Microsecond)

	secondRows := []ChunkVector{
		fixtureChunkVector("Writing/a.md", 0, identity.dimension, 2),
		fixtureChunkVector("Writing/b.md", 0, identity.dimension, 3),
	}
	second := prepareBuild(t, writer, &identity, secondRows)
	putBuildRows(t, second, secondRows)
	if err := second.activate(ctx); err != nil {
		t.Fatalf("second Activate() error: %v", err)
	}
	assertActiveGeneration(t, writer, &identity, secondRows, 0)

	thirdRows := []ChunkVector{fixtureChunkVector("Writing/c.md", 0, identity.dimension, 4)}
	third := prepareBuild(t, writer, &identity, thirdRows)
	putBuildRows(t, third, thirdRows)
	if err := third.activate(ctx); err != nil {
		t.Fatalf("third Activate() error: %v", err)
	}
	assertActiveGeneration(t, writer, &identity, thirdRows, 0)

	count, err := writer.q.GenerationCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("GenerationCount() = %d, want active + previous = 2", count)
	}
	catalog, err := writer.q.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.ActiveGenerationID.Valid || !catalog.PreviousGenerationID.Valid || catalog.StagingGenerationID.Valid {
		t.Fatalf("catalog after third activation = %+v, want active + previous and no staging", catalog)
	}
	if catalog.ActiveGenerationID.Int64 == catalog.PreviousGenerationID.Int64 {
		t.Fatal("active and previous generation IDs are equal")
	}
}

func TestGenerationStoreNormalizesNoteMetadata(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("normalized-notes")
	rows := []ChunkVector{
		fixtureChunkVector("Writing/one-note.md", 0, identity.dimension, 1),
		fixtureChunkVector("Writing/one-note.md", 1, identity.dimension, 2),
	}
	build := prepareBuild(t, writer, &identity, rows)
	putBuildRows(t, build, rows)
	if err := build.activate(ctx); err != nil {
		t.Fatal(err)
	}
	catalog, err := writer.q.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	noteCount, err := writer.q.GenerationNoteCount(ctx, catalog.ActiveGenerationID.Int64)
	if err != nil {
		t.Fatal(err)
	}
	if noteCount != 1 {
		t.Fatalf("GenerationNoteCount() = %d, want one note for two chunks", noteCount)
	}
}

func TestGenerationStoreRejectsUnboundOrUnsetManifestDigests(t *testing.T) {
	t.Parallel()

	writer, openErr := openWriter(t.Context(), fixtureStorePath(t))
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("manifest-seal")
	policySource := fixturePolicySource(&identity)
	valid := fixtureChunkTarget("Writing/target.md", 1)

	tests := map[string]ChunkTarget{
		"unbound composite": {
			RelPath:       valid.RelPath,
			NoteHash:      valid.NoteHash,
			Ordinal:       valid.Ordinal,
			SubmittedHash: valid.SubmittedHash,
		},
		"mutated after binding": func() ChunkTarget {
			mutated := valid
			mutated.SubmittedHash[0] ^= 0xff
			return mutated
		}(),
		"zero note digest": func() ChunkTarget {
			zero := valid
			zero.NoteHash = [sha256.Size]byte{}
			return bindChunkTarget(&zero)
		}(),
		"zero submitted digest": func() ChunkTarget {
			zero := valid
			zero.SubmittedHash = [sha256.Size]byte{}
			return bindChunkTarget(&zero)
		}(),
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := writer.prepare(t.Context(), &identity, policySource, []ChunkTarget{target}); !errors.Is(err, ErrInvalidCorpus) {
				t.Fatalf("PrepareTargets() error = %v, want ErrInvalidCorpus", err)
			}
		})
	}
}

func TestGenerationStoreRejectsUnsetPolicySourceFingerprint(t *testing.T) {
	t.Parallel()

	writer, openErr := openWriter(t.Context(), fixtureStorePath(t))
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("unset-policy-source")
	target := fixtureChunkTarget("Writing/target.md", 1)
	policySource := fixturePolicySource(&identity)
	if _, err := writer.prepare(
		t.Context(),
		nil,
		policySource,
		[]ChunkTarget{target},
	); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("PrepareTargets() with nil identity error = %v, want ErrInvalidIdentity", err)
	}

	if _, err := writer.prepare(
		t.Context(),
		&identity,
		[sha256.Size]byte{},
		[]ChunkTarget{target},
	); !errors.Is(err, ErrInvalidCorpus) {
		t.Fatalf("PrepareTargets() with zero policy source error = %v, want ErrInvalidCorpus", err)
	}
}

func TestGenerationStoreSchemaRejectsMalformedVectorLength(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("vector-length-check")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	if _, err := writer.db.ExecContext(ctx,
		`UPDATE chunks SET vector = x'00' WHERE generation_id = ?`, build.id,
	); err == nil {
		t.Fatal("malformed vector UPDATE error = nil, want CHECK rejection")
	}
}

func TestGenerationStoreSchemaRejectsTextInBinaryHashColumns(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("binary-hash-check")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	textHash := strings.Repeat("x", sha256.Size)

	statements := []string{
		`UPDATE generations SET protocol_epoch = ? WHERE id = ?`,
		`UPDATE generations SET chunker_epoch = ? WHERE id = ?`,
		`UPDATE generations SET corpus_policy_fingerprint = ? WHERE id = ?`,
		`UPDATE generations SET policy_source_fingerprint = ? WHERE id = ?`,
		`UPDATE generations SET target_corpus_fingerprint = ? WHERE id = ?`,
		`UPDATE notes SET note_hash = ? WHERE generation_id = ?`,
		`UPDATE chunks SET submitted_hash = ? WHERE generation_id = ?`,
	}
	for _, statement := range statements {
		if _, err := writer.db.ExecContext(ctx, statement, textHash, build.id); err == nil {
			t.Errorf("%s accepted 32-byte TEXT in a binary hash column", statement)
		}
	}
}

func TestGenerationStoreSchemaUsesStrictStorageClassesForEveryTable(t *testing.T) {
	t.Parallel()

	path := fixtureStorePath(t)
	writer, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	rows, err := writer.db.QueryContext(t.Context(), `PRAGMA table_list`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	want := map[string]bool{
		"attempts": false, "catalog": false, "chunks": false,
		"generations": false, "notes": false,
	}
	for rows.Next() {
		var schemaName, name, tableType string
		var columns, withoutRowID, strict int
		if err := rows.Scan(&schemaName, &name, &tableType, &columns, &withoutRowID, &strict); err != nil {
			t.Fatal(err)
		}
		if _, expected := want[name]; expected {
			want[name] = strict == 1
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for name, strict := range want {
		if !strict {
			t.Errorf("table %s is not STRICT", name)
		}
	}
}

func TestGenerationStoreRejectsInvalidUTF8TextDuringHydration(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("invalid-utf8")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, err := openWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	putBuildRows(t, build, []ChunkVector{row})
	if err := build.activate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(ctx,
		`UPDATE generations SET model = CAST(x'ff' AS TEXT) WHERE id = ?`, build.id,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Active(ctx); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("Active() error = %v, want ErrStoreCorrupt", err)
	}
}

func TestEmbeddedStoreSchemaMatchesPinnedFingerprint(t *testing.T) {
	t.Parallel()

	path := fixtureStorePath(t)
	writer, err := openWriter(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	got, err := storeSchemaFingerprint(t.Context(), writer.db)
	if err != nil {
		t.Fatal(err)
	}
	if got != expectedStoreSchemaFingerprint {
		t.Fatalf("embedded store schema fingerprint = %x, want %x", got, expectedStoreSchemaFingerprint)
	}
}

func TestGenerationStoreSQLiteIntegrityPragmasDetectInvalidRows(t *testing.T) {
	t.Parallel()

	t.Run("complete store", func(t *testing.T) {
		t.Parallel()
		writer, openErr := openWriter(t.Context(), fixtureStorePath(t))
		if openErr != nil {
			t.Fatal(openErr)
		}
		closeOnCleanup(t, writer)
		identity := fixtureGenerationIdentity("integrity-ok")
		row := fixtureChunkVector("Writing/ok.md", 0, identity.dimension, 1)
		build := prepareBuild(t, writer, &identity, []ChunkVector{row})
		putBuildRows(t, build, []ChunkVector{row})
		if err := build.activate(t.Context()); err != nil {
			t.Fatal(err)
		}
		var result string
		if err := writer.db.QueryRowContext(t.Context(), `PRAGMA main.integrity_check`).Scan(&result); err != nil {
			t.Fatal(err)
		}
		if result != "ok" {
			t.Fatalf("integrity_check = %q, want %q", result, "ok")
		}
		violations, err := foreignKeyViolationCount(t.Context(), writer.db)
		if err != nil || violations != 0 {
			t.Fatalf("foreign_key_check = (%d, %v), want (0, nil)", violations, err)
		}
	})

	t.Run("check constraint", func(t *testing.T) {
		t.Parallel()
		writer, openErr := openWriter(t.Context(), fixtureStorePath(t))
		if openErr != nil {
			t.Fatal(openErr)
		}
		closeOnCleanup(t, writer)
		if _, err := writer.db.ExecContext(t.Context(), `PRAGMA ignore_check_constraints = ON`); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.db.ExecContext(t.Context(), `UPDATE catalog SET singleton = 2`); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.db.ExecContext(t.Context(), `PRAGMA ignore_check_constraints = OFF`); err != nil {
			t.Fatal(err)
		}
		var result string
		if err := writer.db.QueryRowContext(t.Context(), `PRAGMA main.integrity_check`).Scan(&result); err != nil {
			t.Fatal(err)
		}
		if result == "ok" {
			t.Fatal("integrity_check = ok, want injected CHECK violation")
		}
	})

	t.Run("foreign key", func(t *testing.T) {
		t.Parallel()
		writer, openErr := openWriter(t.Context(), fixtureStorePath(t))
		if openErr != nil {
			t.Fatal(openErr)
		}
		closeOnCleanup(t, writer)
		if _, err := writer.db.ExecContext(t.Context(), `PRAGMA foreign_keys = OFF`); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.db.ExecContext(t.Context(), `UPDATE catalog SET active_generation_id = 9223372036854775807`); err != nil {
			t.Fatal(err)
		}
		violations, err := foreignKeyViolationCount(t.Context(), writer.db)
		if err != nil || violations != 1 {
			t.Fatalf("foreign_key_check = (%d, %v), want (1, nil)", violations, err)
		}
	})
}

func TestGenerationStorePersistsCompleteTargetBeforeEmbedding(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("pending-targets")
	targets := []ChunkTarget{
		fixtureChunkTarget("Writing/a.md", 1),
		fixtureChunkTarget("Writing/b.md", 2),
	}
	build, err := writer.prepare(ctx, &identity, fixturePolicySource(&identity), targets)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := build.pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(targets, pending, compareManifestRows); diff != "" {
		t.Fatalf("Pending() mismatch (-want +got):\n%s", diff)
	}
	completed, err := build.rows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 0 {
		t.Fatalf("Rows() before embedding = %d, want 0", len(completed))
	}
}

func TestGenerationStoreReusesExactSubmittedBytesAfterNoteChanges(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("changed-note-reuse")
	oldNoteHash := sha256.Sum256([]byte("old complete note bytes"))
	newNoteHash := sha256.Sum256([]byte("new complete note bytes"))
	oldRows := []ChunkVector{
		fixtureChunkVector("Writing/note.md", 0, identity.dimension, 1),
		fixtureChunkVector("Writing/note.md", 1, identity.dimension, 2),
	}
	for i := range oldRows {
		oldRows[i].NoteHash = oldNoteHash
		oldRows[i] = bindChunkVector(&oldRows[i])
	}
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	oldBuild := prepareBuild(t, writer, &identity, oldRows)
	putBuildRows(t, oldBuild, oldRows)
	if err := oldBuild.activate(ctx); err != nil {
		t.Fatal(err)
	}

	newRows := []ChunkVector{oldRows[0], oldRows[1]}
	for i := range newRows {
		newRows[i].NoteHash = newNoteHash
	}
	newRows[1].SubmittedHash = sha256.Sum256([]byte("changed submitted chunk bytes"))
	newRows[1].Vector = []float32{0, 0, 0, 1}
	for i := range newRows {
		newRows[i] = bindChunkVector(&newRows[i])
	}
	build := prepareBuild(t, writer, &identity, newRows)
	completed, err := build.rows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]ChunkVector{newRows[0]}, completed, compareManifestRows); diff != "" {
		t.Fatalf("reused rows mismatch (-want unchanged submitted bytes +got):\n%s", diff)
	}
	pending, err := build.pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget := ChunkTarget{
		RelPath:       newRows[1].RelPath,
		NoteHash:      newRows[1].NoteHash,
		Ordinal:       newRows[1].Ordinal,
		SubmittedHash: newRows[1].SubmittedHash,
	}
	wantPending := []ChunkTarget{bindChunkTarget(&wantTarget)}
	if diff := cmp.Diff(wantPending, pending, compareManifestRows); diff != "" {
		t.Fatalf("pending rows mismatch (-want changed submitted bytes +got):\n%s", diff)
	}
}

func TestReuseQueryCannotBypassGenerationIdentity(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	writer, err := openWriter(ctx, fixtureStorePath(t))
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)

	sourceIdentity := fixtureGenerationIdentity("query-identity-source")
	row := fixtureChunkVector("Writing/note.md", 0, sourceIdentity.dimension, 1)
	source := prepareBuild(t, writer, &sourceIdentity, []ChunkVector{row})
	putBuildRows(t, source, []ChunkVector{row})
	if activateErr := source.activate(ctx); activateErr != nil {
		t.Fatal(activateErr)
	}

	targetIdentity := sourceIdentity
	targetIdentity.model = "numerically-incompatible-model"
	target := prepareBuild(t, writer, &targetIdentity, []ChunkVector{row})
	affected, err := writer.q.ReuseGenerationChunkVectors(ctx, catalogdb.ReuseGenerationChunkVectorsParams{
		SourceGenerationID: source.id,
		TargetGenerationID: target.id,
	})
	if err != nil {
		t.Fatal(err)
	}
	if affected != 0 {
		t.Fatalf("ReuseGenerationChunkVectors() affected %d rows across incompatible identities, want 0", affected)
	}
	completed, err := target.rows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 0 {
		t.Fatalf("incompatible direct reuse returned %d completed rows, want 0", len(completed))
	}
}

func TestGenerationStoreRejectsIncompatibleSchemaVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	if _, err := writer.db.ExecContext(ctx, `PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(ctx, path); !errors.Is(err, ErrStoreSchemaMismatch) {
		t.Fatalf("openStore() error = %v, want ErrStoreSchemaMismatch", err)
	}
	if _, err := openWriter(ctx, path); !errors.Is(err, ErrStoreSchemaMismatch) {
		t.Fatalf("openWriter() error = %v, want ErrStoreSchemaMismatch", err)
	}
}

func TestGenerationStoreExplicitRebuildReplacesIncompatibleSchema(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("schema-rebuild")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	putBuildRows(t, build, []ChunkVector{row})
	if err := build.activate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(ctx, `PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	rebuilt, err := openRebuildWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, rebuilt)
	if _, err := rebuilt.Active(ctx); !errors.Is(err, ErrNoActiveGeneration) {
		t.Fatalf("Active() after explicit schema rebuild error = %v, want ErrNoActiveGeneration", err)
	}
	var version int
	if err := rebuilt.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != storeSchemaVersion {
		t.Fatalf("rebuilt schema version = %d, want %d", version, storeSchemaVersion)
	}
}

func TestGenerationStoreExplicitRebuildRecoversCorruptOrStructurallyIncompleteStore(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*testing.T, string){
		"invalid sqlite bytes": func(t *testing.T, path string) {
			t.Helper()
			if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"missing catalog row": func(t *testing.T, path string) {
			t.Helper()
			writer, openErr := openWriter(t.Context(), path)
			if openErr != nil {
				t.Fatal(openErr)
			}
			if _, err := writer.db.ExecContext(t.Context(), `DELETE FROM catalog`); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
		},
	}

	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := fixtureStorePath(t)
			arrange(t, path)
			writer, openErr := openRebuildWriter(t.Context(), path)
			if openErr != nil {
				t.Fatal(openErr)
			}
			closeOnCleanup(t, writer)
			if _, err := writer.Active(t.Context()); !errors.Is(err, ErrNoActiveGeneration) {
				t.Fatalf("Active() after explicit recovery error = %v, want ErrNoActiveGeneration", err)
			}
		})
	}
}

func TestGenerationStoreExplicitRebuildRecoversDanglingCatalogRoles(t *testing.T) {
	t.Parallel()

	for _, role := range []struct {
		name  string
		query string
	}{
		{name: "active_generation_id", query: `UPDATE catalog SET active_generation_id = 9223372036854775807`},
		{name: "previous_generation_id", query: `UPDATE catalog SET previous_generation_id = 9223372036854775807`},
		{name: "staging_generation_id", query: `UPDATE catalog SET staging_generation_id = 9223372036854775807`},
	} {
		t.Run(role.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			path := fixtureStorePath(t)
			writer, err := openWriter(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if _, execErr := writer.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); execErr != nil {
				closeNow(t, writer)
				t.Fatal(execErr)
			}
			if _, execErr := writer.db.ExecContext(ctx, role.query); execErr != nil {
				closeNow(t, writer)
				t.Fatal(execErr)
			}
			if closeErr := writer.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}

			ordinary, err := openWriter(ctx, path)
			if ordinary != nil {
				closeNow(t, ordinary)
			}
			if !errors.Is(err, ErrStoreCorrupt) {
				t.Fatalf("openWriter() error = %v, want ErrStoreCorrupt", err)
			}

			rebuilt, err := openRebuildWriter(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			closeOnCleanup(t, rebuilt)
			if _, err := rebuilt.Active(ctx); !errors.Is(err, ErrNoActiveGeneration) {
				t.Fatalf("Active() after explicit recovery error = %v, want ErrNoActiveGeneration", err)
			}
		})
	}
}

func TestGenerationStoreExplicitRebuildRecoversOrphanedManifestRows(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*testing.T, *writer){
		"note without generation": func(t *testing.T, writer *writer) {
			t.Helper()
			if _, err := writer.db.ExecContext(t.Context(),
				`INSERT INTO notes(generation_id, rel_path, note_hash)
				 VALUES (9223372036854775807, 'Writing/orphan.md', zeroblob(32))`,
			); err != nil {
				t.Fatal(err)
			}
		},
		"chunk without note": func(t *testing.T, writer *writer) {
			t.Helper()
			identity := fixtureGenerationIdentity("orphan-chunk")
			row := fixtureChunkVector("Writing/orphan.md", 0, identity.dimension, 1)
			build := prepareBuild(t, writer, &identity, []ChunkVector{row})
			if _, err := writer.db.ExecContext(t.Context(),
				`DELETE FROM notes WHERE generation_id = ?`, build.id,
			); err != nil {
				t.Fatal(err)
			}
		},
		"attempt without chunk": func(t *testing.T, writer *writer) {
			t.Helper()
			identity := fixtureGenerationIdentity("orphan-attempt")
			row := fixtureChunkVector("Writing/orphan.md", 0, identity.dimension, 1)
			build := prepareBuild(t, writer, &identity, []ChunkVector{row})
			if _, err := build.reserveAttempt(t.Context(), row.RelPath, row.Ordinal, time.Unix(0, 0)); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.db.ExecContext(t.Context(),
				`DELETE FROM chunks WHERE generation_id = ?`, build.id,
			); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, corrupt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			path := fixtureStorePath(t)
			writer, err := openWriter(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if _, execErr := writer.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); execErr != nil {
				closeNow(t, writer)
				t.Fatal(execErr)
			}
			corrupt(t, writer)
			if closeErr := writer.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}

			ordinary, err := openWriter(ctx, path)
			if ordinary != nil {
				closeNow(t, ordinary)
			}
			if !errors.Is(err, ErrStoreCorrupt) {
				t.Fatalf("openWriter() error = %v, want ErrStoreCorrupt", err)
			}

			rebuilt, err := openRebuildWriter(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			closeOnCleanup(t, rebuilt)
			if _, err := rebuilt.Active(ctx); !errors.Is(err, ErrNoActiveGeneration) {
				t.Fatalf("Active() after explicit recovery error = %v, want ErrNoActiveGeneration", err)
			}
		})
	}
}

func TestGenerationStoreExplicitRebuildRepairsCurrentVersionWithMissingRequiredTable(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	db, dbErr := openStoreDB(ctx, path, false)
	if dbErr != nil {
		t.Fatal(dbErr)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE chunks`); err != nil {
		closeNow(t, db)
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := openWriter(ctx, path); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("openWriter() error = %v, want ErrStoreCorrupt", err)
	}
	rebuilt, rebuildErr := openRebuildWriter(ctx, path)
	if rebuildErr != nil {
		t.Fatalf("openRebuildWriter(): %v", rebuildErr)
	}
	defer func() {
		if err := rebuilt.Close(); err != nil {
			t.Errorf("Close(): %v", err)
		}
	}()
	if _, err := rebuilt.Active(ctx); !errors.Is(err, ErrNoActiveGeneration) {
		t.Fatalf("Active() error = %v, want ErrNoActiveGeneration", err)
	}
}

func TestGenerationStoreRejectsCurrentVersionLookalikeWithoutConstraints(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := openStoreDB(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	weakSchema := `
CREATE TABLE generations (
  id INTEGER PRIMARY KEY, vector_format_version INTEGER, model TEXT,
  dimension INTEGER, protocol_epoch BLOB, chunker_epoch BLOB, vault_root TEXT,
  corpus_policy_fingerprint BLOB, policy_source_fingerprint BLOB,
  target_corpus_fingerprint BLOB, expected_chunks INTEGER, top_k_p95_us INTEGER
);
CREATE TABLE catalog (
  singleton INTEGER PRIMARY KEY, active_generation_id INTEGER,
  previous_generation_id INTEGER, staging_generation_id INTEGER
);
CREATE TABLE notes (generation_id INTEGER, rel_path TEXT, note_hash BLOB);
CREATE TABLE chunks (
  generation_id INTEGER, rel_path TEXT, ordinal INTEGER,
  submitted_hash BLOB, vector BLOB
);
CREATE TABLE attempts (
  generation_id INTEGER, rel_path TEXT, ordinal INTEGER, attempts INTEGER,
  retry_not_before_unix_ms INTEGER
);
INSERT INTO catalog(singleton) VALUES (1);
PRAGMA user_version = 1;
`
	if _, execErr := db.ExecContext(ctx, weakSchema); execErr != nil {
		closeNow(t, db)
		t.Fatal(execErr)
	}
	if closeErr := db.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	writer, err := openWriter(ctx, path)
	if writer != nil {
		closeNow(t, writer)
	}
	if !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("openWriter() error = %v, want ErrStoreCorrupt", err)
	}
	rebuilt, err := openRebuildWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, rebuilt)
}

func TestCanonicalSchemaSQLIgnoresOnlyFormatting(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		statement string
		want      string
	}{
		{
			name:      "outside whitespace and comment",
			statement: "CREATE  TABLE x (\n value TEXT -- explanation\n );",
			want:      "CREATE TABLE x ( value TEXT );",
		},
		{
			name:      "single quoted content",
			statement: "CREATE TABLE x (value TEXT CHECK (value = 'a  -- b')); -- explanation",
			want:      "CREATE TABLE x (value TEXT CHECK (value = 'a  -- b'));",
		},
		{
			name:      "escaped quote content",
			statement: "CREATE TABLE x (value TEXT CHECK (value = 'it''s  -- literal'));",
			want:      "CREATE TABLE x (value TEXT CHECK (value = 'it''s  -- literal'));",
		},
		{
			name:      "quoted identifier",
			statement: "CREATE TABLE \"a  -- b\" (value INTEGER);",
			want:      "CREATE TABLE \"a  -- b\" (value INTEGER);",
		},
		{
			name:      "block comment separates tokens",
			statement: "CREATE/* table documentation */TABLE x (value INTEGER);",
			want:      "CREATE TABLE x (value INTEGER);",
		},
		{
			name:      "comment markers in backtick identifier",
			statement: "CREATE TABLE `a  -- b /* c */` (value INTEGER); -- outside",
			want:      "CREATE TABLE `a  -- b /* c */` (value INTEGER);",
		},
		{
			name:      "comment markers in bracket identifier",
			statement: "CREATE TABLE [a  -- b /* c */] (value INTEGER); /* outside */",
			want:      "CREATE TABLE [a  -- b /* c */] (value INTEGER);",
		},
		{
			name:      "block marker in single quoted content",
			statement: "CREATE TABLE x (value TEXT CHECK (value = 'a /* b */ c')); /* outside */",
			want:      "CREATE TABLE x (value TEXT CHECK (value = 'a /* b */ c'));",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canonicalSchemaSQL(tt.statement); got != tt.want {
				t.Fatalf("canonicalSchemaSQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerationStoreBootstrapShellRecoveryAndForeignObjectRefusal(t *testing.T) {
	t.Parallel()

	t.Run("bootstrap shell", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		path := fixtureStorePath(t)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		db, err := openStoreDB(ctx, path, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		if _, err := openStore(ctx, path); !errors.Is(err, ErrStoreNotFound) {
			t.Fatalf("openStore() bootstrap shell error = %v, want ErrStoreNotFound", err)
		}
		writer, openErr := openWriter(ctx, path)
		if openErr != nil {
			t.Fatal(openErr)
		}
		closeOnCleanup(t, writer)
		if _, err := writer.Active(ctx); !errors.Is(err, ErrNoActiveGeneration) {
			t.Fatalf("Active() after shell bootstrap error = %v, want ErrNoActiveGeneration", err)
		}
	})

	t.Run("version zero foreign view", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		path := fixtureStorePath(t)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		db, err := openStoreDB(ctx, path, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `CREATE VIEW foreign_view AS SELECT 1 AS value`); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := openWriter(ctx, path); !errors.Is(err, ErrStoreSchemaMismatch) {
			t.Fatalf("openWriter() foreign view error = %v, want ErrStoreSchemaMismatch", err)
		}
	})
}

func TestGenerationStoreExplicitRebuildRemovesRollbackJournal(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	if _, err := writer.db.ExecContext(ctx, `PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	journalPath := path + "-journal"
	if err := os.WriteFile(journalPath, []byte("stale rollback journal"), 0o600); err != nil {
		t.Fatal(err)
	}

	rebuilt, err := openRebuildWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, rebuilt)
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback journal stat error = %v, want not exist", err)
	}
}

func TestGenerationStoreRebuildPreservesReplacementStore(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	initial, err := openWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, execErr := initial.db.ExecContext(ctx, `PRAGMA user_version = 999`); execErr != nil {
		t.Fatal(execErr)
	}
	if closeErr := initial.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	dir := filepath.Dir(path)
	sentinel := []byte("replacement store must survive\n")
	rebuilt, err := openGenerationWriterWithHooks(ctx, path, true, writerOpenHooks{
		beforeReset: func() {
			if renameErr := os.Rename(dir, dir+"-detached"); renameErr != nil {
				t.Fatal(renameErr)
			}
			if mkdirErr := os.Mkdir(dir, 0o700); mkdirErr != nil {
				t.Fatal(mkdirErr)
			}
			if writeErr := os.WriteFile(path, sentinel, 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
		},
	})
	if rebuilt != nil {
		if closeErr := rebuilt.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("openGenerationWriterWithHooks(rebuild) error = %v, want ErrStorePermissions", err)
	}
	got, readErr := readRootedTestFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if diff := cmp.Diff(sentinel, got); diff != "" {
		t.Errorf("replacement store changed (-want +got):\n%s", diff)
	}
}

func TestGenerationStoreActivationRejectsDirectoryReplacement(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("activation-directory-replacement")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, err := openWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	putBuildRows(t, build, []ChunkVector{row})
	dir := filepath.Dir(path)
	sentinel := []byte("replacement store must survive activation\n")
	_, err = build.activateGeneration(ctx, func([]ChunkVector) (time.Duration, error) {
		if renameErr := os.Rename(dir, dir+"-detached"); renameErr != nil {
			t.Fatal(renameErr)
		}
		if mkdirErr := os.Mkdir(dir, 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if writeErr := os.WriteFile(path, sentinel, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		return 0, nil
	})
	if !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("activateGeneration() error = %v, want ErrStorePermissions", err)
	}
	got, readErr := readRootedTestFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if diff := cmp.Diff(sentinel, got); diff != "" {
		t.Errorf("replacement store changed (-want +got):\n%s", diff)
	}
}

func TestGenerationStoreReaderRejectsDirectoryReplacement(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("reader-directory-replacement")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, err := openWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	putBuildRows(t, build, []ChunkVector{row})
	if activateErr := build.activate(ctx); activateErr != nil {
		t.Fatal(activateErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	reader, err := openStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, reader)
	dir := filepath.Dir(path)
	if err := os.Rename(dir, dir+"-detached"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Active(ctx); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("Active() error = %v, want ErrStorePermissions", err)
	}
}

func TestGenerationStoreWriterReadRejectsDirectoryReplacement(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("writer-read-directory-replacement")
	row := fixtureChunkVector("Writing/target.md", 0, identity.dimension, 1)
	writer, err := openWriter(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	build := prepareBuild(t, writer, &identity, []ChunkVector{row})
	putBuildRows(t, build, []ChunkVector{row})
	if err := build.activate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	if err := os.Rename(dir, dir+"-detached"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Active(ctx); !errors.Is(err, ErrStorePermissions) {
		t.Fatalf("Active() error = %v, want ErrStorePermissions", err)
	}
}

func TestGenerationStoreResetOwnsEverySQLiteFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join("semantic", "vectors.sqlite")
	want := []string{path, path + "-wal", path + "-shm", path + "-journal"}
	if diff := cmp.Diff(want, storeFilePaths(path)); diff != "" {
		t.Fatalf("store file set mismatch (-want +got):\n%s", diff)
	}
}

func TestGenerationStoreSeparatesSchemaAndVectorFormatVersions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)

	var schemaVersion int
	if err := writer.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&schemaVersion); err != nil {
		t.Fatal(err)
	}
	if schemaVersion != storeSchemaVersion {
		t.Fatalf("PRAGMA user_version = %d, want %d", schemaVersion, storeSchemaVersion)
	}
	catalogColumns := tableColumns(t, writer, "catalog")
	if _, ok := catalogColumns["schema_version"]; ok {
		t.Fatalf("catalog columns = %v, schema version must be store metadata, not role data", catalogColumns)
	}
	if _, ok := catalogColumns["format_version"]; ok {
		t.Fatalf("catalog columns = %v, format_version conflates schema and vector format", catalogColumns)
	}
	generationColumns := tableColumns(t, writer, "generations")
	if _, ok := generationColumns["vector_format_version"]; !ok {
		t.Fatalf("generation columns = %v, want vector_format_version", generationColumns)
	}
	if _, ok := generationColumns["format_version"]; ok {
		t.Fatalf("generation columns = %v, ambiguous format_version remains", generationColumns)
	}
}

func TestGenerationStoreResumesOnlyExactStaging(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	identity := fixtureGenerationIdentity("resume")
	baseRows := []ChunkVector{fixtureChunkVector("Writing/base.md", 0, identity.dimension, 1)}

	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	base := prepareBuild(t, writer, &identity, baseRows)
	putBuildRows(t, base, baseRows)
	if err := base.activate(ctx); err != nil {
		t.Fatal(err)
	}

	targetRows := append(
		[]ChunkVector{},
		baseRows...,
	)
	targetRows = append(targetRows, fixtureChunkVector("Writing/paid.md", 0, identity.dimension, 2))
	partial := prepareBuild(t, writer, &identity, targetRows)
	if _, err := partial.reserveAttempt(ctx, targetRows[1].RelPath, targetRows[1].Ordinal, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := partial.put(ctx, &targetRows[1]); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	writer, openErr = openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	exact := prepareBuild(t, writer, &identity, targetRows)
	if !exact.resumedFromStore() {
		t.Fatal("exact staging build resumed = false, want true")
	}
	gotRows, rowsErr := exact.rows(ctx)
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	if diff := cmp.Diff(targetRows, gotRows, compareManifestRows); diff != "" {
		t.Errorf("resumed rows mismatch (-want +got):\n%s", diff)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	writer, openErr = openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	changedSource := sha256.Sum256([]byte("changed contract source bytes"))
	sourceReplaced := prepareBuildWithSource(t, writer, &identity, changedSource, targetRows)
	if sourceReplaced.resumedFromStore() {
		t.Fatal("different policy source resumed = true, want false")
	}
	gotRows, rowsErr = sourceReplaced.rows(ctx)
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	if diff := cmp.Diff(baseRows, gotRows, compareManifestRows); diff != "" {
		t.Errorf("source-replaced staging mismatch (-want active clone +got):\n%s", diff)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	otherRows := []ChunkVector{
		baseRows[0],
		fixtureChunkVector("Writing/other.md", 0, identity.dimension, 3),
	}
	writer, openErr = openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	replaced := prepareBuild(t, writer, &identity, otherRows)
	if replaced.resumedFromStore() {
		t.Fatal("different target resumed = true, want false")
	}
	gotRows, rowsErr = replaced.rows(ctx)
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	if diff := cmp.Diff(baseRows, gotRows, compareManifestRows); diff != "" {
		t.Errorf("fresh staging clone mismatch (-want base +got):\n%s", diff)
	}
	assertActiveGeneration(t, writer, &identity, baseRows, 0)
}

func TestGenerationStoreActivationRejectsIncompleteOrWrongCorpus(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	path := fixtureStorePath(t)
	writer, openErr := openWriter(ctx, path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	closeOnCleanup(t, writer)
	identity := fixtureGenerationIdentity("incomplete")
	activeRows := []ChunkVector{fixtureChunkVector("Writing/active.md", 0, identity.dimension, 1)}
	active := prepareBuild(t, writer, &identity, activeRows)
	putBuildRows(t, active, activeRows)
	if err := active.activate(ctx); err != nil {
		t.Fatal(err)
	}

	targetRows := []ChunkVector{
		fixtureChunkVector("Writing/next.md", 0, identity.dimension, 2),
		fixtureChunkVector("Writing/second.md", 0, identity.dimension, 3),
	}
	incomplete := prepareBuild(t, writer, &identity, targetRows)
	if _, err := incomplete.reserveAttempt(ctx, targetRows[0].RelPath, targetRows[0].Ordinal, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := incomplete.put(ctx, &targetRows[0]); err != nil {
		t.Fatal(err)
	}
	if err := incomplete.activate(ctx); !errors.Is(err, ErrGenerationIncomplete) {
		t.Fatalf("incomplete Activate() error = %v, want ErrGenerationIncomplete", err)
	}
	assertActiveGeneration(t, writer, &identity, activeRows, 0)

	putBuildRows(t, incomplete, targetRows[1:])
	tamperedHash := sha256.Sum256([]byte("tampered manifest identity"))
	if _, err := writer.db.ExecContext(ctx,
		`UPDATE chunks SET submitted_hash = ? WHERE generation_id = ? AND rel_path = ? AND ordinal = ?`,
		tamperedHash[:], incomplete.id, targetRows[1].RelPath, targetRows[1].Ordinal,
	); err != nil {
		t.Fatal(err)
	}
	if err := incomplete.activate(ctx); !errors.Is(err, ErrStoreCorrupt) {
		t.Fatalf("tampered-corpus Activate() error = %v, want ErrStoreCorrupt", err)
	}
	assertActiveGeneration(t, writer, &identity, activeRows, 0)
}

func prepareBuild(t *testing.T, writer *writer, identity *generationIdentity, rows []ChunkVector) *staging {
	t.Helper()
	return prepareBuildWithSource(t, writer, identity, fixturePolicySource(identity), rows)
}

func prepareBuildWithSource(
	t *testing.T,
	writer *writer,
	identity *generationIdentity,
	policySource [sha256.Size]byte,
	rows []ChunkVector,
) *staging {
	t.Helper()
	targets := make([]ChunkTarget, len(rows))
	for i := range rows {
		row := &rows[i]
		target := ChunkTarget{
			RelPath:       row.RelPath,
			NoteHash:      row.NoteHash,
			Ordinal:       row.Ordinal,
			SubmittedHash: row.SubmittedHash,
		}
		targets[i] = bindChunkTarget(&target)
	}
	build, err := writer.prepare(t.Context(), identity, policySource, targets)
	if err != nil {
		t.Fatal(err)
	}
	return build
}

func fixturePolicySource(identity *generationIdentity) [sha256.Size]byte {
	return sha256.Sum256([]byte("policy source:" + identity.vaultRoot))
}

func putBuildRows(t *testing.T, build *staging, rows []ChunkVector) {
	t.Helper()
	type chunkKey struct {
		relPath string
		ordinal uint32
	}
	byKey := make(map[chunkKey]ChunkVector, len(rows))
	for i := range rows {
		row := &rows[i]
		byKey[chunkKey{relPath: row.RelPath, ordinal: row.Ordinal}] = *row
	}
	pending, err := build.pending(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range pending {
		row, ok := byKey[chunkKey{relPath: target.RelPath, ordinal: target.Ordinal}]
		if !ok {
			t.Fatalf("pending target %q/%d has no supplied vector", target.RelPath, target.Ordinal)
		}
		if _, err := build.reserveAttempt(t.Context(), row.RelPath, row.Ordinal, time.Unix(0, 0)); err != nil {
			t.Fatalf("ReserveAttempt(%q, %d) error: %v", row.RelPath, row.Ordinal, err)
		}
		if err := build.put(t.Context(), &row); err != nil {
			t.Fatalf("Put(%q, %d) error: %v", row.RelPath, row.Ordinal, err)
		}
	}
}

func assertActiveGeneration(
	t *testing.T,
	reader interface {
		Active(ctx context.Context) (loadedGeneration, error)
	},
	identity *generationIdentity,
	rows []ChunkVector,
	topKP95 time.Duration,
) {
	t.Helper()
	got, err := reader.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	wantFingerprint, err := CorpusFingerprint(rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.identity != *identity || got.corpusFingerprint != wantFingerprint || got.topKP95 != topKP95 {
		t.Errorf("Active() metadata = %+v, want identity %+v fingerprint %x p95 %s", got, *identity, wantFingerprint, topKP95)
	}
	if diff := cmp.Diff(rows, got.rows, compareManifestRows); diff != "" {
		t.Errorf("Active() rows mismatch (-want +got):\n%s", diff)
	}
}

func fixtureStorePath(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "semantic")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "vectors.sqlite")
}

func readRootedTestFile(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	data, readErr := root.ReadFile(filepath.Base(path))
	return data, errors.Join(readErr, root.Close())
}

func fixtureGenerationIdentity(seed string) generationIdentity {
	return generationIdentity{
		model:                   "fixture-" + seed,
		dimension:               4,
		protocolEpoch:           sha256.Sum256([]byte("protocol-" + seed)),
		chunkerEpoch:            sha256.Sum256([]byte("chunker-" + seed)),
		vectorFormatVersion:     vectorFormatVersion,
		vaultRoot:               filepath.Join(string(filepath.Separator), "vault", seed),
		corpusPolicyFingerprint: sha256.Sum256([]byte("policy-" + seed)),
	}
}

func fixtureChunkVector(relPath string, ordinal uint32, dimension, seed int) ChunkVector {
	vector := make([]float32, dimension)
	vector[seed%dimension] = 1
	row := ChunkVector{
		RelPath:       relPath,
		NoteHash:      sha256.Sum256([]byte("note:" + relPath)),
		Ordinal:       ordinal,
		SubmittedHash: sha256.Sum256([]byte("submitted:" + relPath + strconv.Itoa(seed))),
		Vector:        vector,
	}
	return bindChunkVector(&row)
}

func fixtureChunkTarget(relPath string, seed int) ChunkTarget {
	target := ChunkTarget{
		RelPath:       relPath,
		NoteHash:      sha256.Sum256([]byte("note:" + relPath)),
		Ordinal:       0,
		SubmittedHash: sha256.Sum256([]byte("submitted:" + relPath + strconv.Itoa(seed))),
	}
	return bindChunkTarget(&target)
}

func tableColumns(t *testing.T, writer *writer, table string) map[string]struct{} {
	t.Helper()
	rows, err := writer.db.QueryContext(t.Context(), `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("close schema rows: %v", closeErr)
		}
	}()
	columns := make(map[string]struct{})
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return columns
}

func corruptStoredVectorLength(t *testing.T, writer *writer, generationID int64) {
	t.Helper()
	if _, err := writer.db.ExecContext(t.Context(), `PRAGMA ignore_check_constraints = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(t.Context(),
		`UPDATE chunks SET vector = x'00' WHERE generation_id = ?`, generationID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.db.ExecContext(t.Context(), `PRAGMA ignore_check_constraints = OFF`); err != nil {
		t.Fatal(err)
	}
}

func assertPrivateGenerationFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode = %o, want no group/other permissions", entry.Name(), info.Mode().Perm())
		}
	}
}
