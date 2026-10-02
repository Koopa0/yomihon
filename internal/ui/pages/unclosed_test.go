package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestAFenceThatNeverClosesIsStatedAndNotCalledLegal holds what a reader of such
// a note is owed. The keys sit in the article as the prose they were read as,
// so the page says the block was meant and never closed, in the reader's
// language and above the prose. And the status face does not answer that the
// note has no frontmatter, which says it is legal: that is the sentence this
// shape used to get, and it is the one that told the author nothing was wrong.
//
// Both languages are walked, and both faces, because only one face is present at
// a given width.
func TestAFenceThatNeverClosesIsStatedAndNotCalledLegal(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, tt := range []struct {
			name string
			view NoteView
			// faces is how many status faces say the fence never closes: the rail
			// panel and the foot bar, or neither where the folder governs nothing.
			faces int
		}{
			{
				name:  "a governed note",
				view:  NoteView{Governed: true, Title: "T", RelPath: "Notes/T.md", FrontmatterUnclosed: true},
				faces: 2,
			},
			{
				// The reading-side view of what the parser saw: no block, so the
				// no-frontmatter flag is true here too, and the fence still wins.
				name:  "a governed note the readers took for one with no block",
				view:  NoteView{Governed: true, Title: "T", RelPath: "Notes/T.md", NoFrontmatter: true, FrontmatterUnclosed: true},
				faces: 2,
			},
			{
				// A contract that requires a block gives the absent block a sentence
				// of its own, and a fence that never closes is not that either.
				name:  "a governed note under a contract that requires a block",
				view:  NoteView{Governed: true, Title: "T", RelPath: "Notes/T.md", NoFrontmatter: true, FrontmatterRequired: true, FrontmatterUnclosed: true},
				faces: 2,
			},
			{
				// The fact is about the bytes, so it does not wait for a contract
				// to have an opinion.
				name:  "a folder nothing governs",
				view:  NoteView{Title: "T", RelPath: "Notes/T.md", FrontmatterUnclosed: true},
				faces: 0,
			},
		} {
			t.Run(tt.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				if err := Note(tt.view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render: %v", err)
				}
				html := buf.String()

				notice := wording.FrontmatterNeverCloses.In(lang)
				if got := strings.Count(html, notice); got != 1 {
					t.Errorf("the page states the fence never closes %d times, want once: %q", got, notice)
				}
				if got := strings.Count(html, wording.StatusFrontmatterNeverCloses.In(lang)); got != tt.faces {
					t.Errorf("the status faces state it %d times, want %d", got, tt.faces)
				}
				for _, claim := range []wording.Lang{wording.ZhHant, wording.En} {
					for _, absent := range []wording.Phrase{wording.NoFrontmatter, wording.NoFrontmatterRequired} {
						if strings.Contains(html, absent.In(claim)) {
							t.Errorf("the page calls a note whose fence never closes frontmatter-free: %q", absent.In(claim))
						}
					}
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
}

// TestANoteWhoseFenceClosesCarriesNoSuchNotice is the other half: the notice and
// the status sentence are said for the shape that earned them and for nothing
// else, so a page that drew them everywhere would pass the test above.
func TestANoteWhoseFenceClosesCarriesNoSuchNotice(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for name, view := range map[string]NoteView{
			"a note with no frontmatter": {Governed: true, Title: "T", RelPath: "Notes/T.md", NoFrontmatter: true},
			"a note with a status":       {Governed: true, Title: "T", RelPath: "Notes/T.md", Status: "draft"},
		} {
			t.Run(name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				if err := Note(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render: %v", err)
				}
				html := buf.String()
				for _, phrase := range []wording.Phrase{wording.FrontmatterNeverCloses, wording.StatusFrontmatterNeverCloses} {
					if strings.Contains(html, phrase.In(lang)) {
						t.Errorf("the page says %q of a note whose fence is not the fault", phrase.In(lang))
					}
				}
			})
		}
	}
}

// TestTheFenceSentencesAreWrittenInBothLanguages keeps the two new phrases from
// reading the same in both, which would be one of them left untranslated.
func TestTheFenceSentencesAreWrittenInBothLanguages(t *testing.T) {
	t.Parallel()

	for name, phrase := range map[string]wording.Phrase{
		"FrontmatterNeverCloses":       wording.FrontmatterNeverCloses,
		"StatusFrontmatterNeverCloses": wording.StatusFrontmatterNeverCloses,
	} {
		if phrase.In(wording.ZhHant) == phrase.In(wording.En) {
			t.Errorf("%s reads the same in both languages", name)
		}
		if !strings.Contains(phrase.In(wording.ZhHant), "---") || !strings.Contains(phrase.In(wording.En), "---") {
			t.Errorf("%s does not name the fence in both languages", name)
		}
	}
}
