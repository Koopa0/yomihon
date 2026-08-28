package agent

import (
	"cmp"
	"slices"

	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
)

const (
	rrfConstant  = 60
	channelDepth = 50
)

// semanticHit joins a ranked note to current-vault display evidence. The
// durable cache never stores these display fields.
type semanticHit struct {
	semantic.Result

	Title   string
	Status  string
	Snippet string
	Heading string
}

// FusedHit is the channel-neutral result shape projected onto the CLI wire.
type FusedHit struct {
	Rank    int
	RelPath string
	Title   string
	Status  string
	Snippet string
	Heading string
	// LexicalRank is the hit's 1-indexed position in the lexical channel's
	// own ranking. Zero means the hit did not appear in that channel at all
	// — never a real rank, since ranks start at one.
	LexicalRank int
	// SemanticRank is the hit's 1-indexed position in the semantic channel's
	// own ranking. Zero means the hit did not appear in that channel at all
	// — never a real rank, since ranks start at one.
	SemanticRank int
}

type fusionCandidate struct {
	result FusedHit
	score  float64
}

// fuse applies RRF k=60 to each channel's first 50 notes, then appends every
// lexical match outside that fused block in its shipped order. limit is a final
// presentation bound, never the channel depth.
func fuse(lexical []search.Result, semanticHits []semanticHit, limit int) []FusedHit {
	if limit <= 0 {
		return nil
	}
	candidates := make(map[string]*fusionCandidate, min(len(lexical), channelDepth)+min(len(semanticHits), channelDepth))
	for i, result := range lexical[:min(len(lexical), channelDepth)] {
		candidate := &fusionCandidate{
			result: FusedHit{
				RelPath:     result.RelPath,
				Title:       result.Title,
				Status:      result.Status,
				Snippet:     result.Snippet,
				LexicalRank: i + 1,
			},
			score: reciprocalRank(i + 1),
		}
		candidates[result.RelPath] = candidate
	}
	for i, hit := range semanticHits[:min(len(semanticHits), channelDepth)] {
		candidate, found := candidates[hit.RelPath]
		if !found {
			candidate = &fusionCandidate{result: FusedHit{
				RelPath: hit.RelPath,
				Title:   hit.Title,
				Status:  hit.Status,
			}}
			candidates[hit.RelPath] = candidate
		}
		candidate.score += reciprocalRank(i + 1)
		candidate.result.SemanticRank = i + 1
		// A semantic channel contribution always carries its best current chunk's
		// evidence, including when the lexical channel also matched.
		candidate.result.Snippet = hit.Snippet
		candidate.result.Heading = hit.Heading
	}

	fused := make([]fusionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		fused = append(fused, *candidate)
	}
	slices.SortFunc(fused, func(a, b fusionCandidate) int {
		if byScore := cmp.Compare(b.score, a.score); byScore != 0 {
			return byScore
		}
		return cmp.Compare(a.result.RelPath, b.result.RelPath)
	})

	ordered := make([]FusedHit, 0, len(fused)+max(0, len(lexical)-channelDepth))
	included := make(map[string]int, len(fused))
	for i := range fused {
		ordered = append(ordered, fused[i].result)
		included[fused[i].result.RelPath] = len(ordered) - 1
	}
	if len(lexical) > channelDepth {
		for i, result := range lexical[channelDepth:] {
			lexicalRank := channelDepth + i + 1
			if position, alreadyIncluded := included[result.RelPath]; alreadyIncluded {
				ordered[position].LexicalRank = lexicalRank
				continue
			}
			ordered = append(ordered, FusedHit{
				RelPath:     result.RelPath,
				Title:       result.Title,
				Status:      result.Status,
				Snippet:     result.Snippet,
				LexicalRank: lexicalRank,
			})
		}
	}
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	for i := range ordered {
		ordered[i].Rank = i + 1
	}
	return ordered
}

func reciprocalRank(rank int) float64 {
	return 1 / float64(rrfConstant+rank)
}
