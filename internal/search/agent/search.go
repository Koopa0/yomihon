package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

const (
	defaultSearchLimit = 20
)

type searchArgs struct {
	root     string
	limit    int
	semantic bool
	json     bool
	query    string
}

type searchDeps struct {
	stdout                  io.Writer
	stderr                  io.Writer
	openSemantic            func(semanticActionConfig) (reconcileSearch, error)
	beforeCapabilityRecheck func()
}

type reconcileSearch func(context.Context, semantic.Corpus) (SemanticSearch, error)

type searchFlagsSeen struct {
	rootSet  bool
	limitSet bool
}

type preparedSearch struct {
	parsed      searchArgs
	reader      *vault.Reader
	query       search.Query
	artifact    schema.ArtifactPolicy
	privacy     schema.PrivacyPolicy
	snapshot    *Snapshot
	lexical     []search.Result
	semanticErr error
}

type semanticFailureSource uint8

const (
	semanticFailureReconcile semanticFailureSource = 1
	semanticFailureQuery     semanticFailureSource = 2
)

func parseSearchArgs(args []string) (searchArgs, error) {
	if !validCLIArgs(args) {
		return searchArgs{}, errors.New("arguments must be valid UTF-8")
	}
	parsed := searchArgs{limit: defaultSearchLimit}
	positionals := make([]string, 0, len(args))
	cursor := cliArgCursor{args: args, parsingFlags: true}
	var seen searchFlagsSeen
	for {
		arg, flagPosition, ok := cursor.next()
		if !ok {
			break
		}
		if !flagPosition || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		if err := parseSearchFlag(&parsed, &seen, &cursor, arg); err != nil {
			return parsed, err
		}
	}
	return finishSearchArgs(parsed, positionals)
}

func parseSearchFlag(parsed *searchArgs, seen *searchFlagsSeen, cursor *cliArgCursor, arg string) error {
	name, _, hasInline := strings.Cut(arg, "=")
	switch name {
	case "--semantic":
		if hasInline {
			return fmt.Errorf("flag %s takes no value", name)
		}
		parsed.semantic = true
		return nil
	case "--json":
		if hasInline {
			return fmt.Errorf("flag %s takes no value", name)
		}
		parsed.json = true
		return nil
	case "--root", "--limit":
		return parseSearchValueFlag(parsed, seen, cursor, arg)
	default:
		return errors.New("unknown flag")
	}
}

