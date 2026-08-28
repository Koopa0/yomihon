package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Indexer action errors distinguish incompatible state, unavailable evidence,
// retired protocols, explicit-rebuild requirements, and concurrent vault drift.
var (
	errIndexerConfiguration = errors.New("invalid semantic build configuration")
	ErrGenerationMismatch   = errors.New("semantic active generation identity does not match")
	ErrGenerationUnmeasured = errors.New("semantic active generation has no measured top-k latency")
	ErrEmbedderRetired      = errors.New("semantic active generation uses a retired embedding model")
	ErrRebuildRequired      = errors.New("semantic generation requires an explicit full build")
	ErrVaultChanged         = errors.New("semantic corpus changed during generation update")
	// ErrQueryAlreadyAttempted reports a second query-vector request through
	// one Reconcile action's single-use capability.
	ErrQueryAlreadyAttempted = errors.New("semantic query already attempted for this action")
)

const (
	retiredEmbeddingModel     = "gemini-embedding-001"
	maxInteractiveChunks      = 128
	maxInteractiveProxyTokens = 100_000
	topKObserverQueryCount    = 40
	topKObserverRepeats       = 3
)

var fullBuildRetrySchedule = [...]time.Duration{
	time.Second,
	4 * time.Second,
	9 * time.Second,
	16 * time.Second,
}

// BuildStatus describes whether an action reused an already-current generation
// or published a newly completed one.
type BuildStatus string

const (
	// BuildCurrent means the active generation already matched the local corpus.
	BuildCurrent BuildStatus = "current"
	// BuildPublished means this action activated a new complete generation.
	BuildPublished BuildStatus = "built"
)

// BuildReport is the content-free outcome of an explicit generation build.
// It never carries query capability or corpus evidence.
type BuildReport struct {
	Status   BuildStatus
	Chunks   int
	Embedded int
	Reused   int
	TopKP95  time.Duration
}

// BuildRequest selects the one explicit semantic build action to perform.
// The zero value runs an ordinary resumable build.
type BuildRequest struct {
	RenewAttemptBudget bool
}

type publishedGeneration struct {
	metadata generationMetadata
	corpus   Corpus
	index    *Index
	embedded int
	reused   int
}

type topKWorkload struct {
	CompatibilityIdentity [sha256.Size]byte
	QueryVectors          [][]float32
}

// IndexerConfig binds one command action to its local store and the concrete
// vault capabilities from which identity and corpus bytes are derived.
type IndexerConfig struct {
	StorePath      string
	Vault          *vault.Reader
	Dimension      int
	ChunkTokenCap  int
	ArtifactPolicy schema.ArtifactPolicy
	PrivacyPolicy  schema.PrivacyPolicy
	// LatencyVectors is the 40-query workload used to measure exact scan
	// latency. The command that loads an eval artifact must first validate its
	// protocol identity; this package owns only the numerical workload it runs.
	// NewIndexer copies and validates every vector before it can activate a
	// generation.
	LatencyVectors [][]float32
	// BuildProgress observes persisted rows from an explicit full build. It is
	// never called by Reconcile and receives counts only, never corpus content.
	BuildProgress func(embedded, total int) error
}

type indexerSettings struct {
	storePath               string
	identity                generationIdentity
	policySourceFingerprint [sha256.Size]byte
	topKWorkload            topKWorkload
}

type indexerDeps struct {
	readCorpus             func(context.Context) (Corpus, error)
	validateQueryAuthority func() error
	validateQueryCorpus    func(context.Context, [sha256.Size]byte) error
	newIndex               indexFactory
	openWriter             writerFactory
	beforeRenewCommit      func() error
	afterRenewCommit       func() error
}

type indexerSetup struct {
	settings indexerSettings
	deps     indexerDeps
}

type chunkSender func(context.Context, *CorpusChunk) (EmbeddingResult, error)
type querySender func(context.Context, string) (EmbeddingResult, error)
type providerFactory func() (*actionProvider, error)
type buildClock func() time.Time
type buildWait func(context.Context, time.Duration) error
type topKMeasure func(func() error) (time.Duration, error)
type indexFactory func([]ChunkVector, int) (*Index, error)
type writerFactory func(context.Context, string) (*writer, error)

type actionProvider struct {
	embedChunk chunkSender
	embedQuery querySender
}

// Indexer coordinates local corpus snapshots, provider work, and atomic
// generation publication. It retains only a dormant provider factory; a
// constructed transport exists only within one Build or Reconcile action.
type Indexer struct {
	settings      indexerSettings
	deps          indexerDeps
	openProvider  providerFactory
	now           buildClock
	wait          buildWait
	measure       topKMeasure
	buildProgress func(embedded, total int) error
}

