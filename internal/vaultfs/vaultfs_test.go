package vaultfs

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestProjectionPackagesHaveNoLegacyEntryPoints(t *testing.T) {
	t.Parallel()

	forbiddenByDir := map[string]map[string]bool{
		".": {
			"List":              true,
			"ListStrict":        true,
			"ListStrictContext": true,
			"ReadNote":          true,
		},
		"../graph": {
			"Build": true,
		},
		"../lesson": {
			"BuildConceptIndex": true,
			"BuildSlotIndex":    true,
			"LoadConcept":       true,
			"readSidecar":       true,
		},
		"../nav": {
			"Build": true,
		},
		"../search": {
			"Build": true,
		},
	}
	var found []string
	for dir, forbidden := range forbiddenByDir {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir(%q) error = %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				t.Fatalf("ParseFile(%q) error = %v", path, parseErr)
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Recv == nil && forbidden[function.Name.Name] {
					found = append(found, filepath.ToSlash(path)+":"+function.Name.Name)
				}
			}
		}
	}
	slices.Sort(found)
	if len(found) != 0 {
		t.Errorf("legacy root entrypoint declarations = %v, want none", found)
	}
}

func TestReaderScanCompleteIndexesObservedFilesAndDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rawPath := "Notes/Cafe\u0301.md"
	writeReaderFixture(t, root, rawPath, "cafe")
	modTime := time.Unix(1_700_000_000, 123_000_000)
	if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(rawPath)), modTime, modTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	reader := openTestReader(t, root)
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	files := scan.Files()
	if len(files) != 1 {
		t.Fatalf("ScanComplete().Files() len = %d, want 1", len(files))
	}
	if got, want := files[0].Path(), "Notes/Caf\u00e9.md"; got != want {
		t.Errorf("ScanComplete().Files()[0].Path() = %q, want %q", got, want)
	}
	if got, want := files[0].Size(), int64(len("cafe")); got != want {
		t.Errorf("ScanComplete().Files()[0].Size() = %d, want %d", got, want)
	}
	if got := files[0].ModTime(); !got.Equal(modTime) {
		t.Errorf("ScanComplete().Files()[0].ModTime() = %v, want %v", got, modTime)
	}
	for _, relPath := range []string{".", "Notes", "Notes/Caf\u00e9.md"} {
		if !scan.Contains(relPath) {
			t.Errorf("ScanComplete().Contains(%q) = false, want true", relPath)
		}
	}
	for _, relPath := range []string{"Notes/Cafe\u0301.md", "Notes/missing.md"} {
		if scan.Contains(relPath) {
			t.Errorf("ScanComplete().Contains(%q) = true, want false", relPath)
		}
	}
	entry, ok := scan.Entry("Notes/Caf\u00e9.md")
	if !ok || entry.Path() != "Notes/Caf\u00e9.md" {
		t.Fatalf("ScanComplete().Entry(NFC) = (%q, %t), want canonical entry", entry.Path(), ok)
	}
	if _, ok := scan.Entry("Notes/Cafe\u0301.md"); ok {
		t.Fatal("ScanComplete().Entry(NFD) found an entry, want canonical-only lookup")
	}
	if got := len(scan.Problems()); got != 0 {
		t.Errorf("ScanComplete().Problems() len = %d, want 0", got)
	}
}

func TestReaderSameRootComparesOpenDirectoryIdentity(t *testing.T) {
	t.Parallel()

	selectedPath := filepath.Join(t.TempDir(), "vault")
	if err := os.Mkdir(selectedPath, 0o750); err != nil {
		t.Fatalf("mkdir selected root: %v", err)
	}
	reader, err := Open(selectedPath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", selectedPath, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})

	same, err := os.OpenRoot(selectedPath)
	if err != nil {
		t.Fatalf("open same root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := same.Close(); closeErr != nil {
			t.Errorf("close same root: %v", closeErr)
		}
	})
	if got, sameErr := reader.SameRoot(same); sameErr != nil || !got {
		t.Fatalf("SameRoot(same) = (%t, %v), want (true, nil)", got, sameErr)
	}

	movedPath := selectedPath + "-moved"
	if err = os.Rename(selectedPath, movedPath); err != nil {
		t.Fatalf("rename selected root: %v", err)
	}
	if err = os.Mkdir(selectedPath, 0o750); err != nil {
		t.Fatalf("create replacement root: %v", err)
	}
	replacement, err := os.OpenRoot(selectedPath)
	if err != nil {
		t.Fatalf("open replacement root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := replacement.Close(); closeErr != nil {
			t.Errorf("close replacement root: %v", closeErr)
		}
	})
	if got, sameErr := reader.SameRoot(replacement); sameErr != nil || got {
		t.Errorf("SameRoot(replacement) = (%t, %v), want (false, nil)", got, sameErr)
	}
	if got, sameErr := reader.SameRoot(same); sameErr != nil || !got {
		t.Errorf("SameRoot(open root after rename) = (%t, %v), want (true, nil)", got, sameErr)
	}
}

func TestReaderScanAvailableKeepsUsableNeighborsAfterNestedError(t *testing.T) {
	t.Parallel()
	nestedErr := errors.New("nested entry unavailable")
	walk := newTestSourceWalk(scanAvailable)

	err := walk.visit(t.Context(), "Broken", testDirEntry{
		name:    "Broken",
		dir:     true,
		infoErr: nestedErr,
	}, nil)
	if !errors.Is(err, fs.SkipDir) {
		t.Fatalf("available visit(nested error) = %v, want fs.SkipDir", err)
	}
	if got := walk.problems; len(got) != 1 || got[0].Path() != "Broken" || !errors.Is(got[0].Err(), nestedErr) {
		t.Fatalf("available problems = %#v, want Broken nested error", got)
	}

	root := t.TempDir()
	writeReaderFixture(t, root, "good.md", "good")
	info, statErr := os.Stat(filepath.Join(root, "good.md"))
	if statErr != nil {
		t.Fatalf("Stat(good.md) error = %v", statErr)
	}
	if err := walk.visit(t.Context(), "good.md", testDirEntry{name: "good.md", info: info}, nil); err != nil {
		t.Fatalf("available visit(good.md) error = %v", err)
	}
	if len(walk.entries) != 1 || walk.entries[0].Path() != "good.md" {
		t.Fatalf("available entries = %#v, want good.md", walk.entries)
	}

	walkErr := errors.New("nested walk failed")
	if err := walk.visit(t.Context(), "Lost", nil, walkErr); err != nil {
		t.Fatalf("available visit(walk error) error = %v", err)
	}
	if got := walk.problems; len(got) != 2 || got[1].Path() != "Lost" || !errors.Is(got[1].Err(), walkErr) {
		t.Fatalf("available walk problems = %#v, want Lost walk error", got)
	}
}

