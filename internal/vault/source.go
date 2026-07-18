package vault

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ErrSourceChanged means a vault entry no longer names the regular file that
// was selected for this read.
var ErrSourceChanged = errors.New("vault source changed while it was being read")

// ErrCanonicalCollision means two filesystem names normalize to the same
// vault-relative NFC path. Such a tree cannot be projected without guessing.
var ErrCanonicalCollision = errors.New("vault contains canonically colliding paths")

type readerToken [1]byte

// Reader pins one selected vault directory for the lifetime of an action.
// Paths exposed to callers are NFC-normalized; filesystem lookup retains the
// original directory-entry spelling in Entry.
type Reader struct {
	name  string
	root  *os.Root
	token *readerToken
}

// File is an opened regular file selected through a Reader. Its descriptor
// keeps referring to the selected filesystem object if its path is renamed or
// atomically replaced. Like os.File, it does not freeze bytes written through
// another descriptor to that same object.
type File struct {
	file *os.File
}

// Read reads from f.
func (f *File) Read(p []byte) (int, error) {
	if f == nil || f.file == nil {
		return 0, fs.ErrClosed
	}
	return f.file.Read(p)
}

// Seek sets the offset for the next read from f.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f == nil || f.file == nil {
		return 0, fs.ErrClosed
	}
	return f.file.Seek(offset, whence)
}

// Close releases f.
func (f *File) Close() error {
	if f == nil || f.file == nil {
		return nil
	}
	return f.file.Close()
}

// Entry is an opaque reference to one regular file and its parent chain as
// they were observed by a Reader. Path is the canonical path used by policy
// and output; rawPath and observed are used only for descriptor-relative,
// fail-closed lookup through the Reader that created it.
type Entry struct {
	token    *readerToken
	rawPath  string
	path     string
	observed []sourceObservation
}

// Size returns the observed size of e. An invalid Entry has size zero.
func (e Entry) Size() int64 {
	if len(e.observed) == 0 || e.observed[len(e.observed)-1].info == nil {
		return 0
	}
	return e.observed[len(e.observed)-1].info.Size()
}

// ModTime returns the observed modification time of e. An invalid Entry has
// the zero time.
func (e Entry) ModTime() time.Time {
	if len(e.observed) == 0 || e.observed[len(e.observed)-1].info == nil {
		return time.Time{}
	}
	return e.observed[len(e.observed)-1].info.ModTime()
}

// Problem records one nested path that an available scan could not observe.
type Problem struct {
	path string
	err  error
}

// Path returns the canonical vault-relative path associated with p. The root
// is represented by ".".
func (p Problem) Path() string { return p.path }

// Err returns the observation error associated with p.
func (p Problem) Err() error { return p.err }

// Scan is an immutable observation of one Reader's file domain.
type Scan struct {
	state *scanState
}

type scanState struct {
	valid    bool
	rootName string
	rootInfo fs.FileInfo
	files    []Entry
	entries  map[string]Entry
	contains map[string]struct{}
	problems []Problem
}

// Files returns the observed regular files in canonical path order.
func (s Scan) Files() []Entry {
	if s.state == nil {
		return nil
	}
	files := make([]Entry, len(s.state.files))
	for i := range s.state.files {
		files[i] = cloneEntry(s.state.files[i])
	}
	return files
}

// Entry returns the regular file with canonicalPath. Non-canonical paths do
// not match.
func (s Scan) Entry(canonicalPath string) (Entry, bool) {
	if !validCanonicalPath(canonicalPath) {
		return Entry{}, false
	}
	if s.state == nil {
		return Entry{}, false
	}
	entry, ok := s.state.entries[canonicalPath]
	return cloneEntry(entry), ok
}

// Contains reports whether canonicalPath was observed as a directory or a
// regular file. Non-canonical paths do not match.
func (s Scan) Contains(canonicalPath string) bool {
	if !validCanonicalPath(canonicalPath) {
		return false
	}
	if s.state == nil {
		return false
	}
	_, ok := s.state.contains[canonicalPath]
	return ok
}

