package machinefile_test

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/vault"
)

const (
	thisMachine = "1f3a9c07"
	machinePath = "reader-notes/.yomihon/marks/" + thisMachine + ".jsonl"
)

// entry is one thing in a vault, as a snapshot holds it.
type entry struct {
	mode fs.FileMode
	data string
}

// snapshot is every name under root with its type and bytes, and where a link
// points, so that two snapshots are equal only if nothing in the tree changed.
func snapshot(t *testing.T, root string) map[string]entry {
	t.Helper()
	tree := map[string]entry{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		one := entry{mode: d.Type()}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			one.data, err = os.Readlink(path)
		case d.Type().IsRegular():
			var data []byte
			data, err = os.ReadFile(path) // #nosec G304 G122 -- a file of the test's own temporary vault, which nothing else changes
			one.data = string(data)
		}
		tree[filepath.ToSlash(rel)] = one
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// aVault is a temporary vault holding a note of the author's.
func aVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "courses/pg/vacuum.md", "# Vacuum\n")
	return root
}

func write(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil { // #nosec G306 -- a note of the test's own vault
		t.Fatal(err)
	}
}

func opened(t *testing.T, root string) *machinefile.Folder {
	t.Helper()
	r, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() }) //nolint:errcheck // the test is over
	f, err := machinefile.Open(r)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() }) //nolint:errcheck // the test is over
	return f
}

func replace(t *testing.T, f *machinefile.Folder, data string) {
	t.Helper()
	if err := f.Replace(thisMachine, []byte(data)); err != nil {
		t.Fatalf("Replace: %v", err)
	}
}

func readMachineFile(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(machinePath))) // #nosec G304 -- the test's own vault
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// changed lists the names whose entry differs between two snapshots.
func changed(before, after map[string]entry) []string {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(after)) {
		if was, ok := before[name]; !ok || was != after[name] {
			names = append(names, name)
		}
	}
	for name := range before {
		if _, ok := after[name]; !ok {
			names = append(names, name+" (gone)")
		}
	}
	slices.Sort(names)
	return names
}

// Opening the folder is a read: a vault that has never had a mark written gains
// nothing, not even the folder. The first write makes the three folders and the
// machine file and nothing else, and the next one replaces the file in place
// with nothing left beside it.
func TestTheFirstWriteMakesTheFoldersAndTheMachineFileAndNothingElse(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	before := snapshot(t, root)
	f := opened(t, root)
	if after := snapshot(t, root); len(changed(before, after)) != 0 {
		t.Fatalf("Open changed the vault: %v", changed(before, after))
	}

	replace(t, f, "first\n")
	want := []string{"reader-notes", "reader-notes/.yomihon", "reader-notes/.yomihon/marks", machinePath}
	if got := changed(before, snapshot(t, root)); !slices.Equal(got, want) {
		t.Fatalf("the first write changed %q, want %q", got, want)
	}

	replace(t, f, "second\n")
	if got := readMachineFile(t, root); got != "second\n" {
		t.Errorf("the machine file holds %q, want the second write", got)
	}
	entries, err := os.ReadDir(filepath.Join(root, "reader-notes", ".yomihon", "marks"))
	if err != nil || len(entries) != 1 {
		t.Errorf("the marks folder holds %v, %v; want only the machine file", entries, err)
	}
}

// What the reader keeps in their own folder, and the file another machine
// wrote, are left byte for byte as they were: only this machine's file is
// written.
func TestOnlyThisMachinesFileIsWritten(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	write(t, root, "reader-notes/my essay.md", "# Mine\n")
	write(t, root, "reader-notes/.yomihon/marks/c2e4b815.jsonl", "{\"other\":1}\n")
	before := snapshot(t, root)
	f := opened(t, root)
	replace(t, f, "{\"mine\":1}\n")
	if got, want := changed(before, snapshot(t, root)), []string{machinePath}; !slices.Equal(got, want) {
		t.Errorf("the write changed %q, want only %q", got, want)
	}
}