func TestReaderScanCompleteRejectsNestedError(t *testing.T) {
	t.Parallel()
	nestedErr := errors.New("nested entry unavailable")
	walk := newTestSourceWalk(scanComplete)

	err := walk.visit(t.Context(), "Broken", testDirEntry{
		name:    "Broken",
		dir:     true,
		infoErr: nestedErr,
	}, nil)
	if !errors.Is(err, nestedErr) {
		t.Fatalf("complete visit(nested error) = %v, want nested error", err)
	}
	if got := len(walk.problems); got != 0 {
		t.Errorf("complete problems len = %d, want 0", got)
	}
}

func TestReaderScanAvailableRecordsMissingParentObservation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "entry", "one")
	info, err := os.Stat(filepath.Join(root, "entry"))
	if err != nil {
		t.Fatalf("Stat(entry) error = %v", err)
	}
	walk := newTestSourceWalk(scanAvailable)

	err = walk.visit(t.Context(), "Missing/one.md", testDirEntry{name: "one.md", info: info}, nil)
	if err != nil {
		t.Fatalf("available visit(missing parent observation) error = %v", err)
	}
	// A parent the walk never observed is an unestablished containment chain,
	// not an object that changed under the reader — nothing was observed to
	// change from.
	if got := walk.problems; len(got) != 1 || got[0].Path() != "Missing/one.md" ||
		!errors.Is(got[0].Err(), errUnobservedParent) {
		t.Fatalf("available missing-parent problems = %#v, want errUnobservedParent", got)
	}
	if got := len(walk.entries); got != 0 {
		t.Errorf("available missing-parent entries len = %d, want 0", got)
	}
}

func TestReaderScanAvailableRejectsRootError(t *testing.T) {
	t.Parallel()
	rootErr := errors.New("root unavailable")
	walk := newTestSourceWalk(scanAvailable)

	err := walk.visit(t.Context(), ".", nil, rootErr)
	if !errors.Is(err, rootErr) {
		t.Fatalf("available visit(root error) = %v, want root error", err)
	}
	if got := len(walk.problems); got != 0 {
		t.Errorf("available root problems len = %d, want 0", got)
	}
}

func TestReaderScansRejectCancellation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "one")
	reader := openTestReader(t, root)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	tests := []struct {
		name string
		scan func(context.Context) (Scan, error)
	}{
		{name: "available", scan: reader.ScanAvailable},
		{name: "complete", scan: reader.ScanComplete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scan, err := tt.scan(ctx)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("scan(canceled) error = %v, want context.Canceled", err)
			}
			if len(scan.Files()) != 0 || len(scan.Problems()) != 0 {
				t.Fatalf("scan(canceled) = %#v, want unusable zero scan", scan)
			}
		})
	}
}

func TestSourceWalkRejectsCanonicalCollisionInEveryScan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "entry", "one")
	info, err := os.Stat(filepath.Join(root, "entry"))
	if err != nil {
		t.Fatalf("Stat(entry) error = %v", err)
	}

	for _, completeness := range []scanCompleteness{scanAvailable, scanComplete} {
		walk := newTestSourceWalk(completeness)
		if err := walk.visit(t.Context(), "Cafe\u0301.md", testDirEntry{name: "Cafe\u0301.md", info: info}, nil); err != nil {
			t.Fatalf("visit(first spelling) error = %v", err)
		}
		err := walk.visit(t.Context(), "Caf\u00e9.md", testDirEntry{name: "Caf\u00e9.md", info: info}, nil)
		if !errors.Is(err, ErrCanonicalCollision) {
			t.Errorf("visit(canonical collision, completeness=%d) error = %v, want ErrCanonicalCollision", completeness, err)
		}
	}
}

func TestReaderScansExcludeDotEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, ".hidden.md", "hidden")
	writeReaderFixture(t, root, ".hidden/secret.md", "secret")
	writeReaderFixture(t, root, "Notes/visible.md", "visible")
	reader := openTestReader(t, root)

	for _, scan := range []func(context.Context) (Scan, error){reader.ScanAvailable, reader.ScanComplete} {
		got, err := scan(t.Context())
		if err != nil {
			t.Fatalf("scan() error = %v", err)
		}
		files := got.Files()
		if len(files) != 1 || files[0].Path() != "Notes/visible.md" {
			t.Errorf("scan().Files() = %#v, want only visible.md", files)
		}
		for _, hidden := range []string{".hidden.md", ".hidden", ".hidden/secret.md"} {
			if got.Contains(hidden) {
				t.Errorf("scan().Contains(%q) = true, want false", hidden)
			}
		}
	}
}

// TestScanIndexesEveryRegularFileWhateverItsName pins the negative space of
// the walk: no extension filter exists. A regular, non-hidden file is in the
// scan whether it is markdown, an archive, or carries no extension at all —
// what a kind means is decided by the faces that read the scan, never by the
// scan itself.
func TestScanIndexesEveryRegularFileWhateverItsName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := []string{"Data/notes.tar.gz", "Makefile", "Notes/plain.md", "no-dot-at-all", "weird.XYZ"}
	for _, rel := range paths {
		writeReaderFixture(t, root, rel, "content")
	}

	scan := scanCompleteFixture(t, openTestReader(t, root))
	got := make([]string, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		got = append(got, entry.Path())
	}
	if !slices.Equal(got, paths) {
		t.Errorf("scan().Files() paths = %q, want every regular file: %q", got, paths)
	}
}

