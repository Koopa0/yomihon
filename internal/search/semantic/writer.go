package semantic

import (
	"context"
	"database/sql" //nolint:depguard // modernc's embedded SQLite driver is consumed through database/sql.
	_ "embed"      // Enable go:embed for the canonical SQLite schema below.
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

const storeSchemaVersion = 1

// storeSchemaSQL is schema v1 as validated by sqlc and installed atomically for
// a new store. Schema version and vector format version are independent; only
// the explicit full-build entry point may replace an unknown schema.
//
//go:embed sql/schema.sql
var storeSchemaSQL string

// writer owns the stable external writer lease and is the only constructor of
// mutable staging generations. Active and previous rows have no mutation API.
type writer struct {
	db        *sql.DB
	q         *catalog.Queries
	parent    *storeParent
	lease     *writerLease
	directory *storeDirectory

	beforeReset       func()
	beforeRenewCommit func() error

	closeOnce sync.Once
	closeErr  error
}

// openWriter acquires the nonblocking external lease, creates the private
// database when absent, installs the schema, and initializes its singleton
// catalog. It never resets an existing store implicitly.
func openWriter(ctx context.Context, path string) (*writer, error) {
	return openGenerationWriter(ctx, path, false)
}

// openRebuildWriter is reserved for an explicit full-build action. It behaves
// like the ordinary writer for a compatible store and replaces an incompatible
// schema under the same writer lease. Ordinary semantic search uses the
// non-rebuilding entry point so a query cannot discard paid vectors as a side
// effect.
func openRebuildWriter(ctx context.Context, path string) (*writer, error) {
	return openGenerationWriter(ctx, path, true)
}

// openRenewalWriter opens only an existing, compatible store. It acquires the
// existing writer lease without creating or resetting any store path. SQLite
// may perform crash recovery and WAL housekeeping under that lease; renewal
// eligibility is inspected in a rollback-only transaction.
func openRenewalWriter(ctx context.Context, path string) (*writer, error) {
	if err := requireSemanticStorePlatform(); err != nil {
		return nil, err
	}
	parent, err := openStoreParent(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrStoreNotFound
	}
	if err != nil {
		return nil, err
	}
	lease, err := acquireExistingWriterLease(parent, writerLeaseName(path))
	if err != nil {
		return nil, errors.Join(err, parent.Close())
	}
	writer := &writer{parent: parent, lease: lease}
	directory, err := openStoreDirectory(parent.root, path, false)
	if errors.Is(err, os.ErrNotExist) {
		return failWriter(writer, ErrStoreNotFound)
	}
	if err != nil {
		return failWriter(writer, err)
	}
	writer.directory = directory
	exists, err := directory.inspectDatabase()
	if err != nil {
		return failWriter(writer, err)
	}
	if !exists {
		return failWriter(writer, ErrStoreNotFound)
	}
	if currentErr := writer.requireCurrentFiles(); currentErr != nil {
		return failWriter(writer, currentErr)
	}
	db, err := openStoreDB(ctx, path, false)
	if err != nil {
		return failWriter(writer, classifyStoreReadError(err))
	}
	writer.db = db
	writer.q = catalog.New(db)
	if err := requireStoreSchema(ctx, db); err != nil {
		return failWriter(writer, err)
	}
	if err := inspectWriterStore(ctx, db, writer.q); err != nil {
		return failWriter(writer, classifyStoreReadError(err))
	}
	if err := writer.requireCurrentFiles(); err != nil {
		return failWriter(writer, err)
	}
	return writer, nil
}

type writerOpenHooks struct {
	afterLease  func()
	beforeReset func()
}

func openGenerationWriter(ctx context.Context, path string, rebuildIncompatible bool) (*writer, error) {
	return openGenerationWriterWithHooks(ctx, path, rebuildIncompatible, writerOpenHooks{})
}

func openGenerationWriterWithHooks(
	ctx context.Context,
	path string,
	rebuildIncompatible bool,
	hooks writerOpenHooks,
) (*writer, error) {
	if err := requireSemanticStorePlatform(); err != nil {
		return nil, err
	}
	writer, err := openWriterFiles(path, hooks)
	if err != nil {
		return nil, err
	}
	if fileErr := ensureWriterDatabaseFile(writer); fileErr != nil {
		return failWriter(writer, fileErr)
	}
	needsBootstrap, err := attachWriterDatabase(ctx, writer, path, rebuildIncompatible)
	if err != nil {
		return failWriter(writer, err)
	}
	if needsBootstrap {
		if err := bootstrapStore(ctx, writer.db); err != nil {
			return failWriter(writer, err)
		}
		if err := writer.requireCurrentFiles(); err != nil {
			return failWriter(writer, err)
		}
	}
	if err := ensureWriterCatalog(ctx, writer, path, rebuildIncompatible); err != nil {
		return failWriter(writer, err)
	}
	if err := writer.requireCurrentFiles(); err != nil {
		return failWriter(writer, err)
	}
	return writer, nil
}

func openWriterFiles(path string, hooks writerOpenHooks) (*writer, error) {
	parent, err := openStoreParent(path, true)
	if err != nil {
		return nil, err
	}
	lease, err := acquireWriterLease(parent, writerLeaseName(path))
	if err != nil {
		return nil, errors.Join(err, parent.Close())
	}
	writer := &writer{parent: parent, lease: lease, beforeReset: hooks.beforeReset}
	if hooks.afterLease != nil {
		hooks.afterLease()
	}
	if rootErr := parent.requireCurrent(); rootErr != nil {
		return failWriter(writer, rootErr)
	}
	directory, err := openStoreDirectory(parent.root, path, true)
	if err != nil {
		return failWriter(writer, err)
	}
	writer.directory = directory
	return writer, nil
}

func ensureWriterDatabaseFile(writer *writer) error {
	exists, fileErr := writer.directory.inspectDatabase()
	if fileErr != nil {
		return fileErr
	}
	if !exists {
		if createErr := writer.directory.createDatabase(); createErr != nil {
			return createErr
		}
	}
	return writer.requireCurrentFiles()
}

func writerLeaseName(path string) string {
	dir := filepath.Dir(path)
	return filepath.Base(dir) + ".lock"
}

func attachWriterDatabase(ctx context.Context, writer *writer, path string, rebuild bool) (bool, error) {
	db, err := openStoreDB(ctx, path, false)
	if err != nil {
		return resetCorruptWriter(ctx, writer, path, rebuild, err)
	}
	writer.db = db
	writer.q = catalog.New(db)
	if currentErr := writer.requireCurrentFiles(); currentErr != nil {
		return false, currentErr
	}
	version, err := readStoreSchemaVersion(ctx, db)
	if err != nil {
		return resetCorruptWriter(ctx, writer, path, rebuild, err)
	}
	switch version {
	case storeSchemaVersion:
		return false, nil
	case 0:
		hasObjects, inspectErr := storeHasApplicationObjects(ctx, db)
		if inspectErr != nil {
			return false, classifyStoreReadError(inspectErr)
		}
		if !hasObjects {
			// A SQLite header with no application objects is the deterministic
			// crash image left before the first bootstrap transaction committed.
			return true, nil
		}
		if !rebuild {
			return false, ErrStoreSchemaMismatch
		}
	default:
		if !rebuild {
			return false, ErrStoreSchemaMismatch
		}
	}
	if err := resetWriterDatabase(ctx, writer, path); err != nil {
		return false, err
	}
	return true, nil
}

func resetCorruptWriter(ctx context.Context, writer *writer, path string, rebuild bool, cause error) (bool, error) {
	if !rebuild || !isSQLiteCorruption(cause) {
		return false, classifyStoreReadError(cause)
	}
	if err := resetWriterDatabase(ctx, writer, path); err != nil {
		return false, err
	}
	return true, nil
}

func ensureWriterCatalog(ctx context.Context, writer *writer, path string, rebuild bool) error {
	err := inspectWriterStore(ctx, writer.db, writer.q)
	if err == nil {
		return nil
	}
	if rebuild && (errors.Is(err, ErrStoreCorrupt) || isSQLiteCorruption(err)) {
		if resetErr := resetWriterDatabase(ctx, writer, path); resetErr != nil {
			return resetErr
		}
		if bootstrapErr := bootstrapStore(ctx, writer.db); bootstrapErr != nil {
			return bootstrapErr
		}
		err = inspectWriterStore(ctx, writer.db, writer.q)
	}
	if err != nil {
		return classifyStoreReadError(err)
	}
	return nil
}

func inspectWriterStore(ctx context.Context, db *sql.DB, q *catalog.Queries) error {
	if _, err := q.ValidateSchemaShape(ctx); err != nil {
		return fmt.Errorf("%w: validate schema shape: %w", ErrStoreCorrupt, err)
	}
	if err := requireExactStoreSchema(ctx, db); err != nil {
		return err
	}
	violations, err := foreignKeyViolationCount(ctx, db)
	if err != nil {
		return fmt.Errorf("%w: validate foreign keys: %w", ErrStoreCorrupt, err)
	}
	if violations != 0 {
		return fmt.Errorf("%w: %d foreign-key violations", ErrStoreCorrupt, violations)
	}
	if _, err := q.Catalog(ctx); errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: missing role catalog", ErrStoreCorrupt)
	} else if err != nil {
		return fmt.Errorf("read semantic generation catalog: %w", err)
	}
	return nil
}

