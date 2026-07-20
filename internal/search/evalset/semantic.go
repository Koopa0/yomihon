package evalset

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"

	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/evalset/recording"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

const recallDepth = 5

// errRecordingMismatch means recorded vectors do not cover the current
// committed synthetic suite exactly.
var errRecordingMismatch = errors.New("semantic eval recording does not match suite")

// recallScore is one recall fraction whose denominator is fixed by the suite.
type recallScore struct {
	Numerator   int
	Denominator int
}

// retrievalResult is one content-free recall@5 and contrast-order verdict.
type retrievalResult struct {
	ID                 string
	Top5               []string
	MissingPositives   []string
	MisrankedContrasts []string
	RankOnePositive    bool
	Recall             recallScore
	Passed             bool
}

// hybridResult adds the lexical-preservation and filter invariants that only
// the fused production path can prove.
type hybridResult struct {
	Retrieval       retrievalResult
	LexicalCount    int
	LexicalComplete bool
	FiltersHard     bool
	Passed          bool
}

// evaluationReport is the aggregate regression floor. Passed requires both the semantic
// channel and final hybrid path to satisfy every case.
type evaluationReport struct {
	Hybrid         []hybridResult
	Semantic       []retrievalResult
	HybridRecall   recallScore
	SemanticRecall recallScore
	Passed         bool
}

// evaluateSemantic replays recorded vectors through semantic.Index, the
// production exact-cosine/max-aggregation implementation.
func evaluateSemantic(suite *evalSuite, vectors *recording.Vectors) ([]retrievalResult, error) {
	if err := validateSuite(suite); err != nil {
		return nil, err
	}
	if vectors == nil {
		return nil, recording.ErrAbsent
	}
	rows, err := indexRows(suite, vectors)
	if err != nil {
		return nil, err
	}
	index, err := semantic.NewIndex(rows, vectors.Dimension())
	if err != nil {
		return nil, fmt.Errorf("%w: build production index: %w", recording.ErrInvalid, err)
	}
	queries, err := recordedQueries(suite, vectors)
	if err != nil {
		return nil, err
	}

	results := make([]retrievalResult, 0, len(suite.Cases))
	for position := range suite.Cases {
		testCase := &suite.Cases[position]
		query := &queries[position]
		ranked, rankErr := index.TopNotes(query.Vector, allowedSet(testCase.AllowedPaths), recallDepth)
		if rankErr != nil {
			return nil, fmt.Errorf("%w: rank case %s: %w", recording.ErrInvalid, testCase.ID, rankErr)
		}
		results = append(results, scoreCase(testCase, ranked))
	}
	return results, nil
}

func recordedQueries(suite *evalSuite, vectors *recording.Vectors) ([]recording.Query, error) {
	queries := vectors.Queries()
	if len(queries) != len(suite.Cases) {
		return nil, fmt.Errorf("%w: query count changed", errRecordingMismatch)
	}
	for i := range suite.Cases {
		testCase := &suite.Cases[i]
		bareText := search.Parse(testCase.Query).BareText()
		submissionHash, err := semantic.QuerySubmissionHash(bareText)
		if err != nil {
			return nil, fmt.Errorf("%w: case %s is not text-bearing", errInvalidSuite, testCase.ID)
		}
		if queries[i].ID != testCase.ID || queries[i].SubmissionHash != submissionHash {
			return nil, fmt.Errorf("%w: query text changed for case %s", errRecordingMismatch, testCase.ID)
		}
	}
	return queries, nil
}

type vectorKey struct {
	path    string
	ordinal uint32
}