// TestScanReturnsEntriesInCanonicalPathByteOrder pins the scan's ordering
// contract: entries arrive sorted by the bytes of their canonical paths, not
// in the order the directory walk happened to visit them. A space sorts
// before the path separator, so a root file can rank between two directories'
// contents — an order no per-directory walk produces on its own.
func TestScanReturnsEntriesInCanonicalPathByteOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := []string{"B.md", "a c.md", "a/z.md", "b/x.md"}
	for _, rel := range want {
		writeReaderFixture(t, root, rel, "content")
	}

	scan := scanCompleteFixture(t, openTestReader(t, root))
	got := make([]string, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		got = append(got, entry.Path())
	}
	if !slices.Equal(got, want) {
		t.Errorf("scan().Files() paths = %q, want canonical byte order %q", got, want)
	}
}

// TestScanRecordsASymlinkAsSkippedNotAsAProblem pins the scan half of the
// symlink stance. A symlink inside the tree is never indexed and never
// followed — the read path refuses one by name (ErrSymbolicLink) — but the
// scan does say it saw one, because a vault that organises by link would
// otherwise lose notes with nothing anywhere reporting it.
//
// Which list it lands in is the load-bearing half. A problem is a path that
// could not be observed, and a complete scan fails on one, so recording a
// symlink there would turn every vault containing one into a vault the
// adjudication commands refuse to read at all.
func TestScanRecordsASymlinkAsSkippedNotAsAProblem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/real.md", "real")
	if err := os.Symlink(filepath.Join(root, "Notes", "real.md"), filepath.Join(root, "Notes", "link.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	reader := openTestReader(t, root)
	for _, scan := range []func(context.Context) (Scan, error){reader.ScanAvailable, reader.ScanComplete} {
		got, err := scan(t.Context())
		if err != nil {
			t.Fatalf("scan() error = %v", err)
		}
		files := got.Files()
		if len(files) != 1 || files[0].Path() != "Notes/real.md" {
			t.Errorf("scan().Files() = %#v, want only the real file", files)
		}
		if problems := got.Problems(); len(problems) != 0 {
			t.Errorf("scan().Problems() = %#v, want none — a symlink was observed, not missed", problems)
		}
		skipped := got.Skipped()
		if len(skipped) != 1 || skipped[0].Path() != "Notes/link.md" {
			t.Fatalf("scan().Skipped() = %#v, want the symlink named", skipped)
		}
		if skipped[0].Kind() != SkipSymlink {
			t.Errorf("scan().Skipped()[0].Kind() = %v, want %v", skipped[0].Kind(), SkipSymlink)
		}
		if got.Contains("Notes/link.md") {
			t.Error("scan().Contains(symlink) = true, want false")
		}
	}
}

// TestOutsideScanIsAnyDotLeadingSegment pins the boundary predicate to the
// same rule the walk applies: a path is beyond the scan exactly when any of
// its segments opens with a dot. Callers use it to tell "I looked and it was
// not there" apart from "I never looked", so it has to match the walk's
// exclusion precisely.
func TestOutsideScanIsAnyDotLeadingSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		relPath string
		want    bool
	}{
		{relPath: ".git/config", want: true},
		{relPath: "Notes/.trash/old.md", want: true},
		{relPath: "Notes/sub/.hidden.md", want: true},
		{relPath: ".", want: true},
		{relPath: "Notes/visible.md", want: false},
		{relPath: "a.b/c.d.md", want: false},
		{relPath: "Notes", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			t.Parallel()
			if got := OutsideScan(tt.relPath); got != tt.want {
				t.Errorf("OutsideScan(%q) = %v, want %v", tt.relPath, got, tt.want)
			}
		})
	}
}

func TestScanReturnsDefensiveClones(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "one")
	scan, err := openTestReader(t, root).ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}

	files := scan.Files()
	files[0].observed[0].rawPath = "mutated"
	files[0] = Entry{}
	gotFiles := scan.Files()
	if len(gotFiles) != 1 || gotFiles[0].Path() != "Notes/one.md" || gotFiles[0].observed[0].rawPath != "Notes" {
		t.Fatalf("Files() after caller mutation = %#v, want unchanged one.md entry", gotFiles)
	}
	entry, ok := scan.Entry("Notes/one.md")
	if !ok {
		t.Fatal("Entry(Notes/one.md) = false, want true")
	}
	entry.observed[0].rawPath = "mutated"
	gotEntry, ok := scan.Entry("Notes/one.md")
	if !ok || gotEntry.observed[0].rawPath != "Notes" {
		t.Fatalf("Entry() after caller mutation = (%#v, %t), want unchanged entry", gotEntry, ok)
	}

	firstErr := errors.New("first")
	secondErr := errors.New("second")
	problemScan := Scan{state: &scanState{problems: []Problem{{path: "bad", err: firstErr}}}}
	problems := problemScan.Problems()
	problems[0] = Problem{path: "changed", err: secondErr}
	gotProblems := problemScan.Problems()
	if len(gotProblems) != 1 || gotProblems[0].Path() != "bad" || !errors.Is(gotProblems[0].Err(), firstErr) {
		t.Fatalf("Problems() after caller mutation = %#v, want unchanged problem", gotProblems)
	}
}

