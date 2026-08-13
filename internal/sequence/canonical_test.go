package sequence

import "testing"

// TestTheFirstVisibleInlineDecidesCanonicalForm holds what "the row opens with
// its link" means. It is a question about what a reader sees, so it is settled
// from the inline structure and the invisible zones — not by skipping every
// asterisk and underscore byte, which reads a literal delimiter as if it were
// emphasis and a comment as if it were prose.
func TestTheFirstVisibleInlineDecidesCanonicalForm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  string
		want EntryState
	}{
		{name: "bold around the link", row: "- **[[A]]**\n", want: EntryAccepted},
		{name: "italic around the link", row: "- _[[A]]_\n", want: EntryAccepted},
		{name: "a comment before the link is invisible", row: "- %%hidden%% [[A]]\n", want: EntryAccepted},
		{name: "a literal underscore run is visible", row: "- __ [[A]]\n", want: EntryNoncanonical},
		{name: "prose before the link", row: "- 第一課 [[A]]\n", want: EntryNoncanonical},
		{name: "an embed before the link", row: "- ![[cover]] [[A]]\n", want: EntryNoncanonical},
		{name: "a same-file anchor before the link", row: "- [[#sec]] [[A]]\n", want: EntryNoncanonical},
		{name: "inline code before the link", row: "- `x` [[A]]\n", want: EntryNoncanonical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.row, 1)
			entries := doc.Groups[0].Entries()
			if len(entries) != 1 {
				t.Fatalf("row %q produced %d entries, want 1: %+v", tt.row, len(entries), doc.Groups[0].Items)
			}
			if got := entries[0].State; got != tt.want {
				t.Errorf("row %q state = %v, want %v", tt.row, got, tt.want)
			}
			reported := hasRule(doc, RuleEntryNoncanonical)
			if want := tt.want == EntryNoncanonical; reported != want {
				t.Errorf("row %q reported %s = %t, want %t; diagnostics = %+v",
					tt.row, RuleEntryNoncanonical, reported, want, doc.Diagnostics)
			}
		})
	}
}

// TestASideBranchHangsOnlyFromALessonHoldsItsAnchor. A side branch attaches to
// the lesson above it, and a row the grammar refused is not a lesson: nesting a
// branch under one leaves it with nothing to hang from, whatever the refused
// row happens to name.
func TestASideBranchHangsOnlyFromALesson(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		parent string
	}{
		{name: "a row whose link is not first", parent: "- 補充 [[A]]"},
		{name: "a row naming two lessons", parent: "- [[A]] 與 [[B]]"},
		{name: "a task row", parent: "- [ ] [[A]]"},
		{name: "a row naming nothing", parent: "- 還沒決定"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := "## P {sequence=primary}\n\n" + tt.parent + "\n\t- 旁支 {sequence=local}\n\t\t- [[C]]\n"
			doc := Parse(body, 1)
			if !hasRule(doc, RuleLocalOrphan) {
				t.Errorf("a side branch under %q was given an anchor; diagnostics = %+v", tt.parent, doc.Diagnostics)
			}
			for _, g := range allGroups(doc.Groups) {
				if g.Role == RoleLocal && g.Projectable() {
					t.Errorf("a side branch with nothing to hang from still projects: %+v", g)
				}
			}
		})
	}
}

// TestASideBranchUnderALessonKeepsItsAnchor is the control: the same shape with
// a lesson above it attaches, so the test above cannot pass by refusing every
// side branch.
func TestASideBranchUnderALessonKeepsItsAnchor(t *testing.T) {
	t.Parallel()
	doc := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- 旁支 {sequence=local}\n\t\t- [[C]]\n", 1)
	if hasRule(doc, RuleLocalOrphan) {
		t.Fatalf("a side branch under a lesson was called an orphan: %+v", doc.Diagnostics)
	}
	var local *Group
	for _, g := range allGroups(doc.Groups) {
		if g.Role == RoleLocal {
			local = g
		}
	}
	if local == nil || !local.Projectable() || local.AnchorTarget != "A" {
		t.Errorf("side branch = %+v, want a projectable branch anchored on A", local)
	}
}

