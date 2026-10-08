package pages

import (
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// Pin the words the reader sees, independently of the wording table under test.
func TestCommentContainerDiagnosticsOnNoteAndCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                     string
		lang                     wording.Lang
		label, summary, bodywide string
	}{
		{name: "zh-Hant", lang: wording.ZhHant, label: "所在 Markdown 區塊沒有配對的註解記號", summary: "這個註解記號沒有找到配對，所在 Markdown 區塊剩下的內容被藏了起來。", bodywide: "這個註解記號沒有找到配對，它後面的內容全被藏了起來。"},
		{name: "en", lang: wording.En, label: "Unpaired comment marker in a Markdown container", summary: "This comment marker has no partner, and the rest of its Markdown container is hidden.", bodywide: "This comment marker has no partner, and everything after it is hidden."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := NoteView{Title: "First", RelPath: "First.md", BodyHTML: "<p>After.</p>", RenderDiagnostics: []render.Diagnostic{
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"},
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 8 of the note body hides the rest of its Markdown container"},
			}}
			b := NoteView{Title: "Second", RelPath: "Second.md", BodyHTML: "<p>Kept.</p>", RenderDiagnostics: []render.Diagnostic{
				{Kind: render.DiagCommentUnclosed, Target: "%%", Message: "an unclosed %% comment opened at line 3 of the note body hides everything after it"},
			}}
			chrome := layouts.Chrome{Lang: tt.lang}
			for _, surface := range []struct {
				name string
				page templ.Component
			}{
				{name: "note", page: Note(a, chrome)},
				{name: "compare", page: Compare(CompareView{A: a, B: b}, chrome)},
			} {
				t.Run(surface.name, func(t *testing.T) {
					t.Parallel()
					got := renderedBytes(t, t.Context(), surface.page)
					for _, want := range []string{
						`<div class="y-diag__label">` + tt.label + `</div>`,
						tt.summary,
						`<code class="y-diag__target">&lt;!--</code>`,
						`<code lang="en">an unclosed &lt;!-- comment opened at line 2 of the note body hides the rest of its Markdown container</code>`,
						`<code lang="en">an unclosed &lt;!-- comment opened at line 8 of the note body hides the rest of its Markdown container</code>`,
						"<p>After.</p>",
					} {
						if !strings.Contains(got, want) {
							t.Errorf("caught: localized container diagnostic page lacks %q", want)
						}
					}
					if strings.Contains(got, "comment-container-unclosed") {
						t.Error("caught: container kind reached the reader as its raw slug")
					}
					if surface.name == "compare" {
						for _, want := range []string{tt.bodywide, "<p>Kept.</p>", `<code lang="en">an unclosed %% comment opened at line 3 of the note body hides everything after it</code>`} {
							if !strings.Contains(got, want) {
								t.Errorf("body-wide comparison diagnostic page lacks %q", want)
							}
						}
					} else if strings.Contains(got, tt.bodywide) {
						t.Error("caught: container-only note claims body-wide hiding")
					}
				})
			}
		})
	}
}

func TestOnlyBodyWideCommentSuppressesEmptyBodyInvitation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		lang  wording.Lang
		empty string
	}{
		{name: "zh-Hant", lang: wording.ZhHant, empty: "這篇筆記還沒有內容。"},
		{name: "en", lang: wording.En, empty: "This note has no content yet."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			container := NoteView{Title: "Container", RelPath: "Container.md", RenderDiagnostics: []render.Diagnostic{{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"}}}
			bodywide := NoteView{Title: "Body-wide", RelPath: "Bodywide.md", RenderDiagnostics: []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: "an unclosed <!-- comment opened at line 1 of the note body hides everything after it"}}}
			chrome := layouts.Chrome{Lang: tt.lang}
			for _, surface := range []struct {
				name       string
				page       templ.Component
				ordinal    int
				invitation bool
			}{
				{name: "container note", page: Note(container, chrome), invitation: true},
				{name: "body-wide note", page: Note(bodywide, chrome)},
				{name: "container compare column", page: Compare(CompareView{A: container, B: bodywide}, chrome), invitation: true},
				{name: "body-wide compare column", page: Compare(CompareView{A: container, B: bodywide}, chrome), ordinal: 1},
			} {
				t.Run(surface.name, func(t *testing.T) {
					t.Parallel()
					page := renderedBytes(t, t.Context(), surface.page)
					article := articleAt(t, page, surface.ordinal)
					if got := strings.Contains(article, tt.empty); got != surface.invitation {
						t.Errorf("caught: empty-body invitation present = %t, want %t in %s", got, surface.invitation, surface.name)
					}
				})
			}
		})
	}
}
