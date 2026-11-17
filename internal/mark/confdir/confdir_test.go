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

func dirFor(t *testing.T, configDir, vaultRoot string) string {
	t.Helper()
	dir, err := confdir.Dir(configDir, vaultRoot)
	if err != nil {
		t.Fatalf("Dir(%q, %q): %v", configDir, vaultRoot, err)
	}
	return dir
}

func TestADirectoryNeedsBothThingsThatNameIt(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, configDir, vaultRoot string }{
		{"no configuration directory", "", "/vaults/notes"},
		{"no vault root", "/config", ""},
		{"neither", "", ""},
	} {
		if dir, err := confdir.Dir(tt.configDir, tt.vaultRoot); !errors.Is(err, confdir.ErrNotNamed) {
			t.Errorf("%s: Dir = %q, %v; want ErrNotNamed", tt.name, dir, err)
		}
	}
}

func TestTwoVaultsGetTwoDirectories(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if first, second := dirFor(t, config, "/vaults/notes"), dirFor(t, config, "/vaults/other"); first == second {
		t.Fatalf("two vaults share one directory at %s", first)
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
		t.Errorf("the same folder spelled two ways got two directories:\n%s\n%s", a, b)
	}
}

// TestTheDirectoryNameSpellsNoPath holds the other half of that decision: a
// path is not a safe directory name, so none of it appears in one.
func TestTheDirectoryNameSpellsNoPath(t *testing.T) {
	t.Parallel()
	dir := dirFor(t, t.TempDir(), "/vaults/secret project/notes")
	name := filepath.Base(dir)
	for _, fragment := range []string{"secret", "project", "notes", "vaults", " ", "/"} {
		if strings.Contains(name, fragment) {
			t.Errorf("the directory name %q carries %q from the vault path", name, fragment)
		}
	}
	if parent := filepath.Base(filepath.Dir(dir)); parent != "yomihon" {
		t.Errorf("the directory sits under %q, want yomihon", parent)
	}
}

func TestReplaceInstallsAFileOwnerOnlyAndLeavesNothingBesideIt(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "yomihon", "v")
	for _, data := range []string{"first\n", "second\n"} {
		if err := confdir.Replace(dir, "marks.json", []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := confdir.Read(dir, "marks.json", 1<<10)
	if err != nil || string(got) != "second\n" {
		t.Fatalf("Read = %q, %v; want the second write", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "marks.json" {
		t.Errorf("the directory holds %v, %v; want only marks.json", entries, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info, statErr := os.Stat(filepath.Join(dir, "marks.json")); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, %v; want 600", info, statErr)
	}
	if info, statErr := os.Stat(dir); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("directory mode = %v, %v; want 700", info, statErr)
	}
}

// A name is a file of the directory and nothing else: a path in it would write
// outside the directory this package exists to confine writes to.
func TestReplaceRefusesAnythingButAFileName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"", ".", "..", "../x.json", "a/b.json", `a\b.json`, "/abs.json"} {
		if err := confdir.Replace(dir, name, []byte("x")); err == nil {
			t.Errorf("Replace(%q) = nil, want a refusal", name)
		}
	}
	if entries, err := os.ReadDir(filepath.Dir(dir)); err != nil || len(entries) != 1 {
		t.Errorf("a refused name wrote beside the directory: %v, %v", entries, err)
	}
}

func TestReadRefusesAFileBeyondItsLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
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
