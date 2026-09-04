package judge

import (
	"strings"
	"testing"

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

// TestTheReportStillGroupsLessonsUnderTheNestedRoot holds the half of the
// grouping no contract can express. The declaration takes a first path segment,
// so a vault filing its lessons under Writing/lessons/<domain>/ has no way to
// say where their domains are; reading only the declaration filed every one of
// them under the no-domain heading, which on the vault this product was built
// for merged two domains' worth of findings into one.
//
// The root is therefore still written into the report, and this is what says so
// out loud, so that removing it is a decision somebody makes rather than a
// tidying that looks safe.
func TestTheReportStillGroupsLessonsUnderTheNestedRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`knowledge_dirs = ["Writing"]`,
		}))
	write(t, root, "Writing/lessons/golang/L1.md",
		"---\ntitle: L1\ntype: lesson\nstatus: draft\ndomain: golang\nslug: l1\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nsee [[Nowhere]]\n")

	stdout, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	report := string(stdout)
	if !strings.Contains(report, "golang") {
		t.Errorf("a lesson under the nested root lost its domain heading:\n%s", report)
	}
	if strings.Contains(report, "(other)") {
		t.Errorf("a lesson under the nested root is filed with no domain:\n%s", report)
	}
}
