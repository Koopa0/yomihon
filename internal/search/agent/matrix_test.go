package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/koopa0/yomihon/internal/search/semantic"
)

type matrixSearch struct {
	corpus semantic.Corpus
	err    error
	calls  int
}

func (s *matrixSearch) CorpusFingerprint() [sha256.Size]byte {
	return s.corpus.Fingerprint
}

func (s *matrixSearch) Search(
	_ context.Context,
	_ string,
	allowed map[string]struct{},
	_ int,
) ([]semantic.Result, error) {
	s.calls++
	if s.err != nil || len(s.corpus.Chunks) == 0 {
		return nil, s.err
	}
	document := s.corpus.Chunks[0]
	if _, ok := allowed[document.RelPath]; !ok {
		return nil, ErrEvidenceMismatch
	}
	return []semantic.Result{{
		RelPath:      document.RelPath,
		ChunkOrdinal: document.Ordinal,
		Score:        1,
	}}, nil
}

type matrixFailureStage uint8

const (
	matrixReconcileFailure matrixFailureStage = 1
	matrixQueryFailure     matrixFailureStage = 2
)

type matrixCase struct {
	row      int
	variant  string
	stage    matrixFailureStage
	err      error
	wantExit int
	reason   searchReason
}

func TestSearchGateMatrix(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})

	provider := func(kind semantic.EmbedFailureKind) error {
		return &semantic.EmbedError{Kind: kind}
	}
	tests := []matrixCase{
		{row: 0, variant: "unsupported platform", stage: matrixReconcileFailure, err: semantic.ErrStoreUnsupportedPlatform, wantExit: 3, reason: searchReasonUnsupportedPlatform},
		{row: 1, variant: "cold", stage: matrixReconcileFailure, err: semantic.ErrStoreNotFound, wantExit: 3, reason: searchReasonCacheCold},
		{row: 2, variant: "corrupt", stage: matrixReconcileFailure, err: semantic.ErrStoreCorrupt, wantExit: 3, reason: searchReasonCacheCorrupt},
		{row: 3, variant: "identity mismatch", stage: matrixReconcileFailure, err: semantic.ErrGenerationMismatch, wantExit: 3, reason: searchReasonCacheMismatch},
		{row: 4, variant: "retired", stage: matrixReconcileFailure, err: semantic.ErrEmbedderRetired, wantExit: 3, reason: searchReasonEmbedderRetired},
		{row: 5, variant: "capacity", stage: matrixReconcileFailure, err: semantic.ErrIndexCapacity, wantExit: 3, reason: searchReasonCapacity},
		{row: 6, variant: "unconfigured", stage: matrixReconcileFailure, err: semantic.ErrEmbedderUnconfigured, wantExit: 3, reason: searchReasonEmbedderUnconfigured},
		{row: 7, variant: "success", wantExit: 0},
		{row: 8, variant: "query transport", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureUnreachable), wantExit: 3, reason: searchReasonEmbedderUnreachable},
		{row: 9, variant: "query throttle", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureRateLimited), wantExit: 3, reason: searchReasonRateLimited},
		{row: 10, variant: "query rejected", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureRejected), wantExit: 3, reason: searchReasonEmbedderRejected},
		{row: 11, variant: "query provider", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureProvider), wantExit: 3, reason: searchReasonEmbedderFailed},
		{row: 11, variant: "query unknown", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureKind("future-provider-status")), wantExit: 3, reason: searchReasonEmbedderFailed},
		{row: 12, variant: "query malformed", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureMalformedRequest), wantExit: 1},
		{row: 12, variant: "query local formation", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureInternal), wantExit: 1},
		{row: 13, variant: "rebuild required", stage: matrixReconcileFailure, err: semantic.ErrRebuildRequired, wantExit: 3, reason: searchReasonRebuildRequired},
		{row: 14, variant: "drift unconfigured", stage: matrixReconcileFailure, err: semantic.ErrEmbedderUnconfigured, wantExit: 3, reason: searchReasonEmbedderUnconfigured},
		{row: 15, variant: "writer held", stage: matrixReconcileFailure, err: semantic.ErrWriterHeld, wantExit: 3, reason: searchReasonIndexRefreshing},
		{row: 16, variant: "winner current success", wantExit: 0},
		{row: 16, variant: "winner current unreachable", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureUnreachable), wantExit: 3, reason: searchReasonEmbedderUnreachable},
		{row: 16, variant: "winner current throttle", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureRateLimited), wantExit: 3, reason: searchReasonRateLimited},
		{row: 16, variant: "winner current rejected", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureRejected), wantExit: 3, reason: searchReasonEmbedderRejected},
		{row: 16, variant: "winner current provider", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureProvider), wantExit: 3, reason: searchReasonEmbedderFailed},
		{row: 16, variant: "winner current malformed", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureMalformedRequest), wantExit: 1},
		{row: 17, variant: "incomplete", stage: matrixReconcileFailure, err: semantic.ErrGenerationIncomplete, wantExit: 3, reason: searchReasonIndexIncomplete},
		{row: 18, variant: "future retry", stage: matrixReconcileFailure, err: semantic.ErrRetryNotReady, wantExit: 3, reason: searchReasonRateLimited},
		{row: 18, variant: "document throttle", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureRateLimited), wantExit: 3, reason: searchReasonRateLimited},
		{row: 19, variant: "document attempt budget exhausted", stage: matrixReconcileFailure, err: semantic.ErrAttemptLimit, wantExit: 3, reason: searchReasonAttemptBudgetExhausted},
		{row: 20, variant: "document transport", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureUnreachable), wantExit: 3, reason: searchReasonEmbedderUnreachable},
		{row: 21, variant: "document rejected", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureRejected), wantExit: 3, reason: searchReasonEmbedderRejected},
		{row: 22, variant: "document provider", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureProvider), wantExit: 3, reason: searchReasonEmbedderFailed},
		{row: 22, variant: "document unknown", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureKind("future-provider-status")), wantExit: 3, reason: searchReasonEmbedderFailed},
		{row: 23, variant: "document malformed", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureMalformedRequest), wantExit: 1},
		{row: 23, variant: "document local formation", stage: matrixReconcileFailure, err: provider(semantic.EmbedFailureInternal), wantExit: 1},
		{row: 24, variant: "vault changed", stage: matrixReconcileFailure, err: semantic.ErrVaultChanged, wantExit: 3, reason: searchReasonVaultChanged},
		{row: 25, variant: "activated success", wantExit: 0},
		{row: 25, variant: "activated unreachable", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureUnreachable), wantExit: 3, reason: searchReasonEmbedderUnreachable},
		{row: 25, variant: "activated throttle", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureRateLimited), wantExit: 3, reason: searchReasonRateLimited},
		{row: 25, variant: "activated rejected", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureRejected), wantExit: 3, reason: searchReasonEmbedderRejected},
		{row: 25, variant: "activated provider", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureProvider), wantExit: 3, reason: searchReasonEmbedderFailed},
		{row: 25, variant: "activated malformed", stage: matrixQueryFailure, err: provider(semantic.EmbedFailureMalformedRequest), wantExit: 1},
	}

	assertMatrixRows(t, tests)
	for _, tt := range tests {
		t.Run(fmt.Sprintf("row-%02d/%s", tt.row, tt.variant), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			var query *matrixSearch
			reconcileCalls := 0
			exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
				stdout: &stdout,
				stderr: &stderr,
				openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
					return func(_ context.Context, corpus semantic.Corpus) (SemanticSearch, error) {
						reconcileCalls++
						if tt.stage == matrixReconcileFailure {
							return nil, tt.err
						}
						query = &matrixSearch{corpus: corpus}
						if tt.stage == matrixQueryFailure {
							query.err = tt.err
						}
						return query, nil
					}, nil
				},
			})
			if exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q", exit, tt.wantExit, stdout.Bytes(), stderr.Bytes())
			}
			if reconcileCalls != 1 {
				t.Fatalf("Reconcile calls = %d, want 1", reconcileCalls)
			}
			wantQueryCalls := 0
			if tt.stage != matrixReconcileFailure {
				wantQueryCalls = 1
			}
			if query != nil && query.calls != wantQueryCalls {
				t.Fatalf("semantic Search calls = %d, want %d", query.calls, wantQueryCalls)
			}
			assertMatrixEnvelope(t, stdout.Bytes(), stderr.String(), tt.wantExit, tt.reason)
		})
	}
}

