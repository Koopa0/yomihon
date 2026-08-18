package nav

import (
	"path/filepath"
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
		"## Warnings\n- [[Duplicate Name]]\n- [[Card]]\n- [[No Such Note]]",
	} {
		f.Add(seed)
	}

	// A small fixed vault instead of an empty one: the parser keeps only
	// uniquely resolved governed entries, so an empty index prunes every
	// branch and the assertions below never execute. These notes make each
	// resolution outcome reachable — unique targets the seeds link to, a
	// duplicate basename pair for ambiguity, and a note inside the fixture
	// contract's non-instance directory.
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{Path: "Effective Go.md"},
		{Path: "Lessons/L01 はじめまして.md"},
		{Path: "Concepts/Duplicate Name.md"},
		{Path: "Sources/Duplicate Name.md"},
		{Path: "System/templates/Card.md"},
	}, nil)
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		f.Fatalf("schema.LoadFile: %v", err)
	}
	policy := contract.ArtifactPolicy()

	// The harness proves its own reach before fuzzing: if the fixed vault
	// stopped resolving the seed links, every tree would be empty again and
	// the tree assertions would pass over anything.
	smoke := parseBranches("## part\n- [[Effective Go]]", idx, nil, policy)
	if len(smoke) != 1 || len(smoke[0].Entries) != 1 || smoke[0].Entries[0].Kind != EntryResolved {
		f.Fatalf("harness self-check: parseBranches() = %#v, want one branch holding one resolved entry", smoke)
	}

	f.Fuzz(func(t *testing.T, body string) {
		const maxInput = 32 << 10
		if len(body) > maxInput {
			return
		}

		first := parseBranches(body, idx, nil, policy)
		second := parseBranches(body, idx, nil, policy)
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
			if entry.Kind != EntryResolved || entry.RelPath == "" || len(entry.Candidates) != 0 {
				t.Fatalf("kept entry = %#v, want uniquely resolved with a path — this parser drops warning kinds", entry)
			}
		}
		subNodes, subEntries := checkBranchTree(t, branch.Subbranches, branch.Level)
		nodes += subNodes
		entries += subEntries
	}
	return nodes, entries
}
