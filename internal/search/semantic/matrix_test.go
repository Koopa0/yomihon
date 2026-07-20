package semantic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

type matrixHTTPResult struct {
	status int
	body   string
	err    error
}

type matrixTransport struct {
	results  []matrixHTTPResult
	requests []string
}

func (t *matrixTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	t.requests = append(t.requests, string(body))
	if len(t.requests) > len(t.results) {
		return nil, errors.New("unexpected HTTP send")
	}
	result := t.results[len(t.requests)-1]
	if result.err != nil {
		return nil, result.err
	}
	return response(result.status, result.body), nil
}

func TestIndexerDocumentHTTPBoundaryMatrix(t *testing.T) {
	providerError := func(code int, status string) string {
		return `{"error":{"code":` + strconv.Itoa(code) + `,"status":"` + status + `"}}`
	}
	tests := []struct {
		name string
		step matrixHTTPResult
		want EmbedFailureKind
	}{
		{name: "row 18 throttle", step: matrixHTTPResult{status: 429, body: providerError(429, "RESOURCE_EXHAUSTED")}, want: EmbedFailureRateLimited},
		{name: "row 19 transport", step: matrixHTTPResult{err: errors.New("offline")}, want: EmbedFailureUnreachable},
		{name: "row 20 rejected", step: matrixHTTPResult{status: 403, body: providerError(403, "PERMISSION_DENIED")}, want: EmbedFailureRejected},
		{name: "row 21 provider", step: matrixHTTPResult{status: 500, body: providerError(500, "INTERNAL")}, want: EmbedFailureProvider},
		{name: "row 21 unknown", step: matrixHTTPResult{status: 418, body: providerError(418, "TEAPOT")}, want: EmbedFailureProvider},
		{name: "row 22 malformed", step: matrixHTTPResult{status: 400, body: providerError(400, "INVALID_ARGUMENT")}, want: EmbedFailureMalformedRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer, active, target, transport := matrixIndexer(t, []matrixHTTPResult{tt.step})
			_, err := indexer.Reconcile(t.Context(), target)
			if !isEmbedFailure(err, tt.want) {
				t.Fatalf("Reconcile() error = %v, want %q", err, tt.want)
			}
			if len(transport.requests) != 1 {
				t.Fatalf("HTTP sends = %d, want one document send", len(transport.requests))
			}
			if strings.Contains(transport.requests[0], queryPromptPrefix) {
				t.Fatal("document terminal reached query embedding")
			}
			assertActiveCorpus(t, indexer.settings.storePath, active.Fingerprint)
		})
	}
}

func TestIndexerActivatedQueryHTTPBoundaryMatrix(t *testing.T) {
	providerError := func(code int, status string) string {
		return `{"error":{"code":` + strconv.Itoa(code) + `,"status":"` + status + `"}}`
	}
	tests := []struct {
		name    string
		query   matrixHTTPResult
		want    EmbedFailureKind
		success bool
	}{
		{name: "row 24 success", query: matrixHTTPResult{status: 200, body: validEmbeddingBody(0)}, success: true},
		{name: "row 24 transport", query: matrixHTTPResult{err: errors.New("offline")}, want: EmbedFailureUnreachable},
		{name: "row 24 throttle", query: matrixHTTPResult{status: 429, body: providerError(429, "RESOURCE_EXHAUSTED")}, want: EmbedFailureRateLimited},
		{name: "row 24 rejected", query: matrixHTTPResult{status: 401, body: providerError(401, "UNAUTHENTICATED")}, want: EmbedFailureRejected},
		{name: "row 24 provider", query: matrixHTTPResult{status: 503, body: providerError(503, "UNAVAILABLE")}, want: EmbedFailureProvider},
		{name: "row 24 unknown", query: matrixHTTPResult{status: 418, body: providerError(418, "TEAPOT")}, want: EmbedFailureProvider},
		{name: "row 24 malformed", query: matrixHTTPResult{status: 400, body: providerError(400, "INVALID_ARGUMENT")}, want: EmbedFailureMalformedRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexer, _, target, transport := matrixIndexer(t, []matrixHTTPResult{
				{status: 200, body: validEmbeddingBody(0)},
				tt.query,
			})
			ready, err := indexer.Reconcile(t.Context(), target)
			if err != nil {
				t.Fatalf("Reconcile() error: %v", err)
			}
			_, err = ready.Search(t.Context(), "query-sentinel", nil, 1)
			if tt.success {
				if err != nil {
					t.Fatalf("Search() error: %v", err)
				}
			} else if !isEmbedFailure(err, tt.want) {
				t.Fatalf("Search() error = %v, want %q", err, tt.want)
			}
			if len(transport.requests) != 2 {
				t.Fatalf("HTTP sends = %d, want one document plus one query", len(transport.requests))
			}
			if strings.Contains(transport.requests[0], queryPromptPrefix) || !strings.Contains(transport.requests[1], queryPromptPrefix+"query-sentinel") {
				t.Fatalf("HTTP order is not document then query: %#v", transport.requests)
			}
			assertActiveCorpus(t, indexer.settings.storePath, target.Fingerprint)
		})
	}
}

