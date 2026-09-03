package status_test

import (
	"context"
	"encoding/hex"
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

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// formIdentity is the hex content_identity form value for fixture bytes as
// written — what the page's hidden field carries for that render.
func formIdentity(content string) string {
	identity := vault.ContentIdentity([]byte(content))
	return hex.EncodeToString(identity[:])
}

// wellFormedIdentity is a syntactically valid identity for requests refused
// before any note is read, where no rendered content exists to identify.
const wellFormedIdentity = "0000000000000000000000000000000000000000000000000000000000000000"

func newHandlerServer(t *testing.T, writer *status.Writer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	status.NewHandler(writer, func() nav.Shell { return nav.Shell{} }, slog.New(slog.DiscardHandler)).Register(mux)
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
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	srv := newHandlerServer(t, writer)

	code, location, _ := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {formIdentity(lessonContent("draft"))}})
	if code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", code, http.StatusSeeOther)
	}
	// Every successful flip lands back on the note page, and the target names
	// the status the note just left. That is what lets the page state the
	// change once instead of leaving a re-rendered chip to imply it — a chip
	// reads the same whether this press worked or another did, and a reader
	// who cannot see it receives nothing.
	if want := "/notes/" + testRel + "?from=draft"; location != want {
		t.Errorf("Location = %q, want %q", location, want)
	}
}

func TestHandlerMissingFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	srv := newHandlerServer(t, writer)

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing path", url.Values{"from": {"draft"}, "to": {schema.SealStatus}}},
		{"missing from", url.Values{"path": {"a.md"}, "to": {schema.SealStatus}}},
		{"missing to", url.Values{"path": {"a.md"}, "from": {"draft"}}},
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
	srv := newHandlerServer(t, newWriter(t, t.TempDir(), nil))
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
	root := t.TempDir()
	writer := newWriter(t, root, nil) // no contract: fail-closed
	srv := newHandlerServer(t, writer)

	code, _, body := postStatus(t, srv, url.Values{"path": {"a.md"}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {wellFormedIdentity}})
	if code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(body, "無法使用") || !strings.Contains(body, "已關閉") {
		t.Errorf("body = %q, want the Traditional Chinese unavailable-and-closed explanation", body)
	}
}

func TestHandlerPathValidationPrecedesClosure(t *testing.T) {
	t.Parallel()
	srv := newHandlerServer(t, newWriter(t, t.TempDir(), nil))
	code, _, body := postStatus(t, srv, url.Values{
		"path":             {"../outside.md"},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {wellFormedIdentity},
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
	root := t.TempDir()
	srv := newHandlerServer(t, newWriter(t, root, loadContract(t)))

	code, _, body := postStatus(t, srv, url.Values{
		"path":             {"System/templates/Missing.md"},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {wellFormedIdentity},
	})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("POST non-instance status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(body, wording.NonInstanceReason.In(wording.ZhHant)) {
		t.Errorf("POST non-instance body = %q, want the governed-instance explanation", body)
	}
}

// TestHandlerRefusesANoteOutsideTheKnowledgeLayer holds the write face to the
// layer the contract declares: a note whose bytes, identity and transition are
// in every other way acceptable is refused unchanged when it sits outside
// scan.knowledge_dirs — on the same page and with the same code as a template,
// under its own sentence rather than the template's.
func TestHandlerRefusesANoteOutsideTheKnowledgeLayer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	srv := newHandlerServer(t, newWriter(t, root, loadContract(t)))

	const outside = "System/agent-guides/L05.md"
	body := lessonContent("draft")
	writeVaultFile(t, root, outside, body)

	code, _, page := postStatus(t, srv, url.Values{
		"path":             {outside},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {formIdentity(body)},
	})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(page, wording.OutsideKnowledgeScope.In(wording.ZhHant)) {
		t.Errorf("body = %q, want the knowledge-layer explanation", page)
	}
	if strings.Contains(page, wording.NonInstanceReason.In(wording.ZhHant)) {
		t.Errorf("body = %q, states the template sentence for a note the layer refused", page)
	}
	assertVaultFileUnchanged(t, root, outside, body)
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
			writer := newWriter(t, t.TempDir(), tt.contract)
			srv := newHandlerServer(t, writer)
			code, _, body := postStatus(t, srv, url.Values{
				"path":             {testRel},
				"from":             {"draft"},
				"to":               {schema.SealStatus},
				"content_identity": {wellFormedIdentity},
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

	srv := newHandlerServer(t, newWriter(t, t.TempDir(), contract))
	code, _, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {wellFormedIdentity},
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
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	srv := newHandlerServer(t, writer)

	// The page claims "imported"; the file actually says "draft".
	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"imported"}, "to": {schema.SealStatus}, "content_identity": {formIdentity(lessonContent("draft"))}})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, "頁面已過期") || !strings.Contains(body, "重新載入") {
		t.Errorf("body = %q, want the Traditional Chinese stale-page recovery", body)
	}
	// The repair a recovery page asks for is a hand edit, so a refusal that
	// knows which note it was about links straight to it in Obsidian.
	if !strings.Contains(body, wording.OpenInObsidian.In(wording.ZhHant)) || !strings.Contains(body, `href="obsidian://open?path=`) {
		t.Errorf("body = %q, want an Obsidian editor link on the recovery page", body)
	}
	if !strings.Contains(body, testRel) {
		t.Errorf("body = %q, want the Obsidian href to name %q", body, testRel)
	}
}