// NewIndexer constructs a generation indexer whose provider factory remains
// dormant until Build proves that a chunk is missing, or Reconcile passes
// the local state and work-bound gates and needs one query-capable action.
// readAPIKey is called only inside that factory, after every local gate.
func NewIndexer(
	config *IndexerConfig,
	readAPIKey func() string,
) (*Indexer, error) {
	return newIndexer(config, readAPIKey, productionEmbeddingTransport)
}

func newIndexer(
	config *IndexerConfig,
	readAPIKey func() string,
	newTransport embeddingTransportFactory,
) (*Indexer, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: indexer configuration is nil", errIndexerConfiguration)
	}
	if readAPIKey == nil {
		return nil, fmt.Errorf("%w: API key source is nil", ErrEmbedderConfiguration)
	}
	if newTransport == nil {
		return nil, fmt.Errorf("%w: embedding transport source is nil", ErrEmbedderConfiguration)
	}
	setup, authorize, err := newIndexerSetup(config)
	if err != nil {
		return nil, err
	}
	openProvider := func() (*actionProvider, error) {
		embedder, embedErr := newGeminiEmbedder(
			readAPIKey(),
			setup.settings.identity.Dimension(),
			newTransport(),
			authorize,
		)
		if embedErr != nil {
			return nil, embedErr
		}
		return &actionProvider{embedChunk: embedder.EmbedChunk, embedQuery: embedder.EmbedQuery}, nil
	}
	indexer, err := newIndexerWithProvider(setup, openProvider, time.Now, waitForBuildRetry)
	if err != nil {
		return nil, err
	}
	indexer.buildProgress = config.BuildProgress
	return indexer, nil
}

func newIndexerSetup(config *IndexerConfig) (*indexerSetup, chunkAuthorizer, error) {
	owned := *config
	if owned.Vault == nil || owned.Vault.Name() == "" {
		return nil, nil, fmt.Errorf("%w: vault reader is unavailable", errIndexerConfiguration)
	}
	root := owned.Vault.Name()
	policyFingerprint, ok := schema.CorpusPolicyFingerprint(owned.ArtifactPolicy, owned.PrivacyPolicy)
	if !ok {
		return nil, nil, ErrPolicySourceChanged
	}
	policySourceFingerprint, ok := schema.PolicySourceFingerprint(owned.Vault, owned.ArtifactPolicy, owned.PrivacyPolicy)
	if !ok {
		return nil, nil, ErrPolicySourceChanged
	}
	identity, err := newGenerationIdentity(IdentityConfig{
		Dimension:               owned.Dimension,
		ChunkTokenCap:           owned.ChunkTokenCap,
		VaultRoot:               root,
		CorpusPolicyFingerprint: policyFingerprint,
	})
	if err != nil {
		return nil, nil, err
	}
	if locationErr := validateStoreLocation(identity.VaultRoot(), owned.StorePath); locationErr != nil {
		return nil, nil, locationErr
	}
	authorize, err := newChunkAuthorizer(
		owned.Vault,
		owned.ArtifactPolicy,
		owned.PrivacyPolicy,
		owned.ChunkTokenCap,
	)
	if err != nil {
		return nil, nil, err
	}
	workload, err := newTopKWorkload(owned.LatencyVectors, identity.Dimension())
	if err != nil {
		return nil, nil, err
	}
	readCorpus := func(ctx context.Context) (Corpus, error) {
		return readCorpus(ctx, owned.Vault, owned.ArtifactPolicy, owned.PrivacyPolicy, owned.ChunkTokenCap)
	}
	setup := &indexerSetup{
		settings: indexerSettings{
			storePath:               owned.StorePath,
			identity:                identity,
			policySourceFingerprint: policySourceFingerprint,
			topKWorkload:            workload,
		},
		deps: indexerDeps{
			readCorpus: readCorpus,
			validateQueryAuthority: func() error {
				if !owned.ArtifactPolicy.ValidateSource().Available() || !owned.PrivacyPolicy.ValidateSource().Available() {
					return ErrPolicySourceChanged
				}
				return nil
			},
			validateQueryCorpus: func(ctx context.Context, expected [sha256.Size]byte) error {
				current, err := readCorpus(ctx)
				if err != nil {
					return err
				}
				if current.Fingerprint != expected {
					return ErrVaultChanged
				}
				return nil
			},
		},
	}
	return setup, authorize, nil
}

