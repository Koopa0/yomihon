package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestParsedHeadingFactsKeepPresentationIdentity(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want map[string]bool
	}{
		{
			name: "rich words and branch declaration",
			body: "## Name `Code` &amp; [[Other|Shown]] ^[hidden] {sequence=primary}\n",
			want: map[string]bool{"name-code-shown": true},
		},
		{
			name: "comment free words keep code",
			body: "## Before %%hidden%% `Code`\n",
			want: map[string]bool{"before-code": true},
		},
		{
			name: "ruby keeps only base characters",
			body: "## <ruby>Base<rt>reading</rt></ruby> `Code`\n",
			want: map[string]bool{"base-code": true},
		},
		{
			name: "quote and list headings",
			body: "> ## Quoted `Code`\n\n- ## Listed `Code`\n",
			want: map[string]bool{"quoted-code": true, "listed-code": true},
		},
		{
			name: "setext and empty headings",
			body: "Underlined `Code`\n---\n\n##\n",
			want: map[string]bool{"underlined-code": true, "section": true},
		},
		{
			name: "unspoken heading shapes",
			body: "%%\n## Hidden\n%%\n\n```text\n## Fenced\n```\n\n<div>\n## HTML\n</div>\n",
			want: map[string]bool{},
		},
		{
			name: "used footnote heading only",
			body: "## Main\n\nref[^used]\n\n[^unused]:\n    ## Hidden\n\n[^used]:\n    ## Shown\n",
			want: map[string]bool{"main": true, "shown": true},
		},
		{name: "empty", want: map[string]bool{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := make(map[string]bool)
			collectParsedHeadings(graph.ReadBody(tt.body), got)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: rich heading identity differs (-want +got):\n%s", diff)
			}
		})
	}
}
