package sequence

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// authoringExample is the worked example from the tracked authoring contract,
// byte for byte. It exercises every part of the grammar at once — an ordered
// main line, a declared side branch hanging from the third lesson, a block
// declared out of the course, a lesson that is planned but unwritten, and a
// marker that must not appear in a displayed name — and the contract states in
// prose exactly what it produces.
const authoringExample = "" +
	"## 主線 {sequence=primary}\n" +
	"\n" +
	"1. [[L01 器材認識]]\n" +
	"2. [[L02 咖啡豆基礎]]\n" +
	"3. [[L03 研磨]]\n" +
	"\t- 進階選修（卡住才讀） {sequence=local}\n" +
	"\t\t1. [[磨豆機校正基礎]]\n" +
	"\t\t2. [[粒徑分布判讀]]\n" +
	"\t\t3. [[校正實作]]\n" +
	"4. [[L04 水溫]]\n" +
	"5. [[L05 注水手法]]\n" +
	"6. [[L06 比例與時間]]\n" +
	"7. [[L07 品飲]]\n" +
	"8. [[L08 常見問題排除]] *(尚未撰寫)*\n" +
	"\n" +
	"## 日常練習 {sequence=none}\n" +
	"\n" +
	"- [[注水練習]]：空壺練 10 分鐘\n" +
	"- [[沖煮記錄]]：每天沖一杯，把參數記下來\n"

// TestAuthoringExampleReadsAsItsContractSays holds the parser to the one
// document whose result is written down. Every clause below is a sentence of
// the contract, not a shape the implementation happened to produce.
func TestAuthoringExampleReadsAsItsContractSays(t *testing.T) {
	t.Parallel()
	doc := Parse(authoringExample, 1)

	// A document that meets the contract has nothing to tell its author. This
	// is the assertion that catches reading the undeclared-nesting rule too
	// broadly: L03 carries a nested list, and that list is the declared side
	// branch, so nothing about it is undeclared.
	if len(doc.Diagnostics) != 0 {
		t.Errorf("the contract's own example reports %d problems, want none: %+v", len(doc.Diagnostics), doc.Diagnostics)
	}

	if len(doc.Groups) != 2 {
		t.Fatalf("example has %d top-level branches, want 2: %+v", len(doc.Groups), doc.Groups)
	}
	main, routine := doc.Groups[0], doc.Groups[1]

	if main.Role != RolePrimary || routine.Role != RoleNone {
		t.Errorf("branch roles = (%v, %v), want (primary, none)", main.Role, routine.Role)
	}
	// The marker is a declaration, not part of the name.
	if main.Name != "主線" || routine.Name != "日常練習" {
		t.Errorf("branch names = (%q, %q), want (%q, %q); a recognized marker is stripped from the displayed name",
			main.Name, routine.Name, "主線", "日常練習")
	}

	// "Home reads 8 課" — the main line lists eight lessons, every one
	// canonical, and the side branch's three are not among them.
	entries := main.Entries()
	if got := len(entries); got != 8 {
		t.Errorf("the main line lists %d lessons, want 8: %+v", got, entries)
	}
	for _, e := range entries {
		if !e.Accepted() {
			t.Errorf("lesson %q has state %v, want accepted; the contract's example is canonical throughout", e.Target, e.State)
		}
	}
	// "The side branch shows as three, hanging under L03."
	subgroups := main.Subgroups()
	if len(subgroups) != 1 {
		t.Fatalf("the main line holds %d child branches, want 1: %+v", len(subgroups), subgroups)
	}
	side := subgroups[0]
	if side.Role != RoleLocal || !side.Container {
		t.Errorf("side branch = (role %v, container %t), want (local, true)", side.Role, side.Container)
	}
	if !side.Projectable() {
		t.Errorf("a valid declared side branch is not projectable; nothing else may decide this but the type")
	}
	if got := len(side.Entries()); got != 3 {
		t.Errorf("the side branch lists %d lessons, want 3: %+v", got, side.Entries())
	}
	if side.AnchorTarget != "L03 研磨" {
		t.Errorf("the side branch hangs from %q, want %q", side.AnchorTarget, "L03 研磨")
	}
	// The anchor is an identity, not just a name: it is the L03 row itself.
	if side.AnchorSpan.Zero() {
		t.Errorf("the side branch carries no anchor identity; a target string alone cannot say which row it hangs from")
	}
	if side.AnchorSpan != entries[2].Span {
		t.Errorf("side branch anchor span = %+v, want L03's own span %+v", side.AnchorSpan, entries[2].Span)
	}
	if side.Name != "進階選修（卡住才讀）" {
		t.Errorf("side branch name = %q, want the container's text without its marker", side.Name)
	}

	// The side branch sits where the author wrote it: immediately after the
	// entry it hangs from, in the branch's own item order. A consumer reads
	// this order; it never reconstructs it from targets or line numbers.
	if len(main.Items) != 9 {
		t.Fatalf("the main line holds %d items, want 9 (eight lessons and one side branch)", len(main.Items))
	}
	if main.Items[2].Entry == nil || main.Items[2].Entry.Target != "L03 研磨" {
		t.Errorf("item 3 of the main line = %+v, want the L03 entry", main.Items[2])
	}
	if main.Items[3].Branch != side {
		t.Errorf("item 4 of the main line = %+v, want the side branch that hangs from L03", main.Items[3])
	}

	// The routine block reads normally on the page but lists nothing for the
	// course; its rows are still recognized, which is why the branch is `none`
	// rather than structural — and none never projects.
	if got := len(routine.Entries()); got != 2 {
		t.Errorf("the routine block lists %d rows, want 2: %+v", got, routine.Entries())
	}
	if routine.Projectable() {
		t.Errorf("a branch declared out of the course reports itself projectable")
	}

	// The main line's order is the order it was written in, ordered-list
	// numbering included.
	wantMain := []string{
		"L01 器材認識", "L02 咖啡豆基礎", "L03 研磨", "L04 水溫",
		"L05 注水手法", "L06 比例與時間", "L07 品飲", "L08 常見問題排除",
	}
	if diff := cmp.Diff(wantMain, targets(entries)); diff != "" {
		t.Errorf("main line order mismatch (-want +got):\n%s", diff)
	}
	wantSide := []string{"磨豆機校正基礎", "粒徑分布判讀", "校正實作"}
	if diff := cmp.Diff(wantSide, targets(side.Entries())); diff != "" {
		t.Errorf("side branch order mismatch (-want +got):\n%s", diff)
	}
}

