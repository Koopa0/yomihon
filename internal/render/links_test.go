package render_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

func TestSourceLinksReportWhereTheParserFoundEachOne(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		body string
		want []string // each as body[start:end]=destination
	}{
		{"an inline link", "see [the plan](Private/x.md \"t\") now", []string{"[the plan](Private/x.md \"t\")=Private/x.md"}},
		{"an image", "an ![alt](Private/p.png) here", []string{"![alt](Private/p.png)=Private/p.png"}},
		{"a link whose text holds a bracket", "[a `]` b](Private/x.md)", []string{"[a `]` b](Private/x.md)=Private/x.md"}},
		{"a link whose text runs over a line", "[multi\nline text](Private/x.md)", []string{"[multi\nline text](Private/x.md)=Private/x.md"}},
		{"a reference and its definition", "[q][r]\n\n[r]: Private/x.md\n", []string{"[q][r]=Private/x.md", "[r]: Private/x.md=Private/x.md"}},
		{"a shortcut reference", "[r] here\n\n[r]: Private/x.md", []string{"[r]=Private/x.md", "[r]: Private/x.md=Private/x.md"}},
		{"a definition in a block quote", "> [q][r]\n>\n> [r]: Private/x.md\n", []string{"[q][r]=Private/x.md", "[r]: Private/x.md=Private/x.md"}},
		{"a definition in a list item", "- [q][r]\n\n  [r]: Private/x.md\n", []string{"[q][r]=Private/x.md", "[r]: Private/x.md=Private/x.md"}},
		{"a definition with its address on the next line", "[r3]:\n  Private/x.md\n\n[r3]", []string{"[r3]:\n  Private/x.md=Private/x.md", "[r3]=Private/x.md"}},
		{"an autolink", "<https://example.com/a> end", []string{"<https://example.com/a>=https://example.com/a"}},
		{"raw html", "an <b>x</b> and <a href=\"Private/x.md\">y</a>", []string{"<b>=", "</b>=", "<a href=\"Private/x.md\">=", "</a>="}},
		{"an html block", "<div>\n<img srcset=\"Private/p.png 2x\">\n</div>\n\nafter", []string{"<div>\n<img srcset=\"Private/p.png 2x\">\n</div>="}},
		{"a link in a code span is not one", "`[t](Private/x.md)` and ``[[a]]``", nil},
		{"a link in a fence is not one", "```\n[t](Private/x.md)\n```\n", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			for _, link := range render.SourceLinks(tt.body) {
				got = append(got, tt.body[link.Start:link.End]+"="+link.Destination)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SourceLinks(%q) = %q, want %q", tt.body, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SourceLinks(%q)[%d] = %q, want %q", tt.body, i, got[i], tt.want[i])
				}
			}
		})
	}
}
