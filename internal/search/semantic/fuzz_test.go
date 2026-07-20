package semantic

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// FuzzChunkNote keeps the chunker's cap and continuation invariants true over
// arbitrary Markdown. The input is bounded because production notes are finite
// files and one fuzz worker must not turn a generated megabyte into hundreds
// of thousands of one-rune chunks.
func FuzzChunkNote(f *testing.F) {
	f.Add("Title", "plain body\n", int16(128))
	f.Add("日本語", "## 課\n\n本文\n\n### 例\n\n```go\nline one\nline two\n```\n", int16(40))
	f.Add(strings.Repeat("界", 20), "## "+strings.Repeat("界", 20)+"\n\nbody\n", int16(10))
	f.Add("", "", int16(0))

	f.Fuzz(func(t *testing.T, title, body string, rawCap int16) {
		if len(title) > 8<<10 || len(body) > 64<<10 {
			t.Skip()
		}
		tokenCap := max(-1, min(int(rawCap), 1024))

		first := ChunkNote(title, body, tokenCap)
		second := ChunkNote(title, body, tokenCap)
		if diff := cmp.Diff(first, second); diff != "" {
			t.Errorf("ChunkNote(%q, %q, %d) is not deterministic (-first +second):\n%s", title, body, tokenCap, diff)
		}

		for i, chunk := range first.Chunks {
			if chunk.Ordinal != i {
				t.Errorf("ChunkNote chunk %d ordinal = %d, want %d", i, chunk.Ordinal, i)
			}
			if chunk.Body == "" || !strings.HasSuffix(chunk.Submitted, chunk.Body) {
				t.Errorf("ChunkNote chunk %d = body %q submitted %q, want non-empty submitted suffix", i, chunk.Body, chunk.Submitted)
			}
			if chunk.ProxyTokens != ProxyTokens(chunk.Submitted) {
				t.Errorf("ChunkNote chunk %d proxy tokens = %d, recomputed %d", i, chunk.ProxyTokens, ProxyTokens(chunk.Submitted))
			}
			if tokenCap <= 0 || chunk.ProxyTokens > tokenCap {
				t.Errorf("ChunkNote chunk %d proxy tokens = %d, cap %d", i, chunk.ProxyTokens, tokenCap)
			}
			if chunk.Part < 1 || chunk.Part > chunk.Parts || chunk.Parts < 1 {
				t.Errorf("ChunkNote chunk %d continuation = %d/%d", i, chunk.Part, chunk.Parts)
			}
			if utf8.ValidString(title) && utf8.ValidString(body) &&
				(!utf8.ValidString(chunk.Body) || !utf8.ValidString(chunk.Submitted)) {
				t.Errorf("ChunkNote(valid UTF-8) chunk %d is invalid UTF-8", i)
			}
		}
		for _, failure := range first.Failures {
			if failure.Section < 0 ||
				(failure.Reason != ChunkFailureInvalidCap && failure.Reason != ChunkFailurePrefixConsumesCap) {
				t.Errorf("ChunkNote failure = %+v, want a declared failure kind and non-negative section", failure)
			}
		}
		if tokenCap <= 0 && len(first.Chunks) != 0 {
			t.Errorf("ChunkNote(tokenCap %d) returned %d chunks, want none", tokenCap, len(first.Chunks))
		}
	})
}

// FuzzDecodeEmbeddingResponse keeps the provider's untrusted success body
// fail-closed. Success is possible only for one exact-length, finite, non-zero
// float32 vector; every other byte shape must remain a provider-owned failure.
func FuzzDecodeEmbeddingResponse(f *testing.F) {
	f.Add([]byte(`{"embedding":{"values":[1,0]}}`), uint8(1))
	f.Add([]byte(`{"embedding":{"values":[0,0]}}`), uint8(1))
	f.Add([]byte(`{"embedding":{"values":[1]}} {}`), uint8(0))
	f.Add([]byte(`{`), uint8(4))

	f.Fuzz(func(t *testing.T, body []byte, rawDimension uint8) {
		if len(body) > 1<<20 {
			t.Skip()
		}
		dimension := 1 + int(rawDimension%32)
		got, err := decodeEmbeddingResponse(body, dimension)
		if err != nil {
			embedErr, ok := errors.AsType[*EmbedError](err)
			if !ok || embedErr.Kind != EmbedFailureProvider {
				t.Errorf("decodeEmbeddingResponse(%q, %d) error = %#v, want provider failure", body, dimension, err)
			}
			return
		}
		if len(got.Vector) != dimension {
			t.Fatalf("decodeEmbeddingResponse(%q, %d) vector length = %d", body, dimension, len(got.Vector))
		}
		if _, ok := vectorMagnitude(got.Vector); !ok {
			t.Errorf("decodeEmbeddingResponse(%q, %d) accepted a zero or non-finite vector", body, dimension)
		}
	})
}

// FuzzDecodeStoredVector keeps the SQLite blob boundary exact: successful
// decoding consumes dimension*4 little-endian bytes, rejects non-finite values,
// and round-trips every accepted bit pattern byte-for-byte.
func FuzzDecodeStoredVector(f *testing.F) {
	f.Add([]byte{0, 0, 0x80, 0x3f}, int8(1))
	f.Add([]byte{0, 0, 0xc0, 0x7f}, int8(1))
	f.Add([]byte{0}, int8(1))
	f.Add([]byte{}, int8(0))

	f.Fuzz(func(t *testing.T, raw []byte, rawDimension int8) {
		if len(raw) > 4096 {
			t.Skip()
		}
		dimension := int(rawDimension)
		first, err := decodeStoredVector(raw, dimension)
		if err != nil {
			if !errors.Is(err, ErrStoreCorrupt) {
				t.Errorf("decodeStoredVector(%x, %d) error = %v, want ErrStoreCorrupt", raw, dimension, err)
			}
			return
		}
		second, secondErr := decodeStoredVector(raw, dimension)
		if secondErr != nil {
			t.Fatalf("decodeStoredVector(%x, %d) second error = %v", raw, dimension, secondErr)
		}
		if diff := cmp.Diff(first, second); diff != "" {
			t.Errorf("decodeStoredVector(%x, %d) is not deterministic (-first +second):\n%s", raw, dimension, diff)
		}
		if dimension <= 0 || len(first) != dimension || len(raw) != dimension*4 {
			t.Fatalf("decodeStoredVector(%x, %d) accepted inconsistent shape len=%d", raw, dimension, len(first))
		}
		for _, value := range first {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				t.Errorf("decodeStoredVector(%x, %d) accepted non-finite value", raw, dimension)
			}
		}
		if encoded := encodeStoredVector(first); !bytes.Equal(encoded, raw) {
			t.Errorf("decodeStoredVector(%x, %d) re-encoded %x", raw, dimension, encoded)
		}
	})
}
