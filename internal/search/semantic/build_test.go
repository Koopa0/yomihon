package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func newTestIndexer(setup *indexerSetup, send chunkSender, now buildClock, wait buildWait) (*Indexer, error) {
	if send == nil {
		return nil, fmt.Errorf("%w: chunk sender is nil", ErrEmbedderConfiguration)
	}
	return newTestIndexerWithSender(setup, func() (chunkSender, error) { return send, nil }, now, wait)
}

func newTestIndexerWithSender(
	setup *indexerSetup,
	openSender func() (chunkSender, error),
	now buildClock,
	wait buildWait,
) (*Indexer, error) {
	if openSender == nil {
		return nil, fmt.Errorf("%w: chunk sender factory is nil", ErrEmbedderConfiguration)
	}
	return newIndexerWithProvider(setup, func() (*actionProvider, error) {
		send, err := openSender()
		if err != nil {
			return nil, err
		}
		return &actionProvider{
			embedChunk: send,
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				return EmbeddingResult{}, ErrQueryNotApplicable
			},
		}, nil
	}, now, wait)
}

func TestIndexerCurrentGenerationSendsNothing(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	indexBuilds := 0
	config.deps.newIndex = func(rows []ChunkVector, dimension int) (*Index, error) {
		indexBuilds++
		return NewIndex(rows, dimension)
	}
	var sent []CorpusChunk
	factoryCalls := 0
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		factoryCalls++
		return &actionProvider{
			embedChunk: func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
				sent = append(sent, *document)
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
			embedQuery: func(_ context.Context, _ string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildCurrent || got.Chunks != 1 || got.Embedded != 0 || got.Reused != 1 {
		t.Errorf("Build() = %+v, want current 1 chunk with zero sends", got)
	}
	if len(sent) != 0 || factoryCalls != 0 {
		t.Fatalf("Build() used provider for a current generation: documents=%d factories=%d, want zero", len(sent), factoryCalls)
	}
	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(sent) != 0 || factoryCalls != 1 {
		t.Fatalf("Reconcile() sent documents=%d after factories=%d, want zero/one", len(sent), factoryCalls)
	}
	ranked, rankErr := ready.Search(t.Context(), "shared provider", nil, 1)
	if rankErr != nil {
		t.Fatal(rankErr)
	}
	if len(ranked) != 1 || ranked[0].RelPath != corpus.Chunks[0].RelPath {
		t.Fatalf("TopNotes() = %+v, want retained current index", ranked)
	}
	if querySends != 1 || factoryCalls != 1 || indexBuilds != 1 {
		t.Fatalf("query sends=%d factories=%d index builds=%d, want one action-owned provider and retained index", querySends, factoryCalls, indexBuilds)
	}
}

func TestIndexerBuildReportsPersistedProgressAtHundredsAndCompletion(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 205)
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got [][2]int
	indexer.buildProgress = func(embedded, total int) error {
		got = append(got, [2]int{embedded, total})
		return nil
	}

	report, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if report.Embedded != 205 {
		t.Errorf("Build().Embedded = %d, want 205", report.Embedded)
	}
	want := [][2]int{{100, 205}, {200, 205}, {205, 205}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("build progress mismatch (-want +got):\n%s", diff)
	}
}

func TestIndexerReconcileKeepsTheAgentSnapshotThroughQuerySend(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	reads := 0
	config.deps.readCorpus = func(context.Context) (Corpus, error) {
		reads++
		return corpus, nil
	}
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("current generation sent a document")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 0 {
		t.Fatalf("Reconcile() performed %d independent corpus reads, want zero", reads)
	}
	if _, err := ready.Search(t.Context(), "bound snapshot", nil, 1); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || querySends != 1 {
		t.Fatalf("Search() corpus reads=%d query sends=%d, want one final revalidation and one send", reads, querySends)
	}
}

func TestIndexerReconcileRejectsChangedSnapshotBeforeQuerySend(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 2)
	target := fixtureIndexerCorpus(t, corpus.Chunks[:1])
	changed := fixtureIndexerCorpus(t, corpus.Chunks[1:])
	seedActiveGeneration(t, &config, target)
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return changed, nil }
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("current generation sent a document")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := indexer.Reconcile(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ready.Search(t.Context(), "changed", nil, 1); !errors.Is(err, ErrVaultChanged) {
		t.Fatalf("Search() error = %v, want ErrVaultChanged", err)
	}
	if querySends != 0 {
		t.Fatalf("query sends = %d, want zero after corpus drift", querySends)
	}
}

func TestIndexerReconcileBuildsCurrentIndexBeforeProvider(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	indexCalls := 0
	config.deps.newIndex = func([]ChunkVector, int) (*Index, error) {
		indexCalls++
		return nil, ErrDimensionMismatch
	}
	factoryCalls := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		factoryCalls++
		return nil, errors.New("provider must remain dormant after index failure")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), corpus)
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("Reconcile() error = %v, want index validation failure", err)
	}
	if indexCalls != 1 || factoryCalls != 0 {
		t.Fatalf("index calls=%d provider factories=%d, want index before provider", indexCalls, factoryCalls)
	}
}

func TestIndexerReconcileCurrentRequiresConfigurationPreflight(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	factoryCalls := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		factoryCalls++
		return nil, fmt.Errorf("%w: key is absent", ErrEmbedderConfiguration)
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), corpus)
	if !errors.Is(err, ErrEmbedderConfiguration) {
		t.Fatalf("Reconcile() error = %v, want ErrEmbedderConfiguration", err)
	}
	if factoryCalls != 1 {
		t.Fatalf("provider preflight calls = %d, want one", factoryCalls)
	}
}