// A machine id is the whole of the file's name but its extension, so anything
// but eight lowercase hexadecimal digits could name another file, and is
// refused before anything is made.
func TestAMachineIDIsEightLowercaseHexDigits(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	f := opened(t, root)
	before := snapshot(t, root)
	for _, id := range []string{"", "1f3a9c0", "1f3a9c070", "1F3A9C07", "../../xx", "1f3a9c07.jsonl", "c2e4b81/", "notes.md"} {
		if err := f.Replace(id, []byte("x\n")); !errors.Is(err, machinefile.ErrMachineID) {
			t.Errorf("Replace(%q) = %v, want ErrMachineID", id, err)
		}
	}
	if got := changed(before, snapshot(t, root)); len(got) != 0 {
		t.Errorf("refused writes changed %q", got)
	}
}

// chmodFor gives the folder at path mode for the rest of the test. refused,
// when there is one, is what the mode is to refuse: when it goes through all
// the same, this user is a privileged one the mode does not hold, the fixture
// shows nothing, and the test is skipped.
func chmodFor(t *testing.T, path string, mode os.FileMode, refused func() error) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, mode); err != nil { // #nosec G302 -- the test's own folder
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, info.Mode().Perm()); err != nil { // #nosec G302 -- restores the mode so the folder can be removed
			t.Errorf("restore the mode of %s: %v", path, err)
		}
	})
	if refused != nil && refused() == nil {
		t.Skip("this user is let through what the mode refuses (a privileged user), so the fixture shows nothing")
	}
}

// symlink makes a link, or skips a test on a system that does not let this
// user make one, as Windows without developer mode does not.
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("this user cannot make a link: %v", err)
		}
		t.Fatal(err)
	}
}

// A folder on the way to the machine file that is a link is refused wherever it
// points, and so is one that is not a folder: a reader folder linked to a
// folder of notes would have the marks written among the notes. Open refuses it
// and so does a write, and what the link points at is left as it was. The
// refusal says whether the name is a link, so the reader is told which.
func TestAFolderOnTheWayThatIsALinkOrNotAFolderIsRefused(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		build    func(t *testing.T, root string)
		wantLink bool
	}{
		{"the reader folder is a link to the notes", func(t *testing.T, root string) {
			t.Helper()
			symlink(t, "courses", filepath.Join(root, "reader-notes"))
		}, true},
		{"the reader folder is a file", func(t *testing.T, root string) {
			t.Helper()
			write(t, root, "reader-notes", "not a folder\n")
		}, false},
		{".yomihon is a link to a folder of the reader's", func(t *testing.T, root string) {
			t.Helper()
			write(t, root, "reader-notes/essays/marks/keep.md", "# Keep\n")
			symlink(t, "essays", filepath.Join(root, "reader-notes", ".yomihon"))
		}, true},
		{"marks is a link to the notes", func(t *testing.T, root string) {
			t.Helper()
			symlink(t, filepath.Join("..", "..", "courses"), filepath.Join(root, "reader-notes", ".yomihon", "marks"))
		}, true},
		{"marks is a link inside the reader folder", func(t *testing.T, root string) {
			t.Helper()
			write(t, root, "reader-notes/essays/keep.md", "# Keep\n")
			symlink(t, filepath.Join("..", "essays"), filepath.Join(root, "reader-notes", ".yomihon", "marks"))
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := aVault(t)
			tt.build(t, root)
			before := snapshot(t, root)
			r, err := vault.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close() //nolint:errcheck // the test is over
			f, err := machinefile.Open(r)
			if err == nil {
				_ = f.Close() //nolint:errcheck // the failure below is the report
			}
			if !errors.Is(err, machinefile.ErrNotAFolder) || errors.Is(err, machinefile.ErrLink) != tt.wantLink {
				t.Errorf("Open = %v, want ErrNotAFolder, and ErrLink = %v", err, tt.wantLink)
			}
			if got := changed(before, snapshot(t, root)); len(got) != 0 {
				t.Errorf("a refused Open changed %q", got)
			}
		})
	}
}

