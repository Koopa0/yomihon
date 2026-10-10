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
			message: "Target.md: an unclosed <!-- comment opened at line 3 of the note body hides everything after it",
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
			want := []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: tt.message}}
			if diff := cmp.Diff(want, got.Diagnostics); diff != "" {
				t.Errorf("unclosed HTML comment diagnostics (-want +got):\n%s", diff)
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
	tests := []struct {
		name, body, want string
		diagnostics      []render.Diagnostic
	}{
		{name: "quoted close", body: "> Before <!-- private\n> secret --> after.", want: "<blockquote>\n<p>Before\nafter.</p>\n</blockquote>\n"},
		{name: "unclosed quote", body: "> Before\n> <!-- private\n> secret\n\nAfter.", want: "<blockquote>\n<p>Before</p>\n</blockquote>\n<p>After.</p>\n", diagnostics: []render.Diagnostic{{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"}}},
		{name: "unclosed list", body: "- Item\n  <!-- private\n  secret\n\nAfter.", want: "<ul>\n<li>Item</li>\n</ul>\n<p>After.</p>\n", diagnostics: []render.Diagnostic{{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"}}},
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
			if diff := cmp.Diff(tt.diagnostics, got.Diagnostics); diff != "" {
				t.Errorf("caught: container comment diagnostics (-want +got):\n%s", diff)
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

// Every independently bounded opener keeps its source line and its own record.
func TestHTMLContainerCommentsKeepEveryDiagnostic(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	const containers = "> Before\n> <!-- private\n> secret\n\nAfter.\n\n- Item\n  <!-- private\n  secret\n\nFinally."
	const visible = "<blockquote>\n<p>Before</p>\n</blockquote>\n<p>After.</p>\n<ul>\n<li>Item</li>\n</ul>\n<p>Finally.</p>\n"
	tests := []struct {
		name        string
		body        string
		diagnostics []render.Diagnostic
	}{
		{
			name: "two containers",
			body: containers,
			diagnostics: []render.Diagnostic{
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"},
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 8 of the note body hides the rest of its Markdown container"},
			},
		},
		{
			name: "containers then body-wide HTML",
			body: containers + "\n\n<!-- tail\nhidden",
			diagnostics: []render.Diagnostic{
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"},
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 8 of the note body hides the rest of its Markdown container"},
				{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: "an unclosed <!-- comment opened at line 13 of the note body hides everything after it"},
			},
		},
		{
			name: "containers then body-wide percent",
			body: containers + "\n\n%% tail\nhidden",
			diagnostics: []render.Diagnostic{
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides the rest of its Markdown container"},
				{Kind: render.DiagnosticKind("comment-container-unclosed"), Target: "<!--", Message: "an unclosed <!-- comment opened at line 8 of the note body hides the rest of its Markdown container"},
				{Kind: render.DiagCommentUnclosed, Target: "%%", Message: "an unclosed %% comment opened at line 13 of the note body hides everything after it"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.En)
			if diff := cmp.Diff(visible, got.HTML); diff != "" {
				t.Errorf("container visible HTML (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.diagnostics, got.Diagnostics); diff != "" {
				t.Errorf("caught: every container diagnostic (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEmbeddedHTMLContainerCommentKeepsOriginalLine(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{
		"Target.md": "# Title\n\n## Kept\n> Before\n> <!-- private\n> hidden\n\nAfter. ^kept\n\n## Other\nOutside.",
	})
	for _, body := range []string{"![[Target]]", "![[Target#Kept]]", "![[Target#^kept]]"} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Host.md", "", body, wording.En)
			var want []render.Diagnostic
			if body != "![[Target#^kept]]" {
				want = []render.Diagnostic{{Kind: render.DiagCommentContainerUnclosed, Target: "<!--", Message: "Target.md: an unclosed <!-- comment opened at line 5 of the note body hides the rest of its Markdown container"}}
			}
			if diff := cmp.Diff(want, got.Diagnostics); diff != "" {
				t.Errorf("caught: embedded original-line diagnostic (-want +got):\n%s", diff)
			}
			visible := []string{"After."}
			if body != "![[Target#^kept]]" {
				visible = append(visible, "Before")
			}
			for _, text := range visible {
				if !strings.Contains(got.HTML, text) {
					t.Errorf("embedded visible HTML = %q, want %q", got.HTML, text)
				}
			}
			if strings.Contains(got.HTML, "private") || strings.Contains(got.HTML, "hidden") {
				t.Errorf("embedded hidden HTML = %q, want no comment payload", got.HTML)
			}
			if body == "![[Target#Kept]]" && strings.Contains(got.HTML, "Outside.") {
				t.Errorf("section HTML = %q, want no following section", got.HTML)
			}
		})
	}
	linked := r.HTML("Host.md", "", "[[Target#Kept]]", wording.En)
	if diff := cmp.Diff([]render.Diagnostic(nil), linked.Diagnostics); diff != "" {
		t.Errorf("non-embedded target diagnostics (-want +got):\n%s", diff)
	}
}

func TestEmbeddedCommentDiagnosticsFollowExcerpt(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, source, embed string
		want                []render.Diagnostic
	}{
		{
			name:   "whole note container",
			source: "# Title\n\n> Kept. ^b\n> <!-- private\n> hidden\n\nAfter.", embed: "![[T]]",
			want: []render.Diagnostic{{Kind: render.DiagCommentContainerUnclosed, Target: "<!--", Message: "T.md: an unclosed <!-- comment opened at line 4 of the note body hides the rest of its Markdown container"}},
		},
		{
			name:   "block contains container comment",
			source: "# Title\n\n> Kept. ^b\n> <!-- private\n> hidden\n\nAfter.", embed: "![[T#^b]]",
			want: []render.Diagnostic{{Kind: render.DiagCommentContainerUnclosed, Target: "<!--", Message: "T.md: an unclosed <!-- comment opened at line 4 of the note body hides the rest of its Markdown container"}},
		},
		{
			name:   "whole note body-wide HTML",
			source: "# Title\n\nKept. ^b <!-- private\nhidden", embed: "![[T]]",
			want: []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: "T.md: an unclosed <!-- comment opened at line 3 of the note body hides everything after it"}},
		},
		{
			name:   "block contains body-wide HTML",
			source: "# Title\n\nKept. ^b <!-- private\nhidden", embed: "![[T#^b]]",
			want: []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: "T.md: an unclosed <!-- comment opened at line 3 of the note body hides everything after it"}},
		},
		{
			name:   "block contains body-wide percent",
			source: "# Title\n\nKept. ^b %% private\nhidden", embed: "![[T#^b]]",
			want: []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "%%", Message: "T.md: an unclosed %% comment opened at line 3 of the note body hides everything after it"}},
		},
		{
			name:   "container before block",
			source: "> Before\n> <!-- private\n> hidden\n\nKept. ^b", embed: "![[T#^b]]",
		},
		{
			name:   "container after block",
			source: "Kept. ^b\n\n> After\n> <!-- private\n> hidden\n\nEnd.", embed: "![[T#^b]]",
		},
		{
			name:   "body-wide after block",
			source: "Kept. ^b\n\n<!-- private\nhidden", embed: "![[T#^b]]",
		},
		{
			name:   "same visible block in another section",
			source: "## Other\n> Kept.\n> <!-- private\n> hidden\n\n## Selected\n> Kept.\n> <!-- closed -->\n> hidden\n", embed: "![[T#Selected]]",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "T.md"}}, nil, transclusions{"T.md": tt.source})
			got := r.HTML("Host.md", "", tt.embed, wording.En)
			if diff := cmp.Diff(tt.want, got.Diagnostics); diff != "" {
				t.Errorf("embedded excerpt diagnostics (-want +got):\n%s", diff)
			}
			if !strings.Contains(got.HTML, "Kept.") || strings.Contains(got.HTML, "private") {
				t.Errorf("embedded visible words or comment privacy changed: %s", got.HTML)
			}
		})
	}
}