func foreignKeyViolationCount(ctx context.Context, db *sql.DB) (count int, resultErr error) {
	rows, err := db.QueryContext(ctx, `PRAGMA main.foreign_key_check`)
	if err != nil {
		return 0, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, rows.Close())
	}()
	for rows.Next() {
		var table, parent string
		var rowID sql.NullInt64
		var foreignKeyID int64
		if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			return 0, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func failWriter(writer *writer, primary error) (*writer, error) {
	return nil, errors.Join(primary, writer.Close())
}

func resetWriterDatabase(ctx context.Context, writer *writer, path string) error {
	// This destructive path is limited to an explicit build after the current
	// binary has proved the store corrupt or schema-incompatible; no generation
	// in that file is serveable here. SQLite journals and WAL files are part of
	// the database, so renaming only the main file while another connection is
	// open is not an atomic replacement. A future compatible upgrade requires a
	// version-specific copy-forward path with its own publication proof.
	if writer.beforeReset != nil {
		hook := writer.beforeReset
		writer.beforeReset = nil
		hook()
	}
	if err := writer.requireCurrentFiles(); err != nil {
		return err
	}
	if writer.db != nil {
		if err := writer.db.Close(); err != nil {
			return fmt.Errorf("close incompatible semantic store: %w", err)
		}
	}
	writer.db = nil
	writer.q = nil
	if err := writer.directory.resetDatabaseFiles(); err != nil {
		return err
	}
	if err := writer.requireCurrentFiles(); err != nil {
		return err
	}
	db, err := openStoreDB(ctx, path, false)
	if err != nil {
		return err
	}
	writer.db = db
	writer.q = catalog.New(db)
	if err := writer.requireCurrentFiles(); err != nil {
		return err
	}
	return nil
}

func bootstrapStore(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin semantic generation bootstrap: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	if _, err := tx.ExecContext(ctx, storeSchemaSQL); err != nil {
		return fmt.Errorf("initialize semantic generation schema: %w", err)
	}
	if err := catalog.New(tx).InitializeCatalog(ctx); err != nil {
		return fmt.Errorf("initialize semantic generation catalog: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit semantic generation bootstrap: %w", err)
	}
	return nil
}

func requireStoreSchema(ctx context.Context, db *sql.DB) error {
	version, err := readStoreSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version == 0 {
		hasObjects, err := storeHasApplicationObjects(ctx, db)
		if err != nil {
			return err
		}
		if !hasObjects {
			return ErrStoreNotFound
		}
	}
	if version != storeSchemaVersion {
		return ErrStoreSchemaMismatch
	}
	if _, err := catalog.New(db).ValidateSchemaShape(ctx); err != nil {
		return fmt.Errorf("%w: validate schema shape: %w", ErrStoreCorrupt, err)
	}
	if err := requireExactStoreSchema(ctx, db); err != nil {
		return err
	}
	return nil
}

func readStoreSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read semantic store schema version: %w", err)
	}
	return version, nil
}

