package search

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// unknownFilterServer answers over one note whose words are reachable both as
// plain text and through the path, so a query that fails as a filter can still
// be seen succeeding as text.
func unknownFilterServer(t *testing.T) *httptest.Server {
	t.Helper()
	idx := lexical.NewIndex([]lexical.Document{
		// The body carries the query verbatim, so "the literal search ran" can
		// be told apart from "the literal search ran and this corpus has
		// nothing". Without a note that really contains the characters, an
		// empty result proves neither.
		{RelPath: "Notes/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Status: "draft",
			PlainText: "kafka is a distributed log, written up as path:Notes in the index"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestAQueryShapedLikeAFilterIsNotSilentlyReadAsText covers the failure a
// reader cannot see: a word before a colon that this grammar does not know is
// searched for as text, and the page that comes back is the page a genuinely
// empty search returns. The reader is left believing their filter ran.
func TestAQueryShapedLikeAFilterIsNotSilentlyReadAsText(t *testing.T) {
	t.Parallel()
	srv := unknownFilterServer(t)

	code, body := getBody(t, srv.Client(), srv.URL+"/search?q=path%3ANotes")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "path") {
		t.Errorf("the page does not name the key it did not recognise:\n%s", body)
	}
	if !strings.Contains(body, "data-unknown-filter") {
		t.Errorf("a query shaped like a filter was answered with no word about it:\n%s", body)
	}
	// The literal search still ran: this note's body carries those exact
	// characters, so a hit is proof the term was searched for rather than
	// refused.
	if !strings.Contains(body, "Kafka Basics") {
		t.Errorf("the literal search did not run, so the reader got a refusal instead of results:\n%s", body)
	}
	// Every key the grammar does know is offered, so the reader can repair it.
	for _, key := range lexical.FilterKeys() {
		if !strings.Contains(body, key+":") {
			t.Errorf("the hint does not offer %q, so a reader cannot see what to write instead", key)
		}
	}

	// A recognised filter is untouched.
	if _, known := getBody(t, srv.Client(), srv.URL+"/search?q=type%3Alesson"); strings.Contains(known, "data-unknown-filter") {
		t.Errorf("a filter this grammar knows was reported as unknown:\n%s", known)
	}
	// A genuinely empty result is not dressed up as a syntax problem.
	if _, none := getBody(t, srv.Client(), srv.URL+"/search?q=nothingmatchesthis"); strings.Contains(none, "data-unknown-filter") {
		t.Errorf("a search that simply found nothing was blamed on its syntax:\n%s", none)
	}
	// Quoting is the grammar's own way of asking for characters, so it is not
	// second-guessed: the reader already said they meant text.
	if _, quoted := getBody(t, srv.Client(), srv.URL+"/search?q=%22path%3ANotes%22"); strings.Contains(quoted, "data-unknown-filter") {
		t.Errorf("a quoted term was treated as a mistyped filter:\n%s", quoted)
	}
}

// TestTheBlankSearchPageSaysWhatItUnderstands covers the moment a reader has
// nothing to go on. The field accepts constraints that are discoverable
// nowhere else, and a blank page is the one time naming them costs nobody an
// answer.
func TestTheBlankSearchPageSaysWhatItUnderstands(t *testing.T) {
	t.Parallel()
	srv := unknownFilterServer(t)

	_, blank := getBody(t, srv.Client(), srv.URL+"/search")
	if !strings.Contains(blank, "data-search-filters") {
		t.Errorf("the blank search page offers nothing to go on:\n%s", blank)
	}
	for _, key := range lexical.FilterKeys() {
		if !strings.Contains(blank, key+":") {
			t.Errorf("the blank page does not name the %q filter", key)
		}
	}

	// The control: once a query has been asked, the answer is what the page is
	// for, and the list would stand between the reader and it.
	if _, answered := getBody(t, srv.Client(), srv.URL+"/search?q=kafka"); strings.Contains(answered, "data-search-filters") {
		t.Errorf("the filter list is still shown over an answered query:\n%s", answered)
	}
}

// TestTheUnknownFilterHintSpeaksTheReadersLanguage holds the new sentences to
// the rule the rest of the interface follows. A phrase existing in both
// languages is checked where the phrases are written; what is checked here is
// that this page reaches for the reader's own, which is a different claim and
// the one a reader would notice failing.
func TestTheUnknownFilterHintSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()
	srv := unknownFilterServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/search?q=path%3ANotes", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	// Set as a header rather than built as a cookie value: only the name and
	// the value cross the wire on a request, and the attributes a cookie type
	// carries belong to the server that sets one.
	req.Header.Set("Cookie", wording.CookieName+"="+string(wording.En))
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	page := string(body)

	if !strings.Contains(page, "is not a filter yomihon knows") {
		t.Errorf("the hint did not reach the reader's language:\n%s", page)
	}
	if strings.Contains(page, "不是 yomihon 認得的篩選器") {
		t.Errorf("the page carries both languages at once:\n%s", page)
	}
}