func TestScanSameFilesDetectsObservationChanges(t *testing.T) {
	t.Parallel()

	t.Run("unchanged domain", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		second := scanCompleteFixture(t, reader)
		if !first.SameFiles(second) {
			t.Fatal("SameFiles(unchanged scan) = false, want true")
		}
	})

	t.Run("leaf metadata", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		path := filepath.Join(root, "Notes", "one.md")
		changed := first.Files()[0].ModTime().Add(2 * time.Second)
		if err := os.Chtimes(path, changed, changed); err != nil {
			t.Fatalf("Chtimes() error = %v", err)
		}
		second := scanCompleteFixture(t, reader)
		if first.SameFiles(second) {
			t.Fatal("SameFiles(changed metadata) = true, want false")
		}
	})

	t.Run("leaf size", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		file := first.Files()[0]
		path := filepath.Join(root, "Notes", "one.md")
		if err := os.WriteFile(path, []byte("different size"), 0o600); err != nil {
			t.Fatalf("WriteFile(one.md) error = %v", err)
		}
		if err := os.Chtimes(path, file.ModTime(), file.ModTime()); err != nil {
			t.Fatalf("Chtimes(one.md) error = %v", err)
		}
		second := scanCompleteFixture(t, reader)
		if first.SameFiles(second) {
			t.Fatal("SameFiles(changed size) = true, want false")
		}
	})

	// The comparison is deliberately identity-and-metadata only: bytes
	// rewritten in place with size and mtime restored are invisible to it.
	// That boundary is covered rather than accepted — the scanner's consumer
	// runs a slow reconciliation cycle that periodically rebuilds without
	// this comparison, so such an edit is still republished eventually.
	t.Run("leaf content, same size and mtime", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		file := first.Files()[0]
		path := filepath.Join(root, "Notes", "one.md")
		handle, err := os.OpenFile(path, os.O_WRONLY, 0) // #nosec G304 -- a path inside this test's own TempDir
		if err != nil {
			t.Fatalf("OpenFile(one.md) error = %v", err)
		}
		if _, err := handle.WriteAt([]byte("swap"), 0); err != nil {
			t.Fatalf("WriteAt(one.md) error = %v", err)
		}
		if err := handle.Close(); err != nil {
			t.Fatalf("Close(one.md) error = %v", err)
		}
		if err := os.Chtimes(path, file.ModTime(), file.ModTime()); err != nil {
			t.Fatalf("Chtimes(one.md) error = %v", err)
		}
		second := scanCompleteFixture(t, reader)
		if !first.SameFiles(second) {
			t.Fatal("SameFiles(in-place same-size same-mtime edit) = false, want true: the comparison is metadata-only")
		}
	})

	t.Run("leaf mode", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		if err := os.Chmod(filepath.Join(root, "Notes", "one.md"), 0o400); err != nil {
			t.Fatalf("Chmod(one.md) error = %v", err)
		}
		second := scanCompleteFixture(t, reader)
		if first.SameFiles(second) {
			t.Fatal("SameFiles(changed mode) = true, want false")
		}
	})

	t.Run("leaf object", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		file := first.Files()[0]
		replacement := filepath.Join(root, "replacement.md")
		if err := os.WriteFile(replacement, []byte("same"), 0o600); err != nil {
			t.Fatalf("WriteFile(replacement) error = %v", err)
		}
		if err := os.Chtimes(replacement, file.ModTime(), file.ModTime()); err != nil {
			t.Fatalf("Chtimes(replacement) error = %v", err)
		}
		if err := os.Rename(replacement, filepath.Join(root, "Notes", "one.md")); err != nil {
			t.Fatalf("Rename(replacement) error = %v", err)
		}
		second := scanCompleteFixture(t, reader)
		if first.SameFiles(second) {
			t.Fatal("SameFiles(replaced leaf) = true, want false")
		}
	})

	t.Run("parent object", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		reader := openTestReader(t, root)
		first := scanCompleteFixture(t, reader)
		oldParent := filepath.Join(root, "OldNotes")
		if err := os.Rename(filepath.Join(root, "Notes"), oldParent); err != nil {
			t.Fatalf("Rename(Notes) error = %v", err)
		}
		if err := os.Mkdir(filepath.Join(root, "Notes"), 0o750); err != nil {
			t.Fatalf("Mkdir(Notes) error = %v", err)
		}
		if err := os.Rename(filepath.Join(oldParent, "one.md"), filepath.Join(root, "Notes", "one.md")); err != nil {
			t.Fatalf("Rename(one.md) error = %v", err)
		}
		second := scanCompleteFixture(t, reader)
		if first.SameFiles(second) {
			t.Fatal("SameFiles(replaced parent) = true, want false")
		}
	})

	t.Run("root object", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		root := filepath.Join(parent, "vault")
		writeReaderFixture(t, root, "Notes/one.md", "same")
		firstReader := openTestReader(t, root)
		first := scanCompleteFixture(t, firstReader)
		oldRoot := filepath.Join(parent, "old-vault")
		if err := os.Rename(root, oldRoot); err != nil {
			t.Fatalf("Rename(vault) error = %v", err)
		}
		if err := os.Mkdir(root, 0o750); err != nil {
			t.Fatalf("Mkdir(vault) error = %v", err)
		}
		if err := os.Rename(filepath.Join(oldRoot, "Notes"), filepath.Join(root, "Notes")); err != nil {
			t.Fatalf("Rename(Notes into replacement root) error = %v", err)
		}
		second := scanCompleteFixture(t, openTestReader(t, root))
		if first.SameFiles(second) {
			t.Fatal("SameFiles(replaced root) = true, want false")
		}
	})

	t.Run("raw spelling", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/Caf\u00e9.md", "same")
		first := scanCompleteFixture(t, openTestReader(t, root))
		second := first
		state := *first.state
		state.files = append([]Entry(nil), first.state.files...)
		state.files[0] = cloneEntry(state.files[0])
		state.files[0].rawPath = "Notes/Cafe\u0301.md"
		state.files[0].observed[len(state.files[0].observed)-1].rawPath = state.files[0].rawPath
		second.state = &state
		if first.SameFiles(second) {
			t.Fatal("SameFiles(changed raw spelling) = true, want false")
		}
	})

	t.Run("canonical path", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		first := scanCompleteFixture(t, openTestReader(t, root))
		second := cloneScanState(first)
		second.state.files[0].path = "Notes/other.md"
		if first.SameFiles(second) {
			t.Fatal("SameFiles(changed canonical path) = true, want false")
		}
	})

	t.Run("root name", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeReaderFixture(t, root, "Notes/one.md", "same")
		first := scanCompleteFixture(t, openTestReader(t, root))
		second := cloneScanState(first)
		second.state.rootName += "-renamed"
		if first.SameFiles(second) {
			t.Fatal("SameFiles(changed root name) = true, want false")
		}
	})
}

