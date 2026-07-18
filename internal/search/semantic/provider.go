package semantic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	geminiEmbeddingModel      = "gemini-embedding-2"
	geminiEmbeddingEndpoint   = "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-2:embedContent"
	minEmbeddingDimension     = 128
	maxEmbeddingDimension     = 3072
	maxEmbeddingResponseBytes = 8 << 20
	embeddingRequestTimeout   = 30 * time.Second
	queryPromptPrefix         = "task: search result | query: "
	protocolDescriptorVersion = "yomihon-gemini-embedding-2-protocol-v1"
	responseHandlingVersion   = "one-finite-float32-vector-no-client-normalization-v1"
)

// ErrEmbedderUnconfigured means the explicit semantic action has no provider
// credential. It is distinct from an invalid configured transport or protocol.
var ErrEmbedderUnconfigured = errors.New("embedding provider is not configured")

// ErrEmbedderConfiguration means a configured provider dependency or protocol
// value is invalid.
var ErrEmbedderConfiguration = errors.New("invalid embedding provider configuration")

// EmbedFailureKind is the total provider terminal taxonomy. Only
// EmbedFailureMalformedRequest and EmbedFailureInternal are owned by yomihon;
// all other values preserve the lexical answer while semantic search degrades.
type EmbedFailureKind string

// Provider failure kinds preserve fault ownership at the CLI boundary.
const (
	EmbedFailureUnreachable      EmbedFailureKind = "embedder-unreachable"
	EmbedFailureRateLimited      EmbedFailureKind = "rate-limited"
	EmbedFailureRejected         EmbedFailureKind = "embedder-rejected"
	EmbedFailureProvider         EmbedFailureKind = "embedder-failed"
	EmbedFailureMalformedRequest EmbedFailureKind = "confirmed-malformed"
	EmbedFailureInternal         EmbedFailureKind = "internal"
)

// EmbedError contains classification only. It never retains provider response
// text, request text, or a credential.
type EmbedError struct {
	Kind            EmbedFailureKind
	RetryAfter      time.Duration
	RetryAfterValid bool
}

func (e *EmbedError) Error() string {
	if e == nil {
		return "embedding failed"
	}
	switch e.Kind {
	case EmbedFailureUnreachable:
		return "embedding API did not answer"
	case EmbedFailureRateLimited:
		return "embedding API rate limited the request"
	case EmbedFailureRejected:
		return "embedding API rejected the credential"
	case EmbedFailureMalformedRequest:
		return "embedding request was malformed"
	case EmbedFailureInternal:
		return "embedding request could not be formed"
	default:
		return "embedding API returned an unrecoverable error"
	}
}

type geminiWire struct {
	apiKey    string
	dimension int
	client    *http.Client
}

type embeddingTransportFactory func() http.RoundTripper

func productionEmbeddingTransport() http.RoundTripper {
	return newProductionEmbeddingTransport()
}

// NewIndexerWithTransport constructs an indexer that sends the pinned
// provider request through transport. It is a hermetic-test seam, not a user
// configuration surface: the endpoint, protocol, and final-send authorization
// remain owned by this package, and the egress boundary test freezes callers.
func NewIndexerWithTransport(
	config *IndexerConfig,
	readAPIKey func() string,
	transport http.RoundTripper,
) (*Indexer, error) {
	if transport == nil {
		return nil, fmt.Errorf("%w: embedding transport is nil", ErrEmbedderConfiguration)
	}
	return newIndexer(config, readAPIKey, func() http.RoundTripper { return transport })
}

// geminiEmbedder owns the provider's HTTP boundary and the final chunk-egress
// authorization check. It performs no automatic retries.
type geminiEmbedder struct {
	wire      *geminiWire
	authorize chunkAuthorizer
}

func newProductionEmbeddingTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		// An environment proxy is an unruled second egress destination. Provider
		// traffic is direct until a proxy boundary is explicitly designed.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func newGeminiEmbedder(
	apiKey string,
	dimension int,
	transport http.RoundTripper,
	authorize chunkAuthorizer,
) (*geminiEmbedder, error) {
	wire, err := newGeminiWire(apiKey, dimension, transport)
	if err != nil {
		return nil, err
	}
	if authorize == nil {
		return nil, fmt.Errorf("%w: chunk authorizer is nil", ErrEmbedderConfiguration)
	}
	return &geminiEmbedder{wire: wire, authorize: authorize}, nil
}

// EmbedChunk owns the final-send choke. It re-binds the exact submitted
// bytes to their collection hash and asks current authority immediately before
// the single HTTP request.
func (e *geminiEmbedder) EmbedChunk(ctx context.Context, chunk *CorpusChunk) (EmbeddingResult, error) {
	if e == nil || e.wire == nil || e.authorize == nil ||
		chunk == nil ||
		sha256.Sum256(chunk.Submitted) != chunk.SubmittedHash {
		return EmbeddingResult{}, ErrChunkEgressDenied
	}
	identity := chunk.EgressIdentity()
	err := e.authorize(ctx, &identity)
	if err != nil {
		return EmbeddingResult{}, fmt.Errorf("authorize chunk egress: %w", err)
	}
	return e.wire.embedChunk(ctx, chunk.Submitted)
}

// EmbedQuery performs exactly one HTTP request. Applicability and explicit
// action gating happen before this method is reachable; an empty value remains
// a defensive no-send terminal.
func (e *geminiEmbedder) EmbedQuery(ctx context.Context, query string) (EmbeddingResult, error) {
	if e == nil || e.wire == nil {
		return EmbeddingResult{}, ErrQueryNotApplicable
	}
	return e.wire.embedQuery(ctx, query)
}

// QuerySubmissionHash returns the digest of the exact text submitted to the
// provider for a semantic query. Recorded vectors use it to prove that their
// query text still matches the current evaluation suite.
func QuerySubmissionHash(query string) ([sha256.Size]byte, error) {
	submitted, err := querySubmission(query)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256([]byte(submitted)), nil
}

func newGeminiWire(apiKey string, dimension int, transport http.RoundTripper) (*geminiWire, error) {
	err := validateEmbedderKey(apiKey)
	if err != nil {
		return nil, err
	}
	if dimension < minEmbeddingDimension || dimension > maxEmbeddingDimension {
		return nil, fmt.Errorf("%w: dimension is outside the model range", ErrEmbedderConfiguration)
	}
	if transport == nil {
		return nil, fmt.Errorf("%w: transport is nil", ErrEmbedderConfiguration)
	}
	return &geminiWire{
		apiKey:    apiKey,
		dimension: dimension,
		client: &http.Client{
			Transport: transport,
			Timeout:   embeddingRequestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func validateEmbedderKey(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("%w: API key is empty", ErrEmbedderUnconfigured)
	}
	if apiKey != strings.TrimSpace(apiKey) {
		return fmt.Errorf("%w: API key has surrounding whitespace", ErrEmbedderConfiguration)
	}
	return nil
}

func (w *geminiWire) embedChunk(ctx context.Context, submitted []byte) (EmbeddingResult, error) {
	if !utf8.Valid(submitted) {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureInternal}
	}
	return w.embed(ctx, string(submitted))
}

func (w *geminiWire) embedQuery(ctx context.Context, query string) (EmbeddingResult, error) {
	submitted, err := querySubmission(query)
	if err != nil {
		return EmbeddingResult{}, err
	}
	return w.embed(ctx, submitted)
}

func querySubmission(query string) (string, error) {
	err := validateQueryText(query)
	if err != nil {
		return "", err
	}
	return queryPromptPrefix + query, nil
}

func validateQueryText(query string) error {
	if strings.TrimSpace(query) == "" {
		return ErrQueryNotApplicable
	}
	if !utf8.ValidString(query) {
		return &EmbedError{Kind: EmbedFailureInternal}
	}
	return nil
}

type embedRequest struct {
	Content struct {
		Parts []embedPart `json:"parts"`
	} `json:"content"`
	Config embedConfig `json:"embedContentConfig"`
}

type embedPart struct {
	Text string `json:"text"`
}

type embedConfig struct {
	AutoTruncate         bool `json:"autoTruncate"`
	OutputDimensionality int  `json:"outputDimensionality"`
}

type embedResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}

