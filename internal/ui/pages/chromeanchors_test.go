package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestChromeAnchorsStayOutsideEveryAuthoredHeading(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(lang.Tag(), func(t *testing.T) {
			t.Parallel()
			contract := rejectedCapabilityContract(t)
			faulted := nav.New(nil, nil, graph.BuildFromNotes(nil, nil), contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(), contract.JournalDir(), contract.ArticleLanguage(), contract.AuthoredDate(), contract.Settlement(), nil)
			for _, variant := range []struct {
				name  string
				model *nav.Model
			}{{"ordinary", buildModel(t)}, {"capability fault", faulted}} {
				t.Run(variant.name, func(t *testing.T) {
					t.Parallel()
					model := variant.model
					chrome := recordedChrome()
					chrome.Lang = lang
					view := renderedNoteView(t, model, "Writing/lessons/go/L01.md", "")
					view.TitleAnchor = ""
					view.TOC = nil
					empty := chromeAnchorPage(t, &view, &chrome)
					ids := regexp.MustCompile(`\sid="([^"]+)"`)
					declared := ids.FindAllStringSubmatch(empty, -1)
					if len(declared) == 0 {
						t.Fatal("empty note page has no chrome IDs")
					}
					var body strings.Builder
					for _, match := range declared {
						id := match[1]
						if graph.SectionID(id) == id {
							t.Errorf("chrome ID %q is an authored section address", id)
						}
						body.WriteString("## " + strings.NewReplacer("-", " ", "_", " ").Replace(id) + "\n\nPassage.\n\n")
					}
					for _, heading := range []string{"Main content", "Nav rail", "Nav rail body", "Kbd pref note", "Schema notices", "Status panel label", "Status bar label"} {
						body.WriteString("## " + heading + "\n\nOriginal address.\n\n")
					}
					view = renderedNoteView(t, model, "Writing/lessons/go/L01.md", body.String())
					page := chromeAnchorPage(t, &view, &chrome)
					counts := make(map[string]int)
					for _, match := range ids.FindAllStringSubmatch(page, -1) {
						counts[match[1]]++
					}
					for id, count := range counts {
						if count != 1 {
							t.Errorf("note page ID %q occurs %d times, want exactly once", id, count)
						}
					}
					for _, match := range declared {
						if counts[match[1]] != 1 {
							t.Errorf("chrome ID %q names %d elements after headings arrive", match[1], counts[match[1]])
						}
					}
					for _, id := range []string{"main-content", "nav-rail", "nav-rail-body", "kbd-pref-note", "schema-notices", "status-panel-label", "status-bar-label"} {
						if !strings.Contains(view.BodyHTML, `id="`+id+`"`) {
							t.Errorf("authored heading address %q was changed or omitted", id)
						}
					}
					refs := regexp.MustCompile(`\s(?:aria-controls|aria-labelledby|aria-describedby|commandfor|popovertarget)="([^"]+)"`)
					for _, match := range refs.FindAllStringSubmatch(page, -1) {
						for id := range strings.FieldsSeq(match[1]) {
							if counts[id] != 1 {
								t.Errorf("chrome IDREF %q names %d elements, want exactly one", id, counts[id])
							}
						}
					}
					main := regexp.MustCompile(`<main id="([^"]+)" tabindex="-1"`).FindStringSubmatch(page)
					if main == nil {
						t.Fatal("note page has no focusable main")
					}
					for _, opening := range []string{`<a class="y-skiplink" href="#`, `<link rel="expect" href="#`} {
						if !strings.Contains(page, opening+main[1]+`"`) {
							t.Errorf("chrome link %q does not address the actual main ID %q", opening, main[1])
						}
					}
				})
			}
		})
	}
}

func chromeAnchorPage(t *testing.T, view *NoteView, chrome *layouts.Chrome) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Note(*view, *chrome).Render(t.Context(), &buf); err != nil {
		t.Fatalf("Note.Render: %v", err)
	}
	return buf.String()
}
