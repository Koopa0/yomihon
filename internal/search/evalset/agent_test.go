package evalset

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/agent"
	"github.com/koopa0/yomihon/internal/search/evalset/recording"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/search/semantic/observer"
	"github.com/koopa0/yomihon/internal/vault"
)

func wireValue[T any](t *testing.T, object map[string]any, key string) T {
	t.Helper()
	value, ok := object[key].(T)
	if !ok {
		t.Fatalf("wire field %q has type %T, want fixture type", key, object[key])
	}
	return value
}

func TestEvaluateUsesProductionRankingForFortyCaseFloor(t *testing.T) {
	t.Parallel()

	vectors := parseRecording(t, validEvaluationRecording(t))
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.HybridRecall.Numerator != 40 || report.HybridRecall.Denominator != 40 ||
		report.SemanticRecall.Numerator != 40 || report.SemanticRecall.Denominator != 40 {
		t.Errorf("Evaluate() report = %+v, want passed 40/40", report)
	}
}

func TestCommittedProviderVectorsMeetFortyCaseFloor(t *testing.T) {
	t.Parallel()

	vectors, err := recording.Committed()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.Dimension() != 1536 {
		t.Fatalf("Committed().Dimension() = %d, want 1536", vectors.Dimension())
	}
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed ||
		report.HybridRecall != (recallScore{Numerator: 40, Denominator: 40}) ||
		report.SemanticRecall != (recallScore{Numerator: 40, Denominator: 40}) {
		t.Errorf("Evaluate(committed 1536 vectors) report = %+v, want passed 40/40", report)
	}
}

func TestProductionObserverArtifactProjectsCommittedEvaluationQueries(t *testing.T) {
	t.Parallel()

	compatibility, err := semantic.QueryVectorCompatibilityIdentity(1536)
	if err != nil {
		t.Fatal(err)
	}
	const wantCompatibility = "0c6be8d840df073e343de8adf4e200a14f7ea364db6198ada907ea0079c78ede"
	if got := hex.EncodeToString(compatibility[:]); got != wantCompatibility {
		t.Fatalf("query compatibility identity = %s, want %s", got, wantCompatibility)
	}
	workload, err := observer.Committed(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := recording.Committed()
	if err != nil {
		t.Fatal(err)
	}
	evaluationQueries, err := committed.QueryVectors(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(evaluationQueries, workload.QueryVectors()); diff != "" {
		t.Errorf("production observer query projection mismatch (-evaluation +production):\n%s", diff)
	}
}

func TestEvaluateFailsContrastiveSiblingRankedAboveRequiredPositive(t *testing.T) {
	t.Parallel()

	wire := validEvaluationRecording(t)
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	first := &suite.Cases[0]
	contrastive := first.ContrastiveSiblings[0]
	pathIndex := make(map[string]int, len(suite.Corpus))
	for i := range suite.Corpus {
		pathIndex[suite.Corpus[i].RelPath] = i
	}
	query := wireValue[[]map[string]any](t, wire, "queries")[0]
	wireValue[[]float32](t, query, "vector")[pathIndex[contrastive]] = 2
	vectors := parseRecording(t, wire)
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatal(err)
	}
	got := report.Hybrid[0]
	if got.Retrieval.Recall.Numerator != got.Retrieval.Recall.Denominator ||
		!slices.Contains(got.Retrieval.MisrankedContrasts, contrastive) || got.Passed {
		t.Errorf("Evaluate() contrastive-order case = %+v, want full recall but failed contrast oracle", got)
	}
}

func TestEvaluateUsesPathAscendingTieAtRankFive(t *testing.T) {
	t.Parallel()

	wire := validEvaluationRecording(t)
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(suite.Corpus))
	for i := range suite.Corpus {
		paths[i] = suite.Corpus[i].RelPath
	}
	slices.Sort(paths)
	suite.Cases[0].RequiredPositives = []string{paths[4]}
	suite.Cases[0].AcceptablePositives = []string{paths[0]}
	suite.Cases[0].ContrastiveSiblings = []string{paths[5]}
	query := wireValue[[]map[string]any](t, wire, "queries")[0]
	queryVector := wireValue[[]float32](t, query, "vector")
	for i := range queryVector {
		queryVector[i] = 0
	}
	for i := range len(suite.Corpus) {
		queryVector[i] = 1
	}
	vectors := parseRecording(t, wire)
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatal(err)
	}
	got := report.Hybrid[0]
	if diff := slices.Compare(got.Retrieval.Top5, paths[:5]); diff != 0 || !got.Passed {
		t.Errorf("Evaluate() tied top five = %v, want %v and passed", got.Retrieval.Top5, paths[:5])
	}
}