func TestIndexerLocalPublicationFailureIsGenerationIncomplete(t *testing.T) {
	indexer, active, target, transport := matrixIndexer(t, []matrixHTTPResult{{
		status: http.StatusOK,
		body:   validEmbeddingBody(0),
	}})
	indexer.deps.newIndex = func([]ChunkVector, int) (*Index, error) {
		return nil, ErrDimensionMismatch
	}
	_, err := indexer.Reconcile(t.Context(), target)
	if !errors.Is(err, ErrGenerationIncomplete) || !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("Reconcile() error = %v, want generation-incomplete dimension failure", err)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("HTTP sends = %d, want one document send before local publication failure", len(transport.requests))
	}
	assertActiveCorpus(t, indexer.settings.storePath, active.Fingerprint)
}

func TestIndexerFinalDriftStopsBeforeQuery(t *testing.T) {
	config, all := fixtureIndexerConfig(t, 3)
	active := fixtureIndexerCorpus(t, all.Chunks[:1])
	target := fixtureIndexerCorpus(t, all.Chunks[1:2])
	changed := fixtureIndexerCorpus(t, all.Chunks[2:])
	seedActiveGeneration(t, &config, active)
	reads := 0
	config.deps.readCorpus = func(context.Context) (Corpus, error) {
		reads++
		if reads < 2 {
			return target, nil
		}
		return changed, nil
	}
	transport := &matrixTransport{results: []matrixHTTPResult{{status: http.StatusOK, body: validEmbeddingBody(0)}}}
	indexer := matrixIndexerWithConfig(t, &config, transport)
	_, err := indexer.Reconcile(t.Context(), target)
	if !errors.Is(err, ErrVaultChanged) {
		t.Fatalf("Reconcile() error = %v, want ErrVaultChanged", err)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("HTTP sends = %d, want one document and zero query sends", len(transport.requests))
	}
	assertActiveCorpus(t, config.settings.storePath, active.Fingerprint)
}

func matrixIndexer(
	t *testing.T,
	results []matrixHTTPResult,
) (indexer *Indexer, active, target Corpus, transport *matrixTransport) {
	t.Helper()
	config, all := fixtureIndexerConfig(t, 2)
	active = fixtureIndexerCorpus(t, all.Chunks[:1])
	target = fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return target, nil }
	seedActiveGeneration(t, &config, active)
	transport = &matrixTransport{results: results}
	indexer = matrixIndexerWithConfig(t, &config, transport)
	return indexer, active, target, transport
}

func matrixIndexerWithConfig(t *testing.T, config *indexerSetup, transport http.RoundTripper) *Indexer {
	t.Helper()
	embedder, err := newGeminiEmbedder(
		"key-sentinel",
		config.settings.identity.Dimension(),
		transport,
		func(context.Context, *ChunkIdentity) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	indexer, err := newIndexerWithProvider(config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: embedder.EmbedChunk,
			embedQuery: embedder.EmbedQuery,
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return indexer
}

func assertActiveCorpus(t *testing.T, storePath string, want [32]byte) {
	t.Helper()
	store, err := openStore(t.Context(), storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, store)
	active, err := store.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if active.corpusFingerprint != want {
		t.Fatalf("active corpus = %x, want %x", active.corpusFingerprint, want)
	}
}
