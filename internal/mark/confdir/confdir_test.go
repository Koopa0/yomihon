package confdir_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

func dirFor(t *testing.T, configDir, vaultRoot string) confdir.Dir {
	t.Helper()
	dir, err := confdir.VaultDir(confdir.At(configDir), vaultRoot)
	if err != nil {
		t.Fatalf("VaultDir(%q, %q): %v", configDir, vaultRoot, err)
	}
	return dir
}

// homeIn is the reader's directory under a fresh configuration directory, made
// and empty, which is the one kind of Dir there is to work in.
func homeIn(t *testing.T) confdir.Dir {
	t.Helper()
	home, err := confdir.Home(confdir.At(t.TempDir()))
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if err = os.MkdirAll(home.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestADirectoryNeedsBothThingsThatNameIt(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		configDir confdir.ConfigDir
		vaultRoot string
	}{
		{"no configuration directory", confdir.ConfigDir{}, "/vaults/notes"},
		{"no vault root", confdir.At(t.TempDir()), ""},
		{"neither", confdir.ConfigDir{}, ""},
	} {
		if dir, err := confdir.VaultDir(tt.configDir, tt.vaultRoot); !errors.Is(err, confdir.ErrNotNamed) {
			t.Errorf("%s: VaultDir = %q, %v; want ErrNotNamed", tt.name, dir.Path(), err)
		}
	}
}

func TestTwoVaultsGetTwoDirectories(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if first, second := dirFor(t, config, "/vaults/notes"), dirFor(t, config, "/vaults/other"); first == second {
		t.Fatalf("two vaults share one directory at %s", first.Path())
	}
}

// TestTheSameFolderSpelledTwoWaysIsOneVault holds the NFC normalization the
// directory name is derived through. A folder whose name carries a composed
// character can be handed to the command in either spelling — the filesystem
// answers to both — and two directories with two different sets of marks in
// them would be one vault quietly split.
func TestTheSameFolderSpelledTwoWaysIsOneVault(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	// One character written the two ways Unicode allows: as itself, and as the
	// letter it is built from followed by the mark that modifies it. They are
	// spelled in code points rather than typed, because the two are identical in
	// every editor and a test whose two inputs were secretly one string would
	// compare nothing and pass.
	const (
		composed   = "/vaults/パ"  // U+30D1 alone
		decomposed = "/vaults/パ" // U+30CF followed by U+309A
	)
	if composed == decomposed {
		t.Fatal("the two spellings are the same bytes, so comparing what they derive proves nothing")
	}
	if a, b := dirFor(t, config, composed), dirFor(t, config, decomposed); a != b {
		t.Errorf("the same folder spelled two ways got two directories:\n%s\n%s", a.Path(), b.Path())
	}
}

// TestTheDirectoryNameSpellsNoPath holds the other half of that decision: a
// path is not a safe directory name, so none of it appears in one.
func TestTheDirectoryNameSpellsNoPath(t *testing.T) {
	t.Parallel()
	dir := dirFor(t, t.TempDir(), "/vaults/secret project/notes")
	name := filepath.Base(dir.Path())
	for _, fragment := range []string{"secret", "project", "notes", "vaults", " ", "/"} {
		if strings.Contains(name, fragment) {
			t.Errorf("the directory name %q carries %q from the vault path", name, fragment)
		}
	}
	if parent := filepath.Base(filepath.Dir(dir.Path())); parent != "yomihon" {
		t.Errorf("the directory sits under %q, want yomihon", parent)
	}
}

func TestReplaceInstallsAFileOwnerOnlyAndLeavesNothingBesideIt(t *testing.T) {
	t.Parallel()
	dir := dirFor(t, t.TempDir(), "/vaults/v")
	for _, data := range []string{"first\n", "second\n"} {
		if err := confdir.Replace(dir, "marks.json", []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := confdir.Read(dir, "marks.json", 1<<10)
	if err != nil || string(got) != "second\n" {
		t.Fatalf("Read = %q, %v; want the second write", got, err)
	}
	entries, err := os.ReadDir(dir.Path())
	if err != nil || len(entries) != 1 || entries[0].Name() != "marks.json" {
		t.Errorf("the directory holds %v, %v; want only marks.json", entries, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info, statErr := os.Stat(dir.Join("marks.json")); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, %v; want 600", info, statErr)
	}
	if info, statErr := os.Stat(dir.Path()); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("directory mode = %v, %v; want 700", info, statErr)
	}
}

// A write counts as kept only once its directory is on stable storage too. A
// directory that can be written and searched but not read lets the file be made
// and renamed, and refuses the open a sync of the directory needs, so a Replace
// that answers success there never asked for one. The refusal comes before the
// file is installed: what was kept before is what the file still holds.
func TestReplaceRefusesAWriteWhoseDirectoryCannotBeSynced(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a directory by its permission bits")
	}
	dir := homeIn(t)
	if err := confdir.Replace(dir, "marks.json", []byte("kept\n")); err != nil {
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

	if err := confdir.Replace(dir, "marks.json", []byte("new\n")); err == nil {
		t.Fatal("Replace into a directory that cannot be synced = nil, want an error")
	}
	if err := os.Chmod(dir.Path(), 0o700); err != nil { // #nosec G302 -- the test's own temporary directory, readable again
		t.Fatal(err)
	}
	if got, err := confdir.Read(dir, "marks.json", 1<<10); err != nil || string(got) != "kept\n" {
		t.Errorf("after the refused write the file reads %q, %v; want what was kept before", got, err)
	}
	if entries, err := os.ReadDir(dir.Path()); err != nil || len(entries) != 1 {
		t.Errorf("the directory holds %v, %v; want only marks.json, with no temporary file left beside it", entries, err)
	}
}

// The first write makes the directory, and the entry that names it is in the
// directory above, so that one is synced too or a power cut can lose the
// directory with the file in it. A configuration directory that can be written
// and searched but not read lets the directory be made and refuses that sync.
func TestReplaceRefusesAFirstWriteWhoseParentCannotBeSynced(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not open a directory by its permission bits")
	}
	config := t.TempDir()
	home, err := confdir.Home(confdir.At(config))
	if err != nil {
		t.Fatal(err)
	}
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

	if err := confdir.Replace(home, "vaults.json", []byte("{}")); err == nil {
		t.Fatal("a first Replace whose new directory's parent cannot be synced = nil, want an error")
	}
	if _, err := os.Stat(home.Join("vaults.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the refused write installed its file: %v", err)
	}
}

// A name is a file of the directory and nothing else: a path in it would write
// outside the directory this package exists to confine writes to.
func TestReplaceRefusesAnythingButAFileName(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	for _, name := range []string{"", ".", "..", "../x.json", "a/b.json", `a\b.json`, "/abs.json"} {
		if err := confdir.Replace(dir, name, []byte("x")); err == nil {
			t.Errorf("Replace(%q) = nil, want a refusal", name)
		}
	}
	if entries, err := os.ReadDir(filepath.Dir(dir.Path())); err != nil || len(entries) != 1 {
		t.Errorf("a refused name wrote beside the directory: %v, %v", entries, err)
	}
}

// Reading outside the directory would hand back a file that is not the reader's
// own, such as one of a vault beside it, to code that then writes it as theirs.
func TestReadRefusesAnythingButAFileName(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	// A file at every place a bad name would reach, so that a refusal is the check
	// and not a file that happens to be missing.
	for _, path := range []string{
		filepath.Join(filepath.Dir(dir.Path()), "x.json"),
		filepath.Join(dir.Path(), "sub", "b.json"),
		filepath.Join(dir.Path(), "abs.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
			t.Fatal(err)
		}
	}
	if err := confdir.Replace(dir, "own.json", []byte("x")); err != nil {
		t.Fatal(err)
	}
	// ".." is the parent directory, which Read would refuse as not a regular file
	// even with no check of the name, so a refusal for what is on disk is not the
	// refusal this asks for.
	for _, name := range []string{"..", "../x.json", "sub/b.json", `sub\b.json`, "/abs.json"} {
		data, err := confdir.Read(dir, name, 10)
		if err == nil {
			t.Errorf("Read(%q) = %q, want a refusal", name, data)
		} else if errors.Is(err, confdir.ErrNotRegular) || errors.Is(err, os.ErrNotExist) {
			t.Errorf("Read(%q) = %v, want it refused for its name and not for what is there", name, err)
		}
	}
	if data, err := confdir.Read(dir, "own.json", 10); err != nil || string(data) != "x" {
		t.Errorf("Read of a file name = %q, %v, want what was written", data, err)
	}
}

func TestReadRefusesAFileBeyondItsLimit(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	if err := confdir.Replace(dir, "f.json", []byte(strings.Repeat("x", 11))); err != nil {
		t.Fatal(err)
	}
	if data, err := confdir.Read(dir, "f.json", 11); err != nil || len(data) != 11 {
		t.Errorf("Read at the limit = %d bytes, %v; want the whole file", len(data), err)
	}
	if data, err := confdir.Read(dir, "f.json", 10); !errors.Is(err, confdir.ErrTooLarge) || data != nil {
		t.Errorf("Read past the limit = %d bytes, %v; want ErrTooLarge", len(data), err)
	}
	if _, err := confdir.Read(dir, "missing.json", 10); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Read of a missing file = %v, want it to be os.ErrNotExist", err)
	}
}

func TestHomeHoldsEveryVaultsDirectory(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	home, err := confdir.Home(confdir.At(config))
	if err != nil {
		t.Fatalf("Home: %v", err)
	}
	if want := filepath.Join(config, "yomihon"); home.Path() != want {
		t.Errorf("Home = %q, want %q", home.Path(), want)
	}
	dir := dirFor(t, config, "/vaults/notes")
	if want := filepath.Join(home.Path(), confdir.VaultID("/vaults/notes")); dir.Path() != want {
		t.Errorf("VaultDir = %q, want the vault's id under Home, %q", dir.Path(), want)
	}
	if _, err := confdir.Home(confdir.ConfigDir{}); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Home(\"\") = %v, want ErrNotNamed", err)
	}
}