// TestOrderedAndUnorderedRowsAreTheSameCandidate holds the one equivalence the
// contract states outright. A course written with numbers and a course written
// with dashes are the same course; punctuation is not a declaration.
func TestOrderedAndUnorderedRowsAreTheSameCandidate(t *testing.T) {
	t.Parallel()
	ordered := Parse("## P {sequence=primary}\n\n1. [[A]]\n2. [[B]]\n", 1)
	unordered := Parse("## P {sequence=primary}\n\n- [[A]]\n* [[B]]\n", 1)

	if diff := cmp.Diff(targets(ordered.Groups[0].Entries()), targets(unordered.Groups[0].Entries())); diff != "" {
		t.Errorf("ordered and unordered rows disagree (-ordered +unordered):\n%s", diff)
	}
	if len(ordered.Diagnostics) != 0 || len(unordered.Diagnostics) != 0 {
		t.Errorf("a well-formed list reports problems: ordered=%+v unordered=%+v", ordered.Diagnostics, unordered.Diagnostics)
	}
}

// TestRowsTheGrammarMustNotCollect is the reject set, one case per way a
// bracketed name can appear in a document without being a lesson the course
// lists. Each is a row a reader would be wrong to be sent to.
func TestRowsTheGrammarMustNotCollect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "prose naming a note", body: "This paragraph mentions [[A]] in passing.\n"},
		{name: "block quote", body: "> - [[A]]\n"},
		{name: "callout", body: "> [!note]\n> - [[A]]\n"},
		{name: "fenced code", body: "```\n- [[A]]\n```\n"},
		{name: "indented code", body: "    - [[A]]\n"},
		{name: "inline code span", body: "- `[[A]]`\n"},
		{name: "obsidian comment", body: "- %%[[A]]%%\n"},
		{name: "embed", body: "- ![[A]]\n"},
		{name: "same-file heading anchor", body: "- [[#section]]\n"},
		{name: "same-file block anchor", body: "- [[^block]]\n"},
		{name: "unchecked task", body: "- [ ] [[A]]\n"},
		{name: "checked task", body: "- [x] [[A]]\n"},
		{name: "capital checked task", body: "- [X] [[A]]\n"},
		{name: "row with no link at all", body: "- 待建\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.body, 1)
			if got := collected(doc.Groups); len(got) != 0 {
				t.Errorf("Parse() collected %v from a row the course does not list; body = %q", got, tt.body)
			}
		})
	}
}