// Problems returns the nested paths an available scan could not observe.
func (s Scan) Problems() []Problem {
	if s.state == nil {
		return nil
	}
	return slices.Clone(s.state.problems)
}

// SameFiles reports whether s and other observed the same rooted file domain.
// It compares every regular file's canonical and raw path, metadata, object
// identity, and parent-chain identity.
func (s Scan) SameFiles(other Scan) bool {
	if s.state == nil || other.state == nil || !s.state.valid || !other.state.valid ||
		s.state.rootName != other.state.rootName || !sameObject(s.state.rootInfo, other.state.rootInfo) ||
		len(s.state.files) != len(other.state.files) {
		return false
	}
	left := slices.Clone(s.state.files)
	right := slices.Clone(other.state.files)
	slices.SortFunc(left, compareEntryPath)
	slices.SortFunc(right, compareEntryPath)
	for i := range left {
		if !sameEntryObservation(left[i], right[i]) {
			return false
		}
	}
	return true
}

func compareEntryPath(a, b Entry) int {
	if byCanonical := strings.Compare(a.path, b.path); byCanonical != 0 {
		return byCanonical
	}
	return strings.Compare(a.rawPath, b.rawPath)
}

func sameEntryObservation(a, b Entry) bool {
	if a.path != b.path || a.rawPath != b.rawPath || len(a.observed) == 0 ||
		len(a.observed) != len(b.observed) {
		return false
	}
	for i := range a.observed {
		if a.observed[i].rawPath != b.observed[i].rawPath {
			return false
		}
		if i == len(a.observed)-1 {
			if !sameObservation(a.observed[i].info, b.observed[i].info) {
				return false
			}
			continue
		}
		if !sameObject(a.observed[i].info, b.observed[i].info) {
			return false
		}
	}
	return true
}

func cloneEntry(entry Entry) Entry {
	entry.observed = slices.Clone(entry.observed)
	return entry
}

func validCanonicalPath(relPath string) bool {
	return fs.ValidPath(relPath) && relPath == NormalizeNFC(relPath)
}

type sourceObservation struct {
	rawPath string
	info    fs.FileInfo
}

// Path returns the NFC, slash-form vault-relative identity of e.
func (e Entry) Path() string { return e.path }

// Open pins root and returns its read capability.
func Open(root string) (*Reader, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("open vault root: %w", err)
	}
	absolute, err = filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return nil, fmt.Errorf("open vault root: %w", err)
	}
	opened, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, fmt.Errorf("open vault root: %w", err)
	}
	return &Reader{name: absolute, root: opened, token: &readerToken{}}, nil
}

// Name returns the clean, resolved path selected when the Reader was opened.
func (r *Reader) Name() string {
	if r == nil {
		return ""
	}
	return r.name
}

// SameRoot reports whether other refers to the directory pinned by r. Both
// identities come from the already-open directory objects; the pathname that
// originally selected either object is not consulted.
func (r *Reader) SameRoot(other *os.Root) (bool, error) {
	if r == nil || r.root == nil {
		return false, fmt.Errorf("compare vault root: reader is closed: %w", fs.ErrClosed)
	}
	if other == nil {
		return false, errors.New("compare vault root: other root is nil")
	}
	selected, err := r.root.Stat(".")
	if err != nil {
		return false, fmt.Errorf("compare vault root: stat reader root: %w", err)
	}
	candidate, err := other.Stat(".")
	if err != nil {
		return false, fmt.Errorf("compare vault root: stat other root: %w", err)
	}
	return os.SameFile(selected, candidate), nil
}

// Close releases the pinned vault directory.
func (r *Reader) Close() error {
	if r == nil || r.root == nil {
		return nil
	}
	return r.root.Close()
}

// ScanAvailable observes every usable regular, non-dot entry. Nested paths
// that cannot be observed are recorded as problems while usable neighbors
// remain available. Root errors, cancellation, and canonical collisions fail
// the scan without returning a partial value.
func (r *Reader) ScanAvailable(ctx context.Context) (Scan, error) {
	return r.scan(ctx, scanAvailable)
}

