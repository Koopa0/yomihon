package main

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestReadingSiteCloseWaitsForWatcherBeforeClosingCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	watchCtx, cancel := context.WithCancel(t.Context())
	site := &readingSite{
		source: source,
		cancel: cancel,
	}
	watcherResult := make(chan error, 1)
	site.watchers.Go(func() {
		<-watchCtx.Done()
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
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	requestResult := make(chan error, 1)
	site := &readingSite{
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(requestStarted)
			<-releaseRequest
			if _, scanErr := source.ScanComplete(context.WithoutCancel(t.Context())); scanErr != nil {
				requestResult <- errors.New("vault reader closed before active request exited")
				return
			}
			requestResult <- nil
			w.WriteHeader(http.StatusNoContent)
		}),
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
	const studyPath = "---\ntype: study-path\n---\n\n# Study Path\n"
	if err = os.WriteFile(mapPath, []byte(studyPath), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
		t.Fatalf("write study path: %v", err)
	}
}

// TestARulesFileForAnotherVersionIsReadAsNoRulesFile holds the two halves of
// that state together through the production composition: the folder reads as
// one without a contract, and the desk says which line of the file to change,
// in the language the browser asked for.
func TestARulesFileForAnotherVersionIsReadAsNoRulesFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for path, content := range map[string]string{
		"Writing/note.md": "# A note\n\nBody.\n",
		schema.ContractRelPath: "# rules\n" +
			"schema_version = \"1\"\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
			t.Fatalf("write %s: %v", path, err)
		}
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	home := httptest.NewRecorder()
	request := siteRequest(t, http.MethodGet, "/", http.NoBody)
	request.Header.Set("Accept-Language", "en")
	site.ServeHTTP(home, request)
	if home.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", home.Code, http.StatusOK)
	}
	body := home.Body.String()
	for _, want := range []string{`<html lang="en"`, "Change line 2 to schema_version"} {
		if !strings.Contains(body, want) {
			t.Errorf("the desk does not carry %q", want)
		}
	}
	for _, unwanted := range []string{wording.RestartYomihon.In(wording.En), "could not be read"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("the desk carries %q, which says this folder is broken or needs a restart", unwanted)
		}
	}

	note := httptest.NewRecorder()
	site.ServeHTTP(note, siteRequest(t, http.MethodGet, "/notes/Writing/note.md", http.NoBody))
	if note.Code != http.StatusOK || !strings.Contains(note.Body.String(), "Body.") {
		t.Errorf("GET a note = %d, want %d with its body: reading must work as it does with no rules file", note.Code, http.StatusOK)
	}
}

// TestTheDeskNamesTheFolderByItsReadmeAndSaysWhereItIs holds the wiring from
// the folder that was opened to the line at the foot of the desk: the name is
// the README's first heading, and the path is where the folder really is.
func TestTheDeskNamesTheFolderByItsReadmeAndSaysWhereItIs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Field Notes\n\nWords.\n"), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
		t.Fatalf("write README: %v", err)
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", root, err)
	}

	home := httptest.NewRecorder()
	site.ServeHTTP(home, siteRequest(t, http.MethodGet, "/", http.NoBody))
	if home.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", home.Code, http.StatusOK)
	}
	_, line, ok := strings.Cut(home.Body.String(), "data-home-vault")
	if !ok {
		t.Fatal("the desk does not say which folder it is of")
	}
	for _, want := range []string{"<span>Field Notes</span>", "<code>" + resolved + "</code>"} {
		if !strings.Contains(line, want) {
			t.Errorf("the desk line does not carry %q", want)
		}
	}
}

// TestEveryPageHeaderNamesTheFolderByItsReadme holds the header's folder name
// to one source for every page it is drawn on: pages answered by four different
// handlers, and one that is not found, all say the README's first heading.
func TestEveryPageHeaderNamesTheFolderByItsReadme(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Field Notes\n\nWords.\n"), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
		t.Fatalf("write README: %v", err)
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	const want = `<a class="y-brand__vault" href="/">Field Notes</a>`
	for _, path := range []string{"/", "/preferences", "/search", "/health", "/notes/missing.md"} {
		rec := httptest.NewRecorder()
		site.ServeHTTP(rec, siteRequest(t, http.MethodGet, path, http.NoBody))
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("GET %s (status %d): the header does not carry %q", path, rec.Code, want)
		}
	}
}

