package evalset

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/agent"
	"github.com/koopa0/yomihon/internal/search/evalset/recording"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

// evaluate runs one validated synthetic suite through the shipped parser,
// lexical index, semantic index, and agent fusion path. It remains private
// until a committed recording and its exact synthetic policy can be bound to
// a production command as one inseparable input.
func evaluate(
	ctx context.Context,
	suite *evalSuite,
	vectors *recording.Vectors,
	artifactPolicy schema.ArtifactPolicy,
	privacyPolicy schema.PrivacyPolicy,
) (evaluationReport, error) {
	if err := validateSuite(suite); err != nil {
		return evaluationReport{}, err
	}
	if vectors == nil {
		return evaluationReport{}, recording.ErrAbsent
	}
	if !artifactPolicy.Available() || !privacyPolicy.Available() {
		return evaluationReport{}, fmt.Errorf("%w: corpus policy unavailable", errInvalidSuite)
	}
	if err := requireFullIdentity(vectors, artifactPolicy, privacyPolicy); err != nil {
		return evaluationReport{}, err
	}
	ready, err := newRecordedSearch(suite, vectors)
	if err != nil {
		return evaluationReport{}, err
	}
	lexicalDocuments := lexicalDocs(suite)
	lexicalIndex := search.NewIndex(lexicalDocuments, artifactPolicy)
	snapshot, err := agent.NewSnapshot(
		ctx,
		snapshotNotes(suite),
		artifactPolicy,
		privacyPolicy,
		vectors.ChunkTokenCap(),
	)
	if err != nil {
		return evaluationReport{}, fmt.Errorf("bind agent snapshot: %w", err)
	}
	queries, err := recordedQueries(suite, vectors)
	if err != nil {
		return evaluationReport{}, err
	}
	outcome, err := evaluateCases(ctx, suite, snapshot, ready, queries, lexicalIndex)
	if err != nil {
		return evaluationReport{}, err
	}
	semanticCases, err := evaluateSemantic(suite, vectors)
	if err != nil {
		return evaluationReport{}, err
	}
	outcome.Semantic = semanticCases
	for _, result := range semanticCases {
		outcome.SemanticRecall.Numerator += result.Recall.Numerator
		outcome.SemanticRecall.Denominator += result.Recall.Denominator
		outcome.Passed = outcome.Passed && result.Passed
	}
	return outcome, nil
}

func evaluateCases(
	ctx context.Context,
	suite *evalSuite,
	snapshot *agent.Snapshot,
	ready *recordedSearch,
	queries []recording.Query,
	lexicalIndex *search.Index,
) (evaluationReport, error) {
	outcome := evaluationReport{Hybrid: make([]hybridResult, 0, len(suite.Cases)), Passed: true}
	for position := range suite.Cases {
		if contextErr := ctx.Err(); contextErr != nil {
			return evaluationReport{}, contextErr
		}
		testCase := &suite.Cases[position]
		query := search.Parse(testCase.Query)
		lexical, searchErr := lexicalIndex.Search(query)
		if searchErr != nil {
			return evaluationReport{}, fmt.Errorf("evaluate lexical case %s: %w", testCase.ID, searchErr)
		}
		allowed, allowedErr := lexicalIndex.AllowedPaths(query)
		if allowedErr != nil {
			return evaluationReport{}, fmt.Errorf("evaluate filters case %s: %w", testCase.ID, allowedErr)
		}
		if testCase.Class == classFilterMixed && !samePaths(testCase.AllowedPaths, allowed) {
			return evaluationReport{}, fmt.Errorf("%w: filter projection changed for case %s", errInvalidSuite, testCase.ID)
		}
		ready.query = queries[position].Vector
		ready.expectedText = query.BareText()
		searchAction, bindErr := agent.NewSearch(snapshot, ready)
		if bindErr != nil {
			return evaluationReport{}, fmt.Errorf("evaluate hybrid case %s: %w", testCase.ID, bindErr)
		}
		fused, runErr := searchAction.Run(ctx, query, len(suite.Corpus))
		if runErr != nil {
			return evaluationReport{}, fmt.Errorf("evaluate hybrid case %s: %w", testCase.ID, runErr)
		}
		result := scoreFinalCase(testCase, fused, lexical, allowed)
		outcome.Hybrid = append(outcome.Hybrid, result)
		outcome.HybridRecall.Numerator += result.Retrieval.Recall.Numerator
		outcome.HybridRecall.Denominator += result.Retrieval.Recall.Denominator
		outcome.Passed = outcome.Passed && result.Passed
	}
	return outcome, nil
}

