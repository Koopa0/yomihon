package render_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEmphasisClosesBesideCJKPunctuation pins the shapes a Chinese or Japanese
// lesson writes: a bold or italic run that ends on full-width punctuation and
// continues with no space. The page shows the emphasis and the searchable text
// carries the words without the asterisks. The English row and the arrow row
// keep CommonMark's refusal, since neither closer follows CJK punctuation.
func TestEmphasisClosesBesideCJKPunctuation(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	tests := []struct{ name, body, html, plain string }{
		{name: "colon before a latin word", body: "**題目：**agent", html: "<p><strong>題目：</strong>agent</p>\n", plain: "題目：agent"},
		{name: "full stop before han", body: "**注意。**後文", html: "<p><strong>注意。</strong>後文</p>\n", plain: "注意。後文"},
		{name: "italic parenthesis before han", body: "*（例）*字", html: "<p><em>（例）</em>字</p>\n", plain: "（例）字"},
		{name: "ascii full stop stays literal", body: "**bold.**text", html: "<p>**bold.**text</p>\n", plain: "**bold.**text"},
		{name: "arrow is not CJK punctuation", body: "**為什麼錯 →**prepare", html: "<p>**為什麼錯 →**prepare</p>\n", plain: "**為什麼錯 →**prepare"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Note.md", "", tt.body, wording.ZhHant)
			if diff := cmp.Diff(tt.html, got.HTML); diff != "" {
				t.Errorf("HTML(%q) (-want +got):\n%s", tt.body, diff)
			}
			if diff := cmp.Diff(tt.plain, render.PlainText(tt.body)); diff != "" {
				t.Errorf("PlainText(%q) (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}