func TestIndexerPublishesCompleteColdGeneration(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 2)
	var sent []CorpusChunk
	factoryCalls := 0
	indexer, err := newTestIndexerWithSender(&config, func() (chunkSender, error) {
		factoryCalls++
		return func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
			sent = append(sent, *document)
			return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildPublished || got.Chunks != 2 || got.Embedded != 2 || got.Reused != 0 {
		t.Errorf("Build() = %+v, want built 2 chunks", got)
	}
	if len(sent) != 2 || factoryCalls != 1 {
		t.Fatalf("Build() sent %d documents, want 2", len(sent))
	}
	store, err := openStore(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, store)
	active, err := store.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if active.corpusFingerprint != corpus.Fingerprint || len(active.rows) != 2 {
		t.Errorf("active generation = %+v, want complete target corpus", active)
	}
}

func TestIndexerFullBuildUsesDurableFallbackRetrySchedule(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	now := time.Unix(1_000, 0)
	var delays []time.Duration
	sends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		if sends < 5 {
			return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureRateLimited}
		}
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		now = now.Add(delay)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	wantDelays := []time.Duration{time.Second, 4 * time.Second, 9 * time.Second, 16 * time.Second}
	if !slices.Equal(delays, wantDelays) {
		t.Errorf("retry delays = %v, want %v", delays, wantDelays)
	}
	if got.Status != BuildPublished || got.Embedded != 1 || sends != 5 {
		t.Errorf("Build() = %+v after %d sends, want one published chunk after five sends", got, sends)
	}
}

func TestIndexerReconcileRejectsOversizedDriftBeforeWriterOrEgress(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 130)
	activeCorpus := fixtureIndexerCorpus(t, corpus.Chunks[:1])
	seedActiveGeneration(t, &config, activeCorpus)
	writer, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	var sent []CorpusChunk
	factoryCalls := 0
	indexer, err := newTestIndexerWithSender(&config, func() (chunkSender, error) {
		factoryCalls++
		return func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
			sent = append(sent, *document)
			return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), corpus)
	if !errors.Is(err, ErrRebuildRequired) {
		t.Fatalf("Reconcile() error = %v, want ErrRebuildRequired", err)
	}
	if len(sent) != 0 || factoryCalls != 0 {
		t.Fatalf("Reconcile() sent %d documents above the bound, want zero", len(sent))
	}
}

func TestIndexerReconcileColdDoesNotOpenProviderFactory(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	factoryCalls := 0
	indexer, err := newTestIndexerWithSender(&config, func() (chunkSender, error) {
		factoryCalls++
		return nil, errors.New("provider factory should remain dormant")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = indexer.Reconcile(t.Context(), corpus)
	if !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("Reconcile() error = %v, want ErrStoreNotFound", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("provider factory calls = %d, want zero for cold reconcile", factoryCalls)
	}
}

func TestIndexerReconcileMismatchDoesNotOpenProviderFactory(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	stored := config
	identity, err := newGenerationIdentity(IdentityConfig{
		Dimension:               config.settings.identity.Dimension(),
		ChunkTokenCap:           257,
		VaultRoot:               config.settings.identity.VaultRoot(),
		CorpusPolicyFingerprint: config.settings.identity.CorpusPolicyFingerprint(),
	})
	if err != nil {
		t.Fatal(err)
	}
	stored.settings.identity = identity
	seedActiveGeneration(t, &stored, corpus)
	factoryCalls := 0
	indexer, err := newTestIndexerWithSender(&config, func() (chunkSender, error) {
		factoryCalls++
		return nil, errors.New("provider factory should remain dormant")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = indexer.Reconcile(t.Context(), corpus)
	if !errors.Is(err, ErrGenerationMismatch) {
		t.Fatalf("Reconcile() error = %v, want ErrGenerationMismatch", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("provider factory calls = %d, want zero for identity mismatch", factoryCalls)
	}
}

func TestIndexerReconcileClassifiesOnlyKnownRetiredModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*generationIdentity)
		wantError error
	}{
		{
			name: "known predecessor",
			mutate: func(identity *generationIdentity) {
				identity.model = "gemini-embedding-001"
			},
			wantError: ErrEmbedderRetired,
		},
		{
			name: "unknown model",
			mutate: func(identity *generationIdentity) {
				identity.model = "unknown-embedding-generation"
			},
			wantError: ErrGenerationMismatch,
		},
		{
			name: "current model protocol mismatch",
			mutate: func(identity *generationIdentity) {
				identity.protocolEpoch = sha256.Sum256([]byte("different protocol"))
			},
			wantError: ErrGenerationMismatch,
		},
		{
			name: "current model dimension mismatch",
			mutate: func(identity *generationIdentity) {
				identity.dimension++
			},
			wantError: ErrGenerationMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config, corpus := fixtureIndexerConfig(t, 1)
			stored := config
			tt.mutate(&stored.settings.identity)
			seedActiveGeneration(t, &stored, corpus)
			factoryCalls := 0
			indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
				factoryCalls++
				return nil, errors.New("identity terminal reached provider")
			}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}

			_, err = indexer.Reconcile(t.Context(), corpus)
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("Reconcile() error = %v, want %v", err, tt.wantError)
			}
			if factoryCalls != 0 {
				t.Fatalf("provider factory calls = %d, want zero", factoryCalls)
			}
		})
	}
}