func validateStoreLocation(root, storePath string) error {
	if !utf8.ValidString(storePath) || containsControl(storePath) ||
		!filepath.IsAbs(storePath) || filepath.Clean(storePath) != storePath {
		return fmt.Errorf("%w: store path must be a clean absolute path", errIndexerConfiguration)
	}
	if isWithinRoot(root, storePath) {
		return fmt.Errorf("%w: store path must be outside the vault", errIndexerConfiguration)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("%w: vault root cannot be resolved safely", errIndexerConfiguration)
	}
	resolvedStore, ok := resolvePlannedLocation(storePath)
	if !ok {
		return fmt.Errorf("%w: store path cannot be resolved safely", errIndexerConfiguration)
	}
	if isWithinRoot(resolvedRoot, resolvedStore) {
		return fmt.Errorf("%w: store path must resolve outside the vault", errIndexerConfiguration)
	}
	return nil
}

func resolvePlannedLocation(path string) (string, bool) {
	var suffix []string
	for {
		info, statErr := os.Lstat(path)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", false
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := range slices.Backward(suffix) {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), true
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", false
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func isWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(relative)
}

func newIndexerWithProvider(setup *indexerSetup, openProvider providerFactory, now buildClock, wait buildWait) (*Indexer, error) {
	if err := validateIndexer(setup, openProvider); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	if wait == nil {
		wait = waitForBuildRetry
	}
	workload, err := validateTopKWorkload(setup.settings.topKWorkload, setup.settings.identity.Dimension())
	if err != nil {
		return nil, err
	}
	if len(workload.QueryVectors) == 0 {
		return nil, fmt.Errorf("%w: top-k observer workload is required", errIndexerConfiguration)
	}
	ownedSettings := setup.settings
	ownedSettings.topKWorkload = workload
	ownedDeps := completeIndexerDeps(setup.deps)
	return &Indexer{
		settings:     ownedSettings,
		deps:         ownedDeps,
		openProvider: openProvider,
		now:          now,
		wait:         wait,
		measure:      measureTopK,
	}, nil
}

func validateIndexer(setup *indexerSetup, openProvider providerFactory) error {
	if setup == nil {
		return fmt.Errorf("%w: indexer configuration is nil", errIndexerConfiguration)
	}
	if setup.settings.storePath == "" || setup.deps.readCorpus == nil ||
		setup.deps.validateQueryAuthority == nil ||
		setup.settings.policySourceFingerprint == ([sha256.Size]byte{}) {
		return fmt.Errorf("%w: incomplete indexer configuration", errIndexerConfiguration)
	}
	if err := validateIdentity(&setup.settings.identity); err != nil {
		return err
	}
	if openProvider == nil {
		return fmt.Errorf("%w: provider factory is nil", ErrEmbedderConfiguration)
	}
	return nil
}

func completeIndexerDeps(deps indexerDeps) indexerDeps {
	if deps.validateQueryCorpus == nil {
		deps.validateQueryCorpus = func(ctx context.Context, expected [sha256.Size]byte) error {
			current, err := deps.readCorpus(ctx)
			if err != nil {
				return err
			}
			if current.Fingerprint != expected {
				return ErrVaultChanged
			}
			return nil
		}
	}
	if deps.newIndex == nil {
		deps.newIndex = newOwnedIndex
	}
	if deps.openWriter == nil {
		deps.openWriter = openWriter
	}
	return deps
}

// Build runs one explicit full-build action. The zero request may return a
// current generation without acquiring a writer or sending. Renewal instead
// requires an exhausted matching staging generation and never takes that early
// success path.
func (ix *Indexer) Build(ctx context.Context, request BuildRequest) (report BuildReport, resultErr error) {
	observation := buildObservation{}
	defer func() {
		resultErr = observation.wrap(resultErr)
	}()
	if request.RenewAttemptBudget {
		return ix.renewAttemptBudget(ctx, &observation)
	}
	corpus, err := ix.readCorpus(ctx)
	if err != nil {
		return BuildReport{}, err
	}
	generation, current, err := ix.currentGeneration(ctx, corpus)
	if err != nil && !errors.Is(err, ErrStoreCorrupt) && !errors.Is(err, ErrStoreSchemaMismatch) &&
		!errors.Is(err, ErrVectorFormatMismatch) {
		return BuildReport{}, err
	}
	if current {
		return newBuildReport(BuildCurrent, &generation.generationMetadata, &corpus, 0, len(corpus.Chunks))
	}
	return ix.buildGeneration(ctx, errors.Is(err, ErrStoreCorrupt), &observation)
}

func (ix *Indexer) renewAttemptBudget(
	ctx context.Context,
	observation *buildObservation,
) (report BuildReport, resultErr error) {
	writer, err := openRenewalWriter(ctx, ix.settings.storePath)
	if err != nil {
		return BuildReport{}, classifyRenewalOpenError(err)
	}
	published := false
	defer func() {
		resultErr = closeUnpublishedWriter(writer, published, resultErr)
	}()

	corpus, err := ix.readCorpus(ctx)
	if err != nil {
		return BuildReport{}, err
	}
	targets, chunks, err := buildTargets(corpus)
	if err != nil {
		return BuildReport{}, err
	}
	manifest, err := newStagingManifest(&ix.settings.identity, ix.settings.policySourceFingerprint, targets)
	if err != nil {
		return BuildReport{}, err
	}
	active, activeErr := writer.Active(ctx)
	observation.active = classifyBuildActive(&active, activeErr, &ix.settings.identity, corpus.Fingerprint)
	// Active-role corruption changes the recovery observation, not the
	// independently validated staging batch's renewal eligibility.
	if !renewalCanProceedPastActiveError(activeErr) {
		return BuildReport{}, errors.Join(ErrAttemptBudgetNotRenewable, activeErr)
	}
	inspection, err := writer.inspectStaging(ctx, manifest)
	observation.staging = inspection.state
	if err != nil {
		return BuildReport{}, errors.Join(ErrAttemptBudgetNotRenewable, err)
	}
	if inspection.state != StagingGenerationRequiresAuthorization {
		return BuildReport{}, ErrAttemptBudgetNotRenewable
	}
	writer.beforeRenewCommit = ix.deps.beforeRenewCommit
	build, err := writer.renewStagingGeneration(ctx, manifest, inspection.id, func(ctx context.Context) error {
		return ix.validateRenewalCommit(ctx, corpus.Fingerprint)
	})
	if err != nil {
		return BuildReport{}, err
	}
	observation.staging = StagingGenerationResumable
	if ix.deps.afterRenewCommit != nil {
		if hookErr := ix.deps.afterRenewCommit(); hookErr != nil {
			return BuildReport{}, hookErr
		}
	}
	pending, err := build.pending(ctx)
	if err != nil {
		return BuildReport{}, err
	}
	prepared := &preparedGeneration{
		corpus:  corpus,
		chunks:  chunks,
		staging: build,
		pending: pending,
		reused:  len(targets) - len(pending),
	}
	embedded, err := ix.embedFull(ctx, prepared, observation)
	if err != nil {
		return BuildReport{}, err
	}
	generation, err := ix.publishGeneration(ctx, prepared, embedded)
	if err != nil {
		return BuildReport{}, err
	}
	published = true
	observation.active = ActiveGenerationPreservedUsable
	observation.staging = StagingGenerationAbsent
	return newBuildReport(
		BuildPublished,
		&generation.metadata,
		&generation.corpus,
		generation.embedded,
		generation.reused,
	)
}

func (ix *Indexer) validateRenewalCommit(ctx context.Context, expected [sha256.Size]byte) error {
	current, err := ix.readCorpus(ctx)
	if err != nil {
		return err
	}
	if current.Fingerprint != expected {
		return ErrVaultChanged
	}
	return nil
}

func renewalCanProceedPastActiveError(err error) bool {
	return err == nil || errors.Is(err, ErrNoActiveGeneration) ||
		errors.Is(err, ErrVectorFormatMismatch) || errors.Is(err, ErrStoreCorrupt) ||
		errors.Is(err, ErrIndexCapacity)
}

func classifyRenewalOpenError(err error) error {
	if errors.Is(err, ErrStoreNotFound) || errors.Is(err, ErrStoreSchemaMismatch) ||
		errors.Is(err, ErrStoreCorrupt) || errors.Is(err, ErrVectorFormatMismatch) {
		return errors.Join(ErrAttemptBudgetNotRenewable, err)
	}
	return err
}

func newBuildReport(
	status BuildStatus,
	metadata *generationMetadata,
	corpus *Corpus,
	embedded int,
	reused int,
) (BuildReport, error) {
	if metadata == nil || corpus == nil || metadata.corpusFingerprint != corpus.Fingerprint {
		return BuildReport{}, ErrVaultChanged
	}
	if embedded < 0 || reused < 0 || embedded+reused != len(corpus.Chunks) ||
		status != BuildCurrent && status != BuildPublished || status == BuildCurrent && embedded != 0 {
		return BuildReport{}, ErrInvalidCorpus
	}
	return BuildReport{
		Status:   status,
		Chunks:   len(corpus.Chunks),
		Embedded: embedded,
		Reused:   reused,
		TopKP95:  metadata.topKP95,
	}, nil
}

func (ix *Indexer) buildGeneration(
	ctx context.Context,
	replaceCorruptActive bool,
	observation *buildObservation,
) (report BuildReport, resultErr error) {
	writer, err := openRebuildWriter(ctx, ix.settings.storePath)
	if err != nil {
		return BuildReport{}, err
	}
	published := false
	defer func() {
		resultErr = closeUnpublishedWriter(writer, published, resultErr)
	}()

	corpus, err := ix.readCorpus(ctx)
	if err != nil {
		return BuildReport{}, err
	}
	active, activeErr := writer.Active(ctx)
	observation.active = classifyBuildActive(&active, activeErr, &ix.settings.identity, corpus.Fingerprint)
	current, err := acceptBuildActive(&active, activeErr, &ix.settings.identity, corpus.Fingerprint, replaceCorruptActive)
	if err != nil {
		return BuildReport{}, err
	}
	if current {
		return newBuildReport(BuildCurrent, &active.generationMetadata, &corpus, 0, len(corpus.Chunks))
	}

	prepared, err := ix.prepareGeneration(ctx, writer, corpus, observation)
	if err != nil {
		return BuildReport{}, err
	}
	embedded, err := ix.embedFull(ctx, prepared, observation)
	if err != nil {
		return BuildReport{}, err
	}
	generation, err := ix.publishGeneration(ctx, prepared, embedded)
	if err != nil {
		return BuildReport{}, err
	}
	published = true
	observation.active = ActiveGenerationPreservedUsable
	observation.staging = StagingGenerationAbsent
	return newBuildReport(
		BuildPublished,
		&generation.metadata,
		&generation.corpus,
		generation.embedded,
		generation.reused,
	)
}

func classifyBuildActive(
	active *loadedGeneration,
	err error,
	identity *generationIdentity,
	corpusFingerprint [sha256.Size]byte,
) ActiveGenerationState {
	if errors.Is(err, ErrNoActiveGeneration) {
		return ActiveGenerationAbsent
	}
	if err != nil {
		return ActiveGenerationPreservedUnusable
	}
	if active != nil && active.identity == *identity && active.corpusFingerprint == corpusFingerprint &&
		active.topKP95 >= time.Microsecond {
		return ActiveGenerationPreservedUsable
	}
	return ActiveGenerationPreservedUnusable
}

func closeUnpublishedWriter(writer *writer, published bool, resultErr error) error {
	closeErr := writer.Close()
	if closeErr != nil && resultErr == nil && !published {
		return closeErr
	}
	return resultErr
}

func requireInteractiveWork(active *loadedGeneration, corpus Corpus) error {
	missing, proxyTokens, err := interactiveWork(active, corpus)
	if err != nil {
		return err
	}
	if missing > maxInteractiveChunks || proxyTokens > maxInteractiveProxyTokens {
		return ErrRebuildRequired
	}
	return nil
}

func acceptBuildActive(
	active *loadedGeneration,
	activeErr error,
	identity *generationIdentity,
	corpusFingerprint [sha256.Size]byte,
	replaceCorrupt bool,
) (bool, error) {
	if activeErr == nil {
		return active.identity == *identity && active.corpusFingerprint == corpusFingerprint &&
			active.topKP95 >= time.Microsecond, nil
	}
	if errors.Is(activeErr, ErrNoActiveGeneration) ||
		errors.Is(activeErr, ErrVectorFormatMismatch) ||
		replaceCorrupt && errors.Is(activeErr, ErrStoreCorrupt) {
		return false, nil
	}
	return false, activeErr
}

type preparedGeneration struct {
	corpus  Corpus
	chunks  map[chunkKey]*CorpusChunk
	staging *staging
	pending []ChunkTarget
	reused  int
}

func (ix *Indexer) prepareGeneration(
	ctx context.Context,
	writer *writer,
	corpus Corpus,
	observation *buildObservation,
) (*preparedGeneration, error) {
	targets, chunks, err := buildTargets(corpus)
	if err != nil {
		return nil, err
	}
	build, err := writer.prepare(ctx, &ix.settings.identity, ix.settings.policySourceFingerprint, targets)
	if err != nil {
		return nil, err
	}
	pending, err := build.pending(ctx)
	if err != nil {
		return nil, err
	}
	exhausted, err := build.hasExhaustedPending(ctx)
	if err != nil {
		return nil, err
	}
	if exhausted {
		observation.staging = StagingGenerationRequiresAuthorization
		return nil, ErrAttemptLimit
	}
	observation.staging = StagingGenerationResumable
	return &preparedGeneration{
		corpus:  corpus,
		chunks:  chunks,
		staging: build,
		pending: pending,
		reused:  len(targets) - len(pending),
	}, nil
}

func (ix *Indexer) embedInteractive(ctx context.Context, prepared *preparedGeneration, provider *actionProvider) (int, error) {
	if len(prepared.pending) == 0 {
		return 0, nil
	}
	for i := range prepared.pending {
		target := &prepared.pending[i]
		chunk, err := prepared.chunk(target)
		if err != nil {
			return 0, err
		}
		if _, reserveErr := prepared.staging.reserveAttempt(ctx, target.RelPath, target.Ordinal, ix.now()); reserveErr != nil {
			return 0, reserveErr
		}
		embedding, sendErr := provider.embedChunk(ctx, chunk)
		if sendErr != nil {
			return 0, sendErr
		}
		if err := completePreparedChunk(ctx, prepared.staging, chunk, embedding); err != nil {
			return 0, err
		}
	}
	return len(prepared.pending), nil
}

func (ix *Indexer) embedFull(
	ctx context.Context,
	prepared *preparedGeneration,
	observation *buildObservation,
) (int, error) {
	if len(prepared.pending) == 0 {
		return 0, nil
	}
	provider, err := ix.openActionProvider()
	if err != nil {
		return 0, err
	}
	for i := range prepared.pending {
		chunk, err := prepared.chunk(&prepared.pending[i])
		if err != nil {
			return 0, err
		}
		embedding, attempt, err := ix.embedFullBuildChunk(
			ctx,
			prepared.staging,
			chunk,
			provider.embedChunk,
			observation,
		)
		if err != nil {
			return 0, err
		}
		if err := completePreparedChunk(ctx, prepared.staging, chunk, embedding); err != nil {
			return 0, err
		}
		if attempt == 5 {
			observation.staging = StagingGenerationResumable
		}
		embedded := i + 1
		if ix.buildProgress != nil && (embedded%100 == 0 || embedded == len(prepared.pending)) {
			if err := ix.buildProgress(embedded, len(prepared.pending)); err != nil {
				return 0, fmt.Errorf("report semantic build progress: %w", err)
			}
		}
	}
	return len(prepared.pending), nil
}

func (p *preparedGeneration) chunk(target *ChunkTarget) (*CorpusChunk, error) {
	chunk, ok := p.chunks[chunkKey{relPath: target.RelPath, ordinal: target.Ordinal}]
	if !ok || chunk.SubmittedHash != target.SubmittedHash || chunk.NoteHash != target.NoteHash {
		return nil, fmt.Errorf("%w: pending target has no current chunk", ErrInvalidCorpus)
	}
	return chunk, nil
}

func completePreparedChunk(
	ctx context.Context,
	staging *staging,
	chunk *CorpusChunk,
	embedding EmbeddingResult,
) error {
	row, err := chunk.Complete(embedding)
	if err != nil {
		return fmt.Errorf("%w: complete embedded chunk: %w", ErrGenerationIncomplete, err)
	}
	if err := staging.put(ctx, &row); err != nil {
		return fmt.Errorf("%w: store embedded chunk: %w", ErrGenerationIncomplete, err)
	}
	return nil
}

func (ix *Indexer) publishGeneration(
	ctx context.Context,
	prepared *preparedGeneration,
	embedded int,
) (publishedGeneration, error) {
	var finalCorpus Corpus
	var index *Index
	generation, err := prepared.staging.activateGeneration(ctx, func(rows []ChunkVector) (time.Duration, error) {
		var observeErr error
		index, observeErr = ix.prepareTopKObserver(rows)
		if observeErr != nil {
			return 0, observeErr
		}
		finalCorpus, observeErr = ix.readCorpus(ctx)
		if observeErr != nil {
			return 0, observeErr
		}
		if finalCorpus.Fingerprint != prepared.corpus.Fingerprint {
			return 0, ErrVaultChanged
		}
		if len(ix.settings.topKWorkload.QueryVectors) == 0 {
			return 0, nil
		}
		return ix.measureWorkloadP95(ctx, index)
	})
	if err != nil {
		return publishedGeneration{}, err
	}
	return publishedGeneration{
		metadata: generation,
		corpus:   finalCorpus,
		index:    index,
		embedded: embedded,
		reused:   prepared.reused,
	}, nil
}

func (ix *Indexer) openActionProvider() (*actionProvider, error) {
	provider, err := ix.openProvider()
	if err != nil {
		return nil, err
	}
	if provider == nil || provider.embedChunk == nil || provider.embedQuery == nil {
		return nil, fmt.Errorf("%w: incomplete provider", ErrEmbedderConfiguration)
	}
	return provider, nil
}

func (ix *Indexer) embedFullBuildChunk(
	ctx context.Context,
	staging *staging,
	chunk *CorpusChunk,
	send chunkSender,
	observation *buildObservation,
) (EmbeddingResult, int, error) {
	for {
		attempt, err := staging.reserveAttempt(ctx, chunk.RelPath, chunk.Ordinal, ix.now())
		if err != nil {
			return EmbeddingResult{}, 0, err
		}
		if attempt == 5 {
			observation.staging = StagingGenerationRequiresAuthorization
		}
		result, err := send(ctx, chunk)
		if err == nil {
			return result, attempt, nil
		}
		embedErr, terminalErr := classifyFullBuildFailure(err, attempt)
		if terminalErr != nil {
			return EmbeddingResult{}, attempt, terminalErr
		}
		delay := fullBuildRetrySchedule[attempt-1]
		if embedErr.RetryAfterValid {
			if embedErr.RetryAfter < 0 {
				return EmbeddingResult{}, attempt, err
			}
			delay = embedErr.RetryAfter
		}
		retryAt := ix.now().Add(delay)
		if err := staging.deferRetry(ctx, chunk.RelPath, chunk.Ordinal, retryAt); err != nil {
			return EmbeddingResult{}, attempt, err
		}
		if delay > 30*time.Second {
			return EmbeddingResult{}, attempt, &retryNotReadyError{At: retryAt}
		}
		if err := ix.wait(ctx, delay); err != nil {
			return EmbeddingResult{}, attempt, err
		}
	}
}

func classifyFullBuildFailure(err error, attempt int) (*EmbedError, error) {
	embedErr, ok := errors.AsType[*EmbedError](err)
	if attempt >= 5 {
		if ok && providerAvailabilityFailure(embedErr.Kind) {
			return nil, errors.Join(ErrAttemptLimit, err)
		}
		return nil, err
	}
	if !ok || embedErr.Kind != EmbedFailureRateLimited {
		return nil, err
	}
	return embedErr, nil
}

func providerAvailabilityFailure(kind EmbedFailureKind) bool {
	switch kind {
	case EmbedFailureUnreachable, EmbedFailureRateLimited, EmbedFailureProvider:
		return true
	case EmbedFailureRejected, EmbedFailureMalformedRequest, EmbedFailureInternal:
		return false
	default:
		return true
	}
}

func buildTargets(corpus Corpus) ([]ChunkTarget, map[chunkKey]*CorpusChunk, error) {
	targets := make([]ChunkTarget, 0, len(corpus.Chunks))
	chunks := make(map[chunkKey]*CorpusChunk, len(corpus.Chunks))
	manifest := make([]ChunkVector, 0, len(corpus.Chunks))
	for i := range corpus.Chunks {
		chunk := &corpus.Chunks[i]
		target, err := chunk.Target()
		if err != nil {
			return nil, nil, err
		}
		key := chunkKey{relPath: target.RelPath, ordinal: target.Ordinal}
		if _, exists := chunks[key]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate chunk target", ErrInvalidCorpus)
		}
		chunks[key] = chunk
		targets = append(targets, target)
		manifest = append(manifest, target.complete(nil))
	}
	fingerprint, err := CorpusFingerprint(manifest)
	if err != nil {
		return nil, nil, err
	}
	if fingerprint != corpus.Fingerprint {
		return nil, nil, fmt.Errorf("%w: corpus fingerprint does not match chunks", ErrInvalidCorpus)
	}
	return targets, chunks, nil
}