// TestRowsTheGrammarMustCollect is the reject set's control. Without it the
// test above passes for a parser that collects nothing at all. Every case here
// is canonical: the row's single live link is its first visible inline.
func TestRowsTheGrammarMustCollect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "plain row", body: "- [[A]]\n", want: "A"},
		{name: "ordered row", body: "1. [[A]]\n", want: "A"},
		{name: "row with display text", body: "- [[A|first lesson]]\n", want: "A"},
		{name: "row with a cross-file anchor", body: "- [[A#section]]\n", want: "A"},
		{name: "row with trailing prose", body: "- [[A]] *(尚未撰寫)*\n", want: "A"},
		{name: "link-first action sentence", body: "- [[A]] 讀完後做一頁筆記\n", want: "A"},
		{name: "bold-wrapped link", body: "- **[[A]]**\n", want: "A"},
		{name: "italic-wrapped link", body: "- *[[A]]*\n", want: "A"},
		{name: "underscore-italic link", body: "- _[[A]]_\n", want: "A"},
		{name: "bold-wrapped link with trailing prose", body: "- **[[A]]** 進度的一半\n", want: "A"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.body, 1)
			if diff := cmp.Diff([]string{tt.want}, collected(doc.Groups)); diff != "" {
				t.Errorf("Parse() mismatch for %q (-want +got):\n%s", tt.body, diff)
			}
			if len(doc.Diagnostics) != 0 {
				t.Errorf("a canonical row reported problems: %+v", doc.Diagnostics)
			}
		})
	}
}

// TestNoncanonicalRowsAreCandidatesButNeverEntries holds the ruling that split
// the candidate grammar from canonical form. A row whose single live link is
// not the first visible inline is still a row — its branch has rows, and a
// side branch can still hang from it — but no entry is accepted from it, and
// the author is told on the exact line. Guessing whether the label or the link
// was the lesson would be inference, and the grammar does not infer.
func TestNoncanonicalRowsAreCandidatesButNeverEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "leading prose", body: "- 第一課 [[A]]\n"},
		{name: "leading embed", body: "- ![[cover]] [[A]]\n"},
		{name: "leading same-file anchor", body: "- [[#section]] [[A]]\n"},
		{name: "leading inline code", body: "- `第一課` [[A]]\n"},
		{name: "bracket run that is not a task", body: "- [x][[A]]\n"},
		{name: "bracketed word that is not a task", body: "- [z] [[A]]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.body, 1)

			entries := doc.Groups[0].Entries()
			if len(entries) != 1 {
				t.Fatalf("the branch lists %d rows, want 1; a noncanonical row is still a row", len(entries))
			}
			e := entries[0]
			if e.State != EntryNoncanonical || e.Accepted() {
				t.Errorf("row state = %v (accepted %t), want noncanonical and not accepted", e.State, e.Accepted())
			}
			if e.Target != "A" {
				t.Errorf("row target = %q, want %q kept for identity even though no entry is accepted", e.Target, "A")
			}
			if got := collected(doc.Groups); len(got) != 0 {
				t.Errorf("a noncanonical row produced accepted targets %v, want none", got)
			}
			d := findRule(doc, RuleEntryNoncanonical)
			if d == nil {
				t.Fatalf("Parse() did not report %s: %+v", RuleEntryNoncanonical, doc.Diagnostics)
			}
			if d.Line != 3 {
				t.Errorf("%s reported on line %d, want 3", RuleEntryNoncanonical, d.Line)
			}
		})
	}
}

// TestTargetScopeReadsTheWholeItem holds the target-scope ruling from both
// sides at once. A loose item's paragraph after its child list is still the
// item's own words — a second live link there makes the row multi-target — and
// the child list's own rows are never the item's words, or the control case
// would report the same thing.
func TestTargetScopeReadsTheWholeItem(t *testing.T) {
	t.Parallel()
	// The continuation paragraph names a second note: the row is multi-target
	// even though the second link sits after the nested list.
	doc := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- [[B]]\n\n\t這段補充又提到 [[C]]。\n", 1)
	d := findRule(doc, RuleEntryMultiTarget)
	if d == nil {
		t.Fatalf("a second live link in the item's continuation was not reported: %+v", doc.Diagnostics)
	}
	if d.Line != 3 {
		t.Errorf("%s reported on line %d, want 3, the row it is about", RuleEntryMultiTarget, d.Line)
	}
	entries := doc.Groups[0].Entries()
	if len(entries) != 1 || entries[0].State != EntryMultiTarget {
		t.Errorf("row = %+v, want one multi-target candidate", entries)
	}

	// The control: the same shape with no link in the continuation is one
	// canonical entry. This is what proves the child list's [[B]] was excluded
	// from the row's scope — included, it would make this row multi-target too.
	control := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- [[B]]\n\n\t這段補充沒有連結。\n", 1)
	if findRule(control, RuleEntryMultiTarget) != nil {
		t.Fatalf("the nested list's own link was counted into the enclosing row's scope: %+v", control.Diagnostics)
	}
	got := control.Groups[0].Entries()
	if len(got) != 1 || !got[0].Accepted() || got[0].Target != "A" {
		t.Errorf("control row = %+v, want one accepted entry for A", got)
	}
}

