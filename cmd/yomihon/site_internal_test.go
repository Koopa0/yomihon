package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestProductionStatusFailureUsesTheReadingShell(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})
	form := url.Values{"path": {"Maps/study.md"}, "from": {"draft"}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, req)

	response := recorder.Result()
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST /status = %d, want %d", response.StatusCode, http.StatusUnprocessableEntity)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	html := string(body)
	for _, want := range []string{
		"y-brand",
		"搜尋筆記",
		"狀態尚未變更",
		`href="/notes/Maps/study.md"`,
		`data-sidebar-group="paths"`,
		"Study Path",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("production recovery page is missing %q; body = %q", want, html)
		}
	}
	if strings.Contains(html, `action="/status"`) {
		t.Errorf("production recovery page offers a POST retry; body = %q", html)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestReadingSiteCloseWaitsForWatcherBeforeClosingCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	contract, err := schema.LoadReader(t.Context(), source)
	if err != nil {
		t.Fatalf("schema.LoadReader() error = %v", err)
	}
	lifecycle, err := status.Open(source, contract)
	if err != nil {
		t.Fatalf("status.Open() error = %v", err)
	}
	watchCtx, cancel := context.WithCancel(t.Context())
	site := &readingSite{
		lifecycle: lifecycle,
		source:    source,
		cancel:    cancel,
	}
	watcherResult := make(chan error, 1)
	site.watchers.Go(func() {
		<-watchCtx.Done()
		if lifecycle.View().Closed() {
			watcherResult <- errors.New("lifecycle closed before watcher exited")
			return
		}
		if _, scanErr := source.ScanComplete(context.WithoutCancel(t.Context())); scanErr != nil {
			watcherResult <- errors.New("vault reader closed before watcher exited")
			return
		}
		watcherResult <- nil
	})

	if err = site.close(); err != nil {
		t.Fatalf("readingSite.close() error = %v", err)
	}
	if err = <-watcherResult; err != nil {
		t.Fatal(err)
	}
	if !lifecycle.View().Closed() {
		t.Error("Lifecycle remained open after readingSite.close()")
	}
	if _, err = source.ScanComplete(t.Context()); err == nil {
		t.Error("Reader.ScanComplete() after readingSite.close() = nil, want closed-reader error")
	}
	if err = site.close(); err != nil {
		t.Errorf("second readingSite.close() error = %v, want nil", err)
	}
}

func TestReadingSiteCloseWaitsForActiveRequestsBeforeClosingCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	contract, err := schema.LoadReader(t.Context(), source)
	if err != nil {
		t.Fatalf("schema.LoadReader() error = %v", err)
	}
	lifecycle, err := status.Open(source, contract)
	if err != nil {
		t.Fatalf("status.Open() error = %v", err)
	}
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	requestResult := make(chan error, 1)
	site := &readingSite{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(requestStarted)
			<-releaseRequest
			if lifecycle.View().Closed() {
				requestResult <- errors.New("lifecycle closed before active request exited")
				return
			}
			if _, scanErr := source.ScanComplete(context.WithoutCancel(t.Context())); scanErr != nil {
				requestResult <- errors.New("vault reader closed before active request exited")
				return
			}
			requestResult <- nil
			w.WriteHeader(http.StatusNoContent)
		}),
		lifecycle: lifecycle,
		source:    source,
	}
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	}()
	<-requestStarted

	closeResult := make(chan error, 1)
	go func() { closeResult <- site.close() }()

	// Wait for close to shut the admission gate before exercising its rejected
	// request path. Reading the gate under its production mutex avoids sleeps
	// and avoids admitting a second blocking request.
	for {
		site.requestMu.Lock()
		closing := site.closing
		site.requestMu.Unlock()
		if closing {
			break
		}
		runtime.Gosched()
	}
	rejected := httptest.NewRecorder()
	site.ServeHTTP(rejected, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	if rejected.Code != http.StatusServiceUnavailable {
		t.Fatalf("request after close began = %d, want %d", rejected.Code, http.StatusServiceUnavailable)
	}
	select {
	case err = <-closeResult:
		t.Fatalf("readingSite.close() returned %v before active request exited", err)
	default:
	}

	close(releaseRequest)
	if err = <-requestResult; err != nil {
		t.Fatal(err)
	}
	<-requestDone
	if err = <-closeResult; err != nil {
		t.Fatalf("readingSite.close() error = %v", err)
	}
	if !lifecycle.View().Closed() {
		t.Error("Lifecycle remained open after readingSite.close()")
	}
	if _, err = source.ScanComplete(t.Context()); err == nil {
		t.Error("Reader.ScanComplete() after readingSite.close() = nil, want closed-reader error")
	}
}

