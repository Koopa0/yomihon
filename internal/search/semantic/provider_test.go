package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const testEmbeddingDimension = 128

func TestGeminiWireSendsPinnedDocumentRequest(t *testing.T) {
	t.Parallel()

	const key = "key-sentinel"
	providerResponse := response(
		http.StatusOK,
		validEmbeddingBody(9),
	)
	t.Cleanup(func() {
		if closeErr := providerResponse.Body.Close(); closeErr != nil {
			t.Errorf("close provider response: %v", closeErr)
		}
	})
	transport := &recordingRoundTripper{response: providerResponse}
	wire, err := newGeminiWire(key, testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}
	got, err := wire.embedChunk(t.Context(), []byte("title: Public | text: body"))
	if err != nil {
		t.Fatal(err)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("RoundTrip calls = %d, want 1", len(transport.requests))
	}
	request := transport.requests[0]
	if request.method != http.MethodPost || request.url != geminiEmbeddingEndpoint {
		t.Errorf("request = %s %s", request.method, request.url)
	}
	if request.key != key || request.contentType != "application/json" {
		t.Errorf("headers = key %q content-type %q", request.key, request.contentType)
	}
	wantBody := `{"content":{"parts":[{"text":"title: Public | text: body"}]},"embedContentConfig":{"autoTruncate":false,"outputDimensionality":128}}`
	if request.body != wantBody {
		t.Errorf("body = %s, want %s", request.body, wantBody)
	}
	if strings.Contains(request.url, key) || strings.Contains(request.body, key) {
		t.Error("API key escaped its header-only boundary")
	}
	if request.idempotencyKey != "" || request.xIdempotencyKey != "" {
		t.Errorf("request carries a replay-enabling idempotency header: %+v", request)
	}
	if len(got.Vector) != testEmbeddingDimension || got.Vector[0] != 1 {
		t.Errorf("result = %+v", got)
	}
}

