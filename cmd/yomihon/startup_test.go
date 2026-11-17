package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/wording"
)

type fakeCount struct {
	done, total int
	folder      string
}

func (f fakeCount) Files() (done, total int) { return f.done, f.total }

func (f fakeCount) Folder() string { return f.folder }

func newTestStartup(t *testing.T, count fakeCount, grace time.Duration) *startup {
	t.Helper()
	return newStartup("notes", "/home/reader/notes", count, grace, slog.New(slog.DiscardHandler))
}

func openedSite(t *testing.T) *readingSite {
	t.Helper()
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
	return site
}

// TestListenLoopbackBindsOnlyLoopback is the listener half of the second
// promise: whatever port is asked for, the socket is on 127.0.0.1.
func TestListenLoopbackBindsOnlyLoopback(t *testing.T) {
	t.Parallel()
	listener, err := listenLoopback(t.Context(), "0")
	if err != nil {
		t.Fatalf("listenLoopback: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Errorf("close listener: %v", closeErr)
		}
	})
	got, err := netip.ParseAddrPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("parse %q: %v", listener.Addr(), err)
	}
	if got.Addr() != netip.MustParseAddr("127.0.0.1") {
		t.Errorf("caught: the listener is bound to %v, want 127.0.0.1 and nothing else", got.Addr())
	}
}

// TestListenFailureNamesTheSettingThatMovesIt covers the port the operator
// cannot have: the error is the one a terminal shows, so it says which setting
// to change.
func TestListenFailureNamesTheSettingThatMovesIt(t *testing.T) {
	t.Parallel()
	taken, err := listenLoopback(t.Context(), "0")
	if err != nil {
		t.Fatalf("listenLoopback: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := taken.Close(); closeErr != nil {
			t.Errorf("close listener: %v", closeErr)
		}
	})
	_, port, err := net.SplitHostPort(taken.Addr().String())
	if err != nil {
		t.Fatalf("split %q: %v", taken.Addr(), err)
	}

	_, err = listenLoopback(t.Context(), port)
	listenErr, ok := errors.AsType[*listenError](err)
	if !ok {
		t.Fatalf("a port already in use gave %v, want a *listenError", err)
	}
	if !strings.Contains(listenErr.Error(), "YOMIHON_PORT") {
		t.Errorf("error = %q, want it to name YOMIHON_PORT", listenErr)
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Errorf("error %v does not keep the cause, want it to wrap EADDRINUSE", err)
	}
}

// ordered records what the process says and when, in one sequence, so a test
// can ask which of two things came first.
type ordered struct {
	mu     sync.Mutex
	events []string
}

func (o *ordered) add(event string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
}

func (o *ordered) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.events...)
}

type orderedWriter struct{ o *ordered }

func (w orderedWriter) Write(p []byte) (int, error) {
	w.o.add("out: " + strings.TrimSpace(string(p)))
	return len(p), nil
}

type orderedHandler struct {
	slog.Handler

	o *ordered
}

func (orderedHandler) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature
func (h orderedHandler) Handle(ctx context.Context, record slog.Record) error {
	h.o.add("log: " + record.Message)
	return h.Handler.Handle(ctx, record)
}

func orderedLog(o *ordered) *slog.Logger {
	return slog.New(orderedHandler{Handler: slog.DiscardHandler, o: o})
}

func indexOf(events []string, prefix string) int {
	for i, event := range events {
		if strings.HasPrefix(event, prefix) {
			return i
		}
	}
	return -1
}

