package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestTheReportGroupsByTheFoldersTheContractDeclares holds the report's domain
// headings to the vault's own declaration. The folders were written into this
// package as constants — "Concepts/" and "Writing/lessons/" — which is a second
// spelling of a contract key for the first and a layout no contract names at
// all for the second. A vault that files its knowledge anywhere else had every
// finding gathered under the no-domain heading while its contract said plainly
// where its domains live.
//
// The fixture renames the root, which is what makes this discriminating: a
// vault declaring "Concepts" cannot tell a report that reads the contract from
// one that hardcodes the same word.
func TestTheReportGroupsByTheFoldersTheContractDeclares(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{`domain_equals_folder_under = ["Concepts"]`, `domain_equals_folder_under = ["Ideas"]`},
		[2]string{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`knowledge_dirs = ["Ideas"]`,
		}))
	write(t, root, "Ideas/golang/Cited.md",
		"---\ntitle: Cited\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Cited]]\"\n---\n\nsee [[Nowhere]]\n")

	stdout, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	report := string(stdout)
	if !strings.Contains(report, "golang") {
		t.Fatalf("the report names no domain at all, so it proves nothing about grouping:\n%s", report)
	}
	if strings.Contains(report, "(other)") {
		t.Errorf("a note under the folder the contract declares is filed with no domain:\n%s", report)
	}
}

// TestRunCheckDomainHeadings catches hardcoded roots, guessed undeclared
// domains, character-prefix matches, and direct-child filenames as domains.
func TestRunCheckDomainHeadings(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		roots string
		scope string
		path  string
		want  string
	}{
		{"declared nested", `["Writing/lessons"]`, "Writing", "Writing/lessons/golang/L1.md", "golang"},
		{"renamed nested", `["Archive/studies"]`, "Archive", "Archive/studies/rust/L1.md", "rust"},
		{"deep directory", `["Writing/lessons"]`, "Writing", "Writing/lessons/japanese/drills/D1.md", "japanese"},
		{"undeclared nested", `["Sources"]`, "Writing", "Writing/lessons/golang/L1.md", "(other)"},
		{"empty roots", `[]`, "Writing", "Writing/lessons/golang/L1.md", "(other)"},
		{"nested direct child", `["Writing/lessons"]`, "Writing", "Writing/lessons/Overview.md", "(other)"},
		{"top level direct child", `["Writing"]`, "Writing", "Writing/Overview.md", "(other)"},
		{"sibling prefix", `["Writing/lessons"]`, "Writing", "Writing/lessons-extra/golang/L1.md", "(other)"},
		{"case distinct", `["Writing/Lessons"]`, "Writing", "Writing/lessons/golang/L1.md", "(other)"},
		{"normalization distinct", `["Writing/Cafe\u0301"]`, "Writing", "Writing/Caf\u00e9/golang/L1.md", "(other)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil,
				[2]string{`domain_equals_folder_under = ["Concepts"]`, `domain_equals_folder_under = ` + tt.roots},
				[2]string{`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `knowledge_dirs = ["` + tt.scope + `"]`},
			))
			// Legal no-frontmatter notes still yield an actionable broken link.
			write(t, root, tt.path, "see [[Nowhere]]\n")
			stdout, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
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