func indexRows(suite *evalSuite, vectors *recording.Vectors) ([]semantic.ChunkVector, error) {
	type recordedVector struct {
		submittedHash [sha256.Size]byte
		vector        []float32
	}
	chunks := vectors.Chunks()
	recorded := make(map[vectorKey]recordedVector, len(chunks))
	for position := range chunks {
		chunk := &chunks[position]
		recorded[vectorKey{path: chunk.RelPath, ordinal: chunk.Ordinal}] = recordedVector{
			submittedHash: chunk.SubmittedHash,
			vector:        chunk.Vector,
		}
	}
	rows := make([]semantic.ChunkVector, 0, len(chunks))
	for notePosition := range suite.Corpus {
		note := &suite.Corpus[notePosition]
		parsed := vault.Parse(note.RelPath, note.Data)
		chunked := semantic.ChunkNote(parsed.Title(), parsed.Body, vectors.ChunkTokenCap())
		if len(chunked.Failures) != 0 || len(chunked.Chunks) == 0 {
			return nil, fmt.Errorf("%w: current synthetic corpus cannot be chunked", errRecordingMismatch)
		}
		noteHash := sha256.Sum256(note.Data)
		for chunkPosition := range chunked.Chunks {
			chunk := &chunked.Chunks[chunkPosition]
			key := vectorKey{path: note.RelPath, ordinal: uint32(chunk.Ordinal)} // #nosec G115 -- ordinal is a non-negative slice index.
			recordedRow, exists := recorded[key]
			if !exists {
				return nil, fmt.Errorf("%w: missing current chunk", errRecordingMismatch)
			}
			submitted := []byte(chunk.Submitted)
			submittedHash := sha256.Sum256(submitted)
			if submittedHash != recordedRow.submittedHash {
				return nil, fmt.Errorf("%w: submitted bytes changed", errRecordingMismatch)
			}
			corpusChunk := semantic.CorpusChunk{
				RelPath:       note.RelPath,
				NoteHash:      noteHash,
				Ordinal:       key.ordinal,
				Submitted:     submitted,
				SubmittedHash: submittedHash,
			}
			row, err := corpusChunk.Complete(semantic.EmbeddingResult{Vector: recordedRow.vector})
			if err != nil {
				return nil, fmt.Errorf("%w: bind current chunk: %w", errRecordingMismatch, err)
			}
			rows = append(rows, row)
			delete(recorded, key)
		}
	}
	if len(recorded) != 0 || len(rows) != len(chunks) {
		return nil, fmt.Errorf("%w: recording contains stale chunks", errRecordingMismatch)
	}
	return rows, nil
}

func allowedSet(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(paths))
	for _, relPath := range paths {
		allowed[relPath] = struct{}{}
	}
	return allowed
}

func scoreCase(testCase *evalCase, ranked []semantic.Rank) retrievalResult {
	paths := make([]string, 0, len(ranked))
	for _, rank := range ranked {
		paths = append(paths, rank.RelPath)
	}
	return scorePaths(testCase, paths)
}

func scorePaths(testCase *evalCase, paths []string) retrievalResult {
	result := retrievalResult{
		ID:   testCase.ID,
		Top5: slices.Clone(paths),
		Recall: recallScore{
			Denominator: testCase.RecallDenominator,
		},
	}
	for _, required := range testCase.RequiredPositives {
		if slices.Contains(result.Top5, required) {
			result.Recall.Numerator++
		} else {
			result.MissingPositives = append(result.MissingPositives, required)
		}
	}
	result.RankOnePositive = len(result.Top5) > 0 &&
		(slices.Contains(testCase.RequiredPositives, result.Top5[0]) ||
			slices.Contains(testCase.AcceptablePositives, result.Top5[0]))
	for _, contrastive := range testCase.ContrastiveSiblings {
		contrastRank := slices.Index(result.Top5, contrastive)
		if contrastRank < 0 {
			continue
		}
		for _, required := range testCase.RequiredPositives {
			requiredRank := slices.Index(result.Top5, required)
			if requiredRank < 0 || contrastRank < requiredRank {
				result.MisrankedContrasts = append(result.MisrankedContrasts, contrastive)
				break
			}
		}
	}
	result.Passed = result.RankOnePositive &&
		len(result.MissingPositives) == 0 &&
		len(result.MisrankedContrasts) == 0
	return result
}
