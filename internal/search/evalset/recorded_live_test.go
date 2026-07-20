package evalset

import (
	"os"
	"testing"

	"github.com/koopa0/yomihon/internal/search/evalset/recording"
)

const evaluationRecordingEnv = "YOMIHON_EVAL_RECORDING"

// TestRecordedProviderVectors runs the fixed forty-case oracle over one local
// synthetic recording. It opens no provider and accepts no raw query input.
func TestRecordedProviderVectors(t *testing.T) {
	name := os.Getenv(evaluationRecordingEnv)
	if name == "" {
		t.Skip("set YOMIHON_EVAL_RECORDING to evaluate a local synthetic recording")
	}
	vectors, err := recording.Load(name)
	if err != nil {
		t.Fatalf("load recorded vectors: %v", err)
	}
	suite, err := loadSuite()
	if err != nil {
		t.Fatalf("load synthetic suite: %v", err)
	}
	report, err := evaluateFixture(t, &suite, vectors)
	if err != nil {
		t.Fatalf("evaluate recorded vectors: %v", err)
	}
	for i := range report.Hybrid {
		if !report.Hybrid[i].Passed || !report.Semantic[i].Passed {
			t.Logf(
				"%s hybrid=%+v semantic=%+v",
				report.Hybrid[i].Retrieval.ID,
				report.Hybrid[i],
				report.Semantic[i],
			)
		}
	}
	t.Logf(
		"dimension=%d hybrid_recall=%d/%d semantic_recall=%d/%d passed=%t",
		vectors.Dimension(),
		report.HybridRecall.Numerator,
		report.HybridRecall.Denominator,
		report.SemanticRecall.Numerator,
		report.SemanticRecall.Denominator,
		report.Passed,
	)
	if !report.Passed {
		t.Fail()
	}
}
