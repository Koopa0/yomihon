package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/koopa0/yomihon/internal/asset"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// scanGrace is how long the first read may run before a page is answered with
// the one that says the vault is being read. A small vault is read well inside
// it, and its reader never sees that page flash by. It counts from the start of
// the read, not from the request, so a reader who arrives late in a long read
// is shown the page at once rather than after a further wait.
const scanGrace = 400 * time.Millisecond

// scanAddress is the route that reports how far the first read has got. It
// exists only while that read runs; once the site is open the address is a
// page that is not there, which is how a page waiting on it learns to reload.
const scanAddress = "/scan"

// startup is what the listener serves from the moment it is bound. Until the
// first read of the vault has finished it answers every address with the page
// that says so, instead of a vault that looks empty; after, it hands every
// request to the site.
type startup struct {
	name     string
	path     string
	reading  http.Handler
	progress fileCount
	grace    time.Duration
	began    time.Time
	log      *slog.Logger

	site     atomic.Pointer[readingSite]
	ready    chan struct{}
	openOnce sync.Once

	// begin starts a read when a page is asked of the front. It is nil for a
	// front whose read is already under way. A vault the reader has listed and
	// not opened is bound and answers at once, but is not read until somebody
	// asks for it, and one that could not be read is read again when somebody
	// asks after it, since the reader may have put the folder back.
	begin func()
	// failed is why the vault could not be read, for the one answer that says so.
	failed atomic.Pointer[error]

	// reads guards whether a read is under way and when the last one failed, so
	// that one read runs at a time and a failed one is not begun again at once.
	reads    sync.Mutex
	started  bool
	failedAt time.Time
	// retryGap is how long a failed read is left before the next request begins
	// another.
	retryGap time.Duration
}

// retryGap is how soon after a failed read the next request for a page may begin
// another. The read of a folder that is not there costs nothing, and one of a
// large folder that failed half way is not begun again by every request that
// arrives while it is being looked at.
const retryGap = time.Second

// fileCount is how far the first read has got. It is defined here, by its only
// consumer, so a test can say any count it likes; the product's is
// *snapshot.Progress.
type fileCount interface {
	Files() (done, total int)
	Folder() string
}

func newStartup(vaultName, vaultPath string, progress fileCount, grace time.Duration, log *slog.Logger) *startup {
	s := &startup{name: vaultName, path: vaultPath, progress: progress, grace: grace, began: time.Now(), log: log, ready: make(chan struct{}), retryGap: retryGap}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+scanAddress, s.state)
	asset.Register(mux)
	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		s.page(w, r, vaultName, vaultPath)
	})
	mux.HandleFunc("/{path...}", refuse)
	// The same two edges the site stands behind: a response that carries the
	// browser policy, and a request that names this machine and nothing else.
	// Binding the socket to loopback keeps other machines out but not other
	// names, and this page is as much a reason to answer to one as the vault is.
	s.reading = origin.Protect(origin.LoopbackOnly(mux))
	return s
}

// whenAsked sets the read to start at the first request, which is also when the
// grace begins to count. start begins it at once.
func (s *startup) whenAsked(begin func()) { s.begin = begin }

// start begins a read if none is under way and the vault is not open. A read
// that failed is begun again, once the gap has passed, so the reader who put the
// folder back is not left on the page that says it was not there.
func (s *startup) start() {
	if s.begin == nil || s.site.Load() != nil {
		return
	}
	s.reads.Lock()
	if s.started && (s.failed.Load() == nil || time.Since(s.failedAt) < s.retryGap) {
		s.reads.Unlock()
		return
	}
	s.started = true
	s.failed.Store(nil)
	s.began = time.Now()
	s.reads.Unlock()
	s.begin()
}

// sinceBegan is how long the read now under way has run.
func (s *startup) sinceBegan() time.Duration {
	s.reads.Lock()
	defer s.reads.Unlock()
	return time.Since(s.began)
}

// fail says the vault cannot be read, so that its address answers that and not
// a page that waits for a read that will not finish.
func (s *startup) fail(err error) {
	s.reads.Lock()
	s.failedAt = time.Now()
	s.reads.Unlock()
	s.failed.Store(&err)
}

