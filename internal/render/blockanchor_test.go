package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
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
	page := r.HTML("B.md", "", destBody, wording.ZhHant)

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
	page := r.HTML("B.md", "", destBody, wording.ZhHant)
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
			got := r.HTML("note.md", "", tt.body, wording.ZhHant)
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
	got := r.HTML("note.md", "", "[[B#^nope]]\n", wording.ZhHant)

	if !strings.Contains(got.HTML, `<a href="/notes/B.md" class="wikilink wikilink-degraded"`) {
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
	page := r.HTML("B.md", "", body, wording.ZhHant)

	if !strings.Contains(page.HTML, `<p>FIRST paragraph. <span id="^dup">^dup</span></p>`) {
		t.Errorf("the first block did not take the address:\n%s", page.HTML)
	}
	if strings.Count(page.HTML, `id="^dup"`) != 1 {
		t.Errorf("the address is stamped %d times, want once:\n%s", strings.Count(page.HTML, `id="^dup"`), page.HTML)
	}
	if !strings.Contains(page.HTML, `<p>SECOND paragraph. <span>^dup</span></p>`) {
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
		got := r.HTML("note.md", "", "[[Ghost#^quux]]\n", wording.ZhHant)
		if strings.Contains(got.HTML, "href=") {
			t.Errorf("an unresolved target must not become a link at all:\n%s", got.HTML)
		}
	})

	t.Run("an ambiguous target carrying the block still gets no link", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, []graph.NoteInput{{RelPath: "a/Foo.md"}, {RelPath: "b/Foo.md"}}, nil,
			transclusions{"a/Foo.md": destBody, "b/Foo.md": destBody})
		got := r.HTML("note.md", "", "[[Foo#^quux]]\n", wording.ZhHant)
		if strings.Contains(got.HTML, "href=") {
			t.Errorf("an ambiguous target must not become a link at all:\n%s", got.HTML)
		}
	})

	t.Run("a resource that is not a note gets no fragment", func(t *testing.T) {
		t.Parallel()
		r := newRenderer(t, nil, []string{"Files/plan.pdf"}, nil)
		got := r.HTML("note.md", "", "[[plan.pdf#^quux]]\n", wording.ZhHant)
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

	page := r.HTML("B.md", "", body, wording.ZhHant)
	if strings.Contains(page.HTML, `id="^fenced"`) {
		t.Errorf("a marker inside a fence was made into an anchor:\n%s", page.HTML)
	}

	got := r.HTML("note.md", "", "[[B#^fenced]]\n", wording.ZhHant)
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
		{
			name: "an interior caret in a code span", address: "^right`", addressed: false,
			body: "The XOR expression is `result := left ^right`\n", reasonWhen: "the caret is inside a code span",
		},
		{
			name: "an interior caret in a code span written across lines", address: "^right`", addressed: false,
			body: "The XOR expression is `result := left\n^right`\n", reasonWhen: "the caret is inside a code span",
		},
		{
			name: "an interior caret on a code span's opening line", address: "^right", addressed: false,
			body: "The XOR expression is `result := left ^right\nmore`\n", reasonWhen: "the caret is inside a code span",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": tt.body})
			page := r.HTML("B.md", "", tt.body, wording.ZhHant)
			got := r.HTML("note.md", "", "[[B#"+tt.address+"]]\n", wording.ZhHant)

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

// An interior caret in a code span is an expression being shown, not a block
// address, however the author wrapped the span: the CommonMark spec lets a
// line ending stand inside one, and goldmark draws it as a space. The tail
// match would otherwise take the closing backtick with it, leave goldmark an
// unmatched opener, and invent an id from the caret through that backtick.
// The last row is an ordinary address, which still takes its id: the rule is
// that a span owns its carets, not that carets stopped naming blocks.
func TestInteriorCaretInACodeSpanKeepsTheSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      string
		addressed bool
	}{
		{
			name: "written on one line",
			body: "The XOR expression is `result := left ^right`\n",
			want: "<code>result := left ^right</code>",
		},
		{
			name: "wrapped, the caret opening the second line",
			body: "The XOR expression is `result := left\n^right`\n",
			want: "<code>result := left ^right</code>",
		},
		{
			name: "wrapped, the caret inside the second line",
			body: "The XOR expression is `result := left\nx ^right`\n",
			want: "<code>result := left x ^right</code>",
		},
		{
			name: "wrapped, the caret on the opening line",
			body: "The XOR expression is `result := left ^right\nmore`\n",
			want: "<code>result := left ^right more</code>",
		},
		{
			// The stray backticks on either side pair only if the run of
			// lines a span is looked over is left unbounded, which would
			// swallow an address two paragraphs away from either of them.
			name:      "an address at the end of its paragraph",
			body:      "An aside about the ` character.\n\nreal paragraph ^genuine\n\nAnd another ` one.\n",
			want:      `<span id="^genuine">^genuine</span>`,
			addressed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": tt.body})
			page := r.HTML("B.md", "", tt.body, wording.ZhHant)

			if !strings.Contains(page.HTML, tt.want) {
				t.Errorf("want %s in:\n%s", tt.want, page.HTML)
			}
			if got := strings.Contains(page.HTML, `id="^`); got != tt.addressed {
				t.Errorf("the page stamps a block address = %v, want %v:\n%s", got, tt.addressed, page.HTML)
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
	got := r.HTML("note.md", "", "![[B]]\n", wording.ZhHant)

	if !strings.Contains(got.HTML, "SECOND the addressed paragraph.") {
		t.Fatalf("the embed did not reach the body at all:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, `id="^quux"`) {
		t.Errorf("an embedded note's block address was claimed by the page reading it:\n%s", got.HTML)
	}
}

// A code span is looked for over the run of lines goldmark reads together with
// the address, and that run ends where a block ends, not only at a blank line.
// Two stray backticks an author wrote on either side of a heading, a list
// item, a quote opener, a thematic break or a fence used to pair into a span
// goldmark never draws, and the ordinary paragraph caught between them lost
// its address on all three faces at once: the page stamped no id, the excerpt
// came back not found, and the check called a good link broken. The last two
// rows are a span that really does wrap a line ending, which still owns its
// caret: the window narrowed to the block, it did not shrink to the line.
func TestCodeSpanWindowStopsAtABlockBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  string
		at    int
		owned bool
	}{
		{
			name: "an ATX heading on each side",
			body: "Before `\n## Heading\nA paragraph ^genuine\n## Other\nAfter `\n",
			at:   2,
		},
		{
			// "===" underlines the line above it and nothing else reads it,
			// so this row rests on that shape alone.
			name: "a line underlining a heading on each side",
			body: "Before `\n===\nA paragraph ^genuine\n===\nAfter `\n",
			at:   2,
		},
		{
			// "***" is a thematic break and never an underline, so this row
			// rests on that shape alone.
			name: "a thematic break on each side",
			body: "Before `\n***\nA paragraph ^genuine\n***\nAfter `\n",
			at:   2,
		},
		{
			// A list marker ends the line above it, not the one below: the
			// address is the item's own text, continued lazily.
			name: "a list item on each side",
			body: "Before `\n- an item\nA paragraph ^genuine\n- another item\nAfter `\n",
			at:   2,
		},
		{
			name: "a quote opening under the stray",
			body: "Before `\n> A paragraph ^genuine\n> after `\n",
			at:   1,
		},
		{
			name: "a callout opening under the stray",
			body: "Before `\n> [!note] Title\n> A paragraph ^genuine\n> after `\n",
			at:   2,
		},
		{
			// Tilde fences, so the boundary being read is the fence line and
			// not a backtick run the span reader would count.
			name: "a fenced block on each side",
			body: "Before `\n~~~\ncode\n~~~\nA paragraph ^genuine\n~~~\ncode\n~~~\nAfter `\n",
			at:   4,
		},
		{
			name:  "a span the author wrapped inside one paragraph",
			body:  "The XOR is `left ^genuine\nright` ends it\n",
			at:    0,
			owned: true,
		},
		{
			name:  "a span the author wrapped inside one quote",
			body:  "> The XOR is `left ^genuine\n> right` ends it\n",
			at:    0,
			owned: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addressed := !tt.owned

			if got := render.CodeSpanOwnsBlockAddress(strings.Split(tt.body, "\n"), tt.at); got != tt.owned {
				t.Errorf("a code span owns the address = %v, want %v", got, tt.owned)
			}
			if _, found := render.Excerpt(tt.body, "^genuine"); found != addressed {
				t.Errorf("the excerpt finds the address = %v, want %v", found, addressed)
			}

			r := newRenderer(t, []graph.NoteInput{{RelPath: "B.md"}}, nil, transclusions{"B.md": tt.body})
			page := r.HTML("B.md", "", tt.body, wording.ZhHant)
			if got := strings.Contains(page.HTML, `<span id="^genuine">^genuine</span>`); got != addressed {
				t.Errorf("the page anchors the address = %v, want %v:\n%s", got, addressed, page.HTML)
			}
			if tt.owned && !strings.Contains(page.HTML, "<code>left ^genuine right</code>") {
				t.Errorf("the page no longer draws the span that owns the caret:\n%s", page.HTML)
			}
		})
	}
}
