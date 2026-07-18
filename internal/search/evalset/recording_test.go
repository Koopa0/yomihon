package evalset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/evalset/recording"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestEvaluateRejectsSelfAttestedFullIdentity(t *testing.T) {
	t.Parallel()

	wire := validEvaluationRecording(t)
	wire["full_cache_identity"] = strings.Repeat("1", sha256.Size*2)
	vectors := parseRecording(t, wire)
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluateFixture(t, &suite, vectors)
	if !errors.Is(err, recording.ErrIdentityMismatch) {
		t.Errorf("Evaluate(self-attested identity) error = %v, want ErrIdentityMismatch", err)
	}
}

func TestEvaluateRejectsQueryTextWithStaleVector(t *testing.T) {
	t.Parallel()

	vectors := parseRecording(t, validEvaluationRecording(t))
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	suite.Cases[0].Query += " 改寫"
	_, err = evaluateFixture(t, &suite, vectors)
	if !errors.Is(err, errRecordingMismatch) {
		t.Errorf("Evaluate(stale query vector) error = %v, want errRecordingMismatch", err)
	}
}

func validMathRecording(t *testing.T) map[string]any {
	t.Helper()

	const dimension = 128 // Unit-test math space only; never a provider configuration choice.
	artifact, privacy := fixturePolicies(t)
	policy, ok := schema.CorpusPolicyFingerprint(artifact, privacy)
	if !ok {
		t.Fatal("fixture corpus policy fingerprint is unavailable")
	}
	full, err := semantic.FullCacheIdentity(semantic.IdentityConfig{
		Dimension:               dimension,
		ChunkTokenCap:           syntheticChunkTokenCap,
		VaultRoot:               syntheticVaultRoot,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	compatibility, err := semantic.QueryVectorCompatibilityIdentity(dimension)
	if err != nil {
		t.Fatal(err)
	}
	suite, err := loadSuite()
	if err != nil {
		t.Fatal(err)
	}
	queries := make([]map[string]any, len(suite.Cases))
	for i := range suite.Cases {
		testCase := &suite.Cases[i]
		vector := make([]float32, dimension)
		vector[i%dimension] = 1
		submissionHash, hashErr := semantic.QuerySubmissionHash(search.Parse(testCase.Query).BareText())
		if hashErr != nil {
			t.Fatal(hashErr)
		}
		queries[i] = map[string]any{
			"id":              testCase.ID,
			"submission_hash": hex.EncodeToString(submissionHash[:]),
			"vector":          vector,
		}
	}
	chunks := make([]map[string]any, len(suite.Corpus))
	for i, note := range suite.Corpus {
		vector := make([]float32, dimension)
		vector[i%dimension] = 1
		parsed := vault.Parse(note.RelPath, note.Data)
		chunked := semantic.ChunkNote(parsed.Title(), parsed.Body, syntheticChunkTokenCap)
		if len(chunked.Failures) != 0 || len(chunked.Chunks) == 0 {
			t.Fatalf("ChunkNote(%q) = %d chunks, %d failures", note.RelPath, len(chunked.Chunks), len(chunked.Failures))
		}
		submittedHash := sha256.Sum256([]byte(chunked.Chunks[0].Submitted))
		chunks[i] = map[string]any{
			"rel_path":       note.RelPath,
			"ordinal":        0,
			"submitted_hash": hex.EncodeToString(submittedHash[:]),
			"vector":         vector,
		}
	}
	return map[string]any{
		"format_version":                      2,
		"dimension":                           dimension,
		"chunk_token_cap":                     syntheticChunkTokenCap,
		"full_cache_identity":                 hex.EncodeToString(full[:]),
		"query_vector_compatibility_identity": hex.EncodeToString(compatibility[:]),
		"queries":                             queries,
		"chunks":                              chunks,
	}
}

func parseRecording(t *testing.T, wire map[string]any) *recording.Vectors {
	t.Helper()

	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := recording.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return vectors
}
