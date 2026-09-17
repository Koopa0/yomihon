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
		Blocks: []render.Block{
			{End: strings.Index(plain, "\n"), Verbatim: true},
			{End: len(plain), Verbatim: true},
		},
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
		Blocks: []render.Block{
			{End: 3, Verbatim: true},
			{End: 7, Verbatim: true},
			{End: len(plain), Verbatim: true},
		},
	}
}

func landingIndex(t *testing.T) *lexical.Index {
	t.Helper()
	return lexical.NewIndex(landingDocs(), validArtifactPolicy(t))
}

func landingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return serverForIndex(t, landingIndex(t))
}

func serverForIndex(t *testing.T, idx *lexical.Index) *httptest.Server {
	t.Helper()
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
			href:  notePath + "#:~:text=%E9%80%99%E6%98%AF-,%E8%A9%A6%E8%AE%80",
		},
		{
			name:  "same-paragraph phrase with a line break",
			query: `"bright crimson"`,
			href:  notePath + "#:~:text=evidence%20records%20a-,bright%20crimson",
		},
		{
			name:  "title",
			query: "長篇觀察",
			href:  notePath + "#:~:text=%E9%95%B7%E7%AF%87%E8%A7%80%E5%AF%9F",
		},
		{
			name:   "cross-paragraph phrase",
			query:  `"cobalt egret"`,
			href:   notePath + "#:~:text=calls%20this%20bird-,cobalt,egret",
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

// renderedOrderBody puts one searchable word in each of three kinds of block:
// an ordinary paragraph, a sentence a ruby reading was taken out of, and a
// paragraph the page gives a footnote's mark. The words are distinct so each
// row is answered by the block it was written for.
const renderedOrderRel = "Notes/Rendered order.md"

const renderedOrderBody = "" +
	"The ledger entry closes with lanthanum here.\n\n" +
	"<ruby>今日<rt>きょう</rt></ruby>は晴れ、cobaltine が続く。\n\n" +
	"The survey recorded its own[^survey] zenithal reading.\n\n" +
	"[^survey]: The reading the survey kept for itself.\n"

// Naming the words a match follows asks a browser to find that run and the
// match side by side in what it is showing. Two of this note's blocks are not
// shown the way the searchable text carries them — the reading is written
// after the sentence it is spoken inside, and the mark on the page is in no
// text here — so on those the request would match nothing and the note would
// open at the top, which is worse than the bare term, and they keep it. The
// ordinary paragraph beside them is what shows the run is still being named
// where it can be: a walk that had stopped vouching for anything would pass
// the other two rows and fail this one.
func TestOnlyABlockThePageReproducesNamesWhatAMatchFollows(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		lexical.DocumentFromNote(vault.Parse(renderedOrderRel, []byte(renderedOrderBody))),
	}, validArtifactPolicy(t))
	srv := serverForIndex(t, idx)
	notePath := "/notes/Notes/Rendered%20order.md"
	tests := []struct {
		name  string
		query string
		href  string
	}{
		{
			name:  "an ordinary paragraph names what the match follows",
			query: "lanthanum",
			href:  notePath + "#:~:text=entry%20closes%20with-,lanthanum",
		},
		{
			name:  "the sentence a ruby reading was taken out of keeps the bare term",
			query: "cobaltine",
			href:  notePath + "#:~:text=cobaltine",
		},
		{
			name:  "the paragraph carrying a footnote reference keeps the bare term",
			query: "zenithal",
			href:  notePath + "#:~:text=zenithal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, body := getBody(t, srv.Client(), srv.URL+"/search/results?"+url.Values{"q": {tt.query}}.Encode())
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if got := resultHref(t, body); got != tt.href {
				t.Errorf("href = %q, want %q; body = %q", got, tt.href, body)
			}
		})
	}
}

// A reader who types part of a word matches part of it, and a term a
// directive carries alone is looked for only where a word begins and where
// one ends — so the tail of a word used to be a request the page could not
// answer, and the note opened at the top with nothing said. The term goes out
// grown to the whole word instead.
//
// The three rows are the three ways the block decides. Where the page shows
// the block as the searchable text carries it, the run of words ahead of the
// match takes that rule off the term and the match itself is still named, byte
// for byte as before. Where it does not, the term stands alone and is grown.
// Where the words are not parted by spaces, the edges of one are a
// segmentation nothing here can find, so the stretch is served as it stands
// rather than grown to the sentence around it — which is a string the page,
// drawing a ruby reading in among the characters it is spoken over, does not
// carry in one piece anyway.
func TestAMatchOpeningInsideAWordIsServedAsTheWholeWord(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		lexical.DocumentFromNote(vault.Parse(renderedOrderRel, []byte(renderedOrderBody))),
	}, validArtifactPolicy(t))
	srv := serverForIndex(t, idx)
	notePath := "/notes/Notes/Rendered%20order.md"
	tests := []struct {
		name  string
		query string
		href  string
	}{
		{
			name:  "a term standing alone is grown to its whole word",
			query: "baltine",
			href:  notePath + "#:~:text=cobaltine",
		},
		{
			name:  "a term a run of words introduces still names the match itself",
			query: "nthanum",
			href:  notePath + "#:~:text=closes%20with%20la-,nthanum",
		},
		{
			name:  "a script that parts no words with spaces is served as it stands",
			query: "晴れ",
			href:  notePath + "#:~:text=%E6%99%B4%E3%82%8C",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, body := getBody(t, srv.Client(), srv.URL+"/search/results?"+url.Values{"q": {tt.query}}.Encode())
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if got := resultHref(t, body); got != tt.href {
				t.Errorf("href = %q, want %q; body = %q", got, tt.href, body)
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
		{name: "cross-paragraph with a decoy", q: `"cobalt egret"`, href: notePath + "#:~:text=calls%20this%20bird-,cobalt,egret"},
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
				!strings.Contains(body, "/notes/Notes/nfd.md#:~:text=evidence%20records%20a-,bright,crimson") {
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