func newTopKWorkload(vectors [][]float32, dimension int) (topKWorkload, error) {
	if len(vectors) == 0 {
		return topKWorkload{}, nil
	}
	compatibility, err := QueryVectorCompatibilityIdentity(dimension)
	if err != nil {
		return topKWorkload{}, err
	}
	return validateTopKWorkload(topKWorkload{
		CompatibilityIdentity: compatibility,
		QueryVectors:          vectors,
	}, dimension)
}

func validateTopKWorkload(workload topKWorkload, dimension int) (topKWorkload, error) {
	if workload.CompatibilityIdentity == ([sha256.Size]byte{}) && len(workload.QueryVectors) == 0 {
		return topKWorkload{}, nil
	}
	compatibility, err := QueryVectorCompatibilityIdentity(dimension)
	if err != nil {
		return topKWorkload{}, err
	}
	if workload.CompatibilityIdentity != compatibility || len(workload.QueryVectors) != topKObserverQueryCount {
		return topKWorkload{}, fmt.Errorf("%w: incompatible top-k observer workload", errIndexerConfiguration)
	}
	owned := topKWorkload{
		CompatibilityIdentity: workload.CompatibilityIdentity,
		QueryVectors:          make([][]float32, len(workload.QueryVectors)),
	}
	for i := range workload.QueryVectors {
		if len(workload.QueryVectors[i]) != dimension {
			return topKWorkload{}, fmt.Errorf("%w: top-k query vector dimension", errIndexerConfiguration)
		}
		if _, ok := vectorMagnitude(workload.QueryVectors[i]); !ok {
			return topKWorkload{}, fmt.Errorf("%w: invalid top-k query vector", errIndexerConfiguration)
		}
		owned.QueryVectors[i] = slices.Clone(workload.QueryVectors[i])
	}
	return owned, nil
}

