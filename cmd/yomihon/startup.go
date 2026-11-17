package main

import (
	"encoding/json"
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

// scanGrace is how long a request waits for the first read to finish before it
// is answered with the page that says the vault is being read. A small vault is
// read well inside it, and its reader never sees that page flash by.
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
	reading  http.Handler
	progress fileCount
	grace    time.Duration
	log      *slog.Logger

	site     atomic.Pointer[readingSite]
	ready    chan struct{}
	openOnce sync.Once
}

// fileCount is how far the first read has got. It is defined here, by its only
// consumer, so a test can say any count it likes; the product's is
// *snapshot.Progress.
type fileCount interface {
	Files() (done, total int)
}

func newStartup(vaultName, vaultPath string, progress fileCount, grace time.Duration, log *slog.Logger) *startup {
	s := &startup{progress: progress, grace: grace, log: log, ready: make(chan struct{})}
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

// open hands the listener to the site. It is called once, when the first
// generation exists.
func (s *startup) open(site *readingSite) {
	s.openOnce.Do(func() {
		s.site.Store(site)
		close(s.ready)
	})
}

func (s *startup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if site := s.site.Load(); site != nil {
		site.ServeHTTP(w, r)
		return
	}
	if holds(r) {
		wait := time.NewTimer(s.grace)
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
	done, total := s.progress.Files()
	view := pages.ScanningView{VaultName: vaultName, VaultPath: vaultPath, Done: done, Total: total}
	w.Header().Set("Retry-After", "1")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	if err := pages.Scanning(view, layouts.ChromeFromRequest(r, vaultName)).Render(r.Context(), w); err != nil {
		s.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write reading page", "error", err)
	}
}

// scanState is what the page asks for: how far the read has got, and the line
// that says so in the reader's language.
type scanState struct {
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Text  string `json:"text"`
}

func (s *startup) state(w http.ResponseWriter, r *http.Request) {
	done, total := s.progress.Files()
	state := scanState{Done: done, Total: total}
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
