package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

// destBody is one note carrying block addresses of every shape a reader
// actually writes: at the end of a paragraph, on a list item, inside a quote,
// and under a paragraph that runs over several lines.
const destBody = "FIRST paragraph.\n\n" +
	"SECOND the addressed paragraph. ^quux\n\n" +
	"- item one ^itm\n" +
	"- item two\n\n" +
	"> quoted line ^qq\n\n" +
	"LAST line one\nline two\n^under\n"

func blockRenderer(t *testing.T) *render.Pipeline {
	t.Helper()
	return newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": destBody})
}

// A link written at a block says which paragraph it means. Without an anchor
// on the destination the reader was told they were going to one block and
// arrived at the top of the note, with nothing on screen saying so.
func TestBlockAddressIsAnchoredOnItsBlock(t *testing.T) {
	t.Parallel()

	r := blockRenderer(t)
	page := r.HTML("B.md", "", destBody)

	anchors := []struct {
		name string
		want string
	}{
		{name: "a paragraph", want: `<p>SECOND the addressed paragraph. <span id="^quux">^quux</span></p>`},
		{name: "a list item", want: `<li>item one <span id="^itm">^itm</span></li>`},
		{name: "a quoted line", want: `<span id="^qq">^qq</span>`},
		{name: "a marker written under its block", want: `<span id="^under">^under</span>`},
	}
	for _, tt := range anchors {
		if !strings.Contains(page.HTML, tt.want) {
			t.Errorf("%s carries no anchor; want %s in:\n%s", tt.name, tt.want, page.HTML)
		}
	}
}

// The marker is left on screen exactly as the author wrote it. Hiding it is a
// separate question about what a reader should see, and this change does not
// answer it.
func TestBlockAddressStaysVisible(t *testing.T) {
	t.Parallel()

	r := blockRenderer(t)
	page := r.HTML("B.md", "", destBody)
	for _, marker := range []string{"^quux", "^itm", "^qq", "^under"} {
		if !strings.Contains(page.HTML, marker) {
			t.Errorf("the marker %q is no longer on the page:\n%s", marker, page.HTML)
		}
	}
}

