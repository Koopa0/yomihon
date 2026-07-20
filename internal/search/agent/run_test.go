package agent

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/koopa0/yomihon/internal/search/semantic"
)

type recordedEmbeddingRequest struct {
	Host string
	Path string
	Key  string
	Text string
}

func TestRunCommandBuildThenSemanticSearch(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	cache := isolateUserCache(t, t.TempDir())
	recorder := newEmbeddingRecorder(t)
	keyReads := 0
	readKey := func() string {
		keyReads++
		return "key-sentinel"
	}

	var buildOut, buildErr bytes.Buffer
	command := CLI{
		Root:             root,
		Stdout:           &buildOut,
		Stderr:           &buildErr,
		ReadEmbeddingKey: readKey,
	}
	newIndexer := func(config *semantic.IndexerConfig, readKey func() string) (*semantic.Indexer, error) {
		return semantic.NewIndexerWithTransport(config, readKey, recorder.transport)
	}
	buildExit := runCommand(
		t.Context(),
		"search-index",
		[]string{"build"},
		command,
		newIndexer,
	)
	if buildExit != 0 {
		t.Fatalf("search-index build exit = %d, want 0; stderr = %q", buildExit, buildErr.String())
	}
	if got, want := buildOut.String(), "semantic index built: 1 chunks (1 embedded, 0 reused)\n"; got != want {
		t.Errorf("search-index build stdout = %q, want %q", got, want)
	}
	if got, want := buildErr.String(), "yomihon search-index: embedded 1/1 chunks\n"; got != want {
		t.Errorf("search-index build stderr = %q, want %q", got, want)
	}
	storePath := filepath.Join(cache, "yomihon", "semantic", "generation.sqlite")
	if info, err := filepath.Glob(storePath + "*"); err != nil || len(info) == 0 {
		t.Fatalf("semantic generation files = %v, error = %v, want store at %q", info, err, storePath)
	}

	var searchOut, searchErr bytes.Buffer
	command.Stdout = &searchOut
	command.Stderr = &searchErr
	searchExit := runCommand(
		t.Context(),
		"search",
		[]string{"--json", "--semantic", "token"},
		command,
		newIndexer,
	)
	if searchExit != 0 {
		t.Fatalf("semantic search exit = %d, want 0; stderr = %q", searchExit, searchErr.String())
	}
	const wantSearch = "{\"mode\":\"hybrid\",\"semantic\":\"ok\",\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\",\"semantic\"],\"channel_ranks\":{\"lexical\":1,\"semantic\":1}}]}\n"
	if got := searchOut.String(); got != wantSearch {
		t.Errorf("semantic search stdout = %q, want %q", got, wantSearch)
	}
	if searchErr.Len() != 0 {
		t.Errorf("semantic search stderr = %q, want empty", searchErr.Bytes())
	}
	if keyReads != 2 {
		t.Errorf("embedding key reads = %d, want one build read and one query-action read", keyReads)
	}

	gotRequests := recorder.requests()
	wantRequests := []recordedEmbeddingRequest{
		{
			Host: "generativelanguage.googleapis.com",
			Path: "/v1beta/models/gemini-embedding-2:embedContent",
			Key:  "key-sentinel",
			Text: "title: Public | text: body token",
		},
		{
			Host: "generativelanguage.googleapis.com",
			Path: "/v1beta/models/gemini-embedding-2:embedContent",
			Key:  "key-sentinel",
			Text: "task: search result | query: token",
		},
	}
	if len(gotRequests) != len(wantRequests) {
		t.Fatalf("embedding HTTP requests = %d, want %d: %#v", len(gotRequests), len(wantRequests), gotRequests)
	}
	for i := range wantRequests {
		if gotRequests[i] != wantRequests[i] {
			t.Errorf("embedding HTTP request %d = %#v, want %#v", i, gotRequests[i], wantRequests[i])
		}
	}
}

type embeddingRecorder struct {
	transport *http.Transport
	mu        sync.Mutex
	seen      []recordedEmbeddingRequest
}

func newEmbeddingRecorder(t *testing.T) *embeddingRecorder {
	t.Helper()

	recorder := &embeddingRecorder{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(w, "read request", http.StatusInternalServerError)
			return
		}
		var envelope struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Content.Parts) != 1 {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		recorder.mu.Lock()
		recorder.seen = append(recorder.seen, recordedEmbeddingRequest{
			Host: request.Host,
			Path: request.URL.EscapedPath(),
			Key:  request.Header.Get("X-Goog-Api-Key"),
			Text: envelope.Content.Parts[0].Text,
		})
		recorder.mu.Unlock()
		if _, err := io.WriteString(w, embeddingResponse(1536)); err != nil {
			t.Errorf("write embedding response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	base, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("httptest transport = %T, want *http.Transport", server.Client().Transport)
	}
	transport := base.Clone()
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
	recorder.transport = transport
	return recorder
}

func (r *embeddingRecorder) requests() []recordedEmbeddingRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEmbeddingRequest(nil), r.seen...)
}

func embeddingResponse(dimension int) string {
	return `{"embedding":{"values":[1` + strings.Repeat(",0", dimension-1) + `]}}`
}
