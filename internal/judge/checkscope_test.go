package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestCheckScopeFilters holds what a scope filter may say. A vault-relative
// folder, file or "." narrows the report; a filter that names nothing, an
// absolute path, a path that climbs out of the vault, and a scope the rules
// file withholds are refused, because each would otherwise answer with an
// empty report that reads like a clean one, and under --deny error a pass.
func TestCheckScopeFilters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, []string{"Private"}, [2]string{
		`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
		`checked = ["Writing", "Private"]`,
	}))
	const broken = "---\ntype: not-a-declared-type\ncreated: 2026-01-01\n---\nbody\n"
	write(t, root, "Writing/Public.md", broken)
	write(t, root, "Private/Secret.md", broken)

	for _, tt := range []struct {
		name    string
		scope   string
		refused bool
		// says is a word the refusal carries.
		says string
	}{
		{name: "a folder", scope: "Writing"},
		{name: "a file", scope: "Writing/Public.md"},
		{name: "the vault root", scope: "."},
		{name: "a path that names nothing", scope: "Nowhere", refused: true, says: "names nothing in this vault"},
		{name: "an absolute path", scope: root, refused: true, says: "absolute path"},
		{name: "a path that climbs out of the vault", scope: "../Writing", refused: true},
		{name: "a withheld file", scope: "Private/Secret.md", refused: true, says: "withholds"},
		{name: "a withheld folder", scope: "Private", refused: true, says: "withholds"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, exit, err := checkBytes(t.Context(), &CheckOptions{
				Root: root, Paths: []string{tt.scope}, Deny: []string{"error"}, Format: FormatJSON,
			})
			if !tt.refused {
				if err != nil || exit != 1 {
					t.Errorf("RunCheck(scope %q) = exit %d, error %v; want the public error gated", tt.scope, exit, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.says) {
				t.Errorf("RunCheck(scope %q) = exit %d, error %v; want a refusal saying %q", tt.scope, exit, err, tt.says)
			}
		})
	}
}
