package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql" //nolint:depguard // modernc's embedded SQLite driver is consumed through database/sql.
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/koopa0/yomihon/internal/search/semantic/catalog"
)

// Store errors distinguish absence, corruption, unsafe permissions,
// compatibility, writer ownership, and incomplete generation state.
var (
	ErrStoreNotFound             = errors.New("semantic generation store does not exist")
	ErrNoActiveGeneration        = errors.New("semantic generation store has no active generation")
	ErrStoreCorrupt              = errors.New("semantic generation store is corrupt")
	ErrStorePermissions          = errors.New("semantic generation store permissions are unsafe")
	ErrStoreSchemaMismatch       = errors.New("semantic generation store schema is incompatible")
	ErrStoreUnsupportedPlatform  = errors.New("semantic generation store is unsupported on this platform")
	ErrVectorFormatMismatch      = errors.New("semantic generation vector format is incompatible")
	ErrWriterHeld                = errors.New("semantic generation writer lease is held")
	ErrInvalidIdentity           = errors.New("invalid semantic generation identity")
	ErrInvalidChunk              = errors.New("invalid semantic chunk vector")
	ErrInvalidCorpus             = errors.New("invalid semantic corpus manifest")
	ErrStagingClosed             = errors.New("semantic staging generation is no longer active")
	ErrGenerationIncomplete      = errors.New("semantic staging generation is incomplete")
	ErrAttemptLimit              = errors.New("semantic chunk attempt limit is exhausted")
	ErrAttemptBudgetNotRenewable = errors.New("semantic chunk attempt budget is not renewable")
	ErrRetryNotReady             = errors.New("semantic chunk retry is not ready")
)

// generationMetadata identifies one complete immutable generation without
// claiming that its vector rows are resident in memory.
type generationMetadata struct {
	identity          generationIdentity
	corpusFingerprint [sha256.Size]byte
	topKP95           time.Duration
}

// loadedGeneration is one complete active generation loaded in a single
// SQLite read snapshot. Its rows remain exclusively owned until they are
// consumed into an in-memory index.
type loadedGeneration struct {
	generationMetadata

	rows []ChunkVector
}

// storeParent pins the cache directory that owns both the semantic store
// child and its sibling writer lease. The child may be replaced by a failed or
// adversarial action; the parent capability must remain stable for the handle's
// entire lifetime.
type storeParent struct {
	root *os.Root
	path string
}

func openStoreParent(path string, create bool) (*storeParent, error) {
	parentPath := filepath.Dir(filepath.Dir(path))
	if create {
		if err := createStoreParent(parentPath); err != nil {
			return nil, err
		}
	}
	before, err := lstatStorePath(parentPath)
	if err != nil {
		return nil, fmt.Errorf("stat semantic cache directory: %w", err)
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("%w: cache directory must not be group/world-writable", ErrStorePermissions)
	}
	root, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, fmt.Errorf("open semantic cache directory: %w", err)
	}
	parent := &storeParent{root: root, path: parentPath}
	if err := parent.requireCurrent(); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return parent, nil
}

func createStoreParent(path string) error {
	ancestor, err := existingStoreAncestor(path)
	if err != nil {
		return err
	}
	if ancestor.path == path {
		return nil
	}
	root, err := os.OpenRoot(ancestor.path)
	if err != nil {
		return fmt.Errorf("open semantic cache ancestor: %w", err)
	}
	pinned, err := root.Stat(".")
	if err != nil {
		return errors.Join(fmt.Errorf("inspect semantic cache ancestor: %w", err), root.Close())
	}
	if !os.SameFile(ancestor.info, pinned) {
		return errors.Join(
			fmt.Errorf("%w: cache ancestor changed identity", ErrStorePermissions),
			root.Close(),
		)
	}
	relative, err := filepath.Rel(ancestor.path, path)
	if err != nil {
		return errors.Join(fmt.Errorf("resolve semantic cache path: %w", err), root.Close())
	}
	current := root
	for component := range strings.SplitSeq(relative, string(filepath.Separator)) {
		next, openErr := openOrCreateCacheDirectory(current, component)
		if openErr != nil {
			return errors.Join(openErr, current.Close())
		}
		if closeErr := current.Close(); closeErr != nil {
			return errors.Join(fmt.Errorf("close semantic cache ancestor: %w", closeErr), next.Close())
		}
		current = next
	}
	return current.Close()
}

type storeAncestor struct {
	path string
	info os.FileInfo
}

func existingStoreAncestor(path string) (storeAncestor, error) {
	for candidate := path; ; candidate = filepath.Dir(candidate) {
		info, err := lstatStorePath(candidate)
		if err == nil {
			if !safeCacheDirectory(info) {
				return storeAncestor{}, fmt.Errorf("%w: cache ancestor must not be a symlink or group/world-writable", ErrStorePermissions)
			}
			return storeAncestor{path: candidate, info: info}, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return storeAncestor{}, fmt.Errorf("stat semantic cache ancestor: %w", err)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return storeAncestor{}, fmt.Errorf("stat semantic cache ancestor: %w", err)
		}
	}
}

