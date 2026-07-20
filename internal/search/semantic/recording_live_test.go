package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/evalset/recording"
	"github.com/koopa0/yomihon/internal/vault"
)

const (
	recordingOutputEnv    = "YOMIHON_RECORDING_OUTPUT"
	recordingDimensionEnv = "YOMIHON_RECORDING_DIMENSION"
	recordingQueryOrigin  = "generated-synthetic-v1"
)

type recordingManifest struct {
	FormatVersion      int              `json:"format_version"`
	SyntheticVaultRoot string           `json:"synthetic_vault_root"`
	ChunkTokenCap      int              `json:"chunk_token_cap"`
	Cases              []recordingQuery `json:"cases"`
}

type recordingQuery struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
	Query  string `json:"query"`
}

type recordingWire struct {
	FormatVersion         int                  `json:"format_version"`
	Dimension             int                  `json:"dimension"`
	ChunkTokenCap         int                  `json:"chunk_token_cap"`
	FullIdentity          string               `json:"full_cache_identity"`
	CompatibilityIdentity string               `json:"query_vector_compatibility_identity"`
	Queries               []recordingQueryWire `json:"queries"`
	Chunks                []recordingChunkWire `json:"chunks"`
}

type recordingQueryWire struct {
	ID             string    `json:"id"`
	SubmissionHash string    `json:"submission_hash"`
	Vector         []float32 `json:"vector"`
}

type recordingChunkWire struct {
	RelPath       string    `json:"rel_path"`
	Ordinal       uint32    `json:"ordinal"`
	SubmittedHash string    `json:"submitted_hash"`
	Vector        []float32 `json:"vector"`
}

// TestCaptureSyntheticRecording embeds only the fixed synthetic corpus and
// queries committed under evalset/testdata. The test accepts no text or vault
// path from its environment; only the output filename and candidate dimension
// are variable.
func TestCaptureSyntheticRecording(t *testing.T) {
	output := os.Getenv(recordingOutputEnv)
	if output == "" {
		t.Skip("set YOMIHON_RECORDING_OUTPUT to capture the fixed synthetic recording")
	}
	if os.Getenv(liveProtocolEnv) != "1" {
		t.Fatal("YOMIHON_EMBED_LIVE=1 is required for synthetic recording capture")
	}
	if !filepath.IsAbs(output) {
		t.Fatal("YOMIHON_RECORDING_OUTPUT must be an absolute path")
	}
	dimension, err := strconv.Atoi(os.Getenv(recordingDimensionEnv))
	if err != nil || dimension != 1536 && dimension != 3072 {
		t.Fatal("YOMIHON_RECORDING_DIMENSION must be 1536 or 3072")
	}
	key := os.Getenv("YOMIHON_EMBED_KEY")
	if strings.TrimSpace(key) == "" {
		t.Fatal("YOMIHON_EMBED_KEY is required for synthetic recording capture")
	}

	manifest := loadRecordingManifest(t)
	artifact, privacy := loadRecordingPolicies(t)
	policy, ok := schema.CorpusPolicyFingerprint(artifact, privacy)
	if !ok {
		t.Fatal("synthetic recording policy fingerprint is unavailable")
	}
	full, err := FullCacheIdentity(IdentityConfig{
		Dimension:               dimension,
		ChunkTokenCap:           manifest.ChunkTokenCap,
		VaultRoot:               manifest.SyntheticVaultRoot,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		t.Fatalf("derive full recording identity: %v", err)
	}
	compatibility, err := QueryVectorCompatibilityIdentity(dimension)
	if err != nil {
		t.Fatalf("derive query compatibility identity: %v", err)
	}
	wire, err := newGeminiWire(key, dimension, newProductionEmbeddingTransport())
	if err != nil {
		t.Fatalf("construct live recording client: %v", err)
	}

	queries := make([]recordingQueryWire, len(manifest.Cases))
	for i := range manifest.Cases {
		testCase := &manifest.Cases[i]
		if testCase.ID != "q"+twoDigits(i+1) {
			t.Fatalf("recording query %d has non-canonical ID %q", i+1, testCase.ID)
		}
		bare := search.Parse(testCase.Query).BareText()
		result, embedErr := wire.embedQuery(t.Context(), bare)
		if embedErr != nil {
			t.Fatalf("embed synthetic query %s: %v", testCase.ID, embedErr)
		}
		submissionHash, hashErr := QuerySubmissionHash(bare)
		if hashErr != nil {
			t.Fatalf("hash synthetic query %s: %v", testCase.ID, hashErr)
		}
		queries[i] = recordingQueryWire{
			ID:             testCase.ID,
			SubmissionHash: hex.EncodeToString(submissionHash[:]),
			Vector:         result.Vector,
		}
	}

	chunks := captureCorpusVectors(t, wire, manifest.ChunkTokenCap)
	captured := recordingWire{
		FormatVersion:         2,
		Dimension:             dimension,
		ChunkTokenCap:         manifest.ChunkTokenCap,
		FullIdentity:          hex.EncodeToString(full[:]),
		CompatibilityIdentity: hex.EncodeToString(compatibility[:]),
		Queries:               queries,
		Chunks:                chunks,
	}
	encoded, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("encode synthetic recording: %v", err)
	}
	if _, err := recording.Parse(encoded); err != nil {
		t.Fatalf("self-validate synthetic recording: %v", err)
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0o600); err != nil { // #nosec G703 -- the operator supplies an explicit local capture destination
		t.Fatalf("write synthetic recording: %v", err)
	}
	t.Logf("captured %d queries and %d chunks at dimension %d", len(queries), len(chunks), dimension)
}

