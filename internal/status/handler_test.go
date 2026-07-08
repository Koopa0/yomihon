package status_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/status"
)

func newHandlerServer(t *testing.T, svc *status.Service) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	status.NewHandler(svc, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// postStatus posts to /status without following redirects, so 303 responses
// can be asserted directly.
func postStatus(t *testing.T, srv *httptest.Server, form url.Values) (statusCode int, location, body string) {
	t.Helper()
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/status", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /status: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header.Get("Location"), string(b)
}

func TestHandlerSuccess(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	srv := newHandlerServer(t, svc)

	code, location, _ := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {"ready"}})
	if code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", code, http.StatusSeeOther)
	}
	// The seal (→ ready) redirects with the one-shot ?sealed=1 the reading page
	// plays its settle animation from; a non-seal transition would omit it.
	if want := "/notes/" + testRel + "?sealed=1"; location != want {
		t.Errorf("Location = %q, want %q", location, want)
	}
}

func TestHandlerMissingFields(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))
	srv := newHandlerServer(t, svc)

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing path", url.Values{"from": {"draft"}, "to": {"ready"}}},
		{"missing from", url.Values{"path": {"a.md"}, "to": {"ready"}}},
		{"missing to", url.Values{"path": {"a.md"}, "from": {"draft"}}},
		{"blank path", url.Values{"path": {"  "}, "from": {"draft"}, "to": {"ready"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _, body := postStatus(t, srv, tt.form)
			if code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
			}
			if !strings.Contains(body, "required") {
				t.Errorf("body = %q, want substring %q", body, "required")
			}
		})
	}
}

func TestHandlerClosed(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, nil) // no contract: fail-closed
	srv := newHandlerServer(t, svc)

	code, _, body := postStatus(t, srv, url.Values{"path": {"a.md"}, "from": {"draft"}, "to": {"ready"}})
	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(body, "unavailable") || !strings.Contains(body, "closed") {
		t.Errorf("body = %q, want mention of the contract being unavailable and closed", body)
	}
}

func TestHandlerStale(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	srv := newHandlerServer(t, svc)

	// The page claims "imported"; the file actually says "draft".
	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"imported"}, "to": {"ready"}})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, "stale") || !strings.Contains(body, "reload") {
		t.Errorf("body = %q, want mention of a stale page and reloading", body)
	}
}

func TestHandlerDirty(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	committed := lessonContent("draft")
	writeNote(t, root, committed)
	commitAll(t, root)
	writeNote(t, root, committed+"<!-- uncommitted -->\n")
	srv := newHandlerServer(t, svc)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {"ready"}})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, "uncommitted") {
		t.Errorf("body = %q, want mention of uncommitted changes", body)
	}
}

// ErrStatusLine (422, "schema violation") is deliberately not exercised
// here through a full HTTP round trip: it can only fire when a note's
// current status is "" (either genuinely missing or unparseable — see
// status.rewriteStatusLine), and this handler rejects a blank "from" at
// the 422 pre-check above before Flip is ever called. So the branch is
// structurally unreachable through this endpoint; it exists in Flip as a
// defense-in-depth total-function guard for any future non-HTTP caller of
// Service.Flip. Its behavior is covered directly at the Service layer by
// status_test.go's TestFlipMalformedStatusLine.

func TestHandlerIllegalTransition(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("imported"))
	commitAll(t, root)
	srv := newHandlerServer(t, svc)

	// imported -> ready skips the required "draft" stage.
	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"imported"}, "to": {"ready"}})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	// The response must carry the schema's own rejection reason verbatim,
	// not a generic message: schema.Transition's actual wrapped text
	// ("transition not allowed by lifecycle: imported → ready for type
	// \"lesson\"") must survive to the response, not just a fixed string
	// that would be identical for every 422 regardless of cause.
	for _, want := range []string{"transition not allowed by lifecycle", "imported", "ready", "lesson"} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, want it to contain the schema's own rejection reason (missing %q)", body, want)
		}
	}
}

func TestHandlerGenericFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // deliberately not a git repo
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	srv := newHandlerServer(t, svc)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {"ready"}})
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if strings.Contains(body, "fatal:") || strings.Contains(body, root) {
		t.Errorf("body = %q, leaks internal error detail (git output or filesystem path)", body)
	}
}

// TestHandlerCommitFailedRoutesGitAddFailure guards handler.go's switch: a
// failing `git add` inside commit() must route to the same informative
// ErrCommitFailed branch as a failing `git commit`, not fall through to the
// generic "cannot flip status" 500 — the operator is owed the
// file-already-changed warning and raw git error for either staging or
// committing failing.
func TestHandlerCommitFailedRoutesGitAddFailure(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	srv := newHandlerServer(t, svc)

	lockPath := filepath.Join(root, ".git", "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("create index.lock: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(lockPath) })

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {"ready"}})
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if !strings.Contains(body, "rewritten") || !strings.Contains(body, "git commit failed") {
		t.Errorf("body = %q, want the file-already-changed + git-failed message, not the generic 500", body)
	}
}
