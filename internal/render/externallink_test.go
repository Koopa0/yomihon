package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// The attributes and the sentence are written out here rather than read back
// from the renderer, so a lock that agreed with whatever the renderer wrote
// cannot pass for one that holds the bytes.
const (
	externalAttributes = ` target="_blank" rel="external noopener noreferrer" referrerpolicy="no-referrer"`
	noteZh             = `<span class="y-offscreen">（另開分頁）</span>`
	noteEn             = `<span class="y-offscreen"> (opens in a new tab)</span>`
)

// TestALinkThatLeavesTheLibraryOpensInANewTab holds every way an absolute http
// or https destination can be written to the same markup: a link, a link with a
// title, a scheme written in capitals, an autolink in angle brackets, and the
// bare address and www form the GFM autolink extension finds in prose.
func TestALinkThatLeavesTheLibraryOpensInANewTab(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "a link",
			body: "[docs](https://example.org/a?x=1&y=2)\n",
			want: `<a href="https://example.org/a?x=1&amp;y=2"` + externalAttributes + `>docs` + noteZh + `</a>`,
		},
		{
			name: "a plain http link",
			body: "[docs](http://example.org/)\n",
			want: `<a href="http://example.org/"` + externalAttributes + `>docs` + noteZh + `</a>`,
		},
		{
			name: "a link with a title",
			body: "[docs](https://example.org/ \"The docs\")\n",
			want: `<a href="https://example.org/" title="The docs"` + externalAttributes + `>docs` + noteZh + `</a>`,
		},
		{
			name: "a scheme in capitals",
			body: "[docs](HTTPS://Example.org/)\n",
			want: `<a href="HTTPS://Example.org/"` + externalAttributes + `>docs` + noteZh + `</a>`,
		},
		{
			name: "an autolink in angle brackets",
			body: "<https://example.org/auto>\n",
			want: `<a href="https://example.org/auto"` + externalAttributes + `>https://example.org/auto` + noteZh + `</a>`,
		},
		{
			name: "a bare address in prose",
			body: "see https://example.org/bare for more\n",
			want: `<a href="https://example.org/bare"` + externalAttributes + `>https://example.org/bare` + noteZh + `</a>`,
		},
		{
			name: "a www address in prose",
			body: "see www.example.org for more\n",
			want: `<a href="http://www.example.org"` + externalAttributes + `>www.example.org` + noteZh + `</a>`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			if !strings.Contains(got, tt.want) {
				t.Errorf("missing %q in:\n%s", tt.want, got)
			}
			// The attributes belong to the one link and are written once.
			if n := strings.Count(got, `target="_blank"`); n != 1 {
				t.Errorf("%d links open in a new tab, want 1:\n%s", n, got)
			}
		})
	}
}

// TestTheSentenceIsSaidInTheLanguageOfThePage holds the words a listener is
// told to the language the page was rendered in, and holds them to what the
// reader would be told in each, with the mark each language puts around them.
func TestTheSentenceIsSaidInTheLanguageOfThePage(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	for _, tt := range []struct {
		lang wording.Lang
		want string
	}{
		{wording.ZhHant, `>docs` + noteZh + `</a>`},
		{wording.En, `>docs` + noteEn + `</a>`},
	} {
		got := r.HTML("note.md", "", "[docs](https://example.org/)\n", tt.lang).HTML
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: missing %q in:\n%s", tt.lang, tt.want, got)
		}
	}
}

// TestALinkThatStaysInTheLibraryIsLeftAlone holds the other half. A note, a
// section of this one, a path, an address with no scheme, an address that names
// another scheme, a wikilink and an image that is a link already are written as
// they were: no target, no new relation, no sentence. A renderer that marked
// every link would pass the test above and tell a reader nothing.
func TestALinkThatStaysInTheLibraryIsLeftAlone(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)

	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"another note by path", "[n](Notes/Other.md)\n", `<a href="Notes/Other.md">n</a>`},
		{"a section of this note", "[s](#section)\n", `<a href="#section">s</a>`},
		{"a path from the root", "[p](/notes/Other.md)\n", `<a href="/notes/Other.md">p</a>`},
		{"an address with no scheme", "[h](//example.org/x)\n", `<a href="//example.org/x">h</a>`},
		{"a mail address", "[m](mailto:a@example.org)\n", `<a href="mailto:a@example.org">m</a>`},
		{"a mail autolink", "<a@example.org>\n", `<a href="mailto:a@example.org">a@example.org</a>`},
		{"a wikilink", "[[Target]]\n", `<a href="/notes/Target.md" class="wikilink">Target</a>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.ZhHant).HTML
			if !strings.Contains(got, tt.want) {
				t.Errorf("missing %q in:\n%s", tt.want, got)
			}
			for _, banned := range []string{`target=`, `noopener`, noteZh} {
				if strings.Contains(got, banned) {
					t.Errorf("a link that stays in the library carries %q:\n%s", banned, got)
				}
			}
		})
	}
}

// TestADangerousDestinationIsNotMadeAnExternalLink holds the destinations
// goldmark already refuses. They keep the empty address it writes for them and
// gain nothing: a link that opens nowhere is not one that opens somewhere else.
func TestADangerousDestinationIsNotMadeAnExternalLink(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	for _, destination := range []string{
		"javascript:globalThis.linkRan=true",
		"JavaScript:alert(1)",
		"vbscript:msgbox(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"file:///etc/passwd",
	} {
		got := r.HTML("note.md", "", "[x]("+destination+")\n", wording.ZhHant).HTML
		if !strings.Contains(got, `<a href="">x</a>`) {
			t.Errorf("%s: want the empty address and nothing more in:\n%s", destination, got)
		}
		for _, banned := range []string{`target=`, `noopener`, noteZh, `javascript:`} {
			if strings.Contains(got, banned) {
				t.Errorf("%s: the link carries %q:\n%s", destination, banned, got)
			}
		}
	}
}

// TestARemoteImageIsStillTheLinkItWas holds what the image rule already did. An
// image that cannot be shown becomes a link carrying its own relation and no
// target, and this change leaves it so: it is an alternative to a picture, and
// a reader who follows it has not asked to leave the page the way one who
// followed a link written in the text has.
func TestARemoteImageIsStillTheLinkItWas(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	got := r.HTML("note.md", "", "![chart](https://example.org/chart.png)\n", wording.ZhHant).HTML
	const want = `<a href="https://example.org/chart.png" rel="external noreferrer" referrerpolicy="no-referrer">chart</a>`
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
	if strings.Contains(got, `target=`) || strings.Contains(got, "y-offscreen") {
		t.Errorf("the image link gained the external link's markup:\n%s", got)
	}
}

// TestAnExternalLinkInAHeadingDoesNotRenameTheSection holds the contents entry
// and the anchor to the words the reader sees. The sentence a listener is told
// is carried in the element a heading's words are read without, and a section
// called "See Go (opens in a new tab)" would be a name no author wrote.
func TestAnExternalLinkInAHeadingDoesNotRenameTheSection(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		page := r.HTML("note.md", "", "## See [Go](https://go.dev)\n\nbody\n", lang)
		if len(page.TOC) != 1 || page.TOC[0].Text != "See Go" || page.TOC[0].ID != "see-go" {
			t.Errorf("%s: the contents entry is %+v, want the words See Go under the id see-go", lang, page.TOC)
		}
		if !strings.Contains(page.HTML, externalAttributes) {
			t.Errorf("%s: the link in the heading does not open in a new tab:\n%s", lang, page.HTML)
		}
	}
}