func TestEntryMetadataZeroValue(t *testing.T) {
	t.Parallel()
	var entry Entry
	if got := entry.Size(); got != 0 {
		t.Errorf("Entry{}.Size() = %d, want 0", got)
	}
	if got := entry.ModTime(); !got.IsZero() {
		t.Errorf("Entry{}.ModTime() = %v, want zero time", got)
	}
}

func TestReaderReadPrefix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "abcdef")
	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	tests := []struct {
		name    string
		limit   int64
		want    string
		wantErr error
	}{
		{name: "zero", limit: 0, want: ""},
		{name: "prefix", limit: 3, want: "abc"},
		{name: "past end", limit: 20, want: "abcdef"},
		{name: "negative", limit: -1, want: "", wantErr: fs.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := reader.ReadPrefix(t.Context(), entry, tt.limit)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ReadPrefix(limit=%d) error = %v, want %v", tt.limit, err, tt.wantErr)
			}
			if string(got) != tt.want {
				t.Errorf("ReadPrefix(limit=%d) = %q, want %q", tt.limit, got, tt.want)
			}
		})
	}
}

func TestReaderReadPrefixDiscardsBytesWhenLeafChangesAfterRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")
	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	mutated := false
	got, err := reader.readPrefix(t.Context(), entry, 3, func(stage readStage, _ string) error {
		if stage != readAfterLeafRead || mutated {
			return nil
		}
		mutated = true
		path := filepath.Join(root, "Notes", "one.md")
		if renameErr := os.Rename(path, filepath.Join(root, "Notes", "old.md")); renameErr != nil {
			return renameErr
		}
		return os.WriteFile(path, []byte("private"), 0o600)
	})
	if !mutated {
		t.Fatal("readPrefix race hook did not run")
	}
	if !errors.Is(err, ErrSourceChanged) || len(got) != 0 {
		t.Fatalf("ReadPrefix(changed leaf) = (%q, %v), want empty ErrSourceChanged", got, err)
	}
}

func TestReaderOpenFilePinsSelectedObjectAcrossAtomicReplacement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "selected")
	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	file, err := reader.OpenFile(t.Context(), entry)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("File.Close() error = %v", closeErr)
		}
	})

	path := filepath.Join(root, "Notes", "one.md")
	if renameErr := os.Rename(path, filepath.Join(root, "Notes", "selected.md")); renameErr != nil {
		t.Fatalf("rename selected file: %v", renameErr)
	}
	if writeErr := os.WriteFile(path, []byte("replacement"), 0o600); writeErr != nil {
		t.Fatalf("write replacement: %v", writeErr)
	}

	got, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll(open file) error = %v", err)
	}
	if string(got) != "selected" {
		t.Errorf("ReadAll(open file) = %q, want bytes from selected object", got)
	}
}

func TestReaderOpenFileRejectsChangedPathBeforeOpen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*testing.T, string)
	}{
		{
			name: "leaf replacement",
			change: func(t *testing.T, root string) {
				t.Helper()
				path := filepath.Join(root, "Notes", "one.md")
				if err := os.Rename(path, filepath.Join(root, "Notes", "old.md")); err != nil {
					t.Fatalf("rename leaf: %v", err)
				}
				if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
					t.Fatalf("write replacement leaf: %v", err)
				}
			},
		},
		{
			name: "parent replacement",
			change: func(t *testing.T, root string) {
				t.Helper()
				parent := filepath.Join(root, "Notes")
				if err := os.Rename(parent, filepath.Join(root, "old-notes")); err != nil {
					t.Fatalf("rename parent: %v", err)
				}
				writeReaderFixture(t, root, "Notes/one.md", "replacement")
			},
		},
		{
			name: "leaf symlink",
			change: func(t *testing.T, root string) {
				t.Helper()
				path := filepath.Join(root, "Notes", "one.md")
				if err := os.Remove(path); err != nil {
					t.Fatalf("remove leaf: %v", err)
				}
				if err := os.Symlink(filepath.Join(root, "old-notes", "one.md"), path); err != nil {
					t.Fatalf("replace leaf with symlink: %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeReaderFixture(t, root, "Notes/one.md", "selected")
			reader := openTestReader(t, root)
			entry, err := reader.Lookup("Notes/one.md")
			if err != nil {
				t.Fatalf("Lookup() error = %v", err)
			}
			if tt.name == "leaf symlink" {
				writeReaderFixture(t, root, "old-notes/one.md", "replacement")
			}
			tt.change(t, root)

			file, err := reader.OpenFile(t.Context(), entry)
			if file != nil {
				t.Cleanup(func() {
					if closeErr := file.Close(); closeErr != nil {
						t.Errorf("unexpected File.Close() error = %v", closeErr)
					}
				})
			}
			if !errors.Is(err, ErrSourceChanged) || file != nil {
				t.Fatalf("OpenFile(changed path) = (%v, %v), want nil ErrSourceChanged", file, err)
			}
		})
	}
}

func TestReaderOpenFileObservesCancellationDuringOpen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "selected")
	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	mutated := false
	file, err := reader.openFile(ctx, entry, func(stage readStage, _ string) error {
		if stage == readAfterLeafCheck && !mutated {
			mutated = true
			cancel()
		}
		return nil
	})
	if !mutated {
		t.Fatal("OpenFile cancellation hook did not run")
	}
	if !errors.Is(err, context.Canceled) || file != nil {
		t.Fatalf("OpenFile(cancelled) = (%v, %v), want nil context.Canceled", file, err)
	}
}

