// Package vaultfs pins one vault directory as a read capability and answers
// what is under it, what each file was when observed, and what its bytes are
// now. It never writes. Every read descends the recorded path component by
// component and refuses the moment an object stops being the one observed, so
// a rename under a reader's feet costs the read rather than yielding bytes.
package vaultfs

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

	"github.com/koopa0/yomihon/internal/vault"
)

// ErrSourceChanged means the filesystem object a vault entry selected is no
// longer the object at that path. It is reported only where an earlier
// observation exists to compare against, so it always describes a change
// someone made, never a path that was never there.
var ErrSourceChanged = errors.New("vault entry no longer names the observed file")

// ErrNotRegular means a path exists but holds something other than readable
// bytes where readable bytes are required.
var ErrNotRegular = errors.New("vault path is not a regular file")

// ErrNotDirectory means a component in the middle of a path exists but is not a
// directory, so the walk cannot descend through it. It is distinct from
// [ErrNotRegular], which a plain file standing there would be misdescribed by.
var ErrNotDirectory = errors.New("vault path component is not a directory")

// ErrSymbolicLink means a path is a symbolic link. The pinned root never
// follows one, because a link is the one name that can point at bytes the
// reader never chose to expose. This is a refusal, not an accident, and it is
// named separately so a log can show the refusal happening.
var ErrSymbolicLink = errors.New("vault path is a symbolic link")

// errUnobservedParent means a walk reached a nested file whose parent directory
// it never observed, leaving the file's containment chain unestablished. It
// stays unexported because no decision outside this package turns on it.
var errUnobservedParent = errors.New("vault parent directory was not observed in this scan")

// ErrCanonicalCollision means two filesystem names normalize to the same
// vault-relative NFC path. Such a tree cannot be projected without guessing.
var ErrCanonicalCollision = errors.New("vault contains canonically colliding paths")

// readerToken is the identity one Reader hands to every Entry it produces, so
// owns can tell its own Entry from another Reader's by comparing the two
// pointers. The single byte is what makes that comparison mean anything: a
// zero-size value gives the allocator nothing to distinguish: every one of them
// escaping to the heap is handed the same address, so two Readers would carry
// tokens that compare equal every time, and owns would accept any Reader's
// Entry rather than sometimes. Narrowing this to struct{} compiles and saves
// nothing; what it costs is Refresh's promise that an Entry from another Reader
// fails closed, and two tests in this package say so out loud.
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
// keeps referring to the selected object if the path is renamed or atomically
// replaced, and like os.File it does not freeze bytes another descriptor
// writes. It wraps the *os.File rather than returning it so a caller reaches
// Read, Seek and Close and nothing else: a write to the vault is then a
// compile error rather than a slip.
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

// Skipped records one path the scan saw plainly and did not index, with the
// reason it is not one of the vault's files. It is not a Problem: a problem is
// a path the scan could not observe at all, and a complete scan fails on one,
// whereas a skipped path was read without trouble and is simply not something
// this vault can hold a note in. A vault that organises by symbolic link loses
// notes here, so the skip is recorded rather than passed over in silence.
type Skipped struct {
	path string
	kind SkipKind
}

// SkipKind is why a path the scan saw is not one of the vault's files. It is a
// closed set, and this package owns it: the words below reach an external
// consumer through the judging commands, where they are part of a finding's
// frozen bytes, so a second spelling anywhere else would change what that
// consumer reads.
//
// A symbolic link has its own member because it is the one an author makes on
// purpose and expects to be read. The rest share a member: a socket and a
// device file are the same news to a reader and take the same repair.
type SkipKind uint8

const (
	// SkipSymlink is a symbolic link, whose target this reader never follows.
	SkipSymlink SkipKind = iota + 1
	// SkipNotRegular is anything else a note cannot be read out of: a socket,
	// a device file, a named pipe.
	SkipNotRegular
	// skipKindEnd is one past the last member, so a walk over the set has a
	// bound that does not depend on String() answering. A terminator that
	// asked String() would stop at exactly the member somebody forgot to give
	// a phrase to, which is the member a walk exists to catch.
	skipKindEnd
)

