package render_test

import (
	"regexp"
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

func TestInertFormattingTagsShareReadingWords(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	tests := []struct{ name, body, html, words string }{
		{name: "keyboard", body: "Before <kbd>Control</kbd> after", html: "<p>Before <kbd>Control</kbd> after</p>\n", words: "Before Control after"},
		{name: "subscript", body: "Before H<sub>2</sub>O after", html: "<p>Before H<sub>2</sub>O after</p>\n", words: "Before H2O after"},
		{name: "superscript", body: "Before x<sup>2</sup> after", html: "<p>Before x<sup>2</sup> after</p>\n", words: "Before x2 after"},
		{name: "highlight", body: "Before <mark>marked</mark> after", html: "<p>Before <mark>marked</mark> after</p>\n", words: "Before marked after"},
		{name: "underline", body: "Before <u>under</u> after", html: "<p>Before <u>under</u> after</p>\n", words: "Before under after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if diff := cmp.Diff(tt.html, got.HTML); diff != "" {
				t.Errorf("HTML formatting (-want +got):\n%s", diff)
			}
			if len(got.Diagnostics) != 0 {
				t.Errorf("formatting diagnostics = %v, want none", got.Diagnostics)
			}
			if diff := cmp.Diff(tt.words, render.HeadingWords(tt.body)); diff != "" {
				t.Errorf("HeadingWords formatting (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.words, render.PlainText(tt.body)); diff != "" {
				t.Errorf("PlainText formatting (-want +got):\n%s", diff)
			}
			note := vault.Parse("Notes/Formatting.md", []byte("---\ntitle: Format\n---\n"+tt.body))
			idx := lexical.NewIndex([]lexical.Document{lexical.DocumentFromNote(note)}, schema.ArtifactPolicy{})
			answer, err := idx.Search(lexical.Parse(`"`+tt.words+`"`), -1)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff([]string{"Notes/Formatting.md"}, searchPaths(answer.Results)); diff != "" {
				t.Errorf("Search formatting words (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInertFormattingTagsRefuseAttributes(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	tests := []struct{ name, body, want string }{
		{name: "keyboard event", body: "Before <kbd onclick=bad()>key</kbd> after", want: "<p>Before &lt;kbd onclick=bad()&gt;key</kbd> after</p>\n"},
		{name: "subscript class", body: "Before <sub class=sample>2</sub> after", want: "<p>Before &lt;sub class=sample&gt;2</sub> after</p>\n"},
		{name: "superscript language", body: `Before <sup lang="en">2</sup> after`, want: "<p>Before &lt;sup lang=&quot;en&quot;&gt;2</sup> after</p>\n"},
		{name: "highlight style", body: "Before <mark style=color:red>word</mark> after", want: "<p>Before &lt;mark style=color:red&gt;word</mark> after</p>\n"},
		{name: "underline id", body: "Before <u id=sample>word</u> after", want: "<p>Before &lt;u id=sample&gt;word</u> after</p>\n"},
		{name: "closing attribute", body: "Before <kbd>key</kbd class=sample> after", want: "<p>Before &lt;kbd&gt;key&lt;/kbd class=sample&gt; after</p>\n"},
		{name: "outside ruled set", body: "Before <small>small</small> <s>strike</s> after", want: "<p>Before &lt;small&gt;small&lt;/small&gt; &lt;s&gt;strike&lt;/s&gt; after</p>\n"},
		{name: "nested unsafe", body: "Before <kbd><button onclick=bad()>key</button></kbd> after", want: "<p>Before <kbd>&lt;button onclick=bad()&gt;key&lt;/button&gt;</kbd> after</p>\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if diff := cmp.Diff(tt.want, got.HTML); diff != "" {
				t.Errorf("HTML attributed formatting (-want +got):\n%s", diff)
			}
			if strings.Contains(got.HTML, "<kbd onclick=") {
				t.Errorf("attributed keyboard markup became active: %s", got.HTML)
			}
		})
	}
}

// TestAFormattingOpenerNeedsItsCloser holds the container rule: an opener is
// markup only when a closer of its own name follows it in the same paragraph,
// cell, heading or HTML block. Anything else is shown as the text it is, so no
// element the author opened can outlive the place they wrote it.
func TestAFormattingOpenerNeedsItsCloser(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	tests := []struct{ name, body, want string }{
		{name: "unclosed in a paragraph", body: "para <u>open\n\nLater.", want: "<p>para &lt;u&gt;open</p>\n<p>Later.</p>\n"},
		{name: "self-closing", body: "self <u/>closing", want: "<p>self &lt;u/&gt;closing</p>\n"},
		{name: "second opener unclosed", body: "<u>a</u> and <u>b", want: "<p><u>a</u> and &lt;u&gt;b</p>\n"},
		{name: "inner opener claims the closer", body: "<mark>a <mark>b</mark>", want: "<p>&lt;mark&gt;a <mark>b</mark></p>\n"},
		{name: "closer of another name", body: "<u>a</mark>", want: "<p>&lt;u&gt;a</mark></p>\n"},
		{name: "closer before opener", body: "</kbd>a<kbd>", want: "<p></kbd>a&lt;kbd&gt;</p>\n"},
		{name: "closer outside emphasis", body: "**<sup>a**</sup>", want: "<p><strong><sup>a</strong></sup></p>\n"},
		{name: "closer inside an image description", body: "<u>a ![x</u>](p.png)", want: "<p>&lt;u&gt;a <img src=\"/raw/p.png\" alt=\"x\"></p>\n"},
		{name: "closer in the next paragraph", body: "<sub>a\n\nb</sub>", want: "<p>&lt;sub&gt;a</p>\n<p>b</sub></p>\n"},
		{name: "unclosed html block", body: "<u>\nblock\n\nafter", want: "&lt;u&gt;\nblock\n<p>after</p>\n"},
		{name: "paired html block", body: "<u>\nblock</u>\n\nafter", want: "<u>\nblock</u>\n<p>after</p>\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if diff := cmp.Diff(tt.want, got.HTML); diff != "" {
				t.Errorf("unpaired formatting (-want +got):\n%s", diff)
			}
		})
	}

	table := r.HTML("Note.md", "", "| <u>a | b</u> |\n|---|---|\n| c | d |\n", wording.ZhHant).HTML
	if strings.Contains(table, "<u>") || !strings.Contains(table, "&lt;u&gt;a") {
		t.Errorf("an opener paired with a closer in another cell: %s", table)
	}

	for raw, want := range map[string]string{"A <u>b": "A <u>b", "A <u>b</u>": "A b", "A <u/>b": "A <u/>b"} {
		if got := render.HeadingWords(raw); got != want {
			t.Errorf("HeadingWords(%q) = %q, want %q, the words the page shows", raw, got, want)
		}
	}
}

