package semantic

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestGeminiWireHTTPBoundarySendsPinnedRequest(t *testing.T) {
	t.Parallel()

	type observedRequest struct {
		Method       string
		Host         string
		Path         string
		RawQuery     string
		ContentType  string
		Key          string
		Idempotency  string
		XIdempotency string
		Body         string
	}
	observed := make(chan observedRequest, 1)
	var requests atomic.Int64
	transport := geminiServerTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read request", http.StatusInternalServerError)
			return
		}
		observed <- observedRequest{
			Method:       r.Method,
			Host:         r.Host,
			Path:         r.URL.EscapedPath(),
			RawQuery:     r.URL.RawQuery,
			ContentType:  r.Header.Get("Content-Type"),
			Key:          r.Header.Get("X-Goog-Api-Key"),
			Idempotency:  r.Header.Get("Idempotency-Key"),
			XIdempotency: r.Header.Get("X-Idempotency-Key"),
			Body:         string(body),
		}
		if _, err := io.WriteString(w, validEmbeddingBody(0)); err != nil {
			return
		}
	}))
	wire, err := newGeminiWire("key-sentinel", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := wire.embedQuery(t.Context(), "query-sentinel"); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests = %d, want 1", got)
	}
	got := <-observed
	want := observedRequest{
		Method:      http.MethodPost,
		Host:        "generativelanguage.googleapis.com",
		Path:        "/v1beta/models/gemini-embedding-2:embedContent",
		ContentType: "application/json",
		Key:         "key-sentinel",
		Body:        `{"content":{"parts":[{"text":"task: search result | query: query-sentinel"}]},"embedContentConfig":{"autoTruncate":false,"outputDimensionality":128}}`,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("provider request mismatch (-want +got):\n%s", diff)
	}
}

func TestGeminiWireHTTPBoundaryDoesNotFollowRedirect(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	transport := geminiServerTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/redirected" {
			if _, err := io.WriteString(w, validEmbeddingBody(0)); err != nil {
				return
			}
			return
		}
		http.Redirect(
			w,
			r,
			"https://generativelanguage.googleapis.com/redirected",
			http.StatusFound,
		)
	}))
	wire, err := newGeminiWire("key-sentinel", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}

	_, err = wire.embedQuery(t.Context(), "query-sentinel")
	if !isEmbedFailure(err, EmbedFailureProvider) {
		t.Fatalf("embedQuery error = %v, want provider failure", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests = %d, want one redirect response and no follow", got)
	}
}

func TestGeminiWireHTTPBoundaryPreservesCallerCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseServer := func() {
		releaseOnce.Do(func() { close(release) })
	}
	var requests atomic.Int64
	transport := geminiServerTransport(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
		started <- struct{}{}
		<-release
	}))
	t.Cleanup(releaseServer)
	wire, err := newGeminiWire("key-sentinel", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, sendErr := wire.embedQuery(ctx, "query-sentinel")
		result <- sendErr
	}()

	awaitSignal(t, started, "provider request")
	cancel()
	if err := awaitError(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("embedQuery error = %v, want context.Canceled", err)
	}
	releaseServer()
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests = %d, want 1", got)
	}
}

func TestGeminiWireHTTPBoundaryEnforcesTotalTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		transport := &timeoutRoundTripper{}
		wire, err := newGeminiWire("key-sentinel", testEmbeddingDimension, transport)
		if err != nil {
			t.Fatal(err)
		}
		wire.client.Timeout = time.Second
		started := time.Now()

		_, err = wire.embedQuery(t.Context(), "query-sentinel")
		if !isEmbedFailure(err, EmbedFailureUnreachable) {
			t.Fatalf("embedQuery error = %v, want unreachable after total timeout", err)
		}
		if elapsed := time.Since(started); elapsed != time.Second {
			t.Fatalf("request elapsed = %v, want 1s client timeout", elapsed)
		}
		if got := transport.requests.Load(); got != 1 {
			t.Fatalf("HTTP requests = %d, want 1", got)
		}
	})
}