// TestCheckboxNeverAnchorsALocalBranch holds the checkbox ruling at its second
// door. The row itself is never a candidate — the reject set already says so —
// and it cannot hold a side branch either: a local container nested under a
// checkbox has nothing to hang from, and the branch does not project.
func TestCheckboxNeverAnchorsALocalBranch(t *testing.T) {
	t.Parallel()
	doc := Parse("## P {sequence=primary}\n\n- [ ] [[A]]\n\t- 旁支 {sequence=local}\n\t\t- [[B]]\n", 1)

	if findRule(doc, RuleLocalOrphan) == nil {
		t.Fatalf("a side branch under a checkbox row was not reported as an orphan: %+v", doc.Diagnostics)
	}
	subgroups := doc.Groups[0].Subgroups()
	if len(subgroups) != 1 {
		t.Fatalf("branch holds %d subgroups, want the orphaned side branch alone: %+v", len(subgroups), subgroups)
	}
	side := subgroups[0]
	if !side.Invalid || side.Projectable() {
		t.Errorf("orphaned side branch = (invalid %t, projectable %t), want (true, false)", side.Invalid, side.Projectable())
	}
	if side.AnchorTarget != "" || !side.AnchorSpan.Zero() {
		t.Errorf("orphaned side branch still carries anchor (%q, %+v); a checkbox row is not an anchor", side.AnchorTarget, side.AnchorSpan)
	}
	if got := len(doc.Groups[0].Entries()); got != 0 {
		t.Errorf("the checkbox row itself was counted as a row: %d entries", got)
	}
}

// TestRoleOnEntryKeepsTheRowAndProjectsNothing holds the both-at-once ruling.
// A row that is a lesson and a branch heading is a contradiction the author
// resolves — but the row is still a row its branch lists, so the branch does
// not misread as merely structural, and neither the entry nor the branch it
// opened projects in the meantime. Diagnosing while quietly guessing one of
// the two meanings would be worse than either.
func TestRoleOnEntryKeepsTheRowAndProjectsNothing(t *testing.T) {
	t.Parallel()
	doc := Parse("## P\n\n- [[B]] {sequence=local}\n\t- [[C]]\n", 1)

	if findRule(doc, RuleRoleOnEntry) == nil {
		t.Fatalf("a row that is both lesson and branch was not reported: %+v", doc.Diagnostics)
	}
	branch := doc.Groups[0]
	// The row keeps the branch's state honest: rows and no declaration is
	// unclassified, never structural.
	if branch.Role != RoleUnclassified {
		t.Errorf("branch role = %v, want unclassified; the conflicted row is still a row it lists", branch.Role)
	}
	if findRule(doc, RuleRoleMissing) == nil {
		t.Errorf("an undeclared branch whose only row is conflicted was not asked to declare itself: %+v", doc.Diagnostics)
	}
	entries := branch.Entries()
	if len(entries) != 1 {
		t.Fatalf("branch lists %d rows, want 1: %+v", len(entries), entries)
	}
	if entries[0].State != EntryRoleOnEntry || entries[0].Accepted() {
		t.Errorf("conflicted row state = %v (accepted %t), want role-on-entry and not accepted", entries[0].State, entries[0].Accepted())
	}
	if entries[0].Target != "B" {
		t.Errorf("conflicted row target = %q, want %q kept for identity", entries[0].Target, "B")
	}
	subgroups := branch.Subgroups()
	if len(subgroups) != 1 {
		t.Fatalf("branch holds %d subgroups, want the conflicted container alone", len(subgroups))
	}
	if !subgroups[0].Invalid || subgroups[0].Projectable() {
		t.Errorf("conflicted container = (invalid %t, projectable %t), want (true, false)", subgroups[0].Invalid, subgroups[0].Projectable())
	}
}