func TestIndexerReconcilePublishesBoundedDriftWithOneAttemptPerMissingRow(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 3)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:2])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	var sent []CorpusChunk
	factoryCalls := 0
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		factoryCalls++
		return &actionProvider{
			embedChunk: func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
				sent = append(sent, *document)
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
			embedQuery: func(_ context.Context, _ string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, func(context.Context, time.Duration) error {
		t.Fatal("interactive reconciliation attempted to sleep")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ready, err := indexer.Reconcile(t.Context(), targetCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0].SubmittedHash != all.Chunks[2].SubmittedHash {
		t.Errorf("sent documents = %+v, want only the new target", sent)
	}
	if ready.CorpusFingerprint() != targetCorpus.Fingerprint {
		t.Errorf("ready fingerprints differ from target corpus")
	}
	if _, err := ready.Search(t.Context(), "same action", nil, 1); err != nil {
		t.Fatal(err)
	}
	if factoryCalls != 1 || querySends != 1 {
		t.Fatalf("provider factories=%d query sends=%d, want one shared instance", factoryCalls, querySends)
	}
}

func TestIndexerReconcilePreflightsBeforeWriterOrStagingMutation(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	held, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, held)
	factoryCalls := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		factoryCalls++
		return nil, fmt.Errorf("%w: key is absent", ErrEmbedderConfiguration)
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), targetCorpus)
	if !errors.Is(err, ErrEmbedderConfiguration) {
		t.Fatalf("Reconcile() error = %v, want configuration failure before ErrWriterHeld", err)
	}
	catalog, err := held.q.Catalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if catalog.StagingGenerationID.Valid || factoryCalls != 1 {
		t.Fatalf("preflight left staging=%v after factories=%d, want no staging and one preflight", catalog.StagingGenerationID.Valid, factoryCalls)
	}
}

func TestIndexerReconcileReturnsVerifiedSnapshotWithoutPostActivationRead(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	seedActiveGeneration(t, &config, activeCorpus)
	reads := 0
	config.deps.readCorpus = func(context.Context) (Corpus, error) {
		reads++
		if reads > 3 {
			return Corpus{}, errors.New("post-activation corpus reread")
		}
		return targetCorpus, nil
	}
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := indexer.Reconcile(t.Context(), targetCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("corpus reads = %d, want under-lease and pre-activation reads only", reads)
	}
	if ready.CorpusFingerprint() != targetCorpus.Fingerprint {
		t.Fatalf("ready search does not bind the activated generation to its verified corpus")
	}
}

func TestReadySearchRevalidatesAuthorityAndAllowsOneQueryAttempt(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	authorityValid := true
	config.deps.validateQueryAuthority = func() error {
		if !authorityValid {
			return ErrPolicySourceChanged
		}
		return nil
	}
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("current generation sent a document")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	authorityValid = false
	if _, err := ready.Search(t.Context(), "must stay local", nil, 1); !errors.Is(err, ErrPolicySourceChanged) {
		t.Fatalf("Search() error = %v, want ErrPolicySourceChanged", err)
	}
	if querySends != 0 {
		t.Fatalf("query sends after authority drift = %d, want zero", querySends)
	}
	if _, err := ready.Search(t.Context(), "second attempt", nil, 1); !errors.Is(err, ErrQueryAlreadyAttempted) {
		t.Fatalf("second Search() error = %v, want ErrQueryAlreadyAttempted", err)
	}
}

func TestReadySearchRejectsInvalidDepthBeforeQueryAttempt(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("unexpected chunk send")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ready.Search(t.Context(), "query", nil, 0); !errors.Is(err, ErrInvalidSearchState) {
		t.Fatalf("Search(depth=0) error = %v, want ErrInvalidSearchState", err)
	}
	if querySends != 0 {
		t.Fatalf("Search(depth=0) query sends = %d, want zero", querySends)
	}
	if _, err := ready.Search(t.Context(), "query", nil, 1); err != nil {
		t.Fatalf("Search(depth=1) error = %v", err)
	}
	if querySends != 1 {
		t.Fatalf("Search(depth=1) query sends = %d, want one", querySends)
	}
}

func TestReadySearchRejectsInvalidQueryBeforeQueryAttempt(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	validations := 0
	config.deps.validateQueryAuthority = func() error {
		validations++
		return nil
	}
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("unexpected chunk send")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}

	if _, queryErr := ready.Search(t.Context(), "  \t", nil, 1); !errors.Is(queryErr, ErrQueryNotApplicable) {
		t.Fatalf("Search(blank) error = %v, want ErrQueryNotApplicable", queryErr)
	}
	_, err = ready.Search(t.Context(), string([]byte{0xff}), nil, 1)
	embedErr, ok := errors.AsType[*EmbedError](err)
	if !ok || embedErr.Kind != EmbedFailureInternal {
		t.Fatalf("Search(invalid UTF-8) error = %v, want internal formation failure", err)
	}
	if querySends != 0 || validations != 0 {
		t.Fatalf("invalid Search() validations/sends = %d/%d, want zero/zero", validations, querySends)
	}
	if _, err := ready.Search(t.Context(), "query", nil, 1); err != nil {
		t.Fatalf("Search(valid after invalid) error = %v", err)
	}
	if querySends != 1 || validations != 1 {
		t.Fatalf("valid Search() validations/sends = %d/%d, want one/one", validations, querySends)
	}
}

func TestReadySearchNamesInvalidCapabilityState(t *testing.T) {
	var ready ReadySearch
	if _, err := ready.Search(t.Context(), "query", nil, 1); !errors.Is(err, ErrInvalidSearchState) {
		t.Fatalf("Search() error = %v, want ErrInvalidSearchState", err)
	}
}

func TestIndexerReconcileRejectsFinalCorpusChangeWithoutActivation(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 3)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:2])
	changedCorpus := fixtureIndexerCorpus(t, all.Chunks[2:])
	seedActiveGeneration(t, &config, activeCorpus)
	reads := 0
	config.deps.readCorpus = func(context.Context) (Corpus, error) {
		reads++
		if reads < 2 {
			return targetCorpus, nil
		}
		return changedCorpus, nil
	}
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), targetCorpus)
	if !errors.Is(err, ErrVaultChanged) {
		t.Fatalf("Reconcile() error = %v, want ErrVaultChanged", err)
	}
	store, err := openStore(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, store)
	active, err := store.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if active.corpusFingerprint != activeCorpus.Fingerprint {
		t.Errorf("active fingerprint = %x, want unchanged %x", active.corpusFingerprint, activeCorpus.Fingerprint)
	}
}

