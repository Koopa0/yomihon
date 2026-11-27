package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestRunCheckDomainHeadings holds the report's domain headings to the folders
// the rules file declares: a note under a declared root, renamed from the
// default, is grouped by its domain, and one under no declared root is filed
// under (none). Which paths fall under a root is schema's to answer.
func TestRunCheckDomainHeadings(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		roots string
		scope string
		path  string
		want  string
	}{
		{"renamed nested", `["Archive/studies"]`, "Archive", "Archive/studies/rust/L1.md", "rust"},
		{"undeclared nested", `["Sources"]`, "Writing", "Writing/lessons/golang/L1.md", "(none)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil,
				[2]string{`domain_from = ["Concepts"]`, `domain_from = ` + tt.roots},
				[2]string{`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `checked = ["` + tt.scope + `"]`},
			))
			// Legal no-frontmatter notes still yield an actionable broken link.
			write(t, root, tt.path, "see [[Nowhere]]\n")
			stdout, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			var headings []string
			for line := range strings.SplitSeq(string(stdout), "\n") {
				if heading, ok := strings.CutPrefix(line, "\u258c "); ok {
					headings = append(headings, heading)
				}
			}
			if diff := cmp.Diff([]string{tt.want}, headings); diff != "" {
				t.Errorf("RunCheck() domain headings mismatch (-want +got):\n%s\nreport:\n%s", diff, stdout)
			}
		})
	}
}