// TestInvalidBranchesSaySoInTheType holds the one-verdict rule. Navigation and
// the judge read Projectable and Invalid off the group; neither re-derives the
// verdict from the diagnostics, because two readings of one report are two
// different courses.
func TestInvalidBranchesSaySoInTheType(t *testing.T) {
	t.Parallel()

	t.Run("a role conflict does not project", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## Part {sequence=none}\n\n### Inner {sequence=primary}\n\n- [[A]]\n", 1)
		inner := doc.Groups[0].Subgroups()[0]
		if !inner.Invalid || inner.Projectable() {
			t.Errorf("conflicted branch = (invalid %t, projectable %t), want (true, false)", inner.Invalid, inner.Projectable())
		}
	})

	t.Run("an orphaned side branch does not project", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## Part {sequence=primary}\n\n- 旁支 {sequence=local}\n\t- [[A]]\n", 1)
		side := doc.Groups[0].Subgroups()[0]
		if !side.Invalid || side.Projectable() {
			t.Errorf("orphaned branch = (invalid %t, projectable %t), want (true, false)", side.Invalid, side.Projectable())
		}
	})

	t.Run("a side branch nested too deep does not project, and the outer one still does", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## Part {sequence=primary}\n\n- [[A]]\n\t- 旁支 {sequence=local}\n\t\t- [[B]]\n\t\t\t- 更深 {sequence=local}\n\t\t\t\t- [[C]]\n", 1)
		outer := doc.Groups[0].Subgroups()[0]
		if outer.Invalid || !outer.Projectable() {
			t.Errorf("outer side branch = (invalid %t, projectable %t), want (false, true); the error is the inner one's", outer.Invalid, outer.Projectable())
		}
		inner := outer.Subgroups()[0]
		if !inner.Invalid || inner.Projectable() {
			t.Errorf("inner side branch = (invalid %t, projectable %t), want (true, false)", inner.Invalid, inner.Projectable())
		}
	})
}

// TestCandidateIdentityDisambiguatesRepeatedTargets holds the reason a span
// exists at all. Two rows may name the same note; the side branch under the
// second must say it hangs from the second, and a target string alone cannot.
func TestCandidateIdentityDisambiguatesRepeatedTargets(t *testing.T) {
	t.Parallel()
	doc := Parse("## P {sequence=primary}\n\n- [[A]]\n- [[A]]\n\t- 旁支 {sequence=local}\n\t\t- [[B]]\n", 1)

	entries := doc.Groups[0].Entries()
	if len(entries) != 2 {
		t.Fatalf("branch lists %d rows, want the two that both name A", len(entries))
	}
	if entries[0].Span == entries[1].Span {
		t.Fatalf("two distinct rows share one span %+v; a span that cannot tell rows apart identifies nothing", entries[0].Span)
	}
	side := doc.Groups[0].Subgroups()[0]
	if side.AnchorSpan != entries[1].Span {
		t.Errorf("side branch anchor span = %+v, want the second A row's span %+v", side.AnchorSpan, entries[1].Span)
	}
	if side.AnchorSpan == entries[0].Span {
		t.Errorf("side branch anchors to the first A row; the enclosing row is the second")
	}
}

// TestBranchStateComesFromCandidatesNotFromResolution holds the split the
// contract is built on. A branch that lists a row nobody can resolve is still a
// branch that lists rows, so it is unclassified once and stays unclassified —
// fixing the row never produces a second round of "this branch has no role".
func TestBranchStateComesFromCandidatesNotFromResolution(t *testing.T) {
	t.Parallel()
	// A row naming two notes is a candidate canonical validation refuses to
	// turn into an entry. The branch it sits in must still count as listing
	// rows.
	doc := Parse("## P\n\n- [[A]] and [[B]]\n", 1)
	if len(doc.Groups) != 1 {
		t.Fatalf("branches = %d, want 1", len(doc.Groups))
	}
	if doc.Groups[0].Role != RoleUnclassified {
		t.Errorf("branch role = %v, want unclassified; a row the grammar recognized is a row", doc.Groups[0].Role)
	}
	if !hasRule(doc, RuleRoleMissing) {
		t.Errorf("an undeclared branch that lists rows was not reported: %+v", doc.Diagnostics)
	}
	if !hasRule(doc, RuleEntryMultiTarget) {
		t.Errorf("a row naming two notes was not reported: %+v", doc.Diagnostics)
	}
	entries := doc.Groups[0].Entries()
	if len(entries) != 1 || entries[0].State != EntryMultiTarget || entries[0].Target != "" {
		t.Errorf("row = %+v, want one multi-target candidate with no target; naming one of two notes would be a guess", entries)
	}
	// It is never turned into a lesson: guessing the first would be a guess and
	// taking both would invent one.
	if got := collected(doc.Groups); len(got) != 0 {
		t.Errorf("a row naming two notes produced lesson targets %v, want none", got)
	}
}