func loadRecordingManifest(t *testing.T) recordingManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "evalset", "testdata", "cases.json"))
	if err != nil {
		t.Fatalf("read synthetic recording manifest: %v", err)
	}
	var manifest recordingManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode synthetic recording manifest: %v", err)
	}
	if err := validateRecordingManifest(manifest); err != nil {
		t.Fatalf("validate synthetic recording manifest: %v", err)
	}
	return manifest
}

func validateRecordingManifest(manifest recordingManifest) error {
	if manifest.FormatVersion != 1 ||
		manifest.SyntheticVaultRoot != "/yomihon-synthetic-semantic-eval-v1" ||
		manifest.ChunkTokenCap != 1024 ||
		len(manifest.Cases) != 40 {
		return errors.New("manifest metadata is not the frozen 40-query v1 shape")
	}
	for i, testCase := range manifest.Cases {
		wantID := "q" + twoDigits(i+1)
		if testCase.ID != wantID {
			return fmt.Errorf("query %d has ID %q, want %q", i+1, testCase.ID, wantID)
		}
		if testCase.Origin != recordingQueryOrigin {
			return fmt.Errorf("query %s has synthetic origin %q, want %q", testCase.ID, testCase.Origin, recordingQueryOrigin)
		}
		if strings.TrimSpace(testCase.Query) == "" {
			return fmt.Errorf("query %s is empty", testCase.ID)
		}
	}
	return nil
}

func TestValidateRecordingManifestRejectsUnmarkedQuery(t *testing.T) {
	manifest := loadRecordingManifest(t)
	manifest.Cases[0].Origin = ""

	err := validateRecordingManifest(manifest)
	if err == nil || !strings.Contains(err.Error(), "synthetic origin") {
		t.Fatalf("validateRecordingManifest(unmarked query) = %v, want synthetic-origin error", err)
	}
}

func loadRecordingPolicies(t *testing.T) (schema.ArtifactPolicy, schema.PrivacyPolicy) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read synthetic policy fixture: %v", err)
	}
	data = append(data, []byte("\n[privacy]\nnever_egress_dirs = [\"Private\"]\n")...)
	name := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(name, data, 0o600); writeErr != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatalf("write synthetic policy fixture: %v", writeErr)
	}
	contract, err := schema.LoadFile(name)
	if err != nil {
		t.Fatalf("load synthetic policy fixture: %v", err)
	}
	return contract.ArtifactPolicy(), contract.PrivacyPolicy()
}

func captureCorpusVectors(t *testing.T, wire *geminiWire, tokenCap int) []recordingChunkWire {
	t.Helper()
	root := filepath.Join("..", "evalset", "testdata", "vault")
	var paths []string
	if err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(name) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatalf("walk synthetic corpus: %v", err)
	}
	sort.Strings(paths)

	var chunks []recordingChunkWire
	for _, relPath := range paths {
		if !recording.IsSyntheticPath(relPath) {
			t.Fatalf("synthetic corpus contains unmarked path %q", relPath)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath))) // #nosec G304 -- relPath came from the fixed synthetic fixture walk
		if err != nil {
			t.Fatalf("read synthetic note %q: %v", relPath, err)
		}
		note := vault.Parse(relPath, data)
		chunking := ChunkNote(note.Title(), note.Body, tokenCap)
		if len(chunking.Failures) != 0 || len(chunking.Chunks) == 0 {
			t.Fatalf(
				"chunk synthetic note %q = %d chunks, %d failures",
				relPath,
				len(chunking.Chunks),
				len(chunking.Failures),
			)
		}
		for _, chunk := range chunking.Chunks {
			if chunk.Ordinal < 0 || uint64(chunk.Ordinal) > uint64(^uint32(0)) {
				t.Fatalf("synthetic chunk ordinal %d is outside uint32", chunk.Ordinal)
			}
			ordinal := uint32(chunk.Ordinal) // #nosec G115 -- the immediately preceding bounds check proves this conversion safe
			result, embedErr := wire.embed(t.Context(), chunk.Submitted)
			if embedErr != nil {
				t.Fatalf("embed synthetic chunk %q#%d: %v", relPath, chunk.Ordinal, embedErr)
			}
			submittedHash := sha256.Sum256([]byte(chunk.Submitted))
			chunks = append(chunks, recordingChunkWire{
				RelPath:       relPath,
				Ordinal:       ordinal,
				SubmittedHash: hex.EncodeToString(submittedHash[:]),
				Vector:        result.Vector,
			})
		}
	}
	if len(chunks) == 0 {
		t.Fatal("synthetic recording captured no corpus chunks")
	}
	return chunks
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
