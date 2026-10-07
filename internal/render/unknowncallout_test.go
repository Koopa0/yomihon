package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

const unknownCalloutIcon = `<svg class="callout-icon" aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"></circle><path d="M12 11v5"></path><path d="M12 8h.01"></path></svg>`

func TestUnknownCalloutShell(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	got := r.HTML("note.md", "", "> [!my-callout] Title\n> body\n", wording.En)
	t.Log("producer-hit: actual unknown callout rendering")
	want := `<div class="callout callout-note"><p class="callout-title">` + unknownCalloutIcon + `Title</p><div class="callout-body">` + "\n<p>body</p>\n</div></div>\n"
	if diff := cmp.Diff(want, got.HTML); diff != "" {
		t.Errorf("caught: unknown callout did not render its ruled shell (-want +got):\n%s", diff)
	}
	if len(got.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %+v, want exactly one", got.Diagnostics)
	}
	if got.Diagnostics[0].Kind != render.DiagUnknownCallout || got.Diagnostics[0].Target != "my-callout" {
		t.Errorf("Diagnostic = %+v, want unknown-callout for my-callout", got.Diagnostics[0])
	}
	for _, tt := range []struct {
		name  string
		body  string
		title string
		open  string
		tag   string
		close string
	}{
		{name: "untitled canonical identifier", body: "> [!CUSTOM_TYPE2]\n> body\n", title: "Custom_type2", open: `<div class="callout callout-note">`, tag: "p", close: "</div>"},
		{name: "open fold", body: "> [!banana]+\n> body\n", title: "Banana", open: `<details class="callout callout-note" open>`, tag: "summary", close: "</details>"},
		{name: "closed fold", body: "> [!banana]- Title\n> body\n", title: "Title", open: `<details class="callout callout-note">`, tag: "summary", close: "</details>"},
		{name: "escaped authored title", body: "> [!banana] Author <script> & **plain**\n> body\n", title: "Author &lt;script&gt; &amp; **plain**", open: `<div class="callout callout-note">`, tag: "p", close: "</div>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := r.HTML("note.md", "", tt.body, wording.ZhHant)
			t.Log("producer-hit: actual unknown callout rendering")
			expected := tt.open + "<" + tt.tag + ` class="callout-title">` + unknownCalloutIcon + tt.title + "</" + tt.tag + `><div class="callout-body">` + "\n<p>body</p>\n</div>" + tt.close + "\n"
			if diff := cmp.Diff(expected, result.HTML); diff != "" {
				t.Errorf("caught: unknown callout did not render its ruled shell (-want +got):\n%s", diff)
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != render.DiagUnknownCallout {
				t.Errorf("Diagnostics = %+v, want one unknown-callout", result.Diagnostics)
			}
		})
	}
}

func TestUnknownCalloutOpeningAddress(t *testing.T) {
	t.Parallel()
	const body = "> [!zzz] Unknown ^zzz\n> body\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": body})
	page := r.HTML("Target.md", "", body, wording.En)
	location := render.DeclaredLocation(&page, "", graph.Wikilink{Target: "Target", Block: "zzz"}, "", wording.En)
	link := r.HTML("Source.md", "", "[[Target#^zzz]]\n", wording.En)
	t.Log("producer-hit: actual unknown opening address and location/link consumers")
	if diff := cmp.Diff([]string{"^zzz"}, page.Blocks); diff != "" {
		t.Errorf("caught: unknown opening address no longer reaches its block; Blocks (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"^zzz"}, emittedBlockIDs(t, page.HTML)); diff != "" {
		t.Errorf("caught: unknown opening address no longer reaches its block; emitted ids (-want +got):\n%s", diff)
	}
	if strings.Count(page.HTML, `<span id="^zzz">^zzz</span>`) != 1 {
		t.Errorf("caught: unknown opening address no longer reaches its block; HTML = %q", page.HTML)
	}
	if diff := cmp.Diff(render.SourceLocation{Label: "^zzz", Fragment: "^zzz"}, location); diff != "" {
		t.Errorf("caught: unknown opening address no longer reaches its block; location (-want +got):\n%s", diff)
	}
	wantLink := "<p>" + `<a href="/notes/Target.md#%5Ezzz" class="wikilink">Target#^zzz</a>` + "</p>\n"
	if diff := cmp.Diff(wantLink, link.HTML); diff != "" {
		t.Errorf("caught: unknown opening address no longer reaches its block; link (-want +got):\n%s", diff)
	}
	if len(link.Diagnostics) != 0 {
		t.Errorf("link Diagnostics = %+v, want none", link.Diagnostics)
	}
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{name: "opening claims before body", body: "> [!zzz] First ^same\n> Duplicate ^same\n", want: []string{"^same"}},
		{name: "earlier paragraph keeps first claim", body: "First ^same\n\n> [!zzz] Duplicate ^same\n> body\n", want: []string{"^same"}},
		{name: "canonical folded opening", body: "> [!zzz]- Title ^CAFE\u0301\n> body\n", want: []string{"^café"}},
		{name: "source code span owns caret", body: "> [!zzz] `shown ^inside\n> expression`\n", want: nil},
		{name: "opening code span owns body caret", body: "> [!zzz] `start\n> middle ^fake\n> end`\n", want: nil},
		{name: "known opening remains refused", body: "> [!note] Known ^known\n> body\n", want: nil},
		{name: "transcluded opening remains source owned", body: "![[Target]]\n", want: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := r.HTML("Source.md", "", tt.body, wording.En)
			t.Log("producer-hit: actual unknown opening address controls")
			if diff := cmp.Diff(tt.want, result.Blocks); diff != "" {
				t.Errorf("caught: unknown opening address no longer reaches its block; Blocks (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, emittedBlockIDs(t, result.HTML)); diff != "" {
				t.Errorf("caught: unknown opening address no longer reaches its block; emitted ids (-want +got):\n%s", diff)
			}
			if tt.name == "opening claims before body" && !strings.Contains(result.HTML, `First <span id="^same">^same</span>`) {
				t.Errorf("caught: unknown opening address no longer reaches its block; first owner HTML = %q", result.HTML)
			}
			if tt.name == "earlier paragraph keeps first claim" && !strings.Contains(result.HTML, `<p>First <span id="^same">^same</span></p>`) {
				t.Errorf("caught: unknown opening address no longer reaches its block; first owner HTML = %q", result.HTML)
			}
		})
	}
}
