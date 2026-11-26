package render

import (
	"strings"
	"testing"
)

const (
	testFirst = `<mark class="a" data-mark-id="1">`
	testRest  = `<mark class="b" data-mark-id="1">`
)

// spanOf is the span of the first occurrence of words in the text of fragment,
// measured the way a page measures it.
func spanOf(t *testing.T, fragment, words string) Span {
	t.Helper()
	text := TextContent(fragment)
	at := strings.Index(text, words)
	if at < 0 {
		t.Fatalf("%q is not in the text %q", words, text)
	}
	return Span{Start: at, End: at + len(words), First: testFirst, Rest: testRest}
}

func TestOverlayWrapsWordsWhereTheyAreAndNothingElse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fragment string
		words    string
		want     string
	}{
		{
			name:     "inside one text",
			fragment: `<p>Alpha beta gamma.</p>`,
			words:    "beta",
			want:     `<p>Alpha ` + testFirst + `beta</mark> gamma.</p>`,
		},
		{
			name:     "across an inline element, each piece wrapped alone",
			fragment: `<p>Alpha <em>beta</em> gamma.</p>`,
			words:    "ha beta ga",
			want:     `<p>Alp` + testFirst + `ha </mark><em>` + testRest + `beta</mark></em>` + testRest + ` ga</mark>mma.</p>`,
		},
		{
			name:     "across two paragraphs",
			fragment: `<p>One two.</p><p>Three four.</p>`,
			words:    "two.  Three",
			want:     `<p>One ` + testFirst + `two.</mark></p><p>` + testRest + `Three</mark> four.</p>`,
		},
		{
			name:     "characters that markup spells",
			fragment: `<p>a &lt; b &amp; c</p>`,
			words:    "< b &",
			want:     `<p>a ` + testFirst + `&lt; b &amp;</mark> c</p>`,
		},
		{
			name:     "a Japanese passage with a ruby reading that is not words",
			fragment: `<p>国<ruby>境<rt>きょう</rt></ruby>の長い</p>`,
			words:    "境の長",
			want:     `<p>国<ruby>` + testFirst + `境</mark><rt>きょう</rt></ruby>` + testRest + `の長</mark>い</p>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			span := spanOf(t, tt.fragment, tt.words)
			if got := Overlay(tt.fragment, []Span{span}, nil, ""); got != tt.want {
				t.Errorf("Overlay() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestOverlayLeavesAFragmentWithNothingToDrawAsItCame(t *testing.T) {
	t.Parallel()
	const fragment = "<p>It &#39;stays&#39; <b>as</b> it came.</p>\n<!-- a comment -->\n"
	if got := Overlay(fragment, nil, nil, ""); got != fragment {
		t.Errorf("Overlay() = %q, want %q", got, fragment)
	}
}

func TestOverlayMarksNestWhereTwoStretchesOverlap(t *testing.T) {
	t.Parallel()
	const fragment = `<p>one two three four</p>`
	a := spanOf(t, fragment, "two three")
	b := spanOf(t, fragment, "three four")
	b.First, b.Rest = `<mark class="b1">`, `<mark class="b2">`
	want := `<p>one ` + testFirst + `two </mark>` + testRest + `<mark class="b1">three</mark></mark><mark class="b2"> four</mark></p>`
	if got := Overlay(fragment, []Span{a, b}, nil, ""); got != want {
		t.Errorf("Overlay() =\n%s\nwant\n%s", got, want)
	}
}

func TestOverlayPutsANoteAfterTheBlockThatHoldsTheEndOfTheWords(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fragment string
		words    string
		want     string
	}{
		{
			name:     "a paragraph: after it, never inside",
			fragment: `<p>Alpha beta.</p><p>Next.</p>`,
			words:    "beta",
			want:     `<p>Alpha ` + testFirst + `beta</mark>.</p>N<p>Next.</p>`,
		},
		{
			name:     "a heading",
			fragment: `<h2>Alpha beta</h2><p>Next.</p>`,
			words:    "beta",
			want:     `<h2>Alpha ` + testFirst + `beta</mark></h2>N<p>Next.</p>`,
		},
		{
			name:     "a list item: at its end, inside",
			fragment: `<ul><li>Alpha beta</li><li>Next</li></ul>`,
			words:    "beta",
			want:     `<ul><li>Alpha ` + testFirst + `beta</mark>N</li><li>Next</li></ul>`,
		},
		{
			name:     "a cell of a table: after the table",
			fragment: `<table><tr><td>Alpha beta</td><td>x</td></tr></table><p>Next.</p>`,
			words:    "beta",
			want:     `<table><tr><td>Alpha ` + testFirst + `beta</mark></td><td>x</td></tr></table>N<p>Next.</p>`,
		},
		{
			name:     "words nested in inline markup: the paragraph that holds them",
			fragment: `<p>Alpha <a href="x"><b>beta</b></a> tail</p>`,
			words:    "beta",
			want:     `<p>Alpha <a href="x"><b>` + testFirst + `beta</mark></b></a> tail</p>N`,
		},
		{
			name:     "a block that holds the words directly: at its end",
			fragment: `<div class="callout-body">Alpha beta</div>`,
			words:    "beta",
			want:     `<div class="callout-body">Alpha ` + testFirst + `beta</mark>N</div>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			span := spanOf(t, tt.fragment, tt.words)
			got := Overlay(tt.fragment, []Span{span}, []Note{{At: span.End, HTML: "N"}}, "")
			if got != tt.want {
				t.Errorf("Overlay() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestOverlayAddsTheTailAtTheEnd(t *testing.T) {
	t.Parallel()
	if got, want := Overlay("<p>x</p>", nil, nil, "<aside></aside>"), "<p>x</p><aside></aside>"; got != want {
		t.Errorf("Overlay() = %q, want %q", got, want)
	}
}

func FuzzOverlayKeepsTheWords(f *testing.F) {
	f.Add(`<p>Alpha <em>beta</em> gamma.</p><ul><li>one</li></ul>`, 3, 9)
	f.Add(`<table><tr><td>a</td></tr></table><pre>x &lt; y</pre>`, 0, 5)
	f.Add(`<p>国<ruby>境<rt>きょう</rt></ruby>の</p>`, 1, 4)
	f.Fuzz(func(t *testing.T, fragment string, start, end int) {
		text := TextContent(fragment)
		if start < 0 || end > len(text) || start >= end {
			return
		}
		got := Overlay(fragment, []Span{{Start: start, End: end, First: testFirst, Rest: testRest}}, []Note{{At: end, HTML: "<!--note-->"}}, "")
		if TextContent(got) != text {
			t.Errorf("the words changed: %q, want %q", TextContent(got), text)
		}
	})
}