// ScanComplete observes every regular, non-dot entry. Any observation error
// fails the scan without returning a partial value.
func (r *Reader) ScanComplete(ctx context.Context) (Scan, error) {
	return r.scan(ctx, scanComplete)
}

func (r *Reader) scan(ctx context.Context, completeness scanCompleteness) (Scan, error) {
	if r == nil || r.root == nil {
		return Scan{}, fmt.Errorf("vault reader is closed: %w", fs.ErrClosed)
	}
	if err := ctx.Err(); err != nil {
		return Scan{}, err
	}
	rootInfo, err := r.root.Stat(".")
	if err != nil {
		return Scan{}, fmt.Errorf("list pinned vault: %w", err)
	}
	walk := sourceWalk{
		token:        r.token,
		completeness: completeness,
		seen:         make(map[string]string),
		directories:  make(map[string]fs.FileInfo),
		contains:     map[string]struct{}{".": {}},
	}
	err = fs.WalkDir(r.root.FS(), ".", func(raw string, d fs.DirEntry, walkErr error) error {
		return walk.visit(ctx, raw, d, walkErr)
	})
	if err != nil {
		return Scan{}, fmt.Errorf("list pinned vault: %w", err)
	}
	slices.SortFunc(walk.entries, func(a, b Entry) int {
		return strings.Compare(a.path, b.path)
	})
	slices.SortFunc(walk.problems, func(a, b Problem) int {
		if byPath := strings.Compare(a.path, b.path); byPath != 0 {
			return byPath
		}
		return strings.Compare(a.err.Error(), b.err.Error())
	})
	entries := make(map[string]Entry, len(walk.entries))
	for _, entry := range walk.entries {
		entries[entry.path] = entry
	}
	return Scan{state: &scanState{
		valid:    true,
		rootName: r.name,
		rootInfo: rootInfo,
		files:    walk.entries,
		entries:  entries,
		contains: walk.contains,
		problems: walk.problems,
	}}, nil
}

// Entries returns every regular, non-dot entry in canonical path order.
func (r *Reader) Entries(ctx context.Context) ([]Entry, error) {
	scan, err := r.ScanComplete(ctx)
	if err != nil {
		return nil, err
	}
	return scan.Files(), nil
}

type scanCompleteness uint8

const (
	scanComplete scanCompleteness = iota
	scanAvailable
)

type sourceWalk struct {
	token        *readerToken
	completeness scanCompleteness
	seen         map[string]string
	directories  map[string]fs.FileInfo
	contains     map[string]struct{}
	entries      []Entry
	problems     []Problem
}