// TestABranchThatOnlyGroupsOtherBranchesIsSilent separates the two undeclared
// states. Forgetting to declare is a problem; a heading whose whole job is to
// hold other headings is not, and reporting it would train the author to ignore
// the report.
func TestABranchThatOnlyGroupsOtherBranchesIsSilent(t *testing.T) {
	t.Parallel()
	doc := Parse("## Part\n\n### Module {sequence=primary}\n\n- [[A]]\n", 1)
	if len(doc.Diagnostics) != 0 {
		t.Errorf("a branch that lists nothing of its own was reported: %+v", doc.Diagnostics)
	}
	if doc.Groups[0].Role != RoleStructural {
		t.Errorf("outer branch role = %v, want structural", doc.Groups[0].Role)
	}
}

// TestDiagnosticsNameWhatTheAuthorHasToDecide walks the rules one at a time.
// Each case is a document a person could plausibly write, and the assertion is
// that the right rule fires on the right line.
func TestDiagnosticsNameWhatTheAuthorHasToDecide(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		body     string
		wantRule string
		wantLine int
	}{
		{
			name:     "a branch that lists lessons but declares nothing",
			body:     "## Part\n\n- [[A]]\n",
			wantRule: RuleRoleMissing,
			wantLine: 1,
		},
		{
			name:     "two roles on one branch",
			body:     "## Part {sequence=primary} {sequence=local}\n\n- [[A]]\n",
			wantRule: RuleRoleDuplicate,
			wantLine: 1,
		},
		{
			name:     "a branch in the course under a branch out of it",
			body:     "## Part {sequence=none}\n\n### Inner {sequence=primary}\n\n- [[A]]\n",
			wantRule: RuleRoleConflict,
			wantLine: 3,
		},
		{
			name:     "a side branch with nothing to hang from",
			body:     "## Part {sequence=primary}\n\n- 旁支 {sequence=local}\n\t- [[A]]\n",
			wantRule: RuleLocalOrphan,
			wantLine: 3,
		},
		{
			name:     "a side branch inside a side branch",
			body:     "## Part {sequence=primary}\n\n- [[A]]\n\t- 旁支 {sequence=local}\n\t\t- [[B]]\n\t\t\t- 更深 {sequence=local}\n\t\t\t\t- [[C]]\n",
			wantRule: RuleNestingTooDeep,
			wantLine: 6,
		},
		{
			name:     "a row that is both a lesson and a branch heading",
			body:     "## Part {sequence=primary}\n\n- [[A]]\n\t- [[B]] {sequence=local}\n\t\t- [[C]]\n",
			wantRule: RuleRoleOnEntry,
			wantLine: 4,
		},
		{
			name:     "a value outside the three",
			body:     "## Part {sequence=main}\n\n- [[A]]\n",
			wantRule: RuleRoleInvalid,
			wantLine: 1,
		},
		{
			name:     "a marker with no value",
			body:     "## Part {sequence}\n\n- [[A]]\n",
			wantRule: RuleRoleInvalid,
			wantLine: 1,
		},
		{
			name:     "a marker in a paragraph",
			body:     "## Part {sequence=primary}\n\n這一段 {sequence=local} 沒有宣告任何東西。\n\n- [[A]]\n",
			wantRule: RuleRoleMisplaced,
			wantLine: 3,
		},
		{
			// The type's own comment names a quote among the blocks a stray
			// marker is reported in, and the contract promises a misplaced
			// declaration never fails silently.
			name:     "a marker in a quote",
			body:     "## Part {sequence=primary}\n\n> 引用裡的 {sequence=local} 宣告不了任何東西。\n\n- [[A]]\n",
			wantRule: RuleRoleMisplaced,
			wantLine: 3,
		},
		{
			name:     "a marker on a level-1 heading",
			body:     "# Course {sequence=primary}\n\n## Part {sequence=primary}\n\n- [[A]]\n",
			wantRule: RuleRoleMisplaced,
			wantLine: 1,
		},
		{
			name:     "a marker on a row that opens no list",
			body:     "## Part {sequence=primary}\n\n- 旁支 {sequence=local}\n",
			wantRule: RuleRoleMisplaced,
			wantLine: 3,
		},
		{
			name:     "a marker that is not at the end of its line",
			body:     "## Part {sequence=primary} 主線\n\n- [[A]]\n",
			wantRule: RuleRoleMisplaced,
			wantLine: 1,
		},
		{
			name:     "a marker in a row's continuation paragraph",
			body:     "## Part {sequence=primary}\n\n- [[A]]\n\t- [[B]]\n\n\t這一段的結尾 {sequence=local}\n",
			wantRule: RuleRoleMisplaced,
			wantLine: 6,
		},
		{
			name:     "a lesson listed before the first branch",
			body:     "- [[A]]\n\n## Part {sequence=primary}\n\n- [[B]]\n",
			wantRule: RuleEntryOutsideBranch,
			wantLine: 1,
		},
		{
			name:     "an undeclared nested list",
			body:     "## Part {sequence=primary}\n\n- [[A]]\n\t- [[B]]\n",
			wantRule: RuleRoleMissing,
			wantLine: 4,
		},
		{
			name:     "a row naming more than one note",
			body:     "## Part {sequence=primary}\n\n- [[A]] 或 [[B]]\n",
			wantRule: RuleEntryMultiTarget,
			wantLine: 3,
		},
		{
			name:     "a row whose link is not first",
			body:     "## Part {sequence=primary}\n\n- 第一課 [[A]]\n",
			wantRule: RuleEntryNoncanonical,
			wantLine: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse(tt.body, 1)
			found := findRule(doc, tt.wantRule)
			if found == nil {
				t.Fatalf("Parse() did not report %s; got %+v", tt.wantRule, doc.Diagnostics)
			}
			if found.Line != tt.wantLine {
				t.Errorf("%s reported on line %d, want %d; a diagnostic an editor cannot find is not a report",
					tt.wantRule, found.Line, tt.wantLine)
			}
			if found.Message == "" {
				t.Errorf("%s carries no message", tt.wantRule)
			}
		})
	}
}