func TestEvaluateUsesFinalProductionHybridPath(t *testing.T) {
	t.Parallel()

	vectors := parseRecording(t, validEvaluationRecording(t))
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.HybridRecall.Numerator != 40 || report.HybridRecall.Denominator != 40 ||
		report.SemanticRecall.Numerator != 40 || report.SemanticRecall.Denominator != 40 {
		t.Errorf("Evaluate() final hybrid report = %+v, want passed 40/40", report)
	}
	var lexicalCases int
	var fusionChanged bool
	for _, result := range report.Hybrid {
		if !result.LexicalComplete {
			t.Errorf("Evaluate() case %s dropped a lexical result", result.Retrieval.ID)
		}
		if result.LexicalCount > 0 {
			lexicalCases++
		}
	}
	for i := range report.Hybrid {
		if !slices.Equal(report.Hybrid[i].Retrieval.Top5, report.Semantic[i].Top5) {
			fusionChanged = true
		}
	}
	if lexicalCases == 0 || !fusionChanged {
		t.Errorf("Evaluate() lexical cases/change = %d/%t, want lexical evidence that changes final ranking", lexicalCases, fusionChanged)
	}
}

func TestEvaluateFinalOracleDependsOnLexicalHeadRank(t *testing.T) {
	t.Parallel()

	wire := validEvaluationRecording(t)
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(suite.Corpus))
	pathIndex := make(map[string]int, len(suite.Corpus))
	for i := range suite.Corpus {
		paths[i] = suite.Corpus[i].RelPath
		pathIndex[suite.Corpus[i].RelPath] = i
	}
	slices.Sort(paths)
	testCase := &suite.Cases[0]
	testCase.Query = "合成評測"
	testCase.AcceptablePositives = []string{"Writing/lessons/golang/__synthetic_eval_context.md"}
	submissionHash, err := semantic.QuerySubmissionHash(testCase.Query)
	if err != nil {
		t.Fatal(err)
	}
	queryWire := wireValue[[]map[string]any](t, wire, "queries")[0]
	queryWire["submission_hash"] = hex.EncodeToString(submissionHash[:])
	testCase.RequiredPositives = []string{paths[0]}
	testCase.ContrastiveSiblings = []string{paths[len(paths)-1]}
	vector := make([]float32, len(wireValue[[]float32](t, queryWire, "vector")))
	semanticOrder := append([]string{}, paths[1:8]...)
	semanticOrder = append(semanticOrder, paths[0])
	semanticOrder = append(semanticOrder, paths[8:len(paths)-1]...)
	semanticOrder = append(semanticOrder, paths[len(paths)-1])
	for rank, relPath := range semanticOrder {
		vector[pathIndex[relPath]] = float32(len(paths) - rank)
	}
	queryWire["vector"] = vector
	vectors := parseRecording(t, wire)
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Hybrid[0].Passed {
		t.Errorf("Evaluate() lexical-head oracle = %+v, want passed with shipped lexical rank", report.Hybrid[0])
	}
	if report.Semantic[0].Passed {
		t.Errorf("Evaluate() semantic oracle = %+v, want failed without the lexical head rank", report.Semantic[0])
	}
	if report.Passed {
		t.Error("Evaluate() aggregate passed even though the semantic-channel floor failed")
	}
}

func TestEvaluateRejectsStaleFilterProjection(t *testing.T) {
	t.Parallel()

	vectors := parseRecording(t, validEvaluationRecording(t))
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	testCase := &suite.Cases[32]
	testCase.AllowedPaths = slices.Delete(testCase.AllowedPaths, 1, 2)
	_, err = evaluateFixture(t, &suite, vectors)
	if !errors.Is(err, errInvalidSuite) {
		t.Errorf("Evaluate(stale filter projection) error = %v, want errInvalidSuite", err)
	}
}