// TestADeclarationLivesOnTheRowsOwnLine holds where a marker is read. A row can
// run to several lines; the declaration belongs on the first, and one written
// further down is read by nobody — so it declares nothing and says so.
func TestADeclarationLivesOnTheRowsOwnLine(t *testing.T) {
	t.Parallel()
	// The marker sits on the second line of the row's own paragraph.
	doc := Parse("## P {sequence=primary}\n\n- 旁支\n  說明 {sequence=local}\n\t- [[A]]\n", 1)
	if !hasRule(doc, RuleRoleMisplaced) {
		t.Errorf("a marker on a continuation line was read as a declaration: %+v", doc.Diagnostics)
	}
	for _, g := range allGroups(doc.Groups) {
		if g.Role == RoleLocal {
			t.Errorf("a marker off the row's own line opened a side branch: %+v", g)
		}
	}
}

// TestAnUndeclaredNestedListIsNeverSilent holds the ruling that nesting nobody
// declared keeps its source, projects nothing, and is reported. A nested list
// holding no links at all is still nesting the author has to explain; letting it
// vanish is the silent flattening the ruling forbids.
func TestAnUndeclaredNestedListIsNeverSilent(t *testing.T) {
	t.Parallel()
	doc := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- 還沒決定怎麼歸\n", 1)
	if !hasRule(doc, RuleRoleMissing) {
		t.Errorf("an undeclared nested list vanished without a word: %+v", doc.Diagnostics)
	}
	var nested *Group
	for _, g := range allGroups(doc.Groups) {
		if g.Container {
			nested = g
		}
	}
	if nested == nil {
		t.Fatalf("the nested list left no branch behind: %+v", doc.Groups)
	}
	if nested.Projectable() {
		t.Errorf("an undeclared nested list projects: %+v", nested)
	}
}

// TestAnUndeclaredContainerSaysWhatItIs keeps the report honest. A heading that
// lists lessons and a nested list that lists none are both undeclared, and
// telling the author the second "lists lessons" sends them looking for lessons
// that are not there.
func TestAnUndeclaredContainerSaysWhatItIs(t *testing.T) {
	t.Parallel()
	nested := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- 還沒決定怎麼歸\n", 1)
	heading := Parse("## P\n\n- [[A]]\n", 1)

	nestedMessage := ruleMessage(nested, RuleRoleMissing)
	headingMessage := ruleMessage(heading, RuleRoleMissing)
	if nestedMessage == "" || headingMessage == "" {
		t.Fatalf("both shapes must report: nested=%q heading=%q", nestedMessage, headingMessage)
	}
	if nestedMessage == headingMessage {
		t.Errorf("an undeclared nested list is told it %q, the same sentence a heading gets", nestedMessage)
	}
}

// TestAQuietStructuralHeadingStaysQuiet is the exception the contract keeps: a
// heading whose whole job is to hold other headings lists nothing and is
// correct, so it draws no report.
func TestAQuietStructuralHeadingStaysQuiet(t *testing.T) {
	t.Parallel()
	doc := Parse("## Part\n\n### Module {sequence=primary}\n\n- [[A]]\n", 1)
	if len(doc.Diagnostics) != 0 {
		t.Errorf("a heading that only groups other headings was reported: %+v", doc.Diagnostics)
	}
}

// allGroups is every branch in the document, at any depth.
func allGroups(groups []*Group) []*Group {
	var out []*Group
	for _, g := range groups {
		out = append(out, g)
		for _, item := range g.Items {
			if item.Branch != nil {
				out = append(out, allGroups([]*Group{item.Branch})...)
			}
		}
	}
	return out
}

func ruleMessage(doc Document, rule string) string {
	for _, d := range doc.Diagnostics {
		if d.Rule == rule {
			return d.Message
		}
	}
	return ""
}
