package note_test

import (
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestHealthSaysOnlyWhatIsTrueOfAFenceThatNeverClosesAndWhereItIsCounted holds
// the health page to the note it now lists. A note whose opening fence nothing
// closes is a frontmatter that could not be read, so it stands under that
// heading, but no YAML was parsed and the folder index does not count it in the
// lifecycle cell for notes whose frontmatter could not be read: it declared no
// status as far as that index can tell. The lede under the heading said, in
// both languages, that these notes' YAML is not valid and that the grouping by
// status keeps them in that one cell, so each of those claims has to name the
// notes it is true of.
func TestHealthSaysOnlyWhatIsTrueOfAFenceThatNeverClosesAndWhereItIsCounted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, n := range []struct{ path, body string }{
		{"Writing/Declared.md", "---\ntitle: Declared\ntype: doc\nstatus: draft\n---\n\nbody\n"},
		{"Writing/Unclosed.md", "---\ntitle: Unclosed\ntype: doc\nstatus: draft\n--\n\nbody\n"},
	} {
		full := filepath.Join(root, filepath.FromSlash(n.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", n.path, err)
		}
		if err := os.WriteFile(full, []byte(n.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", n.path, err)
		}
	}
	srv := newServerWithContract(t, root, distributionContract(t))

	// The fact the lede must not contradict: the folder index counts the
	// unclosed note among those that declared no status, and in no cell for
	// frontmatter it could not read.
	code, folders := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	if cells := unstatedCounts(t, homeSection(t, folders, `data-home-block="lifecycle"`)); len(cells) != 1 || cells[0] != 1 {
		t.Fatalf("the lifecycle block holds the cells %v for one unclosed note, want one cell of 1 and none for frontmatter that could not be read", cells)
	}

	sentenceEnd := regexp.MustCompile(`[。.]\s*`)
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
			// Every sentence that places a note in the grouping by status is
			// about YAML that is not valid, because that is the only kind of
			// note the folder index keeps in the cell it names.
			for _, sentence := range sentenceEnd.Split(lede, -1) {
				if (strings.Contains(sentence, "依狀態分組") || strings.Contains(sentence, "grouping by status")) &&
					!strings.Contains(sentence, "YAML") {
					t.Errorf("the lede promises the lifecycle cell to every note it lists, which an unclosed fence is not in: %q", sentence)
				}
			}
		})
	}
}
