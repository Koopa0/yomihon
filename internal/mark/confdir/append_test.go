package confdir_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// Append adds and never truncates: each call's bytes follow the last call's,
// and the first call makes the directory and the file, owner-only.
func TestAppendAddsToTheEndAndMakesTheFileOwnerOnly(t *testing.T) {
	t.Parallel()
	dir := dirFor(t, t.TempDir(), "/vaults/v")
	for _, data := range []string{"first\n", "second\n", "third"} {
		if err := confdir.Append(dir, "ledger.jsonl", []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := confdir.Read(dir, "ledger.jsonl", 1<<10)
	if err != nil || string(got) != "first\nsecond\nthird" {
		t.Fatalf("Read = %q, %v; want every append in order", got, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info, statErr := os.Stat(dir.Join("ledger.jsonl")); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, %v; want 600", info, statErr)
	}
	if info, statErr := os.Stat(dir.Path()); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("directory mode = %v, %v; want 700", info, statErr)
	}
}

// The first append makes an entry in the directory, and that entry is on stable
// storage only once the directory is synced. A directory that can be written
// and searched but not read refuses the open a sync needs, so an append that
// answers success there never asked for one. The refusal comes before a byte is
// written: there is no file holding part of what was refused.
func TestAppendRefusesAFirstWriteWhoseDirectoryCannotBeSynced(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a directory by its permission bits")
	}
	dir := homeIn(t)
	if err := os.Chmod(dir.Path(), 0o300); err != nil { // #nosec G302 -- the test's own temporary directory, writable and searchable
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir.Path(), 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if opened, err := os.Open(dir.Path()); err == nil {
		_ = opened.Close() //nolint:errcheck // only whether it opened matters
		t.Skip("this user opens a directory it may not read (a privileged user), so the fixture shows nothing")
	}

	if err := confdir.Append(dir, "ledger.jsonl", []byte("line\n")); err == nil {
		t.Fatal("a first Append into a directory that cannot be synced = nil, want an error")
	}
	if _, err := os.Lstat(dir.Join("ledger.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refused append made its file: %v", err)
	}
}

// A file this process has appended to already, its directory synced then, is
// appended to again without its directory being opened at all: only the file's
// own sync stands between the bytes and stable storage, so a directory that
// cannot be read does not refuse it.
func TestAppendToAFileThatIsThereSyncsOnlyTheFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a directory by its permission bits")
	}
	dir := homeIn(t)
	if err := confdir.Append(dir, "ledger.jsonl", []byte("one\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir.Path(), 0o300); err != nil { // #nosec G302 -- the test's own temporary directory, writable and searchable
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir.Path(), 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if err := confdir.Append(dir, "ledger.jsonl", []byte("two\n")); err != nil {
		t.Fatalf("Append to a file that is there = %v, want nil", err)
	}
	if got, err := os.ReadFile(dir.Join("ledger.jsonl")); err != nil || string(got) != "one\ntwo\n" {
		t.Errorf("the file reads %q, %v; want both lines", got, err)
	}
}

// A file this process has not appended to may have been made by a call that
// failed after it made it, or by a process that stopped there, and so its
// directory's entry for it may not be on stable storage: the first append to it
// syncs the directory too, and a directory that cannot be synced refuses it.
func TestTheFirstAppendToAFileThereSyncsItsDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a directory by its permission bits")
	}
	dir := homeIn(t)
	if err := os.WriteFile(dir.Join("ledger.jsonl"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir.Path(), 0o300); err != nil { // #nosec G302 -- the test's own temporary directory, writable and searchable
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir.Path(), 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if opened, err := os.Open(dir.Path()); err == nil {
		_ = opened.Close() //nolint:errcheck // only whether it opened matters
		t.Skip("this user opens a directory it may not read (a privileged user), so the fixture shows nothing")
	}
	if err := confdir.Append(dir, "ledger.jsonl", []byte("two\n")); err == nil {
		t.Fatal("the first Append to a file another made, in a directory that cannot be synced = nil, want an error")
	}
}

// What stands at the name and is not a regular file is refused before it is
// opened: a directory cannot hold a line, and a link would put the bytes
// wherever it points.
func TestAppendRefusesWhatIsNotARegularFile(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	if err := os.Mkdir(dir.Join("folder.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := confdir.Append(dir, "folder.jsonl", []byte("x\n")); !errors.Is(err, confdir.ErrNotRegular) {
		t.Errorf("Append to a directory = %v, want ErrNotRegular", err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.jsonl")
	if err := os.Symlink(outside, dir.Join("link.jsonl")); err != nil {
		t.Skipf("this platform or user cannot make a link: %v", err)
	}
	if err := confdir.Append(dir, "link.jsonl", []byte("x\n")); !errors.Is(err, confdir.ErrNotRegular) {
		t.Errorf("Append through a link = %v, want ErrNotRegular", err)
	}
	if _, err := os.Lstat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Append made %s through a link: %v", outside, err)
	}
}

func TestAppendAndRemoveLeftoversRefuseAnythingButAFileName(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	for _, name := range []string{"", ".", "..", "../outside.jsonl", "a/b.jsonl", `a\b.jsonl`} {
		if err := confdir.Append(dir, name, []byte("x\n")); err == nil {
			t.Errorf("Append(%q) = nil, want a refusal", name)
		}
		if err := confdir.RemoveLeftovers(dir, name); err == nil {
			t.Errorf("RemoveLeftovers(%q) = nil, want a refusal", name)
		}
	}
	var none confdir.Dir
	if err := confdir.Append(none, "ledger.jsonl", []byte("x\n")); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Append to the zero Dir = %v, want ErrNotNamed", err)
	}
	if err := confdir.RemoveLeftovers(none, "ledger.jsonl"); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("RemoveLeftovers in the zero Dir = %v, want ErrNotNamed", err)
	}
}

// RemoveLeftovers takes exactly the temporary files Replace makes for one name,
// as os.CreateTemp names them, and nothing a slip in the pattern would also
// take: the file itself, a temporary file of another file, a name that only
// starts or ends like one, a directory or a link of that shape.
func TestRemoveLeftoversTakesOnlyWhatReplaceLeftForThatName(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	var made []string
	for range 20 {
		// The pattern Replace hands os.CreateTemp for ledger.jsonl, written out
		// here rather than read from the package; the ledger's kill test holds
		// the same against temporary files a killed Replace really left.
		f, err := os.CreateTemp(dir.Path(), ".ledger-*.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, filepath.Base(f.Name()))
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	kept := []string{
		"ledger.jsonl", "ledger.lock", "ledger.damaged.jsonl", "marks.json",
		".marks-123.json", ".ledger.damaged-123.jsonl", ".ledger-123.json",
		".ledger-123.jsonl.bak", "x.ledger-123.jsonl", ".ledger-12a.jsonl",
		".ledger-.jsonl", ".ledger-12345678901.jsonl", ".ledger--1.jsonl", "123.jsonl", "ledger-123.jsonl",
	}
	for _, name := range kept {
		if err := os.WriteFile(dir.Join(name), []byte("keep\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(dir.Join(".ledger-7.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	kept = append(kept, ".ledger-7.jsonl")
	if err := os.Symlink(dir.Join("ledger.jsonl"), dir.Join(".ledger-8.jsonl")); err == nil {
		kept = append(kept, ".ledger-8.jsonl")
	}

	if err := confdir.RemoveLeftovers(dir, "ledger.jsonl"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir.Path())
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, entry := range entries {
		left = append(left, entry.Name())
	}
	slices.Sort(kept)
	if !slices.Equal(left, kept) {
		t.Errorf("after RemoveLeftovers the directory holds %q; want %q (made and removed: %q)", left, kept, made)
	}
	if err := confdir.RemoveLeftovers(dirFor(t, t.TempDir(), "/vaults/none"), "ledger.jsonl"); err != nil {
		t.Errorf("RemoveLeftovers in a directory that is not there = %v, want nil", err)
	}
}

// A lock is taken before the first write and makes the directory that write
// goes in, so the lock is where the directory's own entry has to reach stable
// storage: Replace and Append sync only the directories they make, and after
// the lock there are none left to make. A configuration directory that can be
// written and searched but not read refuses that sync, and a change inside the
// lock that answers success there was never made durable.
func TestALockMakesItsDirectoryDurably(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a directory by its permission bits")
	}
	config := t.TempDir()
	dir := dirFor(t, config, "/vaults/v")
	if err := os.Chmod(config, 0o300); err != nil { // #nosec G302 -- the test's own temporary directory, writable and searchable
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(config, 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if opened, err := os.Open(config); err == nil { // #nosec G304 -- the test's own temporary directory
		_ = opened.Close() //nolint:errcheck // only whether it opened matters
		t.Skip("this user opens a directory it may not read (a privileged user), so the fixture shows nothing")
	}

	err := confdir.Exclusive(dir, "marks.lock", 0, func() error {
		return confdir.Replace(dir, "marks.json", []byte("{}"))
	})
	if err == nil {
		t.Fatal("a first change whose new directories' parent cannot be synced = nil, want an error")
	}
	if _, statErr := os.Stat(dir.Join("marks.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("the refused change installed its file: %v", statErr)
	}
}
