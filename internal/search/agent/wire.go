package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
)

type searchMode string

const (
	searchModeLexical searchMode = "lexical"
	searchModeHybrid  searchMode = "hybrid"
)

type searchSemantic string

const (
	searchSemanticOff           searchSemantic = "off"
	searchSemanticOK            searchSemantic = "ok"
	searchSemanticNotApplicable searchSemantic = "not-applicable"
	searchSemanticUnavailable   searchSemantic = "unavailable"
)

type searchAnswerEnvelope struct {
	Mode     searchMode         `json:"mode"`
	Semantic searchSemantic     `json:"semantic"`
	Coverage *searchCoverage    `json:"coverage,omitempty"`
	Results  []searchWireResult `json:"results"`
}

type searchCoverage struct {
	Reason searchReason `json:"reason"`
}

type searchErrorEnvelope struct {
	Error searchError `json:"error"`
}

type searchError struct {
	Reason searchReason `json:"reason"`
}

type searchInternalEnvelope struct {
	InternalError searchInternalError `json:"internal_error"`
}

type searchInternalError struct {
	Detail string `json:"detail"`
}

const searchInternalDetail = "the request could not be formed correctly"

func newSearchInternalEnvelope() searchInternalEnvelope {
	return searchInternalEnvelope{
		InternalError: searchInternalError{Detail: searchInternalDetail},
	}
}

type searchReason string

const (
	searchReasonNotApplicable             searchReason = "not-applicable"
	searchReasonPrivacyUnavailable        searchReason = "privacy-capability-unavailable"
	searchReasonMetadataUnavailable       searchReason = "metadata-filters-unavailable"
	searchReasonArtifactPolicyUnavailable searchReason = "artifact-policy-unavailable"
	searchReasonCacheCold                 searchReason = "cache-cold"
	searchReasonCacheCorrupt              searchReason = "cache-corrupt"
	searchReasonCacheMismatch             searchReason = "cache-mismatch"
	searchReasonEmbedderRetired           searchReason = "embedder-retired"
	searchReasonCapacity                  searchReason = "capacity"
	searchReasonEmbedderUnconfigured      searchReason = "embedder-unconfigured"
	searchReasonRebuildRequired           searchReason = "rebuild-required"
	searchReasonIndexRefreshing           searchReason = "index-refreshing"
	searchReasonIndexIncomplete           searchReason = "index-incomplete"
	searchReasonVaultChanged              searchReason = "vault-changed"
	searchReasonEmbedderUnreachable       searchReason = "embedder-unreachable"
	searchReasonEmbedderRejected          searchReason = "embedder-rejected"
	searchReasonEmbedderFailed            searchReason = "embedder-failed"
	searchReasonRateLimited               searchReason = "rate-limited"
	searchReasonAttemptBudgetExhausted    searchReason = "attempt-budget-exhausted"
	searchReasonAttemptBudgetNotRenewable searchReason = "attempt-budget-not-renewable"
	searchReasonUnsupportedPlatform       searchReason = "unsupported-platform"
)

type searchWireResult struct {
	Rank         int                `json:"rank"`
	RelPath      string             `json:"rel_path"`
	Title        string             `json:"title"`
	Status       string             `json:"status,omitempty"`
	Snippet      string             `json:"snippet,omitempty"`
	Heading      string             `json:"heading,omitempty"`
	Channels     []searchChannel    `json:"channels"`
	ChannelRanks searchChannelRanks `json:"channel_ranks"`
}

type searchChannel string

const (
	searchChannelLexical  searchChannel = "lexical"
	searchChannelSemantic searchChannel = "semantic"
)

type searchChannelRanks struct {
	Lexical  *int `json:"lexical,omitempty"`
	Semantic *int `json:"semantic,omitempty"`
}

type searchIndexStatus string

const (
	searchIndexStatusCurrent searchIndexStatus = "current"
	searchIndexStatusBuilt   searchIndexStatus = "built"
)

type searchIndexAnswerEnvelope struct {
	Status    searchIndexStatus `json:"status"`
	Chunks    int               `json:"chunks"`
	Embedded  int               `json:"embedded"`
	Reused    int               `json:"reused"`
	TopKP95US int64             `json:"top_k_p95_us"`
}