func openOrCreateCacheDirectory(parent *os.Root, name string) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		if mkdirErr := parent.Mkdir(name, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
			return nil, fmt.Errorf("create semantic cache directory: %w", mkdirErr)
		}
		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, fmt.Errorf("stat semantic cache directory: %w", err)
	}
	if !safeCacheDirectory(info) {
		return nil, fmt.Errorf("%w: cache directory must not be a symlink or group/world-writable", ErrStorePermissions)
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open semantic cache directory: %w", err)
	}
	pinned, err := root.Stat(".")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("inspect semantic cache directory: %w", err), root.Close())
	}
	if !os.SameFile(info, pinned) {
		return nil, errors.Join(
			fmt.Errorf("%w: cache directory changed identity", ErrStorePermissions),
			root.Close(),
		)
	}
	return root, nil
}

func (p *storeParent) requireCurrent() error {
	if p == nil || p.root == nil {
		return ErrStorePermissions
	}
	current, err := lstatStorePath(p.path)
	if err != nil {
		return fmt.Errorf("stat semantic cache directory: %w", err)
	}
	pinned, err := p.root.Stat(".")
	if err != nil {
		return fmt.Errorf("inspect semantic cache directory: %w", err)
	}
	if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || current.Mode().Perm()&0o022 != 0 ||
		!os.SameFile(current, pinned) {
		return fmt.Errorf("%w: cache directory changed identity or is group/world-writable", ErrStorePermissions)
	}
	return nil
}

func lstatStorePath(path string) (info os.FileInfo, resultErr error) {
	if path == string(filepath.Separator) {
		root, err := os.OpenRoot(path)
		if err != nil {
			return nil, err
		}
		defer func() {
			resultErr = errors.Join(resultErr, root.Close())
		}()
		return root.Stat(".")
	}
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, parent.Close())
	}()
	return parent.Lstat(filepath.Base(path))
}

func (p *storeParent) Close() error {
	if p == nil || p.root == nil {
		return nil
	}
	return p.root.Close()
}

type storeDirectory struct {
	root         *os.Root
	name         string
	database     string
	databaseInfo os.FileInfo
}

func openStoreDirectory(parent *os.Root, path string, create bool) (*storeDirectory, error) {
	name := filepath.Base(filepath.Dir(path))
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) && create {
		if mkdirErr := parent.Mkdir(name, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
			return nil, fmt.Errorf("create semantic generation directory: %w", mkdirErr)
		}
		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, fmt.Errorf("stat semantic generation directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return nil, fmt.Errorf("%w: dedicated directory must have mode 0700", ErrStorePermissions)
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open semantic generation directory: %w", err)
	}
	pinned, err := root.Stat(".")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("inspect semantic generation directory: %w", err), root.Close())
	}
	if !os.SameFile(info, pinned) {
		return nil, errors.Join(
			fmt.Errorf("%w: semantic generation directory changed identity", ErrStorePermissions),
			root.Close(),
		)
	}
	return &storeDirectory{
		root:     root,
		name:     name,
		database: filepath.Base(path),
	}, nil
}

func (d *storeDirectory) requireCurrent(parent *os.Root) error {
	if d == nil || d.root == nil || parent == nil {
		return ErrStorePermissions
	}
	current, err := parent.Lstat(d.name)
	if err != nil {
		return fmt.Errorf("stat semantic generation directory: %w", err)
	}
	pinned, err := d.root.Stat(".")
	if err != nil {
		return fmt.Errorf("inspect semantic generation directory: %w", err)
	}
	if !privateStoreDirectory(current) || !os.SameFile(current, pinned) {
		return fmt.Errorf("%w: semantic generation directory changed identity or permissions", ErrStorePermissions)
	}
	if d.databaseInfo == nil {
		return fmt.Errorf("%w: semantic generation database identity is not pinned", ErrStorePermissions)
	}
	database, err := d.root.Lstat(d.database)
	if err != nil {
		return fmt.Errorf("stat semantic generation store: %w", err)
	}
	if !privateStoreFile(database) || !os.SameFile(database, d.databaseInfo) {
		return fmt.Errorf("%w: database file changed identity or permissions", ErrStorePermissions)
	}
	return d.requirePrivateSidecars()
}

func (d *storeDirectory) requirePrivateSidecars() error {
	for _, name := range storeFileNames(d.database)[1:] {
		info, err := d.root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat semantic generation sidecar %q: %w", name, err)
		}
		if !privateStoreFile(info) {
			return fmt.Errorf("%w: SQLite sidecar %q must be a regular 0600 file", ErrStorePermissions, name)
		}
	}
	return nil
}

