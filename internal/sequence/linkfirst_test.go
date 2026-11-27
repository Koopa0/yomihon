package sequence

import "testing"

// TestARefusedRowAnchorsNothing holds the single-owner rule. Whether a row is a
// lesson is decided once, when the row is read; asking again from the raw text
// lets a row the grammar refused for one reason pass a second, looser test and
// end up carrying a side branch.
func TestARefusedRowAnchorsNothing(t *testing.T) {
	t.Parallel()
	// The parent declares a role and also names a lesson, so it is refused as
	// course.role_on_entry — it is neither a lesson nor a plain branch heading.
	doc := Parse("## P {sequence=primary}\n\n- [[A]] {sequence=primary}\n\t- 支線 {sequence=local}\n\t\t- [[C]]\n", 1)

	if findRule(doc, RuleRoleOnEntry) == nil {
		t.Fatalf("the fixture no longer produces a refused parent row: %+v", doc.Diagnostics)
	}
	if findRule(doc, RuleLocalOrphan) == nil {
		t.Errorf("a side branch hung from a row the grammar refused: %+v", doc.Diagnostics)
	}
	for _, item := range walk(doc.Groups) {
		g := item.Branch
		if g == nil || g.Role != RoleLocal {
			continue
		}
		if g.Projectable() {
			t.Errorf("a side branch with nothing to hang from still projects: %+v", g)
		}
		if g.AnchorTarget != "" {
			t.Errorf("a side branch borrowed %q from a refused row", g.AnchorTarget)
		}
	}
}

// TestAContainerDeclarationIsReadOnTheRowsOwnLine holds that the two readers of
// a row agree about where its declaration lives. One decides whether the row
// opens a branch and the other decides what the branch is; reading different
// lines routes the nested list one way and names it the other.
func TestAContainerDeclarationIsReadOnTheRowsOwnLine(t *testing.T) {
	t.Parallel()
	// The container row sits inside a nested list nobody declared, which is the
	// path the helper takes rather than the main one.
	doc := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- 旁支\n\t  說明 {sequence=local}\n\t\t- [[B]]\n", 1)

	if findRule(doc, RuleRoleMisplaced) == nil {
		t.Errorf("a marker on a continuation line was read as a declaration: %+v", doc.Diagnostics)
	}
	for _, item := range walk(doc.Groups) {
		if g := item.Branch; g != nil && g.Role == RoleLocal {
			t.Errorf("a marker off the row's own line opened a side branch: %+v", g)
		}
	}
	// The routing must be the one the row's own line dictates. Two lists are
	// nested here and neither was declared, so both are reported and the deeper
	// one sits inside the shallower — a marker read on the wrong line must not
	// quietly lift a list up a level.
	if got := countRule(doc, RuleRoleMissing); got != 2 {
		t.Errorf("undeclared nesting reported %d times, want 2 — one per level nobody declared: %+v",
			got, doc.Diagnostics)
	}
	depth := -1
	for d, item := range walk(doc.Groups) {
		if item.Entry != nil && item.Entry.Target == "B" {
			depth = d
		}
	}
	if depth != 3 {
		t.Errorf("the row naming B sits inside %d branches, want 3: its part and the two lists nobody declared; a misplaced marker moved it", depth)
	}
}