func assertMatrixRows(t *testing.T, tests []matrixCase) {
	t.Helper()
	if missing := missingMatrixRows(tests); len(missing) != 0 {
		t.Fatalf("matrix rows missing: %v", missing)
	}
}

func missingMatrixRows(tests []matrixCase) []int {
	rows := make(map[int]struct{}, 26)
	for _, item := range tests {
		row := item.row
		if row < 0 || row > 25 {
			continue
		}
		rows[row] = struct{}{}
	}
	missing := make([]int, 0, 26-len(rows))
	for row := range 26 {
		if _, ok := rows[row]; !ok {
			missing = append(missing, row)
		}
	}
	return missing
}

func assertMatrixEnvelope(t *testing.T, stdout []byte, stderr string, exit int, reason searchReason) {
	t.Helper()
	var body map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &body); err != nil {
		t.Fatalf("stdout is not one JSON object: %v; bytes=%q", err, stdout)
	}
	if _, echoed := body["query"]; echoed {
		t.Fatal("wire contains a dedicated query echo field")
	}
	switch exit {
	case 0:
		if string(body["mode"]) != `"hybrid"` || string(body["semantic"]) != `"ok"` {
			t.Fatalf("success discriminant = mode %s semantic %s", body["mode"], body["semantic"])
		}
		if _, ok := body["results"]; !ok {
			t.Fatal("success has no results field")
		}
		if stderr != "" {
			t.Fatalf("success stderr = %q, want empty", stderr)
		}
	case 1:
		if string(body["internal_error"]) != `{"detail":"the request could not be formed correctly"}` || len(body) != 1 {
			t.Fatalf("internal envelope = %s", stdout)
		}
		if stderr != searchInternalStderrLine() {
			t.Fatalf("internal stderr = %q, want %q", stderr, searchInternalStderrLine())
		}
	case 3:
		if string(body["mode"]) != `"lexical"` || string(body["semantic"]) != `"unavailable"` {
			t.Fatalf("unavailable discriminant = mode %s semantic %s", body["mode"], body["semantic"])
		}
		if _, ok := body["results"]; !ok {
			t.Fatal("unavailable answer has no lexical results field")
		}
		var coverage searchCoverage
		if err := json.Unmarshal(body["coverage"], &coverage); err != nil || coverage.Reason != reason {
			t.Fatalf("coverage = %s, want reason %q (error=%v)", body["coverage"], reason, err)
		}
		want, err := searchStderrLine(reason)
		if err != nil {
			t.Fatalf("test row uses unknown reason %q", reason)
		}
		if stderr != want {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	default:
		t.Fatalf("test has unsupported exit %d", exit)
	}
}

func TestSearchGateMatrixRejectsMissingBaseRow(t *testing.T) {
	missing := missingMatrixRows([]matrixCase{{row: 0}})
	if len(missing) != 25 || missing[0] != 1 || missing[len(missing)-1] != 25 {
		t.Fatalf("missingMatrixRows() = %v, want 1..25", missing)
	}
}
