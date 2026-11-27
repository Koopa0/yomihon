package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHeadingPathFindingsMatchDisplayedAncestors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, target, citer string
		want                []string
	}{
		{name: "child", target: "## A\n### B\n", citer: "[[N#A#B]]"},
		{name: "missing child", target: "## A\n### B\n## Other\n### C\n", citer: "[[N#A#C]]", want: []string{"link.section_missing N#A#C"}},
		{name: "outside embed cut", target: "## A\n![[Child#C]]\n", citer: "[[N#A#B]]", want: []string{"link.section_missing N#A#B"}},
		{name: "path embed", target: "## A\n### B\n", citer: "![[N#A#B]]"},
		{name: "missing path embed", target: "## A\n### B\n", citer: "![[N#A#C]]", want: []string{"embed.section_missing N#A#C"}},
		{name: "block wins", target: "## A\n### B\n\nwords ^real\n", citer: "[[N^real#Other#Absent]]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findingKeys(fragmentRun(t, map[string]string{"N.md": tt.target, "Citer.md": tt.citer, "Child.md": "### B\nCHILD\n### C\nOTHER\n"}))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fragmentFindings() (-want +got):\n%s", diff)
			}
		})
	}
}