func requireFullIdentity(
	vectors *recording.Vectors,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
) error {
	policy, ok := schema.CorpusPolicyFingerprint(artifact, privacy)
	if !ok {
		return fmt.Errorf("%w: corpus policy unavailable", errInvalidSuite)
	}
	full, err := semantic.FullCacheIdentity(semantic.IdentityConfig{
		Dimension:               vectors.Dimension(),
		ChunkTokenCap:           syntheticChunkTokenCap,
		VaultRoot:               syntheticVaultRoot,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		return fmt.Errorf("%w: derive current full identity: %w", recording.ErrInvalid, err)
	}
	if vectors.ChunkTokenCap() != syntheticChunkTokenCap {
		return recording.ErrIdentityMismatch
	}
	return vectors.RequireFullIdentity(full)
}

func snapshotNotes(suite *evalSuite) []agent.SnapshotNote {
	notes := make([]agent.SnapshotNote, len(suite.Corpus))
	for i := range suite.Corpus {
		notes[i] = agent.SnapshotNote{RelPath: suite.Corpus[i].RelPath, Data: suite.Corpus[i].Data}
	}
	return notes
}

func samePaths(expected []string, actual map[string]struct{}) bool {
	if len(expected) != len(actual) {
		return false
	}
	for _, relPath := range expected {
		if _, ok := actual[relPath]; !ok {
			return false
		}
	}
	return true
}

type recordedSearch struct {
	fingerprint  [sha256.Size]byte
	index        *semantic.Index
	query        []float32
	expectedText string
}

func (r *recordedSearch) CorpusFingerprint() [sha256.Size]byte {
	return r.fingerprint
}

func (r *recordedSearch) Search(
	ctx context.Context,
	text string,
	allowed map[string]struct{},
	depth int,
) ([]semantic.Rank, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if text != r.expectedText || text == "" {
		return nil, errors.New("eval query text differs from the parsed bare text")
	}
	return r.index.TopNotes(slices.Clone(r.query), allowed, depth)
}

func newRecordedSearch(suite *evalSuite, vectors *recording.Vectors) (*recordedSearch, error) {
	rows, err := indexRows(suite, vectors)
	if err != nil {
		return nil, err
	}
	index, err := semantic.NewIndex(rows, vectors.Dimension())
	if err != nil {
		return nil, fmt.Errorf("%w: build production index: %w", recording.ErrInvalid, err)
	}
	fingerprint, err := semantic.CorpusFingerprint(rows)
	if err != nil {
		return nil, fmt.Errorf("%w: fingerprint current corpus: %w", errRecordingMismatch, err)
	}
	return &recordedSearch{
		fingerprint: fingerprint,
		index:       index,
	}, nil
}

func lexicalDocs(suite *evalSuite) []search.Document {
	docs := make([]search.Document, 0, len(suite.Corpus))
	for position := range suite.Corpus {
		note := &suite.Corpus[position]
		parsed := vault.Parse(note.RelPath, note.Data)
		docs = append(docs, search.DocumentFromNote(parsed))
	}
	return docs
}

func scoreFinalCase(
	testCase *evalCase,
	fused []agent.Result,
	lexical []search.Result,
	allowed map[string]struct{},
) hybridResult {
	depth := min(recallDepth, len(fused))
	top5 := make([]string, depth)
	all := make(map[string]struct{}, len(fused))
	filtersHard := true
	for position := range fused {
		result := &fused[position]
		all[result.RelPath] = struct{}{}
		if _, ok := allowed[result.RelPath]; !ok {
			filtersHard = false
		}
		if position < depth {
			top5[position] = result.RelPath
		}
	}
	lexicalComplete := true
	for position := range lexical {
		if _, ok := all[lexical[position].RelPath]; !ok {
			lexicalComplete = false
		}
	}
	retrieval := scorePaths(testCase, top5)
	return hybridResult{
		Retrieval:       retrieval,
		LexicalCount:    len(lexical),
		LexicalComplete: lexicalComplete,
		FiltersHard:     filtersHard,
		Passed:          retrieval.Passed && lexicalComplete && filtersHard,
	}
}