func (ix *Indexer) prepareTopKObserver(rows []ChunkVector) (*Index, error) {
	index, err := ix.deps.newIndex(rows, ix.settings.identity.Dimension())
	if err != nil {
		return nil, fmt.Errorf("%w: prepare query index: %w", ErrGenerationIncomplete, err)
	}
	return index, nil
}

func (ix *Indexer) measureWorkloadP95(ctx context.Context, index *Index) (time.Duration, error) {
	durations := make([]time.Duration, 0, topKObserverQueryCount*topKObserverRepeats)
	for queryIndex := range ix.settings.topKWorkload.QueryVectors {
		query := ix.settings.topKWorkload.QueryVectors[queryIndex]
		for repetition := range topKObserverRepeats {
			if contextErr := ctx.Err(); contextErr != nil {
				return 0, contextErr
			}
			duration, measureErr := ix.measure(func() error {
				_, rankErr := index.TopNotes(query, nil, 50)
				return rankErr
			})
			if measureErr != nil {
				return 0, measureErr
			}
			if queryIndex != 0 || repetition != 0 {
				durations = append(durations, duration)
			}
		}
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return 0, contextErr
	}
	duration, measureErr := ix.measure(func() error {
		_, rankErr := index.TopNotes(ix.settings.topKWorkload.QueryVectors[0], nil, 50)
		return rankErr
	})
	if measureErr != nil {
		return 0, measureErr
	}
	durations = append(durations, duration)
	slices.Sort(durations)
	nearestRank := (95*len(durations) + 99) / 100
	p95 := durations[nearestRank-1]
	if p95 < time.Microsecond {
		return time.Microsecond, nil
	}
	return p95, nil
}

