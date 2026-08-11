package sequence

import (
	"strings"
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

	// "Home reads 8 課" — the main line lists eight lessons, and the side
	// branch's three are not among them.
	if got := len(main.Candidates); got != 8 {
		t.Errorf("the main line lists %d lessons, want 8: %+v", got, main.Candidates)
	}
	// "The side branch shows as three, hanging under L03."
	if len(main.Groups) != 1 {
		t.Fatalf("the main line holds %d child branches, want 1: %+v", len(main.Groups), main.Groups)
	}
	side := main.Groups[0]
	if side.Role != RoleLocal || !side.Container {
		t.Errorf("side branch = (role %v, container %t), want (local, true)", side.Role, side.Container)
	}
	if got := len(side.Candidates); got != 3 {
		t.Errorf("the side branch lists %d lessons, want 3: %+v", got, side.Candidates)
	}
	if side.AnchorTarget != "L03 研磨" {
		t.Errorf("the side branch hangs from %q, want %q", side.AnchorTarget, "L03 研磨")
	}
	if side.Name != "進階選修（卡住才讀）" {
		t.Errorf("side branch name = %q, want the container's text without its marker", side.Name)
	}

	// The routine block reads normally on the page but lists nothing for the
	// course; its rows are still recognized, which is why the branch is `none`
	// rather than structural.
	if got := len(routine.Candidates); got != 2 {
		t.Errorf("the routine block lists %d rows, want 2: %+v", got, routine.Candidates)
	}

	// The main line's order is the order it was written in, ordered-list
	// numbering included.
	wantMain := []string{
		"L01 器材認識", "L02 咖啡豆基礎", "L03 研磨", "L04 水溫",
		"L05 注水手法", "L06 比例與時間", "L07 品飲", "L08 常見問題排除",
	}
	if diff := cmp.Diff(wantMain, targets(main.Candidates)); diff != "" {
		t.Errorf("main line order mismatch (-want +got):\n%s", diff)
	}
	wantSide := []string{"磨豆機校正基礎", "粒徑分布判讀", "校正實作"}
	if diff := cmp.Diff(wantSide, targets(side.Candidates)); diff != "" {
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

	if diff := cmp.Diff(targets(ordered.Groups[0].Candidates), targets(unordered.Groups[0].Candidates)); diff != "" {
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
// test above passes for a parser that collects nothing at all.
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
		{name: "row with leading prose", body: "- 第一課 [[A]]\n", want: "A"},
		{name: "bracket run that is not a task", body: "- [x][[A]]\n", want: "A"},
		{name: "bracketed word that is not a task", body: "- [z] [[A]]\n", want: "A"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse("## P {sequence=primary}\n\n"+tt.body, 1)
			if diff := cmp.Diff([]string{tt.want}, collected(doc.Groups)); diff != "" {
				t.Errorf("Parse() mismatch for %q (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

// TestBranchStateComesFromCandidatesNotFromResolution holds the split the
// contract is built on. A branch that lists a row nobody can resolve is still a
// branch that lists rows, so it is unclassified once and stays unclassified —
// fixing the row never produces a second round of "this branch has no role".
func TestBranchStateComesFromCandidatesNotFromResolution(t *testing.T) {
	t.Parallel()
	// A row naming two notes is a candidate the consumer will refuse to turn
	// into an entry. The branch it sits in must still count as listing rows.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse(tt.body, 1)
			var found *Diagnostic
			for i := range doc.Diagnostics {
				if doc.Diagnostics[i].Rule == tt.wantRule {
					found = &doc.Diagnostics[i]
					break
				}
			}
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

func targets(cs []Candidate) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Target)
	}
	return out
}

// collected is every resolvable target the document lists, at any depth.
func collected(groups []Group) []string {
	var out []string
	for _, g := range groups {
		for _, c := range g.Candidates {
			if c.Target != "" {
				out = append(out, c.Target)
			}
		}
		out = append(out, collected(g.Groups)...)
	}
	return out
}

func hasRule(doc Document, rule string) bool {
	for _, d := range doc.Diagnostics {
		if d.Rule == rule {
			return true
		}
	}
	return false
}

var _ = strings.TrimSpace
