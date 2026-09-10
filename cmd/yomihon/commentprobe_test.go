package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

const (
	closedProbeTitle   = "Closed probe"
	unclosedProbeTitle = "Unclosed probe"
	visibleLessonOne   = "What yomihon is"
	visibleLessonTwo   = "The vault contract"
	hiddenLessonOne    = "Diagnostics are reports"
	hiddenLessonTwo    = "Hidden Broken Link"
	probeCourse        = "## Course {sequence=primary}\n\n" +
		"- [[" + visibleLessonOne + "]]\n" +
		"- [[" + visibleLessonTwo + "]]\n\n" +
		"%%\n" +
		"- [[" + hiddenLessonOne + "]]\n" +
		"- [[" + hiddenLessonTwo + "]]\n"
)

// TestPathsCountAgreesWithWhatThePageShows is the fixture pair the ruling
// named: two study-paths identical except that one closes the comment. The
// page hides everything after an unclosed %%; /paths used to count the parked
// rows anyway. Both courses now show 2 課, and each note's article names
// only the two lessons written before the mark.
func TestPathsCountAgreesWithWhatThePageShows(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCommentProbeVault(t, root)
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	paths := readingPage(t, site, "/paths")
	for _, title := range []string{closedProbeTitle, unclosedProbeTitle} {
		card := indexCard(t, paths, title)
		if !strings.Contains(card, "2 課") {
			t.Errorf("/paths lists %q as %q, want 2 課", title, card)
		}
		if strings.Contains(card, "4 課") {
			t.Errorf("/paths still counts the rows a comment hid on %q: %q", title, card)
		}
	}

	for _, rel := range []string{"Maps/Closed probe.md", "Maps/Unclosed probe.md"} {
		page := readingPage(t, site, "/notes/"+strings.ReplaceAll(rel, " ", "%20"))
		article := noteArticle(t, page)
		for _, visible := range []string{visibleLessonOne, visibleLessonTwo} {
			if !strings.Contains(article, visible) {
				t.Errorf("%s article is missing visible lesson %q; article = %q", rel, visible, article)
			}
		}
		for _, hidden := range []string{hiddenLessonOne, hiddenLessonTwo} {
			if strings.Contains(article, hidden) {
				t.Errorf("%s article still shows parked lesson %q; article = %q", rel, hidden, article)
			}
		}
	}
}

func writeCommentProbeVault(t *testing.T, root string) {
	t.Helper()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	files := map[string]string{
		schema.ContractRelPath: string(contract),
		"Maps/Closed probe.md": "---\ntitle: " + closedProbeTitle + "\ntype: study-path\n---\n\n" +
			probeCourse + "%%\n",
		"Maps/Unclosed probe.md": "---\ntitle: " + unclosedProbeTitle + "\ntype: study-path\n---\n\n" +
			probeCourse,
		"Writing/" + visibleLessonOne + ".md": "---\ntitle: " + visibleLessonOne + "\ntype: lesson\nstatus: draft\n---\n\nbody\n",
		"Writing/" + visibleLessonTwo + ".md": "---\ntitle: " + visibleLessonTwo + "\ntype: lesson\nstatus: draft\n---\n\nbody\n",
		"Writing/" + hiddenLessonOne + ".md":  "---\ntitle: " + hiddenLessonOne + "\ntype: lesson\nstatus: draft\n---\n\nbody\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err = os.WriteFile(full, []byte(body), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func indexCard(t *testing.T, page, title string) string {
	t.Helper()
	at := strings.Index(page, title)
	if at < 0 {
		t.Fatalf("no /paths card for %q; page = %q", title, page)
	}
	end := strings.Index(page[at:], "</a>")
	if end < 0 {
		t.Fatalf("unterminated /paths card for %q", title)
	}
	return page[at : at+end]
}

func noteArticle(t *testing.T, page string) string {
	t.Helper()
	_, after, ok := strings.Cut(page, "<article")
	if !ok {
		t.Fatalf("note page has no article; page = %q", page)
	}
	article, _, ok := strings.Cut(after, "</article>")
	if !ok {
		t.Fatalf("note page has no closing article; page = %q", page)
	}
	return article
}
