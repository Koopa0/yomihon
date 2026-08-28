package semantic

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

const generationBenchmarkEnabled = "YOMIHON_GENERATION_BENCHMARK"

type generationBenchmarkFixture struct {
	identity                generationIdentity
	policySourceFingerprint [sha256.Size]byte
	revision                int
	documents               []CorpusChunk
	targets                 []ChunkTarget
	rows                    map[generationBenchmarkKey]ChunkVector
}

type generationBenchmarkKey struct {
	relPath string
	ordinal uint32
}

type generationStoreFootprint struct {
	database int64
	wal      int64
	shm      int64
	journal  int64
}

type generationFootprintObservation struct {
	footprint generationStoreFootprint
	err       error
}

type generationFootprintObserver struct {
	stopCh   chan struct{}
	resultCh chan generationFootprintObservation
}

func (f generationStoreFootprint) total() int64 {
	return f.database + f.wal + f.shm + f.journal
}

func (f generationStoreFootprint) max(other generationStoreFootprint) generationStoreFootprint {
	if other.total() > f.total() {
		return other
	}
	return f
}

var generationBenchmarkSink loadedGeneration

func TestGenerationBenchmarkFixtureDriftsOneSubmittedDocumentPerRevision(t *testing.T) {
	t.Parallel()

	fixture := newGenerationBenchmarkFixture(t, 64, 8, 128, t.TempDir())
	first := driftOneGenerationBenchmarkDocument(t, &fixture)
	second := driftOneGenerationBenchmarkDocument(t, &first)
	assertGenerationBenchmarkDrift(t, &fixture, &first)
	assertGenerationBenchmarkDrift(t, &first, &second)
}

