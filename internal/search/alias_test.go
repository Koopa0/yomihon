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

// aliasServer answers over notes whose aliases are the only place a term
// appears, so a hit proves the alias was indexed and nothing else could have
// produced it.
func aliasServer(t *testing.T) *httptest.Server {
	t.Helper()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Concepts/Goroutine.md", Title: "Goroutine", NoteType: "concept", Draft: true,
			Aliases: []string{"green thread"}, PlainText: "a unit of concurrency"},
		{RelPath: "Concepts/Pointer.md", Title: "Pointer", NoteType: "concept", Draft: true,
			Aliases: []string{"指標"}, PlainText: "an address held as a value"},
		{RelPath: "Concepts/Kana.md", Title: "Kana", NoteType: "concept", Draft: true,
			Aliases: []string{"かな"}, PlainText: "a syllabary"},
		{RelPath: "Concepts/Plain.md", Title: "Plain", NoteType: "concept", Draft: true,
			PlainText: "no alias at all"},
		// Two names, and the query names the second: with one alias apiece,
		// "the name that matched" and "the first name" are the same string,
		// and a row that always showed the first would look correct.
		{RelPath: "Concepts/Several.md", Title: "Several", NoteType: "concept", Draft: true,
			Aliases: []string{"first name", "second name"}, PlainText: "unrelated"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestANoteIsFoundByTheNamesItIsAlsoKnownBy closes the gap between what this
// program follows and what it finds. A wikilink written to an alias resolves,
// because an alias is one of the names a note answers to; a search for that
// same name found nothing, so the two faces disagreed about what a note is
// called.
func TestANoteIsFoundByTheNamesItIsAlsoKnownBy(t *testing.T) {
	t.Parallel()
	srv := aliasServer(t)

	for _, c := range []struct{ name, query, want string }{
		{"an alias in ASCII", "green+thread", "Concepts/Goroutine.md"},
		{"an alias in Han characters", "%E6%8C%87%E6%A8%99", "Concepts/Pointer.md"},
		{"an alias in kana", "%E3%81%8B%E3%81%AA", "Concepts/Kana.md"},
	} {
		code, body := getBody(t, srv.Client(), srv.URL+"/search?q="+c.query)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d", c.name, code)
		}
		if !linksToNote(body, c.want) {
			t.Errorf("%s: the note is not found by a name it answers to:\n%s", c.name, body)
		}
	}
}

// TestAnAliasHitSaysWhichNameAnsweredTheQuery is the other half. The row shows
// a note's title, and an alias is not the title — so a hit reached through one
// looks like a hit on words the reader cannot find anywhere on the row, which
// is the same silence a path hit used to have.
func TestAnAliasHitSaysWhichNameAnsweredTheQuery(t *testing.T) {
	t.Parallel()
	srv := aliasServer(t)

	_, body := getBody(t, srv.Client(), srv.URL+"/search?q=green+thread")
	row := resultRow(t, body, "Concepts/Goroutine.md")

	// The name is shown inside its own element, with the matched stretches
	// marked — so the words arrive cut into runs rather than as one string,
	// and the assertion reads the element rather than looking for the alias
	// spelled out whole.
	named := elementText(t, row, "y-result__alias")
	for _, want := range []string{"green", "thread"} {
		if !strings.Contains(named, want) {
			t.Errorf("the name that answered the query is missing %q from the row: %q", want, named)
		}
	}
	if !strings.Contains(named, "<mark>") {
		t.Errorf("the row shows the name but never says which part the query met: %q", named)
	}
	if strings.Contains(row, `y-result__title">Goroutine`) && !strings.Contains(named, "green") {
		t.Errorf("the row shows only the title, which is not what was searched for: %q", row)
	}

	// A note known by several names says the one that answered, not the one
	// its author happened to list first.
	_, several := getBody(t, srv.Client(), srv.URL+"/search?q=second+name")
	answering := elementText(t, resultRow(t, several, "Concepts/Several.md"), "y-result__alias")
	if !strings.Contains(answering, "second") {
		t.Errorf("the row does not name the alias that matched: %q", answering)
	}
	if strings.Contains(answering, "first") {
		t.Errorf("the row names an alias the query never touched: %q", answering)
	}

	// The control: a note found by its title needs no such attribution, and
	// giving one to every row would make the mark mean nothing.
	_, plain := getBody(t, srv.Client(), srv.URL+"/search?q=Plain")
	if strings.Contains(resultRow(t, plain, "Concepts/Plain.md"), "y-result__alias") {
		t.Errorf("a note found by its own title was given an alias attribution:\n%s", plain)
	}
}

