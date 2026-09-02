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
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
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
	req := siteRequest(t, http.MethodPost, "/status", strings.NewReader(form.Encode()))
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
	source, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	contract, err := schema.LoadReader(t.Context(), source)
	if err != nil {
		t.Fatalf("schema.LoadReader() error = %v", err)
	}
	writer, err := status.Open(source, contract, contract.Governance(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("status.Open() error = %v", err)
	}
	watchCtx, cancel := context.WithCancel(t.Context())
	site := &readingSite{
		writer: writer,
		source: source,
		cancel: cancel,
	}
	watcherResult := make(chan error, 1)
	site.watchers.Go(func() {
		<-watchCtx.Done()
		if writer.Authority().Closed() {
			watcherResult <- errors.New("writer closed before watcher exited")
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
	if !writer.Authority().Closed() {
		t.Error("Writer remained open after readingSite.close()")
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
	source, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	contract, err := schema.LoadReader(t.Context(), source)
	if err != nil {
		t.Fatalf("schema.LoadReader() error = %v", err)
	}
	writer, err := status.Open(source, contract, contract.Governance(), slog.New(slog.DiscardHandler))
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
			if writer.Authority().Closed() {
				requestResult <- errors.New("writer closed before active request exited")
				return
			}
			if _, scanErr := source.ScanComplete(context.WithoutCancel(t.Context())); scanErr != nil {
				requestResult <- errors.New("vault reader closed before active request exited")
				return
			}
			requestResult <- nil
			w.WriteHeader(http.StatusNoContent)
		}),
		writer: writer,
		source: source,
	}
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
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
	site.ServeHTTP(rejected, siteRequest(t, http.MethodGet, "/", http.NoBody))
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
	if !writer.Authority().Closed() {
		t.Error("Writer remained open after readingSite.close()")
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
		wantFault bool   // the page carries a contract-fault block
		wantCause string // decoder text the page must reproduce
	}{
		{name: "no contract", contract: "", wantLevel: "INFO"},
		{
			name:      "unparseable contract",
			contract:  "this is not toml [[[\n",
			wantLevel: "WARN",
			wantFault: true,
			wantCause: "toml:",
		},
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

			// The log is the half nobody reads. Which of the two folders this
			// is has to survive into the page, and only this wiring decides
			// it: everything downstream is handed the answer. Driving the
			// production composition is the point — a handler assembled by a
			// test would carry whatever the test chose to say.
			rec := httptest.NewRecorder()
			site.ServeHTTP(rec, siteRequest(t, http.MethodGet, "/", http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
			}
			page := rec.Body.String()
			if gotFault := strings.Contains(page, `data-home-block="faults"`); gotFault != tt.wantFault {
				t.Errorf("Home states a contract fault = %t, want %t", gotFault, tt.wantFault)
			}
			if tt.wantCause != "" && !strings.Contains(page, tt.wantCause) {
				t.Errorf("the page never reproduces the decoder's own words %q; the operator is left with the log", tt.wantCause)
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

// siteRequest builds a request the way a browser on this machine builds one:
// addressed to loopback by name. The site refuses any other name before a
// handler runs, so a request that omits it is not the request a reader makes.
func siteRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, body)
	req.Host = "127.0.0.1:9610"
	return req
}

// TestTheSiteRefusesACrossSiteWrite locks the cross-origin middleware into the
// served assembly. The status write is the one state-changing endpoint, and a
// hostile page in the reader's own browser is the one party that can drive a
// POST at the loopback listener from off this machine; the browser labels such
// a request with Sec-Fetch-Site, and the assembly has to turn it away before
// any handler runs. Everything else that watches this boundary watches
// response headers, socket binding, or the Host name — none of them sends a
// hostile cross-site request expecting the refusal, so unwiring the middleware
// left every test green. This test is that lock: it drives the production
// composition, not the middleware in isolation.
func TestTheSiteRefusesACrossSiteWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	notePath := filepath.Join(root, "Maps", "study.md")
	before, err := os.ReadFile(notePath) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatalf("read fixture note: %v", err)
	}
	site, err := newReadingSite(t.Context(), root, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	postStatus := func(t *testing.T, fetchSite string) int {
		t.Helper()
		form := url.Values{"path": {"Maps/study.md"}, "from": {"draft"}}
		req := siteRequest(t, http.MethodPost, "/status", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if fetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, req)
		return recorder.Code
	}

	// A browser-labeled cross-site write is refused outright. The note's bytes
	// are re-read afterwards as well, but only the 403 carries the assertion:
	// this form names no target status, so the handler behind the middleware
	// would decline it too and no admitted request could have written anything.
	if got := postStatus(t, "cross-site"); got != http.StatusForbidden {
		t.Errorf("cross-site POST /status = %d, want %d", got, http.StatusForbidden)
	}
	after, err := os.ReadFile(notePath) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatalf("re-read fixture note: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("cross-site POST changed the note: before %q, after %q", before, after)
	}

	// A same-origin browser write is admitted. 422 is the status handler's own
	// recovery answer for this form, deeper in the stack than the middleware;
	// any non-403 would prove admission, and the exact code is pinned by the
	// production-recovery test above.
	if got := postStatus(t, "same-origin"); got != http.StatusUnprocessableEntity {
		t.Errorf("same-origin POST /status = %d, want %d from the status handler, never %d",
			got, http.StatusUnprocessableEntity, http.StatusForbidden)
	}

	// A request with no Sec-Fetch-Site and no Origin is a non-browser caller
	// on this machine — curl, the operator's own tooling. The middleware
	// deliberately admits it: the boundary defends against browser-carried
	// cross-site requests, which always label themselves.
	if got := postStatus(t, ""); got != http.StatusUnprocessableEntity {
		t.Errorf("headerless POST /status = %d, want %d from the status handler", got, http.StatusUnprocessableEntity)
	}
}

// TestTheSiteRefusesARequestAddressedElsewhere asserts the assembled site — not
// the middleware in isolation — turns away a request whose Host names anything
// but this machine.
//
// Binding the socket to the loopback address is not the whole of that promise.
// A name that resolves to 127.0.0.1 reaches a loopback socket from any browser
// that can be made to ask for it, so the served handler has to read the name it
// was addressed by and refuse the ones that are not this machine. The refusal
// lives in one line of the assembly, and everything that watched it watched the
// middleware rather than the assembly: removing that line left the middleware,
// its own tests, and the socket-binding check all green while the vault
// answered to any name at all.
func TestTheSiteRefusesARequestAddressedElsewhere(t *testing.T) {
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

	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "this machine by address", host: "127.0.0.1:9610", want: http.StatusOK},
		{name: "this machine by name", host: "localhost:9610", want: http.StatusOK},
		{name: "this machine over the sixth version", host: "[::1]:9610", want: http.StatusOK},
		{name: "a name that resolves back here", host: "127.0.0.1.nip.io:9610", want: http.StatusForbidden},
		{name: "somewhere else entirely", host: "evil.example", want: http.StatusForbidden},
		{name: "a name wearing this one as a prefix", host: "localhost.evil.example:9610", want: http.StatusForbidden},
		{name: "an address on the machine's own network", host: "192.168.1.9:9610", want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			req.Host = tt.host
			recorder := httptest.NewRecorder()
			site.ServeHTTP(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("GET / addressed to %q = %d, want %d", tt.host, recorder.Code, tt.want)
			}
		})
	}
}

// TestTheStoppingRefusalSpeaksTheReadersLanguage covers the one sentence the
// composition writes for a person. It used to be an English literal, so a
// reader working in the other language met it untranslated — on the one
// response that has to explain itself, since nothing else on screen says why
// the page stopped answering.
func TestTheStoppingRefusalSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cookie string
		want   wording.Lang
	}{
		{name: "no choice means the default", want: wording.ZhHant},
		{name: "a reader who chose English", cookie: string(wording.En), want: wording.En},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			site := &readingSite{
				handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					t.Error("a request arriving after close began reached the handler")
				}),
				closing: true,
			}
			request := siteRequest(t, http.MethodGet, "/", http.NoBody)
			if tt.cookie != "" {
				request.Header.Set("Cookie", wording.CookieName+"="+tt.cookie)
			}
			response := httptest.NewRecorder()
			site.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
			want := wording.ServerStopping.In(tt.want)
			if got := strings.TrimSpace(response.Body.String()); got != want {
				t.Errorf("refusal = %q, want %q", got, want)
			}
		})
	}
}
