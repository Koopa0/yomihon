package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAFenceThatNeverClosesIsStated holds what a reader of such a note is
// owed. The keys sit in the article as the prose they were read as, so the
// page says the block was meant and never closed, once, in the reader's
// language and above the prose. The fact is about the bytes, so it does not
// wait for a contract to have an opinion.
func TestAFenceThatNeverClosesIsStated(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			view := NoteView{Title: "T", RelPath: "Notes/T.md", FrontmatterUnclosed: true}
			var buf bytes.Buffer
			if err := Note(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()

			notice := wording.FrontmatterNeverCloses.In(lang)
			if got := strings.Count(html, notice); got != 1 {
				t.Errorf("the page states the fence never closes %d times, want once: %q", got, notice)
			}

			_, afterArticle, ok := strings.Cut(html, "<article ")
			if !ok {
				t.Fatal("the page has no article")
			}
			article, _, ok := strings.Cut(afterArticle, "</article>")
			if !ok {
				t.Fatal("the article is not closed")
			}
			noticeAt := strings.Index(article, notice)
			proseAt := strings.Index(article, `<div class="y-prose">`)
			if noticeAt < 0 || proseAt < 0 || noticeAt > proseAt {
				t.Errorf("the notice must sit in the article above the prose; notice@%d prose@%d", noticeAt, proseAt)
			}
		})
	}
}

// TestANoteWhoseFenceClosesCarriesNoSuchNotice is the other half: the notice is
// said for the shape that earned it and for nothing else, so a page that drew it
// everywhere would pass the test above.
func TestANoteWhoseFenceClosesCarriesNoSuchNotice(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for name, view := range map[string]NoteView{
			"a note with no frontmatter": {Title: "T", RelPath: "Notes/T.md"},
			"a draft":                    {Title: "T", RelPath: "Notes/T.md", Draft: true},
		} {
			t.Run(name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				if err := Note(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render: %v", err)
				}
				html := buf.String()
				if phrase := wording.FrontmatterNeverCloses.In(lang); strings.Contains(html, phrase) {
					t.Errorf("the page says %q of a note whose fence is not the fault", phrase)
				}
			})
		}
	}
}

// TestTheFenceSentenceIsWrittenInBothLanguages keeps the phrase from reading the
// same in both, which would be one of them left untranslated.
func TestTheFenceSentenceIsWrittenInBothLanguages(t *testing.T) {
	t.Parallel()

	for name, phrase := range map[string]wording.Phrase{
		"FrontmatterNeverCloses": wording.FrontmatterNeverCloses,
	} {
		if phrase.In(wording.ZhHant) == phrase.In(wording.En) {
			t.Errorf("%s reads the same in both languages", name)
		}
		if !strings.Contains(phrase.In(wording.ZhHant), "---") || !strings.Contains(phrase.In(wording.En), "---") {
			t.Errorf("%s does not name the fence in both languages", name)
		}
	}
}
