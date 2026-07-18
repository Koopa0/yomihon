package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestCollectEmbeddingDocumentsUsesBothCapabilities(t *testing.T) {
	t.Parallel()

	artifact, privacy := fixtureEgressPolicies(t)
	sources := []SourceNote{
		fixtureSourceNote("Writing/public.md", "Public", "public marker"),
		fixtureSourceNote("System/templates/card.md", "Template", "artifact marker"),
		fixtureSourceNote("Private/local.md", "Local", "private marker"),
	}
	sources[0].Status = "ready"
	got, err := collectCorpusChunks(sources, artifact, privacy, 100)
	if err != nil {
		t.Fatalf("collectCorpusChunks: %v", err)
	}
	if len(got.Failures) != 0 {
		t.Fatalf("collection failures = %+v, want none", got.Failures)
	}
	if len(got.Chunks) != 1 {
		t.Fatalf("documents = %+v, want public document only", got.Chunks)
	}
	document := got.Chunks[0]
	if document.RelPath != "Writing/public.md" || document.NoteHash != sources[0].NoteHash {
		t.Errorf("public document identity = %+v", document)
	}
	if document.Title != "Public" || document.Status != "ready" {
		t.Errorf("current display evidence = title %q status %q", document.Title, document.Status)
	}
	if string(document.Submitted) != "title: Public | text: public marker" {
		t.Errorf("submitted bytes = %q", document.Submitted)
	}
	if document.SubmittedHash != sha256.Sum256(document.Submitted) {
		t.Error("submitted hash does not bind exact submitted bytes")
	}
}

func TestReadCorpusBuildsOnlyEligibleCurrentDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	writeCorpusNote(t, root, "Writing/public.md", "---\ntitle: Public\ntype: concept\nstatus: ready\n---\n\n## Head\n\npublic marker\n")
	writeCorpusNote(t, root, "System/templates/card.md", "template marker")
	writeCorpusNote(t, root, "Private/local.md", "private marker")

	corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Chunks) != 1 {
		t.Fatalf("readCorpus() documents = %+v, want one eligible document", corpus.Chunks)
	}
	document := corpus.Chunks[0]
	if document.RelPath != "Writing/public.md" || document.Ordinal != 0 {
		t.Errorf("document identity = %+v", document)
	}
	if document.Title != "Public" || document.Status != "ready" {
		t.Errorf("current display evidence = title %q status %q", document.Title, document.Status)
	}
	if string(document.Submitted) != "title: Public › Head | text: public marker" {
		t.Errorf("submitted bytes = %q", document.Submitted)
	}
	if document.Body != "public marker" || len(document.Headings) != 1 || document.Headings[0] != "Head" {
		t.Errorf("local evidence = body %q headings %q", document.Body, document.Headings)
	}
	if document.ProxyTokens != ProxyTokens(string(document.Submitted)) || corpus.ProxyTokens != document.ProxyTokens {
		t.Errorf("proxy tokens = document %d corpus %d", document.ProxyTokens, corpus.ProxyTokens)
	}
	target, err := document.Target()
	if err != nil {
		t.Fatal(err)
	}
	wantFingerprint, err := TargetFingerprint([]ChunkTarget{target})
	if err != nil {
		t.Fatal(err)
	}
	if corpus.Fingerprint != wantFingerprint {
		t.Errorf("corpus fingerprint = %x, want %x", corpus.Fingerprint, wantFingerprint)
	}

	writeCorpusNote(t, root, "Writing/public.md", "---\ntitle: Public\ntype: concept\nstatus: ready\n---\n\n## Head\n\nchanged marker\n")
	changed, err := readCorpus(t.Context(), reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Fingerprint == corpus.Fingerprint {
		t.Error("content change did not change corpus fingerprint")
	}
}

func TestReadCorpusStaysBoundToTheVaultThatSuppliedItsPolicies(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	reader, artifact, privacy := writeCorpusContract(t, root)
	writeCorpusNote(t, root, "Writing/public.md", "original marker")

	selected := filepath.Join(parent, "selected-vault")
	if err := os.Rename(root, selected); err != nil {
		t.Fatal(err)
	}
	writeCorpusContractFile(t, root)
	writeCorpusNote(t, root, "Writing/public.md", "replacement marker")

	corpus, err := readCorpus(
		t.Context(),
		reader,
		artifact,
		privacy,
		100,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Chunks) != 1 || corpus.Chunks[0].Body != "original marker" {
		t.Fatalf("readCorpus() chunks = %+v, want bytes from the policy-owning vault", corpus.Chunks)
	}
}

