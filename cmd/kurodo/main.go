// kurodo is the local reading-and-adjudication interface for the
// Obsidian vault: it reads everything, writes exactly one frontmatter field
// (status), and never leaves 127.0.0.1.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/koopa0/kurodo/internal/asset"
	"github.com/koopa0/kurodo/internal/lesson"
	"github.com/koopa0/kurodo/internal/nav"
	"github.com/koopa0/kurodo/internal/note"
	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/report"
	"github.com/koopa0/kurodo/internal/schema"
	"github.com/koopa0/kurodo/internal/search"
	"github.com/koopa0/kurodo/internal/snapshot"
	"github.com/koopa0/kurodo/internal/status"
	"github.com/koopa0/kurodo/internal/syllabus"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if len(os.Args) < 2 || os.Args[1] != "serve" {
		log.Error("usage: kurodo serve")
		os.Exit(2)
	}
	if err := run(log); err != nil {
		log.Error("kurodo exited", "error", err)
		os.Exit(1)
	}
}

type config struct {
	root string
	port string
}

func loadConfig() (config, error) {
	cfg := config{
		root: os.Getenv("KURODO_ROOT"),
		port: os.Getenv("KURODO_PORT"),
	}
	if cfg.root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return config{}, fmt.Errorf("resolve vault root: %w", err)
		}
		cfg.root = filepath.Join(home, "obsidian")
	}
	if cfg.port == "" {
		cfg.port = "9610"
	}
	if _, err := os.Stat(cfg.root); err != nil { // #nosec G703 -- root is the operator's own vault path from local config
		return config{}, fmt.Errorf("vault root: %w", err)
	}
	return cfg, nil
}

func run(log *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Fault tolerance is asymmetric by direction: a missing/broken contract
	// must never abort the server. Reading has no dependency on it at all;
	// only the write face (internal/status) does, and it fails closed on
	// a nil contract — no transition keys shown, every POST /status
	// rejected. Do not turn this back into a fatal error: a closed write
	// face is harmless, but losing the reading face over a schema problem
	// is not.
	contract, err := schema.Load(cfg.root)
	if err != nil {
		log.Warn("vault contract unavailable; write face is closed (fail-closed)", "error", err)
		contract = nil
	} else {
		log.Info("vault contract loaded",
			"version", contract.Version, "lifecycle_stages", len(contract.Lifecycle))
	}

	statusSvc := status.NewService(cfg.root, contract)

	// Lesson slot sidecars load once from System/slots/, a separate read
	// path from the vault scanner (slots are never indexed as notes). Fail-open
	// like the contract: a missing or broken slots dir just means lessons render
	// without the pattern machine, never a server abort.
	slots, err := lesson.BuildSlotIndex(filepath.Join(cfg.root, "System", "slots"))
	if err != nil {
		log.Warn("slot sidecars unavailable; lessons render without the pattern machine", "error", err)
		slots = nil
	} else {
		log.Info("slot sidecars loaded", "lessons", len(slots))
	}

	// The concept index (also a separate, fail-open read path): the grammar notes
	// a lesson links to, for the in-app concept sheet. Absent dir → empty index →
	// wikilinks just navigate to the concept notes.
	concepts, err := lesson.BuildConceptIndex(cfg.root)
	if err != nil {
		log.Warn("concept index unavailable; lesson wikilinks navigate instead of opening a sheet", "error", err)
		concepts = nil
	} else {
		log.Info("concept index built", "concepts", len(concepts))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The shared vault Snapshot: graph, nav, and search built together
	// from the vault and held behind an atomic.Pointer — rebuilt and swapped
	// as one so a fresh graph is never paired with a stale nav. New does the
	// initial synchronous build (each model degrades to empty on failure,
	// never aborting the server — the same asymmetric fault tolerance as the
	// contract load above, owned by internal/snapshot). Run drives
	// the ~2s mtime scanner that rebuilds and swaps on any vault change, so an
	// edited note stops going stale until restart. It is
	// cancellable via ctx for graceful shutdown.
	store := snapshot.New(cfg.root, log)
	go store.Run(ctx)

	// The renderer resolves wikilinks against the store's live graph on every
	// call (store.Resolver reads the current Snapshot), so it is built once yet
	// always current. The reading and search handlers read the current
	// Snapshot's Nav / Search per request through provider closures.
	renderer := render.New(cfg.root, store.Resolver())
	navProvider := func() *nav.Model { return store.Current().Nav }
	searchProvider := func() *search.Index { return store.Current().Search }
	countsProvider := func() map[string]int { return store.Current().Search.CountByStatus() }

	mux := http.NewServeMux()
	note.NewHandler(note.Deps{
		Root:       cfg.root,
		Renderer:   renderer,
		Status:     statusSvc,
		Nav:        navProvider,
		Counts:     countsProvider,
		Provenance: statusSvc.LastCommitHash,
		Log:        log,
		Slots:      slots,
		Concepts:   concepts,
	}).Register(mux)
	status.NewHandler(statusSvc, log).Register(mux)
	search.NewHandler(searchProvider, log).Register(mux)
	syllabus.NewHandler(syllabus.Deps{Nav: navProvider, Log: log}).Register(mux)
	report.NewHandler(report.Deps{Root: cfg.root, Nav: navProvider, Log: log}).Register(mux)
	asset.Register(mux)

	// Browser-only hardening: a same-origin form POST
	// triggers no CORS preflight, so any website can otherwise fire a
	// cross-site POST at 127.0.0.1's /status. CrossOriginProtection blocks
	// that class of request. It does NOT and cannot address
	// local, non-browser processes (curl, an
	// agent) — same-account local processes are cryptographically
	// indistinguishable, so that limit is accepted policy, not something
	// to engineer around with tokens.
	handler := http.NewCrossOriginProtection().Handler(mux)

	// Loopback is hardcoded; only the port is configurable — kurodo and
	// everything it derives from the vault must never be reachable from
	// another machine.
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", cfg.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()
	log.Info("kurodo serving", "addr", listener.Addr().String(), "vault", cfg.root)

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	log.Info("kurodo stopped")
	return nil
}
