package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A backslash in front of a wikilink is the author showing the syntax rather
// than writing a link — the CommonMark backslash escape — and the reading page
// prints it as the text it is, with nothing to report. The adjudicator reads
// the same bytes for the same vault, so a name written that way is not a
// citation here either: one product cannot red a link on a page that shows no
// link, and the same extraction feeds the list of notes citing a note, which
// would otherwise name a citation the citing page does not make.
func TestExtractedLinksSkipEscapedBrackets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a plain link is a citation", body: `[[Real]]`, want: []string{"Real"}},
		{name: "an escaped link is not", body: `\[[Ghost]]`, want: nil},
		{name: "an escaped embed is not", body: `\![[Phantom]]`, want: nil},
		{name: "a plain embed still is", body: `![[Real]]`, want: []string{"Real"}},
		{name: "an escaped backslash leaves the link", body: `\\[[Real]]`, want: []string{"Real"}},
		{name: "an odd run keeps the link escaped", body: `\\\[[Ghost]]`, want: nil},
		{name: "one escape does not cover the line", body: `see \[[Ghost]] then [[Real]]`, want: []string{"Real"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LinkTargets(tt.body)
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

// TestGapCheckboxItemKeepsItsMarker pins what the code does with a task-form
// gap row today: the checkbox marker stays part of the harvested name, so
// "- [ ] 甲" declares the literal name "[ ] 甲" — which no wikilink target
// ever normalizes to, meaning a checkbox row softens no broken link. Whether
// such a row should declare 甲 instead is an open product question; the pin
// makes any future answer a visible change rather than a silent drift.
func TestGapCheckboxItemKeepsItsMarker(t *testing.T) {
	t.Parallel()

	got := extractPlannedNames("## 缺口\n- [ ] 甲\n")
	if diff := cmp.Diff([]string{"[ ] 甲"}, got); diff != "" {
		t.Errorf("extractPlannedNames checkbox row mismatch (-want +got):\n%s", diff)
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

// A gap mark quoted in code is shown, not written: a code span inside a
// heading is not heading prose, so it opens no gap section.
func TestGapMarkInsideACodeSpanOpensNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []string
	}{
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
		{name: "an uppercase spelling is a resource, not a ref", body: "[doc](Concepts/a.MD)", want: nil},
		{name: "a mixed-case spelling is not a ref either", body: "[doc](Concepts/a.Md)", want: nil},
		{name: "a backticked lowercase path is checkable", body: "see `Concepts/a.md`", want: []string{"Concepts/a.md"}},
		{name: "a backticked uppercase spelling is not", body: "see `Concepts/a.MD`", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, ref := range extractPathRefs(tt.body, 1) {
				got = append(got, ref.target)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("extractPathRefs(%q) targets (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}