func TestReadCorpusFailsClosedWhenPolicySourceChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	writeCorpusNote(t, root, "Writing/public.md", "body")
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	file, err := os.OpenFile(contractPath, os.O_APPEND|os.O_WRONLY, 0) // #nosec G304 -- contractPath is rooted in t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n# changed after action start\n"); err != nil {
		closeNow(t, file)
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 100); !errors.Is(err, ErrPolicySourceChanged) || len(corpus.Chunks) != 0 {
		t.Fatalf("readCorpus() after source drift = (%+v, %v), want empty ErrPolicySourceChanged", corpus, err)
	}
}

func TestReadCorpusHonorsCancellationBeforeFilesystemIO(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	corpus, err := readCorpus(ctx, reader, artifact, privacy, 100)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("readCorpus() error = %v, want context.Canceled", err)
	}
	if len(corpus.Chunks) != 0 {
		t.Fatalf("readCorpus() documents = %d, want none", len(corpus.Chunks))
	}
}

func TestCollectEmbeddingDocumentsFailsClosedWhenEitherCapabilityUnavailable(t *testing.T) {
	t.Parallel()

	artifact, privacy := fixtureEgressPolicies(t)
	sources := []SourceNote{fixtureSourceNote("Writing/public.md", "Public", "body")}
	tests := []struct {
		name     string
		artifact schema.ArtifactPolicy
		privacy  schema.PrivacyPolicy
		wantErr  error
	}{
		{name: "artifact", privacy: privacy, wantErr: ErrArtifactPolicyUnavailable},
		{name: "privacy", artifact: artifact, wantErr: ErrPrivacyPolicyUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := collectCorpusChunks(sources, tt.artifact, tt.privacy, 100)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if len(got.Chunks) != 0 {
				t.Fatalf("unavailable capability returned %d documents", len(got.Chunks))
			}
		})
	}
}

func TestGeminiEmbedderRevalidatesAtFinalSend(t *testing.T) {
	t.Parallel()

	document := CorpusChunk{
		RelPath:       "Writing/public.md",
		NoteHash:      sha256.Sum256([]byte("note")),
		Ordinal:       0,
		Submitted:     []byte("sentinel document bytes"),
		SubmittedHash: sha256.Sum256([]byte("sentinel document bytes")),
	}
	tests := []struct {
		name      string
		authorize chunkAuthorizer
		wantCalls int
		wantErr   error
	}{
		{name: "current", authorize: func(context.Context, *ChunkIdentity) error { return nil }, wantCalls: 1},
		{name: "reclassified", authorize: func(context.Context, *ChunkIdentity) error { return ErrChunkEgressDenied }, wantErr: ErrChunkEgressDenied},
		{
			name: "hash changed",
			authorize: func(_ context.Context, got *ChunkIdentity) error {
				if got == nil || got.NoteHash != sha256.Sum256([]byte("other")) {
					return ErrPolicySourceChanged
				}
				return nil
			},
			wantErr: ErrPolicySourceChanged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			providerResponse := response(
				http.StatusOK,
				validEmbeddingBody(0),
			)
			t.Cleanup(func() {
				if closeErr := providerResponse.Body.Close(); closeErr != nil {
					t.Errorf("close provider response: %v", closeErr)
				}
			})
			transport := &recordingRoundTripper{response: providerResponse}
			embedder, err := newGeminiEmbedder("key-sentinel", testEmbeddingDimension, transport, tt.authorize)
			if err != nil {
				t.Fatal(err)
			}
			_, err = embedder.EmbedChunk(t.Context(), &document)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("EmbedChunk error = %v, want %v", err, tt.wantErr)
			}
			if len(transport.requests) != tt.wantCalls {
				t.Fatalf("HTTP requests = %d, want %d", len(transport.requests), tt.wantCalls)
			}
		})
	}
}

