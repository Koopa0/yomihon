package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestCalloutIdentifierDiagnostics(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, tt := range []struct{ typ, target string }{{"my-callout", "my-callout"}, {"todo2", "todo2"}, {"custom_type", "custom_type"}, {"CUSTOM_TYPE", "custom_type"}, {"2", "2"}, {"_", "_"}, {"-", "-"}} {
		typ := tt.typ
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			body := "> [!" + typ + "] Heading\n> detail\n"
			got := r.HTML("note.md", "", body, wording.ZhHant)
			wantHTML := `<div class="callout callout-note"><p class="callout-title">` + unknownCalloutIcon + `Heading</p><div class="callout-body">` + "\n<p>detail</p>\n</div></div>\n"
			if diff := cmp.Diff(wantHTML, got.HTML); diff != "" {
				t.Errorf("unknown identifier HTML (-want +got):\n%s", diff)
			}
			if len(got.Diagnostics) != 1 {
				t.Fatalf("unknown identifier diagnostics = %+v, want exactly one", got.Diagnostics)
			}
			diag := got.Diagnostics[0]
			if diag.Kind != render.DiagUnknownCallout || cmp.Diff(tt.target, diag.Target) != "" {
				t.Errorf("unknown identifier diagnostic = %+v", diag)
			}
			if render.UnanchorableLine("> [!" + typ + "] Heading ^address") {
				t.Error("unknown identifier refused its opening address")
			}
		})
	}
	for _, body := range []string{"> [!] Empty\n> detail\n", "> [!my.callout] Dot\n> detail\n", "    > [!todo2] Indented\n", "    > [!success] Indented\n", "\t> [!success] Tabbed\n"} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", body, wording.ZhHant)
			if strings.Contains(body, "Indented") || strings.Contains(body, "Tabbed") {
				if !strings.Contains(got.HTML, "<pre><code>&gt; [!") || !strings.Contains(render.PlainText(body), "[!") {
					t.Errorf("indented callout source lost its literal code: %+v", got)
				}
			}
			if len(got.Diagnostics) != 0 || strings.Contains(got.HTML, `class="callout`) {
				t.Errorf("literal control was parsed as a callout: %+v", got)
			}
		})
	}
}

func TestCalloutNewTypesKeepTitleOwnership(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	// The note icon is the existing look all five built-ins are assigned to.
	const icon = `<svg class="callout-icon" aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"></circle><path d="M12 11v5"></path><path d="M12 8h.01"></path></svg>`
	for _, tt := range []struct{ typ, title string }{{"success", "Success"}, {"check", "Check"}, {"done", "Done"}, {"important", "Important"}, {"tldr", "Tldr"}} {
		for _, fold := range []string{"", "+", "-"} {
			t.Run(tt.typ+fold, func(t *testing.T) {
				t.Parallel()
				body := "> [!" + strings.ToUpper(tt.typ) + "]" + fold + "\n> body text\n"
				got := r.HTML("note.md", "", body, wording.ZhHant)
				opening, heading, closing := `<div class="callout callout-note">`, "p", "</div>"
				if fold != "" {
					opening, heading, closing = `<details class="callout callout-note">`, "summary", "</details>"
					if fold == "+" {
						opening = `<details class="callout callout-note" open>`
					}
				}
				want := opening + `<` + heading + ` class="callout-title">` + icon + tt.title + `</` + heading + `><div class="callout-body">` + "\n<p>body text</p>\n</div>" + closing + "\n"
				if diff := cmp.Diff(want, got.HTML); diff != "" {
					t.Errorf("built-in callout HTML (-want +got):\n%s", diff)
				}
				if len(got.Diagnostics) != 0 {
					t.Errorf("known type diagnostics = %+v", got.Diagnostics)
				}
				if !render.UnanchorableLine("> [!" + tt.typ + "] Title ^address") {
					t.Error("known callout title was accepted as a block address")
				}
			})
		}
	}
	for _, typ := range []string{"tip", "bug", "faq", "success", "check", "done", "important", "tldr"} {
		t.Run(typ+" authored", func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", "> [!"+typ+"]+ Author <script> & **plain**\n> detail\n", wording.En)
			if !strings.Contains(got.HTML, `Author &lt;script&gt; &amp; **plain**</summary>`) || strings.Contains(got.HTML, "<script>") || len(got.Diagnostics) != 0 {
				t.Errorf("authored title was replaced or activated: %+v", got)
			}
		})
	}
}

func TestCalloutIdentifierSeparatesAdjacentBodies(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, typ := range []string{"my-callout", "todo2", "custom_type"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", "> [!note] Known\n> first body\n> [!"+typ+"] Unknown\n> second body\n\n> [!check] Next\n> third body\n", wording.En)
			boundary := "<p>first body</p>\n</div></div>\n" + `<div class="callout callout-note"><p class="callout-title">` + unknownCalloutIcon + `Unknown</p><div class="callout-body">` + "\n<p>second body</p>\n</div></div>"
			if !strings.Contains(got.HTML, boundary) || strings.Count(got.HTML, `class="callout callout-note"`) != 3 {
				t.Errorf("adjacent callout boundary missing %q:\n%s", boundary, got.HTML)
			}
			if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagUnknownCallout || got.Diagnostics[0].Target != typ {
				t.Errorf("adjacent callout diagnostic = %+v", got.Diagnostics)
			}
		})
	}
}

