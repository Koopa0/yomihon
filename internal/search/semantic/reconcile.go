package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"time"
)

// ErrInvalidSearchState reports an internal violation of the single-action
// semantic query capability. It is never a cache, corpus, or provider fault.
var ErrInvalidSearchState = errors.New("semantic search capability is invalid")

// ReadySearch is the single-action query capability returned only by a
// successful Reconcile call.
type ReadySearch struct {
	state *readySearchState
}

type readySearchState struct {
	corpusFingerprint [sha256.Size]byte
	query             queryCapability
	index             *Index
}

// CorpusFingerprint returns the exact corpus manifest bound to this query
// capability. Display evidence remains owned by the agent action snapshot.
func (r *ReadySearch) CorpusFingerprint() [sha256.Size]byte {
	if r == nil || r.state == nil {
		return [sha256.Size]byte{}
	}
	return r.state.corpusFingerprint
}

// Search performs the action's one authorized query-vector request and ranks
// that vector against the already-bound immutable generation. Keeping both
// steps in one method prevents consumers from violating their temporal order.
func (r *ReadySearch) Search(
	ctx context.Context,
	query string,
	allowedPaths map[string]struct{},
	depth int,
) ([]Rank, error) {
	if r == nil || r.state == nil || r.state.query.validate == nil ||
		r.state.query.send == nil || r.state.index == nil || depth <= 0 {
		return nil, ErrInvalidSearchState
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateQueryText(query); err != nil {
		return nil, err
	}
	embedding, err := r.state.query.embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.state.index.TopNotes(embedding.Vector, allowedPaths, depth)
}

type queryCapability struct {
	mu        sync.Mutex
	attempted bool
	validate  func(context.Context) error
	send      querySender
}

func (q *queryCapability) embed(ctx context.Context, query string) (EmbeddingResult, error) {
	if q == nil || q.validate == nil || q.send == nil {
		return EmbeddingResult{}, ErrInvalidSearchState
	}
	if err := ctx.Err(); err != nil {
		return EmbeddingResult{}, err
	}
	q.mu.Lock()
	if q.attempted {
		q.mu.Unlock()
		return EmbeddingResult{}, ErrQueryAlreadyAttempted
	}
	q.attempted = true
	q.mu.Unlock()
	if err := q.validate(ctx); err != nil {
		return EmbeddingResult{}, err
	}
	return q.send(ctx, query)
}

// Reconcile refreshes compatible drift against the agent action's already-
// captured corpus and returns its single-use query capability. Cold or
// incompatible state is never built.
func (ix *Indexer) Reconcile(ctx context.Context, corpus Corpus) (*ReadySearch, error) {
	if err := ix.validateCorpus(ctx, corpus); err != nil {
		return nil, err
	}
	return ix.reconcileCorpus(ctx, corpus)
}

func (ix *Indexer) reconcileCorpus(ctx context.Context, corpus Corpus) (*ReadySearch, error) {
	generation, err := ix.activeGeneration(ctx)
	if err != nil {
		return nil, err
	}
	if generation.identity != ix.settings.identity {
		return nil, generationIdentityError(&generation.identity)
	}
	if generation.topKP95 < time.Microsecond {
		return nil, ErrGenerationUnmeasured
	}
	if generation.corpusFingerprint == corpus.Fingerprint {
		index, indexErr := ix.consumeGenerationRows(&generation)
		if indexErr != nil {
			return nil, indexErr
		}
		provider, providerErr := ix.openActionProvider()
		if providerErr != nil {
			return nil, providerErr
		}
		return ix.newReadySearch(&generation.generationMetadata, &corpus, provider, index)
	}
	missing, proxyTokens, err := interactiveWork(&generation, corpus)
	if err != nil {
		return nil, err
	}
	if missing > maxInteractiveChunks || proxyTokens > maxInteractiveProxyTokens {
		return nil, ErrRebuildRequired
	}
	provider, err := ix.openActionProvider()
	if err != nil {
		return nil, err
	}
	result, err := ix.reconcileGeneration(ctx, provider, corpus)
	if !errors.Is(err, ErrWriterHeld) {
		return result, err
	}
	return ix.recoverAfterWriterHeld(ctx, corpus, provider)
}

func (ix *Indexer) reconcileGeneration(
	ctx context.Context,
	provider *actionProvider,
	target Corpus,
) (result *ReadySearch, resultErr error) {
	writer, err := ix.deps.openWriter(ctx, ix.settings.storePath)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		resultErr = closeUnpublishedWriter(writer, published, resultErr)
	}()

	current, err := ix.readCorpus(ctx)
	if err != nil {
		return nil, err
	}
	if current.Fingerprint != target.Fingerprint {
		return nil, ErrVaultChanged
	}
	active, err := writer.Active(ctx)
	if err != nil {
		return nil, err
	}
	if active.identity != ix.settings.identity {
		return nil, generationIdentityError(&active.identity)
	}
	if active.topKP95 < time.Microsecond {
		return nil, ErrGenerationUnmeasured
	}
	if active.corpusFingerprint == target.Fingerprint {
		index, indexErr := ix.consumeGenerationRows(&active)
		if indexErr != nil {
			return nil, indexErr
		}
		return ix.newReadySearch(&active.generationMetadata, &target, provider, index)
	}
	if workErr := requireInteractiveWork(&active, target); workErr != nil {
		return nil, workErr
	}
	prepared, err := ix.prepareGeneration(ctx, writer, target)
	if err != nil {
		return nil, err
	}
	embedded, err := ix.embedInteractive(ctx, prepared, provider)
	if err != nil {
		return nil, err
	}
	publishedGeneration, err := ix.publishGeneration(ctx, prepared, embedded)
	if err != nil {
		return nil, err
	}
	published = true
	return ix.newReadySearch(
		&publishedGeneration.metadata,
		&publishedGeneration.corpus,
		provider,
		publishedGeneration.index,
	)
}