// SkipKinds returns every member of the set, in declared order. A consumer
// that must cover all of them asks for the set rather than keeping a list
// beside it, the way the study-path grammar hands out its rules: a list written
// by hand agrees with itself and with nothing else.
func SkipKinds() []SkipKind {
	kinds := make([]SkipKind, 0, int(skipKindEnd)-int(SkipSymlink))
	for kind := SkipSymlink; kind < skipKindEnd; kind++ {
		kinds = append(kinds, kind)
	}
	return kinds
}

// String is the phrase the scan reports this kind by. It is part of what the
// judging commands emit, so these two strings are frozen.
func (k SkipKind) String() string {
	switch k {
	case SkipSymlink:
		return "symbolic link"
	case SkipNotRegular:
		return "not a regular file"
	case skipKindEnd:
		// Not a member of the set: the bound a walk over it stops at, named
		// here so the compiler's own exhaustiveness answer covers the whole
		// type rather than the part a reader remembered.
	}
	return "unknown"
}

// Path returns the canonical vault-relative path that was not indexed.
func (s Skipped) Path() string { return s.path }

// Kind returns why the path is not one of the vault's files.
func (s Skipped) Kind() SkipKind { return s.kind }

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
	skipped  []Skipped
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

// Problems returns the nested paths an available scan could not observe,
// sorted by path and then by the observation error's text, so two scans of
// the same trouble report it in the same order.
func (s Scan) Problems() []Problem {
	if s.state == nil {
		return nil
	}
	return slices.Clone(s.state.problems)
}

