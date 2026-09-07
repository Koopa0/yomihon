package search

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// landingBody is the four-case note the lock names: a CJK sentence, a title
// the row can search, a phrase broken only by a wrap inside one paragraph,
// the same two words parted by a paragraph boundary, and an earlier decoy
// copy of the first of those words so a bare first-block directive would
// land on the wrong sentence. A same-block `" second"` sits after, so a
// one-character slip in the block-end test cannot hide behind the lock.
const landingBody = "" +
	"# 長篇觀察\n\n" +
	"這是試讀用的導言。\n\n" +
	"The decoy cobalt sits in the opening paragraph.\n\n" +
	"The evidence records a bright\ncrimson heron near the tower.\n\n" +
	"The field notebook calls this bird cobalt\n\n" +
	"egret beside the old lighthouse.\n\n" +
	"hello\n\n" +
	"second word here.\n"

const landingRel = "Notes/Cross paragraph.md"

const threeBlockRel = "Notes/Three blocks.md"

const threeBlockBody = "" +
	"# Three blocks\n\n" +
	"alpha\n\n" +
	"beta\n\n" +
	"gamma\n"

func landingDocs() []lexical.Document {
	return []lexical.Document{
		lexical.DocumentFromNote(vault.Parse(landingRel, []byte(landingBody))),
		lexical.DocumentFromNote(vault.Parse(threeBlockRel, []byte(threeBlockBody))),
		unlocatedCrossingDoc(),
		emptyLandingDoc(),
		lexical.DocumentFromNote(vault.Parse("Notes/nfd.md", []byte(""+
			"# NFD\n\n"+
			"caf\u0065\u0301 sits in the opening.\n\n"+
			"The evidence records a bright\n\n"+
			"crimson heron.\n"))),
	}
}

// unlocatedCrossingDoc is a phrase whose first-block stretch is only
// whitespace. The last block still holds the word, so the href can name it.
func unlocatedCrossingDoc() lexical.Document {
	plain := "abc def    \nghi"
	return lexical.Document{
		RelPath:   "Notes/Unlocated.md",
		Title:     "Unlocated",
		PlainText: plain,
		BlockEnds: []int{strings.Index(plain, "\n"), len(plain)},
	}
}

// emptyLandingDoc is a crossing whose first and last stretches collapse to
// nothing, so the row has no term a text directive can name.
func emptyLandingDoc() lexical.Document {
	plain := "   \nxxx\n   "
	return lexical.Document{
		RelPath:   "Notes/Empty landing.md",
		Title:     "Empty landing",
		PlainText: plain,
		BlockEnds: []int{3, 7, len(plain)},
	}
}

func landingIndex(t *testing.T) *lexical.Index {
	t.Helper()
	return lexical.NewIndex(landingDocs(), validArtifactPolicy(t))
}

func landingServer(t *testing.T) *httptest.Server {
	t.Helper()
	idx := landingIndex(t)
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// The four cases the lock holds together. A phrase the index accepts across
// two blocks must not hand the browser a text directive those blocks cannot
// satisfy: that directive fails silently and leaves the reader at the top.
// The first and last block of the match are still on the page, so that is
// what the href names. The other three rows keep the directive they already
// had.
func TestSearchLandingHoldsTheFourCases(t *testing.T) {
	t.Parallel()

	srv := landingServer(t)
	notePath := "/notes/Notes/Cross%20paragraph.md"
	tests := []struct {
		name   string
		query  string
		href   string
		blocks bool
	}{
		{
			name:  "plain CJK",
			query: "試讀",
			href:  notePath + "#:~:text=%E8%A9%A6%E8%AE%80",
		},
		{
			name:  "same-paragraph phrase with a line break",
			query: `"bright crimson"`,
			href:  notePath + "#:~:text=bright%20crimson",
		},
		{
			name:  "title",
			query: "長篇觀察",
			href:  notePath + "#:~:text=%E9%95%B7%E7%AF%87%E8%A7%80%E5%AF%9F",
		},
		{
			name:   "cross-paragraph phrase",
			query:  `"cobalt egret"`,
			href:   notePath + "#:~:text=cobalt,egret",
			blocks: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, body := getBody(t, srv.Client(), srv.URL+"/search/results?"+url.Values{"q": {tt.query}}.Encode())
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			got := resultHref(t, body)
			if got != tt.href {
				t.Errorf("href = %q, want %q; body = %q", got, tt.href, body)
			}
			if !tt.blocks {
				return
			}
			html := landingHTML(t)
			if !strings.Contains(html, "<p>The field notebook calls this bird cobalt</p>") {
				t.Errorf("the first word is not its own block; html = %q", html)
			}
			if !strings.Contains(html, "<p>egret beside the old lighthouse.</p>") {
				t.Errorf("the second word is not its own block; html = %q", html)
			}
			if sameBlock(html, "cobalt", "egret") {
				t.Errorf("cobalt and egret rendered in one block, so this case is not the one the lock named; html = %q", html)
			}
		})
	}
}

