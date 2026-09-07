package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNotePageCarriesExactlyOneH1 is the lock that a screen reader cycling
// headings lands on the page title once. The chrome already emits
// <h1 class="y-title">; a body heading that the title-equal fold does not
// take used to survive as a second h1. The three shapes are the fold's own
// case, a leading # whose text is not the title, and a # later in the body —
// the last two are the ones that used to duplicate the title, and the first
// stays folded so the page does not grow a heading it had already removed.
func TestNotePageCarriesExactlyOneH1(t *testing.T) {
	t.Parallel()

	const title = "Probe heading levels"
	tests := []struct {
		name      string
		body      string
		wantBody  string
		wantLevel string
	}{
		{
			name: "a leading # equal to the title is folded",
			body: "# Probe heading levels\n\nopening prose\n",
		},
		{
			name:      "a leading # different from the title is demoted",
			body:      "# A different opening\n\nopening prose\n",
			wantBody:  `<h2 id="a-different-opening">A different opening</h2>`,
			wantLevel: `data-level="2"`,
		},
		{
			name:      "a # later in the body is demoted",
			body:      "opening prose\n\n# Later heading\n\nmore prose\n",
			wantBody:  `<h2 id="later-heading">Later heading</h2>`,
			wantLevel: `data-level="2"`,
		},
	}

	r := render.New(graph.BuildFromNotes(nil, nil), emptyBodies{}, noDeclaredTitles{}, vaultHolds{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Notes/Probe.md", title, tt.body, wording.ZhHant)
			var buf bytes.Buffer
			view := NoteView{
				Title:       title,
				RelPath:     "Notes/Probe.md",
				BodyHTML:    got.HTML,
				TitleAnchor: got.TitleAnchor,
				TOC:         got.TOC,
			}
			if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			page := buf.String()
			if n := strings.Count(page, "<h1"); n != 1 {
				t.Errorf("page contains %d <h1, want exactly 1:\n%s", n, page)
			}
			if !strings.Contains(page, `<h1 class="y-title">`+title+`</h1>`) &&
				!strings.Contains(page, `<h1 id="`+got.TitleAnchor+`" class="y-title">`+title+`</h1>`) {
				t.Errorf("the one h1 is not the chrome title:\n%s", page)
			}
			if tt.wantBody != "" && !strings.Contains(page, tt.wantBody) {
				t.Errorf("page missing demoted heading %s:\n%s", tt.wantBody, page)
			}
			if tt.wantLevel != "" && !strings.Contains(page, tt.wantLevel) {
				t.Errorf("contents list did not move with the heading; missing %s:\n%s", tt.wantLevel, page)
			}
			if strings.Contains(page, `data-level="1"`) {
				t.Errorf("contents list still advertises a body h1:\n%s", page)
			}
		})
	}
}

type emptyBodies struct{}

func (emptyBodies) Transclusion(string) (string, bool) { return "", false }

type noDeclaredTitles struct{}

func (noDeclaredTitles) TitledBy(string) []string { return nil }

type vaultHolds struct{}

func (vaultHolds) MissingFile(string) bool { return false }
