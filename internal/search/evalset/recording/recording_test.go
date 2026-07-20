package recording

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func fixtureValue[T any](t *testing.T, object map[string]any, key string) T {
	t.Helper()
	value, ok := object[key].(T)
	if !ok {
		t.Fatalf("fixture field %q has type %T, want expected fixture type", key, object[key])
	}
	return value
}

func TestLoadReportsAbsentVectors(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "recorded-vectors.json"))
	if !errors.Is(err, ErrAbsent) {
		t.Errorf("Load(missing) error = %v, want ErrAbsent", err)
	}
}

func TestParseOwnsVectors(t *testing.T) {
	t.Parallel()

	wire := validWire()
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	query := fixtureValue[[]map[string]any](t, wire, "queries")[0]
	fixtureValue[[]float32](t, query, "vector")[0] = -1
	first := vectors.Queries()
	first[0].Vector[0] = -2
	if got := vectors.Queries()[0].Vector[0]; got != 1 {
		t.Errorf("Parse() retained vector alias: got %v, want 1", got)
	}
}

func TestParseRejectsMalformedSurfaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*testing.T, map[string]any)
		want   error
	}{
		{name: "queries absent", mutate: func(_ *testing.T, w map[string]any) { w["queries"] = []any{} }, want: ErrInvalid},
		{name: "chunks absent", mutate: func(_ *testing.T, w map[string]any) { w["chunks"] = []any{} }, want: ErrInvalid},
		{name: "unknown field", mutate: func(_ *testing.T, w map[string]any) { w["credential"] = "must-not-exist" }, want: ErrInvalid},
		{name: "wrong query count", mutate: func(t *testing.T, w map[string]any) {
			t.Helper()
			w["queries"] = fixtureValue[[]map[string]any](t, w, "queries")[:39]
		}, want: ErrInvalid},
		{name: "zero query hash", mutate: func(t *testing.T, w map[string]any) {
			t.Helper()
			fixtureValue[[]map[string]any](t, w, "queries")[0]["submission_hash"] = string(make([]byte, sha256.Size*2))
		}, want: ErrInvalid},
		{name: "wrong query dimension", mutate: func(t *testing.T, w map[string]any) {
			t.Helper()
			fixtureValue[[]map[string]any](t, w, "queries")[0]["vector"] = make([]float32, 2)
		}, want: ErrInvalid},
		{name: "unmarked chunk path", mutate: func(t *testing.T, w map[string]any) {
			t.Helper()
			fixtureValue[[]map[string]any](t, w, "chunks")[0]["rel_path"] = "Writing/real.md"
		}, want: ErrInvalid},
		{name: "duplicate chunk", mutate: func(t *testing.T, w map[string]any) {
			t.Helper()
			chunks := fixtureValue[[]map[string]any](t, w, "chunks")
			chunks[1]["rel_path"] = chunks[0]["rel_path"]
			chunks[1]["ordinal"] = chunks[0]["ordinal"]
		}, want: ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wire := validWire()
			tt.mutate(t, wire)
			data, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Parse(data)
			if !errors.Is(err, tt.want) {
				t.Errorf("Parse(%s) error = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

func TestIdentityChecksAreConsumerOwned(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(validWire())
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := vectors.RequireFullIdentity(sha256.Sum256([]byte("other"))); !errors.Is(err, ErrIdentityMismatch) {
		t.Errorf("RequireFullIdentity(other) error = %v, want ErrIdentityMismatch", err)
	}
	if _, err := vectors.QueryVectors(sha256.Sum256([]byte("other"))); !errors.Is(err, ErrIdentityMismatch) {
		t.Errorf("QueryVectors(other) error = %v, want ErrIdentityMismatch", err)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte(`{"format_version":2}`))
	f.Add([]byte(`{"format_version":2} {}`))
	valid, err := json.Marshal(validWire())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		first, firstErr := Parse(data)
		second, secondErr := Parse(data)
		if recordingErrorText(firstErr) != recordingErrorText(secondErr) {
			t.Errorf("Parse(%q) errors = %q then %q", data, recordingErrorText(firstErr), recordingErrorText(secondErr))
		}
		if firstErr != nil {
			if !errors.Is(firstErr, ErrAbsent) && !errors.Is(firstErr, ErrInvalid) {
				t.Errorf("Parse(%q) error = %v, want ErrAbsent or ErrInvalid", data, firstErr)
			}
			return
		}
		if first == nil || second == nil {
			t.Fatalf("Parse(%q) returned nil vectors without an error", data)
		}

		if first.Dimension() <= 0 || first.ChunkTokenCap() <= 0 ||
			first.Dimension() != second.Dimension() || first.ChunkTokenCap() != second.ChunkTokenCap() {
			t.Errorf("Parse(%q) dimensions = (%d,%d) then (%d,%d)", data, first.Dimension(), first.ChunkTokenCap(), second.Dimension(), second.ChunkTokenCap())
		}
		queries := first.Queries()
		chunks := first.Chunks()
		if diff := cmp.Diff(queries, second.Queries()); diff != "" {
			t.Errorf("Parse(%q) queries are not deterministic (-first +second):\n%s", data, diff)
		}
		if diff := cmp.Diff(chunks, second.Chunks()); diff != "" {
			t.Errorf("Parse(%q) chunks are not deterministic (-first +second):\n%s", data, diff)
		}
		if len(queries) != 40 || len(chunks) == 0 {
			t.Errorf("Parse(%q) accepted %d queries and %d chunks", data, len(queries), len(chunks))
		}
		for i, query := range queries {
			if query.ID != fmtQueryID(i+1) || !validVector(query.Vector, first.Dimension()) ||
				query.SubmissionHash == ([sha256.Size]byte{}) {
				t.Errorf("Parse(%q) accepted invalid query vector %d: %+v", data, i+1, query)
			}
		}
		for i, chunk := range chunks {
			if !IsSyntheticPath(chunk.RelPath) || !validVector(chunk.Vector, first.Dimension()) ||
				chunk.SubmittedHash == ([sha256.Size]byte{}) {
				t.Errorf("Parse(%q) accepted invalid chunk vector %d: %+v", data, i+1, chunk)
			}
		}
		if err := first.RequireFullIdentity(first.fullIdentity); err != nil {
			t.Errorf("Parse(%q) rejected its full identity: %v", data, err)
		}
		queryVectors, err := first.QueryVectors(first.compatibilityIdentity)
		if err != nil {
			t.Errorf("Parse(%q) rejected its query compatibility identity: %v", data, err)
		}
		assertFuzzVectorsOwned(t, first, queryVectors)
	})
}

func recordingErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func assertFuzzVectorsOwned(t *testing.T, vectors *Vectors, queryVectors [][]float32) {
	t.Helper()
	if len(queryVectors) == 0 || len(queryVectors[0]) == 0 {
		t.Fatal("successful Parse returned no query-vector components")
	}
	originalQuery := queryVectors[0][0]
	queryVectors[0][0] = differentFloat(originalQuery)
	freshQueries, err := vectors.QueryVectors(vectors.compatibilityIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if freshQueries[0][0] != originalQuery {
		t.Error("QueryVectors exposes recording-owned storage")
	}

	chunks := vectors.Chunks()
	if len(chunks) == 0 || len(chunks[0].Vector) == 0 {
		t.Fatal("successful Parse returned no chunk-vector components")
	}
	originalChunk := chunks[0].Vector[0]
	chunks[0].Vector[0] = differentFloat(originalChunk)
	if vectors.Chunks()[0].Vector[0] != originalChunk {
		t.Error("Chunks exposes recording-owned storage")
	}
}

func differentFloat(value float32) float32 {
	if value == 0 {
		return 1
	}
	return 0
}

func validWire() map[string]any {
	const dimension = 3
	full := sha256.Sum256([]byte("full identity"))
	compatibility := sha256.Sum256([]byte("query compatibility"))
	queries := make([]map[string]any, 40)
	for i := range queries {
		submission := sha256.Sum256([]byte{byte(i + 1)})
		queries[i] = map[string]any{
			"id":              fmtQueryID(i + 1),
			"submission_hash": hex.EncodeToString(submission[:]),
			"vector":          []float32{1, float32(i), -1},
		}
	}
	chunkHash := sha256.Sum256([]byte("chunk"))
	chunks := []map[string]any{
		{"rel_path": "__synthetic_eval_a.md", "ordinal": 0, "submitted_hash": hex.EncodeToString(chunkHash[:]), "vector": []float32{1, 0, 0}},
		{"rel_path": "__synthetic_eval_b.md", "ordinal": 0, "submitted_hash": hex.EncodeToString(chunkHash[:]), "vector": []float32{0, 1, 0}},
	}
	return map[string]any{
		"format_version":                      2,
		"dimension":                           dimension,
		"chunk_token_cap":                     100,
		"full_cache_identity":                 hex.EncodeToString(full[:]),
		"query_vector_compatibility_identity": hex.EncodeToString(compatibility[:]),
		"queries":                             queries,
		"chunks":                              chunks,
	}
}

func fmtQueryID(position int) string {
	return fmt.Sprintf("q%02d", position)
}