func privateStoreDirectory(info os.FileInfo) bool {
	return info != nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0o700
}

func safeCacheDirectory(info os.FileInfo) bool {
	return info != nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0o022 == 0
}

func privateStoreFile(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0o600
}

func (d *storeDirectory) inspectDatabase() (bool, error) {
	info, err := d.root.Lstat(d.database)
	if errors.Is(err, os.ErrNotExist) {
		d.databaseInfo = nil
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat semantic generation store: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return false, fmt.Errorf("%w: database file must have mode 0600", ErrStorePermissions)
	}
	d.databaseInfo = info
	return true, nil
}

func (d *storeDirectory) createDatabase() error {
	file, err := d.root.OpenFile(d.database, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create semantic generation store: %w", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		return errors.Join(fmt.Errorf("stat semantic generation store: %w", statErr), file.Close())
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.Join(ErrStorePermissions, file.Close())
	}
	d.databaseInfo = info
	if err := file.Close(); err != nil {
		return fmt.Errorf("close semantic generation bootstrap file: %w", err)
	}
	return nil
}

func (d *storeDirectory) resetDatabaseFiles() error {
	for _, name := range storeFileNames(d.database) {
		if err := d.root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove incompatible semantic store file %q: %w", name, err)
		}
	}
	file, err := d.root.OpenFile(d.database, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("recreate semantic generation store: %w", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		return errors.Join(fmt.Errorf("stat recreated semantic generation store: %w", statErr), file.Close())
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.Join(ErrStorePermissions, file.Close())
	}
	d.databaseInfo = info
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("close recreated semantic generation store: %w", closeErr)
	}
	return nil
}

func (d *storeDirectory) Close() error {
	if d == nil || d.root == nil {
		return nil
	}
	return d.root.Close()
}

// store is a read-only handle to a semantic generation database.
type store struct {
	db        *sql.DB
	q         *catalog.Queries
	parent    *storeParent
	directory *storeDirectory

	closeOnce sync.Once
	closeErr  error

	// afterGeneration is a deterministic test hook for proving that catalog,
	// generation metadata, and rows share one read snapshot.
	afterGeneration func()
}

// openStore opens an existing database read-only without creating any file.
func openStore(ctx context.Context, path string) (*store, error) {
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
	directory, err := openStoreDirectory(parent.root, path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrStoreNotFound, parent.Close())
	}
	if err != nil {
		return nil, errors.Join(err, parent.Close())
	}
	exists, err := directory.inspectDatabase()
	if err != nil {
		return nil, errors.Join(err, directory.Close(), parent.Close())
	}
	if !exists {
		return nil, errors.Join(ErrStoreNotFound, directory.Close(), parent.Close())
	}
	reader := &store{parent: parent, directory: directory}
	if currentErr := reader.requireCurrentFiles(); currentErr != nil {
		return nil, errors.Join(currentErr, reader.Close())
	}
	db, err := openStoreDB(ctx, path, true)
	if err != nil {
		return nil, errors.Join(classifyStoreReadError(err), reader.Close())
	}
	reader.db = db
	reader.q = catalog.New(db)
	if currentErr := reader.requireCurrentFiles(); currentErr != nil {
		return nil, errors.Join(currentErr, reader.Close())
	}
	if schemaErr := requireStoreSchema(ctx, db); schemaErr != nil {
		return nil, errors.Join(schemaErr, reader.Close())
	}
	if currentErr := reader.requireCurrentFiles(); currentErr != nil {
		return nil, errors.Join(currentErr, reader.Close())
	}
	return reader, nil
}
func isSQLiteCorruption(err error) bool {
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok {
		return false
	}
	code := sqliteErr.Code() & 0xff
	return code == sqlite3.SQLITE_CORRUPT || code == sqlite3.SQLITE_NOTADB
}

func classifyStoreReadError(err error) error {
	if isSQLiteCorruption(err) {
		return fmt.Errorf("%w: %w", ErrStoreCorrupt, err)
	}
	return err
}

func openStoreDB(ctx context.Context, path string, readOnly bool) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteStoreURL(path, readOnly))
	if err != nil {
		return nil, fmt.Errorf("open semantic generation store: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("ping semantic generation store: %w", err), db.Close())
	}
	return db, nil
}

func sqliteStoreURL(path string, readOnly bool) string {
	u := &url.URL{Scheme: "file", Path: path}
	query := u.Query()
	query.Add("_pragma", "busy_timeout(5000)")
	if readOnly {
		query.Set("mode", "ro")
		query.Add("_pragma", "query_only(ON)")
	} else {
		query.Set("mode", "rw")
		query.Add("_pragma", "foreign_keys(ON)")
		query.Add("_pragma", "journal_mode(WAL)")
		query.Add("_pragma", "synchronous(FULL)")
	}
	u.RawQuery = query.Encode()
	return u.String()
}