func TestReaderOperationsReportClosedReader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "one")
	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got, readErr := reader.ReadPrefix(t.Context(), entry, 1)
	if !errors.Is(readErr, fs.ErrClosed) || len(got) != 0 {
		t.Errorf("ReadPrefix(closed reader) = (%q, %v), want empty fs.ErrClosed", got, readErr)
	}
	got, readErr = reader.ReadFile(t.Context(), entry)
	if !errors.Is(readErr, fs.ErrClosed) || len(got) != 0 {
		t.Errorf("ReadFile(closed reader) = (%q, %v), want empty fs.ErrClosed", got, readErr)
	}
	if _, scanErr := reader.ScanComplete(t.Context()); !errors.Is(scanErr, fs.ErrClosed) {
		t.Errorf("ScanComplete(closed reader) error = %v, want fs.ErrClosed", scanErr)
	}
	if _, scanErr := reader.ScanAvailable(t.Context()); !errors.Is(scanErr, fs.ErrClosed) {
		t.Errorf("ScanAvailable(closed reader) error = %v, want fs.ErrClosed", scanErr)
	}
}

func TestReaderReadsObservedRegularFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "one")

	reader := openTestReader(t, root)
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	entries := scan.Files()
	if len(entries) != 1 || entries[0].Path() != "Notes/one.md" {
		t.Fatalf("ScanComplete().Files() = %#v, want Notes/one.md", entries)
	}
	got, err := reader.ReadFile(t.Context(), entries[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "one" {
		t.Fatalf("ReadFile() = %q, want one", got)
	}
}

func TestReaderRejectsEntryFromAnotherReader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "one")

	first := openTestReader(t, root)
	second := openTestReader(t, root)
	entry, err := first.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	got, err := second.ReadFile(t.Context(), entry)
	if err == nil || len(got) != 0 {
		t.Fatalf("ReadFile(entry from another reader) = (%q, %v), want empty error", got, err)
	}
}

func TestReaderRefreshRejectsEntryFromAnotherReader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "one")

	first := openTestReader(t, root)
	second := openTestReader(t, root)
	entry, err := first.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if _, err := second.Refresh(entry); err == nil {
		t.Fatal("Refresh() accepted an Entry from another Reader")
	}
}

func TestReaderRefreshAcceptsAtomicRegularReplacement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "before")

	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	replacement := filepath.Join(root, "Notes", "replacement.md")
	if writeErr := os.WriteFile(replacement, []byte("after"), 0o600); writeErr != nil {
		t.Fatalf("write replacement: %v", writeErr)
	}
	if renameErr := os.Rename(replacement, filepath.Join(root, "Notes", "one.md")); renameErr != nil {
		t.Fatalf("install replacement: %v", renameErr)
	}

	refreshed, err := reader.Refresh(entry)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	got, err := reader.ReadFile(t.Context(), refreshed)
	if err != nil {
		t.Fatalf("ReadFile(refreshed) error = %v", err)
	}
	if string(got) != "after" {
		t.Fatalf("ReadFile(refreshed) = %q, want after", got)
	}
}

func TestReaderRejectsLeafSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.md")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Notes"), 0o750); err != nil {
		t.Fatalf("mkdir Notes: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "Notes", "one.md")); err != nil {
		t.Fatalf("symlink leaf: %v", err)
	}

	reader := openTestReader(t, root)
	_, err := reader.Lookup("Notes/one.md")
	if !errors.Is(err, ErrSymbolicLink) {
		t.Fatalf("Lookup(symlink) error = %v, want ErrSymbolicLink", err)
	}
	// The refusal is deliberate, so it must not present as a race someone lost.
	if errors.Is(err, ErrSourceChanged) {
		t.Errorf("Lookup(symlink) error = %v, which also reads as a concurrent change", err)
	}
}

func TestReaderRejectsParentSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	private := t.TempDir()
	writeReaderFixture(t, private, "one.md", "private")
	if err := os.Symlink(private, filepath.Join(root, "Notes")); err != nil {
		t.Fatalf("symlink parent: %v", err)
	}

	reader := openTestReader(t, root)
	_, err := reader.Lookup("Notes/one.md")
	if !errors.Is(err, ErrSymbolicLink) {
		t.Fatalf("Lookup(parent symlink) error = %v, want ErrSymbolicLink", err)
	}
	if errors.Is(err, ErrSourceChanged) {
		t.Errorf("Lookup(parent symlink) error = %v, which also reads as a concurrent change", err)
	}
}

func TestReaderRejectsEnumeratedLeafReplacementBeforeRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")
	reader := openTestReader(t, root)
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	entries := scan.Files()
	if len(entries) != 1 {
		t.Fatalf("ScanComplete().Files() len = %d, want 1", len(entries))
	}

	path := filepath.Join(root, "Notes", "one.md")
	if renameErr := os.Rename(path, filepath.Join(root, "Notes", "old.md")); renameErr != nil {
		t.Fatalf("rename observed leaf: %v", renameErr)
	}
	if writeErr := os.WriteFile(path, []byte("private"), 0o600); writeErr != nil {
		t.Fatalf("replace observed leaf: %v", writeErr)
	}
	got, err := reader.ReadFile(t.Context(), entries[0])
	if !errors.Is(err, ErrSourceChanged) || len(got) != 0 {
		t.Fatalf("ReadFile(replaced observed leaf) = (%q, %v), want empty ErrSourceChanged", got, err)
	}
}