// BenchmarkGenerationStore measures the shipped active/previous/staging
// lifecycle rather than an alternate storage prototype. It is opt-in and
// one-shot because each row completion performs durable SQLite commits.
func BenchmarkGenerationStore(b *testing.B) {
	if os.Getenv(generationBenchmarkEnabled) != "1" {
		b.Skip("set " + generationBenchmarkEnabled + "=1")
	}
	count := generationBenchmarkPositiveInt(b, "YOMIHON_GENERATION_CHUNKS", 128)
	dimension := generationBenchmarkPositiveInt(b, "YOMIHON_GENERATION_DIMENSION", 128)
	notes := generationBenchmarkPositiveInt(b, "YOMIHON_GENERATION_NOTES", min(count, 32))
	if notes > count {
		b.Fatalf("YOMIHON_GENERATION_NOTES=%d exceeds chunks=%d", notes, count)
	}

	b.Run("InitialBuild", func(b *testing.B) {
		requireOneShotBenchmark(b)
		for b.Loop() {
			path := generationBenchmarkStorePath(b)
			fixture := newGenerationBenchmarkFixture(b, count, notes, dimension, filepath.Dir(path))
			writer, err := openRebuildWriter(b.Context(), path)
			if err != nil {
				b.Fatal(err)
			}
			observer := startGenerationFootprintObserver(path)
			buildGenerationBenchmark(b, writer, &fixture)
			reportGenerationFootprint(b, "open", generationStoreFootprintAt(b, path))
			closeErr := writer.Close()
			peak := observer.finish(b)
			if closeErr != nil {
				b.Fatal(closeErr)
			}
			reportGenerationFootprint(b, "observed_peak", peak)
			reportGenerationFootprint(b, "closed", generationStoreFootprintAt(b, path))
		}
	})

	b.Run("CompatibleOneNoteDrift", func(b *testing.B) {
		requireOneShotBenchmark(b)
		path := generationBenchmarkStorePath(b)
		fixture := newGenerationBenchmarkFixture(b, count, notes, dimension, filepath.Dir(path))
		writer, err := openRebuildWriter(b.Context(), path)
		if err != nil {
			b.Fatal(err)
		}
		closeOnCleanup(b, writer)
		buildGenerationBenchmark(b, writer, &fixture)
		drifted := driftOneGenerationBenchmarkDocument(b, &fixture)
		b.ResetTimer()
		for b.Loop() {
			build := prepareGenerationBenchmark(b, writer, &drifted)
			pending := pendingGenerationBenchmarkRows(b, build)
			if len(pending) != 1 {
				b.Fatalf("Pending() rows = %d, want 1 changed chunk", len(pending))
			}
			completeGenerationBenchmark(b, build, &drifted, pending)
			if err := build.activate(b.Context()); err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(float64(count-len(pending)), "reused_chunks/op")
			b.ReportMetric(float64(len(pending)), "embedded_chunks/op")
		}
	})

	b.Run("ColdLoad", func(b *testing.B) {
		requireOneShotBenchmark(b)
		path := generationBenchmarkStorePath(b)
		fixture := newGenerationBenchmarkFixture(b, count, notes, dimension, filepath.Dir(path))
		writer, err := openRebuildWriter(b.Context(), path)
		if err != nil {
			b.Fatal(err)
		}
		buildGenerationBenchmark(b, writer, &fixture)
		if err := writer.Close(); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for b.Loop() {
			store, openErr := openStore(b.Context(), path)
			if openErr != nil {
				b.Fatal(openErr)
			}
			active, activeErr := store.Active(b.Context())
			if activeErr != nil {
				closeNow(b, store)
				b.Fatal(activeErr)
			}
			if closeErr := store.Close(); closeErr != nil {
				b.Fatal(closeErr)
			}
			generationBenchmarkSink = active
		}
	})

	b.Run("ActivePreviousStagingFootprint", func(b *testing.B) {
		requireOneShotBenchmark(b)
		for b.Loop() {
			path := generationBenchmarkStorePath(b)
			fixture := newGenerationBenchmarkFixture(b, count, notes, dimension, filepath.Dir(path))
			writer, err := openRebuildWriter(b.Context(), path)
			if err != nil {
				b.Fatal(err)
			}
			observer := startGenerationFootprintObserver(path)
			buildGenerationBenchmark(b, writer, &fixture)

			second := driftOneGenerationBenchmarkDocument(b, &fixture)
			buildGenerationBenchmark(b, writer, &second)

			third := driftOneGenerationBenchmarkDocument(b, &second)
			staging := prepareGenerationBenchmark(b, writer, &third)
			pending := pendingGenerationBenchmarkRows(b, staging)
			if len(pending) != 1 {
				b.Fatalf("third-generation Pending() rows = %d, want 1", len(pending))
			}
			reportGenerationFootprint(b, "three_roles_open", generationStoreFootprintAt(b, path))
			closeErr := writer.Close()
			peak := observer.finish(b)
			if closeErr != nil {
				b.Fatal(closeErr)
			}
			reportGenerationFootprint(b, "observed_peak", peak)
			reportGenerationFootprint(b, "three_roles_closed", generationStoreFootprintAt(b, path))
		}
	})
}

func requireOneShotBenchmark(b *testing.B) {
	b.Helper()
	if b.N != 1 {
		b.Fatalf("benchmark iterations = %d, want 1; run with -benchtime=1x", b.N)
	}
}

func newGenerationBenchmarkFixture(
	tb testing.TB,
	count, noteCount, dimension int,
	vaultRoot string,
) generationBenchmarkFixture {
	tb.Helper()
	identity, err := newGenerationIdentity(IdentityConfig{
		Dimension:               dimension,
		ChunkTokenCap:           512,
		VaultRoot:               vaultRoot,
		CorpusPolicyFingerprint: sha256.Sum256([]byte("generation benchmark corpus policy")),
	})
	if err != nil {
		tb.Fatal(err)
	}
	fixture := generationBenchmarkFixture{
		identity:                identity,
		policySourceFingerprint: sha256.Sum256([]byte("generation benchmark policy source")),
		documents:               make([]CorpusChunk, count),
		targets:                 make([]ChunkTarget, count),
		rows:                    make(map[generationBenchmarkKey]ChunkVector, count),
	}
	for i := range count {
		noteIndex := i % noteCount
		ordinal := uint32(i / noteCount) // #nosec G115 -- benchmark sizes are bounded positive ints
		relPath := fmt.Sprintf("Writing/benchmark/note-%06d.md", noteIndex)
		submitted := fmt.Appendf(nil, "document %06d ordinal %06d", noteIndex, ordinal)
		document := CorpusChunk{
			RelPath:       relPath,
			NoteHash:      sha256.Sum256([]byte("note:" + relPath)),
			Ordinal:       ordinal,
			Submitted:     submitted,
			SubmittedHash: sha256.Sum256(submitted),
		}
		target, targetErr := document.Target()
		if targetErr != nil {
			tb.Fatal(targetErr)
		}
		row, rowErr := document.Complete(EmbeddingResult{Vector: generationBenchmarkVector(i, dimension)})
		if rowErr != nil {
			tb.Fatal(rowErr)
		}
		fixture.documents[i] = document
		fixture.targets[i] = target
		fixture.rows[generationBenchmarkKey{relPath: relPath, ordinal: ordinal}] = row
	}
	return fixture
}