func TestIndexerReconcileRejectsTargetIndexBeforeActivation(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	config.deps.newIndex = func([]ChunkVector, int) (*Index, error) {
		return nil, ErrDimensionMismatch
	}
	documentSends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		documentSends++
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), targetCorpus)
	if !errors.Is(err, ErrGenerationIncomplete) || !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("Reconcile() error = %v, want generation-incomplete target index failure", err)
	}
	store, err := openStore(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, store)
	active, err := store.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if active.corpusFingerprint != activeCorpus.Fingerprint || documentSends != 1 {
		t.Fatalf("index failure activated target or skipped bounded work: active=%x sends=%d", active.corpusFingerprint, documentSends)
	}
}

func TestIndexerReconcileDoesNotRetryRateLimit(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	sends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureRateLimited, RetryAfter: time.Second, RetryAfterValid: true}
	}, nil, func(context.Context, time.Duration) error {
		t.Fatal("interactive reconciliation attempted to wait")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), targetCorpus)
	embedErr, ok := errors.AsType[*EmbedError](err)
	if !ok || embedErr.Kind != EmbedFailureRateLimited {
		t.Fatalf("Reconcile() error = %v, want FailureRateLimited", err)
	}
	if sends != 1 {
		t.Fatalf("Reconcile() sends = %d, want exactly one", sends)
	}
	store, err := openStore(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, store)
	active, err := store.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if active.corpusFingerprint != activeCorpus.Fingerprint {
		t.Errorf("active fingerprint = %x, want unchanged %x", active.corpusFingerprint, activeCorpus.Fingerprint)
	}
}

func TestIndexerFullBuildPersistsLongRetryWithoutWaiting(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	now := time.Unix(2_000, 0)
	sends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		return EmbeddingResult{}, &EmbedError{
			Kind:            EmbedFailureRateLimited,
			RetryAfter:      31 * time.Second,
			RetryAfterValid: true,
		}
	}, func() time.Time { return now }, func(context.Context, time.Duration) error {
		t.Fatal("full build waited past its per-action ceiling")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Build(t.Context())
	retryErr, ok := errors.AsType[*retryNotReadyError](err)
	wantRetryAt := now.Add(31 * time.Second)
	if !ok || !retryErr.At.Equal(wantRetryAt) {
		t.Fatalf("Build() error = %v, want retry at %s", err, wantRetryAt)
	}
	if sends != 1 {
		t.Fatalf("Build() sends = %d, want one", sends)
	}
	writer, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	targets, _, err := buildTargets(corpus)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := writer.prepare(t.Context(), &config.settings.identity, config.settings.policySource, targets)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resumed.reserveAttempt(t.Context(), corpus.Chunks[0].RelPath, corpus.Chunks[0].Ordinal, now)
	if !errors.Is(err, ErrRetryNotReady) {
		t.Fatalf("resumed ReserveAttempt() error = %v, want durable ErrRetryNotReady", err)
	}
}

func TestIndexerFullBuildHonorsValidZeroRetryAfter(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	now := time.Unix(3_000, 0)
	sends := 0
	var delays []time.Duration
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		if sends == 1 {
			return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureRateLimited, RetryAfterValid: true}
		}
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		now = now.Add(delay)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildPublished || sends != 2 || !slices.Equal(delays, []time.Duration{0}) {
		t.Errorf("Build() = %+v after sends=%d delays=%v, want immediate valid retry", got, sends, delays)
	}
}