func TestDocumentAuthorizerBindsProviderBytesToCurrentEligibleNote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	writeCorpusNote(t, root, "Writing/public.md", "---\ntitle: Public\ntype: concept\n---\n\nbody\n")
	corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := newChunkAuthorizer(reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	providerResponse := response(
		http.StatusOK,
		validEmbeddingBody(0),
	)
	t.Cleanup(func() {
		if closeErr := providerResponse.Body.Close(); closeErr != nil {
			t.Errorf("close provider response: %v", closeErr)
		}
	})
	transport := &recordingRoundTripper{response: providerResponse}
	embedder, err := newGeminiEmbedder("key-sentinel", testEmbeddingDimension, transport, authorize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.EmbedChunk(t.Context(), &corpus.Chunks[0]); err != nil {
		t.Fatalf("EmbedChunk(current): %v", err)
	}

	forged := corpus.Chunks[0]
	forged.Submitted = []byte("bytes that never came from the eligible note")
	forged.SubmittedHash = sha256.Sum256(forged.Submitted)
	if _, err := embedder.EmbedChunk(t.Context(), &forged); !errors.Is(err, ErrSourceNoteChanged) {
		t.Fatalf("EmbedChunk(forged) error = %v, want ErrDocumentSourceChanged", err)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("HTTP requests = %d, want only the authorized send", len(transport.requests))
	}
}

func TestDocumentAuthorizerRejectsSymlinkedSourceEvenWhenBytesMatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	const body = "---\ntitle: Public\ntype: concept\n---\n\nbody\n"
	writeCorpusNote(t, root, "Writing/public.md", body)
	corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := newChunkAuthorizer(reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}

	private := filepath.Join(t.TempDir(), "private.md")
	if writeErr := os.WriteFile(private, []byte(body), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	public := filepath.Join(root, "Writing", "public.md")
	if removeErr := os.Remove(public); removeErr != nil {
		t.Fatal(removeErr)
	}
	if linkErr := os.Symlink(private, public); linkErr != nil {
		t.Fatal(linkErr)
	}

	transport := &recordingRoundTripper{}
	embedder, err := newGeminiEmbedder("key-sentinel", testEmbeddingDimension, transport, authorize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.EmbedChunk(t.Context(), &corpus.Chunks[0]); !errors.Is(err, ErrSourceNoteChanged) {
		t.Fatalf("EmbedChunk(symlink replacement) error = %v, want ErrSourceNoteChanged", err)
	}
	if len(transport.requests) != 0 {
		t.Fatalf("symlinked source sent %d HTTP requests, want 0", len(transport.requests))
	}
}

func TestDocumentAuthorizerRejectsRegularSourceReplacementEvenWhenBytesMatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	const body = "---\ntitle: Public\ntype: concept\n---\n\nbody\n"
	writeCorpusNote(t, root, "Writing/public.md", body)
	corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := newChunkAuthorizer(reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}

	public := filepath.Join(root, "Writing", "public.md")
	if renameErr := os.Rename(public, filepath.Join(root, "Writing", "observed.md")); renameErr != nil {
		t.Fatal(renameErr)
	}
	if writeErr := os.WriteFile(public, []byte(body), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	transport := &recordingRoundTripper{}
	embedder, err := newGeminiEmbedder("key-sentinel", testEmbeddingDimension, transport, authorize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.EmbedChunk(t.Context(), &corpus.Chunks[0]); !errors.Is(err, ErrSourceNoteChanged) {
		t.Fatalf("EmbedChunk(regular replacement) error = %v, want ErrSourceNoteChanged", err)
	}
	if len(transport.requests) != 0 {
		t.Fatalf("replaced source sent %d HTTP requests, want 0", len(transport.requests))
	}
}

func TestDocumentAuthorizerPreservesPolicySourceDriftReason(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := writeCorpusContract(t, root)
	writeCorpusNote(t, root, "Writing/public.md", "body")
	corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	authorize, err := newChunkAuthorizer(reader, artifact, privacy, 100)
	if err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	contract, err := os.ReadFile(contractPath) // #nosec G304 -- contractPath is rooted in t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(contractPath, append(contract, []byte("\n# drift\n")...), 0o600); writeErr != nil { // #nosec G703 -- contractPath is rooted in t.TempDir
		t.Fatal(writeErr)
	}
	transport := &recordingRoundTripper{}
	embedder, err := newGeminiEmbedder("key-sentinel", testEmbeddingDimension, transport, authorize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.EmbedChunk(t.Context(), &corpus.Chunks[0]); !errors.Is(err, ErrPolicySourceChanged) {
		t.Fatalf("EmbedChunk() error = %v, want ErrPolicySourceChanged", err)
	}
	if len(transport.requests) != 0 {
		t.Fatalf("policy drift sent %d HTTP requests, want 0", len(transport.requests))
	}
}

func TestGeminiEmbedderQueryIsOneHTTPRequest(t *testing.T) {
	t.Parallel()

	providerResponse := response(
		http.StatusOK,
		validEmbeddingBody(0),
	)
	t.Cleanup(func() {
		if closeErr := providerResponse.Body.Close(); closeErr != nil {
			t.Errorf("close provider response: %v", closeErr)
		}
	})
	transport := &recordingRoundTripper{response: providerResponse}
	embedder, err := newGeminiEmbedder(
		"key-sentinel",
		testEmbeddingDimension,
		transport,
		func(context.Context, *ChunkIdentity) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.EmbedQuery(t.Context(), "sentinel query bytes"); err != nil {
		t.Fatal(err)
	}
	if len(transport.requests) != 1 {
		t.Fatalf("HTTP requests = %d, want exactly 1", len(transport.requests))
	}
}

func TestProductionEmbeddingTransportDoesNotUseEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:8080")

	transport := newProductionEmbeddingTransport()
	if transport == http.DefaultTransport {
		t.Fatal("production embedding transport aliases http.DefaultTransport")
	}
	if transport.Proxy != nil {
		t.Fatal("production embedding transport can route provider bytes through an environment proxy")
	}
}

func TestGeminiEmbedderRejectsInvalidInputsBeforeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*geminiEmbedder) error
		want error
	}{
		{
			name: "nil document",
			call: func(embedder *geminiEmbedder) error {
				_, err := embedder.EmbedChunk(t.Context(), nil)
				return err
			},
			want: ErrChunkEgressDenied,
		},
		{
			name: "empty query",
			call: func(embedder *geminiEmbedder) error {
				_, err := embedder.EmbedQuery(t.Context(), " \t ")
				return err
			},
			want: ErrQueryNotApplicable,
		},
		{
			name: "invalid utf8 query",
			call: func(embedder *geminiEmbedder) error {
				_, err := embedder.EmbedQuery(t.Context(), string([]byte{0xff}))
				return err
			},
		},
		{
			name: "submitted hash mismatch",
			call: func(embedder *geminiEmbedder) error {
				document := CorpusChunk{
					RelPath:       "Writing/public.md",
					NoteHash:      sha256.Sum256([]byte("note")),
					Submitted:     []byte("changed after collection"),
					SubmittedHash: sha256.Sum256([]byte("original collection bytes")),
				}
				_, err := embedder.EmbedChunk(t.Context(), &document)
				return err
			},
			want: ErrChunkEgressDenied,
		},
		{
			name: "invalid utf8 document",
			call: func(embedder *geminiEmbedder) error {
				submitted := []byte{0xff}
				document := CorpusChunk{
					RelPath:       "Writing/public.md",
					NoteHash:      sha256.Sum256([]byte("note")),
					Submitted:     submitted,
					SubmittedHash: sha256.Sum256(submitted),
				}
				_, err := embedder.EmbedChunk(t.Context(), &document)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			transport := &recordingRoundTripper{}
			embedder, err := newGeminiEmbedder(
				"key-sentinel",
				testEmbeddingDimension,
				transport,
				func(context.Context, *ChunkIdentity) error { return nil },
			)
			if err != nil {
				t.Fatal(err)
			}
			err = tt.call(embedder)
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("call error = %v, want %v", err, tt.want)
			}
			if err == nil {
				t.Fatal("call error = nil, want local rejection")
			}
			if len(transport.requests) != 0 {
				t.Fatalf("rejected input sent %d HTTP requests, want 0", len(transport.requests))
			}
		})
	}
}

