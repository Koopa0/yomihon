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

func TestHTMLCommentsHideTheirContentsAndLinks(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": "Embedded <!-- embeddedprivate [[Ghost]] --> words."})
	tests := []struct{ name, body, want string }{
		{name: "block", body: "<!-- confidential [[Ghost]] -->\n", want: ""},
		{name: "inline", body: "Before <!-- private [[Ghost]] --> after.", want: "<p>Before  after.</p>\n"},
		{name: "multiline", body: "Before <!-- first\nsecond [[Ghost]] --> after.", want: "<p>Before\nafter.</p>\n"},
		{name: "greater than inside", body: "A<!-- x > [[Ghost]] -->B", want: "<p>AB</p>\n"},
		{name: "percent signs inside", body: "A<!-- %% [[Ghost]] -->B", want: "<p>AB</p>\n"},
		{name: "HTML opener inside percent", body: "A%% <!-- [[Ghost]] %%B", want: "<p>AB</p>\n"},
		{name: "fence inside comment", body: "A\n<!--\n```\n[[Ghost]]\n```\n-->\nB", want: "<p>A</p>\n<p>B</p>\n"},
		{name: "short empty", body: "A<!-->B<!--->C", want: "<p>ABC</p>\n"},
		{name: "escaped", body: `A\<!-- literal -->B`, want: "<p>A&lt;!-- literal --&gt;B</p>\n"},
		{name: "code span", body: "`<!-- literal -->`", want: "<p><code>&lt;!-- literal --&gt;</code></p>\n"},
		{name: "fenced code", body: "```text\n<!-- literal -->\n```", want: "<pre class=\"chroma\"><code><span class=\"line\"><span class=\"cl\">&lt;!-- literal --&gt;\n</span></span></code></pre>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if diff := cmp.Diff(tt.want, got.HTML); diff != "" {
				t.Errorf("HTML comments (-want +got):\n%s", diff)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("hidden comments produced diagnostics: %v", got.Diagnostics)
			}
		})
	}
	embedded := r.HTML("Note.md", "", "![[Target]]", wording.ZhHant)
	if strings.Contains(embedded.HTML, "embeddedprivate") || strings.Contains(embedded.HTML, "Ghost") || len(embedded.Diagnostics) != 0 {
		t.Errorf("embedded comment reached the page: %+v", embedded)
	}
	if !strings.Contains(embedded.HTML, "Embedded  words.") {
		t.Errorf("embedded visible words lost: %s", embedded.HTML)
	}
}

// An HTML comment that never closes hides every word after it, as an unclosed
// %% does, so the page says where that silence starts instead of just stopping.
func TestUnclosedHTMLCommentNamesItsLine(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": "Embedded\n\nwords <!-- todo\n\nhidden"})
	tests := []struct{ name, body, want, message string }{
		{
			name:    "opened in prose",
			body:    "Text <!-- todo\n\nfirst\n\nsecond\n\nthird\n",
			want:    "<p>Text</p>\n",
			message: "an unclosed <!-- comment opened at line 1 of the note body hides everything after it",
		},
		{
			name:    "opened after visible paragraphs",
			body:    "First.\n\nA<!-- private\n[[Ghost]]",
			want:    "<p>First.</p>\n<p>A</p>\n",
			message: "an unclosed <!-- comment opened at line 3 of the note body hides everything after it",
		},
		{
			name:    "opened as a block",
			body:    "First.\n\n<!-- private\n\nsecret\n",
			want:    "<p>First.</p>\n",
			message: "an unclosed <!-- comment opened at line 3 of the note body hides everything after it",
		},
		{
			name:    "embedded",
			body:    "![[Target]]",
			message: "an unclosed <!-- comment opened at line 3 of the note body hides everything after it",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if tt.want != "" {
				if diff := cmp.Diff(tt.want, got.HTML); diff != "" {
					t.Errorf("unclosed HTML comment (-want +got):\n%s", diff)
				}
			}
			if strings.Contains(got.HTML, "hidden") || strings.Contains(got.HTML, "secret") || strings.Contains(got.HTML, "Ghost") {
				t.Errorf("unclosed comment words reached the page: %s", got.HTML)
			}
			var reported []render.Diagnostic
			for _, d := range got.Diagnostics {
				if d.Kind == render.DiagCommentUnclosed {
					reported = append(reported, d)
				}
			}
			if len(reported) != 1 || reported[0].Target != "<!--" || reported[0].Message != tt.message {
				t.Errorf("unclosed HTML comment diagnostics = %+v, want one at %q saying %q", got.Diagnostics, "<!--", tt.message)
			}
		})
	}
}

