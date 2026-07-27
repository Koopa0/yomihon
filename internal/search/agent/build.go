package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search/semantic"
)

type buildArgs struct {
	root               string
	json               bool
	renewAttemptBudget bool
}

type buildCommandDeps struct {
	stdout                  io.Writer
	stderr                  io.Writer
	openBuilder             func(semanticActionConfig) (generationBuilder, error)
	beforeCapabilityRecheck func()
}

type generationBuilder interface {
	Build(ctx context.Context, request semantic.BuildRequest) (semantic.BuildReport, error)
}

func parseBuildArgs(args []string) (buildArgs, error) {
	if !validCLIArgs(args) {
		return buildArgs{}, errors.New("arguments must be valid UTF-8")
	}
	var parsed buildArgs
	if len(args) == 0 {
		return parsed, errors.New("subcommand is required; use build")
	}
	if args[0] != "build" {
		return parsed, errors.New("unknown subcommand; use build")
	}
	cursor := cliArgCursor{args: args[1:], parsingFlags: true}
	rootSet := false
	for {
		arg, flagPosition, ok := cursor.next()
		if !ok {
			break
		}
		if !flagPosition || !strings.HasPrefix(arg, "-") {
			return parsed, errors.New("build takes no arguments")
		}
		if err := parseBuildFlag(&parsed, &rootSet, &cursor, arg); err != nil {
			return parsed, err
		}
	}
	return parsed, nil
}

func parseBuildFlag(parsed *buildArgs, rootSet *bool, cursor *cliArgCursor, arg string) error {
	name, inline, hasInline := strings.Cut(arg, "=")
	switch name {
	case "--json":
		if hasInline {
			return fmt.Errorf("flag %s takes no value", name)
		}
		parsed.json = true
		return nil
	case "--renew-attempt-budget":
		if hasInline {
			return fmt.Errorf("flag %s takes no value", name)
		}
		parsed.renewAttemptBudget = true
		return nil
	case "--root":
		if *rootSet {
			return fmt.Errorf("flag %s specified more than once", name)
		}
		value, rest, err := takeFlagValue(name, inline, hasInline, cursor.args)
		if err != nil {
			return err
		}
		if err := validateCLIRoot(value); err != nil {
			return err
		}
		cursor.args = rest
		parsed.root = value
		*rootSet = true
		return nil
	default:
		return errors.New("unknown flag")
	}
}

func runSearchIndex(ctx context.Context, args []string, config rootConfig, deps buildCommandDeps) int {
	parsed, err := parseBuildArgs(args)
	if err != nil {
		return writeSearchIndexToolError(deps.stderr, err)
	}
	root, err := resolveRoot(parsed.root, config)
	if err != nil {
		return writeSearchIndexToolError(deps.stderr, err)
	}
	capabilities, err := loadVaultCapabilities(ctx, root)
	if err != nil {
		return writeSearchIndexToolError(deps.stderr, errors.New("cannot read vault"))
	}
	defer releaseVaultReader(capabilities.reader)
	privacy := capabilities.privacy.ValidateSource()
	if !privacy.Available() {
		return writeSearchIndexRefusal(
			deps,
			parsed.json,
			searchReasonPrivacyUnavailable,
			buildGenerationObservationFromError(nil),
			privacyGateNotice("yomihon search-index", capabilities.governed),
		)
	}
	artifact := capabilities.artifact.ValidateSource()
	if !artifact.Available() {
		return writeSearchIndexUnavailable(deps, parsed.json, searchReasonArtifactPolicyUnavailable, nil)
	}
	if deps.openBuilder == nil {
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic command is not wired"))
	}
	action := semanticActionConfig{reader: capabilities.reader, artifact: artifact, privacy: privacy}
	if !parsed.json {
		action.buildProgress = func(embedded, total int) error {
			return writeSearchText(deps.stderr, fmt.Sprintf(
				"yomihon search-index: embedded %d/%d chunks\n",
				embedded,
				total,
			))
		}
	}
	builder, err := deps.openBuilder(action)
	if err != nil {
		if errors.Is(err, semantic.ErrPolicySourceChanged) {
			return writeSearchIndexFailure(deps, parsed.json, privacy, artifact, err)
		}
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic command setup failed"))
	}
	if builder == nil {
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic command setup failed"))
	}
	report, err := builder.Build(ctx, semantic.BuildRequest{RenewAttemptBudget: parsed.renewAttemptBudget})
	if err != nil {
		return writeSearchIndexFailure(deps, parsed.json, privacy, artifact, err)
	}
	callBeforeCapabilityRecheck(deps.beforeCapabilityRecheck)
	if !privacy.ValidateSource().Available() {
		return writeSearchIndexUnavailableAfterSuccess(deps, parsed.json, searchReasonPrivacyUnavailable, report)
	}
	if !artifact.ValidateSource().Available() {
		return writeSearchIndexUnavailableAfterSuccess(deps, parsed.json, searchReasonArtifactPolicyUnavailable, report)
	}
	answer, err := searchIndexAnswer(report)
	if err != nil {
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic build returned an invalid result"))
	}
	return writeSearchIndexSuccess(deps, parsed.json, answer)
}

