package note_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
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

// TestHealthNamesANoteOverTheSourceBound is the wire from the generation's
// size skip to the health page. The snapshot lock holds that Skipped() names
// the note, and the page lock holds that a filled HealthView renders the row;
// neither asks whether healthSkipped carried Size across. Dropping that field
// leaves both green and /health silent about how large the note is.
func TestHealthNamesANoteOverTheSourceBound(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	huge := "# Huge\n\nrarespelunker sits here too.\n" + strings.Repeat("padding padding padding\n", 60000)
	if len(huge) <= render.MaxSourceBytes {
		t.Fatalf("the oversize fixture is %d bytes, which is under the cap; this would prove nothing", len(huge))
	}
	if err := os.WriteFile(filepath.Join(root, "huge.md"), []byte(huge), 0o600); err != nil {
		t.Fatalf("write huge note: %v", err)
	}

	srv := newServer(t, root)
	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	if !strings.Contains(page, "huge.md") {
		t.Error("the health page does not name the over-bound note")
	}
	if !strings.Contains(page, "over the source bound") {
		t.Error("the health page names the path without saying it is over the source bound")
	}
	if want := humanSizeZhHant(int64(len(huge))); !strings.Contains(page, want) {
		t.Errorf("the health page does not name the size %q", want)
	}
	if strings.Contains(page, "yomihon check") {
		t.Error("the page says the folder has nothing to answer for while holding an over-bound note")
	}
}

// humanSizeZhHant is the size the health row prints for a note over the bound:
// the same climb and thousands grouping the file page uses, in the language
// GET /health speaks when no cookie chose another.
func humanSizeZhHant(n int64) string {
	value := float64(n)
	unit := "KB"
	for _, u := range []string{"KB", "MB", "GB"} {
		value /= 1024
		unit = u
		if value < 1024 {
			break
		}
	}
	digits := strconv.FormatInt(n, 10)
	var grouped strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}
	return fmt.Sprintf(wording.ByteSizeFmt.In(wording.ZhHant), value, unit, grouped.String())
}
