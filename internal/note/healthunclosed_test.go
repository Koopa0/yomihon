package note_test

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestHealthSaysOnlyWhatIsTrueOfAFenceThatNeverCloses holds the health page to
// the note it lists. A note whose opening fence nothing closes is a frontmatter
// that could not be read, so it stands under that heading, and the lede under
// the heading has to name a fence that never closes as one of the reasons.
func TestHealthSaysOnlyWhatIsTrueOfAFenceThatNeverCloses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, n := range []struct{ path, body string }{
		{"Writing/Declared.md", "---\ntitle: Declared\ntype: writing\n---\n\nbody\n"},
		{"Writing/Unclosed.md", "---\ntitle: Unclosed\ntype: writing\n--\n\nbody\n"},
	} {
		full := filepath.Join(root, filepath.FromSlash(n.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", n.path, err)
		}
		if err := os.WriteFile(full, []byte(n.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", n.path, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			page := html.UnescapeString(getInLanguage(t, srv.Client(), srv.URL+"/health", lang))

			if !strings.Contains(page, wording.HealthFrontmatterTitle.In(lang)) {
				t.Fatalf("the health page has no section for %q", wording.HealthFrontmatterTitle.In(lang))
			}
			if !strings.Contains(page, "Unclosed") {
				t.Errorf("the health page does not name the note whose fence never closes")
			}
			lede := wording.HealthFrontmatterLede.In(lang)
			if !strings.Contains(page, lede) {
				t.Fatalf("the section does not carry the lede %q", lede)
			}
			if !strings.Contains(lede, "---") {
				t.Errorf("the lede does not name a fence that never closes, which is why this note is listed: %q", lede)
			}
		})
	}
}