func TestIndexerExplicitBuildRecoversCorruptStore(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	if err := os.WriteFile(config.settings.storePath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	sends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildPublished || got.Chunks != 1 || sends != 1 {
		t.Errorf("Build() = %+v after %d sends, want recovered one-chunk generation", got, sends)
	}
}

func TestIndexerExplicitBuildReplacesLogicallyCorruptActive(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	writer, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, execErr := writer.db.ExecContext(t.Context(), `DELETE FROM chunks`); execErr != nil {
		t.Fatal(execErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildPublished {
		t.Errorf("Build() = %+v, want replacement complete generation", got)
	}
}

func TestIndexerExplicitBuildReplacesIncompatibleVectorFormat(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	writer, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, execErr := writer.db.ExecContext(t.Context(), `UPDATE generations SET vector_format_version = 999`); execErr != nil {
		t.Fatal(execErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	sends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildPublished || got.Chunks != 1 || sends != 1 {
		t.Fatalf("Build() = %+v after %d sends, want one fresh vector-format rebuild", got, sends)
	}
}

func TestIndexerReconcileReturnsHeldWriterWhenTheSingleRereadIsStillStale(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	writer, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	var sent []CorpusChunk
	indexer, err := newTestIndexer(&config, func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
		sent = append(sent, *document)
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), targetCorpus)
	if !errors.Is(err, ErrWriterHeld) {
		t.Fatalf("Reconcile() error = %v, want ErrWriterHeld", err)
	}
	if len(sent) != 0 {
		t.Fatalf("Reconcile() sent %d documents without the writer, want zero", len(sent))
	}
}

func TestIndexerReconcileUsesGenerationPublishedByTheLeaseWinner(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)

	winner, err := openWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, winner)
	config.deps.openWriter = func(ctx context.Context, _ string) (*writer, error) {
		targets, _, targetErr := buildTargets(targetCorpus)
		if targetErr != nil {
			t.Fatal(targetErr)
		}
		build, prepareErr := winner.prepare(ctx, &config.settings.identity, config.settings.policySource, targets)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		for i := range targetCorpus.Chunks {
			document := &targetCorpus.Chunks[i]
			if _, reserveErr := build.reserveAttempt(ctx, document.RelPath, document.Ordinal, time.Unix(1, 0)); reserveErr != nil {
				t.Fatal(reserveErr)
			}
			row, completeErr := document.Complete(EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())})
			if completeErr != nil {
				t.Fatal(completeErr)
			}
			if putErr := build.put(ctx, &row); putErr != nil {
				t.Fatal(putErr)
			}
		}
		if measureErr := build.setTopKP95(ctx, time.Microsecond); measureErr != nil {
			t.Fatal(measureErr)
		}
		if activateErr := build.activate(ctx); activateErr != nil {
			t.Fatal(activateErr)
		}
		if closeErr := winner.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		return nil, ErrWriterHeld
	}

	documentSends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		documentSends++
		return EmbeddingResult{}, errors.New("unexpected document send")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := indexer.Reconcile(t.Context(), targetCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if ready.CorpusFingerprint() != targetCorpus.Fingerprint {
		t.Fatalf("Reconcile() fingerprint = %x, want the winner's current generation", ready.CorpusFingerprint())
	}
	if documentSends != 0 {
		t.Fatalf("document sends = %d, want zero after lease winner published", documentSends)
	}
}

func TestIndexerReconcileRejectsProxyTokenBoundBeforeEgress(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetDocument := all.Chunks[1]
	targetDocument.ProxyTokens = maxInteractiveProxyTokens + 1
	targetCorpus := fixtureIndexerCorpus(t, []CorpusChunk{targetDocument})
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	var sent []CorpusChunk
	indexer, err := newTestIndexer(&config, func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
		sent = append(sent, *document)
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Reconcile(t.Context(), targetCorpus)
	if !errors.Is(err, ErrRebuildRequired) {
		t.Fatalf("Reconcile() error = %v, want ErrRebuildRequired", err)
	}
	if len(sent) != 0 {
		t.Fatalf("Reconcile() sent %d documents above the proxy-token bound, want zero", len(sent))
	}
}

func TestNewIndexerBindsProviderToItsOwnCorpusAuthority(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storeRoot := t.TempDir()
	reader, artifact, privacy := fixtureIndexerPolicies(t, root)
	if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o700); err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(root, "Writing", "note.md")
	if err := os.WriteFile(notePath, []byte("# Public\n\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	corpus, err := readCorpus(t.Context(), reader, artifact, privacy, 256)
	if err != nil {
		t.Fatal(err)
	}
	keyReads := 0
	indexer, err := NewIndexer(&IndexerConfig{
		StorePath:      filepath.Join(storeRoot, "semantic", "generation.sqlite"),
		Vault:          reader,
		Dimension:      testEmbeddingDimension,
		ChunkTokenCap:  256,
		ArtifactPolicy: artifact,
		PrivacyPolicy:  privacy,
		LatencyVectors: fixtureTopKWorkload(t, testEmbeddingDimension).QueryVectors,
	}, func() string {
		keyReads++
		return "key-sentinel"
	})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := indexer.openActionProvider()
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(notePath, []byte("# Public\n\nchanged\n"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = provider.embedChunk(t.Context(), &corpus.Chunks[0])
	if !errors.Is(err, ErrSourceNoteChanged) {
		t.Fatalf("EmbedChunk() error = %v, want ErrSourceNoteChanged", err)
	}
	if keyReads != 1 {
		t.Fatalf("API key reads = %d, want one", keyReads)
	}
}

func TestNewIndexerRejectsPoliciesLoadedFromAnotherRoot(t *testing.T) {
	t.Parallel()

	policyRoot := t.TempDir()
	selectedRoot := t.TempDir()
	storeRoot := t.TempDir()
	_, artifact, privacy := fixtureIndexerPolicies(t, policyRoot)
	reader := openIndexerVault(t, selectedRoot)
	keyReads := 0
	_, err := NewIndexer(&IndexerConfig{
		StorePath:      filepath.Join(storeRoot, "semantic", "generation.sqlite"),
		Vault:          reader,
		Dimension:      testEmbeddingDimension,
		ChunkTokenCap:  256,
		ArtifactPolicy: artifact,
		PrivacyPolicy:  privacy,
		LatencyVectors: fixtureTopKWorkload(t, testEmbeddingDimension).QueryVectors,
	}, func() string {
		keyReads++
		return "key-sentinel"
	})
	if !errors.Is(err, ErrPolicySourceChanged) {
		t.Fatalf("NewIndexer() error = %v, want cross-root policy rejection", err)
	}
	if keyReads != 0 {
		t.Fatalf("API key reads = %d, want zero", keyReads)
	}
}

func TestNewIndexerRejectsPathLoadedPoliciesForPinnedVault(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storeRoot := t.TempDir()
	reader, _, _ := fixtureIndexerPolicies(t, root)
	pathLoaded, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	keyReads := 0
	_, err = NewIndexer(&IndexerConfig{
		StorePath:      filepath.Join(storeRoot, "semantic", "generation.sqlite"),
		Vault:          reader,
		Dimension:      testEmbeddingDimension,
		ChunkTokenCap:  256,
		ArtifactPolicy: pathLoaded.ArtifactPolicy(),
		PrivacyPolicy:  pathLoaded.PrivacyPolicy(),
		LatencyVectors: fixtureTopKWorkload(t, testEmbeddingDimension).QueryVectors,
	}, func() string {
		keyReads++
		return "key-sentinel"
	})
	if !errors.Is(err, ErrPolicySourceChanged) {
		t.Fatalf("NewIndexer(path-loaded policies) error = %v, want ErrPolicySourceChanged", err)
	}
	if keyReads != 0 {
		t.Fatalf("NewIndexer(path-loaded policies) key reads = %d, want zero", keyReads)
	}
}

func TestNewIndexerRejectsStoreInsideVaultBeforeReadingKey(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := fixtureIndexerPolicies(t, root)
	keyReads := 0
	_, err := NewIndexer(&IndexerConfig{
		StorePath:      filepath.Join(root, ".yomihon", "generation.sqlite"),
		Vault:          reader,
		Dimension:      testEmbeddingDimension,
		ChunkTokenCap:  256,
		ArtifactPolicy: artifact,
		PrivacyPolicy:  privacy,
		LatencyVectors: fixtureTopKWorkload(t, testEmbeddingDimension).QueryVectors,
	}, func() string {
		keyReads++
		return "key-sentinel"
	})
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("NewIndexer(store inside vault) error = %v, want errIndexerConfiguration", err)
	}
	if keyReads != 0 {
		t.Fatalf("NewIndexer(store inside vault) key reads = %d, want zero", keyReads)
	}
}

func TestNewIndexerRejectsStoreWhoseAncestorResolvesIntoVaultBeforeReadingKey(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := fixtureIndexerPolicies(t, root)
	cacheRoot := t.TempDir()
	storeRoot := filepath.Join(cacheRoot, "vault-link")
	if err := os.Symlink(root, storeRoot); err != nil {
		t.Fatal(err)
	}
	keyReads := 0
	_, err := NewIndexer(&IndexerConfig{
		StorePath:      filepath.Join(storeRoot, ".yomihon", "generation.sqlite"),
		Vault:          reader,
		Dimension:      testEmbeddingDimension,
		ChunkTokenCap:  256,
		ArtifactPolicy: artifact,
		PrivacyPolicy:  privacy,
		LatencyVectors: fixtureTopKWorkload(t, testEmbeddingDimension).QueryVectors,
	}, func() string {
		keyReads++
		return "key-sentinel"
	})
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("NewIndexer(store through symlink into vault) error = %v, want errIndexerConfiguration", err)
	}
	if keyReads != 0 {
		t.Fatalf("NewIndexer(store through symlink into vault) key reads = %d, want zero", keyReads)
	}
}

func TestNewIndexerRejectsDanglingStoreSymlinkPointingIntoVaultBeforeReadingKey(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	reader, artifact, privacy := fixtureIndexerPolicies(t, root)
	storePath := filepath.Join(t.TempDir(), "generation.sqlite")
	if err := os.Symlink(filepath.Join(root, ".yomihon", "generation.sqlite"), storePath); err != nil {
		t.Fatal(err)
	}
	keyReads := 0
	_, err := NewIndexer(&IndexerConfig{
		StorePath:      storePath,
		Vault:          reader,
		Dimension:      testEmbeddingDimension,
		ChunkTokenCap:  256,
		ArtifactPolicy: artifact,
		PrivacyPolicy:  privacy,
		LatencyVectors: fixtureTopKWorkload(t, testEmbeddingDimension).QueryVectors,
	}, func() string {
		keyReads++
		return "key-sentinel"
	})
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("NewIndexer(dangling store symlink into vault) error = %v, want errIndexerConfiguration", err)
	}
	if keyReads != 0 {
		t.Fatalf("NewIndexer(dangling store symlink into vault) key reads = %d, want zero", keyReads)
	}
}

func TestIndexerFullBuildNeverSendsSixthAttemptAcrossResume(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	now := time.Unix(4_000, 0)
	sends := 0
	sender := func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		return EmbeddingResult{}, &EmbedError{Kind: EmbedFailureRateLimited}
	}
	wait := func(_ context.Context, delay time.Duration) error {
		now = now.Add(delay)
		return nil
	}
	indexer, err := newTestIndexer(&config, sender, func() time.Time { return now }, wait)
	if err != nil {
		t.Fatal(err)
	}
	_, err = indexer.Build(t.Context())
	embedErr, ok := errors.AsType[*EmbedError](err)
	if !ok || embedErr.Kind != EmbedFailureRateLimited || sends != 5 {
		t.Fatalf("first Build() = (%v, sends %d), want rate limit after five", err, sends)
	}

	resumed, err := newTestIndexer(&config, sender, func() time.Time { return now }, wait)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resumed.Build(t.Context())
	if !errors.Is(err, ErrAttemptLimit) {
		t.Fatalf("resumed Build() error = %v, want ErrAttemptLimit", err)
	}
	if sends != 5 {
		t.Fatalf("resumed Build() total sends = %d, want no sixth send", sends)
	}
}

func TestIndexerReconcileInterruptionLeavesActiveAndResumesStaging(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 3)
	activeCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return targetCorpus, nil }
	seedActiveGeneration(t, &config, activeCorpus)
	firstSends := 0
	interrupted, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		firstSends++
		if firstSends == 2 {
			return EmbeddingResult{}, context.Canceled
		}
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = interrupted.Reconcile(t.Context(), targetCorpus)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("first Reconcile() error = %v, want context.Canceled", err)
	}
	store, err := openStore(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	active, err := store.Active(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := store.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if active.corpusFingerprint != activeCorpus.Fingerprint {
		t.Fatalf("active changed after interruption: %x", active.corpusFingerprint)
	}

	var resumedSends []CorpusChunk
	resumed, err := newTestIndexer(&config, func(_ context.Context, document *CorpusChunk) (EmbeddingResult, error) {
		resumedSends = append(resumedSends, *document)
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resumed.Reconcile(t.Context(), targetCorpus)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumedSends) != 1 || resumedSends[0].SubmittedHash != all.Chunks[2].SubmittedHash {
		t.Errorf("resumed sends = %+v, want only interrupted target", resumedSends)
	}
}

func TestIndexerRecordsNearestRankTopKP95FromFixedWorkload(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	measurements := 0
	indexer.measure = func(run func() error) (time.Duration, error) {
		measurements++
		return time.Duration(measurements) * time.Microsecond, run()
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if measurements != 121 {
		t.Errorf("top-k executions = %d, want 121 including discarded warm-up", measurements)
	}
	if got.TopKP95 != 115*time.Microsecond {
		t.Errorf("top-k p95 = %s, want nearest-rank 115us", got.TopKP95)
	}
}

func TestIndexerBuildsWithoutRecordedTopKWorkloadAndLeavesP95Unmeasured(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic fixture bootstrap is the sole unmeasured state. Production
	// construction cannot express it; the package test reaches it only after
	// validating an otherwise production-shaped indexer.
	indexer.settings.topKWorkload = topKWorkload{}
	indexer.measure = func(func() error) (time.Duration, error) {
		t.Fatal("absent observer workload reached measurement")
		return 0, nil
	}

	got, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != BuildPublished || got.TopKP95 != 0 {
		t.Fatalf("Build() = %+v, want published generation with unmeasured p95", got)
	}
	if _, err := indexer.Reconcile(t.Context(), corpus); !errors.Is(err, ErrGenerationUnmeasured) {
		t.Fatalf("Reconcile() error = %v, want ErrGenerationUnmeasured", err)
	}
}

func TestIndexerRejectsMissingTopKWorkloadOutsideFixtureCapture(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	config.settings.topKWorkload = topKWorkload{}
	_, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("newTestIndexer() error = %v, want errIndexerConfiguration", err)
	}
}

func TestIndexerRejectsMissingQueryAuthorityValidator(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	config.deps.validateQueryAuthority = nil
	_, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, time.Now, nil)
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("newTestIndexer() error = %v, want errIndexerConfiguration", err)
	}
}