// open hands the listener to the site. It is called once, when the first
// generation exists.
func (s *startup) open(site *readingSite) {
	s.openOnce.Do(func() {
		s.site.Store(site)
		close(s.ready)
	})
}

func (s *startup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if wakes(r) {
		s.start()
	}
	if site := s.site.Load(); site != nil {
		site.ServeHTTP(w, r)
		return
	}
	if s.failed.Load() == nil && holds(r) {
		wait := time.NewTimer(max(s.grace-s.sinceBegan(), 0))
		defer wait.Stop()
		select {
		case <-s.ready:
			s.site.Load().ServeHTTP(w, r)
			return
		case <-wait.C:
		case <-r.Context().Done():
			return
		}
	}
	s.reading.ServeHTTP(w, r)
}

// wakes says whether a request may begin the read of a vault that has not been
// read. A browser that says where the request came from is believed: a request
// of the vault's own page, or one the reader typed or followed to a page, begins
// it; a picture, a script or a frame that some other page asked for does not,
// because a page the reader never opened must not make this process read a
// folder. A client that says nothing, such as a program on this machine, asked
// for the page itself.
func wakes(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "none", "same-origin":
		return true
	}
	return r.Header.Get("Sec-Fetch-Dest") == "document" && r.Header.Get("Sec-Fetch-Mode") == "navigate"
}

// holds says whether a request is one that waits the grace out. Only a page a
// reader asked for does: the answers about the read itself, and the assets of
// the page that tells it, are given at once.
func holds(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.URL.Path != scanAddress && !strings.HasPrefix(r.URL.Path, "/static/")
}

// page answers a request for any address with the vault's name and where its
// read has got to. It is a 503: the page a reader asked for is not there yet,
// and a program that asks must not take this for it.
func (s *startup) page(w http.ResponseWriter, r *http.Request, vaultName, vaultPath string) {
	if failed := s.failed.Load(); failed != nil {
		s.unreadable(w, r, *failed)
		return
	}
	done, total := s.progress.Files()
	view := pages.ScanningView{VaultName: vaultName, VaultPath: vaultPath, Done: done, Total: total, Folder: s.progress.Folder()}
	w.Header().Set("Retry-After", "1")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	chrome := layouts.ChromeFromRequest(r, vaultName)
	chrome.TitleAlone = true
	if err := pages.Scanning(view, chrome).Render(r.Context(), w); err != nil {
		s.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write reading page", "error", err)
	}
}

// unreadable answers any address of a vault whose folder could not be read with
// the page that says why. It is still a 503: the page asked for is not there,
// and a program that asks must not take this for it.
func (s *startup) unreadable(w http.ResponseWriter, r *http.Request, why error) {
	lang := origin.Language(r)
	reason := wording.VaultFolderOther
	switch {
	case errors.Is(why, fs.ErrNotExist):
		reason = wording.VaultFolderMissing
	case errors.Is(why, fs.ErrPermission):
		reason = wording.VaultFolderDenied
	}
	view := pages.VaultUnreadableView{VaultName: s.name, VaultPath: s.path, Reason: reason.In(lang)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	chrome := layouts.ChromeFromRequest(r, s.name)
	chrome.TitleAlone = true
	if err := pages.VaultUnreadable(view, chrome).Render(r.Context(), w); err != nil {
		s.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write unreadable vault page", "error", err)
	}
}

// scanState is what the page asks for: how far the read has got, the line that
// says so in the reader's language, and the folder the read is in.
type scanState struct {
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Text   string `json:"text"`
	Folder string `json:"folder"`
}

func (s *startup) state(w http.ResponseWriter, r *http.Request) {
	done, total := s.progress.Files()
	state := scanState{Done: done, Total: total, Folder: s.progress.Folder()}
	if total > 0 {
		state.Text = wording.ScanningProgress(origin.Language(r), done, total)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		s.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write reading state", "error", err)
	}
}

// refuse turns back anything that is not a read of a page while the vault is
// still being read: the routes that would answer it are not built yet.
func refuse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", "1")
	http.Error(w, wording.ScanningKicker.In(origin.Language(r)), http.StatusServiceUnavailable)
}
