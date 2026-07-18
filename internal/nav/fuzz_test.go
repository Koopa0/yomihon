package nav

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

func FuzzParseBranches(f *testing.F) {
	for _, seed := range []string{
		"## part | Part | 部分\n- [[Effective Go]]",
		"## 第一部\n### 第一課\n* [[L01 はじめまして|Lesson 1]]",
		"###not-a-heading\n- [[Target]]",
		"## Tasks\n- [ ] [[Unchecked]]\n- [x] [[Checked]]",
		"## Ordered\n1. [[Not an entry]]",
		"## Anchors\n- [[#same-file]]",
		"######## Deep\n+ [[深いリンク]]",
		"## Controls\x00\n- [[目標\u0085名]]",
	} {
		f.Add(seed)
	}

	idx := graph.BuildFromNotes(nil, nil)
	policy := schema.ArtifactPolicy{}
	f.Fuzz(func(t *testing.T, body string) {
		const maxInput = 32 << 10
		if len(body) > maxInput {
			return
		}

		first := parseBranches(body, idx, nil, policy, true)
		second := parseBranches(body, idx, nil, policy, true)
		if diff := cmp.Diff(first, second); diff != "" {
			t.Fatalf("parseBranches() is not deterministic (-first +second):\n%s", diff)
		}

		maxLines := 1 + strings.Count(body, "\n")
		nodes, entries := checkBranchTree(t, first, 1)
		if nodes > maxLines {
			t.Fatalf("parseBranches() produced %d branches from %d lines", nodes, maxLines)
		}
		if entries > maxLines {
			t.Fatalf("parseBranches() produced %d entries from %d lines", entries, maxLines)
		}
	})
}

func checkBranchTree(t *testing.T, branches []Branch, parentLevel int) (nodes, entries int) {
	t.Helper()
	for _, branch := range branches {
		if branch.Level < 2 || branch.Level <= parentLevel {
			t.Fatalf("branch %q has level %d beneath level %d", branch.Heading, branch.Level, parentLevel)
		}
		if len(branch.Entries) == 0 && len(branch.Subbranches) == 0 {
			t.Fatalf("branch %q survived pruning without entries", branch.Heading)
		}
		nodes++
		entries += len(branch.Entries)
		for _, entry := range branch.Entries {
			if entry.Kind != EntryUnresolved || entry.RelPath != "" || len(entry.Candidates) != 0 {
				t.Fatalf("entry from empty resolver = %#v, want unresolved without path or candidates", entry)
			}
		}
		subNodes, subEntries := checkBranchTree(t, branch.Subbranches, branch.Level)
		nodes += subNodes
		entries += subEntries
	}
	return nodes, entries
}