func TestReadySearchCapturesCorpusFingerprint(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return corpus, nil }
	seedActiveGeneration(t, &config, corpus)
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("current generation sent a document")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}

	want := corpus.Fingerprint
	corpus.Fingerprint = [sha256.Size]byte{}
	if got := ready.CorpusFingerprint(); got != want {
		t.Fatalf("ReadySearch.CorpusFingerprint() = %x, want captured %x", got, want)
	}
}

func TestWriterWinnerPreservesStoreReadFailure(t *testing.T) {
	t.Parallel()

	want := errors.New("read active generation")
	_, err := writerWinnerCurrent(&loadedGeneration{}, want, &generationIdentity{}, [sha256.Size]byte{})
	if !errors.Is(err, want) {
		t.Fatalf("writerWinnerCurrent() error = %v, want original store error", err)
	}
}

func TestMeasuredTopKP95UsesOneMicrosecondAsTheSmallestMeasuredValue(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	indexer.measure = func(run func() error) (time.Duration, error) { return 0, run() }
	result, err := indexer.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if result.TopKP95 != time.Microsecond {
		t.Fatalf("Build().TopKP95 = %s, want 1us measured sentinel", result.TopKP95)
	}
}

func TestReadySearchCancellationConsumesOnlyTheStartedQueryAttempt(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	querySends := 0
	ctx, cancel := context.WithCancel(t.Context())
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("current generation sent a document")
			},
			embedQuery: func(ctx context.Context, _ string) (EmbeddingResult, error) {
				querySends++
				cancel()
				return EmbeddingResult{}, ctx.Err()
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ready.Search(ctx, "cancel during send", nil, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("Search() error = %v, want context.Canceled", err)
	}
	if _, err := ready.Search(t.Context(), "must not resend", nil, 1); !errors.Is(err, ErrQueryAlreadyAttempted) {
		t.Fatalf("second Search() error = %v, want ErrQueryAlreadyAttempted", err)
	}
	if querySends != 1 {
		t.Fatalf("query sends = %d, want one started attempt", querySends)
	}
}