type timeoutRoundTripper struct {
	requests atomic.Int64
}

func (t *timeoutRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	t.requests.Add(1)
	<-request.Context().Done()
	return nil, request.Context().Err()
}

func TestGeminiWireHTTPBoundaryRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	body := validEmbeddingBody(0)
	body += strings.Repeat(" ", maxEmbeddingResponseBytes+1-len(body))
	var requests atomic.Int64
	transport := geminiServerTransport(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if _, err := io.WriteString(w, body); err != nil {
			return
		}
	}))
	wire, err := newGeminiWire("key-sentinel", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}

	_, err = wire.embedQuery(t.Context(), "query-sentinel")
	if !isEmbedFailure(err, EmbedFailureProvider) {
		t.Fatalf("embedQuery error = %v, want provider failure", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests = %d, want 1", got)
	}
}

func TestReadySearchConcurrentCallsSendOneHTTPRequest(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	transport := geminiServerTransport(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if _, err := io.WriteString(w, validEmbeddingBody(0)); err != nil {
			return
		}
	}))
	wire, err := newGeminiWire("key-sentinel", testEmbeddingDimension, transport)
	if err != nil {
		t.Fatal(err)
	}
	vector := make([]float32, testEmbeddingDimension)
	vector[0] = 1
	row := ChunkVector{
		RelPath:       "Writing/public.md",
		NoteHash:      sha256.Sum256([]byte("note")),
		SubmittedHash: sha256.Sum256([]byte("submitted")),
		Vector:        vector,
	}
	row = bindChunkVector(&row)
	index, err := NewIndex([]ChunkVector{row}, testEmbeddingDimension)
	if err != nil {
		t.Fatal(err)
	}
	ready := &ReadySearch{state: &readySearchState{
		index: index,
		query: queryCapability{
			validate: func(context.Context) error { return nil },
			send:     wire.embedQuery,
		},
	}}
	start := make(chan struct{})
	results := make(chan error, 2)
	var callers sync.WaitGroup
	for _, query := range []string{"first query", "second query"} {
		callers.Go(func() {
			<-start
			_, searchErr := ready.Search(t.Context(), query, nil, 1)
			results <- searchErr
		})
	}
	close(start)
	callers.Wait()
	close(results)

	var succeeded, alreadyAttempted int
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, ErrQueryAlreadyAttempted):
			alreadyAttempted++
		default:
			t.Errorf("Search() error = %v, want success or ErrQueryAlreadyAttempted", result)
		}
	}
	if succeeded != 1 || alreadyAttempted != 1 {
		t.Errorf("Search() outcomes = success:%d attempted:%d, want 1/1", succeeded, alreadyAttempted)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests = %d, want exactly 1", got)
	}
}

func geminiServerTransport(t *testing.T, handler http.Handler) *http.Transport {
	t.Helper()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("httptest transport = %T, want *http.Transport", server.Client().Transport)
	}
	transport = transport.Clone()
	certificate := server.Certificate()
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	tlsConfig := transport.TLSClientConfig.Clone()
	tlsConfig.InsecureSkipVerify = false
	tlsConfig.RootCAs = roots
	switch {
	case len(certificate.DNSNames) > 0:
		tlsConfig.ServerName = certificate.DNSNames[0]
	case len(certificate.IPAddresses) > 0:
		tlsConfig.ServerName = certificate.IPAddresses[0].String()
	default:
		t.Fatal("httptest certificate has no subject alternative name")
	}
	transport.TLSClientConfig = tlsConfig
	dialer := &net.Dialer{}
	address := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", address)
	}
	t.Cleanup(transport.CloseIdleConnections)
	return transport
}

func awaitSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func awaitError(t *testing.T, result <-chan error) error {
	t.Helper()

	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for provider result")
		return nil
	}
}
