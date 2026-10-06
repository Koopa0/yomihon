package pages

import (
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestEmptyBodyDistinguishesHiddenContent(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		body        string
		diagnostics []render.Diagnostic
		empty       bool
	}{
		{name: "empty", empty: true},
		{name: "unrelated diagnostic", diagnostics: []render.Diagnostic{{Kind: render.DiagImageMissing}}, empty: true},
		{name: "unclosed comment", diagnostics: []render.Diagnostic{{Kind: render.DiagCommentUnclosed}}},
		{name: "comment after unrelated diagnostic", diagnostics: []render.Diagnostic{{Kind: render.DiagImageMissing}, {Kind: render.DiagCommentUnclosed}}},
		{name: "visible prefix before comment", body: "<p>Visible prose.</p>", diagnostics: []render.Diagnostic{{Kind: render.DiagCommentUnclosed}}},
	} {
		for _, language := range []struct {
			lang     wording.Lang
			sentence string
		}{
			{lang: wording.ZhHant, sentence: "這篇筆記還沒有內容。"},
			{lang: wording.En, sentence: "This note has no content yet."},
		} {
			view := NoteView{Title: "Draft", BodyHTML: test.body, RenderDiagnostics: test.diagnostics}
			chrome := layouts.Chrome{Lang: language.lang}
			for _, face := range []struct {
				name      string
				component templ.Component
			}{
				{name: "note", component: Note(view, chrome)},
				{name: "compare", component: Compare(CompareView{A: view, B: NoteView{Title: "Other", BodyHTML: "<p>Other note.</p>"}}, chrome)},
			} {
				t.Run(test.name+"/"+string(language.lang)+"/"+face.name, func(t *testing.T) {
					t.Parallel()
					body := renderedBytes(t, t.Context(), face.component)
					want := 0
					if test.empty {
						want = 1
					}
					if got := strings.Count(body, language.sentence); got != want {
						t.Errorf("caught: empty-body sentence count = %d, want %d", got, want)
					}
					if test.body != "" {
						if got := strings.Count(body, "Visible prose."); got != 1 {
							t.Errorf("caught: visible prose count = %d, want 1", got)
						}
					}
				})
			}
		}
	}
}
