package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestEveryPageDrawsTheSidebarItsKindOfPageCalls holds which sidebar each page
// beside the reading surfaces draws: a way into the folder stands in the
// library, the list of indexes in the index mode, and a page that is one thing
// or one tool draws the library only as a drawer.
func TestEveryPageDrawsTheSidebarItsKindOfPageCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, body := range map[string]string{
		"README.md":   "# Vault\n",
		"Notes/a.md":  "# A\n\nFirst.\n",
		"Notes/b.md":  "# B\n\nSecond.\n",
		"Notes/c.txt": "plain\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}
	srv := newServer(t, root)
	aside := regexp.MustCompile(`<aside class="y-rail-left"[^>]*data-sidebar-mode="([a-z]*)"([^>]*)>`)
	for _, tc := range []struct {
		path       string
		mode       string
		drawerOnly bool
	}{
		{"/", "library", false},
		{"/folders", "library", false},
		{"/folders/Notes", "library", false},
		{"/marks", "library", false},
		{"/indexes", "index", false},
		{"/journal", "library", true},
		{"/health", "library", true},
		{"/compare/Notes/a.md?with=Notes%2Fb.md", "library", true},
	} {
		code, page := getPage(t, srv.Client(), srv.URL+tc.path)
		if code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", tc.path, code, http.StatusOK)
			continue
		}
		found := aside.FindStringSubmatch(page)
		if found == nil {
			t.Errorf("GET %s draws no sidebar", tc.path)
			continue
		}
		if found[1] != tc.mode {
			t.Errorf("GET %s sidebar mode = %q, want %q", tc.path, found[1], tc.mode)
		}
		if got := regexp.MustCompile(`\bdata-drawer-only\b`).MatchString(found[2]); got != tc.drawerOnly {
			t.Errorf("GET %s drawer-only = %v, want %v", tc.path, got, tc.drawerOnly)
		}
	}
}