func (w *sourceWalk) visit(ctx context.Context, raw string, d fs.DirEntry, walkErr error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if walkErr != nil {
		return w.problem(raw, d, walkErr)
	}
	if raw == "." {
		return nil
	}
	if strings.HasPrefix(d.Name(), ".") {
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	canonical := NormalizeNFC(filepath.ToSlash(raw))
	if err := recordCanonicalPath(w.seen, raw, canonical); err != nil {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return w.problem(raw, d, err)
	}
	if d.IsDir() {
		w.directories[raw] = info
		w.contains[canonical] = struct{}{}
		return nil
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	observed, err := observedSource(raw, w.directories, info)
	if err != nil {
		return w.problem(raw, d, err)
	}
	w.entries = append(w.entries, Entry{token: w.token, rawPath: raw, path: canonical, observed: observed})
	w.contains[canonical] = struct{}{}
	return nil
}

func (w *sourceWalk) problem(raw string, d fs.DirEntry, err error) error {
	if raw == "." || w.completeness == scanComplete {
		return err
	}
	canonical := NormalizeNFC(filepath.ToSlash(raw))
	if collisionErr := recordCanonicalPath(w.seen, raw, canonical); collisionErr != nil {
		return collisionErr
	}
	w.problems = append(w.problems, Problem{path: canonical, err: err})
	if d != nil && d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

func observedSource(rawPath string, directories map[string]fs.FileInfo, leaf fs.FileInfo) ([]sourceObservation, error) {
	components := strings.Split(rawPath, "/")
	observed := make([]sourceObservation, 0, len(components))
	for i := range components {
		name := path.Join(components[:i+1]...)
		info := leaf
		if i < len(components)-1 {
			info = directories[name]
		}
		if info == nil {
			return nil, ErrSourceChanged
		}
		observed = append(observed, sourceObservation{rawPath: name, info: info})
	}
	return observed, nil
}

func recordCanonicalPath(seen map[string]string, raw, canonical string) error {
	if previous, ok := seen[canonical]; ok && previous != raw {
		return ErrCanonicalCollision
	}
	seen[canonical] = raw
	return nil
}

// Lookup resolves one canonical vault-relative path without reading its bytes.
func (r *Reader) Lookup(relPath string) (Entry, error) {
	if r == nil || r.root == nil || relPath == "." || !fs.ValidPath(relPath) || relPath != NormalizeNFC(relPath) {
		return Entry{}, errors.New("invalid vault entry path")
	}
	return r.observe(relPath)
}

// Refresh selects the current regular file at e's original filesystem
// spelling under this Reader's pinned root. It lets a live reading surface see
// an atomic local edit without reopening the configured root path. A symlink,
// non-regular component, canonical-identity change, or Entry from another
// Reader fails closed.
func (r *Reader) Refresh(e Entry) (Entry, error) {
	if !r.owns(e) {
		return Entry{}, errors.New("entry does not belong to vault reader")
	}
	refreshed, err := r.observe(e.rawPath)
	if err != nil {
		return Entry{}, err
	}
	if refreshed.path != e.path {
		return Entry{}, ErrSourceChanged
	}
	return refreshed, nil
}

func (r *Reader) observe(relPath string) (entry Entry, resultErr error) {
	components := strings.Split(relPath, "/")
	current := r.root
	openedRoots := make([]*os.Root, 0, len(components)-1)
	defer func() {
		if closeErr := closeRoots(openedRoots); closeErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
			entry = Entry{}
		}
	}()

	observed := make([]sourceObservation, 0, len(components))
	for i, name := range components[:len(components)-1] {
		before, err := current.Lstat(name)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return Entry{}, ErrSourceChanged
		}
		child, err := current.OpenRoot(name)
		if err != nil {
			return Entry{}, ErrSourceChanged
		}
		opened, openErr := child.Stat(".")
		after, afterErr := current.Lstat(name)
		if openErr != nil || afterErr != nil || !opened.IsDir() || !sameObject(before, opened) ||
			!sameObject(before, after) {
			return Entry{}, closeChangedRoot(child)
		}
		openedRoots = append(openedRoots, child)
		current = child
		observed = append(observed, sourceObservation{
			rawPath: path.Join(components[:i+1]...),
			info:    before,
		})
	}

	leafName := components[len(components)-1]
	leaf, err := current.Lstat(leafName)
	if err != nil || !leaf.Mode().IsRegular() || leaf.Mode()&os.ModeSymlink != 0 {
		return Entry{}, ErrSourceChanged
	}
	observed = append(observed, sourceObservation{rawPath: relPath, info: leaf})
	return Entry{
		token:    r.token,
		rawPath:  relPath,
		path:     NormalizeNFC(filepath.ToSlash(relPath)),
		observed: observed,
	}, nil
}

type readStage uint8

const (
	readAfterParentCheck readStage = iota + 1
	readAfterLeafCheck
	readAfterLeafOpen
	readAfterLeafRead
)

type readHook func(readStage, string) error

// ReadFile reads e through this Reader.
func (r *Reader) ReadFile(ctx context.Context, e Entry) ([]byte, error) {
	return r.readFile(ctx, e, nil)
}

// readFile accepts a checkpoint callback so an identity change can be staged
// between filesystem operations. Production reads pass nil.
func (r *Reader) readFile(ctx context.Context, e Entry, hook readHook) (data []byte, resultErr error) {
	return r.readEntry(ctx, e, readExtent{}, hook)
}

// ReadPrefix reads at most n bytes from the beginning of e through this
// Reader. A negative n is invalid.
func (r *Reader) ReadPrefix(ctx context.Context, e Entry, n int64) ([]byte, error) {
	return r.readPrefix(ctx, e, n, nil)
}

func (r *Reader) readPrefix(ctx context.Context, e Entry, n int64, hook readHook) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("read vault entry prefix: %w", fs.ErrInvalid)
	}
	return r.readEntry(ctx, e, readExtent{prefix: true, limit: n}, hook)
}

