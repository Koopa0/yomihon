package status

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestRecoveryClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		err        error
		code       int
		changed    bool
		wantDetail string
		noDetail   bool
	}{
		{name: "invalid path", err: ErrInvalidPath, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "closed", err: ErrClosed, code: http.StatusServiceUnavailable, noDetail: true},
		{name: "artifact policy", err: errors.Join(ErrArtifactPolicyUnavailable, errors.New("policy detail")), code: http.StatusServiceUnavailable, wantDetail: "policy detail"},
		{name: "non instance", err: ErrNonInstance, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "stale", err: ErrStale, code: http.StatusConflict, noDetail: true},
		{name: "concurrent write", err: ErrConcurrentWrite, code: http.StatusConflict, noDetail: true},
		{name: "dirty", err: ErrDirty, code: http.StatusConflict, noDetail: true},
		{name: "work tree unreadable", err: errors.Join(ErrWorkTreeUnreadable, errors.New("fatal: not a git repository")), code: http.StatusServiceUnavailable, noDetail: true},
		{name: "detached head", err: ErrDetachedHead, code: http.StatusConflict, noDetail: true},
		{name: "status line", err: ErrStatusLine, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "status syntax unsupported", err: ErrStatusSyntaxUnsupported, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "unknown status", err: errors.Join(schema.ErrUnknownStatus, errors.New("unknown-status detail")), code: http.StatusUnprocessableEntity, wantDetail: "unknown-status detail"},
		{name: "illegal transition", err: errors.Join(schema.ErrIllegalTransition, errors.New("transition detail")), code: http.StatusUnprocessableEntity, wantDetail: "transition detail"},
		{name: "owner forbidden", err: errors.Join(schema.ErrOwnerForbidden, errors.New("owner detail")), code: http.StatusUnprocessableEntity, wantDetail: "owner detail"},
		{name: "publication uncertain", err: errors.Join(ErrPublishUncertain, errors.New("disk barrier failed")), code: http.StatusInternalServerError, changed: true, noDetail: true},
		{name: "commit failed", err: errors.Join(ErrCommitFailed, errors.New("git detail")), code: http.StatusInternalServerError, changed: true, wantDetail: "git detail"},
		{name: "receipt diverged", err: errors.Join(ErrReceiptDiverged, errors.New("receipt detail")), code: http.StatusInternalServerError, changed: true, wantDetail: "receipt detail"},
		{name: "publication stranded", err: errors.Join(ErrPublicationStranded, errors.New("stranded detail")), code: http.StatusInternalServerError, changed: true, wantDetail: "stranded detail"},
		{name: "target removed", err: fs.ErrNotExist, code: http.StatusNotFound, noDetail: true},
		{name: "unknown internal", err: errors.New("secret internal detail"), code: http.StatusInternalServerError, noDetail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := recoveryFor(tt.err)
			if got.code != tt.code {
				t.Errorf("code = %d, want %d", got.code, tt.code)
			}
			if got.changed != tt.changed {
				t.Errorf("changed = %v, want %v", got.changed, tt.changed)
			}
			if got.summary == "" || got.nextAction == "" {
				t.Errorf("recovery must include summary and next action: %#v", got)
			}
			if got.code >= http.StatusInternalServerError && (got.logMessage == "" || got.cause == nil) {
				t.Errorf("server failure must retain a log message and cause: %#v", got)
			}
			if tt.wantDetail != "" && !strings.Contains(got.technicalDetail, tt.wantDetail) {
				t.Errorf("technical detail = %q, want it to contain %q", got.technicalDetail, tt.wantDetail)
			}
			if tt.noDetail && got.technicalDetail != "" {
				t.Errorf("technical detail = %q, want none", got.technicalDetail)
			}
		})
	}
}

func TestOnlyPostPublicationFailuresClaimChanged(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		ErrInvalidPath,
		ErrClosed,
		ErrArtifactPolicyUnavailable,
		ErrNonInstance,
		ErrStale,
		ErrConcurrentWrite,
		ErrDirty,
		ErrDetachedHead,
		ErrStatusLine,
		ErrStatusSyntaxUnsupported,
		schema.ErrUnknownStatus,
		schema.ErrIllegalTransition,
		schema.ErrOwnerForbidden,
		fs.ErrNotExist,
		errors.New("unknown"),
	} {
		if got := recoveryFor(err); got.changed {
			t.Errorf("recoveryFor(%v).changed = true, want false", err)
		}
	}
	for _, err := range []error{ErrPublishUncertain, ErrCommitFailed, ErrReceiptDiverged, ErrPublicationStranded} {
		if got := recoveryFor(err); !got.changed {
			t.Errorf("recoveryFor(%v).changed = false, want true", err)
		}
	}
}