// A link put in the place of the reader folder after it was opened is refused
// at the write, since every write enters the folders again.
func TestALinkPutInPlaceAfterOpenIsRefusedAtTheWrite(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	f := opened(t, root)
	replace(t, f, "kept\n")
	if err := os.Rename(filepath.Join(root, "reader-notes"), filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	symlink(t, "courses", filepath.Join(root, "reader-notes"))
	before := snapshot(t, root)
	if err := f.Replace(thisMachine, []byte("new\n")); !errors.Is(err, machinefile.ErrNotAFolder) {
		t.Errorf("a write through a reader folder that became a link = %v, want ErrNotAFolder", err)
	}
	if got := changed(before, snapshot(t, root)); len(got) != 0 {
		t.Errorf("the refused write changed %q", got)
	}
}

// The folder Open writes in is the one the server pinned: a folder put at the
// vault's path since is some other folder, and is refused.
func TestOpenRefusesAFolderPutWhereTheVaultWas(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	write(t, root, "a.md", "# A\n")
	r, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close() //nolint:errcheck // the test is over
	if err := os.Rename(root, filepath.Join(parent, "elsewhere")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("Windows does not rename a folder that is open: %v", err)
		}
		t.Fatal(err)
	}
	write(t, root, "b.md", "# B\n")
	if f, err := machinefile.Open(r); !errors.Is(err, machinefile.ErrMoved) {
		if err == nil {
			_ = f.Close() //nolint:errcheck // the failure below is the report
		}
		t.Errorf("Open of a vault moved away = %v, want ErrMoved", err)
	}
}

// A machine file longer than any reader of the folder reads would be a file no
// other machine can read, and is refused with nothing made.
func TestAMachineFileLongerThanItsLimitIsRefused(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	f := opened(t, root)
	before := snapshot(t, root)
	if err := f.Replace(thisMachine, make([]byte, machinefile.MaxFileBytes+1)); !errors.Is(err, machinefile.ErrTooLarge) {
		t.Errorf("Replace of %d bytes = %v, want ErrTooLarge", machinefile.MaxFileBytes+1, err)
	}
	if got := changed(before, snapshot(t, root)); len(got) != 0 {
		t.Errorf("the refused write changed %q", got)
	}
	if err := f.Replace(thisMachine, make([]byte, machinefile.MaxFileBytes)); err != nil {
		t.Errorf("Replace of exactly the limit = %v, want nil", err)
	}
}

// A write that cannot make its temporary file fails before anything is
// installed: the machine file holds what it held and nothing is left beside it.
func TestAWriteThatCannotMakeItsTemporaryFileLeavesTheMachineFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not refuse a new file by a folder's permission bits")
	}
	root := aVault(t)
	f := opened(t, root)
	replace(t, f, "kept\n")
	marks := filepath.Join(root, "reader-notes", ".yomihon", "marks")
	chmodFor(t, marks, 0o500, func() error {
		probe, err := os.Create(filepath.Join(marks, "probe")) // #nosec G304 -- the test's own folder
		if err == nil {
			_ = probe.Close() //nolint:errcheck // only whether it was made matters
		}
		return err
	})
	if err := f.Replace(thisMachine, []byte("new\n")); err == nil {
		t.Fatal("a write whose temporary file cannot be made = nil, want an error")
	}
	if got := readMachineFile(t, root); got != "kept\n" {
		t.Errorf("after the refused write the machine file holds %q, want what was kept", got)
	}
	if entries, err := os.ReadDir(marks); err != nil || len(entries) != 1 {
		t.Errorf("the marks folder holds %v, %v; want only the machine file", entries, err)
	}
}

// A rename that fails, here because the machine file's name is a folder, leaves
// the name as it was and removes the temporary file it would have installed.
func TestAFailedRenameLeavesNoTemporaryFile(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	write(t, root, machinePath+"/inside", "a folder in the machine file's place\n")
	before := snapshot(t, root)
	f := opened(t, root)
	if err := f.Replace(thisMachine, []byte("new\n")); err == nil {
		t.Fatal("a rename over a folder = nil, want an error")
	}
	if got := changed(before, snapshot(t, root)); len(got) != 0 {
		t.Errorf("the failed rename changed %q", got)
	}
}

// A temporary file older than ten minutes is what a crash between creating it
// and renaming it leaves, and the next write removes it. A younger one may be
// another process's write in progress, slow to sync on a USB disk or a network
// share, and is left; so is any file not named as this package names a
// temporary file, another machine's file of marks above all.
func TestAStaleTemporaryFileIsRemovedAndNothingElse(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	f := opened(t, root)
	replace(t, f, "kept\n")
	marks := "reader-notes/.yomihon/marks/"
	aged := func(name string, age time.Duration) {
		write(t, root, marks+name, "x")
		at := time.Now().Add(-age)
		if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(marks+name)), at, at); err != nil {
			t.Fatal(err)
		}
	}
	aged(".tmp-0123456789ab", 20*time.Minute)
	aged(".tmp-0123456789abc", 20*time.Minute)
	aged(".tmp-ABCDEF012345", 20*time.Minute)
	aged("c2e4b815.jsonl", 20*time.Minute)
	aged("notes.md", 20*time.Minute)
	aged(".tmp-fedcba987654", 5*time.Minute)
	aged(".tmp-a1b2c3d4e5f6", 0)
	before := snapshot(t, root)
	replace(t, f, "new\n")
	want := []string{marks + ".tmp-0123456789ab (gone)", machinePath}
	if got := changed(before, snapshot(t, root)); !slices.Equal(got, want) {
		t.Errorf("the write changed %q, want %q", got, want)
	}
}

