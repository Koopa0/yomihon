package render_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/vault"
)

// TestRealVaultRendersWithoutFaults is the mechanical definition of
// spec.md §1's acceptance criterion: "every real .md file opens: zero 500s,
// zero blank pages".
// It builds a real graph.Index and render.Renderer against
// ~/obsidian (or KURODO_ROOT) and renders every single .md file under
// it, asserting for each: no panic, no error, non-empty HTML. This is a
// permanent regression guard, following the same t.Skipf-when-vault-
// absent pattern as internal/schema's TestLoadRealContract — it runs (and
// is skipped loudly, not silently vacuous) whenever the real vault is
// present.
func TestRealVaultRendersWithoutFaults(t *testing.T) {
	t.Parallel()

	root := os.Getenv("KURODO_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		root = filepath.Join(home, "obsidian")
	}
	if _, err := os.Stat(root); err != nil { // #nosec G703 -- probing the operator's own vault to decide whether to skip
		t.Skipf("real vault not available: %v", err)
	}

	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	r := render.New(root, idx)

	paths, err := vault.List(root)
	if err != nil {
		t.Fatalf("vault.List(%q) = %v", root, err)
	}

	var mdCount int
	for _, p := range paths {
		if filepath.Ext(p) != ".md" {
			continue
		}
		mdCount++
		t.Run(p, func(t *testing.T) {
			t.Parallel()

			n, err := vault.ReadNote(root, p)
			if err != nil {
				t.Fatalf("ReadNote(%q) = %v", p, err)
			}

			result := r.HTML(n.Body)
			if result.HTML == "" {
				t.Errorf("HTML(%q) produced a blank page", p)
			}
		})
	}

	// A vault-model.md-documented count (419+ at spec-authoring time,
	// growing) — this is the "did the sweep actually run against the
	// real vault, not a near-empty stand-in" guard, not a hardcoded
	// vault census. See docs/vault-model.md's Layer 2 scale snapshot.
	const minExpectedNotes = 400
	if mdCount < minExpectedNotes {
		t.Errorf("swept %d .md files, want at least %d — is %s the real vault?", mdCount, minExpectedNotes, root)
	} else {
		t.Logf("swept %d .md files under %s with 0 faults", mdCount, root)
	}
}