func TestAVaultIDIsTheNameOfItsDirectoryAndSpellsNoPath(t *testing.T) {
	t.Parallel()
	id := confdir.VaultID("/vaults/notes")
	if len(id) != 32 {
		t.Errorf("VaultID = %q, want 32 hex digits", id)
	}
	if strings.Contains(id, "vaults") || strings.Contains(id, "notes") || strings.ContainsAny(id, `/\`) {
		t.Errorf("VaultID %q spells the path it names", id)
	}
	if other := confdir.VaultID("/vaults/other"); other == id {
		t.Errorf("two vaults share the id %q", id)
	}
	// One folder reached by two spellings of its name is one vault.
	if nfc, nfd := confdir.VaultID("/vaults/café"), confdir.VaultID("/vaults/café"); nfc != nfd {
		t.Errorf("one folder has two ids, %q and %q", nfc, nfd)
	}
}

func TestRemoveDeletesAFileAndAFileThatIsNotThereIsAlreadyRemoved(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	if err := confdir.Replace(dir, "vaults.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := confdir.Remove(dir, "vaults.json"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(dir.Join("vaults.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the file is still there: %v", err)
	}
	if err := confdir.Remove(dir, "vaults.json"); err != nil {
		t.Errorf("Remove of a file that is not there = %v, want nil", err)
	}
	for _, name := range []string{"", ".", "..", "../vaults.json", "a/b"} {
		if err := confdir.Remove(dir, name); err == nil {
			t.Errorf("Remove accepted %q", name)
		}
	}
}

func TestStatSaysHowLongAFileIsWithoutReadingIt(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	if _, err := confdir.Stat(dir, "vaults.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat of a file that is not there = %v, want ErrNotExist", err)
	}
	if err := confdir.Replace(dir, "vaults.json", []byte("12345")); err != nil {
		t.Fatal(err)
	}
	info, err := confdir.Stat(dir, "vaults.json")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 5 {
		t.Errorf("size = %d, want 5", info.Size())
	}
	if info.ModTime().IsZero() {
		t.Error("Stat gave no modification time")
	}
	if _, err := confdir.Stat(dir, "../vaults.json"); err == nil {
		t.Error("Stat accepted a name that leaves the directory")
	}
}

// The zero Dir names nothing, so a value nobody made cannot put a file beside
// whatever folder the process started in.
func TestAZeroDirIsRefusedByEverythingThatChangesAFile(t *testing.T) {
	t.Parallel()
	var none confdir.Dir
	if err := confdir.Replace(none, "f.json", []byte("x")); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Replace = %v, want ErrNotNamed", err)
	}
	if err := confdir.Remove(none, "f.json"); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Remove = %v, want ErrNotNamed", err)
	}
	if err := confdir.Rename(none, "f.json", "g.json"); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Rename = %v, want ErrNotNamed", err)
	}
	if _, err := confdir.Acquire(t.Context(), none, "l.lock", 0); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Acquire = %v, want ErrNotNamed", err)
	}
	if _, err := confdir.Read(none, "f.json", 1); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Read = %v, want ErrNotNamed", err)
	}
	if _, err := confdir.Stat(none, "f.json"); !errors.Is(err, confdir.ErrNotNamed) {
		t.Errorf("Stat = %v, want ErrNotNamed", err)
	}
}

// unreadableDir gives path, a directory of the test's own, mode, and gives it
// 0o700 back when the test ends. It skips where a mode refuses nothing: on
// Windows, which does not refuse a directory by its permission bits, and for a
// user who opens a directory it may not read (a privileged user).
func unreadableDir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not refuse a directory by its permission bits")
	}
	if err := os.Chmod(path, mode); err != nil { // #nosec G302 -- the test's own temporary directory
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if opened, err := os.Open(path); err == nil { // #nosec G304 -- the test's own temporary directory
		_ = opened.Close() //nolint:errcheck // only whether it opened matters
		t.Skip("this user opens a directory it may not read (a privileged user), so the fixture shows nothing")
	}
}