func measureTopK(run func() error) (time.Duration, error) {
	started := time.Now()
	err := run()
	return time.Since(started), err
}

func interactiveWork(generation *loadedGeneration, corpus Corpus) (missing, proxyTokens int, resultErr error) {
	if _, _, targetErr := buildTargets(corpus); targetErr != nil {
		return 0, 0, targetErr
	}
	reusable := make(map[[sha256.Size]byte]struct{}, len(generation.rows))
	for i := range generation.rows {
		reusable[generation.rows[i].SubmittedHash] = struct{}{}
	}
	maxInt := int(^uint(0) >> 1)
	for i := range corpus.Chunks {
		chunk := &corpus.Chunks[i]
		if _, ok := reusable[chunk.SubmittedHash]; ok {
			continue
		}
		if chunk.ProxyTokens < 0 || chunk.ProxyTokens > maxInt-proxyTokens {
			return 0, 0, fmt.Errorf("%w: invalid interactive proxy-token total", ErrInvalidCorpus)
		}
		missing++
		proxyTokens += chunk.ProxyTokens
	}
	return missing, proxyTokens, nil
}

func (ix *Indexer) readCorpus(ctx context.Context) (Corpus, error) {
	if ix == nil || ix.deps.readCorpus == nil {
		return Corpus{}, ErrInvalidCorpus
	}
	if err := ctx.Err(); err != nil {
		return Corpus{}, err
	}
	corpus, err := ix.deps.readCorpus(ctx)
	if err != nil {
		return Corpus{}, err
	}
	if !withinExactScanCapacity(len(corpus.Chunks), ix.settings.identity.Dimension()) {
		return Corpus{}, ErrIndexCapacity
	}
	if err := ctx.Err(); err != nil {
		return Corpus{}, err
	}
	return corpus, nil
}