// A link to a block reaches the block. The address travels percent-escaped, so
// a block name is safe inside both the attribute and the URL, and the page's
// own anchor is what the browser lands on after it decodes it.
func TestBlockLinkAddressesTheBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the way Obsidian writes one",
			body: "[[B#^quux]]\n",
			want: `<a href="/notes/B.md#%5Equux" class="wikilink">B#^quux</a>`,
		},
		{
			name: "the bare form",
			body: "[[B^quux]]\n",
			want: `<a href="/notes/B.md#%5Equux" class="wikilink">B^quux</a>`,
		},
		{
			name: "written in different capitals from the marker",
			body: "[[B#^QUUX]]\n",
			want: `<a href="/notes/B.md#%5Equux" class="wikilink">B#^QUUX</a>`,
		},
		{
			name: "a block address beside a section name",
			body: "[[B^quux#Internals]]\n",
			want: `<a href="/notes/B.md#%5Equux" class="wikilink">B^quux#Internals</a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := blockRenderer(t)
			got := r.HTML("note.md", "", tt.body)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q) missing %q:\n%s", tt.body, tt.want, got.HTML)
			}
			for _, d := range got.Diagnostics {
				if d.Kind == render.DiagLinkFragmentMissing {
					t.Errorf("a block the note carries was reported absent: %s", d.Message)
				}
			}
		})
	}
}

// A block the destination does not carry gets no fragment and says so. An
// address that reaches nothing is the one thing worse than no address: the
// reader is told where they are going and arrives somewhere else.
func TestBlockLinkWithNoSuchBlockDegradesAndReports(t *testing.T) {
	t.Parallel()

	r := blockRenderer(t)
	got := r.HTML("note.md", "", "[[B#^nope]]\n")

	if !strings.Contains(got.HTML, `<a href="/notes/B.md" class="wikilink">`) {
		t.Errorf("the link did not fall back to the note itself:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "#%5Enope") {
		t.Errorf("the link kept an address the destination has no anchor for:\n%s", got.HTML)
	}

	var messages []string
	for _, d := range got.Diagnostics {
		if d.Kind == render.DiagLinkFragmentMissing {
			messages = append(messages, d.Message)
		}
	}
	want := `no block in "B.md" matched "^nope"; the link leads to the note itself`
	if len(messages) != 1 || messages[0] != want {
		t.Errorf("fragment diagnostics = %q, want exactly one reading %q", messages, want)
	}
}

// Two blocks written under one name are one name: the first is where the
// address leads, the same reading the excerpt scan takes, and the second
// carries no anchor rather than a second element answering to it.
func TestRepeatedBlockAddressBelongsToTheFirstBlock(t *testing.T) {
	t.Parallel()

	const body = "FIRST paragraph. ^dup\n\nSECOND paragraph. ^dup\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": body})
	page := r.HTML("B.md", "", body)

	if !strings.Contains(page.HTML, `<p>FIRST paragraph. <span id="^dup">^dup</span></p>`) {
		t.Errorf("the first block did not take the address:\n%s", page.HTML)
	}
	if strings.Count(page.HTML, `id="^dup"`) != 1 {
		t.Errorf("the address is stamped %d times, want once:\n%s", strings.Count(page.HTML, `id="^dup"`), page.HTML)
	}
	if !strings.Contains(page.HTML, "SECOND paragraph. ^dup") {
		t.Errorf("the second block lost its text:\n%s", page.HTML)
	}
}

// A fragment is an offset into a page, so it is written only for a name that
// placed exactly one file. A name that placed none has no page, and one that
// placed several is never chosen between here.
func TestBlockLinkFragmentOnlyWhenTheNoteIsCertain(t *testing.T) {
	t.Parallel()

	t.Run("an unresolved target gets no link at all", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, nil, transclusions{"B.md": destBody})
		got := r.HTML("note.md", "", "[[Ghost#^quux]]\n")
		if strings.Contains(got.HTML, "href=") {
			t.Errorf("an unresolved target must not become a link at all:\n%s", got.HTML)
		}
	})

	t.Run("an ambiguous target carrying the block still gets no link", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Foo.md"}, {RelPath: "b/Foo.md"}}, nil,
			transclusions{"a/Foo.md": destBody, "b/Foo.md": destBody})
		got := r.HTML("note.md", "", "[[Foo#^quux]]\n")
		if strings.Contains(got.HTML, "href=") {
			t.Errorf("an ambiguous target must not become a link at all:\n%s", got.HTML)
		}
	})

	t.Run("a resource that is not a note gets no fragment", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, []string{"Files/plan.pdf"}, nil)
		got := r.HTML("note.md", "", "[[plan.pdf#^quux]]\n")
		if strings.Contains(got.HTML, "#%5Equux") {
			t.Errorf("a file nothing inside is addressable from here kept a fragment:\n%s", got.HTML)
		}
	})
}

// A marker shown inside a fenced code block is quoted text, not an address, on
// both surfaces: the page stamps nothing for it and a link naming it leads to
// the note itself.
func TestBlockAddressInsideAFenceIsNotAnAddress(t *testing.T) {
	t.Parallel()

	const body = "```\nsample line ^fenced\n```\n\nordinary paragraph.\n"
	r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": body})

	page := r.HTML("B.md", "", body)
	if strings.Contains(page.HTML, `id="^fenced"`) {
		t.Errorf("a marker inside a fence was made into an anchor:\n%s", page.HTML)
	}

	got := r.HTML("note.md", "", "[[B#^fenced]]\n")
	if strings.Contains(got.HTML, "#%5Efenced") {
		t.Errorf("a link named a marker the page shows as code:\n%s", got.HTML)
	}
}

// Every line the excerpt scan can match is a line the page anchors, so a link
// whose fragment survives always has somewhere to land. These are the shapes
// where the two could most easily disagree.
func TestBlockAddressAndExcerptScanAgreeOnUnusualLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		address    string
		addressed  bool // the shape carries a block address at all
		reasonWhen string
	}{
		{
			name: "a heading line", address: "^hd", addressed: true,
			body: "## Section ^hd\n\nbody text.\n",
		},
		{
			name: "a line underlined into a heading", address: "^set", addressed: true,
			body: "Section ^set\n===\n\nbody text.\n",
		},
		{
			name: "an indented code line", address: "^ind", addressed: true,
			body: "paragraph.\n\n    sample ^ind\n",
		},
		{
			name: "a line of authored markup", address: "^raw", addressed: true,
			body: "<div>\nraw ^raw\n</div>\n",
		},
		{
			name: "a callout body line", address: "^cal", addressed: true,
			body: "> [!note] Title\n> quoted body ^cal\n",
		},
		{
			name: "a callout's own opening line", address: "^ttl", addressed: false,
			body: "> [!note] Title ^ttl\n> quoted body\n", reasonWhen: "the line is the callout's title",
		},
		{
			name: "a fenced line inside a callout", address: "^cf", addressed: false,
			body: "> [!note] T\n> ```\n> code ^cf\n> ```\n", reasonWhen: "the line is code",
		},
		{
			name: "a table row", address: "^tbl", addressed: false,
			body: "| a | b ^tbl |\n| --- | --- |\n| c | d |\n", reasonWhen: "the address sits inside a cell",
		},
		{
			name: "a table row with the address after its last cell", address: "^tt", addressed: false,
			body: "| a | b |\n| --- | --- |\n| c | d | ^tt\n", reasonWhen: "the row drops what follows its last cell",
		},
		{
			name: "a quoted table row", address: "^qt", addressed: false,
			body: "> [!note] T\n> | a | b |\n> | --- | --- |\n> | c | d | ^qt\n", reasonWhen: "the row drops what follows its last cell",
		},
		{
			name: "an address shown in a code span", address: "^cs", addressed: false,
			body: "text `^cs`\n", reasonWhen: "the address is quoted text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": tt.body})
			page := r.HTML("B.md", "", tt.body)
			got := r.HTML("note.md", "", "[[B#"+tt.address+"]]\n")

			hasFragment := strings.Contains(got.HTML, "/notes/B.md#")
			hasAnchor := strings.Contains(page.HTML, `id="`+tt.address+`"`)

			if hasFragment && !hasAnchor {
				t.Errorf("the link carries a fragment the page has no anchor for\nlink: %s\npage: %s", got.HTML, page.HTML)
			}
			if hasAnchor != tt.addressed {
				t.Errorf("the page anchors this address = %v, want %v (%s):\n%s", hasAnchor, tt.addressed, tt.reasonWhen, page.HTML)
			}
			if hasFragment != tt.addressed {
				t.Errorf("the link addresses this block = %v, want %v (%s):\n%s", hasFragment, tt.addressed, tt.reasonWhen, got.HTML)
			}
		})
	}
}

// A transcluded body's blocks belong to the note it came from, not to the note
// being read, so an excerpt brings no anchors with it. Left in, an address
// this note does not have would land the reader inside someone else's block.
func TestATranscludedBodyBringsNoBlockAnchors(t *testing.T) {
	t.Parallel()

	r := blockRenderer(t)
	got := r.HTML("note.md", "", "![[B]]\n")

	if !strings.Contains(got.HTML, "SECOND the addressed paragraph.") {
		t.Fatalf("the embed did not reach the body at all:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, `id="^quux"`) {
		t.Errorf("an embedded note's block address was claimed by the page reading it:\n%s", got.HTML)
	}
}