func parseSearchValueFlag(parsed *searchArgs, seen *searchFlagsSeen, cursor *cliArgCursor, arg string) error {
	name, inline, hasInline := strings.Cut(arg, "=")
	if name == "--root" && seen.rootSet || name == "--limit" && seen.limitSet {
		return fmt.Errorf("flag %s specified more than once", name)
	}
	value, rest, err := takeFlagValue(name, inline, hasInline, cursor.args)
	if err != nil {
		return err
	}
	cursor.args = rest
	if name == "--root" {
		if validationErr := validateCLIRoot(value); validationErr != nil {
			return validationErr
		}
		parsed.root = value
		seen.rootSet = true
		return nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 1000 {
		return errors.New("--limit must be an integer from 1 to 1000")
	}
	parsed.limit = limit
	seen.limitSet = true
	return nil
}

func finishSearchArgs(parsed searchArgs, positionals []string) (searchArgs, error) {
	if len(positionals) == 0 {
		return parsed, errors.New("query is required")
	}
	parsed.query = strings.Join(positionals, " ")
	if len(parsed.query) > maxCLIInputBytes {
		return parsed, fmt.Errorf("query exceeds %d bytes", maxCLIInputBytes)
	}
	if containsCLIControl(parsed.query) {
		return parsed, errors.New("query contains control characters")
	}
	return parsed, nil
}

func runSearch(ctx context.Context, args []string, config rootConfig, deps searchDeps) int {
	parsed, err := parseSearchArgs(args)
	if err != nil {
		return writeSearchToolError(deps.stderr, err)
	}
	prepared, exit := prepareSearch(ctx, parsed, config, deps)
	if prepared == nil {
		return exit
	}
	defer releaseVaultReader(prepared.reader)
	return runPreparedSearch(ctx, prepared, deps)
}

func prepareSearch(ctx context.Context, parsed searchArgs, config rootConfig, deps searchDeps) (prepared *preparedSearch, exit int) {
	root, err := resolveRoot(parsed.root, config)
	if err != nil {
		return nil, writeSearchToolError(deps.stderr, err)
	}
	capabilities, err := loadVaultCapabilities(ctx, root)
	if err != nil {
		return nil, writeSearchToolError(deps.stderr, errors.New("cannot read vault"))
	}
	keepReader := false
	defer func() {
		if !keepReader {
			releaseVaultReader(capabilities.reader)
		}
	}()
	privacy := capabilities.privacy.ValidateSource()
	if !privacy.Available() {
		return nil, writeSearchRefusal(
			deps,
			parsed.json,
			searchReasonPrivacyUnavailable,
			privacyGateNotice("yomihon search", capabilities.governed),
		)
	}
	query := search.Parse(parsed.query)
	artifact := capabilities.artifact.ValidateSource()
	if query.RequiresMetadata() && !artifact.Available() {
		return nil, writeSearchUnavailable(deps, parsed.json, searchReasonMetadataUnavailable)
	}
	withSemantic := parsed.semantic && query.BareText() != "" && artifact.Available()
	captured, err := readSnapshot(ctx, capabilities.reader, artifact, privacy, withSemantic)
	if err != nil {
		return nil, writeSearchToolError(deps.stderr, errors.New("cannot read search snapshot"))
	}
	lexical, err := captured.snapshot.lexical.Search(query)
	if err != nil {
		if errors.Is(err, search.ErrMetadataUnavailable) {
			return nil, writeSearchUnavailable(deps, parsed.json, searchReasonMetadataUnavailable)
		}
		return nil, writeSearchToolError(deps.stderr, errors.New("cannot run lexical search"))
	}
	prepared = &preparedSearch{
		parsed:      parsed,
		reader:      capabilities.reader,
		query:       query,
		artifact:    artifact,
		privacy:     privacy,
		snapshot:    captured.snapshot,
		lexical:     lexical,
		semanticErr: captured.semanticErr,
	}
	keepReader = true
	return prepared, 0
}

func runPreparedSearch(ctx context.Context, prepared *preparedSearch, deps searchDeps) int {
	parsed := prepared.parsed
	if !parsed.semantic {
		if exit, failed := writeCurrentCapabilityFailure(deps, prepared, false); failed {
			return exit
		}
		answer := searchAnswerEnvelope{
			Mode:     searchModeLexical,
			Semantic: searchSemanticOff,
			Results:  lexicalWireResults(prepared.lexical, parsed.limit),
		}
		return writeSearchAnswer(deps, parsed.json, answer, 0, "")
	}
	if prepared.query.BareText() == "" {
		if exit, failed := writeCurrentCapabilityFailure(deps, prepared, false); failed {
			return exit
		}
		answer := searchAnswerEnvelope{
			Mode:     searchModeLexical,
			Semantic: searchSemanticNotApplicable,
			Coverage: &searchCoverage{Reason: searchReasonNotApplicable},
			Results:  lexicalWireResults(prepared.lexical, parsed.limit),
		}
		return writeSearchAnswer(deps, parsed.json, answer, 0, "")
	}
	if !prepared.artifact.Available() {
		if exit, failed := writeCurrentCapabilityFailure(deps, prepared, true); failed {
			return exit
		}
		return writeSemanticUnavailable(
			deps,
			parsed.json,
			prepared.lexical,
			parsed.limit,
			searchReasonArtifactPolicyUnavailable,
		)
	}
	return runSemanticSearch(ctx, prepared, deps)
}

func runSemanticSearch(ctx context.Context, prepared *preparedSearch, deps searchDeps) int {
	parsed := prepared.parsed
	if prepared.semanticErr != nil {
		return writeSearchSemanticFailure(deps, prepared, prepared.semanticErr, semanticFailureReconcile)
	}
	if deps.openSemantic == nil {
		return writeSearchToolError(deps.stderr, errors.New("semantic command is not wired"))
	}
	reconcile, err := deps.openSemantic(semanticActionConfig{
		reader:   prepared.reader,
		artifact: prepared.artifact,
		privacy:  prepared.privacy,
	})
	if err != nil {
		if errors.Is(err, semantic.ErrPolicySourceChanged) {
			return writeSearchSemanticFailure(deps, prepared, err, semanticFailureReconcile)
		}
		return writeSearchToolError(deps.stderr, errors.New("semantic command setup failed"))
	}
	if reconcile == nil {
		return writeSearchToolError(deps.stderr, errors.New("semantic command setup failed"))
	}
	ready, err := reconcile(ctx, prepared.snapshot.semantic)
	if err != nil {
		return writeSearchSemanticFailure(deps, prepared, err, semanticFailureReconcile)
	}
	if ready == nil {
		return writeSearchToolError(deps.stderr, errors.New("semantic action failed"))
	}
	searchAction, err := NewSearch(prepared.snapshot, ready)
	if err != nil {
		return writeSearchSemanticFailure(deps, prepared, err, semanticFailureQuery)
	}
	fused, err := searchAction.Run(ctx, prepared.query, parsed.limit)
	if err != nil {
		return writeSearchSemanticFailure(deps, prepared, err, semanticFailureQuery)
	}
	if exit, failed := writeCurrentCapabilityFailure(deps, prepared, true); failed {
		return exit
	}
	answer := searchAnswerEnvelope{
		Mode:     searchModeHybrid,
		Semantic: searchSemanticOK,
		Results:  hybridWireResults(fused),
	}
	return writeSearchAnswer(deps, parsed.json, answer, 0, "")
}

func classifySearchSemanticError(err error, source semanticFailureSource) commandFailure {
	if errors.Is(err, semantic.ErrAttemptLimit) {
		return unavailableFailure(searchReasonAttemptBudgetExhausted)
	}
	if embedErr, ok := errors.AsType[*semantic.EmbedError](err); ok {
		return classifyEmbedError(embedErr)
	}
	if failure, ok := classifySemanticActionError(err); ok {
		return failure
	}
	if source == semanticFailureReconcile {
		return classifyReconcileStateError(err)
	}
	return commandFailure{kind: commandFailureTool}
}

func classifySemanticActionError(err error) (commandFailure, bool) {
	switch {
	case isInternalSearchError(err):
		return commandFailure{kind: commandFailureInternal}, true
	case matchesAnyError(err, semantic.ErrPrivacyPolicyUnavailable, semantic.ErrPolicySourceChanged):
		return commandFailure{kind: commandFailureUnanswerable, reason: searchReasonPrivacyUnavailable}, true
	case errors.Is(err, semantic.ErrArtifactPolicyUnavailable):
		return unavailableFailure(searchReasonArtifactPolicyUnavailable), true
	case errors.Is(err, semantic.ErrEmbedderUnconfigured):
		return unavailableFailure(searchReasonEmbedderUnconfigured), true
	case errors.Is(err, semantic.ErrRebuildRequired):
		return unavailableFailure(searchReasonRebuildRequired), true
	case matchesAnyError(err, semantic.ErrWriterHeld, semantic.ErrStagingClosed):
		return unavailableFailure(searchReasonIndexRefreshing), true
	case matchesAnyError(err, semantic.ErrCorpusRead, semantic.ErrCorpusChunking, semantic.ErrInvalidCorpus, semantic.ErrGenerationIncomplete):
		return unavailableFailure(searchReasonIndexIncomplete), true
	case matchesAnyError(err, semantic.ErrVaultChanged, semantic.ErrChunkEgressDenied, semantic.ErrSourceNoteChanged, semantic.ErrSourceNoteUnavailable):
		return unavailableFailure(searchReasonVaultChanged), true
	case errors.Is(err, semantic.ErrRetryNotReady):
		return unavailableFailure(searchReasonRateLimited), true
	case errors.Is(err, semantic.ErrAttemptLimit):
		return unavailableFailure(searchReasonAttemptBudgetExhausted), true
	default:
		return commandFailure{}, false
	}
}

func isInternalSearchError(err error) bool {
	return matchesAnyError(
		err,
		ErrEvidenceMismatch,
		ErrInvalidSnapshot,
		ErrInvalidSearch,
		semantic.ErrInvalidSearchState,
		semantic.ErrQueryAlreadyAttempted,
		semantic.ErrQueryNotApplicable,
	)
}

func classifyReconcileStateError(err error) commandFailure {
	switch {
	case errors.Is(err, semantic.ErrStoreUnsupportedPlatform):
		return unavailableFailure(searchReasonUnsupportedPlatform)
	case matchesAnyError(err, semantic.ErrStoreNotFound, semantic.ErrNoActiveGeneration):
		return unavailableFailure(searchReasonCacheCold)
	case matchesAnyError(err, semantic.ErrStoreCorrupt, semantic.ErrInvalidChunk, semantic.ErrDimensionMismatch, semantic.ErrZeroVector, semantic.ErrDuplicateChunk, semantic.ErrGenerationUnmeasured):
		return unavailableFailure(searchReasonCacheCorrupt)
	case matchesAnyError(err, semantic.ErrStoreSchemaMismatch, semantic.ErrVectorFormatMismatch, semantic.ErrGenerationMismatch, semantic.ErrInvalidIdentity):
		return unavailableFailure(searchReasonCacheMismatch)
	case errors.Is(err, semantic.ErrEmbedderRetired):
		return unavailableFailure(searchReasonEmbedderRetired)
	case errors.Is(err, semantic.ErrIndexCapacity):
		return unavailableFailure(searchReasonCapacity)
	default:
		return commandFailure{kind: commandFailureTool}
	}
}

func writeSemanticUnavailable(
	deps searchDeps,
	jsonOutput bool,
	lexical []search.Result,
	limit int,
	reason searchReason,
) int {
	answer := searchAnswerEnvelope{
		Mode:     searchModeLexical,
		Semantic: searchSemanticUnavailable,
		Coverage: &searchCoverage{Reason: reason},
		Results:  lexicalWireResults(lexical, limit),
	}
	return writeSearchAnswer(deps, jsonOutput, answer, 3, reason)
}

func writeSearchSemanticFailure(
	deps searchDeps,
	prepared *preparedSearch,
	err error,
	source semanticFailureSource,
) int {
	if exit, failed := writeCurrentCapabilityFailure(deps, prepared, true); failed {
		return exit
	}
	failure := classifySearchSemanticError(err, source)
	switch failure.kind {
	case commandFailureUnavailable:
		return writeSemanticUnavailable(
			deps,
			prepared.parsed.json,
			prepared.lexical,
			prepared.parsed.limit,
			failure.reason,
		)
	case commandFailureUnanswerable:
		return writeSearchUnavailable(deps, prepared.parsed.json, failure.reason)
	case commandFailureInternal:
		return writeSearchInternal(deps, prepared.parsed.json)
	case commandFailureTool:
		return writeSearchToolError(deps.stderr, errors.New("semantic action failed"))
	default:
		return writeSearchToolError(deps.stderr, errors.New("semantic action failed"))
	}
}

func writeCurrentCapabilityFailure(
	deps searchDeps,
	prepared *preparedSearch,
	semanticRequired bool,
) (exit int, failed bool) {
	callBeforeCapabilityRecheck(deps.beforeCapabilityRecheck)
	if !prepared.privacy.ValidateSource().Available() {
		return writeSearchUnavailable(deps, prepared.parsed.json, searchReasonPrivacyUnavailable), true
	}
	if !prepared.artifact.ValidateSource().Available() {
		if prepared.query.RequiresMetadata() {
			return writeSearchUnavailable(deps, prepared.parsed.json, searchReasonMetadataUnavailable), true
		}
		if semanticRequired {
			return writeSemanticUnavailable(
				deps,
				prepared.parsed.json,
				prepared.lexical,
				prepared.parsed.limit,
				searchReasonArtifactPolicyUnavailable,
			), true
		}
	}
	return 0, false
}

func writeSearchUnavailable(deps searchDeps, jsonOutput bool, reason searchReason) int {
	line, err := searchStderrLine(reason)
	if err != nil {
		return writeSearchToolError(deps.stderr, errors.New("classify command failure"))
	}
	return writeSearchRefusal(deps, jsonOutput, reason, line)
}

// writeSearchRefusal emits one closed-capability answer: the frozen envelope a
// program reads, then whatever a person is told about it. The notice is a
// parameter because a refusal decided before any work began can say more than
// the same refusal arriving mid-flight, where the capability was there when the
// command started and went away under it.
func writeSearchRefusal(deps searchDeps, jsonOutput bool, reason searchReason, notice string) int {
	if jsonOutput {
		if err := writeSearchJSON(deps.stdout, searchErrorEnvelope{Error: searchError{Reason: reason}}); err != nil {
			return writeSearchToolError(deps.stderr, errors.New("write output"))
		}
	}
	if err := writeSearchText(deps.stderr, notice); err != nil {
		return 2
	}
	return 3
}

func writeSearchAnswer(
	deps searchDeps,
	jsonOutput bool,
	answer searchAnswerEnvelope,
	exit int,
	reason searchReason,
) int {
	var err error
	if jsonOutput {
		err = writeSearchJSON(deps.stdout, answer)
	} else {
		err = writeSearchHuman(deps.stdout, answer)
	}
	if err != nil {
		return writeSearchToolError(deps.stderr, errors.New("write output"))
	}
	if exit == 3 {
		line, lineErr := searchStderrLine(reason)
		if lineErr != nil {
			return writeSearchToolError(deps.stderr, errors.New("classify command failure"))
		}
		if writeErr := writeSearchText(deps.stderr, line); writeErr != nil {
			return 2
		}
	}
	return exit
}

func writeSearchInternal(deps searchDeps, jsonOutput bool) int {
	if jsonOutput {
		if err := writeSearchJSON(deps.stdout, newSearchInternalEnvelope()); err != nil {
			return writeSearchToolError(deps.stderr, errors.New("write output"))
		}
	}
	if err := writeSearchText(deps.stderr, searchInternalStderrLine()); err != nil {
		return 2
	}
	return 1
}

func writeSearchToolError(stderr io.Writer, err error) int {
	if stderr != nil {
		if writeErr := writeSearchText(stderr, fmt.Sprintf("yomihon search: %v\n", err)); writeErr != nil {
			return 2
		}
	}
	return 2
}