// TestContractAbsenceIsNotReportedAsAFault locks the difference between a
// folder that never declared a contract and one whose declaration is broken.
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
			site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.NewTextHandler(&logged, nil)))
			if err != nil {
				t.Fatalf("newReadingSite: %v", err)
			}
			t.Cleanup(func() {
				if closeErr := site.close(); closeErr != nil {
					t.Errorf("readingSite.close() error = %v", closeErr)
				}
			})

			lines := logged.lines()
			got, ok := levelFor(lines, "rules file")
			if !ok {
				t.Fatalf("startup logged nothing about the rules file; log = %v", lines)
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

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
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
// served assembly. The routes a reader keeps a reading place and a question
// mark through change state outside the vault, and a hostile page in the reader's own browser is the one party that can drive a
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
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	// The write endpoint is behind the wrapper rather than a guard of its own,
	// which is exactly why it is driven here: the protection is a property of
	// the assembly, and nothing in the mark package's own tests would notice
	// it being unwired.
	postMark := func(t *testing.T, fetchSite string) int {
		t.Helper()
		form := url.Values{"path": {"Maps/study.md"}, "offset": {"0"}, "identity": {strings.Repeat("a", 64)}}
		req := siteRequest(t, http.MethodPost, "/bookmark", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if fetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, req)
		return recorder.Code
	}
	if got := postMark(t, "cross-site"); got != http.StatusForbidden {
		t.Errorf("cross-site POST /marks = %d, want %d", got, http.StatusForbidden)
	}
	// And admitted from the reader's own page, so the refusal above is the
	// middleware rather than the route being missing or broken.
	if got := postMark(t, "same-origin"); got != http.StatusNoContent {
		t.Errorf("same-origin POST /marks = %d, want %d, never %d",
			got, http.StatusNoContent, http.StatusForbidden)
	}

	// A request with no Sec-Fetch-Site and no Origin is a non-browser caller
	// on this machine — curl, the operator's own tooling. The middleware
	// deliberately admits it: the boundary defends against browser-carried
	// cross-site requests, which always label themselves.
	if got := postMark(t, ""); got != http.StatusNoContent {
		t.Errorf("headerless POST /marks = %d, want %d", got, http.StatusNoContent)
	}
}

// TestTheSiteRefusesAMarkWriteFromAnotherSiteOrAnotherName holds the routes
// that keep, change, remove and put back a mark to the two protections the
// bookmark route has, through the assembly: a request a browser labels
// cross-site is refused before a handler runs, and so is one addressed to
// a name that is not this machine. Nothing is written for either, and the
// same four requests from the reader's own page go through.
func TestTheSiteRefusesAMarkWriteFromAnotherSiteOrAnotherName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "Plain.md"), []byte("# Plain\n\nThe topic particle marks what the sentence is about.\n"), 0o600); err != nil {
		t.Fatalf("write a note with words in it: %v", err)
	}
	configDir := t.TempDir()
	site, err := newReadingSite(t.Context(), root, configDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	id := strings.Repeat("a", 32)
	thought := `{"kind":"thought","path":"Maps/study.md","body":"a thought"}`
	put := `{"id":"` + id + `","kind":"thought","path":"Maps/study.md","body":"a thought","at":"2026-09-22T11:00:00Z"}`
	writes := []struct{ method, target, body string }{
		{http.MethodPost, "/mark", thought},
		{http.MethodPut, "/mark/" + id, put},
		{http.MethodPatch, "/mark/" + id, `{"body":"changed"}`},
		{http.MethodDelete, "/mark/" + id, ""},
	}
	send := func(method, target, body, fetchSite, host string) *httptest.ResponseRecorder {
		t.Helper()
		req := siteRequest(t, method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if fetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		if host != "" {
			req.Host = host
		}
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, req)
		return recorder
	}

	for _, w := range writes {
		if got := send(w.method, w.target, w.body, "cross-site", "").Code; got != http.StatusForbidden {
			t.Errorf("cross-site %s %s = %d, want %d", w.method, w.target, got, http.StatusForbidden)
		}
		if got := send(w.method, w.target, w.body, "same-origin", "evil.example").Code; got != http.StatusForbidden {
			t.Errorf("%s %s addressed to another name = %d, want %d", w.method, w.target, got, http.StatusForbidden)
		}
	}
	if entries, readErr := os.ReadDir(configDir); readErr != nil || len(entries) != 0 {
		t.Fatalf("the refused requests left %d entries in the configuration directory, %v; want nothing written", len(entries), readErr)
	}

	// From the reader's own page the same requests are admitted, so the
	// refusals above are the protections and not routes that are missing.
	for _, tt := range []struct {
		method, target, body string
		want                 int
	}{
		{http.MethodPut, "/mark/" + id, put, http.StatusOK},
		{http.MethodPatch, "/mark/" + id, `{"body":"changed"}`, http.StatusOK},
		{http.MethodDelete, "/mark/" + id, "", http.StatusOK},
		{http.MethodPost, "/mark", thought, http.StatusCreated},
		// Words the note shows are kept, through the reading room's own text of the page.
		{http.MethodPost, "/mark", `{"kind":"highlight","path":"Plain.md","quote":"topic particle marks"}`, http.StatusCreated},
		{http.MethodPost, "/mark", `{"kind":"highlight","path":"Plain.md","quote":"words the note never had"}`, http.StatusUnprocessableEntity},
	} {
		if got := send(tt.method, tt.target, tt.body, "same-origin", "").Code; got != tt.want {
			t.Errorf("same-origin %s %s %s = %d, want %d", tt.method, tt.target, tt.body, got, tt.want)
		}
	}
}