type providerErrorEnvelope struct {
	Error struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
	} `json:"error"`
}

func (w *geminiWire) embed(ctx context.Context, text string) (EmbeddingResult, error) {
	requestBody := embedRequest{Config: embedConfig{AutoTruncate: false, OutputDimensionality: w.dimension}}
	requestBody.Content.Parts = []embedPart{{Text: text}}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureInternal}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, geminiEmbeddingEndpoint, bytes.NewReader(encoded))
	if err != nil {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureInternal}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Goog-Api-Key", w.apiKey)

	response, requestErr := w.client.Do(request)
	if requestErr != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close() //nolint:errcheck // requestErr is primary and this response cannot be consumed
		}
		contextErr := ctx.Err()
		if contextErr != nil {
			return EmbeddingResult{}, contextErr
		}
		if response != nil {
			return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureProvider}
		}
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureUnreachable}
	}
	if response == nil || response.Body == nil {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureProvider}
	}
	defer func() {
		_ = response.Body.Close() //nolint:errcheck // the fully read response or read error is the actionable transport result
	}()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxEmbeddingResponseBytes+1))
	if readErr != nil || len(body) > maxEmbeddingResponseBytes {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureProvider}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return EmbeddingResult{}, classifyProviderError(response, body)
	}
	return decodeEmbeddingResponse(body, w.dimension)
}

func classifyProviderError(response *http.Response, body []byte) error {
	var envelope providerErrorEnvelope
	if !decodeSingleJSON(body, &envelope) || envelope.Error.Code != response.StatusCode {
		return &EmbedError{Kind: EmbedFailureProvider}
	}
	kind := EmbedFailureProvider
	switch {
	case response.StatusCode == http.StatusTooManyRequests && envelope.Error.Status == "RESOURCE_EXHAUSTED":
		kind = EmbedFailureRateLimited
	case response.StatusCode == http.StatusUnauthorized && envelope.Error.Status == "UNAUTHENTICATED":
		kind = EmbedFailureRejected
	case response.StatusCode == http.StatusForbidden && envelope.Error.Status == "PERMISSION_DENIED":
		kind = EmbedFailureRejected
	case response.StatusCode == http.StatusBadRequest && envelope.Error.Status == "INVALID_ARGUMENT":
		kind = EmbedFailureMalformedRequest
	}
	retryAfter, retryAfterValid := parseRetryAfter(response.Header.Get("Retry-After"), time.Now())
	return &EmbedError{Kind: kind, RetryAfter: retryAfter, RetryAfterValid: retryAfterValid}
}

func decodeEmbeddingResponse(body []byte, dimension int) (EmbeddingResult, error) {
	var response embedResponse
	if !decodeSingleJSON(body, &response) || len(response.Embedding.Values) != dimension {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureProvider}
	}
	vector := make([]float32, dimension)
	for i, value := range response.Embedding.Values {
		converted := float32(value)
		if math.IsNaN(value) || math.IsInf(value, 0) || math.IsNaN(float64(converted)) || math.IsInf(float64(converted), 0) {
			return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureProvider}
		}
		vector[i] = converted
	}
	if _, ok := vectorMagnitude(vector); !ok {
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureProvider}
	}
	return EmbeddingResult{Vector: vector}, nil
}

func decodeSingleJSON(body []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	err := decoder.Decode(target)
	if err != nil {
		return false
	}
	var trailing any
	return decoder.Decode(&trailing) == io.EOF
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 &&
		seconds <= math.MaxInt64/int64(time.Second) {
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	return max(time.Duration(0), when.Sub(now)), true
}
