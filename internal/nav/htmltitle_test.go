package nav

import (
	"strings"
	"testing"
)

// TestHTMLTitleIsTheTitleElement pins the first answer: a document that has a
// <title> is called by it, ahead of any heading in its body, and what the
// element holds is read the way a browser's tab would read it — whitespace
// collapsed to single spaces, nothing else changed.
func TestHTMLTitleIsTheTitleElement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "the title in the head",
			html: "<!doctype html><html><head><title>接收者離開後 — Go 並行回顧</title></head><body><h1>Other</h1></body></html>",
			want: "接收者離開後 — Go 並行回顧",
		},
		{
			name: "a title wins over a heading that comes first",
			html: "<body><h1>Heading</h1></body><title>Title</title>",
			want: "Title",
		},
		{
			name: "whitespace collapses and the ends are trimmed",
			html: "<title>\n  Daily \t briefing\r\n  2026-09-22 \n</title>",
			want: "Daily briefing 2026-09-22",
		},
		{
			name: "a non-breaking space is a letter of the title, not whitespace",
			html: "<title>A B</title>",
			want: "A B",
		},
		{
			name: "an empty title defers to the heading",
			html: "<title>  \n </title><h1>Heading</h1>",
			want: "Heading",
		},
		{
			name: "a title inside an svg is a tooltip, not the document's",
			html: "<body><svg viewBox='0 0 1 1'><title>Chart icon</title></svg><h1>Real</h1></body>",
			want: "Real",
		},
		{
			name: "the title after a closed svg still counts",
			html: "<body><svg><title>Chart icon</title></svg></body><title>Real</title>",
			want: "Real",
		},
		{
			name: "only the first title is the document's, even when it is empty",
			html: "<title></title><title>Second</title>",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HTMLTitle([]byte(tt.html)); got != tt.want {
				t.Errorf("HTMLTitle(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}

// TestHTMLTitleFallsBackToTheFirstHeading pins the second answer: with no
// title, the document is called by the words of its first <h1>, tags dropped
// and whitespace collapsed, and a heading that never closes names nothing
// because its words would run to the end of whatever was read.
func TestHTMLTitleFallsBackToTheFirstHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "the heading's own text",
			html: "<body><h1>Go 並行回顧</h1><h2>Sub</h2></body>",
			want: "Go 並行回顧",
		},
		{
			name: "tags inside it are dropped and whitespace collapses",
			html: "<h1> Go <em>並行</em>\n   回顧 </h1>",
			want: "Go 並行 回顧",
		},
		{
			name: "the first heading wins",
			html: "<h1>One</h1><h1>Two</h1>",
			want: "One",
		},
		{
			name: "a line break reads as a space",
			html: "<h1>接收者離開後<br>Go 並行回顧</h1>",
			want: "接收者離開後 Go 並行回顧",
		},
		{
			name: "a script or style inside it adds no words",
			html: "<h1>A<script>var x = 1</script>B<style>b{}</style>C</h1>",
			want: "ABC",
		},
		{
			name: "a heading that never closes names nothing",
			html: "<body><h1>never closed <p>and the rest of the page",
			want: "",
		},
		{
			name: "an empty first heading defers to a later title",
			html: "<h1> </h1><title>T</title>",
			want: "T",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HTMLTitle([]byte(tt.html)); got != tt.want {
				t.Errorf("HTMLTitle(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}

// TestHTMLTitleIsEmptyWhenTheDocumentNamesNothing pins the third answer: no
// title and no heading is no name, and the caller falls back on the file's.
func TestHTMLTitleIsEmptyWhenTheDocumentNamesNothing(t *testing.T) {
	t.Parallel()

	for name, html := range map[string]string{
		"nothing at all":       "",
		"a page of paragraphs": "<html><body><p>just prose</p><h2>and a lesser heading</h2></body></html>",
		"not HTML":             "\x00\x01\x02 plain bytes < > &",
		"a UTF-16 file":        "<\x00t\x00i\x00t\x00l\x00e\x00>\x00A\x00<\x00/\x00t\x00i\x00t\x00l\x00e\x00>\x00",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := HTMLTitle([]byte(html)); got != "" {
				t.Errorf("HTMLTitle(%q) = %q, want no title", html, got)
			}
		})
	}
}

// TestHTMLTitleReadsOnlyTheFirstWindow pins the bound: a document is asked
// for its name from its first HTMLTitleBytes and no further, so a file with
// its title past that is called by what the first window gave, and a title
// still open when the window ends is not trusted to be the whole of one.
//
// The control rows matter as much as the refusals. A bound that had moved
// would still refuse something, but only a title just inside the window being
// found says the window is where it is claimed to be.
func TestHTMLTitleReadsOnlyTheFirstWindow(t *testing.T) {
	t.Parallel()

	// filler is a comment, which the tokenizer reads as one token and a reader
	// does not see, so the bytes before the title are exactly as many as asked.
	filler := func(n int) string {
		const open, shut = "<!--", "-->"
		return open + strings.Repeat("x", n-len(open)-len(shut)) + shut
	}
	title := "<title>The Title</title>"

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "a title ending exactly at the bound is found",
			html: filler(HTMLTitleBytes-len(title)) + title + "<h1>Heading</h1>",
			want: "The Title",
		},
		{
			name: "a title starting past the bound is ignored",
			html: filler(HTMLTitleBytes) + title,
			want: "",
		},
		{
			name: "a title past the bound leaves the heading before it",
			html: "<h1>Heading</h1>" + filler(HTMLTitleBytes) + title,
			want: "Heading",
		},
		{
			name: "a title open at the bound is not a title",
			html: filler(HTMLTitleBytes-len("<title>The Ti")) + title,
			want: "",
		},
		{
			name: "a heading past the bound is ignored",
			html: filler(HTMLTitleBytes) + "<h1>Late</h1>",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HTMLTitle([]byte(tt.html)); got != tt.want {
				t.Errorf("HTMLTitle(%d bytes) = %q, want %q", len(tt.html), got, tt.want)
			}
		})
	}
}

// TestHTMLTitleDecodesEntitiesOnce pins what hostile markup in a title does
// to the words handed back. The tokenizer reads a <title> as text, so a tag
// inside it is characters and arrives as characters; a character reference is
// resolved exactly once, so an escaped one stays escaped. The page that prints
// the result escapes it again on the way out, which is why decoding twice
// here would turn an author's literal "&lt;" into markup downstream.
func TestHTMLTitleDecodesEntitiesOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "a script element in a title is text",
			html: "<title><script>alert(1)</script></title>",
			want: "<script>alert(1)</script>",
		},
		{
			name: "escaped markup decodes to its characters",
			html: "<title>&lt;script&gt;alert(1)&lt;/script&gt;</title>",
			want: "<script>alert(1)</script>",
		},
		{
			name: "a reference to an ampersand is decoded once",
			html: "<title>a &amp;lt;b&amp;gt; &amp;amp; c</title>",
			want: "a &lt;b&gt; &amp; c",
		},
		{
			name: "the same single decoding holds for a heading",
			html: "<h1>a &amp;lt;b&amp;gt;</h1>",
			want: "a &lt;b&gt;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HTMLTitle([]byte(tt.html)); got != tt.want {
				t.Errorf("HTMLTitle(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}
