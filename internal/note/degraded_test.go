package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDegradedFixture builds a folder where one note is captured by the scan
// but cannot be read: note-ok.md cites the locked note, so the health page has
// a citation to classify, and note-locked.md carries no read permission at
// all. The chmod is undone by the test's cleanup so the temp dir can be
// removed.
func writeDegradedFixture(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	okPath := filepath.Join(root, "note-ok.md")
	if err := os.WriteFile(okPath, []byte("---\ntitle: OK\ntype: concept\n---\nsee [[note-locked]]\n"), 0o600); err != nil {
		t.Fatalf("write note-ok.md: %v", err)
	}
	lockedPath := filepath.Join(root, "note-locked.md")
	if err := os.WriteFile(lockedPath, []byte("---\ntitle: Locked\ntype: concept\n---\nlocked body\n"), 0o600); err != nil {
		t.Fatalf("write note-locked.md: %v", err)
	}
	if err := os.Chmod(lockedPath, 0o000); err != nil {
		t.Fatalf("chmod note-locked.md: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(lockedPath, 0o600); err != nil {
			t.Errorf("restore note-locked.md mode: %v", err)
		}
	})
	if _, err := os.ReadFile(lockedPath); err == nil { // #nosec G304 -- probing a path inside this test's own TempDir
		t.Skip("mode 000 does not block reads here (running as a privileged user)")
	}
	return root
}

// TestUnreadableNoteIsVisibleOnEveryReadingSurface pins the three surfaces
// that must say what stderr alone said before: with one note unreadable at
// startup, the published generation is served for availability, and the home
// page states the degradation, the health page names the blocked file and
// stops classifying citations to it as pointing at nothing, and the note's
// own page says the file exists but could not be read rather than answering
// with the plain not-found page.
func TestUnreadableNoteIsVisibleOnEveryReadingSurface(t *testing.T) {
	t.Parallel()
	root := writeDegradedFixture(t)
	srv := newServer(t, root)

	code, home := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", code, http.StatusOK)
	}
	// The rail lists every file, so the name alone proves nothing; the notice
	// and the name it carries are looked for from the notice's marker on.
	if i := strings.Index(home, "讀不進來"); i < 0 {
		t.Error("home page carries no degraded notice for an unreadable source")
	} else if !strings.Contains(home[i:], "note-locked.md") {
		t.Error("home degraded notice does not name the blocked file")
	}

	code, health := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", code, http.StatusOK)
	}
	if i := strings.Index(health, "讀不進來的檔案"); i < 0 {
		t.Error("health page has no blocked-sources section")
	} else if !strings.Contains(health[i:], "note-locked.md") {
		t.Error("health blocked-sources section does not name the blocked file")
	}
	// The citation in note-ok.md lands on a file that exists; listing it with
	// the citations whose targets do not exist sends the reader to write a
	// note that is already on disk.
	if strings.Contains(health, "連到「note-locked」") {
		t.Error("health page classifies a citation to an existing but unreadable file as unwritten")
	}

	code, page := get(t, srv.URL+"/notes/note-locked.md")
	if code != http.StatusNotFound {
		t.Fatalf("GET /notes/note-locked.md status = %d, want %d", code, http.StatusNotFound)
	}
	if !strings.Contains(page, "檔案存在") {
		t.Error("the unreadable note's page does not say the file exists but could not be read")
	}
	if strings.Contains(page, "這裡沒有東西") {
		t.Error("the unreadable note's page is indistinguishable from the plain not-found page")
	}
}