func TestGeminiWireWrapsQueryExactlyOnce(t *testing.T) {
	t.Parallel()

	providerResponse := response(http.StatusOK, validEmbeddingBody(0))
	t.Cleanup(func() {
		if closeErr := providerResponse.Body.Close(); closeErr != nil {
			t.Errorf("close provider response: %v", closeErr)
		}
	})
	transport := &recordingRoundTripper{response: providerResponse}
	wire, err := newGeminiWire("key", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.embedQuery(t.Context(), "raw query"); err != nil {
		t.Fatal(err)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("RoundTrip calls = %d, want 1", len(transport.requests))
	}
	want := `{"content":{"parts":[{"text":"task: search result | query: raw query"}]},"embedContentConfig":{"autoTruncate":false,"outputDimensionality":128}}`
	if transport.requests[0].body != want {
		t.Errorf("query body = %s, want %s", transport.requests[0].body, want)
	}
}

func TestGeminiHTTPBoundaryTerminalMatrix(t *testing.T) {
	t.Parallel()

	providerError := func(code int, status, message string) string {
		message += " query-sentinel document-sentinel key-sentinel"
		return fmt.Sprintf(`{"error":{"code":%d,"message":%q,"status":%q}}`, code, message, status)
	}
	tests := []struct {
		name       string
		statusCode int
		body       string
		transport  error
		want       EmbedFailureKind
		success    bool
	}{
		{name: "success", statusCode: 200, body: validEmbeddingBody(0), success: true},
		{name: "transport non-answer", transport: errors.New("offline query-sentinel document-sentinel key-sentinel"), want: EmbedFailureUnreachable},
		{name: "rate limited", statusCode: 429, body: providerError(429, "RESOURCE_EXHAUSTED", "slow down"), want: EmbedFailureRateLimited},
		{name: "invalid credential", statusCode: 401, body: providerError(401, "UNAUTHENTICATED", "bad key"), want: EmbedFailureRejected},
		{name: "permission denied", statusCode: 403, body: providerError(403, "PERMISSION_DENIED", "bad project"), want: EmbedFailureRejected},
		{name: "confirmed malformed", statusCode: 400, body: providerError(400, "INVALID_ARGUMENT", "bad body"), want: EmbedFailureMalformedRequest},
		{name: "failed precondition", statusCode: 400, body: providerError(400, "FAILED_PRECONDITION", "provider state"), want: EmbedFailureProvider},
		{name: "not found", statusCode: 404, body: providerError(404, "NOT_FOUND", "provider state"), want: EmbedFailureProvider},
		{name: "cancelled", statusCode: 499, body: providerError(499, "CANCELLED", "provider state"), want: EmbedFailureProvider},
		{name: "provider internal", statusCode: 500, body: providerError(500, "INTERNAL", "oops"), want: EmbedFailureProvider},
		{name: "provider unavailable", statusCode: 503, body: providerError(503, "UNAVAILABLE", "later"), want: EmbedFailureProvider},
		{name: "deadline exceeded", statusCode: 504, body: providerError(504, "DEADLINE_EXCEEDED", "provider state"), want: EmbedFailureProvider},
		{name: "unknown status", statusCode: 418, body: providerError(418, "TEAPOT", "unknown"), want: EmbedFailureProvider},
		{name: "HTTP and envelope disagree", statusCode: 429, body: providerError(400, "INVALID_ARGUMENT", "mismatch"), want: EmbedFailureProvider},
		{name: "malformed error envelope", statusCode: 500, body: `{`, want: EmbedFailureProvider},
		{name: "redirect is terminal", statusCode: 302, body: providerError(302, "FOUND", "elsewhere"), want: EmbedFailureProvider},
		{name: "success missing vector", statusCode: 200, body: `{}`, want: EmbedFailureProvider},
		{name: "success wrong dimension", statusCode: 200, body: `{"embedding":{"values":[1,0]}}`, want: EmbedFailureProvider},
		{name: "success zero vector", statusCode: 200, body: `{"embedding":{"values":[` + strings.TrimPrefix(strings.Repeat(",0", testEmbeddingDimension), ",") + `]}}`, want: EmbedFailureProvider},
		{
			name:       "success not float32 finite",
			statusCode: 200,
			body:       `{"embedding":{"values":[1e100` + strings.Repeat(",1", testEmbeddingDimension-1) + `]}}`,
			want:       EmbedFailureProvider,
		},
		{name: "success trailing JSON", statusCode: 200, body: `{"embedding":{"values":[1,0,0,0]}} {}`, want: EmbedFailureProvider},
		{name: "oversized response", statusCode: 200, body: strings.Repeat("x", maxEmbeddingResponseBytes+1), want: EmbedFailureProvider},
	}
	for _, surface := range []string{"query", "document"} {
		for _, tt := range tests {
			t.Run(surface+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				transport := &recordingRoundTripper{err: tt.transport}
				if tt.transport == nil {
					providerResponse := response(tt.statusCode, tt.body)
					t.Cleanup(func() {
						if closeErr := providerResponse.Body.Close(); closeErr != nil {
							t.Errorf("close provider response: %v", closeErr)
						}
					})
					transport.response = providerResponse
				}
				embedder, err := newGeminiEmbedder(
					"key-sentinel",
					testEmbeddingDimension,
					transport,
					func(context.Context, *ChunkIdentity) error { return nil },
				)
				if err != nil {
					t.Fatal(err)
				}
				switch surface {
				case "query":
					_, err = embedder.EmbedQuery(t.Context(), "query-sentinel")
				case "document":
					submitted := []byte("title: Public | text: document-sentinel")
					_, err = embedder.EmbedChunk(t.Context(), &CorpusChunk{
						Submitted:     submitted,
						SubmittedHash: sha256.Sum256(submitted),
					})
				default:
					t.Fatalf("unknown test surface %q", surface)
				}
				if tt.success {
					if err != nil {
						t.Fatalf("error = %v, want success", err)
					}
				} else {
					embedErr, ok := errors.AsType[*EmbedError](err)
					if !ok || embedErr.Kind != tt.want {
						t.Fatalf("error = %#v, want kind %q", err, tt.want)
					}
				}
				visibleError := fmt.Sprintf("%+v", err)
				for _, sensitive := range []string{"query-sentinel", "document-sentinel", "key-sentinel"} {
					if strings.Contains(visibleError, sensitive) {
						t.Errorf("error exposed %q: %q", sensitive, visibleError)
					}
				}
				if len(transport.requests) != 1 {
					t.Fatalf("RoundTrip calls = %d, want exactly 1", len(transport.requests))
				}
			})
		}
	}
}

func TestGeminiClientSendsEachExplicitActionOnce(t *testing.T) {
	t.Parallel()

	transport := &responseRoundTripper{body: validEmbeddingBody(0), status: http.StatusOK}
	embedder, err := newGeminiEmbedder(
		"key-sentinel",
		testEmbeddingDimension,
		transport,
		func(context.Context, *ChunkIdentity) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"first action", "second action"} {
		if _, err := embedder.EmbedQuery(t.Context(), query); err != nil {
			t.Fatalf("EmbedQuery(%q) error: %v", query, err)
		}
	}
	if transport.calls != 2 {
		t.Fatalf("RoundTrip calls = %d, want exactly 2 for two actions", transport.calls)
	}
}