func driftOneGenerationBenchmarkDocument(
	tb testing.TB,
	fixture *generationBenchmarkFixture,
) generationBenchmarkFixture {
	tb.Helper()
	drifted := generationBenchmarkFixture{
		identity:                fixture.identity,
		policySourceFingerprint: fixture.policySourceFingerprint,
		revision:                fixture.revision + 1,
		documents:               slices.Clone(fixture.documents),
		targets:                 make([]ChunkTarget, len(fixture.documents)),
		rows:                    make(map[generationBenchmarkKey]ChunkVector, len(fixture.documents)),
	}
	changedPath := drifted.documents[len(drifted.documents)/2].RelPath
	changedNoteHash := sha256.Sum256([]byte("changed note:" + changedPath + ":" + strconv.Itoa(drifted.revision)))
	changedChunk := false
	for i := range drifted.documents {
		document := &drifted.documents[i]
		if document.RelPath == changedPath {
			document.NoteHash = changedNoteHash
			if !changedChunk {
				document.Submitted = []byte("changed provider document:" + document.RelPath + ":" + strconv.Itoa(drifted.revision))
				document.SubmittedHash = sha256.Sum256(document.Submitted)
				changedChunk = true
			}
		}
		target, targetErr := document.Target()
		if targetErr != nil {
			tb.Fatal(targetErr)
		}
		row, rowErr := document.Complete(EmbeddingResult{
			Vector: generationBenchmarkVector(i+len(drifted.documents), fixture.identity.Dimension()),
		})
		if rowErr != nil {
			tb.Fatal(rowErr)
		}
		drifted.targets[i] = target
		drifted.rows[generationBenchmarkKey{relPath: row.RelPath, ordinal: row.Ordinal}] = row
	}
	if !changedChunk {
		tb.Fatal("benchmark drift changed no chunk")
	}
	return drifted
}

func assertGenerationBenchmarkDrift(
	t *testing.T,
	before, after *generationBenchmarkFixture,
) {
	t.Helper()
	if len(before.documents) != len(after.documents) {
		t.Fatalf("document count changed from %d to %d", len(before.documents), len(after.documents))
	}
	changedSubmitted := 0
	changedNotePaths := make(map[string]struct{})
	for i := range before.documents {
		if before.documents[i].SubmittedHash != after.documents[i].SubmittedHash {
			changedSubmitted++
		}
		if before.documents[i].NoteHash != after.documents[i].NoteHash {
			changedNotePaths[after.documents[i].RelPath] = struct{}{}
		}
	}
	if changedSubmitted != 1 {
		t.Fatalf("submitted-document changes = %d, want 1", changedSubmitted)
	}
	if len(changedNotePaths) != 1 {
		t.Fatalf("changed note paths = %d, want 1", len(changedNotePaths))
	}
}

func generationBenchmarkVector(seed, dimension int) []float32 {
	vector := make([]float32, dimension)
	for i := range vector {
		value := (seed*131 + i*17 + 1) % 1009
		vector[i] = float32(value+1) / 1010
	}
	return vector
}

func buildGenerationBenchmark(
	b *testing.B,
	writer *writer,
	fixture *generationBenchmarkFixture,
) {
	b.Helper()
	build := prepareGenerationBenchmark(b, writer, fixture)
	pending := pendingGenerationBenchmarkRows(b, build)
	completeGenerationBenchmark(b, build, fixture, pending)
	if err := build.activate(b.Context()); err != nil {
		b.Fatal(err)
	}
}

