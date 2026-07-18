package evalset

import (
	"errors"
	"testing"
)

func TestScorePathsAllowsContrastiveSiblingBelowRequiredPositive(t *testing.T) {
	t.Parallel()

	const (
		required    = "Writing/lessons/golang/__synthetic_eval_context.md"
		contrastive = "Writing/lessons/golang/__synthetic_eval_interface.md"
	)
	testCase := &evalCase{
		ID:                  "q01",
		RequiredPositives:   []string{required},
		ContrastiveSiblings: []string{contrastive},
		RecallDenominator:   1,
	}
	got := scorePaths(testCase, []string{required, contrastive})
	if !got.Passed {
		t.Errorf("scorePaths(required then contrastive).Passed = false, want true; result = %+v", got)
	}
}

func TestScorePathsRejectsUnrelatedRankOne(t *testing.T) {
	t.Parallel()

	const (
		required    = "Writing/lessons/golang/__synthetic_eval_context.md"
		contrastive = "Writing/lessons/golang/__synthetic_eval_interface.md"
		unrelated   = "Writing/lessons/golang/__synthetic_eval_memory.md"
	)
	testCase := &evalCase{
		ID:                  "q01",
		RequiredPositives:   []string{required},
		ContrastiveSiblings: []string{contrastive},
		RecallDenominator:   1,
	}
	got := scorePaths(testCase, []string{unrelated, required, contrastive})
	if got.Passed {
		t.Errorf("scorePaths(unrelated rank one).Passed = true, want false; result = %+v", got)
	}
}

func TestScorePathsAllowsAcceptablePositiveAtRankOne(t *testing.T) {
	t.Parallel()

	const (
		required    = "Writing/lessons/golang/__synthetic_eval_context.md"
		acceptable  = "Writing/lessons/golang/__synthetic_eval_concurrency.md"
		contrastive = "Writing/lessons/golang/__synthetic_eval_interface.md"
	)
	testCase := &evalCase{
		ID:                  "q01",
		RequiredPositives:   []string{required},
		AcceptablePositives: []string{acceptable},
		ContrastiveSiblings: []string{contrastive},
		RecallDenominator:   1,
	}
	got := scorePaths(testCase, []string{acceptable, required, contrastive})
	if !got.Passed {
		t.Errorf("scorePaths(acceptable rank one).Passed = false, want true; result = %+v", got)
	}
}

func TestScorePathsRejectsContrastiveSiblingBetweenRequiredPositives(t *testing.T) {
	t.Parallel()

	const (
		firstRequired  = "Writing/lessons/golang/__synthetic_eval_context.md"
		secondRequired = "Writing/lessons/golang/__synthetic_eval_concurrency.md"
		contrastive    = "Writing/lessons/golang/__synthetic_eval_interface.md"
	)
	testCase := &evalCase{
		ID:                  "q01",
		RequiredPositives:   []string{firstRequired, secondRequired},
		ContrastiveSiblings: []string{contrastive},
		RecallDenominator:   2,
	}
	got := scorePaths(testCase, []string{firstRequired, contrastive, secondRequired})
	if got.Passed || len(got.MisrankedContrasts) != 1 || got.MisrankedContrasts[0] != contrastive {
		t.Errorf("scorePaths(contrast between required positives) = %+v, want one misranked contrast and failed", got)
	}
}

func TestEvaluateSemanticRejectsRecordingMissingCurrentChunk(t *testing.T) {
	t.Parallel()

	wire := validEvaluationRecording(t)
	wire["chunks"] = wireValue[[]map[string]any](t, wire, "chunks")[1:]
	vectors := parseRecording(t, wire)
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluateSemantic(&suite, vectors)
	if !errors.Is(err, errRecordingMismatch) {
		t.Errorf("EvaluateSemantic(missing chunk) error = %v, want errRecordingMismatch", err)
	}
}

func TestEvaluateSemanticRejectsChangedCorpusBytes(t *testing.T) {
	t.Parallel()

	vectors := parseRecording(t, validEvaluationRecording(t))
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	suite.Corpus[0].Data = []byte(string(suite.Corpus[0].Data) + "\n同じ節に合成の追記を置く。\n")
	_, err = evaluateSemantic(&suite, vectors)
	if !errors.Is(err, errRecordingMismatch) {
		t.Errorf("EvaluateSemantic(changed submitted bytes) error = %v, want errRecordingMismatch", err)
	}
}
