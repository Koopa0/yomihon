package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/search/semantic"
)

type observedBuildError struct {
	cause   error
	active  semantic.ActiveGenerationState
	staging semantic.StagingGenerationState
}

func (e observedBuildError) Error() string { return e.cause.Error() }
func (e observedBuildError) Unwrap() error { return e.cause }
func (e observedBuildError) ActiveGeneration() semantic.ActiveGenerationState {
	return e.active
}
func (e observedBuildError) StagingGeneration() semantic.StagingGenerationState {
	return e.staging
}

func exhaustedBuildFailure() error {
	return observedBuildError{
		cause:   semantic.ErrAttemptLimit,
		active:  semantic.ActiveGenerationAbsent,
		staging: semantic.StagingGenerationRequiresAuthorization,
	}
}

func TestWriteSearchJSON(t *testing.T) {
	tests := []struct {
		name string
		body searchAnswerEnvelope
		want string
	}{
		{
			name: "lexical off",
			body: searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticOff, Results: []searchWireResult{}},
			want: "{\"mode\":\"lexical\",\"semantic\":\"off\",\"results\":[]}\n",
		},
		{
			name: "hybrid ok",
			body: searchAnswerEnvelope{
				Mode:     searchModeHybrid,
				Semantic: searchSemanticOK,
				Results: []searchWireResult{{
					Rank:     1,
					RelPath:  "Writing/日本語.md",
					Title:    "日本語 <>&",
					Status:   "ready",
					Snippet:  "evidence",
					Heading:  "文法",
					Channels: []searchChannel{searchChannelLexical, searchChannelSemantic},
					ChannelRanks: searchChannelRanks{
						Lexical:  new(2),
						Semantic: new(1),
					},
				}},
			},
			want: readWireGolden(t, "answer.jsonl"),
		},
		{
			name: "not applicable",
			body: searchAnswerEnvelope{
				Mode:     searchModeLexical,
				Semantic: searchSemanticNotApplicable,
				Coverage: &searchCoverage{Reason: searchReasonNotApplicable},
				Results:  []searchWireResult{},
			},
			want: "{\"mode\":\"lexical\",\"semantic\":\"not-applicable\",\"coverage\":{\"reason\":\"not-applicable\"},\"results\":[]}\n",
		},
		{
			name: "unavailable",
			body: searchAnswerEnvelope{
				Mode:     searchModeLexical,
				Semantic: searchSemanticUnavailable,
				Coverage: &searchCoverage{Reason: searchReasonCacheCold},
				Results:  []searchWireResult{},
			},
			want: "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"cache-cold\"},\"results\":[]}\n",
		},
		{
			name: "unsupported platform",
			body: searchAnswerEnvelope{
				Mode:     searchModeLexical,
				Semantic: searchSemanticUnavailable,
				Coverage: &searchCoverage{Reason: searchReasonUnsupportedPlatform},
				Results:  []searchWireResult{},
			},
			want: "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"unsupported-platform\"},\"results\":[]}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeSearchJSON(&buf, tt.body); err != nil {
				t.Fatalf("writeSearchJSON() error: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("writeSearchJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteSearchHuman(t *testing.T) {
	body := searchAnswerEnvelope{
		Mode:     searchModeHybrid,
		Semantic: searchSemanticOK,
		Results: []searchWireResult{
			{
				Rank:     1,
				RelPath:  "Writing/lesson\nname.md",
				Title:    "  日本語\tlesson\a ",
				Status:   "ready\x7f",
				Snippet:  "line one\n\n line two\u0085done",
				Heading:  " 文法\r\n 基礎 ",
				Channels: []searchChannel{searchChannelLexical, searchChannelSemantic},
				ChannelRanks: searchChannelRanks{
					Lexical:  new(2),
					Semantic: new(1),
				},
			},
			{
				Rank:         2,
				RelPath:      "Concepts/x.md",
				Title:        "X",
				Channels:     []searchChannel{searchChannelLexical},
				ChannelRanks: searchChannelRanks{Lexical: new(3)},
			},
		},
	}
	var buf bytes.Buffer
	if err := writeSearchHuman(&buf, body); err != nil {
		t.Fatalf("writeSearchHuman() error: %v", err)
	}
	want := "1. 日本語 lesson — Writing/lesson name.md [ready] § 文法 基礎 (lexical+semantic)\n" +
		"   line one line two done\n" +
		"2. X — Concepts/x.md (lexical)\n"
	if got := buf.String(); got != want {
		t.Errorf("writeSearchHuman() = %q, want %q", got, want)
	}
}

func TestWriteSearchHumanZeroResults(t *testing.T) {
	var buf bytes.Buffer
	body := searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticOff}
	if err := writeSearchHuman(&buf, body); err != nil {
		t.Fatalf("writeSearchHuman() error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("writeSearchHuman() = %q, want empty", buf.Bytes())
	}
}

func TestWriteSearchIndexOutput(t *testing.T) {
	tests := []struct {
		name string
		body searchIndexAnswerEnvelope
		json bool
		want string
	}{
		{
			name: "current JSON",
			body: searchIndexAnswerEnvelope{Status: searchIndexStatusCurrent, Chunks: 8, Reused: 8, TopKP95US: 241},
			json: true,
			want: "{\"status\":\"current\",\"chunks\":8,\"embedded\":0,\"reused\":8,\"top_k_p95_us\":241}\n",
		},
		{
			name: "built JSON",
			body: searchIndexAnswerEnvelope{Status: searchIndexStatusBuilt, Chunks: 8, Embedded: 3, Reused: 5, TopKP95US: 317},
			json: true,
			want: readWireGolden(t, "index-built.jsonl"),
		},
		{
			name: "current human",
			body: searchIndexAnswerEnvelope{Status: searchIndexStatusCurrent, Chunks: 8, Reused: 8, TopKP95US: 241},
			want: "semantic index current: 8 chunks\n",
		},
		{
			name: "built human",
			body: searchIndexAnswerEnvelope{Status: searchIndexStatusBuilt, Chunks: 8, Embedded: 3, Reused: 5, TopKP95US: 317},
			want: "semantic index built: 8 chunks (3 embedded, 5 reused)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var err error
			if tt.json {
				err = writeSearchIndexJSON(&buf, tt.body)
			} else {
				err = writeSearchIndexHuman(&buf, tt.body)
			}
			if err != nil {
				t.Fatalf("write search-index output error: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("write search-index output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteSearchIndexRecoveryEnvelopes(t *testing.T) {
	tests := []struct {
		reason      searchReason
		wantReason  string
		wantNext    string
		wantActive  string
		wantStaging string
	}{
		{reason: searchReasonPrivacyUnavailable, wantReason: "privacy-capability-unavailable", wantNext: "repair-vault-contract"},
		{reason: searchReasonArtifactPolicyUnavailable, wantReason: "artifact-policy-unavailable", wantNext: "repair-vault-contract"},
		{reason: searchReasonCapacity, wantReason: "capacity", wantNext: "review-capacity"},
		{reason: searchReasonUnsupportedPlatform, wantReason: "unsupported-platform", wantNext: "use-supported-platform"},
		{reason: searchReasonEmbedderUnconfigured, wantReason: "embedder-unconfigured", wantNext: "repair-configuration"},
		{reason: searchReasonIndexRefreshing, wantReason: "index-refreshing", wantNext: "wait-and-retry"},
		{reason: searchReasonIndexIncomplete, wantReason: "index-incomplete", wantNext: "repair-input"},
		{reason: searchReasonVaultChanged, wantReason: "vault-changed", wantNext: "retry-build"},
		{reason: searchReasonEmbedderUnreachable, wantReason: "embedder-unreachable", wantNext: "retry-build"},
		{reason: searchReasonEmbedderRejected, wantReason: "embedder-rejected", wantNext: "repair-configuration"},
		{reason: searchReasonEmbedderFailed, wantReason: "embedder-failed", wantNext: "retry-build"},
		{reason: searchReasonRateLimited, wantReason: "rate-limited", wantNext: "wait-and-retry"},
		{
			reason:      searchReasonAttemptBudgetExhausted,
			wantReason:  "attempt-budget-exhausted",
			wantNext:    "renew-attempt-budget",
			wantActive:  "absent",
			wantStaging: "requires-authorization",
		},
		{reason: searchReasonAttemptBudgetNotRenewable, wantReason: "attempt-budget-not-renewable", wantNext: "retry-build"},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			var cause error
			wantActive := tt.wantActive
			if wantActive == "" {
				wantActive = "not-inspected"
			}
			wantStaging := tt.wantStaging
			if wantStaging == "" {
				wantStaging = "not-inspected"
			}
			if tt.reason == searchReasonAttemptBudgetExhausted {
				cause = exhaustedBuildFailure()
			}
			body, err := newBuildErrorEnvelope(tt.reason, cause)
			if err != nil {
				t.Fatalf("newBuildErrorEnvelope() error: %v", err)
			}
			var buf bytes.Buffer
			if err := writeSearchIndexJSON(&buf, body); err != nil {
				t.Fatalf("writeSearchIndexJSON() error: %v", err)
			}
			want := fmt.Sprintf(
				"{\"error\":{\"reason\":%q,\"active_generation\":%q,\"staging_generation\":%q,\"retry_safe\":false,\"next_action\":%q}}\n",
				tt.wantReason,
				wantActive,
				wantStaging,
				tt.wantNext,
			)
			if got := buf.String(); got != want {
				t.Errorf("writeSearchIndexJSON() = %q, want %q", got, want)
			}
		})
	}

	body, err := newBuildInternalEnvelope(nil)
	if err != nil {
		t.Fatalf("newBuildInternalEnvelope() error: %v", err)
	}
	var buf bytes.Buffer
	if err := writeSearchIndexJSON(&buf, body); err != nil {
		t.Fatalf("writeSearchIndexJSON(internal) error: %v", err)
	}
	if got, want := buf.String(), readWireGolden(t, "index-internal-error.jsonl"); got != want {
		t.Errorf("writeSearchIndexJSON(internal) = %q, want %q", got, want)
	}
}

func TestBuildGenerationStateProjectionIsExhaustive(t *testing.T) {
	active := []struct {
		semantic semantic.ActiveGenerationState
		wire     buildActiveState
	}{
		{semantic: semantic.ActiveGenerationNotInspected, wire: buildActiveNotInspected},
		{semantic: semantic.ActiveGenerationAbsent, wire: buildActiveAbsent},
		{semantic: semantic.ActiveGenerationPreservedUsable, wire: buildActivePreservedUsable},
		{semantic: semantic.ActiveGenerationPreservedUnusable, wire: buildActivePreservedUnusable},
	}
	for _, tt := range active {
		got, err := buildActiveFromSemantic(tt.semantic)
		if err != nil || got != tt.wire {
			t.Errorf("buildActiveFromSemantic(%d) = %q, %v; want %q, nil", tt.semantic, got, err, tt.wire)
		}
	}
	if _, err := buildActiveFromSemantic(semantic.ActiveGenerationState(255)); err == nil {
		t.Error("buildActiveFromSemantic(255) error = nil, want fail closed")
	}

	staging := []struct {
		semantic semantic.StagingGenerationState
		wire     buildStagingState
	}{
		{semantic: semantic.StagingGenerationNotInspected, wire: buildStagingNotInspected},
		{semantic: semantic.StagingGenerationAbsent, wire: buildStagingAbsent},
		{semantic: semantic.StagingGenerationIncompatible, wire: buildStagingIncompatible},
		{semantic: semantic.StagingGenerationResumable, wire: buildStagingResumable},
		{semantic: semantic.StagingGenerationRequiresAuthorization, wire: buildStagingRequiresAuthorization},
	}
	for _, tt := range staging {
		got, err := buildStagingFromSemantic(tt.semantic)
		if err != nil || got != tt.wire {
			t.Errorf("buildStagingFromSemantic(%d) = %q, %v; want %q, nil", tt.semantic, got, err, tt.wire)
		}
	}
	if _, err := buildStagingFromSemantic(semantic.StagingGenerationState(255)); err == nil {
		t.Error("buildStagingFromSemantic(255) error = nil, want fail closed")
	}
}

func TestWriteSearchIndexJSONRejectsInvalidRecovery(t *testing.T) {
	valid, err := newBuildErrorEnvelope(searchReasonRateLimited, nil)
	if err != nil {
		t.Fatalf("newBuildErrorEnvelope() error: %v", err)
	}
	tests := []struct {
		name string
		body any
		want string
	}{
		{name: "search envelope", body: searchErrorEnvelope{Error: searchError{Reason: searchReasonPrivacyUnavailable}}, want: "search-index output: unsupported envelope agent.searchErrorEnvelope"},
		{name: "search-only reason", body: buildErrorEnvelope{Error: buildRecovery{Reason: searchReasonCacheCold, ActiveGeneration: buildActiveNotInspected, StagingGeneration: buildStagingNotInspected, NextAction: buildNextRetry}}, want: "search-index output: invalid unavailable reason"},
		{name: "active state", body: buildErrorEnvelope{Error: buildRecovery{Reason: searchReasonRateLimited, ActiveGeneration: "future", StagingGeneration: buildStagingNotInspected, NextAction: buildNextWait}}, want: "search-index output: invalid active generation state"},
		{name: "staging state", body: buildErrorEnvelope{Error: buildRecovery{Reason: searchReasonRateLimited, ActiveGeneration: buildActiveNotInspected, StagingGeneration: "future", NextAction: buildNextWait}}, want: "search-index output: invalid staging generation state"},
		{name: "exhausted without authorization", body: buildErrorEnvelope{Error: buildRecovery{Reason: searchReasonAttemptBudgetExhausted, ActiveGeneration: buildActiveAbsent, StagingGeneration: buildStagingNotInspected, NextAction: buildNextRenew}}, want: "search-index output: exhausted attempt budget requires authorization"},
		{name: "retry safe", body: buildErrorEnvelope{Error: buildRecovery{Reason: searchReasonRateLimited, ActiveGeneration: buildActiveNotInspected, StagingGeneration: buildStagingNotInspected, RetrySafe: true, NextAction: buildNextWait}}, want: "search-index output: retry_safe must be false"},
		{name: "next action", body: buildErrorEnvelope{Error: buildRecovery{Reason: searchReasonRateLimited, ActiveGeneration: buildActiveNotInspected, StagingGeneration: buildStagingNotInspected, NextAction: buildNextRetry}}, want: "search-index output: invalid recovery action"},
		{name: "internal detail", body: buildInternalEnvelope{InternalError: buildInternalRecovery{Detail: "future", ActiveGeneration: buildActiveNotInspected, StagingGeneration: buildStagingNotInspected, NextAction: buildNextRepairYomihon}}, want: "search-index output: invalid internal error detail"},
		{name: "internal recovery", body: buildInternalEnvelope{InternalError: buildInternalRecovery{Detail: searchInternalDetail, ActiveGeneration: buildActiveNotInspected, StagingGeneration: buildStagingNotInspected, NextAction: buildNextRetry}}, want: "search-index output: invalid internal recovery"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := writeSearchIndexJSON(&buf, tt.body)
			if err == nil || err.Error() != tt.want {
				t.Errorf("writeSearchIndexJSON() error = %v, want %q", err, tt.want)
			}
			if buf.Len() != 0 {
				t.Errorf("writeSearchIndexJSON() wrote %q for invalid recovery", buf.Bytes())
			}
		})
	}

	valid.Error.NextAction = buildNextRenew
	var buf bytes.Buffer
	if err := writeSearchIndexJSON(&buf, valid); err == nil {
		t.Error("writeSearchIndexJSON() accepted a mismatched valid-envelope mutation")
	}
}

func TestWriteSearchIndexJSONRejectsInvalidAnswer(t *testing.T) {
	tests := []struct {
		name string
		body searchIndexAnswerEnvelope
		want string
	}{
		{name: "status", body: searchIndexAnswerEnvelope{Status: "future", TopKP95US: 1}, want: "search-index output: invalid status"},
		{name: "negative", body: searchIndexAnswerEnvelope{Status: searchIndexStatusBuilt, Chunks: -1, TopKP95US: 1}, want: "search-index output: negative count"},
		{name: "totals", body: searchIndexAnswerEnvelope{Status: searchIndexStatusBuilt, Chunks: 2, Embedded: 1, TopKP95US: 1}, want: "search-index output: chunk counts do not reconcile"},
		{name: "current embedded", body: searchIndexAnswerEnvelope{Status: searchIndexStatusCurrent, Chunks: 1, Embedded: 1, TopKP95US: 1}, want: "search-index output: current generation cannot report embedded chunks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := writeSearchIndexJSON(&buf, tt.body)
			if err == nil || err.Error() != tt.want {
				t.Errorf("writeSearchIndexJSON() error = %v, want %q", err, tt.want)
			}
			if buf.Len() != 0 {
				t.Errorf("writeSearchIndexJSON() wrote %q for invalid answer", buf.Bytes())
			}
		})
	}
}

func TestWriteSearchJSONErrorEnvelopes(t *testing.T) {
	tests := []struct {
		name string
		body any
		want string
	}{
		{
			name: "privacy unanswerable",
			body: searchErrorEnvelope{Error: searchError{Reason: searchReasonPrivacyUnavailable}},
			want: readWireGolden(t, "unanswerable.jsonl"),
		},
		{
			name: "metadata unanswerable",
			body: searchErrorEnvelope{Error: searchError{Reason: searchReasonMetadataUnavailable}},
			want: "{\"error\":{\"reason\":\"metadata-filters-unavailable\"}}\n",
		},
		{
			name: "internal",
			body: newSearchInternalEnvelope(),
			want: readWireGolden(t, "internal-error.jsonl"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeSearchJSON(&buf, tt.body); err != nil {
				t.Fatalf("writeSearchJSON() error: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("writeSearchJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteSearchJSONNormalizesZeroResults(t *testing.T) {
	body := searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticOff}
	var buf bytes.Buffer
	if err := writeSearchJSON(&buf, body); err != nil {
		t.Fatalf("writeSearchJSON() error: %v", err)
	}
	want := "{\"mode\":\"lexical\",\"semantic\":\"off\",\"results\":[]}\n"
	if got := buf.String(); got != want {
		t.Errorf("writeSearchJSON() = %q, want %q", got, want)
	}
}

func TestWriteSearchJSONSemanticOnlyEvidence(t *testing.T) {
	body := searchAnswerEnvelope{
		Mode: searchModeHybrid, Semantic: searchSemanticOK,
		Results: []searchWireResult{{
			Rank: 1, RelPath: "Concepts/x.md", Title: "X", Snippet: "semantic evidence",
			Channels:     []searchChannel{searchChannelSemantic},
			ChannelRanks: searchChannelRanks{Semantic: new(3)},
		}},
	}
	var buf bytes.Buffer
	if err := writeSearchJSON(&buf, body); err != nil {
		t.Fatalf("writeSearchJSON() error: %v", err)
	}
	want := "{\"mode\":\"hybrid\",\"semantic\":\"ok\",\"results\":[{\"rank\":1,\"rel_path\":\"Concepts/x.md\",\"title\":\"X\",\"snippet\":\"semantic evidence\",\"channels\":[\"semantic\"],\"channel_ranks\":{\"semantic\":3}}]}\n"
	if got := buf.String(); got != want {
		t.Errorf("writeSearchJSON() = %q, want %q", got, want)
	}
}

func TestWriteSearchJSONEscapeClasses(t *testing.T) {
	prefix := []byte(`{"mode":"lexical","semantic":"off","results":[{"rank":1,"rel_path":"x.md","title":"`)
	suffix := []byte(`","channels":["lexical"],"channel_ranks":{"lexical":1}}]}` + "\n")
	tests := []struct {
		name      string
		title     string
		wantTitle []byte
	}{
		{name: "CJK and HTML characters", title: "判<>&", wantTitle: []byte("判<>&")},
		{name: "line separator", title: string(rune(0x2028)), wantTitle: []byte{0xe2, 0x80, 0xa8}},
		{name: "paragraph separator", title: string(rune(0x2029)), wantTitle: []byte{0xe2, 0x80, 0xa9}},
		{name: "short control escape", title: "line\nbreak", wantTitle: []byte(`line\nbreak`)},
		{name: "hex control escape", title: string(rune(0)), wantTitle: []byte(`\u0000`)},
		{name: "literal separator spelling", title: `\u2028`, wantTitle: []byte(`\\u2028`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := searchAnswerEnvelope{
				Mode:     searchModeLexical,
				Semantic: searchSemanticOff,
				Results: []searchWireResult{{
					Rank:         1,
					RelPath:      "x.md",
					Title:        tt.title,
					Channels:     []searchChannel{searchChannelLexical},
					ChannelRanks: searchChannelRanks{Lexical: new(1)},
				}},
			}
			var buf bytes.Buffer
			if err := writeSearchJSON(&buf, body); err != nil {
				t.Fatalf("writeSearchJSON() error: %v", err)
			}
			want := append(bytes.Clone(prefix), tt.wantTitle...)
			want = append(want, suffix...)
			if !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("writeSearchJSON() = %q, want %q", buf.Bytes(), want)
			}
		})
	}
}

func TestWriteSearchJSONRejectsInvalidEnvelope(t *testing.T) {
	tests := []struct {
		name string
		body any
		want string
	}{
		{
			name: "illegal pair",
			body: searchAnswerEnvelope{Mode: searchModeHybrid, Semantic: searchSemanticOff, Results: []searchWireResult{}},
			want: "search output: illegal mode and semantic pair hybrid/off",
		},
		{
			name: "unavailable without coverage",
			body: searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticUnavailable, Results: []searchWireResult{}},
			want: "search output: semantic unavailable requires coverage",
		},
		{
			name: "wrong error reason",
			body: searchErrorEnvelope{Error: searchError{Reason: searchReasonCacheCold}},
			want: "search output: invalid unanswerable reason",
		},
		{
			name: "non-dense result rank",
			body: searchAnswerEnvelope{
				Mode:     searchModeLexical,
				Semantic: searchSemanticOff,
				Results: []searchWireResult{{
					Rank: 2, RelPath: "x.md", Title: "X",
					Channels:     []searchChannel{searchChannelLexical},
					ChannelRanks: searchChannelRanks{Lexical: new(1)},
				}},
			},
			want: "search output: result 0 has rank 2, want 1",
		},
		{
			name: "coverage when off",
			body: searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticOff, Coverage: &searchCoverage{Reason: searchReasonCacheCold}, Results: []searchWireResult{}},
			want: "search output: semantic off cannot carry coverage",
		},
		{
			name: "wrong not-applicable coverage",
			body: searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticNotApplicable, Coverage: &searchCoverage{Reason: searchReasonCacheCold}, Results: []searchWireResult{}},
			want: "search output: semantic not-applicable requires matching coverage",
		},
		{
			name: "unanswerable reason in coverage",
			body: searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticUnavailable, Coverage: &searchCoverage{Reason: searchReasonPrivacyUnavailable}, Results: []searchWireResult{}},
			want: "search output: invalid answerable unavailable reason",
		},
		{
			name: "tampered internal detail",
			body: searchInternalEnvelope{InternalError: searchInternalError{Detail: "query"}},
			want: "search output: invalid internal error detail",
		},
		{
			name: "unsupported envelope",
			body: struct{}{},
			want: "search output: unsupported envelope struct {}",
		},
		{
			name: "invalid channel order",
			body: searchAnswerEnvelope{
				Mode: searchModeHybrid, Semantic: searchSemanticOK,
				Results: []searchWireResult{{
					Rank: 1, RelPath: "x.md", Title: "X", Snippet: "evidence",
					Channels:     []searchChannel{searchChannelSemantic, searchChannelLexical},
					ChannelRanks: searchChannelRanks{Lexical: new(1), Semantic: new(1)},
				}},
			},
			want: "search output: result 0 has invalid channels",
		},
		{
			name: "semantic channel on lexical envelope",
			body: searchAnswerEnvelope{
				Mode: searchModeLexical, Semantic: searchSemanticOff,
				Results: []searchWireResult{{
					Rank: 1, RelPath: "x.md", Title: "X", Snippet: "evidence",
					Channels:     []searchChannel{searchChannelSemantic},
					ChannelRanks: searchChannelRanks{Semantic: new(1)},
				}},
			},
			want: "search output: lexical result 0 carries semantic channel",
		},
		{
			name: "semantic channel without evidence",
			body: searchAnswerEnvelope{
				Mode: searchModeHybrid, Semantic: searchSemanticOK,
				Results: []searchWireResult{{
					Rank: 1, RelPath: "x.md", Title: "X",
					Channels:     []searchChannel{searchChannelSemantic},
					ChannelRanks: searchChannelRanks{Semantic: new(1)},
				}},
			},
			want: "search output: semantic result 0 has no snippet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := writeSearchJSON(&buf, tt.body)
			if err == nil || err.Error() != tt.want {
				t.Errorf("writeSearchJSON() error = %v, want %q", err, tt.want)
			}
			if buf.Len() != 0 {
				t.Errorf("writeSearchJSON() wrote %q for invalid envelope, want empty", buf.Bytes())
			}
		})
	}
}

func TestSearchStderrLine(t *testing.T) {
	tests := []struct {
		name   string
		reason searchReason
		want   string
	}{
		{name: "privacy", reason: searchReasonPrivacyUnavailable, want: "yomihon search: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n"},
		{name: "metadata", reason: searchReasonMetadataUnavailable, want: "yomihon search: metadata-filters-unavailable: the vault contract declares no valid artifact policy, so metadata filters cannot be evaluated\n"},
		{name: "artifact policy", reason: searchReasonArtifactPolicyUnavailable, want: "yomihon search: artifact-policy-unavailable: the vault contract declares no valid artifact policy, so the semantic corpus cannot exist\n"},
		{name: "cold", reason: searchReasonCacheCold, want: "yomihon search: cache-cold: no semantic index exists; run yomihon search-index build\n"},
		{name: "corrupt", reason: searchReasonCacheCorrupt, want: "yomihon search: cache-corrupt: the semantic index is unreadable; run yomihon search-index build\n"},
		{name: "mismatch", reason: searchReasonCacheMismatch, want: "yomihon search: cache-mismatch: the semantic index uses a different configuration; run yomihon search-index build\n"},
		{name: "retired", reason: searchReasonEmbedderRetired, want: "yomihon search: embedder-retired: the old index's embedding model is no longer available\n"},
		{name: "unconfigured", reason: searchReasonEmbedderUnconfigured, want: "yomihon search: embedder-unconfigured: no embedding key is configured, so semantic search is off\n"},
		{name: "unreachable", reason: searchReasonEmbedderUnreachable, want: "yomihon search: embedder-unreachable: the embedding API did not answer\n"},
		{name: "failed", reason: searchReasonEmbedderFailed, want: readWireGolden(t, "embedder-failed.stderr")},
		{name: "rejected", reason: searchReasonEmbedderRejected, want: "yomihon search: embedder-rejected: the embedding API refused the credential\n"},
		{name: "rate limited", reason: searchReasonRateLimited, want: "yomihon search: rate-limited: the embedding API is rate-limiting; try again shortly\n"},
		{name: "attempt budget exhausted", reason: searchReasonAttemptBudgetExhausted, want: "yomihon search: attempt-budget-exhausted: semantic document sends need renewed authorization; run yomihon search-index build --renew-attempt-budget\n"},
		{name: "rebuild", reason: searchReasonRebuildRequired, want: "yomihon search: rebuild-required: the vault changed too much for an interactive refresh; run yomihon search-index build\n"},
		{name: "refreshing", reason: searchReasonIndexRefreshing, want: "yomihon search: index-refreshing: another process is updating the semantic index\n"},
		{name: "incomplete", reason: searchReasonIndexIncomplete, want: "yomihon search: index-incomplete: one or more current chunks could not be indexed\n"},
		{name: "vault changed", reason: searchReasonVaultChanged, want: "yomihon search: vault-changed: the vault changed while the semantic index was being updated; retry\n"},
		{name: "capacity", reason: searchReasonCapacity, want: "yomihon search: capacity: the semantic index could not be loaded into memory\n"},
		{name: "unsupported platform", reason: searchReasonUnsupportedPlatform, want: "yomihon search: unsupported-platform: the semantic generation store is not supported on this platform\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := searchStderrLine(tt.reason)
			if err != nil {
				t.Fatalf("searchStderrLine() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("searchStderrLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearchIndexAttemptBudgetStderrLines(t *testing.T) {
	tests := []struct {
		reason searchReason
		want   string
	}{
		{reason: searchReasonAttemptBudgetExhausted, want: readWireGolden(t, "index-attempt-budget-exhausted.stderr")},
		{reason: searchReasonAttemptBudgetNotRenewable, want: readWireGolden(t, "index-attempt-budget-not-renewable.stderr")},
	}
	for _, tt := range tests {
		got, err := searchIndexStderrLine(tt.reason)
		if err != nil {
			t.Fatalf("searchIndexStderrLine(%q) error: %v", tt.reason, err)
		}
		if got != tt.want {
			t.Errorf("searchIndexStderrLine(%q) = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestSearchInternalStderrLine(t *testing.T) {
	want := readWireGolden(t, "internal-error.stderr")
	if got := searchInternalStderrLine(); got != want {
		t.Errorf("searchInternalStderrLine() = %q, want %q", got, want)
	}
}

func TestSearchErrorsDoNotEchoQuery(t *testing.T) {
	query := "sentinel-private-query"
	_, parseErr := parseSearchArgs([]string{query, "--unknown"})
	if parseErr == nil {
		t.Fatal("parseSearchArgs() error = nil, want usage error")
	}
	if strings.Contains(parseErr.Error(), query) {
		t.Errorf("parseSearchArgs() error echoed query: %q", parseErr)
	}

	body := searchAnswerEnvelope{
		Mode: searchModeLexical, Semantic: searchSemanticOff,
		Results: []searchWireResult{{
			Rank: 2, RelPath: "x.md", Title: query,
			Channels:     []searchChannel{searchChannelLexical},
			ChannelRanks: searchChannelRanks{Lexical: new(1)},
		}},
	}
	var buf bytes.Buffer
	wireErr := writeSearchJSON(&buf, body)
	if wireErr == nil {
		t.Fatal("writeSearchJSON() error = nil, want validation error")
	}
	if strings.Contains(wireErr.Error(), query) {
		t.Errorf("writeSearchJSON() error echoed query: %q", wireErr)
	}
	_, stderrErr := searchStderrLine(searchReason(query))
	if stderrErr == nil {
		t.Fatal("searchStderrLine() error = nil, want unknown-reason error")
	}
	if strings.Contains(stderrErr.Error(), query) {
		t.Errorf("searchStderrLine() error echoed query: %q", stderrErr)
	}
}

func TestWriteSearchJSONWriterErrors(t *testing.T) {
	body := searchAnswerEnvelope{Mode: searchModeLexical, Semantic: searchSemanticOff}
	t.Run("writer error", func(t *testing.T) {
		writeErr := errors.New("disk full")
		err := writeSearchJSON(errorWriter{err: writeErr}, body)
		if !errors.Is(err, writeErr) {
			t.Errorf("writeSearchJSON() error = %v, want wrapped writer error", err)
		}
	})
	t.Run("short write", func(t *testing.T) {
		err := writeSearchJSON(shortWriter{}, body)
		if !errors.Is(err, io.ErrShortWrite) {
			t.Errorf("writeSearchJSON() error = %v, want %v", err, io.ErrShortWrite)
		}
	})
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

func readWireGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name)) // #nosec G304 -- callers supply frozen test-owned basenames
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", name, err)
	}
	return string(data)
}