func TestReadySearchCopiesShareTheSingleQueryAttempt(t *testing.T) {
	t.Parallel()

	config, corpus := fixtureIndexerConfig(t, 1)
	seedActiveGeneration(t, &config, corpus)
	querySends := 0
	indexer, err := newIndexerWithProvider(&config, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("current generation sent a document")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				querySends++
				return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := indexer.Reconcile(t.Context(), corpus)
	if err != nil {
		t.Fatal(err)
	}
	copyOfReady := *ready
	if _, err := ready.Search(t.Context(), "first", nil, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := copyOfReady.Search(t.Context(), "second", nil, 1); !errors.Is(err, ErrQueryAlreadyAttempted) {
		t.Fatalf("copied ReadySearch error = %v, want ErrQueryAlreadyAttempted", err)
	}
	if querySends != 1 {
		t.Fatalf("query sends through original and copy = %d, want one", querySends)
	}
}

func TestIndexerFullBuildRejectsFinalCorpusChangeWithoutActivation(t *testing.T) {
	t.Parallel()

	config, all := fixtureIndexerConfig(t, 2)
	targetCorpus := fixtureIndexerCorpus(t, all.Chunks[:1])
	changedCorpus := fixtureIndexerCorpus(t, all.Chunks[1:])
	reads := 0
	config.deps.readCorpus = func(context.Context) (Corpus, error) {
		reads++
		if reads < 3 {
			return targetCorpus, nil
		}
		return changedCorpus, nil
	}
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		return EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Build(t.Context())
	if !errors.Is(err, ErrVaultChanged) {
		t.Fatalf("Build() error = %v, want ErrVaultChanged", err)
	}
	store, err := openStore(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, store)
	if _, err := store.Active(t.Context()); !errors.Is(err, ErrNoActiveGeneration) {
		t.Fatalf("Active() after rejected publication error = %v, want ErrNoActiveGeneration", err)
	}
}

func TestIndexerRejectsIncompatibleTopKWorkload(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	config.settings.topKWorkload.CompatibilityIdentity = sha256.Sum256([]byte("wrong query vector epoch"))
	_, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		t.Fatal("incompatible observer reached provider sender")
		return EmbeddingResult{}, nil
	}, nil, nil)
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("newTestIndexer() error = %v, want incompatible workload rejection", err)
	}
}

func TestIndexerRejectsPartialTopKWorkload(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	config.settings.topKWorkload.QueryVectors = nil
	_, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		t.Fatal("partial observer workload reached provider sender")
		return EmbeddingResult{}, nil
	}, nil, nil)
	if !errors.Is(err, errIndexerConfiguration) {
		t.Fatalf("newTestIndexer() error = %v, want partial workload rejection", err)
	}
}

