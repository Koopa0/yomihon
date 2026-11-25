package vault_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// Not parallel: it sets the home directory, which t.Setenv does not allow in a
// parallel test.
func TestTildeSpellsAPathUnderTheHomeDirectoryTheWayTheReaderWritesIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	elsewhere := t.TempDir()

	for _, tt := range []struct {
		name string
		path string
		want string
	}{
		{"a folder under home", filepath.Join(home, "notes", "vault"), filepath.Join("~", "notes", "vault")},
		{"home itself", home, "~"},
		{"a path outside home", filepath.Join(elsewhere, "vault"), filepath.Join(elsewhere, "vault")},
		{"a sibling that begins with the same characters", home + "2", home + "2"},
		{"the parent of home", filepath.Dir(home), filepath.Dir(home)},
		{"a path that is not absolute", filepath.Join("notes", "vault"), filepath.Join("notes", "vault")},
		{"no path", "", ""},
	} {
		if got := vault.Tilde(tt.path); got != tt.want {
			t.Errorf("%s: Tilde(%q) = %q, want %q", tt.name, tt.path, got, tt.want)
		}
	}
}

func TestTildeReadsHomeThroughALinkAsWellAsAsResolved(t *testing.T) {
	target := t.TempDir()
	linked := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(target, linked); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", linked)
	t.Setenv("USERPROFILE", linked)

	want := filepath.Join("~", "vault")
	for name, path := range map[string]string{
		"the vault as the link spells it":  filepath.Join(linked, "vault"),
		"the vault as it is once resolved": filepath.Join(resolved, "vault"),
	} {
		if got := vault.Tilde(path); got != want {
			t.Errorf("%s: Tilde(%q) = %q, want %q", name, path, got, want)
		}
	}
}

func TestTildeGivesThePathBackWhenThereIsNoHomeDirectory(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	path := filepath.Join(t.TempDir(), "vault")
	if got := vault.Tilde(path); got != path {
		t.Errorf("Tilde(%q) = %q with no home directory, want it unchanged", path, got)
	}
	t.Setenv("HOME", "relative")
	t.Setenv("USERPROFILE", "relative")
	if got := vault.Tilde(path); got != path {
		t.Errorf("Tilde(%q) = %q with a home directory that is not absolute, want it unchanged", path, got)
	}
}