func TestHandlerTargetRemovedAfterPageLoad(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(testRel))); err != nil {
		t.Fatalf("remove status target: %v", err)
	}
	srv := newHandlerServer(t, writer)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {wellFormedIdentity}})
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

// TestHandlerUnsupportedStatusSyntax locks the operator-facing wording for a
// readable note whose status the surgical rewriter cannot locate: the page
// must name the unsupported syntax and the plain form to switch to, never
// the schema-violation message — the note violates no schema.
func TestHandlerUnsupportedStatusSyntax(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	content := "---\n" +
		"title: L05\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"'status': draft\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-01\n" +
		"---\n" +
		"\nbody\n"
	writeNote(t, root, content)
	srv := newHandlerServer(t, writer)

	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {formIdentity(content)}})
	if code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	for _, want := range []string{"不支援的 YAML 寫法", "status: 值"} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q: %q", want, body)
		}
	}
	if strings.Contains(body, "違反 schema") {
		t.Errorf("body still claims a schema violation for a readable status: %q", body)
	}
}

// TestHandlerSymlinkTargetIsRefusedAsUnprocessable locks the response for a
// path the write face refuses to follow. The refusal itself is pinned at the
// Writer level (nothing on either side of a symlink changes); what this test
// holds is what the operator is told: the same 422 every other refused target
// gets, with the reason stated, never a 500 that sends them to the log for a
// condition the page can name.
func TestHandlerSymlinkTargetIsRefusedAsUnprocessable(t *testing.T) {
	t.Parallel()

	original := lessonContent("draft")
	tests := []struct {
		name string
		// plant creates the symlink shape under root and returns the path of
		// the real file that must stay untouched.
		plant func(t *testing.T, root string) string
		// wantSummary is the sentence for where the shape broke. The note's
		// own entry and a step on the way to it are repaired at different
		// places, so the two refusals name their place rather than sharing
		// one sentence the operator has to disambiguate by hand.
		wantSummary wording.Phrase
	}{
		{
			name:        "the note itself is a symlink",
			wantSummary: wording.TargetNotRegular,
			plant: func(t *testing.T, root string) string {
				t.Helper()
				outside := filepath.Join(t.TempDir(), "outside.md")
				if err := os.WriteFile(outside, []byte(original), 0o600); err != nil {
					t.Fatalf("write outside target: %v", err)
				}
				notePath := filepath.Join(root, filepath.FromSlash(testRel))
				if err := os.MkdirAll(filepath.Dir(notePath), 0o750); err != nil {
					t.Fatalf("mkdir note parent: %v", err)
				}
				if err := os.Symlink(outside, notePath); err != nil {
					t.Fatalf("symlink note: %v", err)
				}
				return outside
			},
		},
		{
			name:        "a directory on the way is a symlink",
			wantSummary: wording.PathNotRegular,
			plant: func(t *testing.T, root string) string {
				t.Helper()
				outsideWriting := filepath.Join(t.TempDir(), "Writing")
				notePath := filepath.Join(outsideWriting, "lessons", "japanese", "L05.md")
				if err := os.MkdirAll(filepath.Dir(notePath), 0o750); err != nil {
					t.Fatalf("mkdir outside note parent: %v", err)
				}
				if err := os.WriteFile(notePath, []byte(original), 0o600); err != nil {
					t.Fatalf("write outside target: %v", err)
				}
				if err := os.Symlink(outsideWriting, filepath.Join(root, "Writing")); err != nil {
					t.Fatalf("symlink Writing: %v", err)
				}
				return notePath
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writer := newWriter(t, root, loadContract(t))
			target := tt.plant(t, root)
			srv := newHandlerServer(t, writer)

			code, _, body := postStatus(t, srv, url.Values{
				"path":             {testRel},
				"from":             {"draft"},
				"to":               {schema.SealStatus},
				"content_identity": {formIdentity(original)},
			})
			if code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
			}
			for _, want := range []string{
				tt.wantSummary.In(wording.ZhHant),
				"狀態尚未變更",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("body is missing %q: %q", want, body)
				}
			}
			// The old shape for this refusal was the catch-all failure page,
			// which sends the operator to the log for a condition the page can
			// state itself.
			if strings.Contains(body, wording.WriteFailedNext.In(wording.ZhHant)) {
				t.Errorf("body still sends the operator to the log: %q", body)
			}
			got, readErr := os.ReadFile(target) // #nosec G304 -- target is a test-owned path under t.TempDir
			if readErr != nil {
				t.Fatalf("read the file behind the link: %v", readErr)
			}
			if got := string(got); got != original {
				t.Errorf("the file behind the link changed:\ngot:  %q\nwant: %q", got, original)
			}
		})
	}
}

