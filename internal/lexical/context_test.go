package lexical

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestLandingContextInsideChangedBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		query  string
		suffix string
	}{
		{name: "struck sentence", body: "~~加一格 buffer 就能取消工作。~~ 它只能暫存一個值。\n", query: "取消", suffix: "工作"},
		{name: "after a struck sentence", body: "~~撤回。~~取消工作。\n", query: "取消", suffix: "工作"},
		{name: "nfd before the struck sentence", body: "cafe\u0301cafe\u0301cafe\u0301。\n\n~~取消工作。~~\n", query: "取消", suffix: "工作"},
		{name: "nfd inside the context", body: "~~取消cafe\u0301工作。~~\n", query: "取消", suffix: "café工作"},
		{name: "trimmed leading space", body: "\n\n~~取消工作。~~\n\n", query: "取消", suffix: "工作"},
		{name: "ruby reading in the same block", body: "<ruby>今日<rt>きょう</rt></ruby>取消工作。\n", query: "取消"},
		{name: "rewritten wikilink in the same block", body: "[[來源]]取消工作。\n", query: "取消"},
		{name: "a consumed highlight", body: "==取消工作。==\n", query: "取消"},
		{name: "no certain boundary before a removed delimiter", body: "~~取消工作~~仍然阻塞。\n", query: "取消"},
		{name: "a suffix crossing a removed delimiter", body: "取消~~工作。~~\n", query: "取消"},
		{name: "nfd contraction cannot cover following markup", body: "~~e\u0301e\u0301e\u0301e\u0301e\u0301e\u0301取消工作~~。\n", query: "取消"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := DocumentFromNote(vault.Parse("Notes/context.md", []byte(tt.body)))
			idx := NewIndex([]Document{doc}, validArtifactPolicy(t))
			got := searchResults(t, idx, Parse(tt.query))
			if len(got) != 1 {
				t.Fatalf("Search(%q) = %v, want one result", tt.query, got)
			}
			if diff := cmp.Diff(tt.suffix, got[0].LandingSuffix); diff != "" {
				t.Errorf("Search(%q).LandingSuffix (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// TestHeadingHitNamesTheSectionOpening pins what a hit inside a heading
// names. The contents list repeats an unchanged heading's words, so a run
// beside the match comes from outside the heading: the section's first words
// after it, which the list's copy is never followed by, or else the previous
// block's last words, which it is never preceded by. A heading hit given
// neither keeps the directive any other heading hit has, and is not pinned
// here; TestGatedHeadingHitsKeepTheDirectiveMainEmits pins that directive
// where the gate is what withheld the context.
func TestHeadingHitNamesTheSectionOpening(t *testing.T) {
	t.Parallel()

	type landing struct{ Prefix, Landing, Bare, Suffix string }
	tests := []struct {
		name, body, query string
		want              landing
	}{
		{name: "cjk heading", body: "## 等待者應該放在哪裡？\n\n若把 `wg.Wait()` 與 `close(results)` 移回 `main`。\n", query: "等待", want: landing{Landing: "等待者應該放在哪裡？", Bare: "等待者應該放在哪裡？", Suffix: "若把 wg.Wait() 與"}},
		{name: "english heading", body: "Intro.\n\n## Where the inkwell waits\n\nThe shelf keeps it dry all winter.\n", query: "inkwell", want: landing{Landing: "inkwell waits", Bare: "inkwell waits", Suffix: "The shelf keeps"}},
		{name: "unspaced opening stops at a certain boundary", body: "## 等待者\n\n若把等待移回主程式之前會發生什麼事情呢如果一直寫下去，就這樣。\n", query: "等待者", want: landing{Landing: "等待者", Bare: "等待者", Suffix: "若把等待移回主程式之前會發生什麼事情呢如果一直寫下去"}},
		{name: "a heading follows, prose before", body: "## First\n\nIntro words here.\n\n## Where the inkwell waits\n\n## Next\n", query: "inkwell", want: landing{Prefix: "Intro words here.", Landing: "Where the inkwell waits", Bare: "Where the inkwell waits"}},
		{name: "a cjk heading follows, prose before", body: "## 前言\n\n先讀這段。\n\n## 等待者\n\n### 下一節\n", query: "等待", want: landing{Prefix: "先讀這段。", Landing: "等待者", Bare: "等待者"}},
		{name: "the title heading the page drops", body: "# heading\n\nOpening words here.\n", query: "heading", want: landing{Landing: "heading", Bare: "heading"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := DocumentFromNote(vault.Parse("Notes/heading.md", []byte(tt.body)))
			idx := NewIndex([]Document{doc}, validArtifactPolicy(t))
			got := searchResults(t, idx, Parse(tt.query))
			if len(got) != 1 {
				t.Fatalf("Search(%q) = %v, want one result", tt.query, got)
			}
			if diff := cmp.Diff(tt.want, landing{got[0].LandingPrefix, got[0].Landing, got[0].LandingBare, got[0].LandingSuffix}); diff != "" {
				t.Errorf("Search(%q) heading landing (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}
