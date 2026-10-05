package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestMarkdownLinksReachTheirCapturedFile(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "vault root", path: "20%20Areas/Databases/vacuum.md"},
		{name: "shortest name", path: "vacuum.md"},
		{name: "note relative", path: "../../20%20Areas/Databases/vacuum.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "20 Areas/Databases/vacuum.md"}}, nil, nil)
			got := r.HTML("Inbox/sub/source.md", "", "[Vacuum]("+tt.path+")\n", wording.En)
			want := "<p><a href=\"/notes/20%20Areas/Databases/vacuum.md\">Vacuum</a></p>\n"
			if diff := cmp.Diff(want, got.HTML); diff != "" {
				t.Errorf("Markdown target (-want +got):\n%s", diff)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("valid Markdown target diagnostics = %+v, want none", got.Diagnostics)
			}
		})
	}
}

func TestMarkdownLinksKeepLabelsAndSourceOwnership(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Host/Other.md"}, {RelPath: "Embedded/Child.md"}, {RelPath: "Embedded/Peer.md"}, {RelPath: "Docs/a:b.md"}}, []string{"Files/A#B?C.txt"}, transclusions{"Embedded/Child.md": "[child](Peer.md)\n"})
	for _, tt := range []struct{ name, owner, body, want string }{
		{name: "formatted inline label", owner: "Host/here.md", body: "[**bold**](Other.md \"authored title\")", want: `<a href="/notes/Host/Other.md" title="authored title"><strong>bold</strong></a>`},
		{name: "reference definition", owner: "Host/here.md", body: "[label][dest]\n\n[dest]: Other.md\n", want: `<a href="/notes/Host/Other.md">label</a>`},
		{name: "empty label", owner: "Host/here.md", body: "[](Other.md)", want: `<a href="/notes/Host/Other.md"></a>`},
		{name: "qualified colon filename", owner: "Host/here.md", body: "[colon](Docs/a%3Ab.md)", want: `<a href="/notes/Docs/a:b.md">colon</a>`},
		{name: "resource and suffix", owner: "Host/here.md", body: "[file](A%23B%3FC.txt?one=1&amp;two=2#part)", want: `<a href="/raw/Files/A%23B%3FC.txt?one=1&amp;two=2#part">file</a>`},
		{name: "child source directory", owner: "Host/here.md", body: "![[Child]]", want: `<a href="/notes/Embedded/Peer.md">child</a>`},
		{name: "callout body", owner: "Host/here.md", body: "> [!note] Title\n> [inside](Other.md)\n", want: `<a href="/notes/Host/Other.md">inside</a>`},
		{name: "footnote body", owner: "Host/here.md", body: "text[^n]\n\n[^n]: [inside](Other.md)\n", want: `<a href="/notes/Host/Other.md">inside</a>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML(tt.owner, "", tt.body, wording.En)
			if !strings.Contains(got.HTML, tt.want) || len(got.Diagnostics) != 0 {
				t.Errorf("Markdown label/owner missing %q or unexpected diagnostics: %+v", tt.want, got)
			}
		})
	}
}

func TestMarkdownAmbiguityNeverChoosesAFile(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Host/Other.md"}, {RelPath: "Else/Other.md"}}, nil, nil)
	got := r.HTML("Host/here.md", "", "[**choice**](Other.md)", wording.En)
	want := []render.Diagnostic{{Kind: render.DiagMarkdownAmbiguous, Target: "Other.md", Message: `Markdown path "Other.md" names several files: Else/Other.md, Host/Other.md`}}
	if diff := cmp.Diff(want, got.Diagnostics); diff != "" {
		t.Errorf("Markdown ambiguity (-want +got):\n%s", diff)
	}
	if strings.Contains(got.HTML, "<a ") || !strings.Contains(got.HTML, `class="wikilink-ambiguous"`) || !strings.Contains(got.HTML, "<strong>choice</strong>") || !strings.Contains(got.HTML, `class="y-offscreen"`) {
		t.Errorf("ambiguous label lost its structure or picked a link:\n%s", got.HTML)
	}
}

func TestMarkdownRelativeLinksBelongToEachEmbeddedBody(t *testing.T) {
	t.Parallel()
	const link = "[peer](../Peers/Peer.md)\n"
	r := newRenderer(t, []graph.NoteInput{
		{RelPath: "Host/Peers/Peer.md"},
		{RelPath: "Embedded/Sub/Child.md"},
		{RelPath: "Embedded/Peers/Peer.md"},
	}, nil, transclusions{"Embedded/Sub/Child.md": link})
	got := r.HTML("Host/Sub/here.md", "", link+"\n![[Child]]\n", wording.En)
	for _, href := range []string{"/notes/Host/Peers/Peer.md", "/notes/Embedded/Peers/Peer.md"} {
		if strings.Count(got.HTML, `href="`+href+`"`) != 1 {
			t.Errorf("source-owned relative href %s must appear once:\n%s", href, got.HTML)
		}
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("source-owned relative links produced diagnostics: %+v", got.Diagnostics)
	}
}

func TestMarkdownAliasesNeverPretendToBePaths(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Real.md", Aliases: []string{"Ghost.md"}}}, nil, nil)
	got := r.HTML("Host/here.md", "", "[ghost](Ghost.md)", wording.En)
	want := []render.Diagnostic{{Kind: render.DiagMarkdownBroken, Target: "Ghost.md", Message: `Markdown path "Ghost.md" resolves to no file`}}
	if diff := cmp.Diff(want, got.Diagnostics); diff != "" {
		t.Errorf("Markdown alias (-want +got):\n%s", diff)
	}
	if strings.Contains(got.HTML, "<a ") {
		t.Errorf("an alias was treated as a filesystem path: %s", got.HTML)
	}
}

func TestMarkdownLinkResolutionLeavesOtherSyntaxAlone(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Host/Other.md"}}, []string{"Host/picture.png"}, nil)
	for _, tt := range []struct{ name, body, want string }{
		{name: "inline code", body: "`[code](Other.md)`", want: "<code>[code](Other.md)</code>"},
		{name: "code fence", body: "```text\n[fence](Other.md)\n```", want: "[fence](Other.md)"},
		{name: "authored HTML", body: `<a href="Other.md">raw</a>`, want: `&lt;a href=&quot;Other.md&quot;&gt;raw&lt;/a&gt;`},
		{name: "fragment", body: "[fragment](#part)", want: `<a href="#part">fragment</a>`},
		{name: "existing product route", body: "[served](/notes/Host/Other.md)", want: `<a href="/notes/Host/Other.md">served</a>`},
		{name: "image", body: "![picture](picture.png)", want: `src="/raw/Host/picture.png"`},
		{name: "remote autolink", body: "<https://example.test/Other.md>", want: `href="https://example.test/Other.md" target="_blank"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Host/here.md", "", tt.body, wording.En)
			if !strings.Contains(got.HTML, tt.want) || len(got.Diagnostics) != 0 {
				t.Errorf("neighbor syntax changed: want %q, got %+v", tt.want, got)
			}
		})
	}
}

func TestMarkdownInvalidAndOutsidePathsKeepTheirLabels(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Other.md"}}, nil, nil)
	for _, destination := range []string{"Bad%2.md", "%2e%2e/%2e%2e/Other.md", "../../Other.md"} {
		t.Run(destination, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Host/here.md", "", "[**retained**]("+destination+")", wording.En)
			if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != render.DiagMarkdownBroken || got.Diagnostics[0].Target != destination || strings.Contains(got.HTML, "<a ") || !strings.Contains(got.HTML, "<strong>retained</strong>") {
				t.Errorf("invalid/outside label and source identity = %+v", got)
			}
		})
	}
}
