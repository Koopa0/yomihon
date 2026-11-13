package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

func searchResultsBody(t *testing.T, snapshot func() RequestSnapshot, query string) string {
	t.Helper()
	h := NewHandler(snapshot, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search/results?q="+query, http.NoBody)
	rr := httptest.NewRecorder()
	h.results(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	return rr.Body.String()
}

// linksToNote reports whether html holds a result link to relPath. The note's
// address is where such an href starts rather than the whole of it: a row whose
// excerpt marked something carries the words the query found after it, so the
// browser opens the note where they are.
func linksToNote(html, relPath string) bool {
	address := `href="/notes/` + relPath
	return strings.Contains(html, address+`"`) || strings.Contains(html, address+`#`)
}

// resultRow cuts one hit out of a rendered results fragment by the note it
// links to, so an assertion about one row cannot be answered by another row.
func resultRow(t *testing.T, body, relPath string) string {
	t.Helper()
	for _, chunk := range strings.Split(body, `<li>`)[1:] {
		row, _, terminated := strings.Cut(chunk, "</li>")
		if !terminated {
			t.Fatalf("unterminated result row: %q", chunk)
		}
		if linksToNote(row, relPath) {
			return row
		}
	}
	t.Fatalf("no result row links to %q; body = %q", relPath, body)
	return ""
}

// TestSearchRowMarksADraftAndNothingElse pins the one per-note fact a result
// row adds after its path: the author's own draft flag. It is the author's
// declaration rather than a value from a contract's vocabulary, so the mark is
// the same whether or not a contract governs the folder, and a finished note
// carries none.
func TestSearchRowMarksADraftAndNothingElse(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Draft.md", Title: "Draft one", NoteType: "lesson", Draft: true, PlainText: "needle"},
		{RelPath: "Writing/Done.md", Title: "Done one", NoteType: "lesson", PlainText: "needle"},
	}, schema.ArtifactPolicy{})
	mark := wording.Draft.In(wording.ZhHant)

	for _, governed := range []bool{true, false} {
		body := searchResultsBody(t, func() RequestSnapshot {
			return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: governed}}
		}, "needle")
		if row := resultRow(t, body, "Writing/Draft.md"); !strings.Contains(row, mark) {
			t.Errorf("governed=%t: the draft row does not say %q; row = %q", governed, mark, row)
		}
		if row := resultRow(t, body, "Writing/Done.md"); strings.Contains(row, mark) || strings.Contains(row, "y-result__sep") {
			t.Errorf("governed=%t: the finished row carries a mark; row = %q", governed, row)
		}
	}
}