func TestScoreFinalCaseRejectsFilterViolationOutsideTopFive(t *testing.T) {
	t.Parallel()

	const (
		required    = "required.md"
		contrastive = "contrastive.md"
		disallowed  = "disallowed.md"
	)
	testCase := &evalCase{
		ID:                  "q01",
		RequiredPositives:   []string{required},
		ContrastiveSiblings: []string{contrastive},
		RecallDenominator:   1,
	}
	fused := []agent.FusedHit{
		{RelPath: required},
		{RelPath: contrastive},
		{RelPath: "third.md"},
		{RelPath: "fourth.md"},
		{RelPath: "fifth.md"},
		{RelPath: disallowed},
	}
	allowed := map[string]struct{}{
		required:    {},
		contrastive: {},
		"third.md":  {},
		"fourth.md": {},
		"fifth.md":  {},
	}
	got := scoreFinalCase(testCase, fused, nil, allowed)
	if got.FiltersHard || got.Passed {
		t.Errorf("scoreFinalCase(disallowed result at rank 6) = %+v, want hard-filter failure", got)
	}
}

func evaluateFixture(t *testing.T, suite *evalSuite, vectors *recording.Vectors) (evaluationReport, error) {
	t.Helper()

	artifact, privacy := fixturePolicies(t)
	return evaluate(t.Context(), suite, vectors, artifact, privacy)
}

func fixturePolicies(t *testing.T) (schema.ArtifactPolicy, schema.PrivacyPolicy) {
	t.Helper()

	fixturePath := filepath.Join("..", "..", "schema", "testdata", "contract.toml")
	data, err := os.ReadFile(fixturePath) // #nosec G304 -- fixed repository fixture selected by the test
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	data = append(data, []byte("\n[privacy]\nnever_egress_dirs = [\"Private\"]\n")...)
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil { // #nosec G703 -- path is beneath t.TempDir and has no input-derived component
		t.Fatal(writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	artifact := contract.ArtifactPolicy()
	privacy := contract.PrivacyPolicy()
	if !artifact.Available() || !privacy.Available() {
		t.Fatal("fixture artifact policy is unavailable")
	}
	return artifact, privacy
}

func validEvaluationRecording(t *testing.T) map[string]any {
	t.Helper()

	wire := validMathRecording(t)
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	const dimension = 128 // Unit-test ranking space only; never serialized as a provider fixture.
	pathIndex := make(map[string]int, len(suite.Corpus))
	chunks := make([]map[string]any, 0, len(suite.Corpus)*2)
	for noteIndex := range suite.Corpus {
		note := &suite.Corpus[noteIndex]
		pathIndex[note.RelPath] = noteIndex
		parsed := vault.Parse(note.RelPath, note.Data)
		chunked := semantic.ChunkNote(parsed.Title(), parsed.Body, 1024)
		if len(chunked.Failures) != 0 || len(chunked.Chunks) == 0 {
			t.Fatalf("ChunkDocument(%q) = %d chunks, %d failures", note.RelPath, len(chunked.Chunks), len(chunked.Failures))
		}
		for _, chunk := range chunked.Chunks {
			vector := make([]float32, dimension)
			vector[noteIndex] = 1
			submittedHash := sha256.Sum256([]byte(chunk.Submitted))
			chunks = append(chunks, map[string]any{
				"rel_path":       note.RelPath,
				"ordinal":        chunk.Ordinal,
				"submitted_hash": hex.EncodeToString(submittedHash[:]),
				"vector":         vector,
			})
		}
	}
	queries := make([]map[string]any, len(suite.Cases))
	for caseIndex := range suite.Cases {
		testCase := &suite.Cases[caseIndex]
		vector := make([]float32, dimension)
		for i := range len(suite.Corpus) {
			vector[i] = -0.1
		}
		vector[pathIndex[testCase.RequiredPositives[0]]] = 1
		for _, relPath := range testCase.ContrastiveSiblings {
			vector[pathIndex[relPath]] = -1
		}
		submissionHash, err := semantic.QuerySubmissionHash(search.Parse(testCase.Query).BareText())
		if err != nil {
			t.Fatal(err)
		}
		queries[caseIndex] = map[string]any{
			"id":              testCase.ID,
			"submission_hash": hex.EncodeToString(submissionHash[:]),
			"vector":          vector,
		}
	}
	wire["queries"] = queries
	wire["chunks"] = chunks
	return wire
}
