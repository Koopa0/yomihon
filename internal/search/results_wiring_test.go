package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// The count and the marked terms are both things a reader looked for and did
// not find: four asked how many results there were, five asked why a given
// result was in the list. Both are asserted on the served page rather than on
// the functions that produce them, because a correct function nothing calls
// reaches nobody — the first version of the marking was wired through a field
// the page never read, and every unit test still passed.
func TestServedResultsCarryTheCountAndTheMarks(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "notes/Kafka.md", Title: "Kafka Basics", PlainText: "kafka is a distributed log"},
		{RelPath: "notes/Streams.md", Title: "Streams", PlainText: "kafka streams build on the log"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.URL+"/search?q=kafka")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// Rendered by the server, so it is there for a reader with no scripting at
	// all — where it used to live, in the live region, it was there for neither.
	if !strings.Contains(body, "共 2 筆") {
		t.Errorf("results page does not say how many were found; body = %q", body)
	}
	if n := strings.Count(body, "<mark>kafka</mark>"); n == 0 {
		t.Errorf("results page marks none of the searched term; body = %q", body)
	}
	// The mark must not have eaten or duplicated the note's own words.
	if !strings.Contains(body, "is a distributed log") {
		t.Errorf("results page lost snippet text around the mark; body = %q", body)
	}

	// The fragment the live search replaces the list with has to agree with the
	// page, or turning JavaScript on quietly removes both.
	fragCode, frag := getBody(t, srv.URL+"/search/results?q=kafka")
	if fragCode != http.StatusOK {
		t.Fatalf("fragment status = %d, want 200", fragCode)
	}
	for _, want := range []string{"共 2 筆", "<mark>kafka</mark>"} {
		if !strings.Contains(frag, want) {
			t.Errorf("live-search fragment missing %q; body = %q", want, frag)
		}
	}
}

// The loosened offers appear exactly where the reader hits the wall — the
// empty answer — and nowhere else. A page with results carrying "step back"
// advice would be the page second-guessing an answer it just gave.
func TestServedStepBacksAppearOnlyOnTheEmptyAnswer(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "臨床/利尿劑調整.md", Title: "利尿劑", PlainText: "Furosemide 起始 20-40mg"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.URL+"/search?q=20mg")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "退一步找") || !strings.Contains(body, "20+mg") {
		t.Errorf("empty answer does not offer the loosened search; body = %q", body)
	}

	// This query has results AND would generate a loosened candidate — the
	// only shape that can tell "the gate held" apart from "there was nothing
	// to offer anyway".
	code, body = getBody(t, srv.URL+"/search?q=20-40mg")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "共 1 筆") {
		t.Fatalf("the with-results probe found nothing, so it can prove nothing; body = %q", body)
	}
	if strings.Contains(body, "退一步找") {
		t.Errorf("an answer with results still carries step-back advice; body = %q", body)
	}
}