func storeHasApplicationObjects(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `
			SELECT count(*)
			FROM sqlite_schema
			WHERE name NOT LIKE 'sqlite_%'
		`).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect semantic store schema: %w", err)
	}
	return count != 0, nil
}

func storeFilePaths(path string) []string {
	return []string{path, path + "-wal", path + "-shm", path + "-journal"}
}

func storeFileNames(name string) []string {
	return []string{name, name + "-wal", name + "-shm", name + "-journal"}
}

func (w *writer) requireCurrentFiles() error {
	if w == nil || w.parent == nil || w.lease == nil || w.directory == nil {
		return ErrStorePermissions
	}
	if err := w.parent.requireCurrent(); err != nil {
		return err
	}
	return w.directory.requireCurrent(w.parent.root)
}

func (w *writer) commit(tx *sql.Tx, operation string) error {
	if err := w.requireCurrentFiles(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", operation, err)
	}
	return w.requireCurrentFiles()
}

// Close closes SQLite and then releases the process-owned writer lease. A
// checkpoint is deliberately not part of command correctness: a committed
// activation remains success even if later cleanup encounters an error.
func (w *writer) Close() error {
	if w == nil {
		return nil
	}
	w.closeOnce.Do(func() {
		w.closeErr = errors.Join(
			closeWriterDB(w.db),
			closeStoreDirectory(w.directory),
			closeStoreParent(w.parent),
			closeWriterLease(w.lease),
		)
	})
	return w.closeErr
}

func closeWriterDB(db *sql.DB) error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close semantic generation writer: %w", err)
	}
	return nil
}

func closeStoreDirectory(directory *storeDirectory) error {
	if directory == nil {
		return nil
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close semantic generation directory: %w", err)
	}
	return nil
}

func closeStoreParent(parent *storeParent) error {
	if parent == nil {
		return nil
	}
	if err := parent.Close(); err != nil {
		return fmt.Errorf("close semantic cache directory: %w", err)
	}
	return nil
}

func closeWriterLease(lease *writerLease) error {
	if lease == nil {
		return nil
	}
	return lease.Close()
}
