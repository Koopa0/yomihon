package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The raw endpoint was implemented, correct, and unreachable: nothing on any
// note page linked to it, and an engineer found it only by guessing the URL.
// The file view had offered the same thing all along, so the surface where a
// reader is most likely to ask what the file actually says was the one surface
// with no way to look.
//
// The link is followed here rather than merely spotted, because a door that
// opens on nothing is the same defect with a different shape.
func TestNotePageOffersTheFileItIsShowing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const body = "---\ntitle: 索引筆記\n---\n\n見 [[別的]]。\n"
	if err := os.WriteFile(filepath.Join(root, "索引筆記.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/索引筆記.md")
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	const want = `href="/raw/%E7%B4%A2%E5%BC%95%E7%AD%86%E8%A8%98.md"`
	if !strings.Contains(page, want) {
		t.Fatalf("note page has no way to reach the file's own bytes; want a link %s", want)
	}

	rawCode, raw := get(t, srv.Client(), srv.URL+"/raw/索引筆記.md")
	if rawCode != http.StatusOK {
		t.Fatalf("raw status = %d, want %d", rawCode, http.StatusOK)
	}
	// What comes back has to be the file, not a rendering of it: the reason to
	// follow this link is to see the frontmatter and the unresolved wikilink
	// exactly as they were typed.
	if raw != body {
		t.Errorf("raw bytes = %q, want the file unchanged %q", raw, body)
	}
}