// TestAQuotedMarkerIsNotAWrittenOne keeps a note that explains this syntax from
// being told off for explaining it. A marker inside a fence or a code span is
// being shown, not declared, and the same zones that keep a quoted link out of
// the course keep a quoted marker out of the report.
func TestAQuotedMarkerIsNotAWrittenOne(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "fenced example", body: "```markdown\n## 主線 {sequence=primary}\n```\n"},
		{name: "code span", body: "`{sequence=local}` 是三個值之一。\n"},
		{name: "obsidian comment", body: "%%舊寫法 {sequence=primary}%%\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## Part {sequence=primary}\n\n- [[A]]\n\n"+tt.body, 1)
			if hasRule(doc, RuleRoleMisplaced) {
				t.Errorf("a quoted marker was reported as misplaced: %+v", doc.Diagnostics)
			}
		})
	}

	// The control: the same sentence with the marker actually written in prose
	// is still reported, so the exemption above cannot be "report nothing".
	doc := Parse("## Part {sequence=primary}\n\n- [[A]]\n\n這一段 {sequence=local} 沒有宣告任何東西。\n", 1)
	if !hasRule(doc, RuleRoleMisplaced) {
		t.Errorf("a marker written in prose was not reported: %+v", doc.Diagnostics)
	}
}

