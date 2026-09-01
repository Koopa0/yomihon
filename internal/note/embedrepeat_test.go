package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repeatedHeadingVault seeds a note that names one section of another, where
// that name belongs to two sections. The excerpt a reader gets is one of them,
// and nothing on the page has ever said the other existed.
func repeatedHeadingVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Concepts/劑量.md", "---\ntitle: 劑量\ntype: concept\ndomain: golang\nstatus: draft\n---\n\n## 用量\n\nFIRSTTEXT\n\n## 其他\n\nx\n\n## 用量\n\nSECONDTEXT\n")
	write("Concepts/入口.md", "---\ntitle: 入口\ntype: concept\ndomain: golang\nstatus: draft\n---\n\n![[劑量#用量]]\n")
	return root
}

// TestNoteSaysAnEmbeddedSectionNameMatchedMoreThanOne holds the page to what
// the excerpt cannot say for itself. A reader looking at one section has no
// way to know a second of that name went unshown, and the page's own account
// of what is wrong with the file is where that belongs.
//
// The kind is new, and a kind the page has no words for falls back to printing
// its internal slug — which is why the assertion is that the row reads as a
// sentence rather than merely that a row appeared.
func TestNoteSaysAnEmbeddedSectionNameMatchedMoreThanOne(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, repeatedHeadingVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/notes/Concepts/%E5%85%A5%E5%8F%A3.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// The excerpt itself carries the count, above the words it chose.
	if !strings.Contains(body, "embed__note") {
		t.Errorf("the excerpt does not say it was one of several:\n%s", body)
	}
	if !strings.Contains(body, "FIRSTTEXT") || strings.Contains(body, "SECONDTEXT") {
		t.Errorf("the excerpt no longer shows the first section alone")
	}

	row := diagRow(t, noteConditions(t, body), "劑量#用量")
	if strings.Contains(row, "embed-fragment-repeated") {
		t.Errorf("the page printed the diagnostic's internal name instead of words for it; row = %q", row)
	}
	if !strings.Contains(row, "不只一個") {
		t.Errorf("the page's account of the file does not say the name matched more than one section; row = %q", row)
	}
}
