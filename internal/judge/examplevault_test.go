package judge_test

import (
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
)

// TestTheExampleVaultScans reads the vault this repository ships as its worked
// example. The README sends a new reader to that contract as the thing to copy,
// and until now nothing checked that it still parses: an example is the one file
// that rots without anybody noticing, because nobody who already has a working
// vault ever opens it.
//
// What it holds is the whole scan rather than the contract alone. A contract can
// load and still describe a vault whose notes it rejects, and that vault would
// teach a reader a shape yomihon refuses.
func TestTheExampleVaultScans(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "examples", "vault")

	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(%q) error = %v; the example vault no longer scans", root, err)
	}

	// The vault sets out to show three faults and no others. An example that
	// demonstrates a diagnostic has to actually produce it — a demonstration
	// that quietly stopped demonstrating is the failure this catches from one
	// side — and an example that produces a fourth is teaching something nobody
	// chose to teach, which is the failure from the other.
	demonstrates := map[string]int{
		"schema.enum": 1, // a note whose status is outside the declared list
		"link.broken": 1, // a wikilink whose target nobody has written
		// The dialect note writes a live link into a section its target does
		// not have, and its own prose says the link degrades rather than
		// breaks; the judge now reports the same miss the page shows.
		"link.section_missing": 1,
	}
	found := map[string]int{}
	for _, f := range findings {
		found[string(f.RuleID)]++
	}
	for rule, want := range demonstrates {
		if found[rule] != want {
			t.Errorf("the example vault reports %d %s findings, want the %d it sets out to demonstrate", found[rule], rule, want)
		}
		delete(found, rule)
	}
	for rule, n := range found {
		t.Errorf("the example vault reports %d %s findings, which it does not set out to demonstrate", n, rule)
	}
}
