package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// facetSite stands up the search face over a small governed vault: several
// statuses, several types, several domains, and a note declaring none of them.
func facetSite(t *testing.T) *httptest.Server {
	t.Helper()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka", NoteType: "lesson", Domain: "golang", Status: "draft", PlainText: "needle in a log"},
		{RelPath: "Writing/Streams.md", Title: "Streams", NoteType: "lesson", Domain: "golang", Status: "ready", PlainText: "needle in a stream"},
		{RelPath: "Concepts/Focus.md", Title: "Focus", NoteType: "concept", Domain: "meta", Status: "draft", PlainText: "needle and focus"},
		{RelPath: "Notes/Loose.md", Title: "Loose", PlainText: "needle with nothing declared"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestOnlyTheFaceWithAColumnAsksForOne pins the one difference between the two
// faces that ask this route for the same rows. The search page has a column
// beside its results and asks for the divisions that fill it; the command
// palette floats over a page, has nowhere to put them, and does not.
//
// This is written down because the two requests are otherwise identical, and a
// later tidying that merged them would either take the column off the page or
// start shipping it into the palette for a stylesheet to paint out — markup a
// reader's browser is handed and told to ignore.
func TestOnlyTheFaceWithAColumnAsksForOne(t *testing.T) {
	t.Parallel()
	srv := facetSite(t)

	for _, tt := range []struct {
		name string
		path string
		want bool
	}{
		{"the palette's own request carries no column", "/search/results?q=needle", false},
		{"the page's request asks for one", "/search/results?q=needle&facets=1", true},
		{"a marker that is not the marker asks for nothing", "/search/results?q=needle&facets=yes", false},
		{"the page itself always has one", "/search?q=needle", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := getBody(t, srv.Client(), srv.URL+tt.path)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if got := strings.Contains(body, "data-search-facets"); got != tt.want {
				t.Errorf("column present = %v, want %v; body = %q", got, tt.want, body)
			}
			// Both faces answer with the same rows whatever the marker says,
			// so a column that arrived by costing the palette its results
			// would be caught here rather than by a reader.
			if !strings.Contains(body, "Kafka") {
				t.Errorf("the answer lost its rows; body = %q", body)
			}
		})
	}
}

// TestEveryDivisionOfferedIsNamedInBothLanguages holds the column's headings
// against the divisions the index actually produces. A key that arrived with
// no name would draw a group headed by nothing, and the reader would be left
// to guess which field a list of values belongs to.
//
// The set comes from a real answer rather than from a list written here, so a
// fourth division added to the grammar arrives in this test on its own.
func TestEveryDivisionOfferedIsNamedInBothLanguages(t *testing.T) {
	t.Parallel()

	answer, err := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka", NoteType: "lesson", Domain: "golang", Status: "draft", PlainText: "needle"},
	}, validArtifactPolicy(t)).Search(lexical.Parse("needle"), -1)
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(answer.Facets) == 0 {
		t.Fatal("the answer divides along nothing, so this check holds nothing")
	}

	var offered []string
	for _, division := range answer.Facets {
		offered = append(offered, division.Key)
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			if wording.FacetHeading(division.Key, lang) == "" {
				t.Errorf("the division along %q has no name in %s", division.Key, lang.Tag())
			}
		}
	}
	// And the other way: a name kept for a key nothing divides along is a
	// heading no page will ever draw, left behind by a rename.
	for _, named := range wording.FacetHeadingKeys() {
		if !slices.Contains(offered, named) {
			t.Errorf("%q is named as a division and no answer divides along it", named)
		}
	}
}

// TestAFacetRowLeadsWhereItSaysItDoes pins what the handler writes into a row:
// a value not in the query narrows to it, a value already in the query leads
// back out of it, and the query the reader typed survives either way. The
// grammar's own rewriting does the work; this holds the handler to using it.
func TestAFacetRowLeadsWhereItSaysItDoes(t *testing.T) {
	t.Parallel()
	srv := facetSite(t)

	_, plain := getBody(t, srv.Client(), srv.URL+"/search?q=needle")
	for _, want := range []string{
		"q=needle+status%3Adraft",
		"q=needle+type%3Alesson",
		"q=needle+domain%3Agolang",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("no row leads to %q; body = %q", want, plain)
		}
	}

	// With the constraint already in the query, its row leads back out of it
	// and takes nothing else with it.
	_, narrowed := getBody(t, srv.Client(), srv.URL+"/search?q=needle+status%3Adraft")
	if !strings.Contains(narrowed, `href="/search?q=needle"`) {
		t.Errorf("the active row does not lead back out of the constraint; body = %q", narrowed)
	}
	if !strings.Contains(narrowed, "data-facet-active") {
		t.Errorf("the constraint in the query is not marked as standing; body = %q", narrowed)
	}
	// Narrowed to draft, every hit is at draft, so the status division holds
	// that one row and offers no sibling the grammar would read as an AND of
	// two statuses — a search nothing can answer.
	if strings.Contains(narrowed, "status%3Aready") {
		t.Errorf("the column offers a second status beside an active one, which no note can satisfy; body = %q", narrowed)
	}
}

// TestADivisionIsNotDrawnWhereItCouldNotBeHonoured pins the two states the
// column stays out of: an ungoverned folder, which has no vocabulary to divide
// by, and an answer whose metadata the artifact policy leaves unavailable,
// where every row would lead to a refusal.
func TestADivisionIsNotDrawnWhereItCouldNotBeHonoured(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka", NoteType: "lesson", Status: "draft", PlainText: "needle"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: false}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.Client(), srv.URL+"/search?q=needle")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "Kafka") {
		t.Fatalf("the ungoverned answer found nothing, so this proves nothing; body = %q", body)
	}
	if strings.Contains(body, "data-search-facets") {
		t.Errorf("an ungoverned folder drew a column of a vocabulary it does not have; body = %q", body)
	}
}