func fixtureSourceNote(relPath, title, body string) SourceNote {
	return SourceNote{
		RelPath:  relPath,
		Title:    title,
		Body:     body,
		NoteHash: sha256.Sum256([]byte(relPath + "\x00" + body)),
	}
}

func fixtureEgressPolicies(t *testing.T) (schema.ArtifactPolicy, schema.PrivacyPolicy) {
	t.Helper()
	root := t.TempDir()
	contract, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	contract = append(contract, []byte("\n[privacy]\nnever_egress_dirs = [\"Private\"]\n")...)
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(contractPath), 0o750); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(contractPath, contract, 0o600); writeErr != nil { // #nosec G703 -- contractPath is rooted in t.TempDir
		t.Fatal(writeErr)
	}
	loaded, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return loaded.ArtifactPolicy(), loaded.PrivacyPolicy()
}

func writeCorpusContract(t *testing.T, root string) (*vault.Reader, schema.ArtifactPolicy, schema.PrivacyPolicy) {
	t.Helper()
	writeCorpusContractFile(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("close vault reader: %v", closeErr)
		}
	})
	loaded, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal(err)
	}
	return reader, loaded.ArtifactPolicy(), loaded.PrivacyPolicy()
}

func writeCorpusContractFile(t *testing.T, root string) {
	t.Helper()
	contract, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	contract = append(contract, []byte("\n[privacy]\nnever_egress_dirs = [\"Private\"]\n")...)
	writeCorpusNote(t, root, schema.ContractRelPath, string(contract))
}

func writeCorpusNote(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(path, []byte(content), 0o600); writeErr != nil { // #nosec G703 -- path is rooted in a caller-owned t.TempDir
		t.Fatal(writeErr)
	}
}