func TestHeadingWordsDropsHTMLCommentPayload(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"Visible <!-- private --> heading", "Visible <!-- private > more --> heading"} {
		if got := render.HeadingWords(raw); got != "Visible  heading" {
			t.Errorf("HeadingWords(%q) = %q, want Visible  heading", raw, got)
		}
	}
}

func TestHTMLCommentsKeepMarkdownContainerBoundaries(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	tests := []struct{ name, body, want string }{
		{name: "quoted close", body: "> Before <!-- private\n> secret --> after.", want: "<blockquote>\n<p>Before\nafter.</p>\n</blockquote>\n"},
		{name: "unclosed quote", body: "> Before\n> <!-- private\n> secret\n\nAfter.", want: "<blockquote>\n<p>Before</p>\n</blockquote>\n<p>After.</p>\n"},
		{name: "unclosed list", body: "- Item\n  <!-- private\n  secret\n\nAfter.", want: "<ul>\n<li>Item</li>\n</ul>\n<p>After.</p>\n"},
		{name: "indented code", body: "    <!-- literal -->\n\nAfter.", want: "<pre><code>&lt;!-- literal --&gt;\n</code></pre>\n<p>After.</p>\n"},
		{name: "wrapped code span", body: "`begin\nmiddle <!-- literal --> end`", want: "<p><code>begin middle &lt;!-- literal --&gt; end</code></p>\n"},
		{name: "indented paragraph continuation", body: "Before\n    <!-- private --> after.", want: "<p>Before\nafter.</p>\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if diff := cmp.Diff(tt.want, got.HTML); diff != "" {
				t.Errorf("HTML container boundary (-want +got):\n%s", diff)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("HTML container diagnostics = %v, want none", got.Diagnostics)
			}
		})
	}
}

func TestSearchHidesHTMLCommentWordsAndKeepsVisibleWords(t *testing.T) {
	t.Parallel()
	body := "Openingvisible <!-- %% private799 [[Ghost]] --> Closingvisible.\n\n```text\n<!-- literal799 -->\n```\n"
	note := vault.Parse("Notes/Comment.md", []byte("---\ntitle: Probe\n---\n"+body))
	idx := lexical.NewIndex([]lexical.Document{lexical.DocumentFromNote(note)}, schema.ArtifactPolicy{})
	tests := []struct {
		query string
		want  []string
	}{
		{query: "private799", want: []string{}}, {query: "Ghost", want: []string{}},
		{query: "Openingvisible", want: []string{"Notes/Comment.md"}},
		{query: "Closingvisible", want: []string{"Notes/Comment.md"}},
		{query: "literal799", want: []string{"Notes/Comment.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			answer, err := idx.Search(lexical.Parse(tt.query), -1)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, searchPaths(answer.Results)); diff != "" {
				t.Errorf("Search HTML comment words (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHeadingWordsKeepsLiteralHTMLCommentSyntax(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`Visible \<!-- literal --> heading`, "Visible `<!-- literal -->` heading"} {
		if got := render.HeadingWords(raw); got != raw {
			t.Errorf("HeadingWords literal comment = %q, want %q", got, raw)
		}
	}
}
