package status_test

import (
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

func newHandlerServer(t *testing.T, lifecycle *status.Lifecycle) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	status.NewHandler(lifecycle, func() pages.Shell { return pages.Shell{} }, slog.New(slog.DiscardHandler)).Register(mux)
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header.Get("Location"), string(b)
}

func TestHandlerSuccess(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	srv := newHandlerServer(t, lifecycle)

	code, location, _ := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}})
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
	lifecycle := newLifecycle(t, root, loadContract(t))
	srv := newHandlerServer(t, lifecycle)

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing path", url.Values{"from": {"draft"}, "to": {schema.SealStatus}}},
		{"missing from", url.Values{"path": {"a.md"}, "to": {schema.SealStatus}}},
		{"missing to", url.Values{"path": {"a.md"}, "from": {"draft"}}},
		{"blank path", url.Values{"path": {"  "}, "from": {"draft"}, "to": {schema.SealStatus}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _, body := postStatus(t, srv, tt.form)
			if code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
			}
			if !strings.Contains(body, "必填") {
				t.Errorf("body = %q, want the Traditional Chinese required-field explanation", body)
			}
		})
	}
}

func TestHandlerRejectsOversizedFormWithRecoveryPage(t *testing.T) {
	t.Parallel()
	srv := newHandlerServer(t, newLifecycle(t, t.TempDir(), nil))
	form := url.Values{
		"path": {strings.Repeat("x", 4097)},
		"from": {"draft"},
		"to":   {schema.SealStatus},
	}
	code, _, body := postStatus(t, srv, form)
	if code != http.StatusBadRequest {
		t.Fatalf("oversized POST status = %d, want %d", code, http.StatusBadRequest)
	}
	if !strings.Contains(body, "表單內容無法解析") || !strings.Contains(body, "狀態尚未變更") {
		t.Errorf("oversized POST body = %q, want the unchanged recovery page", body)
	}
	if strings.Contains(body, strings.Repeat("x", 128)) {
		t.Errorf("oversized POST body echoes request bytes: %q", body)
	}
}

func TestHandlerClosed(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, nil) // no contract: fail-closed
	srv := newHandlerServer(t, lifecycle)

	code, _, body := postStatus(t, srv, url.Values{"path": {"a.md"}, "from": {"draft"}, "to": {schema.SealStatus}})
	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(body, "無法使用") || !strings.Contains(body, "已關閉") {
		t.Errorf("body = %q, want the Traditional Chinese unavailable-and-closed explanation", body)
	}
}

func TestHandlerPathValidationPrecedesClosure(t *testing.T) {
	t.Parallel()
	srv := newHandlerServer(t, newLifecycle(t, t.TempDir(), nil))
	code, _, body := postStatus(t, srv, url.Values{
		"path": {"../outside.md"},
		"from": {"draft"},
		"to":   {schema.SealStatus},
	})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("POST path escape on closed service status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(body, "vault 內的相對 slash 路徑") {
		t.Errorf("POST path escape body = %q, want path-shape reason", body)
	}
	if strings.Contains(body, "contract 無法使用") {
		t.Errorf("POST path escape reached closure before path validation: %q", body)
	}
}

func TestHandlerNonInstance(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	srv := newHandlerServer(t, newLifecycle(t, root, loadContract(t)))

	code, _, body := postStatus(t, srv, url.Values{
		"path": {"System/templates/Missing.md"},
		"from": {"draft"},
		"to":   {schema.SealStatus},
	})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("POST non-instance status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(body, "不屬於生命週期治理範圍") {
		t.Errorf("POST non-instance body = %q, want the governed-instance explanation", body)
	}
}

func TestHandlerArtifactPolicyUnavailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract *schema.Contract
		want     string
	}{
		{name: "missing", contract: loadContractWithArtifactSection(t, ""), want: "contract declares no artifact policy; instance projections disabled until it does"},
		{name: "invalid", contract: loadContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\".\"]\n"), want: `invalid artifact policy: non_instance_dirs contains "."`},
		{name: "incomplete", contract: loadContractWithArtifactSection(t, "[artifacts]\n"), want: `invalid artifact policy: missing required key "non_instance_dirs"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lifecycle := newLifecycle(t, t.TempDir(), tt.contract)
			srv := newHandlerServer(t, lifecycle)
			code, _, body := postStatus(t, srv, url.Values{
				"path": {testRel},
				"from": {"draft"},
				"to":   {schema.SealStatus},
			})
			if code != http.StatusServiceUnavailable {
				t.Errorf("POST with unavailable artifact policy status = %d, want %d", code, http.StatusServiceUnavailable)
			}
			got := html.UnescapeString(strings.TrimSpace(body))
			if !strings.Contains(got, "生命週期寫入已關閉") || !strings.Contains(got, tt.want) {
				t.Errorf("POST with unavailable artifact policy body = %q, want Chinese closure plus exact diagnostic %q", got, tt.want)
			}
			if strings.Contains(body, "the vault contract is unavailable") {
				t.Errorf("artifact-policy response collapsed into core-contract response: %q", body)
			}
		})
	}
}

func TestHandlerRejectsChangedContractSource(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, data, 0o600); err != nil { // #nosec G703 -- fixed basename under this test's TempDir
		t.Fatalf("write mutable test contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", contractPath, err)
	}
	const current = `non_instance_dirs = ["System/templates"]`
	const changed = `non_instance_dirs = ["System/templates", "Writing"]`
	updated := strings.Replace(string(data), current, changed, 1)
	if updated == string(data) {
		t.Fatal("artifact-policy reclassification did not apply")
	}
	if err = os.WriteFile(contractPath, []byte(updated), 0o600); err != nil { // #nosec G703 -- fixed basename under this test's TempDir
		t.Fatalf("write reclassified contract: %v", err)
	}

	srv := newHandlerServer(t, newLifecycle(t, t.TempDir(), contract))
	code, _, body := postStatus(t, srv, url.Values{
		"path": {testRel},
		"from": {"draft"},
		"to":   {schema.SealStatus},
	})
	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	const want = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if got := html.UnescapeString(strings.TrimSpace(body)); !strings.Contains(got, "生命週期寫入已關閉") || !strings.Contains(got, want) {
		t.Errorf("body = %q, want Chinese closure plus exact diagnostic %q", got, want)
	}
}

func TestHandlerStale(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	srv := newHandlerServer(t, lifecycle)

	// The page claims "imported"; the file actually says "draft".
	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"imported"}, "to": {schema.SealStatus}})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, "頁面已過期") || !strings.Contains(body, "重新載入") {
		t.Errorf("body = %q, want the Traditional Chinese stale-page recovery", body)
	}
}

func TestHandlerTargetRemovedAfterPageLoad(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(testRel))); err != nil {
		t.Fatalf("remove status target: %v", err)
	}
	srv := newHandlerServer(t, lifecycle)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}})
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", code, http.StatusNotFound)
	}
	for _, want := range []string{"找不到這篇筆記", "刪除或移動", "狀態尚未變更"} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q: %q", want, body)
		}
	}
	if strings.Contains(body, `/notes/`+testRel) {
		t.Errorf("removed-target recovery offers a broken note link: %q", body)
	}
}

func TestHandlerDirty(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	committed := lessonContent("draft")
	writeNote(t, root, committed)
	commitAll(t, root)
	writeNote(t, root, committed+"<!-- uncommitted -->\n")
	srv := newHandlerServer(t, lifecycle)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, "尚未提交") || !strings.Contains(body, "稽核紀錄") {
		t.Errorf("body = %q, want the Traditional Chinese dirty-file audit warning", body)
	}
}

// ErrStatusLine (422, "schema violation") is deliberately not exercised
// here through a full HTTP round trip: reaching the surgical rewrite requires
// a blank current status, while this handler rejects a blank "from" before
// Flip is called. The branch remains a defense-in-depth guard for future
// non-HTTP callers and is covered directly by TestFlipMalformedStatusLine.

func TestHandlerIllegalTransition(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("imported"))
	commitAll(t, root)
	srv := newHandlerServer(t, lifecycle)

	// imported -> ready skips the required "draft" stage.
	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"imported"}, "to": {schema.SealStatus}})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	// The response must carry the schema's own rejection reason verbatim,
	// not a generic message: schema.Transition's actual wrapped text
	// ("transition not allowed by lifecycle: imported → ready for type
	// \"lesson\"") must survive to the response, not just a fixed string
	// that would be identical for every 422 regardless of cause.
	decoded := html.UnescapeString(body)
	for _, want := range []string{"transition not allowed by lifecycle", "imported", schema.SealStatus, "lesson"} {
		if !strings.Contains(decoded, want) {
			t.Errorf("body = %q, want it to contain the schema's own rejection reason (missing %q)", body, want)
		}
	}
}

// TestHandlerWorkTreeUnreadable covers a folder that is no git repository: the
// working tree cannot be inspected, so no transition can ever be recorded there.
// It is answered as an unavailable capability rather than an unexplained
// failure, and — the part that matters most — neither git's own message nor the
// filesystem path reaches the page.
//
// The generic unexplained-failure path keeps its own coverage in
// handler_internal_test.go's mapping table ("unknown internal").
func TestHandlerWorkTreeUnreadable(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // deliberately not a git repo
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	srv := newHandlerServer(t, lifecycle)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}})
	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(html.UnescapeString(body), status.GitBlockReason) {
		t.Errorf("body = %q, does not state why the write face is closed", body)
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
	lifecycle := newLifecycle(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	srv := newHandlerServer(t, lifecycle)

	lockPath := filepath.Join(root, ".git", "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatalf("create index.lock: %v", err)
	}
	removeOnCleanup(t, lockPath)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}})
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if !strings.Contains(body, "筆記已改寫") || !strings.Contains(body, "git commit 失敗") {
		t.Errorf("body = %q, want the file-already-changed + git-failed message, not the generic 500", body)
	}
}
