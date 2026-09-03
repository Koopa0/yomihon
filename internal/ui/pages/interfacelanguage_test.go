package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestChromeSpeaksTheChosenLanguage walks the copy that has moved into the
// dictionary through the two pipelines that carry it into a page: the shared
// chrome, and a page of its own. Each expectation is the dictionary's own value
// rather than a retyped string, so the copy has one source and the test cannot
// drift from it — what it holds is that the page reached for the language the
// request asked for.
func TestChromeSpeaksTheChosenLanguage(t *testing.T) {
	t.Parallel()

	// Without this the whole table is satisfiable by a Phrase that always
	// answers in one language, which is the failure most worth catching.
	for name, phrase := range map[string]wording.Phrase{
		"theme toggle":     wording.ThemeToggle,
		"not-found title":  wording.NothingHere,
		"freshness notice": wording.FreshnessNewVersion,
		"raw-file link":    wording.RawFile,
		"rail heading":     wording.Folders,
	} {
		if phrase.In(wording.ZhHant) == phrase.In(wording.En) {
			t.Fatalf("the %s reads the same in both languages, so no assertion below can tell them apart", name)
		}
	}

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			chrome := layouts.Chrome{Lang: lang}

			var notFound bytes.Buffer
			if err := NotFound(NotFoundView{Asked: "/notes/x.md"}, chrome).Render(t.Context(), &notFound); err != nil {
				t.Fatalf("render not-found: %v", err)
			}
			page := notFound.String()
			if !strings.Contains(page, `<html lang="`+lang.Tag()+`"`) {
				t.Errorf("the document does not declare %q; page = %q", lang.Tag(), page)
			}
			if !strings.Contains(page, wording.ThemeToggle.In(lang)) {
				t.Errorf("the shared chrome is not speaking %q: want %q in the page", lang, wording.ThemeToggle.In(lang))
			}
			if !strings.Contains(page, wording.NothingHere.In(lang)) {
				t.Errorf("the not-found page is not speaking %q: want %q in the page", lang, wording.NothingHere.In(lang))
			}

			// The freshness notice is the server's too: the script that shows it
			// reads the sentence off the page rather than carrying its own copy.
			var note bytes.Buffer
			view := NoteView{RelPath: "Writing/n.md", ContentIdentity: "abc"}
			if err := Note(view, chrome).Render(t.Context(), &note); err != nil {
				t.Fatalf("render note: %v", err)
			}
			if !strings.Contains(note.String(), wording.FreshnessNewVersion.In(lang)) {
				t.Errorf("the freshness notice does not travel to the page in %q: want %q", lang, wording.FreshnessNewVersion.In(lang))
			}
			// The reading page's own furniture and the rail it shares with every
			// other page both take the language from the chrome. The view used
			// to carry a second copy, which is how a page could have rendered
			// an English frame around Chinese words with nothing failing.
			if !strings.Contains(note.String(), wording.RawFile.In(lang)) {
				t.Errorf("the reading page is not speaking %q: want %q", lang, wording.RawFile.In(lang))
			}
			var rail bytes.Buffer
			if err := sidebar(NewSidebar(nil, ""), chrome).Render(t.Context(), &rail); err != nil {
				t.Fatalf("render rail: %v", err)
			}
			if !strings.Contains(rail.String(), wording.FilterNavigation.In(lang)) {
				t.Errorf("the rail is not speaking %q: want %q", lang, wording.FilterNavigation.In(lang))
			}
		})
	}
}