type buildActiveState string

const (
	buildActiveNotInspected      buildActiveState = "not-inspected"
	buildActiveAbsent            buildActiveState = "absent"
	buildActivePreservedUsable   buildActiveState = "preserved-usable"
	buildActivePreservedUnusable buildActiveState = "preserved-unusable"
)

type buildStagingState string

const (
	buildStagingNotInspected          buildStagingState = "not-inspected"
	buildStagingAbsent                buildStagingState = "absent"
	buildStagingIncompatible          buildStagingState = "incompatible"
	buildStagingResumable             buildStagingState = "resumable"
	buildStagingRequiresAuthorization buildStagingState = "requires-authorization"
)

type buildNextAction string

const (
	buildNextRetry                buildNextAction = "retry-build"
	buildNextWait                 buildNextAction = "wait-and-retry"
	buildNextRenew                buildNextAction = "renew-attempt-budget"
	buildNextRepairConfiguration  buildNextAction = "repair-configuration"
	buildNextRepairVaultContract  buildNextAction = "repair-vault-contract"
	buildNextRepairInput          buildNextAction = "repair-input"
	buildNextUseSupportedPlatform buildNextAction = "use-supported-platform"
	buildNextReviewCapacity       buildNextAction = "review-capacity"
	buildNextRepairYomihon        buildNextAction = "repair-yomihon"
)

type buildRecovery struct {
	Reason            searchReason      `json:"reason"`
	ActiveGeneration  buildActiveState  `json:"active_generation"`
	StagingGeneration buildStagingState `json:"staging_generation"`
	RetrySafe         bool              `json:"retry_safe"`
	NextAction        buildNextAction   `json:"next_action"`
}

type buildErrorEnvelope struct {
	Error buildRecovery `json:"error"`
}

type buildInternalRecovery struct {
	Detail            string            `json:"detail"`
	ActiveGeneration  buildActiveState  `json:"active_generation"`
	StagingGeneration buildStagingState `json:"staging_generation"`
	RetrySafe         bool              `json:"retry_safe"`
	NextAction        buildNextAction   `json:"next_action"`
}

type buildInternalEnvelope struct {
	InternalError buildInternalRecovery `json:"internal_error"`
}

func lexicalWireResults(results []search.Result, limit int) []searchWireResult {
	results = results[:min(len(results), limit)]
	wire := make([]searchWireResult, len(results))
	for i := range results {
		rank := i + 1
		wire[i] = searchWireResult{
			Rank:         rank,
			RelPath:      results[i].RelPath,
			Title:        results[i].Title,
			Status:       results[i].Status,
			Snippet:      results[i].Snippet,
			Channels:     []searchChannel{searchChannelLexical},
			ChannelRanks: searchChannelRanks{Lexical: new(rank)},
		}
	}
	return wire
}

func hybridWireResults(results []FusedHit) []searchWireResult {
	wire := make([]searchWireResult, len(results))
	for i := range results {
		channels := make([]searchChannel, 0, 2)
		var ranks searchChannelRanks
		if results[i].LexicalRank > 0 {
			channels = append(channels, searchChannelLexical)
			ranks.Lexical = new(results[i].LexicalRank)
		}
		if results[i].SemanticRank > 0 {
			channels = append(channels, searchChannelSemantic)
			ranks.Semantic = new(results[i].SemanticRank)
		}
		wire[i] = searchWireResult{
			Rank:         results[i].Rank,
			RelPath:      results[i].RelPath,
			Title:        results[i].Title,
			Status:       results[i].Status,
			Snippet:      results[i].Snippet,
			Heading:      results[i].Heading,
			Channels:     channels,
			ChannelRanks: ranks,
		}
	}
	return wire
}

func writeSearchJSON(w io.Writer, body any) error {
	body, validationErr := validatedSearchBody(body)
	if validationErr != nil {
		return validationErr
	}
	return writeFrozenJSON(w, body)
}

func writeSearchIndexJSON(w io.Writer, body any) error {
	body, validationErr := validatedSearchIndexBody(body)
	if validationErr != nil {
		return validationErr
	}
	return writeFrozenJSON(w, body)
}

