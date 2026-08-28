package agent

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
)

const semanticSnippetBytes = 200

// ErrInvalidSearch reports an incomplete action or a query that cannot be run.
var ErrInvalidSearch = errors.New("agent search action is invalid")

// SemanticSearch is the action-scoped capability consumed by agent search.
// It is defined by the consumer: the provider, credential, and store remain
// private to semantic.Indexer and cannot leak into fusion.
type SemanticSearch interface {
	CorpusFingerprint() [sha256.Size]byte
	Search(ctx context.Context, query string, allowedPaths map[string]struct{}, depth int) ([]semantic.Result, error)
}

// Search binds one snapshot to one semantic capability. The capability is
// single-use: it answers exactly the query it was bound for, so a caller
// with more than one query to send constructs a fresh capability (and a
// fresh Search) per query rather than reusing either one across queries.
// Construct it with NewSearch before any query can run.
type Search struct {
	snapshot *Snapshot
	semantic SemanticSearch
	evidence map[evidenceKey]*semantic.CorpusChunk
}

// NewSearch validates the snapshot and generation evidence before returning
// an action capable of sending a query.
func NewSearch(snapshot *Snapshot, semanticSearch SemanticSearch) (*Search, error) {
	if snapshot == nil || snapshot.lexical == nil || semanticSearch == nil {
		return nil, ErrInvalidSearch
	}
	snapshotEvidence, err := semanticEvidence(snapshot.semantic)
	if err != nil {
		return nil, err
	}
	if semanticSearch.CorpusFingerprint() != snapshot.semantic.Fingerprint {
		return nil, ErrEvidenceMismatch
	}
	return &Search{snapshot: snapshot, semantic: semanticSearch, evidence: snapshotEvidence}, nil
}

// Run performs the action's one query send, applies the snapshot's hard path
// filters before semantic scoring, joins local evidence, and fuses both ranked
// channels.
func (s *Search) Run(ctx context.Context, query search.Query, limit int) ([]FusedHit, error) {
	if s == nil || s.snapshot == nil || s.snapshot.lexical == nil || s.semantic == nil ||
		query.BareText() == "" || limit <= 0 {
		return nil, ErrInvalidSearch
	}
	lexical, err := s.snapshot.lexical.Search(query)
	if err != nil {
		return nil, err
	}
	allowedPaths, err := s.snapshot.lexical.AllowedPaths(query)
	if err != nil {
		return nil, err
	}
	ranked, err := s.semantic.Search(ctx, query.BareText(), allowedPaths, channelDepth)
	if err != nil {
		return nil, err
	}
	hits, err := joinSemanticEvidence(ranked, s.evidence, allowedPaths)
	if err != nil {
		return nil, err
	}
	return fuse(lexical, hits, limit), nil
}

func joinSemanticEvidence(
	ranked []semantic.Result,
	evidence map[evidenceKey]*semantic.CorpusChunk,
	allowedPaths map[string]struct{},
) ([]semanticHit, error) {
	hits := make([]semanticHit, 0, len(ranked))
	for i := range ranked {
		result := ranked[i]
		if allowedPaths != nil {
			if _, allowed := allowedPaths[result.RelPath]; !allowed {
				return nil, ErrEvidenceMismatch
			}
		}
		chunk, ok := evidence[evidenceKey{relPath: result.RelPath, ordinal: result.ChunkOrdinal}]
		if !ok || chunk.Title == "" {
			return nil, ErrEvidenceMismatch
		}
		snippet := semanticSnippet(chunk.Body)
		if snippet == "" {
			return nil, ErrEvidenceMismatch
		}
		hits = append(hits, semanticHit{
			Result:  result,
			Title:   chunk.Title,
			Status:  chunk.Status,
			Snippet: snippet,
			Heading: deepestHeading(chunk.Headings),
		})
	}
	return hits, nil
}

func semanticSnippet(body string) string {
	text := strings.Join(strings.Fields(body), " ")
	if len(text) <= semanticSnippetBytes {
		return text
	}
	end := semanticSnippetBytes
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end] + "…"
}

func deepestHeading(headings []string) string {
	for i := range slices.Backward(headings) {
		if headings[i] != "" {
			return headings[i]
		}
	}
	return ""
}
