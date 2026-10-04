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