// Skipped returns the paths the scan observed and did not index, sorted by
// path and then by kind, so two scans of the same folder report them in the
// same order. Both scan kinds record these: a skipped path is a fact about the
// folder, not a failure of the reading.
func (s Scan) Skipped() []Skipped {
	if s.state == nil {
		return nil
	}
	return slices.Clone(s.state.skipped)
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
	return fs.ValidPath(relPath) && relPath == vault.NormalizeNFC(relPath)
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
	slices.SortFunc(walk.skipped, func(a, b Skipped) int {
		if byPath := strings.Compare(a.path, b.path); byPath != 0 {
			return byPath
		}
		return int(a.kind) - int(b.kind)
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
		skipped:  walk.skipped,
	}}, nil
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
	skipped      []Skipped
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
	if hiddenName(d.Name()) {
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	canonical := vault.NormalizeNFC(filepath.ToSlash(raw))
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
		w.skipped = append(w.skipped, Skipped{path: canonical, kind: skipKind(info.Mode())})
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

// skipKind says which member of the closed set a directory entry falls in.
func skipKind(mode fs.FileMode) SkipKind {
	if mode&os.ModeSymlink != 0 {
		return SkipSymlink
	}
	return SkipNotRegular
}

func (w *sourceWalk) problem(raw string, d fs.DirEntry, err error) error {
	if raw == "." || w.completeness == scanComplete {
		return err
	}
	canonical := vault.NormalizeNFC(filepath.ToSlash(raw))
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
			return nil, errUnobservedParent
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
	if r == nil || r.root == nil || relPath == "." || !fs.ValidPath(relPath) || relPath != vault.NormalizeNFC(relPath) {
		return Entry{}, errors.New("invalid vault entry path")
	}
	return r.observe(relPath)
}

// Refresh selects the current regular file at e's original filesystem spelling
// under this Reader's pinned root, so a live surface sees an atomic local edit
// without reopening the root. A symlink, a non-regular component, a changed
// canonical identity, or an Entry from another Reader fails closed.
func (r *Reader) Refresh(e Entry) (Entry, error) {
	if !r.owns(e) {
		return Entry{}, errors.New("entry does not belong to vault reader")
	}
	return r.observe(e.rawPath)
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
		observedName := path.Join(components[:i+1]...)
		before, err := current.Lstat(name)
		if err != nil {
			// An absent component means the caller's whole path is absent, so
			// the error names what the caller asked for. A refusal below is the
			// opposite case: one specific thing is in the way and naming which
			// component it is, is the actionable half.
			return Entry{}, pathErrorAt(relPath, err)
		}
		if !before.IsDir() {
			return Entry{}, refuse("lookup", observedName, before.Mode(), true)
		}
		// The Lstat above is this walk's own observation of the component, so a
		// failure to open what it just saw is a change under the walk, not a
		// path that was never there.
		child, err := current.OpenRoot(name)
		if err != nil {
			return Entry{}, revalidated(pathErrorAt(relPath, err))
		}
		opened, openErr := child.Stat(".")
		after, afterErr := current.Lstat(name)
		if openErr != nil || afterErr != nil || !opened.IsDir() || !sameObject(before, opened) ||
			!sameObject(before, after) {
			return Entry{}, closeChangedRoot(child)
		}
		openedRoots = append(openedRoots, child)
		current = child
		observed = append(observed, sourceObservation{rawPath: observedName, info: before})
	}

	leafName := components[len(components)-1]
	leaf, err := current.Lstat(leafName)
	if err != nil {
		return Entry{}, pathErrorAt(relPath, err)
	}
	if !leaf.Mode().IsRegular() {
		return Entry{}, refuse("lookup", relPath, leaf.Mode(), false)
	}
	observed = append(observed, sourceObservation{rawPath: relPath, info: leaf})
	return Entry{
		token:    r.token,
		rawPath:  relPath,
		path:     vault.NormalizeNFC(filepath.ToSlash(relPath)),
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
		!fs.ValidPath(e.rawPath) || e.path != vault.NormalizeNFC(filepath.ToSlash(e.rawPath)) {
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
		if err != nil {
			return nil, openedRoots, revalidated(err)
		}
		// sameObject compares the mode, so a symlink or a file swapped in for the
		// observed directory fails identity before any type test could run.
		if !before.IsDir() || !sameObject(entry.observed[i].info, before) {
			return nil, openedRoots, ErrSourceChanged
		}
		if hookErr := callReadHook(hook, readAfterParentCheck, name); hookErr != nil {
			return nil, openedRoots, hookErr
		}
		child, err := current.OpenRoot(name)
		if err != nil {
			return nil, openedRoots, revalidated(err)
		}
		opened, openErr := child.Stat(".")
		after, afterErr := current.Lstat(name)
		if openErr != nil || afterErr != nil || !opened.IsDir() || !sameObject(before, opened) ||
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
	if err != nil {
		return openedLeaf{}, revalidated(err)
	}
	if !before.Mode().IsRegular() || !sameObservation(expected, before) {
		return openedLeaf{}, ErrSourceChanged
	}
	if hookErr := callReadHook(hook, readAfterLeafCheck, entry.rawPath); hookErr != nil {
		return openedLeaf{}, hookErr
	}
	file, err := parent.Open(name)
	if err != nil {
		return openedLeaf{}, revalidated(err)
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

// refuse names why a present path cannot be used at the position it occupies.
// The symbolic-link test comes first because a link is neither a regular file
// nor a directory, and wantDir separates the two positions so a plain file
// standing where a directory belongs is not called "not a regular file".
func refuse(op, relPath string, mode fs.FileMode, wantDir bool) error {
	switch {
	case mode&fs.ModeSymlink != 0:
		return &fs.PathError{Op: op, Path: relPath, Err: ErrSymbolicLink}
	case wantDir:
		return &fs.PathError{Op: op, Path: relPath, Err: ErrNotDirectory}
	default:
		return &fs.PathError{Op: op, Path: relPath, Err: ErrNotRegular}
	}
}

// revalidated classifies a filesystem error met while re-checking an object
// this reader already observed. Only absence proves the object is gone; a
// refusal or an exhausted descriptor table means the machine could not answer,
// so it is reported as itself rather than as a change.
func revalidated(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return ErrSourceChanged
	}
	return err
}

// pathErrorAt restates a filesystem error against the vault-relative path the
// caller named. The walk descends one component at a time, so the raw error
// names the bare component it stood on — "System" for a caller who asked for
// "System/schemas/vault-schema.toml", which reads as a different file missing.
func pathErrorAt(relPath string, err error) error {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return &fs.PathError{Op: pathErr.Op, Path: relPath, Err: pathErr.Err}
	}
	return err
}

// hiddenName reports whether one directory-entry name is hidden from the scan.
func hiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}

// OutsideScan reports whether relPath lies beyond what a scan ever looks at, so
// a caller can tell "I looked and it was not there" apart from "I never looked".
// A judgement about such a path cannot rest on the scan, which holds no
// evidence either way.
func OutsideScan(relPath string) bool {
	for segment := range strings.SplitSeq(relPath, "/") {
		if hiddenName(segment) {
			return true
		}
	}
	return false
}