// A phrase that occupies three blocks must hand the browser an end term
// that lives inside the last block. If blockStartContaining always answers
// the start of the note, the end term swallows the middle block and the
// directive fails the same way a whole-phrase one did.
func TestAThreeBlockPhraseServesAnEndTermInsideTheLastBlock(t *testing.T) {
	t.Parallel()

	srv := landingServer(t)
	code, body := getBody(t, srv.Client(), srv.URL+"/search/results?"+url.Values{"q": {`"alpha beta gamma"`}}.Encode())
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	got := resultHref(t, body)
	want := "/notes/Notes/Three%20blocks.md#:~:text=alpha,gamma"
	if got != want {
		t.Errorf("href = %q, want %q; body = %q", got, want, body)
	}
	_, dir, ok := strings.Cut(got, "#:~:text=")
	if !ok {
		t.Fatalf("href carries no text directive; href = %q", got)
	}
	_, endTerm, ok := strings.Cut(dir, ",")
	if !ok {
		t.Fatalf("directive is not a range; href = %q", got)
	}
	if endTerm != "gamma" {
		t.Errorf("end term = %q, want gamma inside the last block only", endTerm)
	}
	html := threeBlockHTML(t)
	if sameBlock(html, "alpha", "gamma") || sameBlock(html, "beta", "gamma") {
		t.Errorf("gamma shares a block with an earlier term, so this is not a three-block phrase; html = %q", html)
	}
}

// A crossing match with nothing the page can name used to open the note at
// the top and say nothing. The sentence has to be reachable through the
// production mapping: BlockCrossing always arrives with a snippet, and the
// excerpt chain used to hide it.
func TestSearchResultSaysWhenACrossingMatchCannotBeLocated(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{emptyLandingDoc()}, validArtifactPolicy(t))
	q := lexical.Parse(`"  xxx  "`)
	results, _, err := idx.SearchN(q, -1)
	if err != nil {
		t.Fatalf("SearchN: %v", err)
	}
	view := viewResults(results, true, nil, q.Tokens())
	if len(view) != 1 {
		t.Fatalf("viewResults = %+v, want one row", view)
	}
	if view[0].Landing != "" || view[0].LandingEnd != "" || !view[0].BlockCrossing {
		t.Fatalf("viewResults did not produce an empty-landing crossing: %+v", view[0])
	}
	if view[0].Snippet == "" {
		t.Fatalf("snippet is empty, so the excerpt chain would not have hidden the sentence; row = %+v", view[0])
	}

	var buf bytes.Buffer
	if err := pages.SearchResults(pages.SearchView{
		Query:   `"  xxx  "`,
		Total:   1,
		Results: view,
	}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render search results: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, wording.SearchHitUnlocated.In(wording.ZhHant)) {
		t.Errorf("the row does not say the match could not be located; html = %s", html)
	}
	if strings.Contains(html, "#:~:text=") {
		t.Errorf("a crossing match with no landing still carries a text directive; html = %s", html)
	}
}

