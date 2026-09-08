package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// sourceServer answers over a fence-only note and one that repeats the
// same words in prose, so a source label proves the excerpt came from
// the fence and nothing else could have produced it.
func sourceServer(t *testing.T) *httptest.Server {
	t.Helper()
	idx := lexical.NewIndex([]lexical.Document{
		lexical.DocumentFromNote(vault.Parse("Notes/Fence-only.md", []byte(""+
			"# Fence only\n\n"+
			"A paragraph about something else entirely.\n\n"+
			"```d2\n"+
			"direction: right\n"+
			"Source: \"source\\nowns jobs close\"\n"+
			"```\n"))),
		lexical.DocumentFromNote(vault.Parse("Notes/Both.md", []byte(""+
			"# Both\n\n"+
			"```d2\n"+
			"direction: right\n"+
			"Source: \"source\\nowns jobs close\"\n"+
			"```\n\n"+
			"The source owns jobs after the workers close.\n"))),
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestAFenceOnlyHitSaysTheExcerptIsSource is the silence the topic label
// already ended: a fence-only row shows excerpt lines that are not a
// sentence the note wrote, and without the word on the row the reader
// sees no reason those lines are there. Both languages name it, and the
// label is never marked, because MarkHits never sees a word yomihon wrote.
func TestAFenceOnlyHitSaysTheExcerptIsSource(t *testing.T) {
	t.Parallel()
	srv := sourceServer(t)

	_, zh := getBody(t, srv.Client(), srv.URL+"/search?q=%22owns+jobs%22")
	fenceRow := resultRow(t, zh, "Notes/Fence-only.md")
	named := elementText(t, fenceRow, "y-result__source")
	if !strings.Contains(named, wording.ResultSourceLabel.In(wording.ZhHant)) {
		t.Errorf("the default page does not name the fence excerpt in Traditional Chinese: %q", named)
	}
	if strings.Contains(named, "<mark>") {
		t.Errorf("the source label was marked as a hit: %q", named)
	}

	en := getSearch(t, srv, "%22owns+jobs%22", wording.En)
	enNamed := elementText(t, resultRow(t, en, "Notes/Fence-only.md"), "y-result__source")
	if !strings.Contains(enNamed, wording.ResultSourceLabel.In(wording.En)) {
		t.Errorf("the English page does not name the fence excerpt in English: %q", enNamed)
	}
	if strings.Contains(enNamed, wording.ResultSourceLabel.In(wording.ZhHant)) {
		t.Errorf("the English page still carries the Traditional Chinese label: %q", enNamed)
	}
	if strings.Contains(enNamed, "<mark>") {
		t.Errorf("the English source label was marked as a hit: %q", enNamed)
	}

	if strings.Contains(resultRow(t, zh, "Notes/Both.md"), "y-result__source") {
		t.Errorf("a prose hit was given a source attribution:\n%s", zh)
	}

	// An English search for "source" would have marked the injected prefix
	// as a hit. The label must stay unmarked beside the note's own Source:.
	sourceEN := getSearch(t, srv, "source", wording.En)
	sourceNamed := elementText(t, resultRow(t, sourceEN, "Notes/Fence-only.md"), "y-result__source")
	if strings.Contains(sourceNamed, "<mark>") {
		t.Errorf("query source marked the evidence label: %q", sourceNamed)
	}
	if !strings.Contains(sourceNamed, wording.ResultSourceLabel.In(wording.En)) {
		t.Errorf("query source dropped the English source label: %q", sourceNamed)
	}
}