func prepareGenerationBenchmark(
	b *testing.B,
	writer *writer,
	fixture *generationBenchmarkFixture,
) *staging {
	b.Helper()
	build, err := writer.prepare(b.Context(), &fixture.identity, fixture.policySourceFingerprint, fixture.targets)
	if err != nil {
		b.Fatal(err)
	}
	return build
}

func pendingGenerationBenchmarkRows(b *testing.B, build *staging) []ChunkTarget {
	b.Helper()
	pending, err := build.pending(b.Context())
	if err != nil {
		b.Fatal(err)
	}
	return pending
}

func completeGenerationBenchmark(
	b *testing.B,
	build *staging,
	fixture *generationBenchmarkFixture,
	pending []ChunkTarget,
) {
	b.Helper()
	for i := range pending {
		target := &pending[i]
		row, ok := fixture.rows[generationBenchmarkKey{relPath: target.RelPath, ordinal: target.Ordinal}]
		if !ok {
			b.Fatalf("pending target %q/%d has no fixture vector", target.RelPath, target.Ordinal)
		}
		if _, err := build.reserveAttempt(b.Context(), row.RelPath, row.Ordinal, time.Unix(0, 0)); err != nil {
			b.Fatal(err)
		}
		if err := build.put(b.Context(), &row); err != nil {
			b.Fatal(err)
		}
	}
}

func generationBenchmarkStorePath(b *testing.B) string {
	b.Helper()
	dir := filepath.Join(b.TempDir(), "semantic")
	if err := os.Mkdir(dir, 0o700); err != nil {
		b.Fatal(err)
	}
	return filepath.Join(dir, "generation.sqlite")
}

func generationBenchmarkPositiveInt(tb testing.TB, name string, fallback int) int {
	tb.Helper()
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		tb.Fatalf("%s=%q is not a positive integer", name, raw)
	}
	return value
}

func generationStoreFootprintAt(tb testing.TB, path string) generationStoreFootprint {
	tb.Helper()
	footprint, err := readGenerationStoreFootprint(path)
	if err != nil {
		tb.Fatal(err)
	}
	return footprint
}

func readGenerationStoreFootprint(path string) (generationStoreFootprint, error) {
	var footprint generationStoreFootprint
	destinations := []*int64{&footprint.database, &footprint.wal, &footprint.shm, &footprint.journal}
	for i, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		size, err := generationStoreFileSize(path + suffix)
		if err != nil {
			return generationStoreFootprint{}, err
		}
		*destinations[i] = size
	}
	return footprint, nil
}

func generationStoreFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.Size(), nil
	}
	if os.IsNotExist(err) {
		return 0, nil
	}
	return 0, fmt.Errorf("stat %q: %w", path, err)
}

func startGenerationFootprintObserver(path string) *generationFootprintObserver {
	observer := &generationFootprintObserver{
		stopCh:   make(chan struct{}),
		resultCh: make(chan generationFootprintObservation, 1),
	}
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		peak, err := readGenerationStoreFootprint(path)
		for err == nil {
			select {
			case <-ticker.C:
				var current generationStoreFootprint
				current, err = readGenerationStoreFootprint(path)
				peak = peak.max(current)
			case <-observer.stopCh:
				var current generationStoreFootprint
				current, err = readGenerationStoreFootprint(path)
				observer.resultCh <- generationFootprintObservation{footprint: peak.max(current), err: err}
				return
			}
		}
		observer.resultCh <- generationFootprintObservation{err: err}
	}()
	return observer
}

func (o *generationFootprintObserver) finish(tb testing.TB) generationStoreFootprint {
	tb.Helper()
	close(o.stopCh)
	result := <-o.resultCh
	if result.err != nil {
		tb.Fatal(result.err)
	}
	return result.footprint
}

func reportGenerationFootprint(b *testing.B, prefix string, footprint generationStoreFootprint) {
	b.Helper()
	b.ReportMetric(float64(footprint.database), prefix+"_db_bytes/op")
	b.ReportMetric(float64(footprint.wal), prefix+"_wal_bytes/op")
	b.ReportMetric(float64(footprint.shm), prefix+"_shm_bytes/op")
	b.ReportMetric(float64(footprint.journal), prefix+"_journal_bytes/op")
	b.ReportMetric(float64(footprint.total()), prefix+"_total_bytes/op")
}