func writeRecoverySiteFixture(t *testing.T, root string) {
	t.Helper()
	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err = os.MkdirAll(filepath.Dir(contractPath), 0o750); err != nil {
		t.Fatalf("mkdir schema directory: %v", err)
	}
	if err = os.WriteFile(contractPath, contract, 0o600); err != nil { // #nosec G703 -- fixed contract path under t.TempDir
		t.Fatalf("write schema fixture: %v", err)
	}
	mapPath := filepath.Join(root, "Maps", "study.md")
	if err = os.MkdirAll(filepath.Dir(mapPath), 0o750); err != nil {
		t.Fatalf("mkdir map directory: %v", err)
	}
	const studyPath = "---\ntitle: Study Path\ntype: study-path\n---\n\n# Study Path\n"
	if err = os.WriteFile(mapPath, []byte(studyPath), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
		t.Fatalf("write study path: %v", err)
	}
}

// TestContractAbsenceIsNotReportedAsAFault locks the difference between a
// folder that never declared a lifecycle and one whose declaration is broken.
// Both used to arrive here as the same warning, and the absent case additionally
// arrived as "vault source changed while it was being read" — a race nobody ran.
// Reporting an ordinary folder as a fault is what made a working tool look
// broken to a first-time user; silencing a genuinely broken contract would be
// the opposite and worse mistake, so both directions are asserted.
func TestContractAbsenceIsNotReportedAsAFault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		contract  string // empty means write no contract at all
		wantLevel string
	}{
		{name: "no contract", contract: "", wantLevel: "INFO"},
		{name: "unparseable contract", contract: "this is not toml [[[\n", wantLevel: "WARN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "Writing", "note.md"), []byte("# note\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			if tt.contract != "" {
				dir := filepath.Join(root, "System", "schemas")
				if err := os.MkdirAll(dir, 0o750); err != nil {
					t.Fatalf("mkdir schemas: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "vault-schema.toml"), []byte(tt.contract), 0o600); err != nil {
					t.Fatalf("write contract: %v", err)
				}
			}

			var logged lockedBuffer
			site, err := newReadingSite(t.Context(), root, slog.New(slog.NewTextHandler(&logged, nil)))
			if err != nil {
				t.Fatalf("newReadingSite: %v", err)
			}
			t.Cleanup(func() {
				if closeErr := site.close(); closeErr != nil {
					t.Errorf("readingSite.close() error = %v", closeErr)
				}
			})

			lines := logged.lines()
			got, ok := levelFor(lines, "contract")
			if !ok {
				t.Fatalf("startup logged nothing about the contract; log = %v", lines)
			}
			if got != tt.wantLevel {
				t.Errorf("contract log level = %s, want %s (log = %v)", got, tt.wantLevel, lines)
			}
		})
	}
}

// lockedBuffer collects the real handler's output. Asserting the bytes the
// operator would actually read is closer to the thing under test than
// intercepting records, and it needs no slog.Handler of our own.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Split(strings.TrimSpace(b.buf.String()), "\n")
}

// levelFor returns the level of the first logged line mentioning substring.
func levelFor(lines []string, substring string) (string, bool) {
	for _, line := range lines {
		if !strings.Contains(line, substring) {
			continue
		}
		if _, rest, ok := strings.Cut(line, "level="); ok {
			level, _, _ := strings.Cut(rest, " ")
			return level, true
		}
	}
	return "", false
}