func TestHandlerIllegalTransition(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	writeNote(t, root, lessonContent("imported"))
	srv := newHandlerServer(t, writer)

	// imported -> ready skips the required "draft" stage.
	code, _, body := postStatus(t, srv, url.Values{"path": {testRel}, "from": {"imported"}, "to": {schema.SealStatus}, "content_identity": {formIdentity(lessonContent("imported"))}})
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
	// The repair for a schema refusal is a hand edit, and the page already
	// carries the door to where editing happens. The way on must point at
	// that door by its own name rather than asking the reader to correct
	// something this interface offers no control for.
	if !strings.Contains(decoded, "「在 Obsidian 開啟」") {
		t.Errorf("the way on does not name the editor door the page already carries: %q", body)
	}
}

// TestAFlipNobodyIsWaitingForIsNotReportedAsAFailedWrite closes a hole the
// cancellable write face opened. A reader who double-presses the status form
// makes the browser drop the first request, and that request's flip now comes
// back as a cancellation with the note untouched. Sent down the ordinary
// refusal path it would render a recovery page into a closed connection and
// leave "status flip failed" in the operator's log for a write that never
// started — a fault where there was none, which is the one thing a diagnostic
// must never be.
func TestAFlipNobodyIsWaitingForIsNotReportedAsAFailedWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))

	var reported strings.Builder
	mux := http.NewServeMux()
	status.NewHandler(writer, func() nav.Shell { return nav.Shell{} },
		slog.New(slog.NewTextHandler(&reported, nil))).Register(mux)

	form := url.Values{
		"path":             {"Writing/lessons/japanese/L05.md"},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {formIdentity(lessonContent("draft"))},
	}
	gone, cancel := context.WithCancel(t.Context())
	cancel()
	req := httptest.NewRequestWithContext(gone, http.MethodPost, "/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if body := recorder.Body.String(); body != "" {
		t.Errorf("a page was written for a request that had gone away: %q", body)
	}
	if log := reported.String(); strings.Contains(log, "status flip failed") {
		t.Errorf("a flip that never started was logged as a failed write: %q", log)
	}
	// The control on the whole test: the note is exactly as it was, so the
	// silence above is a refusal and not a completed write nobody described.
	onDisk, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("Writing/lessons/japanese/L05.md"))) // #nosec G304 -- a fixed path under this test's TempDir
	if err != nil {
		t.Fatalf("read the note back: %v", err)
	}
	if got, want := string(onDisk), lessonContent("draft"); got != want {
		t.Errorf("the note changed under a cancelled request:\n got %q\nwant %q", got, want)
	}
}