func TestWriteRecoveryBuffersBeforeCommittingResponse(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("render failed")
	component := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "partial secret output"); err != nil {
			return err
		}
		return wantErr
	})
	for _, changed := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		err := writeRecovery(recorder, t.Context(), http.StatusConflict, changed, component)
		if !errors.Is(err, wantErr) {
			t.Errorf("writeRecovery(changed=%v) error = %v, want %v", changed, err, wantErr)
		}
		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("writeRecovery(changed=%v) status = %d, want 500", changed, recorder.Code)
		}
		body := recorder.Body.String()
		if strings.Contains(body, "partial secret output") {
			t.Errorf("writeRecovery(changed=%v) leaked partial render: %q", changed, body)
		}
		if changed && !strings.Contains(body, "請勿重送") {
			t.Errorf("changed fallback = %q, want no-resubmit warning", body)
		}
		if !changed && !strings.Contains(body, "狀態尚未變更") {
			t.Errorf("unchanged fallback = %q, want unchanged truth", body)
		}
	}
}

func TestWriteRecoveryReportsFallbackWriteFailure(t *testing.T) {
	t.Parallel()
	renderErr := errors.New("render failed")
	writeErr := errors.New("client disconnected")
	component := templ.ComponentFunc(func(context.Context, io.Writer) error {
		return renderErr
	})
	w := &failingResponseWriter{header: make(http.Header), err: writeErr}
	err := writeRecovery(w, t.Context(), http.StatusConflict, false, component)
	if !errors.Is(err, renderErr) || !errors.Is(err, writeErr) {
		t.Errorf("writeRecovery() error = %v, want both render and fallback-write failures", err)
	}
	if w.code != http.StatusInternalServerError {
		t.Errorf("fallback status = %d, want %d", w.code, http.StatusInternalServerError)
	}
}

type failingResponseWriter struct {
	header http.Header
	code   int
	err    error
}

func (w *failingResponseWriter) Header() http.Header { return w.header }

func (w *failingResponseWriter) WriteHeader(code int) { w.code = code }

func (w *failingResponseWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteRecoverySetsBrowserSafetyHeaders(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	component := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<!doctype html><title>recovery</title>")
		return err
	})
	if err := writeRecovery(recorder, t.Context(), http.StatusConflict, false, component); err != nil {
		t.Fatalf("writeRecovery() error = %v", err)
	}
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	for name, want := range map[string]string{
		"Content-Type":           "text/html; charset=utf-8",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
	} {
		if got := recorder.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestRecoveryNotePathAcceptsOnlyNormalizedVaultLocalPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: ""},
		{input: "..", want: ""},
		{input: "../x.md", want: ""},
		{input: "/absolute.md", want: ""},
		{input: `a\b.md`, want: ""},
		{input: "Writing/日本 語.md", want: "Writing/日本 語.md"},
		{input: "Writing/a?.md", want: "Writing/a?.md"},
		{input: "Writing/a#.md", want: "Writing/a#.md"},
		{input: "a/../Writing/n.md", want: "Writing/n.md"},
	}
	for _, tt := range tests {
		if got := recoveryNotePath(tt.input); got != tt.want {
			t.Errorf("recoveryNotePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestNormalizeRelPathRefusesControlBytes asserts a path carrying a byte below
// a space, or the delete byte, is refused as a malformed request rather than
// carried further.
//
// Two different harms sit behind the one check. A zero byte cannot name a file
// on any platform this runs on, so the request used to travel all the way to
// the filesystem and return an error nothing recognized, which reached the
// reader as "yomihon could not complete this" — a fault report for what was
// only a malformed path. The others are refused for what they would reach: the
// path is written into the subject line of the commit that records the seal, so
// a line ending inside the path would end that subject and continue in a body
// of the sender's choosing, forging the shape of the receipt this face exists
// to produce.
func TestNormalizeRelPathRefusesControlBytes(t *testing.T) {
	t.Parallel()
	const note = "Writing/lessons/japanese/L01.md"
	tests := []struct {
		name string
		rel  string
	}{
		{name: "trailing zero byte", rel: note + "\x00"},
		{name: "leading zero byte", rel: "\x00" + note},
		{name: "interior zero byte", rel: "Writing/les\x00sons/L01.md"},
		{name: "line ending forges a commit body", rel: note + "\n\nnot the subject line"},
		{name: "carriage return", rel: note + "\r"},
		{name: "tab", rel: "Writing/les\tsons/L01.md"},
		{name: "delete byte", rel: note + "\x7f"},
		{name: "escape", rel: note + "\x1b[2K"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			relSlash, osPath, err := normalizeRelPath(tt.rel)
			if !errors.Is(err, ErrInvalidPath) {
				t.Errorf("normalizeRelPath(%q) = (%q, %q, %v), want ErrInvalidPath", tt.rel, relSlash, osPath, err)
			}
		})
	}
}

// TestNormalizeRelPathKeepsOrdinaryNames asserts the refusal above reaches only
// the bytes it names: a note whose name carries a space, a wide character or
// punctuation is still a note this face can seal.
func TestNormalizeRelPathKeepsOrdinaryNames(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{
		"Writing/lessons/japanese/L01.md",
		"Concepts/humanities/矜持.md",
		"Maps/topics/Go MOC.md",
		"Writing/2026-07-30 週記.md",
		"Notes/B-tree & friends (draft).md",
	} {
		if _, _, err := normalizeRelPath(rel); err != nil {
			t.Errorf("normalizeRelPath(%q) error = %v, want nil", rel, err)
		}
	}
}
