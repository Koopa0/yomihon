package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const liveProtocolEnv = "YOMIHON_EMBED_LIVE"

// TestGeminiEmbedding2LiveProtocol sends only fixed synthetic text and is
// opt-in because it spends the operator's provider quota and requires the
// operator's own credential.
func TestGeminiEmbedding2LiveProtocol(t *testing.T) {
	if os.Getenv(liveProtocolEnv) != "1" {
		t.Skip("set YOMIHON_EMBED_LIVE=1 to run the paid synthetic provider probe")
	}
	key := os.Getenv("YOMIHON_EMBED_KEY")
	if strings.TrimSpace(key) == "" {
		t.Fatal("YOMIHON_EMBED_KEY is required for the live provider probe")
	}

	const shortDocument = "title: synthetic protocol probe | text: deterministic ASCII document"
	for _, dimension := range []int{1536, 3072} {
		t.Run(strconv.Itoa(dimension), func(t *testing.T) {
			wire, err := newGeminiWire(key, dimension, newProductionEmbeddingTransport())
			if err != nil {
				t.Fatalf("newGeminiWire(%d) error: %v", dimension, err)
			}
			result, err := wire.embed(t.Context(), shortDocument)
			if err != nil {
				t.Fatalf("live short request at dimension %d: %v", dimension, err)
			}
			if len(result.Vector) != dimension {
				t.Fatalf("live vector length = %d, want %d", len(result.Vector), dimension)
			}
			if norm := vectorNorm(result.Vector); norm < 0.999 || norm > 1.001 {
				t.Errorf("live vector norm = %.6f, want provider-normalized vector", norm)
			}
		})
	}

	overLimit := "title: synthetic over-limit probe | text: " + strings.Repeat("abcdefghij ", 10_000)
	count, err := liveCountTokens(t.Context(), key, overLimit)
	if err != nil {
		t.Fatalf("count synthetic over-limit tokens: %v", err)
	}
	if count <= 8192 {
		t.Fatalf("synthetic over-limit fixture counted %d tokens, want more than 8192", count)
	}

	wire, err := newGeminiWire(key, 1536, newProductionEmbeddingTransport())
	if err != nil {
		t.Fatalf("newGeminiWire(over-limit) error: %v", err)
	}
	_, err = wire.embed(t.Context(), overLimit)
	embedErr, ok := errors.AsType[*EmbedError](err)
	if !ok || embedErr.Kind != EmbedFailureMalformedRequest {
		t.Fatalf("over-limit request error = %v, want confirmed malformed request", err)
	}
}

func vectorNorm(vector []float32) float64 {
	var sum float64
	for _, value := range vector {
		sum += float64(value) * float64(value)
	}
	return math.Sqrt(sum)
}

type liveCountRequest struct {
	Contents []liveCountContent `json:"contents"`
}

type liveCountContent struct {
	Parts []embedPart `json:"parts"`
}

func liveCountTokens(ctx context.Context, key, text string) (int, error) {
	encoded, err := json.Marshal(liveCountRequest{
		Contents: []liveCountContent{{Parts: []embedPart{{Text: text}}}},
	})
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-2:countTokens",
		bytes.NewReader(encoded),
	)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Goog-Api-Key", key)
	client := &http.Client{
		Transport: newProductionEmbeddingTransport(),
		Timeout:   30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close() //nolint:errcheck // the read or status below is the actionable result
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, &EmbedError{Kind: EmbedFailureProvider}
	}
	var decoded struct {
		TotalTokens int `json:"totalTokens"`
	}
	if !decodeSingleJSON(body, &decoded) || decoded.TotalTokens <= 0 {
		return 0, &EmbedError{Kind: EmbedFailureProvider}
	}
	return decoded.TotalTokens, nil
}
