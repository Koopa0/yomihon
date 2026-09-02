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
	"github.com/koopa0/yomihon/internal/wording"
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
		// wantSummary, when set, pins which sentence this refusal opens with —
		// asked where two sentinels are near neighbours and sharing one
		// sentence would hide which entry the operator has to repair.
		wantSummary wording.Phrase
	}{
		{name: "invalid path", err: ErrInvalidPath, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "closed", err: ErrClosed, code: http.StatusServiceUnavailable, noDetail: true},
		{name: "artifact policy", err: errors.Join(ErrArtifactPolicyUnavailable, errors.New("policy detail")), code: http.StatusServiceUnavailable, wantDetail: "policy detail"},
		{name: "non instance", err: ErrNonInstance, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "target not regular", err: errors.Join(errNotRegular, errors.New("leaf detail")), code: http.StatusUnprocessableEntity, wantDetail: "leaf detail", wantSummary: wording.TargetNotRegular},
		{name: "path not regular", err: errors.Join(errPathNotRegular, errors.New("component detail")), code: http.StatusUnprocessableEntity, wantDetail: "component detail", wantSummary: wording.PathNotRegular},
		{name: "stale", err: ErrStale, code: http.StatusConflict, noDetail: true},
		{name: "concurrent write", err: ErrConcurrentWrite, code: http.StatusConflict, noDetail: true},
		{name: "status line", err: ErrStatusLine, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "status syntax unsupported", err: ErrStatusSyntaxUnsupported, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "published reserved", err: ErrPublishedReserved, code: http.StatusUnprocessableEntity, noDetail: true},
		{name: "unknown status", err: errors.Join(schema.ErrUnknownStatus, errors.New("unknown-status detail")), code: http.StatusUnprocessableEntity, wantDetail: "unknown-status detail"},
		{name: "illegal transition", err: errors.Join(schema.ErrIllegalTransition, errors.New("transition detail")), code: http.StatusUnprocessableEntity, wantDetail: "transition detail"},
		{name: "install uncertain", err: errors.Join(ErrInstallUncertain, errors.New("disk barrier failed")), code: http.StatusInternalServerError, changed: true, noDetail: true},
		{name: "install stranded", err: errors.Join(ErrInstallStranded, errors.New("stranded detail")), code: http.StatusInternalServerError, changed: true, wantDetail: "stranded detail"},
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
			if got.summary == (wording.Phrase{}) || got.nextAction == (wording.Phrase{}) {
				t.Errorf("recovery must include summary and next action: %#v", got)
			}
			if tt.wantSummary != (wording.Phrase{}) && got.summary != tt.wantSummary {
				t.Errorf("summary = %q, want %q", got.summary.In(wording.En), tt.wantSummary.In(wording.En))
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

func TestOnlyPostInstallFailuresClaimChanged(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		ErrInvalidPath,
		ErrClosed,
		ErrArtifactPolicyUnavailable,
		ErrNonInstance,
		errNotRegular,
		errPathNotRegular,
		ErrStale,
		ErrConcurrentWrite,
		ErrStatusLine,
		ErrStatusSyntaxUnsupported,
		ErrPublishedReserved,
		schema.ErrUnknownStatus,
		schema.ErrIllegalTransition,
		fs.ErrNotExist,
		errors.New("unknown"),
	} {
		if got := recoveryFor(err); got.changed {
			t.Errorf("recoveryFor(%v).changed = true, want false", err)
		}
	}
	for _, err := range []error{ErrInstallUncertain, ErrInstallStranded} {
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
		err := writeRecovery(t.Context(), recorder, http.StatusConflict, changed, component, wording.ZhHant)
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
	err := writeRecovery(t.Context(), w, http.StatusConflict, false, component, wording.ZhHant)
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
	if err := writeRecovery(t.Context(), recorder, http.StatusConflict, false, component, wording.ZhHant); err != nil {
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
// A zero byte cannot name a file on any platform this runs on, so the request
// used to travel all the way to the filesystem and return an error nothing
// recognized, which reached the reader as "yomihon could not complete this" —
// a fault report for what was only a malformed path. The rest of the range —
// line endings, tabs, escapes — has the same character: no note is named with
// one, so the request is refused up front rather than quoted onward into
// errors and logs.
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
		{name: "line ending", rel: note + "\n\nmore text"},
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
// punctuation is still a note this face can write.
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

// TestRecoveryNextActionDropsTheDoorItCannotShow holds the repair sentence to
// the page that carries it: the wording that names the "Open in Obsidian"
// action survives only alongside that action, and a page without the door
// states the same repair without pointing below itself.
func TestRecoveryNextActionDropsTheDoorItCannotShow(t *testing.T) {
	t.Parallel()
	plain := &recovery{nextAction: wording.RecoveryStartOver}
	door := &recovery{nextAction: wording.SchemaRefusalNext, nextActionNamesDoor: true}
	cases := []struct {
		name    string
		failure *recovery
		href    string
		want    wording.Phrase
	}{
		{"a door-naming sentence keeps its door", door, "obsidian://open?path=x", wording.SchemaRefusalNext},
		{"a door-naming sentence loses a door the page lacks", door, "", wording.SchemaRefusalNextNoDoor},
		{"a plain sentence is never swapped", plain, "", wording.RecoveryStartOver},
		{"a plain sentence ignores the door", plain, "obsidian://open?path=x", wording.RecoveryStartOver},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := recoveryNextAction(tc.failure, tc.href); got != tc.want {
				t.Fatalf("recoveryNextAction picked %q, want %q", got.In(wording.ZhHant), tc.want.In(wording.ZhHant))
			}
		})
	}
}

// TestSchemaRecoveryNamesTheDoor pins the producing side of that swap: the
// schema refusal is the one recovery whose sentence points at the editor
// action, so it must arrive marked as doing so.
func TestSchemaRecoveryNamesTheDoor(t *testing.T) {
	t.Parallel()
	r := schemaRecovery(wording.StatusOutsideEnum, errors.New("x"))
	if !r.nextActionNamesDoor {
		t.Fatal("schemaRecovery arrived without the door mark its sentence relies on")
	}
}