func (ix *Indexer) recoverAfterWriterHeld(
	ctx context.Context,
	corpus Corpus,
	provider *actionProvider,
) (*ReadySearch, error) {
	generation, err := ix.activeGeneration(ctx)
	current, err := writerWinnerCurrent(&generation, err, &ix.settings.identity, corpus.Fingerprint)
	if err != nil {
		return nil, err
	}
	if !current {
		return nil, ErrWriterHeld
	}
	index, err := ix.consumeGenerationRows(&generation)
	if err != nil {
		return nil, err
	}
	return ix.newReadySearch(&generation.generationMetadata, &corpus, provider, index)
}

func writerWinnerCurrent(
	generation *loadedGeneration,
	readErr error,
	identity *generationIdentity,
	corpusFingerprint [sha256.Size]byte,
) (bool, error) {
	if readErr != nil {
		return false, readErr
	}
	if generation != nil && generation.topKP95 < time.Microsecond {
		return false, ErrGenerationUnmeasured
	}
	return generation != nil && identity != nil &&
		generation.identity == *identity && generation.corpusFingerprint == corpusFingerprint, nil
}

func generationIdentityError(identity *generationIdentity) error {
	if identity != nil && identity.Model() == retiredEmbeddingModel {
		return ErrEmbedderRetired
	}
	return ErrGenerationMismatch
}

// consumeGenerationRows gives one freshly loaded generation's vectors to the
// query index. Clearing Rows prevents a second owner from mutating vectors now
// held by the index.
func (ix *Indexer) consumeGenerationRows(generation *loadedGeneration) (*Index, error) {
	if generation == nil {
		return nil, ErrInvalidSearchState
	}
	index, err := ix.deps.newIndex(generation.rows, generation.identity.Dimension())
	if err != nil {
		return nil, err
	}
	generation.rows = nil
	return index, nil
}

func (ix *Indexer) newReadySearch(
	metadata *generationMetadata,
	corpus *Corpus,
	provider *actionProvider,
	index *Index,
) (*ReadySearch, error) {
	if metadata == nil || corpus == nil || metadata.corpusFingerprint != corpus.Fingerprint {
		return nil, ErrVaultChanged
	}
	if metadata.topKP95 < time.Microsecond {
		return nil, ErrGenerationUnmeasured
	}
	if provider == nil || provider.embedQuery == nil || index == nil {
		return nil, ErrInvalidSearchState
	}
	return &ReadySearch{state: &readySearchState{
		corpusFingerprint: metadata.corpusFingerprint,
		index:             index,
		query: queryCapability{
			validate: func(ctx context.Context) error {
				if err := ix.deps.validateQueryAuthority(); err != nil {
					return err
				}
				return ix.deps.validateQueryCorpus(ctx, corpus.Fingerprint)
			},
			send: provider.embedQuery,
		},
	}}, nil
}