func writeFrozenJSON(w io.Writer, body any) error {
	if w == nil {
		return errors.New("write search output: writer unavailable")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if encodeErr := enc.Encode(body); encodeErr != nil {
		return fmt.Errorf("encode search output: %w", encodeErr)
	}
	line := unescapeSearchLineSeparators(buf.Bytes())
	n, err := w.Write(line)
	if err != nil {
		return fmt.Errorf("write search output: %w", err)
	}
	if n != len(line) {
		return fmt.Errorf("write search output: %w", io.ErrShortWrite)
	}
	return nil
}

func writeSearchHuman(w io.Writer, body searchAnswerEnvelope) error {
	validated, err := validateSearchAnswer(body)
	if err != nil {
		return err
	}
	var buf strings.Builder
	for i := range validated.Results {
		result := &validated.Results[i]
		fmt.Fprintf(&buf, "%d. %s — %s", result.Rank, terminalText(result.Title), terminalText(result.RelPath))
		if result.Status != "" {
			fmt.Fprintf(&buf, " [%s]", terminalText(result.Status))
		}
		if result.Heading != "" {
			fmt.Fprintf(&buf, " § %s", terminalText(result.Heading))
		}
		channels := make([]string, len(result.Channels))
		for channelIndex := range result.Channels {
			channels[channelIndex] = string(result.Channels[channelIndex])
		}
		fmt.Fprintf(&buf, " (%s)\n", strings.Join(channels, "+"))
		if result.Snippet != "" {
			fmt.Fprintf(&buf, "   %s\n", terminalText(result.Snippet))
		}
	}
	return writeSearchText(w, buf.String())
}

func writeSearchIndexHuman(w io.Writer, body searchIndexAnswerEnvelope) error {
	validated, err := validateSearchIndexAnswer(body)
	if err != nil {
		return err
	}
	var line string
	switch validated.Status {
	case searchIndexStatusCurrent:
		line = fmt.Sprintf("semantic index current: %d chunks\n", validated.Chunks)
	case searchIndexStatusBuilt:
		line = fmt.Sprintf(
			"semantic index built: %d chunks (%d embedded, %d reused)\n",
			validated.Chunks,
			validated.Embedded,
			validated.Reused,
		)
	default:
		return errors.New("search-index output: invalid status")
	}
	return writeSearchText(w, line)
}

func writeSearchText(w io.Writer, text string) error {
	if w == nil {
		return errors.New("write search output: writer unavailable")
	}
	n, err := io.WriteString(w, text)
	if err != nil {
		return fmt.Errorf("write search output: %w", err)
	}
	if n != len(text) {
		return fmt.Errorf("write search output: %w", io.ErrShortWrite)
	}
	return nil
}

func terminalText(value string) string {
	collapsed := strings.Join(strings.Fields(value), " ")
	return strings.Map(func(r rune) rune {
		if r <= '\x1f' || r >= '\x7f' && r <= '\x9f' {
			return -1
		}
		return r
	}, collapsed)
}

func validatedSearchBody(body any) (any, error) {
	switch envelope := body.(type) {
	case searchAnswerEnvelope:
		return validateSearchAnswer(envelope)
	case searchErrorEnvelope:
		return validateSearchError(envelope)
	case searchInternalEnvelope:
		return validateSearchInternal(envelope)
	default:
		return nil, fmt.Errorf("search output: unsupported envelope %T", body)
	}
}

func validatedSearchIndexBody(body any) (any, error) {
	switch envelope := body.(type) {
	case searchIndexAnswerEnvelope:
		return validateSearchIndexAnswer(envelope)
	case buildErrorEnvelope:
		return validateBuildError(envelope)
	case buildInternalEnvelope:
		return validateBuildInternal(envelope)
	default:
		return nil, fmt.Errorf("search-index output: unsupported envelope %T", body)
	}
}

func validateSearchIndexAnswer(envelope searchIndexAnswerEnvelope) (searchIndexAnswerEnvelope, error) {
	if envelope.Status != searchIndexStatusCurrent && envelope.Status != searchIndexStatusBuilt {
		return searchIndexAnswerEnvelope{}, errors.New("search-index output: invalid status")
	}
	if envelope.Chunks < 0 || envelope.Embedded < 0 || envelope.Reused < 0 {
		return searchIndexAnswerEnvelope{}, errors.New("search-index output: negative count")
	}
	if envelope.TopKP95US < 1 {
		return searchIndexAnswerEnvelope{}, errors.New("search-index output: top-k latency is unmeasured")
	}
	if envelope.Embedded+envelope.Reused != envelope.Chunks {
		return searchIndexAnswerEnvelope{}, errors.New("search-index output: chunk counts do not reconcile")
	}
	if envelope.Status == searchIndexStatusCurrent && envelope.Embedded != 0 {
		return searchIndexAnswerEnvelope{}, errors.New("search-index output: current generation cannot report embedded chunks")
	}
	return envelope, nil
}

func newBuildErrorEnvelopeFromObservation(
	reason searchReason,
	observation buildGenerationObservation,
) (buildErrorEnvelope, error) {
	active, staging, err := buildGenerationStates(observation)
	if err != nil {
		return buildErrorEnvelope{}, err
	}
	next, err := buildNextActionForReason(reason)
	if err != nil {
		return buildErrorEnvelope{}, err
	}
	return buildErrorEnvelope{Error: buildRecovery{
		Reason:            reason,
		ActiveGeneration:  active,
		StagingGeneration: staging,
		RetrySafe:         false,
		NextAction:        next,
	}}, nil
}

func newBuildInternalEnvelope(cause error) (buildInternalEnvelope, error) {
	active, staging, err := buildGenerationStates(buildGenerationObservationFromError(cause))
	if err != nil {
		return buildInternalEnvelope{}, err
	}
	return buildInternalEnvelope{InternalError: buildInternalRecovery{
		Detail:            searchInternalDetail,
		ActiveGeneration:  active,
		StagingGeneration: staging,
		RetrySafe:         false,
		NextAction:        buildNextRepairYomihon,
	}}, nil
}

type generationStateError interface {
	error
	ActiveGeneration() semantic.ActiveGenerationState
	StagingGeneration() semantic.StagingGenerationState
}

type buildGenerationObservation struct {
	active  semantic.ActiveGenerationState
	staging semantic.StagingGenerationState
}

func buildGenerationObservationFromError(cause error) buildGenerationObservation {
	buildErr, ok := errors.AsType[generationStateError](cause)
	if !ok {
		return buildGenerationObservation{}
	}
	return buildGenerationObservation{
		active:  buildErr.ActiveGeneration(),
		staging: buildErr.StagingGeneration(),
	}
}

func buildGenerationStates(observation buildGenerationObservation) (buildActiveState, buildStagingState, error) {
	active, err := buildActiveFromSemantic(observation.active)
	if err != nil {
		return "", "", err
	}
	staging, err := buildStagingFromSemantic(observation.staging)
	if err != nil {
		return "", "", err
	}
	return active, staging, nil
}

func buildActiveFromSemantic(state semantic.ActiveGenerationState) (buildActiveState, error) {
	switch state {
	case semantic.ActiveGenerationNotInspected:
		return buildActiveNotInspected, nil
	case semantic.ActiveGenerationAbsent:
		return buildActiveAbsent, nil
	case semantic.ActiveGenerationPreservedUsable:
		return buildActivePreservedUsable, nil
	case semantic.ActiveGenerationPreservedUnusable:
		return buildActivePreservedUnusable, nil
	default:
		return "", fmt.Errorf("search-index output: unknown active generation state %d", state)
	}
}

func buildStagingFromSemantic(state semantic.StagingGenerationState) (buildStagingState, error) {
	switch state {
	case semantic.StagingGenerationNotInspected:
		return buildStagingNotInspected, nil
	case semantic.StagingGenerationAbsent:
		return buildStagingAbsent, nil
	case semantic.StagingGenerationIncompatible:
		return buildStagingIncompatible, nil
	case semantic.StagingGenerationResumable:
		return buildStagingResumable, nil
	case semantic.StagingGenerationRequiresAuthorization:
		return buildStagingRequiresAuthorization, nil
	default:
		return "", fmt.Errorf("search-index output: unknown staging generation state %d", state)
	}
}

func buildNextActionForReason(reason searchReason) (buildNextAction, error) {
	switch reason {
	case searchReasonPrivacyUnavailable, searchReasonArtifactPolicyUnavailable:
		return buildNextRepairVaultContract, nil
	case searchReasonEmbedderUnconfigured, searchReasonEmbedderRejected:
		return buildNextRepairConfiguration, nil
	case searchReasonUnsupportedPlatform:
		return buildNextUseSupportedPlatform, nil
	case searchReasonCapacity:
		return buildNextReviewCapacity, nil
	case searchReasonIndexIncomplete:
		return buildNextRepairInput, nil
	case searchReasonIndexRefreshing, searchReasonRateLimited:
		return buildNextWait, nil
	case searchReasonAttemptBudgetExhausted:
		return buildNextRenew, nil
	case searchReasonAttemptBudgetNotRenewable,
		searchReasonVaultChanged,
		searchReasonEmbedderUnreachable,
		searchReasonEmbedderFailed:
		return buildNextRetry, nil
	default:
		return "", fmt.Errorf("search-index output: reason %q has no recovery action", reason)
	}
}

func validateBuildError(envelope buildErrorEnvelope) (buildErrorEnvelope, error) {
	recovery := envelope.Error
	if !isBuildFailureReason(recovery.Reason) {
		return buildErrorEnvelope{}, errors.New("search-index output: invalid unavailable reason")
	}
	if !validBuildActiveState(recovery.ActiveGeneration) {
		return buildErrorEnvelope{}, errors.New("search-index output: invalid active generation state")
	}
	if !validBuildStagingState(recovery.StagingGeneration) {
		return buildErrorEnvelope{}, errors.New("search-index output: invalid staging generation state")
	}
	if recovery.Reason == searchReasonAttemptBudgetExhausted && recovery.StagingGeneration != buildStagingRequiresAuthorization {
		return buildErrorEnvelope{}, errors.New("search-index output: exhausted attempt budget requires authorization")
	}
	if recovery.RetrySafe {
		return buildErrorEnvelope{}, errors.New("search-index output: retry_safe must be false")
	}
	wantNext, err := buildNextActionForReason(recovery.Reason)
	if err != nil || recovery.NextAction != wantNext {
		return buildErrorEnvelope{}, errors.New("search-index output: invalid recovery action")
	}
	return envelope, nil
}

func validateBuildInternal(envelope buildInternalEnvelope) (buildInternalEnvelope, error) {
	recovery := envelope.InternalError
	if recovery.Detail != searchInternalDetail {
		return buildInternalEnvelope{}, errors.New("search-index output: invalid internal error detail")
	}
	if !validBuildActiveState(recovery.ActiveGeneration) {
		return buildInternalEnvelope{}, errors.New("search-index output: invalid active generation state")
	}
	if !validBuildStagingState(recovery.StagingGeneration) {
		return buildInternalEnvelope{}, errors.New("search-index output: invalid staging generation state")
	}
	if recovery.RetrySafe || recovery.NextAction != buildNextRepairYomihon {
		return buildInternalEnvelope{}, errors.New("search-index output: invalid internal recovery")
	}
	return envelope, nil
}

func validBuildActiveState(state buildActiveState) bool {
	switch state {
	case buildActiveNotInspected,
		buildActiveAbsent,
		buildActivePreservedUsable,
		buildActivePreservedUnusable:
		return true
	default:
		return false
	}
}

func validBuildStagingState(state buildStagingState) bool {
	switch state {
	case buildStagingNotInspected,
		buildStagingAbsent,
		buildStagingIncompatible,
		buildStagingResumable,
		buildStagingRequiresAuthorization:
		return true
	default:
		return false
	}
}

func isBuildFailureReason(reason searchReason) bool {
	switch reason {
	case searchReasonPrivacyUnavailable,
		searchReasonArtifactPolicyUnavailable,
		searchReasonCapacity,
		searchReasonUnsupportedPlatform,
		searchReasonEmbedderUnconfigured,
		searchReasonIndexRefreshing,
		searchReasonIndexIncomplete,
		searchReasonVaultChanged,
		searchReasonEmbedderUnreachable,
		searchReasonEmbedderRejected,
		searchReasonEmbedderFailed,
		searchReasonRateLimited,
		searchReasonAttemptBudgetExhausted,
		searchReasonAttemptBudgetNotRenewable:
		return true
	default:
		return false
	}
}

func validateSearchAnswer(envelope searchAnswerEnvelope) (searchAnswerEnvelope, error) {
	if !legalSearchPair(envelope.Mode, envelope.Semantic) {
		return searchAnswerEnvelope{}, fmt.Errorf("search output: illegal mode and semantic pair %s/%s", envelope.Mode, envelope.Semantic)
	}
	if err := validateSearchCoverage(envelope.Semantic, envelope.Coverage); err != nil {
		return searchAnswerEnvelope{}, err
	}
	if envelope.Results == nil {
		envelope.Results = []searchWireResult{}
	}
	if err := validateSearchResults(envelope); err != nil {
		return searchAnswerEnvelope{}, err
	}
	return envelope, nil
}

func validateSearchCoverage(state searchSemantic, coverage *searchCoverage) error {
	switch state {
	case searchSemanticOff, searchSemanticOK:
		if coverage != nil {
			return fmt.Errorf("search output: semantic %s cannot carry coverage", state)
		}
	case searchSemanticNotApplicable:
		if coverage == nil || coverage.Reason != searchReasonNotApplicable {
			return errors.New("search output: semantic not-applicable requires matching coverage")
		}
	case searchSemanticUnavailable:
		if coverage == nil {
			return errors.New("search output: semantic unavailable requires coverage")
		}
		if !isUnavailableReason(coverage.Reason) {
			return errors.New("search output: invalid answerable unavailable reason")
		}
	}
	return nil
}

func validateSearchError(envelope searchErrorEnvelope) (searchErrorEnvelope, error) {
	if envelope.Error.Reason != searchReasonPrivacyUnavailable && envelope.Error.Reason != searchReasonMetadataUnavailable {
		return searchErrorEnvelope{}, errors.New("search output: invalid unanswerable reason")
	}
	return envelope, nil
}

func validateSearchInternal(envelope searchInternalEnvelope) (searchInternalEnvelope, error) {
	if envelope.InternalError.Detail != searchInternalDetail {
		return searchInternalEnvelope{}, errors.New("search output: invalid internal error detail")
	}
	return envelope, nil
}

func validateSearchResults(envelope searchAnswerEnvelope) error {
	for i := range envelope.Results {
		result := &envelope.Results[i]
		if result.Rank != i+1 {
			return fmt.Errorf("search output: result %d has rank %d, want %d", i, result.Rank, i+1)
		}
		hasSemantic, err := validateSearchChannels(i, result)
		if err != nil {
			return err
		}
		if envelope.Mode == searchModeLexical && hasSemantic {
			return fmt.Errorf("search output: lexical result %d carries semantic channel", i)
		}
		if hasSemantic && result.Snippet == "" {
			return fmt.Errorf("search output: semantic result %d has no snippet", i)
		}
		if result.Heading != "" && result.Snippet == "" {
			return fmt.Errorf("search output: result %d has heading without snippet", i)
		}
	}
	return nil
}

func validateSearchChannels(index int, result *searchWireResult) (bool, error) {
	hasLexical, hasSemantic, err := searchChannelSet(index, result.Channels)
	if err != nil {
		return false, err
	}
	lexicalRank := result.ChannelRanks.Lexical
	semanticRank := result.ChannelRanks.Semantic
	if (lexicalRank != nil && *lexicalRank < 1) || (semanticRank != nil && *semanticRank < 1) {
		return false, fmt.Errorf("search output: result %d has a non-positive channel rank", index)
	}
	if (lexicalRank != nil) != hasLexical || (semanticRank != nil) != hasSemantic {
		return false, fmt.Errorf("search output: result %d channel ranks do not match channels", index)
	}
	return hasSemantic, nil
}

func searchChannelSet(index int, channels []searchChannel) (lexical, semanticChannel bool, err error) {
	switch len(channels) {
	case 1:
		switch channels[0] {
		case searchChannelLexical:
			return true, false, nil
		case searchChannelSemantic:
			return false, true, nil
		}
	case 2:
		if channels[0] == searchChannelLexical && channels[1] == searchChannelSemantic {
			return true, true, nil
		}
	}
	return false, false, fmt.Errorf("search output: result %d has invalid channels", index)
}

func isUnavailableReason(reason searchReason) bool {
	switch reason {
	case searchReasonArtifactPolicyUnavailable,
		searchReasonCacheCold,
		searchReasonCacheCorrupt,
		searchReasonCacheMismatch,
		searchReasonEmbedderRetired,
		searchReasonCapacity,
		searchReasonEmbedderUnconfigured,
		searchReasonRebuildRequired,
		searchReasonIndexRefreshing,
		searchReasonIndexIncomplete,
		searchReasonVaultChanged,
		searchReasonEmbedderUnreachable,
		searchReasonEmbedderRejected,
		searchReasonEmbedderFailed,
		searchReasonRateLimited,
		searchReasonAttemptBudgetExhausted,
		searchReasonUnsupportedPlatform:
		return true
	default:
		return false
	}
}

func searchStderrLine(reason searchReason) (string, error) {
	return searchCommandStderrLine("yomihon search", reason)
}

func searchIndexStderrLine(reason searchReason) (string, error) {
	switch reason {
	case searchReasonAttemptBudgetExhausted:
		return "yomihon search-index: attempt-budget-exhausted: provider-send authorization is exhausted; run yomihon search-index build --renew-attempt-budget\n", nil
	case searchReasonAttemptBudgetNotRenewable:
		return "yomihon search-index: attempt-budget-not-renewable: no exhausted matching staging generation exists; run yomihon search-index build\n", nil
	case searchReasonNotApplicable,
		searchReasonPrivacyUnavailable,
		searchReasonMetadataUnavailable,
		searchReasonArtifactPolicyUnavailable,
		searchReasonCacheCold,
		searchReasonCacheCorrupt,
		searchReasonCacheMismatch,
		searchReasonEmbedderRetired,
		searchReasonCapacity,
		searchReasonEmbedderUnconfigured,
		searchReasonRebuildRequired,
		searchReasonIndexRefreshing,
		searchReasonIndexIncomplete,
		searchReasonVaultChanged,
		searchReasonEmbedderUnreachable,
		searchReasonEmbedderRejected,
		searchReasonEmbedderFailed,
		searchReasonRateLimited,
		searchReasonUnsupportedPlatform:
		return searchCommandStderrLine("yomihon search-index", reason)
	}
	return "", errors.New("search output: unknown reason")
}

func searchCommandStderrLine(prefix string, reason searchReason) (string, error) {
	message, ok := searchCapabilityMessage(reason)
	if !ok {
		message, ok = searchGenerationMessage(reason)
	}
	if !ok {
		message, ok = searchEmbedderMessage(reason)
	}
	if !ok {
		return "", errors.New("search output: reason has no stderr line")
	}
	return searchCapabilityLine(prefix, reason, message), nil
}

// searchCapabilityLine formats one refusal for stderr: the command, the reason
// a program matches on, and the sentence a person reads.
func searchCapabilityLine(prefix string, reason searchReason, message string) string {
	return fmt.Sprintf("%s: %s: %s\n", prefix, reason, message)
}

const (
	privacyRejectedMessage = "the vault contract declares no valid privacy policy, so agent-facing search output is closed"
	privacyAbsentMessage   = "this folder has no vault contract, so agent-facing search output is closed"
)

// privacyGateNotice is everything a person is told when the privacy capability
// closes before any work has begun: the machine's line, then the paragraph
// whoever typed the command needs — what is missing, where it belongs, why this
// face needs it, and the face that needs none of it.
//
// A folder carrying no contract and a contract that could not be honoured are
// different facts and get different words. The reason token is the same for
// both, because for egress an undeclared policy and a rejected one are the same
// refusal, and a second token would change a value downstream pipelines match
// on.
func privacyGateNotice(prefix string, governed bool) string {
	if governed {
		return searchCapabilityLine(prefix, searchReasonPrivacyUnavailable, privacyRejectedMessage) +
			"  The contract is at " + schema.ContractRelPath + " and its privacy policy could not\n" +
			"  be honoured, so yomihon cannot tell which directories may leave this machine and\n" +
			"  hands over nothing. The same search runs in the browser, which sends nothing\n" +
			"  anywhere: yomihon serve --root <dir>\n"
	}
	return searchCapabilityLine(prefix, searchReasonPrivacyUnavailable, privacyAbsentMessage) +
		"  yomihon reads " + schema.ContractRelPath + " for, among other things, the\n" +
		"  directories whose contents must never leave this machine. A folder carrying no\n" +
		"  such file has declared nothing, so yomihon cannot tell what it may hand to a\n" +
		"  program and hands over nothing. The same search runs in the browser, which sends\n" +
		"  nothing anywhere: yomihon serve --root <dir>\n"
}

func searchCapabilityMessage(reason searchReason) (string, bool) {
	switch reason {
	case searchReasonPrivacyUnavailable:
		return privacyRejectedMessage, true
	case searchReasonMetadataUnavailable:
		return "the vault contract declares no valid artifact policy, so metadata filters cannot be evaluated", true
	case searchReasonArtifactPolicyUnavailable:
		return "the vault contract declares no valid artifact policy, so the semantic corpus cannot exist", true
	default:
		return "", false
	}
}

func searchGenerationMessage(reason searchReason) (string, bool) {
	switch reason {
	case searchReasonCacheCold:
		return "no semantic index exists; run yomihon search-index build", true
	case searchReasonCacheCorrupt:
		return "the semantic index is unreadable; run yomihon search-index build", true
	case searchReasonCacheMismatch:
		return "the semantic index uses a different configuration; run yomihon search-index build", true
	case searchReasonEmbedderRetired:
		return "the old index's embedding model is no longer available", true
	case searchReasonRebuildRequired:
		return "the vault changed too much for an interactive refresh; run yomihon search-index build", true
	case searchReasonIndexRefreshing:
		return "another process is updating the semantic index", true
	case searchReasonIndexIncomplete:
		return "one or more current chunks could not be indexed", true
	case searchReasonVaultChanged:
		return "the vault changed while the semantic index was being updated; retry", true
	case searchReasonAttemptBudgetExhausted:
		return "semantic document sends need renewed authorization; run yomihon search-index build --renew-attempt-budget", true
	case searchReasonCapacity:
		return "the semantic index could not be loaded into memory", true
	case searchReasonUnsupportedPlatform:
		return "the semantic generation store is not supported on this platform", true
	default:
		return "", false
	}
}

func searchEmbedderMessage(reason searchReason) (string, bool) {
	switch reason {
	case searchReasonEmbedderUnconfigured:
		return "no embedding key is configured, so semantic search is off; set YOMIHON_EMBED_KEY", true
	case searchReasonEmbedderUnreachable:
		return "the embedding API did not answer", true
	case searchReasonEmbedderFailed:
		return "the embedding API returned an error search could not recover from", true
	case searchReasonEmbedderRejected:
		return "the embedding API refused the credential", true
	case searchReasonRateLimited:
		return "the embedding API is rate-limiting; try again shortly", true
	default:
		return "", false
	}
}

func searchInternalStderrLine() string {
	return "yomihon search: internal: " + searchInternalDetail + "\n"
}

func searchIndexInternalStderrLine() string {
	return "yomihon search-index: internal: " + searchInternalDetail + "\n"
}

func legalSearchPair(mode searchMode, state searchSemantic) bool {
	return mode == searchModeLexical && state == searchSemanticOff ||
		mode == searchModeHybrid && state == searchSemanticOK ||
		mode == searchModeLexical && state == searchSemanticNotApplicable ||
		mode == searchModeLexical && state == searchSemanticUnavailable
}

func unescapeSearchLineSeparators(line []byte) []byte {
	if !bytes.Contains(line, []byte(`\u202`)) {
		return line
	}
	out := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		if line[i] != '\\' || i+1 == len(line) {
			out = append(out, line[i])
			continue
		}
		if line[i+1] == 'u' && i+6 <= len(line) {
			switch string(line[i+2 : i+6]) {
			case "2028":
				out = append(out, 0xe2, 0x80, 0xa8)
				i += 5
				continue
			case "2029":
				out = append(out, 0xe2, 0x80, 0xa9)
				i += 5
				continue
			}
		}
		out = append(out, line[i], line[i+1])
		i++
	}
	return out
}
