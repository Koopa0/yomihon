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
		name     string
		body     string
		wantBody string
	}{
		{
			name: "a leading # equal to the title is folded",
			body: "# Probe heading levels\n\nopening prose\n",
		},
		{
			name:     "a leading # different from the title is demoted",
			body:     "# A different opening\n\nopening prose\n",
			wantBody: `<h2 id="a-different-opening" data-level="1">A different opening</h2>`,
		},
		{
			name:     "a # later in the body is demoted",
			body:     "opening prose\n\n# Later heading\n\nmore prose\n",
			wantBody: `<h2 id="later-heading" data-level="1">Later heading</h2>`,
		},
	}

	r := render.New(graph.BuildFromNotes(nil, nil), emptyBodies{}, noDeclaredTitles{}, vaultHolds{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			page := renderNote(t, r, title, tt.body)
			if n := strings.Count(page, "<h1"); n != 1 {
				t.Errorf("page contains %d <h1, want exactly 1:\n%s", n, page)
			}
			if !strings.Contains(page, `class="y-title">`+title+`</h1>`) {
				t.Errorf("the one h1 is not the chrome title:\n%s", page)
			}
			if tt.wantBody != "" && !strings.Contains(page, tt.wantBody) {
				t.Errorf("page missing demoted heading %s:\n%s", tt.wantBody, page)
			}
		})
	}
}

// TestNotePageDemotesTheWholeAuthoredOutline is the lock above internal/render
// for the ##→h5 half. Demoting only # leaves ## as h2 and #### as h4, and
// every pages test that only watched a surviving # stayed green. The look is
// keyed on data-level, so the tags still have to move or a screen-reader
// outline of chapters sits one step too high.
func TestNotePageDemotesTheWholeAuthoredOutline(t *testing.T) {
	t.Parallel()

	const title = "Probe heading levels"
	body := strings.Join([]string{
		"opening prose",
		"",
		"## Alpha",
		"",
		"### Beta",
		"",
		"#### Gamma",
		"",
		"##### Delta",
		"",
		"###### Epsilon",
		"",
	}, "\n")
	r := render.New(graph.BuildFromNotes(nil, nil), emptyBodies{}, noDeclaredTitles{}, vaultHolds{})
	page := renderNote(t, r, title, body)
	for _, want := range []string{
		`<h3 id="alpha" data-level="2">Alpha</h3>`,
		`<h4 id="beta" data-level="3">Beta</h4>`,
		`<h5 id="gamma" data-level="4">Gamma</h5>`,
		`<h6 id="delta" data-level="5">Delta</h6>`,
		`<h6 id="epsilon" data-level="6">Epsilon</h6>`,
		`data-level="4"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %s:\n%s", want, page)
		}
	}
	if strings.Contains(page, `<h2 id="alpha"`) {
		t.Errorf("authored ## survived as h2; demoting only # leaves the rest of the outline unmoved:\n%s", page)
	}
	if strings.Contains(page, `<h4 id="gamma"`) {
		t.Errorf("authored #### survived as h4; the shell must emit it as h5:\n%s", page)
	}
	if n := strings.Count(page, "<h1"); n != 1 {
		t.Errorf("page contains %d <h1, want exactly 1:\n%s", n, page)
	}
}

func renderNote(t *testing.T, r *render.Pipeline, title, body string) string {
	t.Helper()
	got := r.HTML("Notes/Probe.md", title, body, wording.ZhHant)
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
	return buf.String()
}

type emptyBodies struct{}

func (emptyBodies) Transclusion(string) (string, bool) { return "", false }

type noDeclaredTitles struct{}

func (noDeclaredTitles) TitledBy(string) []string { return nil }

type vaultHolds struct{}

func (vaultHolds) MissingFile(string) bool { return false }