// TestServingGivesTheAddressBeforeTheVaultIsRead locks the order the first
// minute depends on: the reader is told where to go, and the port is theirs,
// before the vault has been read.
func TestServingGivesTheAddressBeforeTheVaultIsRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	events := new(ordered)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- serveVault(ctx, func() {}, orderedLog(events),
			&config{root: root, port: "0", configDir: t.TempDir()}, orderedWriter{events})
	}()
	deadline := time.After(10 * time.Second)
	for indexOf(events.snapshot(), "log: vault snapshot built") < 0 {
		select {
		case <-deadline:
			t.Fatalf("the vault was never read; events = %v", events.snapshot())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	if err := <-result; err != nil {
		t.Fatalf("serveVault: %v", err)
	}

	seen := events.snapshot()
	address := indexOf(seen, "out: http://127.0.0.1:")
	read := indexOf(seen, "log: vault snapshot built")
	if address < 0 || !regexp.MustCompile(`^out: http://127\.0\.0\.1:\d+/$`).MatchString(seen[address]) {
		t.Fatalf("no clickable address was printed; events = %v", seen)
	}
	if address > read {
		t.Errorf("the address came after the vault was read; events = %v", seen)
	}
}

// TestStartupAnswersWhileTheVaultIsBeingRead covers what a reader meets in the
// wait: a page that names the vault and says how far the read has got, never a
// vault that looks empty.
func TestStartupAnswersWhileTheVaultIsBeingRead(t *testing.T) {
	t.Parallel()
	front := newTestStartup(t, fakeCount{done: 1204, total: 2413, folder: "courses/system-design"}, 0)

	t.Run("a page", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/notes/anything.md", http.NoBody))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want %d: the page asked for is not there yet", recorder.Code, http.StatusServiceUnavailable)
		}
		if recorder.Header().Get("Retry-After") == "" {
			t.Error("no Retry-After on a page that will be there shortly")
		}
		assertPermissionsPolicy(t, recorder.Result().Header)
		body := recorder.Body.String()
		for _, want := range []string{"data-scanning", "data-scanning-count", "data-scanning-folder", "notes", "/home/reader/notes", wording.ScanningKicker.In(wording.ZhHant), "1,204 / 2,413 個檔案", "courses/system-design"} {
			if !strings.Contains(body, want) {
				t.Errorf("page does not carry %q", want)
			}
		}
		if strings.Contains(body, "data-nothing=\"empty\"") {
			t.Error("the page reads as an empty vault")
		}
		if !strings.Contains(body, "<title>notes</title>") {
			t.Error("the document title is not the vault's name alone")
		}
	})

	t.Run("what the page says comes in the order of a title page", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
		body := recorder.Body.String()
		// The eyebrow, the vault's name, where it is, the meter, and under the
		// meter the count and the folder being read: the number sits under the
		// bar the way a page number sits under the text.
		at := 0
		for _, want := range []string{wording.ScanningKicker.In(wording.ZhHant), `id="scanning-title"`, "/home/reader/notes", "<progress", "data-scanning-count", "data-scanning-folder"} {
			i := strings.Index(body[at:], want)
			if i < 0 {
				t.Fatalf("the page lacks %q after offset %d, in order", want, at)
			}
			at += i
		}
	})

	t.Run("no count before the folder is walked", func(t *testing.T) {
		t.Parallel()
		walking := newTestStartup(t, fakeCount{}, 0)
		recorder := httptest.NewRecorder()
		walking.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
		if strings.Contains(recorder.Body.String(), "0 / 0") {
			t.Error("the page states a count nobody has")
		}
	})

	t.Run("the progress route", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/scan", http.NoBody))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		want := `{"done":1204,"total":2413,"text":"1,204 / 2,413 個檔案","folder":"courses/system-design"}`
		if got := strings.TrimSpace(recorder.Body.String()); got != want {
			t.Errorf("body = %s, want %s", got, want)
		}
	})

	t.Run("in the reader's language", func(t *testing.T) {
		t.Parallel()
		request := siteRequest(t, http.MethodGet, "/scan", http.NoBody)
		request.Header.Set("Cookie", wording.CookieName+"="+string(wording.En))
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, request)
		if !strings.Contains(recorder.Body.String(), "1,204 / 2,413 files") {
			t.Errorf("body = %s, want the English count", recorder.Body.String())
		}
	})

	t.Run("the assets of that page", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/static/app.css", http.NoBody))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET /static/app.css = %d, want %d: the page would be bare", recorder.Code, http.StatusOK)
		}
	})

	t.Run("a write", func(t *testing.T) {
		t.Parallel()
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodPost, "/marks", strings.NewReader("path=a.md")))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Errorf("POST /marks = %d, want %d: the route that would keep it is not built", recorder.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("a name that is not this machine", func(t *testing.T) {
		t.Parallel()
		for _, target := range []string{"/", "/scan", "/static/app.css"} {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
			request.Host = "127.0.0.1.nip.io:9610"
			recorder := httptest.NewRecorder()
			front.ServeHTTP(recorder, request)
			assertPermissionsPolicy(t, recorder.Result().Header)
			if recorder.Code != http.StatusForbidden {
				t.Errorf("caught: GET %s addressed elsewhere = %d, want %d", target, recorder.Code, http.StatusForbidden)
			}
		}
	})
}

// TestStartupEscapesTheFolderName keeps the one thing on the page that a
// stranger chose — the folder's own name — from being read as markup.
func TestStartupEscapesTheFolderName(t *testing.T) {
	t.Parallel()
	front := newStartup(`<img src=x onerror=alert(1)>`, `/tmp/<b>`, fakeCount{}, 0, slog.New(slog.DiscardHandler))
	recorder := httptest.NewRecorder()
	front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
	body := recorder.Body.String()
	if strings.Contains(body, "<img src=x") || strings.Contains(body, "<b>") {
		t.Error("the folder's name reached the page as markup")
	}
}