func TestReaderRejectsEnumeratedParentReplacementBeforeRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")
	reader := openTestReader(t, root)
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	entries := scan.Files()
	if len(entries) != 1 {
		t.Fatalf("ScanComplete().Files() len = %d, want 1", len(entries))
	}

	if renameErr := os.Rename(filepath.Join(root, "Notes"), filepath.Join(root, "old-notes")); renameErr != nil {
		t.Fatalf("rename observed parent: %v", renameErr)
	}
	writeReaderFixture(t, root, "Notes/one.md", "private")
	got, err := reader.ReadFile(t.Context(), entries[0])
	if !errors.Is(err, ErrSourceChanged) || len(got) != 0 {
		t.Fatalf("ReadFile(replaced observed parent) = (%q, %v), want empty ErrSourceChanged", got, err)
	}
}

func TestReaderRejectsEnumeratedLeafMutationBeforeRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")
	reader := openTestReader(t, root)
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	entries := scan.Files()
	if len(entries) != 1 {
		t.Fatalf("ScanComplete().Files() len = %d, want 1", len(entries))
	}

	path := filepath.Join(root, "Notes", "one.md")
	if writeErr := os.WriteFile(path, []byte("private bytes"), 0o600); writeErr != nil {
		t.Fatalf("mutate observed leaf: %v", writeErr)
	}
	got, err := reader.ReadFile(t.Context(), entries[0])
	if !errors.Is(err, ErrSourceChanged) || len(got) != 0 {
		t.Fatalf("ReadFile(mutated observed leaf) = (%q, %v), want empty ErrSourceChanged", got, err)
	}
}

func TestReaderRejectsLeafReplacementBetweenCheckAndOpen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")
	private := filepath.Join(t.TempDir(), "private.md")
	if err := os.WriteFile(private, []byte("private"), 0o600); err != nil {
		t.Fatalf("write private file: %v", err)
	}

	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	mutated := false
	got, err := reader.readFile(t.Context(), entry, func(stage readStage, _ string) error {
		if stage != readAfterLeafCheck || mutated {
			return nil
		}
		mutated = true
		path := filepath.Join(root, "Notes", "one.md")
		if removeErr := os.Remove(path); removeErr != nil {
			return removeErr
		}
		return os.Symlink(private, path)
	})
	if !mutated {
		t.Fatal("race hook did not run")
	}
	if err == nil || len(got) != 0 {
		t.Fatalf("ReadFile(replaced leaf) = (%q, %v), want empty error", got, err)
	}
}

func TestReaderRejectsParentReplacementBetweenCheckAndOpen(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")
	private := t.TempDir()
	writeReaderFixture(t, private, "one.md", "private")

	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	mutated := false
	got, err := reader.readFile(t.Context(), entry, func(stage readStage, name string) error {
		if stage != readAfterParentCheck || name != "Notes" || mutated {
			return nil
		}
		mutated = true
		if renameErr := os.Rename(filepath.Join(root, "Notes"), filepath.Join(root, "old-notes")); renameErr != nil {
			return renameErr
		}
		return os.Symlink(private, filepath.Join(root, "Notes"))
	})
	if !mutated {
		t.Fatal("parent race hook did not run")
	}
	if err == nil || len(got) != 0 {
		t.Fatalf("ReadFile(replaced parent) = (%q, %v), want empty error", got, err)
	}
}

func TestReaderDiscardsBytesWhenLeafNameChangesAfterOpen(t *testing.T) {
	t.Parallel()
	for _, stage := range []readStage{readAfterLeafOpen, readAfterLeafRead} {
		t.Run(stageName(stage), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeReaderFixture(t, root, "Notes/one.md", "public")
			private := filepath.Join(t.TempDir(), "private.md")
			if err := os.WriteFile(private, []byte("private"), 0o600); err != nil {
				t.Fatalf("write private file: %v", err)
			}

			reader := openTestReader(t, root)
			entry, err := reader.Lookup("Notes/one.md")
			if err != nil {
				t.Fatalf("Lookup() error = %v", err)
			}
			mutated := false
			got, err := reader.readFile(t.Context(), entry, func(gotStage readStage, _ string) error {
				if gotStage != stage || mutated {
					return nil
				}
				mutated = true
				path := filepath.Join(root, "Notes", "one.md")
				if renameErr := os.Rename(path, filepath.Join(root, "Notes", "old.md")); renameErr != nil {
					return renameErr
				}
				return os.Symlink(private, path)
			})
			if !mutated {
				t.Fatal("leaf race hook did not run")
			}
			if err == nil || len(got) != 0 {
				t.Fatalf("ReadFile(changed leaf name) = (%q, %v), want empty error", got, err)
			}
		})
	}
}

func stageName(stage readStage) string {
	switch stage {
	case readAfterLeafOpen:
		return "after-open"
	case readAfterLeafRead:
		return "after-read"
	default:
		return "unknown"
	}
}

func TestReaderPinsSelectedRootAcrossPathReplacement(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	writeReaderFixture(t, root, "Notes/one.md", "original")
	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}

	old := filepath.Join(parent, "old-vault")
	if renameErr := os.Rename(root, old); renameErr != nil {
		t.Fatalf("rename selected root: %v", renameErr)
	}
	writeReaderFixture(t, root, "Notes/one.md", "replacement")

	got, err := reader.ReadFile(t.Context(), entry)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("ReadFile() = %q, want bytes from pinned root", got)
	}
}

func TestReaderPreservesRawNFDLookupWithCanonicalNFCPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	raw := "Notes/Cafe\u0301.md"
	writeReaderFixture(t, root, raw, "decomposed")
	reader := openTestReader(t, root)
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	entries := scan.Files()
	if len(entries) != 1 || entries[0].Path() != "Notes/Caf\u00e9.md" {
		t.Fatalf("ScanComplete().Files() = %#v, want canonical NFC path", entries)
	}
	got, err := reader.ReadFile(t.Context(), entries[0])
	if err != nil {
		t.Fatalf("ReadFile(NFD entry) error = %v", err)
	}
	if string(got) != "decomposed" {
		t.Fatalf("ReadFile(NFD entry) = %q", got)
	}
}

