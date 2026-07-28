package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

func TestObsidianCommentsExcludedFromAllProjections(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name    string
		body    string
		present []string
		absent  []string
	}{
		{
			name:    "inline",
			body:    "before %%hidden inline%% after",
			present: []string{"before", "after"},
			absent:  []string{"hidden inline", "%%"},
		},
		{
			name:    "multiline",
			body:    "before\n%%hidden first\nhidden second%%\nafter",
			present: []string{"before", "after"},
			absent:  []string{"hidden first", "hidden second", "%%"},
		},
		{
			name:    "unclosed",
			body:    "before %%hidden through\nthe end",
			present: []string{"before"},
			absent:  []string{"hidden through", "the end", "%%"},
		},
		{
			name:    "empty",
			body:    "before %%%% after",
			present: []string{"before", "after"},
			absent:  []string{"%%"},
		},
		{
			name:    "adjacent",
			body:    "one%%first%%%%second%%two",
			present: []string{"one", "two"},
			absent:  []string{"first", "second", "%%"},
		},
		{
			name:    "fenced literal",
			body:    "before\n```text\n%%literal example%%\n```\nafter %%hidden outside%%",
			present: []string{"before", "%%literal example%%", "after"},
			absent:  []string{"hidden outside"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projections := map[string]string{
				"HTML":          r.HTML("note.md", "", tt.body).HTML,
				"PlainText":     render.PlainText(tt.body),
				"PlainSections": joinedSectionText(render.PlainSections(tt.body)),
			}
			for projection, got := range projections {
				for _, want := range tt.present {
					if !strings.Contains(got, want) {
						t.Errorf("%s projection = %q, want substring %q", projection, got, want)
					}
				}
				for _, unwanted := range tt.absent {
					if strings.Contains(got, unwanted) {
						t.Errorf("%s projection = %q, must not contain %q", projection, got, unwanted)
					}
				}
			}
		})
	}
}

func TestObsidianCommentsExcludedFromEmbeddedNotes(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "Embedded.md"}}, nil, transclusions{
		"Embedded.md": "visible embed %%hidden embed%%",
	})

	got := r.HTML("note.md", "", "![[Embedded]]").HTML
	if !strings.Contains(got, "visible embed") {
		t.Errorf("HTML() = %q, want embedded visible text", got)
	}
	if strings.Contains(got, "hidden embed") || strings.Contains(got, "%%") {
		t.Errorf("HTML() = %q, must not contain the embedded Obsidian comment", got)
	}
}

func joinedSectionText(sections []render.PlainSection) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.Text)
	}
	return strings.Join(parts, "\n")
}
