package agent

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
)

func TestFuseRRFUsesTopFiftyAndDeterministicPathTies(t *testing.T) {
	t.Parallel()

	lexical := []search.Result{
		{RelPath: "a.md", Title: "A", Snippet: "lexical a"},
		{RelPath: "b.md", Title: "B", Snippet: "lexical b"},
	}
	for i := 3; i <= 51; i++ {
		lexical = append(lexical, search.Result{RelPath: fmt.Sprintf("z%02d.md", i), Title: fmt.Sprintf("Z%d", i)})
	}
	semanticHits := []semanticHit{
		{Rank: semantic.Rank{RelPath: "c.md", ChunkOrdinal: 3}, Title: "C", Snippet: "semantic c", Heading: "C head"},
		{Rank: semantic.Rank{RelPath: "b.md", ChunkOrdinal: 1}, Title: "B", Snippet: "semantic b", Heading: "B head"},
	}
	got := fuse(lexical, semanticHits, 100)
	if len(got) != 52 {
		t.Fatalf("Fuse result count = %d, want lexical completeness plus one semantic-only note", len(got))
	}
	if paths := []string{got[0].RelPath, got[1].RelPath, got[2].RelPath}; !cmp.Equal(paths, []string{"b.md", "a.md", "c.md"}) {
		t.Errorf("fused head = %v, want agreement first then rel-path tie", paths)
	}
	if got[0].Snippet != "semantic b" || got[0].Heading != "B head" {
		t.Errorf("hybrid evidence = %+v, want best semantic chunk evidence", got[0])
	}
	if got[0].LexicalRank != 2 || got[0].SemanticRank != 2 {
		t.Errorf("channel ranks = %d/%d, want lexical 2 and semantic 2", got[0].LexicalRank, got[0].SemanticRank)
	}
	last := got[len(got)-1]
	if last.RelPath != "z51.md" || last.LexicalRank != 51 {
		t.Errorf("lexical tail = %+v, want rank-51 hit appended unchanged", last)
	}
	for i, result := range got {
		if result.Rank != i+1 {
			t.Errorf("result %d rank = %d, want dense %d", i, result.Rank, i+1)
		}
	}
}

func TestFuseDoesNotApplyTheChannelDepthAsAResultLimit(t *testing.T) {
	t.Parallel()

	lexical := make([]search.Result, 80)
	for i := range lexical {
		lexical[i] = search.Result{RelPath: fmt.Sprintf("%03d.md", i), Title: fmt.Sprintf("N%d", i)}
	}
	got := fuse(lexical, nil, 80)
	if len(got) != 80 {
		t.Fatalf("Fuse returned %d lexical results, want 80", len(got))
	}
	for i, result := range got {
		if result.RelPath != lexical[i].RelPath || result.LexicalRank != i+1 {
			t.Fatalf("result %d = %+v, want lexical order/rank preserved", i, result)
		}
	}
}

func TestFuseHonorsFinalLimitAfterBuildingCompleteOrder(t *testing.T) {
	t.Parallel()

	lexical := []search.Result{{RelPath: "a.md"}, {RelPath: "b.md"}, {RelPath: "c.md"}}
	got := fuse(lexical, nil, 2)
	if len(got) != 2 || got[1].RelPath != "b.md" {
		t.Errorf("Fuse limit = %+v", got)
	}
	if got := fuse(lexical, nil, 0); got != nil {
		t.Errorf("Fuse zero limit = %+v, want nil", got)
	}
}

func TestFuseCarriesLexicalRankBeyondFusionDepthOnSemanticHeadHit(t *testing.T) {
	t.Parallel()

	lexical := make([]search.Result, 51)
	for i := range lexical {
		lexical[i] = search.Result{RelPath: fmt.Sprintf("%03d.md", i)}
	}
	semanticHits := []semanticHit{{Rank: semantic.Rank{RelPath: lexical[50].RelPath}}}
	got := fuse(lexical, semanticHits, 100)
	if len(got) != len(lexical) {
		t.Fatalf("Fuse result count = %d, want %d unique lexical notes", len(got), len(lexical))
	}
	var joined Result
	for _, result := range got {
		if result.RelPath == lexical[50].RelPath {
			joined = result
			break
		}
	}
	if joined.RelPath == "" {
		t.Fatalf("semantic head did not retain lexical rank-51 note")
	}
	if joined.LexicalRank != 51 || joined.SemanticRank != 1 {
		t.Errorf("channel ranks = %d/%d, want lexical 51 and semantic 1", joined.LexicalRank, joined.SemanticRank)
	}
}
