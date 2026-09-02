package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// TestAResultFoundByItsPathShowsWhereItMatched covers the row that answers
// without saying why. A note reached only through where it lives has no
// snippet — nothing in its words matched — so the reader is handed a title and
// a path and left to work out what any of it had to do with their query.
func TestAResultFoundByItsPathShowsWhereItMatched(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		// The term is in the path and nowhere in the words, which is the whole
		// shape being tested: a hit with no snippet to explain it.
		{RelPath: "Pharmacology/Notes.md", Title: "Opening", NoteType: "concept", Status: "draft",
			PlainText: "nothing here repeats the folder name"},
		// The control: matched by its words, so its own snippet already says
		// why, and its path has no business being marked.
		{RelPath: "Concepts/Other.md", Title: "Other", NoteType: "concept", Status: "draft",
			PlainText: "this one mentions pharmacology in its body"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.URL+"/search?q=pharmacology")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "Pharmacology/Notes.md") {
		t.Fatalf("the fixture's premise is wrong: the path hit is not in the results:\n%s", body)
	}

	row := resultRow(t, body, "Pharmacology/Notes.md")
	if strings.Contains(row, "y-result__snippet") {
		t.Fatalf("the fixture's premise is wrong: this row has a snippet, so it is not the silent case:\n%s", row)
	}
	if !strings.Contains(row, "<mark>Pharmacology</mark>") {
		t.Errorf("the row does not show where the query met this note:\n%s", row)
	}

	// The control's path never held the term, so nothing in it is marked —
	// otherwise the mark would say something about every row alike.
	other := resultRow(t, body, "Concepts/Other.md")
	if strings.Contains(other, "<mark>") && !strings.Contains(other, "y-result__snippet") {
		t.Errorf("a row matched by its words had its path marked:\n%s", other)
	}
}
