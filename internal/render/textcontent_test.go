package render_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

func TestTextContentIsTheWordsASelectionHolds(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, html, want string
	}{
		{"inline markup is not a break", `<p>a <em>b</em>c <code>d()</code></p>`, " a bc d() "},
		{"two paragraphs are two words", `<p>one</p><p>two</p>`, " one  two "},
		{"a line break is a space", `<p>one<br>two</p>`, " one two "},
		{"an entity is the character", `<p>a &amp; b &lt;c&gt;</p>`, " a & b <c> "},
		{"a script is not words", `<p>a</p><script>var x = "<p>b</p>"</script><p>c</p>`, " a  c "},
		{"a style is not words", `<style>p { color: red }</style><p>a</p>`, " a "},
		{"a template is not words", `<template><p>a</p></template><p>b</p>`, " b "},
		{"a ruby reading is selected and its bracket is not", `<ruby>漢字<rp>(</rp><rt>かんじ</rt><rp>)</rp></ruby>`, "漢字かんじ"},
		{"a table cell is a word of its own", `<table><tr><td>a</td><td>b</td></tr></table>`, "   a  b   "},
		{"a comment is not words", `<p>a<!-- b -->c</p>`, " ac "},
		{"nothing is nothing", ``, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := render.TextContent(tt.html); got != tt.want {
				t.Errorf("TextContent(%q) = %q, want %q", tt.html, got, tt.want)
			}
		})
	}
}

// TestTextContentKeepsNothingAScriptCouldSay holds the one thing a quote
// placed in this text must not be able to do: the text of a nested element of
// the name being skipped does not end the skip early.
func TestTextContentKeepsNothingAScriptCouldSay(t *testing.T) {
	t.Parallel()

	got := render.TextContent(`<template>x<template>y</template>z</template><p>w</p>`)
	if want := " w "; got != want {
		t.Errorf("a nested template left %q, want %q", got, want)
	}
}
