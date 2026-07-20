//go:build modernc || mattn

package sqlitedriverbakeoff

import (
	"cmp"
	"crypto/sha256"
	"database/sql" //nolint:depguard // Driver parity requires one workload to exercise both database/sql drivers.
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

type driverChunk struct {
	relPath       string
	noteHash      [sha256.Size]byte
	ordinal       int64
	submittedHash [sha256.Size]byte
	vector        []byte
}

type driverGeneration struct {
	id int64
}

var driverBenchmarkRows []catalog.GenerationChunkPageRow

func TestDriverConnectionPolicies(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "generation.sqlite")
	db := createDriverDatabase(t, path, readCanonicalSchema(t))
	var busyTimeout, foreignKeys, synchronous int
	var journalMode string
	if err := db.QueryRowContext(t.Context(), `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(t.Context(), `PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(t.Context(), `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 || foreignKeys != 1 || synchronous != 2 || journalMode != "wal" {
		t.Fatalf(
			"writer pragmas = (busy_timeout=%d, foreign_keys=%d, synchronous=%d, journal_mode=%q), want (5000, 1, 2, %q)",
			busyTimeout,
			foreignKeys,
			synchronous,
			journalMode,
			"wal",
		)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reader := openDriverDatabase(t, path, true)
	t.Cleanup(func() { closeDriverDatabase(t, reader) })
	var queryOnly int
	if err := reader.QueryRowContext(t.Context(), `PRAGMA query_only`).Scan(&queryOnly); err != nil {
		t.Fatal(err)
	}
	if queryOnly != 1 {
		t.Fatalf("reader query_only = %d, want 1", queryOnly)
	}
	if _, err := reader.ExecContext(t.Context(), `CREATE TABLE forbidden_write (value INTEGER)`); err == nil {
		t.Fatal("reader CREATE TABLE error = nil, want read-only rejection")
	}
}

// BenchmarkDriverGeneration runs one shared sqlc workload through either
// driver. Build tags select only the database/sql driver; schema and queries
// remain the same bytes in both runs.
func BenchmarkDriverGeneration(b *testing.B) {
	count := driverPositiveInt(b, "YOMIHON_DRIVER_CHUNKS", 256)
	dimension := driverPositiveInt(b, "YOMIHON_DRIVER_DIMENSION", 128)
	notes := driverPositiveInt(b, "YOMIHON_DRIVER_NOTES", min(count, 32))
	if notes > count {
		b.Fatalf("YOMIHON_DRIVER_NOTES=%d exceeds chunks=%d", notes, count)
	}
	schema := readCanonicalSchema(b)
	rows := driverRows(count, notes, dimension, 0)

	b.Run("InitialBuild", func(b *testing.B) {
		requireOneDriverIteration(b)
		for b.Loop() {
			path := filepath.Join(b.TempDir(), "generation.sqlite")
			db := createDriverDatabase(b, path, schema)
			generation := writeDriverGeneration(b, db, rows, 0)
			activateDriverGeneration(b, db, generation.id)
			reportDriverFootprint(b, "open", path)
			if err := db.Close(); err != nil {
				b.Fatal(err)
			}
			reportDriverFootprint(b, "closed", path)
		}
	})

	b.Run("CompatibleOneNoteDrift", func(b *testing.B) {
		requireOneDriverIteration(b)
		path := filepath.Join(b.TempDir(), "generation.sqlite")
		db := createDriverDatabase(b, path, schema)
		b.Cleanup(func() { closeDriverDatabase(b, db) })
		first := writeDriverGeneration(b, db, rows, 0)
		activateDriverGeneration(b, db, first.id)
		drifted := driverRows(count, notes, dimension, 1)
		b.ResetTimer()
		for b.Loop() {
			generation := prepareDriverGeneration(b, db, drifted, first.id)
			pending := pendingDriverRows(b, db, generation.id)
			if len(pending) != 1 {
				b.Fatalf("pending rows = %d, want 1", len(pending))
			}
			completeDriverRows(b, db, generation.id, drifted, pending)
			activateDriverGeneration(b, db, generation.id)
			b.ReportMetric(float64(count-1), "reused_chunks/op")
			b.ReportMetric(1, "embedded_chunks/op")
		}
	})

	b.Run("ColdLoad", func(b *testing.B) {
		requireOneDriverIteration(b)
		path := filepath.Join(b.TempDir(), "generation.sqlite")
		db := createDriverDatabase(b, path, schema)
		generation := writeDriverGeneration(b, db, rows, 0)
		activateDriverGeneration(b, db, generation.id)
		if err := db.Close(); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for b.Loop() {
			opened := openDriverDatabase(b, path, true)
			tx, err := opened.BeginTx(b.Context(), &sql.TxOptions{ReadOnly: true})
			if err != nil {
				b.Fatal(err)
			}
			q := catalog.New(tx)
			roleCatalog, err := q.Catalog(b.Context())
			if err != nil {
				b.Fatal(err)
			}
			if _, generationErr := q.GenerationByID(b.Context(), roleCatalog.ActiveGenerationID.Int64); generationErr != nil {
				b.Fatal(generationErr)
			}
			driverBenchmarkRows = loadDriverGenerationChunks(b, q, roleCatalog.ActiveGenerationID.Int64)
			if len(driverBenchmarkRows) != count {
				b.Fatalf("loaded chunks = %d, want %d", len(driverBenchmarkRows), count)
			}
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}
			if err := opened.Close(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func loadDriverGenerationChunks(
	tb testing.TB,
	q *catalog.Queries,
	generationID int64,
) []catalog.GenerationChunkPageRow {
	tb.Helper()
	const pageSize = 256

	var rows []catalog.GenerationChunkPageRow
	afterPath := ""
	afterOrdinal := int64(-1)
	for {
		page, err := q.GenerationChunkPage(tb.Context(), catalog.GenerationChunkPageParams{
			GenerationID: generationID,
			AfterRelPath: afterPath,
			AfterOrdinal: afterOrdinal,
			PageSize:     pageSize,
		})
		if err != nil {
			tb.Fatal(err)
		}
		rows = append(rows, page...)
		if len(page) < pageSize {
			return rows
		}
		last := &page[len(page)-1]
		afterPath = last.RelPath
		afterOrdinal = last.Ordinal
	}
}

func readCanonicalSchema(tb testing.TB) string {
	tb.Helper()
	path := filepath.Join("..", "..", "internal", "search", "semantic", "sql", "schema.sql")
	// #nosec G304 -- this fixed repository-relative path selects the schema exercised by both drivers.
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read canonical schema %q: %v", path, err)
	}
	return string(data)
}

func createDriverDatabase(tb testing.TB, path, schema string) *sql.DB {
	tb.Helper()
	db := openDriverDatabase(tb, path, false)
	if _, err := db.ExecContext(tb.Context(), schema); err != nil {
		closeDriverDatabase(tb, db)
		tb.Fatalf("install schema: %v", err)
	}
	if err := catalog.New(db).InitializeCatalog(tb.Context()); err != nil {
		closeDriverDatabase(tb, db)
		tb.Fatalf("initialize catalog: %v", err)
	}
	return db
}

func openDriverDatabase(tb testing.TB, path string, readOnly bool) *sql.DB {
	tb.Helper()
	db, err := sql.Open(benchmarkDriverName, benchmarkDriverDSN(path, readOnly))
	if err != nil {
		tb.Fatalf("open %s: %v", benchmarkDriverName, err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(tb.Context()); err != nil {
		closeDriverDatabase(tb, db)
		tb.Fatalf("ping %s: %v", benchmarkDriverName, err)
	}
	return db
}

func writeDriverGeneration(
	tb testing.TB,
	db *sql.DB,
	rows []driverChunk,
	reuseGenerationID int64,
) driverGeneration {
	tb.Helper()
	generation := prepareDriverGeneration(tb, db, rows, reuseGenerationID)
	pending := pendingDriverRows(tb, db, generation.id)
	completeDriverRows(tb, db, generation.id, rows, pending)
	return generation
}

func prepareDriverGeneration(
	tb testing.TB,
	db *sql.DB,
	rows []driverChunk,
	reuseGenerationID int64,
) driverGeneration {
	tb.Helper()
	tx, err := db.BeginTx(tb.Context(), nil)
	if err != nil {
		tb.Fatal(err)
	}
	q := catalog.New(tx)
	id, err := q.CreateGeneration(tb.Context(), catalog.CreateGenerationParams{
		VectorFormatVersion:     1,
		Model:                   "driver-benchmark",
		Dimension:               int64(len(rows[0].vector) / 4),
		ProtocolEpoch:           digestBytes("protocol"),
		ChunkerEpoch:            digestBytes("chunker"),
		VaultRoot:               "/driver-benchmark-vault",
		CorpusPolicyFingerprint: digestBytes("policy"),
		PolicySourceFingerprint: digestBytes("source"),
		TargetCorpusFingerprint: digestBytes("corpus:" + strconv.Itoa(len(rows))),
		ExpectedChunks:          int64(len(rows)),
	})
	if err != nil {
		rollbackDriverTx(tb, tx)
		tb.Fatal(err)
	}
	var previousPath string
	for i := range rows {
		row := &rows[i]
		if i == 0 || row.relPath != previousPath {
			if _, err := q.InsertGenerationNote(tb.Context(), catalog.InsertGenerationNoteParams{
				GenerationID: id,
				RelPath:      row.relPath,
				NoteHash:     row.noteHash[:],
			}); err != nil {
				rollbackDriverTx(tb, tx)
				tb.Fatal(err)
			}
			previousPath = row.relPath
		}
		if _, err := q.InsertGenerationChunkTarget(tb.Context(), catalog.InsertGenerationChunkTargetParams{
			GenerationID:  id,
			RelPath:       row.relPath,
			Ordinal:       row.ordinal,
			SubmittedHash: row.submittedHash[:],
		}); err != nil {
			rollbackDriverTx(tb, tx)
			tb.Fatal(err)
		}
	}
	if reuseGenerationID != 0 {
		if _, err := q.ReuseGenerationChunkVectors(tb.Context(), catalog.ReuseGenerationChunkVectorsParams{
			SourceGenerationID: reuseGenerationID,
			TargetGenerationID: id,
		}); err != nil {
			rollbackDriverTx(tb, tx)
			tb.Fatal(err)
		}
	}
	if _, err := q.SetStaging(tb.Context(), sql.NullInt64{Int64: id, Valid: true}); err != nil {
		rollbackDriverTx(tb, tx)
		tb.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
	return driverGeneration{id: id}
}

func pendingDriverRows(tb testing.TB, db *sql.DB, generationID int64) []catalog.PendingGenerationChunksRow {
	tb.Helper()
	rows, err := catalog.New(db).PendingGenerationChunks(tb.Context(), generationID)
	if err != nil {
		tb.Fatal(err)
	}
	return rows
}

func completeDriverRows(
	tb testing.TB,
	db *sql.DB,
	generationID int64,
	rows []driverChunk,
	pending []catalog.PendingGenerationChunksRow,
) {
	tb.Helper()
	byKey := make(map[string]driverChunk, len(rows))
	for i := range rows {
		row := rows[i]
		byKey[fmt.Sprintf("%s:%d", row.relPath, row.ordinal)] = row
	}
	for i := range pending {
		target := &pending[i]
		row, ok := byKey[fmt.Sprintf("%s:%d", target.RelPath, target.Ordinal)]
		if !ok {
			tb.Fatalf("pending target %q/%d has no row", target.RelPath, target.Ordinal)
			continue
		}
		reserve, err := db.BeginTx(tb.Context(), nil)
		if err != nil {
			tb.Fatal(err)
		}
		reserveQ := catalog.New(reserve)
		if _, insertErr := reserveQ.InsertAttempt(tb.Context(), catalog.InsertAttemptParams{
			GenerationID: generationID,
			RelPath:      row.relPath,
			Ordinal:      row.ordinal,
		}); insertErr != nil {
			rollbackDriverTx(tb, reserve)
			tb.Fatal(insertErr)
		}
		if reserveCommitErr := reserve.Commit(); reserveCommitErr != nil {
			tb.Fatal(reserveCommitErr)
		}

		put, err := db.BeginTx(tb.Context(), nil)
		if err != nil {
			tb.Fatal(err)
		}
		putQ := catalog.New(put)
		if _, completeErr := putQ.CompleteGenerationChunk(tb.Context(), catalog.CompleteGenerationChunkParams{
			Vector:              row.vector,
			TargetGenerationID:  generationID,
			TargetRelPath:       row.relPath,
			TargetOrdinal:       row.ordinal,
			TargetSubmittedHash: row.submittedHash[:],
			TargetNoteHash:      row.noteHash[:],
		}); completeErr != nil {
			rollbackDriverTx(tb, put)
			tb.Fatal(completeErr)
		}
		if _, deleteErr := putQ.DeleteAttempt(tb.Context(), catalog.DeleteAttemptParams{
			GenerationID: generationID,
			RelPath:      row.relPath,
			Ordinal:      row.ordinal,
		}); deleteErr != nil {
			rollbackDriverTx(tb, put)
			tb.Fatal(deleteErr)
		}
		if putCommitErr := put.Commit(); putCommitErr != nil {
			tb.Fatal(putCommitErr)
		}
	}
}

func activateDriverGeneration(tb testing.TB, db *sql.DB, generationID int64) {
	tb.Helper()
	tx, err := db.BeginTx(tb.Context(), nil)
	if err != nil {
		tb.Fatal(err)
	}
	q := catalog.New(tx)
	if _, err := q.ActivateStaging(tb.Context(), sql.NullInt64{Int64: generationID, Valid: true}); err != nil {
		rollbackDriverTx(tb, tx)
		tb.Fatal(err)
	}
	if _, err := q.DeleteUnreferencedGenerations(tb.Context()); err != nil {
		rollbackDriverTx(tb, tx)
		tb.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatal(err)
	}
}

func closeDriverDatabase(tb testing.TB, db *sql.DB) {
	tb.Helper()
	if err := db.Close(); err != nil {
		tb.Errorf("close driver database: %v", err)
	}
}

func rollbackDriverTx(tb testing.TB, tx *sql.Tx) {
	tb.Helper()
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		tb.Errorf("roll back driver transaction: %v", err)
	}
}

func driverRows(count, notes, dimension, revision int) []driverChunk {
	rows := make([]driverChunk, count)
	changedPath := fmt.Sprintf("Writing/driver/note-%06d.md", (count/2)%notes)
	for i := range rows {
		noteIndex := i % notes
		relPath := fmt.Sprintf("Writing/driver/note-%06d.md", noteIndex)
		ordinal := int64(i / notes)
		noteSeed := "note:" + relPath
		submittedSeed := fmt.Sprintf("submitted:%s:%d", relPath, ordinal)
		if revision > 0 && relPath == changedPath {
			noteSeed += ":revision:" + strconv.Itoa(revision)
			if ordinal == int64((count/2)/notes) {
				submittedSeed += ":revision:" + strconv.Itoa(revision)
			}
		}
		rows[i] = driverChunk{
			relPath:       relPath,
			noteHash:      sha256.Sum256([]byte(noteSeed)),
			ordinal:       ordinal,
			submittedHash: sha256.Sum256([]byte(submittedSeed)),
			vector:        driverVector(i+revision*count, dimension),
		}
	}
	slicesSortDriverRows(rows)
	return rows
}

func slicesSortDriverRows(rows []driverChunk) {
	slices.SortFunc(rows, func(a, b driverChunk) int {
		if a.relPath < b.relPath {
			return -1
		}
		if a.relPath > b.relPath {
			return 1
		}
		return cmp.Compare(a.ordinal, b.ordinal)
	})
}

func driverVector(seed, dimension int) []byte {
	vector := make([]byte, dimension*4)
	for i := range dimension {
		value := (seed*131 + i*17 + 2) % 1009
		binary.LittleEndian.PutUint32(vector[i*4:], math.Float32bits(float32(value+1)/1010))
	}
	return vector
}

func digestBytes(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}

func driverPositiveInt(tb testing.TB, name string, fallback int) int {
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

func requireOneDriverIteration(b *testing.B) {
	b.Helper()
	if b.N != 1 {
		b.Fatalf("benchmark iterations = %d, want 1; run with -benchtime=1x", b.N)
	}
}

func reportDriverFootprint(b *testing.B, prefix, path string) {
	b.Helper()
	var total int64
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		info, err := os.Stat(path + suffix)
		if err == nil {
			total += info.Size()
			continue
		}
		if !os.IsNotExist(err) {
			b.Fatalf("stat %q: %v", path+suffix, err)
		}
	}
	b.ReportMetric(float64(total), prefix+"_sqlite_bytes/op")
}
