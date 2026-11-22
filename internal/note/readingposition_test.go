package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The browser keeps where a reader stopped in a note against the vault's id and
// the note's path, and only the page can say which note it is. A note page names
// both on the root, the id is the same for every note of one vault and differs
// between two vaults, and a page that is not a note names none, so nothing is
// kept against a desk or a list.
func TestANotePageNamesItsVaultAndItsPathForTheBrowsersPlace(t *testing.T) {
	t.Parallel()

	write := func(t *testing.T, root string) {
		t.Helper()
		for name, body := range map[string]string{
			"README.md": "# Vault\n",
			"one.md":    "# One\n\nfirst.\n",
			"two.md":    "# Two\n\nsecond.\n",
		} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	vaultOf := regexp.MustCompile(` data-note-vault="([0-9a-f]{32})"`)
	pathOf := regexp.MustCompile(` data-note-path="([^"]*)"`)
	read := func(t *testing.T, base, target string) (vault, path string) {
		t.Helper()
		code, body := get(t, http.DefaultClient, base+target)
		if code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", target, code)
		}
		if m := vaultOf.FindStringSubmatch(body); m != nil {
			vault = m[1]
		}
		if m := pathOf.FindStringSubmatch(body); m != nil {
			path = m[1]
		}
		return vault, path
	}

	rootA, rootB := t.TempDir(), t.TempDir()
	write(t, rootA)
	write(t, rootB)
	a, b := newServer(t, rootA), newServer(t, rootB)

	vaultOne, pathOne := read(t, a.URL, "/notes/one.md")
	vaultTwo, pathTwo := read(t, a.URL, "/notes/two.md")
	vaultOther, _ := read(t, b.URL, "/notes/one.md")
	if vaultOne == "" {
		t.Fatal("a note page names no vault for the browser's place: it carries no data-note-vault of 32 hex digits")
	}
	if pathOne != "one.md" || pathTwo != "two.md" {
		t.Errorf("the pages name the paths %q and %q, want one.md and two.md", pathOne, pathTwo)
	}
	if vaultOne != vaultTwo {
		t.Errorf("two notes of one vault name the vaults %q and %q, want one", vaultOne, vaultTwo)
	}
	if vaultOne == vaultOther {
		t.Errorf("two vaults both name %q: a place kept in one would be found in the other, which is what the id is for", vaultOne)
	}
	for _, target := range []string{"/", "/folders"} {
		if vault, path := read(t, a.URL, target); vault != "" || path != "" {
			t.Errorf("GET %s names the note %q in the vault %q, want no note: it is not one", target, path, vault)
		}
	}
}