func TestHTMLContainerCommentLiteralControls(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	tests := []struct {
		name, body, visible string
		diagnostics         []render.Diagnostic
	}{
		{name: "closed list", body: "- Item\n  <!-- private -->\n\nAfter.", visible: "After."},
		{name: "escaped unclosed", body: `\<!-- literal`, visible: "&lt;!-- literal"},
		{name: "inline unclosed code", body: "`<!-- literal`", visible: "<code>&lt;!-- literal</code>"},
		{name: "wrapped unclosed code", body: "`begin\nmiddle <!-- literal`", visible: "<code>begin middle &lt;!-- literal</code>"},
		{name: "fenced unclosed code", body: "```text\n<!-- literal\n```", visible: "&lt;!-- literal"},
		{name: "indented unclosed code", body: "    <!-- literal", visible: "<pre><code>&lt;!-- literal"},
		{name: "read aloud marker", body: "<!-- read-aloud: ja -->\nSpoken.", visible: "Spoken."},
		{
			name:        "quote reaches body end",
			body:        "> Before\n> <!-- private\n> secret",
			visible:     "Before",
			diagnostics: []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides everything after it"}},
		},
		{
			name:        "list reaches body end",
			body:        "- Item\n  <!-- private\n  secret",
			visible:     "Item",
			diagnostics: []render.Diagnostic{{Kind: render.DiagCommentUnclosed, Target: "<!--", Message: "an unclosed <!-- comment opened at line 2 of the note body hides everything after it"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.En)
			if !strings.Contains(got.HTML, tt.visible) {
				t.Errorf("literal HTML = %q, want %q", got.HTML, tt.visible)
			}
			if diff := cmp.Diff(tt.diagnostics, got.Diagnostics); diff != "" {
				t.Errorf("literal comment diagnostics (-want +got):\n%s", diff)
			}
		})
	}
}