// Every served row either names a text directive the page can honour or says
// the match could not be located. The empty-landing fixture is the one that
// reaches the sentence; `" second"` is the one a flipped block-end test
// would strip of both.
func TestEverySearchResultRowLocatesOrSaysSo(t *testing.T) {
	t.Parallel()

	srv := landingServer(t)
	notePath := "/notes/Notes/Cross%20paragraph.md"
	tests := []struct {
		name string
		q    string
		href string
		note bool
	}{
		{name: "ordinary same-block phrase", q: `" second"`, href: notePath + "#:~:text=second"},
		{name: "empty first-block stretch still names the last", q: `"  ghi"`, href: "/notes/Notes/Unlocated.md#:~:text=ghi"},
		{name: "both stretches empty", q: `"  xxx  "`, note: true},
		{name: "three-block phrase", q: `"alpha beta gamma"`, href: "/notes/Notes/Three%20blocks.md#:~:text=alpha,gamma"},
		{name: "cross-paragraph with a decoy", q: `"cobalt egret"`, href: notePath + "#:~:text=cobalt,egret"},
		{name: "NFD note still lands on both blocks", q: `"bright crimson"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, body := getBody(t, srv.Client(), srv.URL+"/search/results?"+url.Values{"q": {tt.q}}.Encode())
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			assertEveryResultRowLocatesOrSaysSo(t, body)
			if tt.href != "" {
				if got := resultHref(t, body); got != tt.href {
					t.Errorf("href = %q, want %q; body = %q", got, tt.href, body)
				}
				if strings.Contains(body, wording.SearchHitUnlocated.In(wording.ZhHant)) {
					t.Errorf("a row that landed still says the match could not be located; body = %q", body)
				}
			}
			if tt.note && !strings.Contains(body, wording.SearchHitUnlocated.In(wording.ZhHant)) {
				t.Errorf("the empty-landing row does not say the match could not be located; body = %q", body)
			}
			if tt.name == "NFD note still lands on both blocks" &&
				!strings.Contains(body, "/notes/Notes/nfd.md#:~:text=bright,crimson") {
				t.Errorf("the NFD note did not keep a crossing range directive; body = %q", body)
			}
		})
	}
}

func assertEveryResultRowLocatesOrSaysSo(t *testing.T, body string) {
	t.Helper()
	const prefix = `class="y-result" href="`
	found := 0
	rest := body
	for {
		_, next, ok := strings.Cut(rest, prefix)
		if !ok {
			break
		}
		found++
		href, after, ok := strings.Cut(next, `"`)
		if !ok {
			t.Fatalf("result href is unclosed; body = %q", body)
		}
		row, _, _ := strings.Cut(after, "</a>")
		hasDirective := strings.Contains(href, "#:~:text=")
		hasNote := strings.Contains(row, wording.SearchHitUnlocated.In(wording.ZhHant))
		if !hasDirective && !hasNote {
			t.Errorf("row has neither a text directive nor the unlocated sentence; href = %q row = %q", href, row)
		}
		rest = after
	}
	if found == 0 {
		t.Fatalf("no result rows; body = %q", body)
	}
}

func resultHref(t *testing.T, body string) string {
	t.Helper()
	const prefix = `class="y-result" href="`
	_, rest, ok := strings.Cut(body, prefix)
	if !ok {
		t.Fatalf("no result href; body = %q", body)
	}
	href, _, ok := strings.Cut(rest, `"`)
	if !ok {
		t.Fatalf("result href is unclosed; body = %q", body)
	}
	return href
}

func landingHTML(t *testing.T) string {
	t.Helper()
	return renderNoteHTML(t, landingRel, "Cross paragraph", landingBody)
}

func threeBlockHTML(t *testing.T) string {
	t.Helper()
	return renderNoteHTML(t, threeBlockRel, "Three blocks", threeBlockBody)
}

func renderNoteHTML(t *testing.T, rel, title, body string) string {
	t.Helper()
	page := render.New(graph.BuildFromNotes(nil, nil), landingBodies{}, landingTitles{}, landingFiles{}).
		HTML(rel, title, body, wording.ZhHant)
	if page.HTML == "" {
		t.Fatal("the note rendered no body, so the block check would pass over nothing")
	}
	return page.HTML
}

func sameBlock(html, a, b string) bool {
	for _, tag := range []string{"p", "li", "h1", "h2", "h3", "h4"} {
		open, end := "<"+tag+">", "</"+tag+">"
		for rest := html; ; {
			i := strings.Index(rest, open)
			if i < 0 {
				break
			}
			rest = rest[i+len(open):]
			j := strings.Index(rest, end)
			if j < 0 {
				return false
			}
			inner := rest[:j]
			if strings.Contains(inner, a) && strings.Contains(inner, b) {
				return true
			}
			rest = rest[j+len(end):]
		}
	}
	return false
}

type landingBodies struct{}

func (landingBodies) Transclusion(string) (string, bool) { return "", false }

type landingTitles struct{}

func (landingTitles) TitledBy(string) []string { return nil }

type landingFiles struct{}

func (landingFiles) MissingFile(string) bool { return false }
