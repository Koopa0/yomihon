package lexical

import (
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// TestALandingPrefersProseWhenTheSameWordsSitInAFence is the first half of
// the fence-eligibility lock: a result lands where a reader opens it, and a d2
// fence that happens to hold the same words must not take that place. The
// fence is written first so the raw earliest offset lands inside it; the prose
// has to win on purpose.
func TestALandingPrefersProseWhenTheSameWordsSitInAFence(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Both.md", []byte(""+
			"# Both\n\n"+
			"```d2\n"+
			"direction: right\n"+
			"Source: \"source\\nowns jobs close\"\n"+
			"```\n\n"+
			"The source owns jobs after the workers close.\n"))),
	}, validArtifactPolicy(t))

	got := soleResult(t, idx, `"owns jobs"`)
	if got.FromFence || got.Lead.Code {
		t.Errorf("FromFence = %v, Lead.Code = %v; the prose answered, so the row is not a fence hit", got.FromFence, got.Lead.Code)
	}
	if got.Landing != "owns jobs" || !strings.Contains(plain(got.Lead.Runs), "The source owns jobs after the workers close.") {
		t.Errorf("Landing = %q, Lead = %q, want both in the prose that holds the words", got.Landing, plain(got.Lead.Runs))
	}
}

// TestALandingKeepsAFenceWhenTheWordsLiveOnlyThere is the other half: a note
// whose only mention is inside a fence still lands on it rather than dropping
// the hit, and the row names it as source so those lines are not presented as
// a sentence the note wrote.
func TestALandingKeepsAFenceWhenTheWordsLiveOnlyThere(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		DocumentFromNote(vault.Parse("Notes/Fence only.md", []byte(""+
			"# Fence only\n\n"+
			"A paragraph about something else entirely.\n\n"+
			"```d2\n"+
			"direction: right\n"+
			"Source: \"source\\nowns jobs close\"\n"+
			"```\n"))),
	}, validArtifactPolicy(t))

	got := soleResult(t, idx, `"owns jobs"`)
	if !got.FromFence || !got.Lead.Code {
		t.Errorf("FromFence = %v, Lead.Code = %v; a fence-only hit must name its excerpt as source", got.FromFence, got.Lead.Code)
	}
	if got.Landing != "owns jobs" {
		t.Errorf("Landing = %q, want the fence words the query asked for", got.Landing)
	}
}

// TestRemapPlainOffsetsPreserveARepeatedBound locks the no-dedupe half
// of the one NFC remap: two fence spans that share a raw offset must
// stay two spans, or pairing a flattened [start, end, …] list shifts
// every later span.
func TestRemapPlainOffsetsPreserveARepeatedBound(t *testing.T) {
	t.Parallel()

	raw := "caf\u0065\u0301\n\nrest"
	off := len("caf\u0065\u0301")
	_, got := remapPlainOffsets(raw, nil, [][2]int{{0, off}, {off, len(raw)}})
	if len(got) != 2 {
		t.Fatalf("remapPlainOffsets fences = %v, want two spans sharing the café bound", got)
	}
	if got[0][1] != got[1][0] {
		t.Fatalf("shared bound mapped to %d and %d; the pair must stay aligned", got[0][1], got[1][0])
	}
	if got[0][1] == off {
		t.Fatalf("mapped offset %d is the raw offset; NFC contracted café 6→5", off)
	}
}

// TestAFenceIsFoundWhereTheFoldMovedIt holds the two remaps a fence's bounds
// pass through: an NFD prefix shrinks under NFC, and fullwidth ASCII narrowing
// and a CJK wrap shrink the folded copy, so neither the stored nor the folded
// fence sits at its source offset. A bound left unmapped classifies a phrase
// that lives only in the fence as prose.
func TestAFenceIsFoundWhereTheFoldMovedIt(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, needle string
		note         *vault.Note
	}{
		{name: "an NFD prefix", needle: "UNIQUE_FENCE_ONLY_PHRASE", note: vault.Parse("Notes/NFC fence.md", []byte(""+
			"# NFC fence\n\n"+
			strings.Repeat("e\u0301", 32)+"\n\n"+
			"```d2\n"+
			"UNIQUE_FENCE_ONLY_PHRASE\n"+
			"```\n\n"+
			"The workers close after the source.\n"))},
		{name: "a prefix the fold shrinks", needle: "UNIQUE_FOLD_FENCE_PHRASE", note: vault.Parse("Notes/Fold fence.md", []byte(""+
			"# Fold fence\n\n"+
			strings.Repeat("Ｇｏ", 20)+"の\n並行処理。\n\n"+
			"```d2\n"+
			"UNIQUE_FOLD_FENCE_PHRASE\n"+
			"```\n\n"+
			"The workers close after the source.\n"))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := NewIndex([]Document{DocumentFromNote(tt.note)}, validArtifactPolicy(t))
			got := soleResult(t, idx, tt.needle)
			if !got.FromFence || got.Landing != tt.needle {
				t.Errorf("FromFence = %v, Landing = %q; want the phrase found in the fence it lives in", got.FromFence, got.Landing)
			}
		})
	}
}

// TestFoldPlainMapsFencesTheWayTheSourceTableDoes holds the one-pass walk to
// the table: a fullwidth and CJK-wrapped prefix makes fold and source offsets
// diverge, so recording `at` would disagree.
func TestFoldPlainMapsFencesTheWayTheSourceTableDoes(t *testing.T) {
	t.Parallel()

	d := DocumentFromNote(vault.Parse("Notes/Fold oracle.md", []byte(""+
		"# Fold oracle\n\n"+
		strings.Repeat("Ｇｏ", 20)+"の\n並行処理。\n\n"+
		"```d2\n"+
		"UNIQUE_FOLD_ORACLE_PHRASE\n"+
		"```\n")))
	plain := vault.NormalizeNFC(d.PlainText)
	_, mapped := remapPlainOffsets(d.PlainText, nil, d.FenceRanges)
	_, got := foldPlain(plain, mapped)
	_, src := foldWithSourceOffsets(plain)
	want := make([][2]int, 0, len(mapped))
	for _, r := range mapped {
		lo := foldIndexOracle(src, r[0])
		hi := foldIndexOracle(src, r[1])
		if hi > lo {
			want = append(want, [2]int{lo, hi})
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("foldPlain() fences = %v, want the source-table mapping %v", got, want)
	}
	if len(mapped) != 1 || len(want) != 1 || want[0][0] == mapped[0][0] {
		t.Fatal("fold and source fence starts agree; the oracle is untested")
	}
}

func foldIndexOracle(srcOfFold []int, src int) int {
	for i, s := range srcOfFold {
		if s >= src {
			return i
		}
	}
	if len(srcOfFold) == 0 {
		return 0
	}
	return len(srcOfFold) - 1
}
