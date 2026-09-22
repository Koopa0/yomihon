package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/wording"
)

// comparedNote is one column's worth of note: enough of a view that the article
// draws its head, the schema's verdict on it, the aids the narrow layout folds
// inline, the body and the way onward — with the id space a column occupies
// already stamped on everything that carries one, as the handler stamps it.
func comparedNote(prefix, title, relPath, language string) NoteView {
	return NoteView{
		Title:    title,
		RelPath:  relPath,
		Language: language,
		Type:     "lesson",
		Status:   "draft",
		Governed: true,
		IDPrefix: prefix,
		BodyHTML: `<h2 id="` + prefix + `ledger" data-level="1">Ledger</h2><p><a href="#` + prefix + `ledger">back up</a></p>`,
		TOC:      []render.TOCEntry{{Level: 1, Text: "Ledger", ID: prefix + "ledger"}},
		SchemaNotices: [][]wording.SchemaPart{
			{{Text: "the contract does not know "}, {Text: "unknown_key", Code: true}},
		},
		BasedOn:     []snapshot.DeclaredSource{{Name: "Source", RelPath: "Writing/Source.md"}},
		Pair:        nav.NoteRef{Name: "The other half", RelPath: "Writing/Other.md"},
		Next:        nav.NoteRef{Name: "Next one", RelPath: "Writing/Next.md"},
		StepsLabel:  "接著讀",
		Transitions: []Transition{{To: "ready"}},
	}
}

// TestACompareColumnIsTheNotesOwnArticle is the promise the page makes: a
// reader holding two notes side by side is reading each of those notes, not a
// second rendering of them that is free to drift. Each column is compared
// against the article component rendered on its own in the id space that column
// occupies, so the one thing the two uses differ by is the write face — which a
// column drops structurally, by passing nothing where the note's own page
// passes the status bar in.
//
// The article counts are asserted first: without them a comparison of two empty
// strings would pass while the page drew nothing at all.
func TestACompareColumnIsTheNotesOwnArticle(t *testing.T) {
	t.Parallel()
	lang := recordedChrome().Lang
	a := comparedNote("a-", "Cutover", "Writing/Cutover.md", "en")
	b := comparedNote("b-", "切換", "Writing/Cutoverzh.md", "zh-Hant")

	page := renderedBytes(t, t.Context(), Note(a, recordedChrome()))
	if got := strings.Count(page, articleOpening); got != 1 {
		t.Fatalf("the note page draws %d articles, want 1", got)
	}
	compared := renderedBytes(t, t.Context(), Compare(CompareView{A: a, B: b}, recordedChrome()))
	if got := strings.Count(compared, articleOpening); got != 2 {
		t.Fatalf("the side-by-side page draws %d articles, want one per note", got)
	}

	alone := renderedBytes(t, templ.WithChildren(t.Context(), statusBar(a, lang)), noteArticle(a, lang))
	if diff := cmp.Diff(alone, articleAt(t, page, 0)); diff != "" {
		t.Errorf("the note page's article is not the component it draws (-component +page):\n%s", diff)
	}
	for ordinal, column := range []NoteView{a, b} {
		want := renderedBytes(t, t.Context(), noteArticle(column, lang))
		if diff := cmp.Diff(want, articleAt(t, compared, ordinal)); diff != "" {
			t.Errorf("column %d is not %s read alone (-component +column):\n%s", ordinal, column.Title, diff)
		}
	}
}

// TestACompareColumnCarriesNoWriteFace is the other half of the same promise,
// asserted on the words rather than on the shape: adjudication happens on a
// note's own page, and the status word here is shown, not offered.
func TestACompareColumnCarriesNoWriteFace(t *testing.T) {
	t.Parallel()
	a := comparedNote("a-", "Cutover", "Writing/Cutover.md", "en")
	b := comparedNote("b-", "切換", "Writing/Cutoverzh.md", "zh-Hant")
	compared := renderedBytes(t, t.Context(), Compare(CompareView{A: a, B: b}, recordedChrome()))
	if strings.Contains(compared, `action="/status"`) {
		t.Error("a column offers a control that would write a status")
	}
	if got := strings.Count(compared, `class="y-sealbar"`); got != 0 {
		t.Errorf("the page draws %d status bars, want none", got)
	}
}

const articleOpening = `<article class="y-article"`

func renderedBytes(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// articleAt cuts the article at the given position out of a rendered page.
// Articles do not nest here, so the close that ends one is the first one after
// it opens.
func articleAt(t *testing.T, page string, ordinal int) string {
	t.Helper()
	rest := page
	for range ordinal {
		at := strings.Index(rest, articleOpening)
		if at < 0 {
			t.Fatalf("the page holds no article at position %d", ordinal)
		}
		rest = rest[at+len(articleOpening):]
	}
	at := strings.Index(rest, articleOpening)
	if at < 0 {
		t.Fatalf("the page holds no article at position %d", ordinal)
	}
	rest = rest[at:]
	end := strings.Index(rest, "</article>")
	if end < 0 {
		t.Fatalf("an article on the page never closes")
	}
	return rest[:end+len("</article>")]
}
