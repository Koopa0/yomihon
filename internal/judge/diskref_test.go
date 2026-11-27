package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestCheckDoesNotInspectOrReportEgressDeniedPathTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		createFile bool
	}{
		{name: "missing"},
		{name: "present", createFile: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeTestContract(t, root, []string{"Restricted"})
			write(t, root, "Concepts/golang/Public.md", `---
---
# Public

[private target](../../Restricted/secret.md)
`)
			if tt.createFile {
				write(t, root, "Restricted/secret.md", "secret")
			}

			findings, err := runCheckAction(t.Context(), root, nil, false)
			if err != nil {
				t.Fatalf("Check(%q): %v", root, err)
			}
			for i := range findings {
				if findings[i].RuleID == "link.broken.path" {
					t.Fatalf("link.broken.path finding leaked an egress-denied target: %+v", findings[i])
				}
			}
		})
	}
}

// TestAPathRefOutsideTheScanIsNotAFinding holds the difference between looking
// and finding nothing, and never looking. The scan skips hidden paths, so a
// reference to a file there is a reference past this reader's boundary, not a
// fault in the vault, while a reference that is really broken is still
// reported, written as a link or in code.
func TestAPathRefOutsideTheScanIsNotAFinding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil, [2]string{
		`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
		`checked = ["Writing"]`,
	}))
	for _, hidden := range []string{".claude/skills/share-rewrite/SKILL.md", "Writing/.config/notes.md", ".hidden.md"} {
		write(t, root, hidden, "held\n")
	}
	write(t, root, "Writing/Guide.md", "[a](../.claude/skills/share-rewrite/SKILL.md)\n"+
		"[b](.config/notes.md)\n"+
		"[c](../.hidden.md)\n"+
		"[d](../Notes/gone.md)\n"+
		"[e](gone.md)\n"+
		"`.claude/skills/share-rewrite/SKILL.md`\n"+
		"`.hidden.md`\n"+
		"`Notes/gone.md`\n")
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	var got []string
	for _, f := range findings {
		if f.RuleID == "link.broken.path" {
			got = append(got, *f.Target)
		}
	}
	if diff := cmp.Diff([]string{"../Notes/gone.md", "gone.md", "Notes/gone.md"}, got); diff != "" {
		t.Errorf("link.broken.path targets (-want +got):\n%s", diff)
	}
}