func TestCalloutIdentifierSearchProjection(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"my-callout", "todo2", "custom_type", "success", "check", "done", "important", "tldr"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			body := "> [!" + typ + "]+ Identifier heading\n> bodyneedle\n"
			if got := render.PlainText(body); strings.Contains(got, "[!") || !strings.Contains(got, "Identifier heading") || !strings.Contains(got, "bodyneedle") {
				t.Errorf("callout corpus = %q", got)
			}
			note := vault.Parse("note.md", []byte(body))
			index := lexical.NewIndex([]lexical.Document{lexical.DocumentFromNote(note)}, schema.ArtifactPolicy{})
			for _, tt := range []struct {
				query string
				hits  int
			}{{`"Identifier heading"`, 1}, {"bodyneedle", 1}, {`"` + typ + `"`, 0}} {
				answer, err := index.Search(lexical.Parse(tt.query), -1)
				if err != nil {
					t.Fatal(err)
				}
				if len(answer.Results) != tt.hits {
					t.Errorf("query %q = %d hits, want %d", tt.query, len(answer.Results), tt.hits)
				}
			}
		})
	}
}

func TestCalloutIdentifierFenceKeepsLiteralSyntax(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, typ := range []string{"my-callout", "todo2", "custom_type", "success"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			body := "```\n> [!" + typ + "] Code\n```\n"
			got := r.HTML("note.md", "", body, wording.En)
			want := `<pre class="chroma"><code><span class="line"><span class="cl">&gt; [!` + typ + "] Code\n</span></span></code></pre>"
			if diff := cmp.Diff(want, got.HTML); diff != "" {
				t.Errorf("fenced callout HTML (-want +got):\n%s", diff)
			}
			if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagRiskyFence {
				t.Errorf("fenced callout diagnostics = %+v", got.Diagnostics)
			}
			if !strings.Contains(render.PlainText(body), "[!"+typ+"]") {
				t.Error("fenced identifier disappeared from searchable code")
			}
		})
	}
}

func TestCalloutIdentifierBlockquoteIndent(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, prefix := range []string{"", " ", "  ", "   "} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", prefix+"> [!tip]\n"+prefix+"> detail\n", wording.En)
			if !strings.Contains(got.HTML, `class="callout callout-note"`) || !strings.Contains(got.HTML, `Tip</p>`) || len(got.Diagnostics) != 0 {
				t.Errorf("blockquote-indent callout = %+v", got)
			}
		})
	}
}

func TestCalloutNewTypesInTransclusions(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"success", "check", "done", "important", "tldr"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "target.md"}, {RelPath: "destination.md"}}, nil, transclusions{"target.md": "> [!" + typ + "] Authored\n> ## Inside\n> [[destination]] and detail\n"})
			got := r.HTML("source.md", "", "![[target]]\n", wording.En)
			if strings.Count(got.HTML, `class="callout callout-note"`) != 1 || !strings.Contains(got.HTML, `Authored</p>`) || !strings.Contains(got.HTML, `href="/notes/destination.md"`) || !strings.Contains(got.HTML, `<h3 id="inside" data-level="2">Inside</h3>`) || len(got.Diagnostics) != 0 {
				t.Errorf("transcluded callout lost its body or title: %+v", got)
			}
		})
	}
}

// A callout nested in a list item stands at the item's content column, which
// can be four spaces or more from the margin without being code. The page, the
// search text and the block-address check all have to read it as the callout
// it is, while the same opener at the margin's four spaces stays code.
func TestCalloutNestedInAListIsStillACallout(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, tt := range []struct{ name, body, class, title string }{
		{"warning two levels deep", "- a\n  - b\n    > [!warning] Deep\n    > nested body\n", "callout-warning", "Deep</p>"},
		{"new type two levels deep", "- a\n  - b\n    > [!success]\n    > nested body\n", "callout-note", "Success</p>"},
		{"ordered item", "1. a\n\n   1. b\n\n      > [!tip] Deeper\n      > nested body\n", "callout-note", "Deeper</p>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.En)
			if !strings.Contains(got.HTML, `<div class="callout `+tt.class+`">`) || !strings.Contains(got.HTML, tt.title) || strings.Contains(got.HTML, "[!") || len(got.Diagnostics) != 0 {
				t.Errorf("nested callout was not recognised: %+v", got)
			}
			if plain := render.PlainText(tt.body); strings.Contains(plain, "[!") || !strings.Contains(plain, "nested body") {
				t.Errorf("nested callout search text = %q", plain)
			}
		})
	}
	t.Run("unknown type nested", func(t *testing.T) {
		t.Parallel()
		got := r.HTML("note.md", "", "- a\n  - b\n    > [!nonesuch] Deep\n", wording.En)
		if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagUnknownCallout || got.Diagnostics[0].Target != "nonesuch" {
			t.Errorf("nested unknown callout diagnostics = %+v", got.Diagnostics)
		}
	})
	if !render.UnanchorableLine("    > [!warning] Deep ^address") {
		t.Error("a nested callout's title was accepted as a block address")
	}
}