func TestGeminiLocalFormationFailuresDoNotSend(t *testing.T) {
	t.Parallel()

	transport := &responseRoundTripper{body: validEmbeddingBody(0), status: http.StatusOK}
	embedder, err := newGeminiEmbedder(
		"key-sentinel",
		testEmbeddingDimension,
		transport,
		func(context.Context, *ChunkIdentity) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.EmbedQuery(t.Context(), string([]byte{0xff})); !isEmbedFailure(err, EmbedFailureInternal) {
		t.Fatalf("invalid query error = %v, want FailureInternal", err)
	}
	submitted := []byte{0xff}
	if _, err := embedder.EmbedChunk(t.Context(), &CorpusChunk{
		Submitted:     submitted,
		SubmittedHash: sha256.Sum256(submitted),
	}); !isEmbedFailure(err, EmbedFailureInternal) {
		t.Fatalf("invalid document error = %v, want FailureInternal", err)
	}
	if transport.calls != 0 {
		t.Fatalf("local formation failures sent %d HTTP requests", transport.calls)
	}
}

func isEmbedFailure(err error, want EmbedFailureKind) bool {
	embedErr, ok := errors.AsType[*EmbedError](err)
	return ok && embedErr.Kind == want
}

func TestGeminiWireNeverReturnsProviderQueryOrKeyBytes(t *testing.T) {
	t.Parallel()

	const query = "query-sentinel"
	const key = "key-sentinel"
	body := fmt.Sprintf(`{"error":{"code":500,"status":"INTERNAL","message":%q,"details":[%q]}}`, query, key)
	providerResponse := response(http.StatusInternalServerError, body)
	t.Cleanup(func() {
		if closeErr := providerResponse.Body.Close(); closeErr != nil {
			t.Errorf("close provider response: %v", closeErr)
		}
	})
	transport := &recordingRoundTripper{response: providerResponse}
	wire, err := newGeminiWire(key, testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}
	_, err = wire.embedQuery(t.Context(), query)
	if err == nil {
		t.Fatal("EmbedQuery error = nil")
	}
	got := fmt.Sprintf("%+v", err)
	if strings.Contains(got, query) || strings.Contains(got, key) {
		t.Errorf("error leaked sensitive bytes: %q", got)
	}
}

func TestParseRetryAfterDistinguishesImmediateFromMissing(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		value     string
		want      time.Duration
		wantValid bool
	}{
		{name: "missing", wantValid: false},
		{name: "invalid", value: "later", wantValid: false},
		{name: "overflow", value: "9223372036854775807", wantValid: false},
		{name: "immediate", value: "0", wantValid: true},
		{name: "seconds", value: "30", want: 30 * time.Second, wantValid: true},
		{name: "past date", value: now.Add(-time.Minute).Format(http.TimeFormat), wantValid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, valid := parseRetryAfter(tt.value, now)
			if got != tt.want || valid != tt.wantValid {
				t.Fatalf("parseRetryAfter(%q) = (%s, %t), want (%s, %t)", tt.value, got, valid, tt.want, tt.wantValid)
			}
		})
	}
}

func TestNewGeminiWireRejectsLocalMisconfigurationWithoutSending(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		key          string
		dimension    int
		nilTransport bool
		want         error
		wantNot      error
	}{
		{
			name:      "empty key",
			dimension: 1536,
			want:      ErrEmbedderUnconfigured,
			wantNot:   ErrEmbedderConfiguration,
		},
		{
			name:      "whitespace key",
			key:       " \t\n ",
			dimension: 1536,
			want:      ErrEmbedderUnconfigured,
			wantNot:   ErrEmbedderConfiguration,
		},
		{
			name:      "surrounding whitespace",
			key:       " key ",
			dimension: 1536,
			want:      ErrEmbedderConfiguration,
			wantNot:   ErrEmbedderUnconfigured,
		},
		{
			name:         "missing key precedes nil transport",
			dimension:    1536,
			nilTransport: true,
			want:         ErrEmbedderUnconfigured,
			wantNot:      ErrEmbedderConfiguration,
		},
		{
			name:    "zero dimension",
			key:     "key",
			want:    ErrEmbedderConfiguration,
			wantNot: ErrEmbedderUnconfigured,
		},
		{
			name:      "dimension above model range",
			key:       "key",
			dimension: 3073,
			want:      ErrEmbedderConfiguration,
			wantNot:   ErrEmbedderUnconfigured,
		},
		{
			name:         "nil transport",
			key:          "key",
			dimension:    1536,
			nilTransport: true,
			want:         ErrEmbedderConfiguration,
			wantNot:      ErrEmbedderUnconfigured,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			transport := &recordingRoundTripper{}
			var roundTripper http.RoundTripper = transport
			if tt.nilTransport {
				roundTripper = nil
			}
			_, err := newGeminiWire(tt.key, tt.dimension, roundTripper)
			if !errors.Is(err, tt.want) {
				t.Fatalf("newGeminiWire error = %v, want %v", err, tt.want)
			}
			if errors.Is(err, tt.wantNot) {
				t.Fatalf("newGeminiWire error = %v, must not match %v", err, tt.wantNot)
			}
			if len(transport.requests) != 0 {
				t.Fatalf("misconfiguration sent %d requests", len(transport.requests))
			}
		})
	}
}