// TestAnAliasHitRanksWithTheTitleItStandsFor holds the ladder. An alias is one
// of the names a link resolves by — the title is not — so a note reached
// through an alias has been named as directly as one reached through its
// title, and ranking it below a body mention would put a passing sentence
// above the note the reader named.
func TestAnAliasHitRanksWithTheTitleItStandsFor(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Concepts/Mentions.md", Title: "Mentions", NoteType: "concept", Draft: true,
			PlainText: "this body happens to say green thread in passing"},
		{RelPath: "Concepts/Named.md", Title: "Named", NoteType: "concept", Draft: true,
			Aliases: []string{"green thread"}, PlainText: "unrelated words"},
	}, validArtifactPolicy(t))

	answer, err := idx.Search(lexical.Parse("green thread"), -1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(answer.Results) != 2 {
		t.Fatalf("want both notes, got %d: %+v", len(answer.Results), answer.Results)
	}
	if answer.Results[0].RelPath != "Concepts/Named.md" {
		t.Errorf("the note that answers to the name ranked below one that mentions it: %+v", answer.Results)
	}
}

// elementText cuts one span out of a row by its class, so an assertion about
// the alias cannot be satisfied by a mark somewhere else on the row.
func elementText(t *testing.T, row, class string) string {
	t.Helper()
	at := strings.Index(row, `class="`+class+`"`)
	if at < 0 {
		t.Fatalf("the row carries no %s element: %q", class, row)
	}
	end := strings.Index(row[at:], "</span>")
	if end < 0 {
		t.Fatalf("the %s element is never closed: %q", class, row)
	}
	return row[at : at+end]
}

func getSearch(t *testing.T, srv *httptest.Server, query string, lang wording.Lang) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/search?q="+query, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if lang == wording.En {
		req.Header.Set("Cookie", wording.CookieName+"="+string(wording.En))
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// TestAnAliasLabelIsSaidInTheReadersLanguage holds the word that introduces
// the answering name to the page's language: a label written in one would
// leave the other reader looking at a title that does not match.
func TestAnAliasLabelIsSaidInTheReadersLanguage(t *testing.T) {
	t.Parallel()
	srv := aliasServer(t)

	zh := elementText(t, resultRow(t, getSearch(t, srv, "green+thread", wording.ZhHant), "Concepts/Goroutine.md"), "y-result__alias")
	if !strings.Contains(zh, wording.ResultAliasLabel.In(wording.ZhHant)) {
		t.Errorf("the default page does not introduce the name in Traditional Chinese: %q", zh)
	}
	en := elementText(t, resultRow(t, getSearch(t, srv, "green+thread", wording.En), "Concepts/Goroutine.md"), "y-result__alias")
	if !strings.Contains(en, wording.ResultAliasLabel.In(wording.En)) {
		t.Errorf("the English page does not introduce the name in English: %q", en)
	}
	if strings.Contains(en, wording.ResultAliasLabel.In(wording.ZhHant)) {
		t.Errorf("the English page still carries the Traditional Chinese label: %q", en)
	}
}

// TestAnAliasLabelKeepsTheAuthorsSpelling is the display half of the fold.
// Matching reads the folded alias, so a lowercase query finds the note; the
// row must still print the name the way its author wrote it.
func TestAnAliasLabelKeepsTheAuthorsSpelling(t *testing.T) {
	t.Parallel()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Notes/Palette.md", Title: "Palette", NoteType: "concept", Draft: true,
			Aliases: []string{"Colour Theory"}, PlainText: "unrelated prose"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	_, body := getBody(t, srv.Client(), srv.URL+"/search?q=colour+theory")
	named := elementText(t, resultRow(t, body, "Notes/Palette.md"), "y-result__alias")
	if !strings.Contains(named, "Colour") || !strings.Contains(named, "Theory") {
		t.Errorf("the row does not show Colour Theory as written: %q", named)
	}
	if strings.Contains(named, "colour theory") {
		t.Errorf("the label printed the fold, not the author's spelling: %q", named)
	}
}