func searchIndexAnswer(report semantic.BuildReport) (searchIndexAnswerEnvelope, error) {
	if report.TopKP95 < time.Microsecond {
		return searchIndexAnswerEnvelope{}, errors.New("semantic build has no measured top-k latency")
	}
	var status searchIndexStatus
	switch report.Status {
	case semantic.BuildCurrent:
		status = searchIndexStatusCurrent
	case semantic.BuildPublished:
		status = searchIndexStatusBuilt
	default:
		return searchIndexAnswerEnvelope{}, errors.New("unknown semantic build status")
	}
	return searchIndexAnswerEnvelope{
		Status:    status,
		Chunks:    report.Chunks,
		Embedded:  report.Embedded,
		Reused:    report.Reused,
		TopKP95US: report.TopKP95.Microseconds(),
	}, nil
}

func writeSearchIndexSuccess(deps buildCommandDeps, jsonOutput bool, answer searchIndexAnswerEnvelope) int {
	var err error
	if jsonOutput {
		err = writeSearchIndexJSON(deps.stdout, answer)
	} else {
		err = writeSearchIndexHuman(deps.stdout, answer)
	}
	if err != nil {
		return writeSearchIndexToolError(deps.stderr, errors.New("write output"))
	}
	return 0
}

func writeSearchIndexUnavailable(deps buildCommandDeps, jsonOutput bool, reason searchReason, cause error) int {
	return writeSearchIndexUnavailableWithObservation(
		deps,
		jsonOutput,
		reason,
		buildGenerationObservationFromError(cause),
	)
}

func writeSearchIndexUnavailableAfterSuccess(
	deps buildCommandDeps,
	jsonOutput bool,
	reason searchReason,
	report semantic.BuildReport,
) int {
	observation := buildGenerationObservation{active: semantic.ActiveGenerationPreservedUnusable}
	switch report.Status {
	case semantic.BuildCurrent:
		observation.staging = semantic.StagingGenerationNotInspected
	case semantic.BuildPublished:
		observation.staging = semantic.StagingGenerationAbsent
	default:
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic build returned an invalid result"))
	}
	return writeSearchIndexUnavailableWithObservation(deps, jsonOutput, reason, observation)
}

func writeSearchIndexUnavailableWithObservation(
	deps buildCommandDeps,
	jsonOutput bool,
	reason searchReason,
	observation buildGenerationObservation,
) int {
	line, err := searchIndexStderrLine(reason)
	if err != nil {
		return writeSearchIndexToolError(deps.stderr, errors.New("classify command failure"))
	}
	return writeSearchIndexRefusal(deps, jsonOutput, reason, observation, line)
}

// writeSearchIndexRefusal emits one closed-capability answer for the index
// command, with the notice a parameter for the same reason its search sibling
// takes one: a refusal decided before any work began can say more than one that
// arrived mid-build.
func writeSearchIndexRefusal(
	deps buildCommandDeps,
	jsonOutput bool,
	reason searchReason,
	observation buildGenerationObservation,
	notice string,
) int {
	if jsonOutput {
		body, err := newBuildErrorEnvelopeFromObservation(reason, observation)
		if err != nil {
			return writeSearchIndexToolError(deps.stderr, errors.New("classify command failure"))
		}
		if err := writeSearchIndexJSON(deps.stdout, body); err != nil {
			return writeSearchIndexToolError(deps.stderr, errors.New("write output"))
		}
	}
	if err := writeSearchText(deps.stderr, notice); err != nil {
		return 2
	}
	return 3
}

