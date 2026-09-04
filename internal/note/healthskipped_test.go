package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHealthNamesAPathTheScanPassedOver holds the reporting half of the
// symlink stance. yomihon does not follow a symbolic link — a note whose bytes
// live outside the folder would carry reading past the boundary the whole
// product is built on — but a vault that files its notes by link used to lose
// them in silence: no page, no listing, no finding, and a scan that reported
// nothing wrong.
//
// The page therefore names the path and says what it is. The second half of
// this test is the other side of the same stance: naming it must not start
// serving it.
func TestHealthNamesAPathTheScanPassedOver(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "Concepts")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Real.md"), []byte("---\ntitle: Real\ntype: concept\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if err := os.Symlink("Real.md", filepath.Join(dir, "Linked.md")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	srv := newServer(t, root)
	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	if !strings.Contains(page, "Concepts/Linked.md") {
		t.Error("the health page does not name the path the scan passed over")
	}
	if !strings.Contains(page, "symbolic link") {
		t.Error("the health page names the path without saying why it was passed over")
	}
	if strings.Contains(page, "yomihon check") {
		t.Error("the page says the folder has nothing to answer for while holding a passed-over path")
	}

	linkCode, _ := get(t, srv.Client(), srv.URL+"/notes/Concepts/Linked.md")
	if linkCode != http.StatusNotFound {
		t.Errorf("the symlink is served with status %d; reporting it must not start following it", linkCode)
	}
}