// Active loads only the active role. A corrupt staging or previous generation
// cannot poison it, and a corrupt active generation is never replaced by an
// automatic previous-generation fallback.
func (s *store) Active(ctx context.Context) (loadedGeneration, error) {
	if s == nil || s.db == nil {
		return loadedGeneration{}, ErrStoreNotFound
	}
	if err := s.requireCurrentFiles(); err != nil {
		return loadedGeneration{}, err
	}
	generation, err := readActive(ctx, s.db, s.afterGeneration)
	if err != nil {
		return loadedGeneration{}, err
	}
	if err := s.requireCurrentFiles(); err != nil {
		return loadedGeneration{}, err
	}
	return generation, nil
}

// Active loads the active generation while the caller retains the writer
// lease. It does not expose or mutate staging.
func (w *writer) Active(ctx context.Context) (loadedGeneration, error) {
	if w == nil || w.db == nil {
		return loadedGeneration{}, ErrStoreNotFound
	}
	if err := w.requireCurrentFiles(); err != nil {
		return loadedGeneration{}, err
	}
	generation, err := readActive(ctx, w.db, nil)
	if err != nil {
		return loadedGeneration{}, err
	}
	if err := w.requireCurrentFiles(); err != nil {
		return loadedGeneration{}, err
	}
	return generation, nil
}

func readActive(ctx context.Context, db *sql.DB, afterGeneration func()) (loadedGeneration, error) {
	// modernc treats TxOptions.ReadOnly as intent, not enforcement. Store
	// readers are actually protected by mode=ro plus query_only(ON) in
	// sqliteStoreURL; retain the option here so the transaction shape is clear.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return loadedGeneration{}, fmt.Errorf("begin semantic generation read: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit; a pre-commit failure is already returned
	generation, err := activeGeneration(ctx, catalog.New(tx), afterGeneration)
	if err != nil {
		return loadedGeneration{}, err
	}
	if err := tx.Commit(); err != nil {
		return loadedGeneration{}, fmt.Errorf("commit semantic generation read: %w", err)
	}
	return generation, nil
}

func activeGeneration(ctx context.Context, q *catalog.Queries, afterGeneration func()) (loadedGeneration, error) {
	roles, err := q.Catalog(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return loadedGeneration{}, fmt.Errorf("%w: missing role catalog", ErrStoreCorrupt)
	}
	if err != nil {
		return loadedGeneration{}, fmt.Errorf("read semantic role catalog: %w", err)
	}
	if !roles.ActiveGenerationID.Valid {
		return loadedGeneration{}, ErrNoActiveGeneration
	}
	metadata, err := storedGenerationByID(ctx, q, roles.ActiveGenerationID.Int64)
	if err != nil {
		return loadedGeneration{}, err
	}
	if metadata.vectorFormatVersion != vectorFormatVersion {
		return loadedGeneration{}, ErrVectorFormatMismatch
	}
	if !withinExactScanCapacity(metadata.expectedChunks, metadata.identity.Dimension()) {
		return loadedGeneration{}, ErrIndexCapacity
	}
	if afterGeneration != nil {
		afterGeneration()
	}
	rows, err := generationRows(ctx, q, &metadata)
	if err != nil {
		return loadedGeneration{}, err
	}
	if validateErr := validateCompleteGenerationRows(ctx, q, &metadata, rows, fmt.Errorf("%w: active row count", ErrStoreCorrupt)); validateErr != nil {
		return loadedGeneration{}, validateErr
	}
	attempts, err := q.GenerationAttemptCount(ctx, metadata.id)
	if err != nil {
		return loadedGeneration{}, fmt.Errorf("read active semantic retry state: %w", err)
	}
	if attempts != 0 {
		return loadedGeneration{}, fmt.Errorf("%w: active generation has retry state", ErrStoreCorrupt)
	}
	return loadedGeneration{
		identity:          metadata.identity,
		corpusFingerprint: metadata.corpusFingerprint,
		topKP95:           metadata.topKP95,
		rows:              rows,
	}, nil
}

// Close closes the read-only handle. It is safe to call more than once.
func (s *store) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		var errs []error
		if s.db != nil {
			if err := s.db.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close semantic generation store: %w", err))
			}
		}
		if s.directory != nil {
			if err := s.directory.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close semantic generation directory: %w", err))
			}
		}
		if s.parent != nil {
			if err := s.parent.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close semantic cache directory: %w", err))
			}
		}
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}

func (s *store) requireCurrentFiles() error {
	if s == nil || s.parent == nil || s.directory == nil {
		return ErrStorePermissions
	}
	if err := s.parent.requireCurrent(); err != nil {
		return err
	}
	return s.directory.requireCurrent(s.parent.root)
}
