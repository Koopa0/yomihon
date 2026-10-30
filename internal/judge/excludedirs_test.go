package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestExcludedDirectoriesTakeNoPartInNameCollisions holds the case that
// motivated the exclusion list: a build tree whose files repeat across
// courses (every lab carries a main.go, every template shares a lesson's name)
// made one collision finding per repeated name and left the real lesson's name
// ambiguous. Excluded, the directories are not in the scan at all, so neither
// the collision nor the unresolved link exists. Not excluded, both do, which
// is what shows the fixture is able to produce them.
func TestExcludedDirectoriesTakeNoPartInNameCollisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		declaration   string
		wantCollision bool
	}{
		{name: "not excluded", declaration: "", wantCollision: true},
		{name: "excluded", declaration: `exclude_dirs = ["courses/*/lab"]`, wantCollision: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath,
				strings.Replace(frontmatterContract, "SCAN_DECLARATION", tt.declaration, 1))
			write(t, root, "courses/C1/lab/main.go", "package main\n")
			write(t, root, "courses/C2/lab/main.go", "package main\n")
			write(t, root, "courses/C4/lab/L1/tpl/L10 Two pointers.md", "---\ntitle: Template\ntype: note\n---\n")
			write(t, root, "courses/C4/modules/L10 Two pointers.md", "---\ntitle: Module\ntype: note\n---\n")
			write(t, root, "Notes/Index.md", "---\ntitle: Index\ntype: note\n---\n\nSee [[L10 Two pointers]].\n")

			// The default report drops findings that touch nothing in the
			// knowledge layer, and the colliding files are outside it, so the
			// whole-vault view is the one that can show them.
			findings, err := runCheckAction(t.Context(), root, nil, true)
			if err != nil {
				t.Fatalf("runCheckAction() error = %v", err)
			}
			var collisions []string
			for i := range findings {
				if findings[i].RuleID == "collision.name" {
					collisions = append(collisions, findings[i].Message)
				}
			}
			if got := len(collisions) > 0; got != tt.wantCollision {
				t.Errorf("collision.name reported = %v, want %v; findings: %v", got, tt.wantCollision, collisions)
			}
			if !tt.wantCollision {
				for i := range findings {
					if findings[i].RuleID == "link.broken" ||
						strings.Contains(findings[i].Message, "main.go") || strings.Contains(findings[i].Path, "/lab/") {
						t.Errorf("a finding names a file under an excluded directory, or the link to the module stopped resolving: %+v", findings[i])
					}
				}
			}
		})
	}
}
