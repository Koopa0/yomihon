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
		{name: "ordinary heading", body: "## 等待者應該放在哪裡？\n", query: "等待", suffix: "者應該放在哪裡"},
		{name: "struck sentence", body: "~~加一格 buffer 就能取消工作。~~ 它只能暫存一個值。\n", query: "取消", suffix: "工作"},
		{name: "after a struck sentence", body: "~~撤回。~~取消工作。\n", query: "取消", suffix: "工作"},
		{name: "nfd before the heading", body: "cafe\u0301cafe\u0301cafe\u0301。\n\n## 等待者應該放在哪裡？\n", query: "等待", suffix: "者應該放在哪裡"},
		{name: "nfd inside the context", body: "~~取消cafe\u0301工作。~~\n", query: "取消", suffix: "café工作"},
		{name: "trimmed leading space", body: "\n\n~~取消工作。~~\n\n", query: "取消", suffix: "工作"},
		{name: "heading with a consumed role", body: "## 等待者 {sequence=primary}\n", query: "等待"},
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
// names. The contents list repeats an unchanged heading's words, so the
// stretch runs to the heading's end and the run after it is the section's
// first words, which the list's copy is never followed by. Where the next
// block is not reproduced as written — another heading above all — there is
// no such run, and the hit keeps the terms any other block gives it.
func TestHeadingHitNamesTheSectionOpening(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, body, query string
		bare, suffix      string
	}{
		{name: "cjk heading", body: "## 等待者應該放在哪裡？\n\n若把 `wg.Wait()` 與 `close(results)` 移回 `main`。\n", query: "等待", bare: "等待者應該放在哪裡？", suffix: "若把 wg.Wait() 與"},
		{name: "english heading", body: "Intro.\n\n## Where the inkwell waits\n\nThe shelf keeps it dry all winter.\n", query: "inkwell", bare: "inkwell waits", suffix: "The shelf keeps"},
		{name: "unspaced opening stops at a certain boundary", body: "## 等待者\n\n若把等待移回主程式之前會發生什麼事情呢如果一直寫下去，就這樣。\n", query: "等待者", bare: "等待者", suffix: "若把等待移回主程式之前會發生什麼事情呢如果一直寫下去"},
		{name: "a heading follows", body: "## 等待者\n\n### 下一節\n\n正文。\n", query: "等待", bare: "等待", suffix: "者"},
		{name: "the next block is changed by the page", body: "## 等待者\n\n~~加一格~~ 它。\n", query: "等待", bare: "等待", suffix: "者"},
		{name: "nothing follows", body: "## 等待者\n", query: "等待", bare: "等待", suffix: "者"},
		{name: "a changed heading", body: "## 等待者 {sequence=primary}\n\n正文。\n", query: "等待", bare: "等待"},
		{name: "the title heading the page drops", body: "# heading\n\nOpening words here.\n", query: "heading", bare: "heading"},
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
			if diff := cmp.Diff(struct{ Bare, Suffix string }{tt.bare, tt.suffix}, struct{ Bare, Suffix string }{got[0].LandingBare, got[0].LandingSuffix}); diff != "" {
				t.Errorf("Search(%q) heading landing (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}
