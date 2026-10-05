package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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
		{name: "closing attribute", body: "Before <kbd>key</kbd class=sample> after", want: "<p>Before <kbd>key&lt;/kbd class=sample&gt; after</p>\n"},
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