func TestIndexerRejectsRungTwoCorpusBeforeProviderOrStaging(t *testing.T) {
	t.Parallel()

	config, _ := fixtureIndexerConfig(t, 1)
	oversized := Corpus{Chunks: make([]CorpusChunk, exactScanChunkTrigger)}
	config.deps.readCorpus = func(context.Context) (Corpus, error) { return oversized, nil }
	sends := 0
	indexer, err := newTestIndexer(&config, func(_ context.Context, _ *CorpusChunk) (EmbeddingResult, error) {
		sends++
		return EmbeddingResult{}, errors.New("unexpected provider send")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = indexer.Build(t.Context())
	if !errors.Is(err, ErrIndexCapacity) {
		t.Fatalf("Build() error = %v, want ErrIndexCapacity", err)
	}
	if sends != 0 {
		t.Fatalf("provider sends = %d, want zero", sends)
	}
}

func fixtureIndexerConfig(t *testing.T, documents int) (indexerSetup, Corpus) {
	t.Helper()

	root := t.TempDir()
	storeRoot := t.TempDir()
	policy := sha256.Sum256([]byte("indexer-policy"))
	identity, err := newGenerationIdentity(IdentityConfig{
		Dimension:               testEmbeddingDimension,
		ChunkTokenCap:           256,
		VaultRoot:               root,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	docs := make([]CorpusChunk, 0, documents)
	for i := range documents {
		suffix := fmt.Sprintf("%03d", i)
		submitted := []byte("title: Note | text: body " + suffix)
		document := CorpusChunk{
			RelPath:       "Writing/note-" + suffix + ".md",
			NoteHash:      sha256.Sum256([]byte("note-" + suffix)),
			Ordinal:       0,
			Submitted:     submitted,
			SubmittedHash: sha256.Sum256(submitted),
			ProxyTokens:   7,
		}
		docs = append(docs, document)
	}
	corpus := fixtureIndexerCorpus(t, docs)
	config := indexerSetup{
		settings: indexerSettings{
			storePath:    filepath.Join(storeRoot, "semantic", "generation.sqlite"),
			identity:     identity,
			policySource: sha256.Sum256([]byte("indexer-policy-source")),
			topKWorkload: fixtureTopKWorkload(t, identity.Dimension()),
		},
		deps: indexerDeps{
			readCorpus: func(context.Context) (Corpus, error) {
				return corpus, nil
			},
			validateQueryAuthority: func() error { return nil },
		},
	}
	return config, corpus
}

func fixtureIndexerCorpus(t *testing.T, documents []CorpusChunk) Corpus {
	t.Helper()

	manifest := make([]ChunkVector, 0, len(documents))
	proxyTokens := 0
	for i := range documents {
		target, err := documents[i].Target()
		if err != nil {
			t.Fatal(err)
		}
		manifest = append(manifest, target.complete(nil))
		proxyTokens += documents[i].ProxyTokens
	}
	fingerprint, err := CorpusFingerprint(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return Corpus{Chunks: slices.Clone(documents), Fingerprint: fingerprint, ProxyTokens: proxyTokens}
}

func seedActiveGeneration(t *testing.T, config *indexerSetup, corpus Corpus) {
	t.Helper()

	writer, err := openRebuildWriter(t.Context(), config.settings.storePath)
	if err != nil {
		t.Fatal(err)
	}
	closeOnCleanup(t, writer)
	targets := make([]ChunkTarget, 0, len(corpus.Chunks))
	for i := range corpus.Chunks {
		document := &corpus.Chunks[i]
		target, targetErr := document.Target()
		if targetErr != nil {
			t.Fatal(targetErr)
		}
		targets = append(targets, target)
	}
	build, err := writer.prepare(t.Context(), &config.settings.identity, config.settings.policySource, targets)
	if err != nil {
		t.Fatal(err)
	}
	for i := range corpus.Chunks {
		document := &corpus.Chunks[i]
		if _, err := build.reserveAttempt(t.Context(), document.RelPath, document.Ordinal, time.Unix(1, 0)); err != nil {
			t.Fatal(err)
		}
		row, err := document.Complete(EmbeddingResult{Vector: fixtureIndexerVector(config.settings.identity.Dimension())})
		if err != nil {
			t.Fatal(err)
		}
		if err := build.put(t.Context(), &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := build.setTopKP95(t.Context(), time.Microsecond); err != nil {
		t.Fatal(err)
	}
	if err := build.activate(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func fixtureIndexerVector(dimension int) []float32 {
	vector := make([]float32, dimension)
	vector[0] = 1
	return vector
}

func fixtureTopKWorkload(t *testing.T, dimension int) topKWorkload {
	t.Helper()

	compatibility, err := QueryVectorCompatibilityIdentity(dimension)
	if err != nil {
		t.Fatal(err)
	}
	vectors := make([][]float32, topKObserverQueryCount)
	for i := range vectors {
		vectors[i] = fixtureIndexerVector(dimension)
	}
	return topKWorkload{CompatibilityIdentity: compatibility, QueryVectors: vectors}
}

func fixtureIndexerPolicies(t *testing.T, root string) (*vault.Reader, schema.ArtifactPolicy, schema.PrivacyPolicy) {
	t.Helper()

	contract := `schema_version = "1"

[enums]
type = ["note"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type"]

[scan]
knowledge_dirs = ["Writing"]

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`
	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contract), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := openIndexerVault(t, root)
	loaded, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal(err)
	}
	return reader, loaded.ArtifactPolicy(), loaded.PrivacyPolicy()
}

func openIndexerVault(t *testing.T, root string) *vault.Reader {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("close vault reader: %v", closeErr)
		}
	})
	return reader
}
