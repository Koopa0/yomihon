package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/egress"
)

// TestLinkTargetsSkipWhatThePageDoesNotLink holds the citations this face
// reads to the ones the page links where a comment, an escape or code meets
// another construct: a backslash escape shows the syntax, a comment left open
// in a quote or a list item ends with its block, and a comment mark inside a
// fence or a code span opens nothing.
func TestLinkTargetsSkipWhatThePageDoesNotLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "an escaped link is not a citation", body: `\[[Ghost]]`},
		{name: "an escaped embed is not either", body: `\![[Phantom]]`},
		{name: "an escaped backslash leaves the link", body: `\\[[Real]]`, want: []string{"Real"}},
		{name: "an odd run keeps the link escaped", body: `\\\[[Ghost]]`},
		{name: "one escape does not cover the line", body: `see \[[Ghost]] then [[Real]]`, want: []string{"Real"}},
		{name: "an escaped comment opener opens nothing", body: `\<!-- [[Shown]] -->`, want: []string{"Shown"}},
		{name: "a comment opener in a code span opens nothing", body: "`<!--` [[Shown]]", want: []string{"Shown"}},
		{name: "a comment mark inside a fence opens nothing", body: "[[Shown]]\n```\n%%\n```\n[[Also]]\n", want: []string{"Shown", "Also"}},
		{name: "a comment left open in a quote ends with the quote", body: "[[Shown]]\n> <!-- [[Hidden]]\n> secret\n\n[[Also]]", want: []string{"Shown", "Also"}},
		{name: "a comment left open in a list item hides the item", body: "- Item\n  <!-- [[Hidden]]\n  secret\n\n[[Also]]", want: []string{"Also"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := egress.Targets(LinkTargets(tt.body))
			if len(got) == 0 {
				got = nil
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("LinkTargets(%q) mismatch (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

// The gap-ledger grammar below is the vault's forward-writing convention: a
// heading marked as a gap opens a section whose list items declare concept
// names the corpus still owes, and an inline planned mark turns a line's
// wikilinks into the same declarations. Both faces consume the harvest through
// Planned; these tests pin the harvesting itself.

func TestInlinePlannedMarksHarvestTheLineTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "a marked line contributes its wikilink targets",
			body: "接下來 [[一致性雜湊]] 與 [[向量時鐘]] 待整理。\n",
			want: []string{"一致性雜湊", "向量時鐘"},
		},
		{
			name: "the next-lesson mark works inline",
			body: "下一課:[[て形]]\n",
			want: []string{"て形"},
		},
		{
			name: "an unmarked line contributes nothing",
			body: "接下來讀 [[一致性雜湊]] 與 [[向量時鐘]]。\n",
			want: nil,
		},
		{
			name: "a marked line inside a fence contributes nothing",
			body: "```\n[[一致性雜湊]] 待整理\n```\n",
			want: nil,
		},
		{
			name: "an escaped link beside a mark contributes nothing",
			body: "待建 \\[[Ghost]]\n",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPlannedNames(tt.body)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractPlannedNames(%q) mismatch (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

func TestGapListItemGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "dash star and plus each open an item",
			body: "## 缺口\n\n- 甲\n* 乙\n+ 丙\n",
			want: []string{"甲", "乙", "丙"},
		},
		{
			name: "an indented continuation extends the item before it",
			body: "## 缺口\n\n- 分散式\n  雜湊環\n",
			want: []string{"分散式 雜湊環"},
		},
		{
			name: "an ordered item contributes nothing and closes the item before it",
			body: "## 缺口\n\n- 甲\n1. 乙\n- 丙\n",
			want: []string{"甲", "丙"},
		},
		{
			name: "a following heading closes the section",
			body: "## 缺口\n- 甲\n## 別的\n- 乙\n",
			want: []string{"甲"},
		},
		{
			name: "an item still open at the end of the body is flushed",
			body: "## 缺口\n- 甲",
			want: []string{"甲"},
		},
		{
			name: "a list outside any gap heading contributes nothing",
			body: "## 筆記\n- 甲\n",
			want: nil,
		},
		{
			name: "a code-span mark does not make a gap heading",
			body: "## 工具 `缺口` 對照\n- 甲\n",
			want: nil,
		},
		{
			name: "a prose mark beside a code span still does",
			body: "## 缺口 `code`\n- 甲\n",
			want: []string{"甲"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPlannedNames(tt.body)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractPlannedNames(%q) mismatch (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

// TestGapRowNamesALinkOrATaskByItsName asserts a gap row written as a
// wikilink or a task declares the name it carries, not its markup.
func TestGapRowNamesALinkOrATaskByItsName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a wikilink row", body: "## 缺口\n- [[Ghost]]\n", want: []string{"Ghost"}},
		{name: "a wikilink row with a display alias", body: "## 缺口\n- [[Ghost|鬼]]\n", want: []string{"Ghost"}},
		{name: "two wikilinks on one row", body: "## 缺口\n- [[甲]]、[[乙]]\n", want: []string{"甲", "乙"}},
		{name: "an open task", body: "## 缺口\n- [ ] 甲\n", want: []string{"甲"}},
		{name: "a done task", body: "## 缺口\n- [x] 甲\n", want: []string{"甲"}},
		{name: "a task naming a wikilink", body: "## 缺口\n- [ ] [[Ghost]]\n", want: []string{"Ghost"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPlannedNames(tt.body)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractPlannedNames(%q) mismatch (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

func TestGapEntrySplitsEnumeratorsAndDropsAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item string
		want []string
	}{
		{
			name: "the enumeration comma splits names",
			item: "甲、乙、丙",
			want: []string{"甲", "乙", "丙"},
		},
		{
			name: "a spaced slash splits names",
			item: "Raft / Paxos",
			want: []string{"Raft", "Paxos"},
		},
		{
			name: "a bare slash stays one name",
			item: "TCP/IP",
			want: []string{"TCP/IP"},
		},
		{
			name: "a fullwidth annotation is dropped",
			item: "一致性雜湊（暫定）",
			want: []string{"一致性雜湊"},
		},
		{
			name: "an ascii annotation is dropped",
			item: "Vector Clocks (draft)",
			want: []string{"Vector Clocks"},
		},
		{
			name: "a nested annotation is dropped whole",
			item: "甲（外（內）層）、乙",
			want: []string{"甲", "乙"},
		},
		{
			name: "enumerators and slashes compose",
			item: "甲、Raft / Paxos",
			want: []string{"甲", "Raft", "Paxos"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPlannedNames("## 缺口\n- " + tt.item + "\n")
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gap entry %q mismatch (-want +got):\n%s", tt.item, diff)
			}
		})
	}
}

// A gap mark in a heading counts where a word starts. The same characters
// buried after another ideograph are ordinary prose — a topic about gaps is not
// an owed section — so the broken links under such a heading keep their
// warning. What follows the mark is free: authors coin headings like 缺口帳.
func TestHeadingGapMarkMatchesAtWordStartOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		heading string
		want    bool
	}{
		{name: "mark inside a topic phrase", heading: "7. 陷阱、產品化缺口與事故對照", want: false},
		{name: "mark after an ideograph in a clause", heading: "3. 核心觀念：不同的排序空間，決定誰被缺口卡住", want: false},
		{name: "mark after an ideograph in a longer word", heading: "尚待補證的書目資訊", want: false},
		{name: "no mark at all", heading: "完成", want: false},
		{name: "mark begins a coined compound", heading: "缺口帳", want: true},
		{name: "mark after a space, followed by an ideograph", heading: "L4 待補範圍", want: true},
		{name: "mark begins the heading, followed by an ideograph", heading: "待補與更新", want: true},
		{name: "bracketed mark", heading: "【待補】清單", want: true},
		{name: "mark after a full-width colon", heading: "L4：待寫", want: true},
		{name: "first occurrence embedded, later one bounded", heading: "產品化缺口與事故 缺口", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := len(extractPlannedNames("## "+tt.heading+"\n- 甲\n")) > 0
			if got != tt.want {
				t.Errorf("heading %q opens a gap section = %v, want %v", tt.heading, got, tt.want)
			}
		})
	}
}

// TestPathRefsSplitNoteFromResourceOnTheExactExtension holds the judge to the
// one Markdown test every other reader uses: the path ends in ".md", those
// exact bytes. An uppercase spelling names a resource, and a resource is not a
// checkable note reference — the resolver and the reading page already read it
// that way, so a judge that counted it would report on a file no other face
// calls a note.
func TestPathRefsSplitNoteFromResourceOnTheExactExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a lowercase link ref is checkable", body: "[doc](Concepts/a.md)", want: []string{"Concepts/a.md"}},
		// A dotfile whose whole name is the extension ends in ".md" like any
		// other note path, and the one shared predicate every face uses asks
		// nothing more of a name — so the judge accepts it as a note ref
		// rather than keeping a private stricter test of its own.
		{name: "a dotfile named exactly .md is a note ref, by the shared predicate", body: "[doc](Concepts/.md)", want: []string{"Concepts/.md"}},
		{name: "an uppercase spelling is a resource, not a ref", body: "[doc](Concepts/a.MD)", want: nil},
		{name: "a mixed-case spelling is not a ref either", body: "[doc](Concepts/a.Md)", want: nil},
		{name: "a backticked lowercase path is checkable", body: "see `Concepts/a.md`", want: []string{"Concepts/a.md"}},
		{name: "a backticked uppercase spelling is not", body: "see `Concepts/a.MD`", want: nil},
		// A code span that opens with a command name quotes a command line, not a
		// note path. The folder-with-a-space case is the accepted cost of that
		// reading: it is no longer judged, where the same span without a space
		// in its first word still is.
		{name: "a backticked git command line is not a ref", body: "`git show main:docs/design.md`", want: nil},
		{name: "a backticked cat command line is not a ref", body: "`cat docs/design.md`", want: nil},
		{name: "a backticked path with no whitespace is still a ref", body: "`Notes/Missing.md`", want: []string{"Notes/Missing.md"}},
		{name: "a backticked path with a space after its first folder is still a ref", body: "`Notes/Reading list/a.md`", want: []string{"Notes/Reading list/a.md"}},
		{name: "a backticked path whose first folder name has a space is not a ref", body: "`Reading list/a.md`", want: nil},
		{name: "a link keeps its written path and fragment without the query", body: "[missing](Nothing%20here.md?at=1#part)", want: []string{"Nothing%20here.md#part"}},
		{name: "an encoded backticked path is not a ref", body: "`Notes/Missing%20note.md`", want: nil},
		{name: "a malformed escape is not a ref", body: "[invalid](Bad%2.md)", want: nil},
		{name: "a remote link is not a ref", body: "[remote](https://host/Remote.md)", want: nil},
		{name: "a resource link is not a ref", body: "[resource](picture.png)", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, ref := range extractPathRefs(tt.body, 1) {
				target := ref.target
				if ref.fragment != "" {
					target += "#" + ref.fragment
				}
				got = append(got, target)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractPathRefs(%q) targets (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

// TestThePlannedSetIsOnlyEnteredThroughTheHarvest holds the one guarantee the
// set exists to make. Its names are folded the way the resolver folds a link,
// so asking about a name answers what resolution would have answered had the
// note been written — and both faces have to reach the same verdict about a
// link that resolves to nothing, or the reader learns to distrust whichever one
// cried wolf.
//
// The set is carried by value through both of them, so a caller reaching one
// before the harvest has run holds the zero set. That is a corpus which
// declared nothing, and reading it must not be a way to bring down the face
// drawing a broken link.
func TestThePlannedSetIsOnlyEnteredThroughTheHarvest(t *testing.T) {
	t.Parallel()

	var unharvested Planned
	if unharvested.Has("一致性雜湊") {
		t.Error("a set nobody harvested reports a name as planned; the zero value means a corpus that declared nothing")
	}

	set := NewPlanned("接下來 [[Consistent Hashing]] 待整理。\n", nil)
	if !set.Has("consistent hashing") {
		t.Error("NewPlanned did not fold the harvested name the way the resolver folds a link to it")
	}
	if set.Has("一致性雜湊") {
		t.Error("the set claims a name nothing declared")
	}
}

// TestMergedPlannedSetsHoldEveryNotesNames asserts the union of the notes' own
// sets answers for every name any of them declared, and invents none.
func TestMergedPlannedSetsHoldEveryNotesNames(t *testing.T) {
	t.Parallel()
	merged := MergePlanned(
		NewPlanned("接下來 [[Consistent Hashing]] 待整理。\n", nil),
		NewPlanned("接下來 [[Bloom Filter]] 待整理。\n", nil),
		Planned{},
	)
	for name, want := range map[string]bool{"consistent hashing": true, "bloom filter": true, "skip list": false} {
		if got := merged.Has(name); got != want {
			t.Errorf("merged.Has(%q) = %t, want %t", name, got, want)
		}
	}
}