// TestAHeadingPairsOnlyTheTagsThePageParses holds the heading's name to the id
// the page stamps on it. A closer written in a code span, after a backslash, or
// inside a link's label is text on the page, not a tag, so it cannot claim an
// opener there; a name that paired over the raw bytes would drop words the
// reader sees and send a link copied off the contents list to the top of the
// note. A bare '<' is text on the page too, so it cannot swallow the tag after
// it, and an underlined heading pairs across all of its lines. A name with a
// line break is written as an underlined heading.
func TestAHeadingPairsOnlyTheTagsThePageParses(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Other.md"}}, nil, nil)
	stamped := regexp.MustCompile(`<h[1-6][^>]* id="([^"]*)"`)
	for _, heading := range []string{
		// A closer the page reads as text.
		"<u>head `</u>`",
		"`<u>` head </u>",
		`<u>a \</u>`,
		`<u>a \</u></u>`,
		`<u>a\\</u>`,
		`\<u>a</u>`,
		"``a`</u>`` <u>b</u>",
		"<u>a <!-- < --> b</u>",
		// A bare '<' before a tag.
		"<kbd>Shift</kbd> + <kbd><</kbd>",
		"<kbd><</kbd>",
		"<kbd>Ctrl</kbd>+<kbd><</kbd>+<kbd>></kbd>",
		"<u>a < b</u>",
		"<u>x<y</u>",
		"<u>a</u> < b > c",
		"<<u>a</u>",
		"<u>a</u> <",
		"<sub>i</sub> < <sup>2</sup> <u>t < s</u>",
		"x < <ruby>漢<rt>かん</rt></ruby>",
		`<kbd>\<</kbd>`,
		"a <b> c",
		// Autolinks.
		"<u>a <http://x/</u>> b",
		"<u>see <http://x/></u>",
		"x <https://example.com> y",
		// Underlined headings over several lines.
		"Setext <u>a\n</u>",
		"Setext <u>a</u>\nsecond line",
		"Setext `<u>a\n</u>`",
		"Setext <u>a <\n</u>",
		"Setext <kbd><</kbd>\n<u>b</u>",
		"Setext <u\n    >a</u>",
		// Code spans.
		"`<kbd>x</kbd>` text",
		"<kbd>`x`</kbd>",
		"`<ruby>x<rt>y</rt></ruby>` z",
		// Link labels.
		"<u>[[Other|x</u>]]",
		"[[Other|<u>x</u>]]",
		"<u>a [[Other|b]] c</u>",
		"[[Other|a < b]] <u>c</u>",
		"[[Other|<ruby>漢<rt>かん</rt></ruby>]]",
		// Character references.
		"&lt;u&gt;a</u>",
		"&#60;u>a</u> <u>b</u>",
		"<u>a &lt; b</u>",
		"<kbd>&lt;</kbd>",
		// Pairing on its own.
		"<U>a</U>",
		"<u >a</u >",
		"<u>a</u> <u>b",
		"<u/>a",
		"<mark>a<sub>2</sub></mark>",
		"a *<u>b*</u>",
		"==<u>a</u>==",
	} {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()
			body := "## " + heading
			if strings.Contains(heading, "\n") {
				body = heading + "\n==="
			}
			page := r.HTML("Note.md", "", body, wording.En).HTML
			m := stamped.FindStringSubmatch(page)
			if m == nil {
				t.Fatalf("the page stamped no heading id: %s", page)
			}
			if named := graph.SectionID(render.HeadingWords(heading)); named != m[1] {
				t.Errorf("SectionID(HeadingWords(%q)) = %q, but the page stamps id %q: %s", heading, named, m[1], page)
			}
		})
	}
}
