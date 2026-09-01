package snapshot

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
)

// TestViewCarriesTheSchemaVerdictForEachNote holds the generation to the same
// verdict the check command reaches. The oracle is the judge seam over the
// same bytes rather than a list written here, so the two cannot drift apart
// while both stay green.
func TestViewCarriesTheSchemaVerdictForEachNote(t *testing.T) {
	t.Parallel()

	const faulty = "Concepts/golang/Bad.md"
	const clean = "Concepts/golang/Fine.md"
	faultyBody := "---\ntitle: Bad\ntype: concept\ndomain: golang\nstatus: bogus\ncreated: 2026-06-01\nupdated: 2026-06-01\nextra: 1\n---\n\nbody\n"
	cleanBody := "---\ntitle: Fine\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Something]]\"\n---\n\nbody\n"

	root := t.TempDir()
	contract := testContract(t, root)
	writeNote(t, root, faulty, faultyBody)
	writeNote(t, root, clean, cleanBody)
	store, _ := newTestStore(t, root, contract)
	view := store.Current()

	want, err := judge.LintFrontmatter(faulty, []byte(faultyBody), contract)
	if err != nil {
		t.Fatalf("judge.LintFrontmatter() error = %v", err)
	}
	if len(want) == 0 {
		t.Fatal("the fixture note draws no schema findings, so this test would pass on any implementation")
	}
	var ruleIDs []string
	for _, f := range want {
		ruleIDs = append(ruleIDs, f.RuleID)
	}
	t.Logf("the fixture draws: %s", strings.Join(ruleIDs, ", "))

	if diff := cmp.Diff(want, view.SchemaFindings(faulty)); diff != "" {
		t.Errorf("View.SchemaFindings(%q) differs from the seam (-seam +view):\n%s", faulty, diff)
	}
	if got := view.SchemaFindings(clean); len(got) != 0 {
		t.Errorf("View.SchemaFindings(%q) = %v, want nothing for a note that satisfies the schema", clean, got)
	}
	if got := view.SchemaFindings("Concepts/golang/Absent.md"); len(got) != 0 {
		t.Errorf("View.SchemaFindings(absent) = %v, want nothing", got)
	}
}

// TestSchemaFindingsAreTheCallersOwnSlice holds the accessor to the promise
// every other collection accessor here makes. Handing out the generation's own
// backing array would let one reader's append reach another reader's page.
func TestSchemaFindingsAreTheCallersOwnSlice(t *testing.T) {
	t.Parallel()

	const faulty = "Concepts/golang/Bad.md"
	body := "---\ntitle: Bad\ntype: concept\ndomain: golang\nstatus: bogus\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"

	root := t.TempDir()
	contract := testContract(t, root)
	writeNote(t, root, faulty, body)
	store, _ := newTestStore(t, root, contract)
	view := store.Current()

	first := view.SchemaFindings(faulty)
	if len(first) == 0 {
		t.Fatal("the fixture note draws no schema findings, so this test would prove nothing")
	}
	first[0] = judge.Finding{RuleID: "scribbled.over"}
	second := view.SchemaFindings(faulty)
	if second[0].RuleID == "scribbled.over" {
		t.Error("writing to one caller's slice changed the next caller's; the accessor handed out the generation's own array")
	}
}