// OpenFile opens e through this Reader and returns a stable handle to the
// selected regular file. Parent and leaf identity are established before the
// handle is returned; later path replacement does not retarget the handle.
func (r *Reader) OpenFile(ctx context.Context, e Entry) (*File, error) {
	return r.openFile(ctx, e, nil)
}

func (r *Reader) openFile(ctx context.Context, e Entry, hook readHook) (*File, error) {
	if !r.owns(e) {
		return nil, errors.New("entry does not belong to vault reader")
	}
	if _, err := r.root.Stat("."); err != nil {
		return nil, fmt.Errorf("inspect vault root: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	checkedHook := readHookWithContext(ctx, hook)
	parent, openedRoots, err := r.openParent(e, checkedHook)
	if err != nil {
		return nil, errors.Join(err, closeRoots(openedRoots))
	}
	leaf, err := openLeaf(parent, e, checkedHook)
	if err != nil {
		return nil, errors.Join(err, closeRoots(openedRoots))
	}
	if closeErr := closeRoots(openedRoots); closeErr != nil {
		fileCloseErr := leaf.file.Close()
		if fileCloseErr != nil {
			fileCloseErr = fmt.Errorf("close vault entry: %w", fileCloseErr)
		}
		return nil, errors.Join(closeErr, fileCloseErr)
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, leaf.file.Close())
	}
	return &File{file: leaf.file}, nil
}

type readExtent struct {
	prefix bool
	limit  int64
}

func (r *Reader) readEntry(
	ctx context.Context,
	e Entry,
	extent readExtent,
	hook readHook,
) (data []byte, resultErr error) {
	if !r.owns(e) {
		return nil, errors.New("entry does not belong to vault reader")
	}
	if _, err := r.root.Stat("."); err != nil {
		return nil, fmt.Errorf("inspect vault root: %w", err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}

	parent, openedRoots, err := r.openParent(e, hook)
	defer func() {
		if closeErr := closeRoots(openedRoots); closeErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
			data = nil
		}
	}()
	if err != nil {
		return nil, err
	}
	leaf, err := openLeaf(parent, e, hook)
	if err != nil {
		return nil, err
	}
	return leaf.read(ctx, extent, hook)
}

func (r *Reader) owns(e Entry) bool {
	if r == nil || r.root == nil || e.token != r.token || e.rawPath == "" ||
		!fs.ValidPath(e.rawPath) || e.path != NormalizeNFC(filepath.ToSlash(e.rawPath)) {
		return false
	}
	components := strings.Split(e.rawPath, "/")
	if len(e.observed) != len(components) {
		return false
	}
	for i, observation := range e.observed {
		if observation.info == nil || observation.rawPath != path.Join(components[:i+1]...) {
			return false
		}
	}
	return true
}

func (r *Reader) openParent(entry Entry, hook readHook) (parent *os.Root, openedRoots []*os.Root, resultErr error) {
	components := strings.Split(entry.rawPath, "/")
	current := r.root
	openedRoots = make([]*os.Root, 0, len(components)-1)

	for i, name := range components[:len(components)-1] {
		before, err := current.Lstat(name)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 ||
			!sameObject(entry.observed[i].info, before) {
			return nil, openedRoots, ErrSourceChanged
		}
		if hookErr := callReadHook(hook, readAfterParentCheck, name); hookErr != nil {
			return nil, openedRoots, hookErr
		}
		child, err := current.OpenRoot(name)
		if err != nil {
			return nil, openedRoots, ErrSourceChanged
		}
		opened, openErr := child.Stat(".")
		after, afterErr := current.Lstat(name)
		if openErr != nil || afterErr != nil || !opened.IsDir() || !after.IsDir() ||
			after.Mode()&os.ModeSymlink != 0 || !sameObject(before, opened) ||
			!sameObject(before, after) {
			return nil, openedRoots, closeChangedRoot(child)
		}
		openedRoots = append(openedRoots, child)
		current = child
	}
	return current, openedRoots, nil
}

type openedLeaf struct {
	parent  *os.Root
	file    *os.File
	before  fs.FileInfo
	name    string
	relPath string
}

func openLeaf(parent *os.Root, entry Entry, hook readHook) (openedLeaf, error) {
	components := strings.Split(entry.rawPath, "/")
	name := components[len(components)-1]
	expected := entry.observed[len(entry.observed)-1].info
	before, err := parent.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 ||
		!sameObservation(expected, before) {
		return openedLeaf{}, ErrSourceChanged
	}
	if hookErr := callReadHook(hook, readAfterLeafCheck, entry.rawPath); hookErr != nil {
		return openedLeaf{}, hookErr
	}
	file, err := parent.Open(name)
	if err != nil {
		return openedLeaf{}, ErrSourceChanged
	}
	opened, openErr := file.Stat()
	afterOpen, afterOpenErr := parent.Lstat(name)
	if openErr != nil || afterOpenErr != nil || !opened.Mode().IsRegular() ||
		afterOpen.Mode()&os.ModeSymlink != 0 || !sameObservation(before, opened) ||
		!sameObservation(before, afterOpen) {
		return openedLeaf{}, closeChangedFile(file)
	}
	if hookErr := callReadHook(hook, readAfterLeafOpen, entry.rawPath); hookErr != nil {
		if closeErr := file.Close(); closeErr != nil {
			return openedLeaf{}, errors.Join(hookErr, fmt.Errorf("close vault entry: %w", closeErr))
		}
		return openedLeaf{}, hookErr
	}
	return openedLeaf{parent: parent, file: file, before: before, name: name, relPath: entry.rawPath}, nil
}

func (leaf openedLeaf) read(
	ctx context.Context,
	extent readExtent,
	hook readHook,
) (data []byte, resultErr error) {
	defer func() {
		if closeErr := leaf.file.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close vault entry: %w", closeErr))
			data = nil
		}
	}()
	var source io.Reader = leaf.file
	if extent.prefix {
		source = io.LimitReader(leaf.file, extent.limit)
	}
	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("read vault entry: %w", err)
	}
	if hookErr := callReadHook(hook, readAfterLeafRead, leaf.relPath); hookErr != nil {
		return nil, hookErr
	}
	afterRead, readErr := leaf.parent.Lstat(leaf.name)
	openedAfterRead, openedReadErr := leaf.file.Stat()
	if readErr != nil || openedReadErr != nil || afterRead.Mode()&os.ModeSymlink != 0 ||
		!sameObservation(leaf.before, afterRead) || !sameObservation(leaf.before, openedAfterRead) {
		return nil, ErrSourceChanged
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	return data, nil
}

