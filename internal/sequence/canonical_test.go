package sequence

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestTheFirstVisibleInlineDecidesCanonicalForm holds what "the row opens with
// its link" means. It is a question about what a reader sees, so it is settled
// from the inline structure and the invisible zones: emphasis Markdown paired
// is markup, an unpaired run prints as itself, and a comment shows nothing. A
// row that fails is still a row and keeps its target for identity, but it is
// not a lesson, and the author is told on the row's own line.
func TestTheFirstVisibleInlineDecidesCanonicalForm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  string
		want EntryState
	}{
		{name: "plain row", row: "- [[A]]\n", want: EntryAccepted},
		{name: "ordered row", row: "1. [[A]]\n", want: EntryAccepted},
		{name: "display text", row: "- [[A|first lesson]]\n", want: EntryAccepted},
		{name: "a cross-file anchor", row: "- [[A#section]]\n", want: EntryAccepted},
		{name: "trailing prose", row: "- [[A]] *(尚未撰寫)*\n", want: EntryAccepted},
		{name: "bold around the link", row: "- **[[A]]**\n", want: EntryAccepted},
		{name: "bold around the link with trailing prose", row: "- **[[A]]** 進度的一半\n", want: EntryAccepted},
		{name: "italic around the link", row: "- *[[A]]*\n", want: EntryAccepted},
		{name: "underscore italic around the link", row: "- _[[A]]_\n", want: EntryAccepted},
		{name: "bold and italic together", row: "- ***[[A]]***\n", want: EntryAccepted},
		{name: "bold wrapping italic", row: "- **_[[A]]_**\n", want: EntryAccepted},
		{name: "a comment before the link is invisible", row: "- %%hidden%% [[A]]\n", want: EntryAccepted},
		{name: "prose before the link", row: "- 第一課 [[A]]\n", want: EntryNoncanonical},
		{name: "an embed before the link", row: "- ![[cover]] [[A]]\n", want: EntryNoncanonical},
		{name: "a same-file anchor before the link", row: "- [[#sec]] [[A]]\n", want: EntryNoncanonical},
		{name: "inline code before the link", row: "- `x` [[A]]\n", want: EntryNoncanonical},
		{name: "unpaired bold", row: "- **[[A]]\n", want: EntryNoncanonical},
		{name: "unpaired italic", row: "- _[[A]]\n", want: EntryNoncanonical},
		{name: "a literal underscore run", row: "- __ [[A]]\n", want: EntryNoncanonical},
		{name: "a bracket run that is not a task", row: "- [x][[A]]\n", want: EntryNoncanonical},
		{name: "a bracketed word that is not a task", row: "- [z] [[A]]\n", want: EntryNoncanonical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.row, 1)
			entries := doc.Groups[0].entries()
			if len(entries) != 1 {
				t.Fatalf("row %q produced %d entries, want 1: %+v", tt.row, len(entries), doc.Groups[0].Items)
			}
			if got := entries[0]; got.State != tt.want || got.Target != "A" {
				t.Errorf("row %q = (%v, target %q), want (%v, target %q)", tt.row, got.State, got.Target, tt.want, "A")
			}
			var want []Diagnostic
			if tt.want == EntryNoncanonical {
				want = []Diagnostic{{Rule: RuleEntryNoncanonical, Line: 3}}
			}
			if diff := cmp.Diff(want, doc.Diagnostics, cmpopts.IgnoreFields(Diagnostic{}, "Message", "Evidence")); diff != "" {
				t.Errorf("row %q diagnostics mismatch (-want +got):\n%s", tt.row, diff)
			}
		})
	}
}

// TestASideBranchHangsOnlyFromALesson. A side branch attaches to the lesson
// above it, and a row the grammar refused is not a lesson: nesting a branch
// under one leaves it with nothing to hang from, whatever the refused row
// happens to name.
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
			if findRule(doc, RuleLocalOrphan) == nil {
				t.Errorf("a side branch under %q was given an anchor; diagnostics = %+v", tt.parent, doc.Diagnostics)
			}
			for _, item := range walk(doc.Groups) {
				if g := item.Branch; g != nil && g.Role == RoleLocal && g.Projectable() {
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
	if findRule(doc, RuleLocalOrphan) != nil {
		t.Fatalf("a side branch under a lesson was called an orphan: %+v", doc.Diagnostics)
	}
	var local *Group
	for _, item := range walk(doc.Groups) {
		if g := item.Branch; g != nil && g.Role == RoleLocal {
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
	if findRule(doc, RuleRoleMisplaced) == nil {
		t.Errorf("a marker on a continuation line was read as a declaration: %+v", doc.Diagnostics)
	}
	for _, item := range walk(doc.Groups) {
		if g := item.Branch; g != nil && g.Role == RoleLocal {
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
	if findRule(doc, RuleRoleMissing) == nil {
		t.Errorf("an undeclared nested list vanished without a word: %+v", doc.Diagnostics)
	}
	var nested *Group
	for _, item := range walk(doc.Groups) {
		if g := item.Branch; g != nil && g.Container {
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