// TestTheSiteKeepsNoPlaceInANoteItDoesNotHold drives the production assembly:
// the route that keeps a reading place is handed the reading room's own answer
// to whether a note is there. On a shared instance this route is open to anyone
// who can reach it and its path is printed on Home, so a path that names no
// note — a sentence, say — must be refused with the question route's own
// status instead of being stored for the desk to show.
func TestTheSiteKeepsNoPlaceInANoteItDoesNotHold(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	serve := func(method, target string, body io.Reader) *httptest.ResponseRecorder {
		req := siteRequest(t, method, target, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, req)
		return recorder
	}
	keep := func(path string) int {
		form := url.Values{"path": {path}, "offset": {"0"}, "identity": {strings.Repeat("a", 64)}}
		return serve(http.MethodPost, "/bookmark", strings.NewReader(form.Encode())).Code
	}
	kept := func() bool {
		home := serve(http.MethodGet, "/", http.NoBody)
		if home.Code != http.StatusOK {
			t.Fatalf("GET / = %d, want 200", home.Code)
		}
		return strings.Contains(home.Body.String(), "data-home-continue")
	}

	for _, path := range []string{"Maps/nope.md", "Visit spam.example for free prizes", "Maps/a\x00b\nc.md"} {
		if got := keep(path); got != http.StatusUnprocessableEntity {
			t.Errorf("POST /marks over %q = %d, want %d", path, got, http.StatusUnprocessableEntity)
		}
	}
	if kept() {
		t.Error("Home offers a way back to a place after every path that names no note was refused")
	}
	if got := keep("Maps/study.md"); got != http.StatusNoContent {
		t.Errorf("POST /marks over a note the vault holds = %d, want %d", got, http.StatusNoContent)
	}
	if !kept() {
		t.Error("Home offers no way back to the place that was kept")
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
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
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
			assertPermissionsPolicy(t, recorder.Result().Header)
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
			assertPermissionsPolicy(t, response.Result().Header)

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

func assertPermissionsPolicy(t *testing.T, header http.Header) {
	t.Helper()
	const want = "camera=(), microphone=(), geolocation=(), usb=(), serial=(), payment=(), display-capture=()"
	t.Log("invoked: production response permissions policy")
	if got := header.Get("Permissions-Policy"); got != want {
		t.Errorf("caught: production Permissions-Policy = %q, want %q", got, want)
	}
}

// TestEveryPageTitlesItsTabWithTheVaultsName holds the wiring from the folder
// that was opened to the end of every tab's title: what the page is, then what
// the vault is called, so two vaults open side by side are told apart on the
// tab strip. The desk and the vault's own README say the name once, and no page
// carries the product's name.
func TestEveryPageTitlesItsTabWithTheVaultsName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
		t.Fatalf("make folder: %v", err)
	}
	for rel, body := range map[string]string{
		"README.md":          "# Field Notes\n\nWords.\n",
		"Writing/note.md":    "# Go Concurrency\n\nBody.\n",
		"Writing/another.md": "# Channels\n\nBody about concurrency.\n",
	} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	titleOf := func(t *testing.T, language, target string) string {
		t.Helper()
		req := siteRequest(t, http.MethodGet, target, http.NoBody)
		req.Header.Set("Cookie", wording.CookieName+"="+language)
		rec := httptest.NewRecorder()
		site.ServeHTTP(rec, req)
		match := regexp.MustCompile(`<title>(.*?)</title>`).FindStringSubmatch(rec.Body.String())
		if match == nil {
			t.Fatalf("GET %s = %d with no title", target, rec.Code)
		}
		return html.UnescapeString(match[1])
	}
	for _, test := range []struct {
		name, language, target, want string
	}{
		{"the desk", "en", "/", "Field Notes"},
		{"a note", "en", "/notes/Writing/note.md", "Go Concurrency — Field Notes"},
		{"the vault's own README", "en", "/notes/README.md", "Field Notes"},
		{"a search", "en", "/search?q=go++concurrency", "Search: go concurrency — Field Notes"},
		{"a search with nothing typed", "en", "/search", "Search — Field Notes"},
		{"a search in Chinese", "zh-Hant", "/search?q=channels", "搜尋：channels — Field Notes"},
		{"a page that is not there", "en", "/notes/absent.md", wording.NotFoundKicker.In(wording.En) + " — Field Notes"},
		{"a page that is not there, in Chinese", "zh-Hant", "/notes/absent.md", wording.NotFoundKicker.In(wording.ZhHant) + " — Field Notes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := titleOf(t, test.language, test.target)
			if got != test.want {
				t.Errorf("the tab of %s says %q, want %q", test.target, got, test.want)
			}
			if strings.Contains(strings.ToLower(got), "yomihon") {
				t.Errorf("the tab of %s names the product: %q", test.target, got)
			}
		})
	}
}