// Every write makes sure the folders on the way are on stable storage, not
// only the write that made them: a write that made a folder and could not sync
// the folder above it failed, and the next one, finding the folder there, has
// to sync it all the same before it says its file is kept. A vault folder that
// can be written and searched but not read lets the reader folder be made and
// refuses the open its sync needs.
func TestEveryWriteSyncsTheFoldersOnTheWay(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a folder by its permission bits")
	}
	root := aVault(t)
	f := opened(t, root)
	chmodFor(t, root, 0o300, func() error {
		probe, err := os.Open(root) // #nosec G304 -- the test's own vault
		if err == nil {
			_ = probe.Close() //nolint:errcheck // only whether it opened matters
		}
		return err
	})
	for _, write := range []string{"the first, which makes the reader folder", "the next, which finds it"} {
		if err := f.Replace(thisMachine, []byte("x\n")); err == nil {
			t.Errorf("%s: a write whose vault folder cannot be synced = nil, want an error", write)
		}
	}
}

// Writers that do not take a lock lose each other's writes, which is what the
// caller's lock is for, but never leave a file that is part of one and part of
// another, or a temporary file behind.
func TestWritersAtOnceLeaveOneWholeFile(t *testing.T) {
	t.Parallel()
	root := aVault(t)
	f := opened(t, root)
	const writers = 8
	datas := make([]string, writers)
	for i := range datas {
		datas[i] = strings.Repeat(fmt.Sprintf("{\"writer\":%d}\n", i), 4096)
	}
	errs := make([]error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() { errs[i] = f.Replace(thisMachine, []byte(datas[i])) })
	}
	wg.Wait()
	// A write another one replaced before it was read back says so, and on
	// Windows a rename over a file another writer is reading back can be
	// refused; neither is a file left part one write and part another, which is
	// what is checked below. One write at least has to have been kept.
	if !slices.Contains(errs, nil) {
		t.Fatalf("no writer's write was kept: %v", errors.Join(errs...))
	}
	if got := readMachineFile(t, root); !slices.Contains(datas, got) {
		t.Errorf("the machine file holds %d bytes that are no one writer's whole write", len(got))
	}
	if entries, err := os.ReadDir(filepath.Join(root, "reader-notes", ".yomihon", "marks")); err != nil || len(entries) != 1 {
		t.Errorf("the marks folder holds %v, %v; want only the machine file", entries, err)
	}
}

// The zero Folder names no vault and writes nowhere.
func TestAZeroFolderIsRefused(t *testing.T) {
	t.Parallel()
	for _, f := range []*machinefile.Folder{nil, {}} {
		if err := f.Replace(thisMachine, []byte("x")); err == nil {
			t.Errorf("Replace on %#v = nil, want an error", f)
		}
		if err := f.Close(); err != nil {
			t.Errorf("Close on %#v = %v, want nil", f, err)
		}
	}
}

// ReadOnly asks the deepest folder that is there, writes nothing, and makes
// nothing.
func TestReadOnlyAsksTheDeepestFolderThatIsThere(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a folder's mode is not what stops a write on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes wherever it likes; the mode is still read, but the test below cannot tell")
	}
	root := aVault(t)
	f := opened(t, root)
	before := snapshot(t, root)
	if ro, err := f.ReadOnly(); err != nil || ro {
		t.Fatalf("ReadOnly on a vault the reader can write = %v, %v; want false", ro, err)
	}
	if got := changed(before, snapshot(t, root)); len(got) != 0 {
		t.Errorf("asking changed %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "reader-notes")); err == nil {
		t.Error("asking made the reader folder")
	}
	chmodFor(t, root, 0o500, nil)
	if ro, err := f.ReadOnly(); err != nil || !ro {
		t.Errorf("ReadOnly on a vault folder with no write bit = %v, %v; want true", ro, err)
	}
}
