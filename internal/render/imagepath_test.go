package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

type missingImageFiles struct{}

func (missingImageFiles) MissingFile(string) bool { return true }

func TestMissingImageOwnerCharacterization(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{name: "relative root and suffix", body: "![](./a.png?size=2#view)\n![](/root.png)\n![](../top.png)\n", want: []string{"Notes/a.png", "root.png", "top.png"}},
		{name: "one decode and dot cleanup", body: "![](a%20b.png)\n![](dir/../a.png)\n![](a%2520b.png)\n", want: []string{"Notes/a b.png", "Notes/a.png", "Notes/a%20b.png"}},
		{name: "empty nested and reference alt", body: "![](a.png) ![**nested**](b.png)\n![r][ref]\n![collapsed][]\n![shortcut]\n\n[ref]: c.png\n[collapsed]: d.png\n[shortcut]: e.png\n", want: []string{"Notes/a.png", "Notes/b.png", "Notes/c.png", "Notes/d.png", "Notes/e.png"}},
		{name: "code comments escapes", body: "`![](inline.png)`\n\n```md\n![](fenced.png)\n```\n\n    ![](indented.png)\n\n%% ![](percent.png) %%\n<!-- ![](html.png) -->\n\\![](escaped.png)\n![](live.png)\n", want: []string{"Notes/live.png"}},
		{name: "malformed percent is refused before image output", body: "![](bad%ZZ.png)\n"},
		{name: "encoded percent names a literal filename", body: "![](bad%25ZZ.png)\n", want: []string{"Notes/bad%ZZ.png"}},
		{name: "excluded sources", body: "![](https://example.invalid/a.png)\n![](//example.invalid/a.png)\n![](data:image/png;base64,AAAA)\n![](/raw/a.png)\n![](/notes/a.png)\n![](/static/a.png)\n![](../../out.png)\n![]()\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := render.New(graph.BuildFromNotes(nil, nil), transclusions{}, noTitles{}, missingImageFiles{})
			page := r.HTML("Notes/Images.md", "", tt.body, wording.En)
			var targets []string
			for _, d := range page.Diagnostics {
				if d.Kind == render.DiagImageMissing {
					targets = append(targets, d.Target)
				}
			}
			if diff := cmp.Diff(tt.want, targets); diff != "" {
				t.Errorf("caught: missing-image owner characterization (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMissingFileWordKinds(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, tt := range []struct {
		target string
		word   string
	}{
		{target: "missing.png", word: "file"},
		{target: "missing.pdf", word: "file"},
		{target: "Go sync.Pool", word: "note"},
		{target: "missing.bin", word: "note"},
	} {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()
			page := r.HTML("Notes/Images.md", "", "![["+tt.target+"]]", wording.En)
			if !strings.Contains(page.HTML, "no "+tt.word) {
				t.Errorf("caught: missing-image file word classification: target=%q want %q in %s", tt.target, tt.word, page.HTML)
			}
		})
	}
}