func sameObservation(want, got fs.FileInfo) bool {
	return sameObject(want, got) &&
		want.Size() == got.Size() && want.ModTime().Equal(got.ModTime())
}

func sameObject(want, got fs.FileInfo) bool {
	return want != nil && got != nil && os.SameFile(want, got) && want.Mode() == got.Mode()
}

func closeChangedRoot(root *os.Root) error {
	if closeErr := root.Close(); closeErr != nil {
		return errors.Join(ErrSourceChanged, fmt.Errorf("close changed vault directory: %w", closeErr))
	}
	return ErrSourceChanged
}

func closeChangedFile(file *os.File) error {
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(ErrSourceChanged, fmt.Errorf("close changed vault entry: %w", closeErr))
	}
	return ErrSourceChanged
}

func closeRoots(roots []*os.Root) error {
	var result error
	for _, root := range slices.Backward(roots) {
		if closeErr := root.Close(); closeErr != nil {
			result = errors.Join(result, fmt.Errorf("close vault directory: %w", closeErr))
		}
	}
	return result
}

func callReadHook(hook readHook, stage readStage, relPath string) error {
	if hook == nil {
		return nil
	}
	return hook(stage, relPath)
}

func readHookWithContext(ctx context.Context, hook readHook) readHook {
	return func(stage readStage, relPath string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return callReadHook(hook, stage, relPath)
	}
}