// TestAQuotedMarkerOnAHeadingIsAReference holds the same rule at the place it
// was measured broken: a heading or container row quoting the syntax it
// explains. The quotation neither declares a role nor draws a report — and a
// real marker on the same line still does both.
func TestAQuotedMarkerOnAHeadingIsAReference(t *testing.T) {
	t.Parallel()

	t.Run("code span on a heading declares nothing and is not misplaced", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## 語法說明 `{sequence=primary}`\n\n- [[A]]\n", 1)
		if hasRule(doc, RuleRoleMisplaced) {
			t.Errorf("a quoted marker on a heading was reported: %+v", doc.Diagnostics)
		}
		if got := doc.Groups[0].Role; got != RoleUnclassified {
			t.Errorf("branch role = %v, want unclassified; a quoted marker must not become a role", got)
		}
		if !hasRule(doc, RuleRoleMissing) {
			t.Errorf("the branch under a quoted marker was not asked to declare itself: %+v", doc.Diagnostics)
		}
	})

	t.Run("obsidian comment on a heading declares nothing", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## 語法說明 %%{sequence=primary}%%\n\n- [[A]]\n", 1)
		if hasRule(doc, RuleRoleMisplaced) {
			t.Errorf("a commented-out marker on a heading was reported: %+v", doc.Diagnostics)
		}
		if got := doc.Groups[0].Role; got != RoleUnclassified {
			t.Errorf("branch role = %v, want unclassified; a switched-off marker must not become a role", got)
		}
	})

	t.Run("a real marker still wins on a line that also quotes one", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## 語法說明 `{sequence=local}` {sequence=primary}\n\n- [[A]]\n", 1)
		if len(doc.Diagnostics) != 0 {
			t.Errorf("a heading with one real marker reported problems: %+v", doc.Diagnostics)
		}
		if got := doc.Groups[0].Role; got != RolePrimary {
			t.Errorf("branch role = %v, want primary from the marker outside the code span", got)
		}
	})

	t.Run("a quoted marker on a container row does not open a branch", func(t *testing.T) {
		t.Parallel()
		doc := Parse("## P {sequence=primary}\n\n- [[A]]\n\t- 旁支 `{sequence=local}`\n\t\t- [[B]]\n", 1)
		if hasRule(doc, RuleRoleMisplaced) {
			t.Errorf("a quoted marker on a row was reported: %+v", doc.Diagnostics)
		}
		if hasRule(doc, RuleLocalOrphan) || hasRule(doc, RuleRoleOnEntry) {
			t.Errorf("a quoted marker opened a branch: %+v", doc.Diagnostics)
		}
		// The nested rows are simply undeclared nesting now, reported as such.
		if !hasRule(doc, RuleRoleMissing) {
			t.Errorf("the undeclared nested list was not reported: %+v", doc.Diagnostics)
		}
	})
}

// TestNoneSuppressesTheMissingRoleBeneathIt holds the contract's second step.
// Declaring a block out of the course is an answer for everything inside it;
// asking again about each nested list would punish the author for answering.
func TestNoneSuppressesTheMissingRoleBeneathIt(t *testing.T) {
	t.Parallel()
	doc := Parse("## 日常 {sequence=none}\n\n- [[A]]\n\t- [[B]]\n", 1)
	if hasRule(doc, RuleRoleMissing) {
		t.Errorf("a branch under a declared none was asked to declare itself: %+v", doc.Diagnostics)
	}
	if len(doc.Diagnostics) != 0 {
		t.Errorf("a block declared out of the course reported problems: %+v", doc.Diagnostics)
	}
}

// TestLineNumbersAreFileLinesNotBodyLines keeps a diagnostic findable in the
// editor the author actually uses: a note with frontmatter reports the line the
// file shows, not the line the body starts counting from.
func TestLineNumbersAreFileLinesNotBodyLines(t *testing.T) {
	t.Parallel()
	const frontmatterLines = 6
	doc := Parse("## Part\n\n- [[A]]\n", frontmatterLines+1)
	if !hasRule(doc, RuleRoleMissing) {
		t.Fatalf("expected an undeclared branch to be reported: %+v", doc.Diagnostics)
	}
	for _, d := range doc.Diagnostics {
		if d.Rule == RuleRoleMissing && d.Line != frontmatterLines+1 {
			t.Errorf("%s reported on line %d, want %d", d.Rule, d.Line, frontmatterLines+1)
		}
	}
}

// TestParseIsDeterministic guards the one property every consumer assumes
// without stating: two reads of one document agree, so a rebuild does not
// reshuffle a course.
func TestParseIsDeterministic(t *testing.T) {
	t.Parallel()
	first := Parse(authoringExample, 1)
	second := Parse(authoringExample, 1)
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("Parse() is not deterministic (-first +second):\n%s", diff)
	}
}

func targets(cs []*Candidate) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Target)
	}
	return out
}

// collected is every accepted target the document lists, at any depth. Only an
// accepted entry may become a lesson, so this is what "the course lists"
// means everywhere below.
func collected(groups []*Group) []string {
	var out []string
	for _, g := range groups {
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				if item.Entry.Accepted() {
					out = append(out, item.Entry.Target)
				}
			case item.Branch != nil:
				out = append(out, collected([]*Group{item.Branch})...)
			}
		}
	}
	return out
}

func hasRule(doc Document, rule string) bool {
	return findRule(doc, rule) != nil
}

func findRule(doc Document, rule string) *Diagnostic {
	for i := range doc.Diagnostics {
		if doc.Diagnostics[i].Rule == rule {
			return &doc.Diagnostics[i]
		}
	}
	return nil
}