func writeSearchIndexFailure(
	deps buildCommandDeps,
	jsonOutput bool,
	privacy schema.PrivacyPolicy,
	artifact schema.ArtifactPolicy,
	err error,
) int {
	callBeforeCapabilityRecheck(deps.beforeCapabilityRecheck)
	if !privacy.ValidateSource().Available() {
		return writeSearchIndexUnavailable(deps, jsonOutput, searchReasonPrivacyUnavailable, err)
	}
	if !artifact.ValidateSource().Available() {
		return writeSearchIndexUnavailable(deps, jsonOutput, searchReasonArtifactPolicyUnavailable, err)
	}
	failure := classifySearchIndexError(err)
	switch failure.kind {
	case commandFailureUnavailable:
		return writeSearchIndexUnavailable(deps, jsonOutput, failure.reason, err)
	case commandFailureInternal:
		return writeSearchIndexInternal(deps, jsonOutput, err)
	case commandFailureTool:
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic build failed"))
	default:
		return writeSearchIndexToolError(deps.stderr, errors.New("semantic build failed"))
	}
}

func classifySearchIndexError(err error) commandFailure {
	if errors.Is(err, semantic.ErrAttemptLimit) {
		return unavailableFailure(searchReasonAttemptBudgetExhausted)
	}
	if embedErr, ok := errors.AsType[*semantic.EmbedError](err); ok {
		return classifyEmbedError(embedErr)
	}
	switch {
	case errors.Is(err, semantic.ErrEmbedderUnconfigured):
		return unavailableFailure(searchReasonEmbedderUnconfigured)
	case matchesAnyError(err, semantic.ErrWriterHeld, semantic.ErrStagingClosed):
		return unavailableFailure(searchReasonIndexRefreshing)
	case matchesAnyError(err, semantic.ErrCorpusRead, semantic.ErrCorpusChunking, semantic.ErrInvalidCorpus, semantic.ErrGenerationIncomplete):
		return unavailableFailure(searchReasonIndexIncomplete)
	case matchesAnyError(err, semantic.ErrVaultChanged, semantic.ErrChunkEgressDenied, semantic.ErrSourceNoteChanged, semantic.ErrSourceNoteUnavailable):
		return unavailableFailure(searchReasonVaultChanged)
	case errors.Is(err, semantic.ErrRetryNotReady):
		return unavailableFailure(searchReasonRateLimited)
	case errors.Is(err, semantic.ErrAttemptBudgetNotRenewable):
		return unavailableFailure(searchReasonAttemptBudgetNotRenewable)
	case errors.Is(err, semantic.ErrIndexCapacity):
		return unavailableFailure(searchReasonCapacity)
	case errors.Is(err, semantic.ErrStoreUnsupportedPlatform):
		return unavailableFailure(searchReasonUnsupportedPlatform)
	default:
		return commandFailure{kind: commandFailureTool}
	}
}

func writeSearchIndexInternal(deps buildCommandDeps, jsonOutput bool, cause error) int {
	if jsonOutput {
		body, err := newBuildInternalEnvelope(cause)
		if err != nil {
			return writeSearchIndexToolError(deps.stderr, errors.New("classify command failure"))
		}
		if err := writeSearchIndexJSON(deps.stdout, body); err != nil {
			return writeSearchIndexToolError(deps.stderr, errors.New("write output"))
		}
	}
	if err := writeSearchText(deps.stderr, searchIndexInternalStderrLine()); err != nil {
		return 2
	}
	return 1
}

func writeSearchIndexToolError(stderr io.Writer, err error) int {
	if stderr != nil {
		if writeErr := writeSearchText(stderr, fmt.Sprintf("yomihon search-index: %v\n", err)); writeErr != nil {
			return 2
		}
	}
	return 2
}