func TestNewGeminiEmbedderReportsMissingKeyBeforeAuthority(t *testing.T) {
	t.Parallel()

	_, err := newGeminiEmbedder(" \t ", testEmbeddingDimension, &recordingRoundTripper{}, nil)
	if !errors.Is(err, ErrEmbedderUnconfigured) {
		t.Fatalf("NewGeminiEmbedder error = %v, want ErrEmbedderUnconfigured", err)
	}
	for _, unexpected := range []error{
		ErrEmbedderConfiguration,
		ErrArtifactPolicyUnavailable,
		ErrPrivacyPolicyUnavailable,
	} {
		if errors.Is(err, unexpected) {
			t.Fatalf("NewGeminiEmbedder error = %v, must not match %v", err, unexpected)
		}
	}
}

func TestNewGeminiEmbedderKeepsAuthorityMisconfigurationDistinct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		want    error
		wantNot error
	}{
		{
			name:    "missing key precedes nil authority",
			want:    ErrEmbedderUnconfigured,
			wantNot: ErrEmbedderConfiguration,
		},
		{
			name:    "nil authority is invalid configuration",
			key:     "key",
			want:    ErrEmbedderConfiguration,
			wantNot: ErrEmbedderUnconfigured,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			transport := &recordingRoundTripper{}
			_, err := newGeminiEmbedder(tt.key, testEmbeddingDimension, transport, nil)
			if !errors.Is(err, tt.want) {
				t.Fatalf("newGeminiEmbedder error = %v, want %v", err, tt.want)
			}
			if errors.Is(err, tt.wantNot) {
				t.Fatalf("newGeminiEmbedder error = %v, must not match %v", err, tt.wantNot)
			}
			if len(transport.requests) != 0 {
				t.Fatalf("misconfigured authority sent %d requests", len(transport.requests))
			}
		})
	}
}

func TestNewIndexerWithTransportRejectsNilTransport(t *testing.T) {
	t.Parallel()

	_, err := NewIndexerWithTransport(nil, nil, nil)
	if !errors.Is(err, ErrEmbedderConfiguration) {
		t.Fatalf("NewIndexerWithTransport(nil) error = %v, want ErrEmbedderConfiguration", err)
	}
}

func TestGeminiWireOwnsTotalTimeoutAndRejectsRedirects(t *testing.T) {
	t.Parallel()

	wire, err := newGeminiWire("key", testEmbeddingDimension, &recordingRoundTripper{})
	if err != nil {
		t.Fatal(err)
	}
	if wire.client == nil || wire.client.Timeout != embeddingRequestTimeout {
		t.Fatalf("client timeout = %v, want %v", wire.client, embeddingRequestTimeout)
	}
	if err := wire.client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect() error = %v, want http.ErrUseLastResponse", err)
	}
}

func TestGeminiWirePreservesCallerCancellationAtHTTPBoundary(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	transport := &cancellationRoundTripper{}
	wire, err := newGeminiWire("key", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}

	_, err = wire.embedQuery(ctx, "query-sentinel")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("embedQuery error = %v, want context.Canceled", err)
	}
	if transport.calls != 1 {
		t.Fatalf("RoundTrip calls = %d, want exactly 1", transport.calls)
	}
}

type recordedRequest struct {
	method          string
	url             string
	key             string
	contentType     string
	idempotencyKey  string
	xIdempotencyKey string
	body            string
}

type recordingRoundTripper struct {
	requests []recordedRequest
	response *http.Response
	err      error
}

type cancellationRoundTripper struct {
	calls int
}

type responseRoundTripper struct {
	status int
	body   string
	calls  int
}

func (t *responseRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return response(t.status, t.body), nil
}

func (t *cancellationRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	t.calls++
	return nil, request.Context().Err()
}

func (t *recordingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	t.requests = append(t.requests, recordedRequest{
		method:          request.Method,
		url:             request.URL.String(),
		key:             request.Header.Get("X-Goog-Api-Key"),
		contentType:     request.Header.Get("Content-Type"),
		idempotencyKey:  request.Header.Get("Idempotency-Key"),
		xIdempotencyKey: request.Header.Get("X-Idempotency-Key"),
		body:            string(body),
	})
	return t.response, t.err
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d status", status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func validEmbeddingBody(promptTokens int) string {
	values := "1"
	if testEmbeddingDimension > 1 {
		values += strings.Repeat(",0", testEmbeddingDimension-1)
	}
	if promptTokens == 0 {
		return `{"embedding":{"values":[` + values + `]}}`
	}
	return fmt.Sprintf(`{"embedding":{"values":[%s]},"usageMetadata":{"promptTokenCount":%d}}`, values, promptTokens)
}
