package sequence

import "testing"

// TestAnUnpairedDelimiterIsVisible holds the question link-first actually asks.
// A run of asterisks is emphasis only when Markdown pairs it with a closer;
// unpaired, it renders as itself and stands in front of the link. Deciding from
// the run's own shape — "something follows it, so it must open emphasis" — reads
// a printed "**" as if it were invisible.
func TestAnUnpairedDelimiterIsVisible(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  string
		want EntryState
	}{
		{name: "paired bold", row: "- **[[A]]**\n", want: EntryAccepted},
		{name: "paired italic", row: "- _[[A]]_\n", want: EntryAccepted},
		{name: "bold and italic together", row: "- ***[[A]]***\n", want: EntryAccepted},
		{name: "bold wrapping italic", row: "- **_[[A]]_**\n", want: EntryAccepted},
		{name: "a comment is still invisible", row: "- %%hidden%% [[A]]\n", want: EntryAccepted},
		{name: "unpaired bold", row: "- **[[A]]\n", want: EntryNoncanonical},
		{name: "unpaired italic", row: "- _[[A]]\n", want: EntryNoncanonical},
		{name: "a literal underscore run", row: "- __ [[A]]\n", want: EntryNoncanonical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.row, 1)
			entries := doc.Groups[0].Entries()
			if len(entries) != 1 {
				t.Fatalf("row %q produced %d entries, want 1", tt.row, len(entries))
			}
			if got := entries[0].State; got != tt.want {
				t.Errorf("row %q state = %v, want %v", tt.row, got, tt.want)
			}
		})
	}
}

// TestARefusedRowAnchorsNothing holds the single-owner rule. Whether a row is a
// lesson is decided once, when the row is read; asking again from the raw text
// lets a row the grammar refused for one reason pass a second, looser test and
// end up carrying a side branch.
func TestARefusedRowAnchorsNothing(t *testing.T) {
	t.Parallel()
	// The parent declares a role and also names a lesson, so it is refused as
	// path.role_on_entry — it is neither a lesson nor a plain branch heading.
	doc := Parse("## P {sequence=primary}\n\n- [[A]] {sequence=primary}\n\t- 支線 {sequence=local}\n\t\t- [[C]]\n", 1)

	if !hasRule(doc, RuleRoleOnEntry) {
		t.Fatalf("the fixture no longer produces a refused parent row: %+v", doc.Diagnostics)
	}
	if !hasRule(doc, RuleLocalOrphan) {
		t.Errorf("a side branch hung from a row the grammar refused: %+v", doc.Diagnostics)
	}
	for _, g := range allGroups(doc.Groups) {
		if g.Role != RoleLocal {
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

	if !hasRule(doc, RuleRoleMisplaced) {
		t.Errorf("a marker on a continuation line was read as a declaration: %+v", doc.Diagnostics)
	}
	for _, g := range allGroups(doc.Groups) {
		if g.Role == RoleLocal {
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
	if depth := groupDepthHolding(doc.Groups, "B", 0); depth != 2 {
		t.Errorf("the row's nested list sits %d branches below its part, want 2; a misplaced marker moved it", depth)
	}
}

// countRule is how many times a rule fired.
func countRule(doc Document, rule string) int {
	n := 0
	for _, d := range doc.Diagnostics {
		if d.Rule == rule {
			n++
		}
	}
	return n
}

// groupDepthHolding is how many branches below the top a target's row sits, or
// -1 when nothing lists it.
func groupDepthHolding(groups []*Group, target string, depth int) int {
	for _, g := range groups {
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				if item.Entry.Target == target {
					return depth
				}
			case item.Branch != nil:
				if found := groupDepthHolding([]*Group{item.Branch}, target, depth+1); found >= 0 {
					return found
				}
			}
		}
	}
	return -1
}