func (ix *Indexer) validateCorpus(ctx context.Context, corpus Corpus) error {
	if ix == nil {
		return ErrInvalidCorpus
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, _, err := buildTargets(corpus); err != nil {
		return err
	}
	if !withinExactScanCapacity(len(corpus.Chunks), ix.settings.identity.Dimension()) {
		return ErrIndexCapacity
	}
	return nil
}

func (ix *Indexer) currentGeneration(ctx context.Context, corpus Corpus) (loadedGeneration, bool, error) {
	generation, err := ix.activeGeneration(ctx)
	if errors.Is(err, ErrStoreNotFound) || errors.Is(err, ErrNoActiveGeneration) {
		return loadedGeneration{}, false, nil
	}
	if err != nil {
		return loadedGeneration{}, false, err
	}
	return generation, generation.identity == ix.settings.identity &&
		generation.corpusFingerprint == corpus.Fingerprint && generation.topKP95 >= time.Microsecond, nil
}

func (ix *Indexer) activeGeneration(ctx context.Context) (loadedGeneration, error) {
	store, err := openStore(ctx, ix.settings.storePath)
	if err != nil {
		return loadedGeneration{}, err
	}
	generation, err := store.Active(ctx)
	closeErr := store.Close()
	if err != nil {
		return loadedGeneration{}, err
	}
	if closeErr != nil {
		return loadedGeneration{}, closeErr
	}
	return generation, nil
}

func waitForBuildRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