// TestStartupHandsEverythingToTheSite checks the hand-over: once the site is
// open every request is the site's, the route that reported the read is gone,
// and the page's script nonce is the one the policy header names, which two
// layers of the browser policy would break.
func TestStartupHandsEverythingToTheSite(t *testing.T) {
	t.Parallel()
	front := newTestStartup(t, fakeCount{done: 1, total: 2}, 0)
	front.open(openedSite(t))

	home := httptest.NewRecorder()
	front.ServeHTTP(home, siteRequest(t, http.MethodGet, "/", http.NoBody))
	if home.Code != http.StatusOK {
		t.Fatalf("GET / after the read = %d, want %d", home.Code, http.StatusOK)
	}
	if strings.Contains(home.Body.String(), "data-scanning") {
		t.Error("the reading page is still being served")
	}
	nonce := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(home.Header().Get("Content-Security-Policy"))
	if len(nonce) != 2 || !strings.Contains(home.Body.String(), `nonce="`+nonce[1]+`"`) {
		t.Errorf("the page's scripts do not carry the nonce the policy names (%v)", nonce)
	}

	scan := httptest.NewRecorder()
	front.ServeHTTP(scan, siteRequest(t, http.MethodGet, "/scan", http.NoBody))
	if scan.Code != http.StatusNotFound {
		t.Errorf("GET /scan after the read = %d, want %d: a waiting page learns it is over from this", scan.Code, http.StatusNotFound)
	}
}

// TestStartupWaitsBeforeShowingThePage covers the grace: a vault read inside it
// is never preceded by the page, and one that is not gets the page.
func TestStartupWaitsBeforeShowingThePage(t *testing.T) {
	t.Parallel()

	t.Run("read in time", func(t *testing.T) {
		t.Parallel()
		front := newTestStartup(t, fakeCount{}, 5*time.Second)
		site := openedSite(t)
		go func() {
			time.Sleep(50 * time.Millisecond)
			front.open(site)
		}()
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET / = %d, want %d: the vault was read inside the grace, so the page should never have shown", recorder.Code, http.StatusOK)
		}
	})

	t.Run("not read in time", func(t *testing.T) {
		t.Parallel()
		front := newTestStartup(t, fakeCount{}, 20*time.Millisecond)
		recorder := httptest.NewRecorder()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Errorf("GET / = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("a reader who comes after the grace is not kept waiting", func(t *testing.T) {
		t.Parallel()
		front := newTestStartup(t, fakeCount{}, time.Minute)
		front.began = time.Now().Add(-2 * time.Minute)
		recorder := httptest.NewRecorder()
		asked := time.Now()
		front.ServeHTTP(recorder, siteRequest(t, http.MethodGet, "/", http.NoBody))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Errorf("GET / = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
		}
		if waited := time.Since(asked); waited > 10*time.Second {
			t.Errorf("the request waited %v, but the read had already run past the grace", waited)
		}
	})
}

// A vault whose folder cannot be read answers every address with the page a
// note that cannot be read has: the vault's name, why in words, where it is, and
// the one thing to do. It is a 503, and it stands behind the same loopback and
// browser-policy edges as the reading page does.
func TestAVaultThatCannotBeReadAnswersWithThePageThatSaysWhy(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		err  error
		zh   string
		en   string
	}{
		{"missing", &fs.PathError{Op: "open", Path: "/home/reader/notes", Err: fs.ErrNotExist}, "找不到資料夾", "The folder cannot be found"},
		{"denied", fmt.Errorf("open vault root: %w", &fs.PathError{Op: "open", Path: "/home/reader/notes", Err: fs.ErrPermission}), "沒有讀取權限", "No permission to read it"},
		{"other", errors.New("something else"), "資料夾讀不進來", "The folder cannot be read"},
	} {
		for _, language := range []string{"zh-Hant", "en"} {
			t.Run(test.name+"/"+language, func(t *testing.T) {
				t.Parallel()
				front := newTestStartup(t, fakeCount{}, 0)
				front.fail(test.err)
				req := siteRequest(t, http.MethodGet, "/notes/anything.md", http.NoBody)
				req.Header.Set("Cookie", wording.CookieName+"="+language)
				rec := httptest.NewRecorder()
				front.ServeHTTP(rec, req)

				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("status = %d, want 503", rec.Code)
				}
				if rec.Header().Get("Content-Security-Policy") == "" {
					t.Error("the answer carries no browser policy, so it did not pass the edge the reading page does")
				}
				body := rec.Body.String()
				want := test.zh
				action := "重新讀取"
				if language == "en" {
					want, action = test.en, "Read again"
				}
				for _, part := range []string{`data-nothing="unreadable"`, "notes", "/home/reader/notes", want, action, `href="/"`} {
					if !strings.Contains(body, part) {
						t.Errorf("the page does not hold %q", part)
					}
				}
				for _, leaked := range []string{"something else", "open vault root", "no such file", "permission denied"} {
					if strings.Contains(body, leaked) {
						t.Errorf("the page shows %q, which is an error and not words for a reader", leaked)
					}
				}
			})
		}
	}
}

func TestAVaultThatCannotBeReadStillTurnsBackAnotherHostName(t *testing.T) {
	t.Parallel()
	front := newTestStartup(t, fakeCount{}, 0)
	front.fail(fs.ErrNotExist)
	req := siteRequest(t, http.MethodGet, "/", http.NoBody)
	req.Host = "evil.example:9610"
	rec := httptest.NewRecorder()
	front.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "/home/reader/notes") {
		t.Errorf("a request for another host name was told where the vault is: %d %q", rec.Code, rec.Body.String())
	}
}