func TestReaderRejectsCanonicalPathCollision(t *testing.T) {
	t.Parallel()
	seen := make(map[string]string)
	if err := recordCanonicalPath(seen, "Notes/Cafe\u0301.md", "Notes/Caf\u00e9.md"); err != nil {
		t.Fatalf("record first spelling: %v", err)
	}
	if err := recordCanonicalPath(seen, "Notes/Caf\u00e9.md", "Notes/Caf\u00e9.md"); !errors.Is(err, ErrCanonicalCollision) {
		t.Fatalf("record canonical collision = %v, want ErrCanonicalCollision", err)
	}
}

func openTestReader(t *testing.T, root string) *Reader {
	t.Helper()
	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return reader
}

func writeReaderFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir fixture parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func scanCompleteFixture(t *testing.T, reader *Reader) Scan {
	t.Helper()
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	return scan
}

func cloneScanState(scan Scan) Scan {
	state := *scan.state
	state.files = make([]Entry, len(scan.state.files))
	for i := range scan.state.files {
		state.files[i] = cloneEntry(scan.state.files[i])
	}
	return Scan{state: &state}
}

type testDirEntry struct {
	name    string
	dir     bool
	info    fs.FileInfo
	infoErr error
}

func (e testDirEntry) Name() string { return e.name }
func (e testDirEntry) IsDir() bool  { return e.dir }
func (e testDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	if e.info == nil {
		return 0
	}
	return e.info.Mode().Type()
}
func (e testDirEntry) Info() (fs.FileInfo, error) {
	if e.infoErr != nil {
		return nil, e.infoErr
	}
	return e.info, nil
}

func newTestSourceWalk(completeness scanCompleteness) sourceWalk {
	return sourceWalk{
		token:        &readerToken{},
		completeness: completeness,
		seen:         make(map[string]string),
		directories:  make(map[string]fs.FileInfo),
		contains:     map[string]struct{}{".": {}},
	}
}

// TestLookupClassifiesAbsentAndRefusedPaths locks the distinction this package
// used to collapse. Every one of these cases answered "vault source changed
// while it was being read" — a sentence that named a race nobody ran. The
// plain-folder case is the one a user met: pointing yomihon at a directory with
// no contract reported that the contract had changed while being read.
func TestLookupClassifiesAbsentAndRefusedPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "body")
	if err := os.WriteFile(filepath.Join(root, "plain"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write plain file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Notes", "sub"), 0o750); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	reader := openTestReader(t, root)

	tests := []struct {
		name    string
		lookup  string
		want    error
		notWant error
	}{
		{
			name:    "absent leaf",
			lookup:  "Notes/missing.md",
			want:    fs.ErrNotExist,
			notWant: ErrSourceChanged,
		},
		{
			name:    "absent parent",
			lookup:  "Missing/one.md",
			want:    fs.ErrNotExist,
			notWant: ErrSourceChanged,
		},
		{
			name:    "leaf is a directory",
			lookup:  "Notes/sub",
			want:    ErrNotRegular,
			notWant: ErrSourceChanged,
		},
		{
			name: "parent is a regular file",
			// Reporting ErrNotRegular here would be its own false diagnosis:
			// the path IS a regular file, it is just standing where a
			// directory belongs.
			lookup:  "plain/one.md",
			want:    ErrNotDirectory,
			notWant: ErrNotRegular,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := reader.Lookup(tt.lookup)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Lookup(%q) error = %v, want %v", tt.lookup, err, tt.want)
			}
			if errors.Is(err, tt.notWant) {
				t.Errorf("Lookup(%q) error = %v, which also reads as %v", tt.lookup, err, tt.notWant)
			}
		})
	}
}

// TestLookupNamesThePathTheCallerAsked locks the path in the error. The walk
// descends one component at a time into child roots, so the raw error names the
// bare component it stood on — a caller asking for a nested contract was told
// "System" was missing, which reads as a different file.
func TestLookupNamesThePathTheCallerAsked(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "body")
	reader := openTestReader(t, root)

	const want = "System/schemas/vault-schema.toml"
	_, err := reader.Lookup(want)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lookup(%q) error = %v, want fs.ErrNotExist", want, err)
	}
	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok {
		t.Fatalf("Lookup(%q) error = %v, want a *fs.PathError carrying the path", want, err)
	}
	if pathErr.Path != want {
		t.Errorf("Lookup(%q) reported path %q, want the path the caller asked for", want, pathErr.Path)
	}
}

// The walk confirms, after opening a directory, that the root it now holds is
// the directory it looked at a moment ago and that the name still points there.
// The leaf checks that follow cannot stand in for it: they compare the file,
// and a directory swapped for one that hardlinks the same file presents an
// identical leaf. What is different is the containment chain — every parent the
// entry recorded — and this is where that is answered.
func TestReaderRejectsAParentSwappedForOneHoldingTheSameFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/one.md", "public")

	decoy := filepath.Join(root, "decoy")
	if err := os.Mkdir(decoy, 0o750); err != nil {
		t.Fatalf("make the replacement directory: %v", err)
	}
	if err := os.Link(filepath.Join(root, "Notes", "one.md"), filepath.Join(decoy, "one.md")); err != nil {
		t.Fatalf("hardlink the same file into the replacement: %v", err)
	}

	reader := openTestReader(t, root)
	entry, err := reader.Lookup("Notes/one.md")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	swapped := false
	got, err := reader.readFile(t.Context(), entry, func(stage readStage, name string) error {
		if stage != readAfterParentCheck || name != "Notes" || swapped {
			return nil
		}
		swapped = true
		if renameErr := os.Rename(filepath.Join(root, "Notes"), filepath.Join(root, "old-notes")); renameErr != nil {
			return renameErr
		}
		return os.Rename(decoy, filepath.Join(root, "Notes"))
	})
	if !swapped {
		t.Fatal("the swap hook never ran, so this test proves nothing")
	}
	if !errors.Is(err, ErrSourceChanged) || len(got) != 0 {
		t.Fatalf("readFile() over a replaced parent = (%q, %v), want no bytes and %v", got, err, ErrSourceChanged)
	}
}
