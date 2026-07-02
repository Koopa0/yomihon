// kurodo (蔵人) is the local reading-and-adjudication interface for the
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
	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/note"
	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/schema"
	"github.com/koopa0/kurodo/internal/status"
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

	// §0.1's asymmetric fault tolerance: a missing/broken contract must
	// never abort the server. Reading has no dependency on it at all;
	// only the write face (internal/status) does, and it fails closed on
	// a nil contract — no transition keys shown, every POST /status
	// rejected. Do not turn this back into a fatal error (wall 3, and see
	// CLAUDE.md's predictable-mistake list for this repo).
	contract, err := schema.Load(cfg.root)
	if err != nil {
		log.Warn("vault contract unavailable; write face is closed (fail-closed)", "error", err)
		contract = nil
	} else {
		log.Info("vault contract loaded",
			"version", contract.Version, "lifecycle_stages", len(contract.Lifecycle))
	}

	statusSvc := status.NewService(cfg.root, contract)

	// Same asymmetric fault tolerance as the contract load above: building
	// the wikilink index walks and parses the whole vault, and a single
	// bad note's frontmatter must not be able to take the reading face
	// down with it (wall 4). graph.Build itself already tolerates a
	// per-note read/parse failure (that note just contributes no alias
	// keys); the only way this returns an error at all is the vault root
	// itself being unwalkable, which loadConfig already checked — so this
	// is normally unreachable, but on the off chance it isn't, fall back
	// to an empty index (every wikilink then resolves as unresolved and
	// is rendered — and diagnosed — as broken) rather than aborting the
	// server.
	idx, err := graph.Build(cfg.root)
	if err != nil {
		log.Warn("vault graph unavailable; wikilinks will render as unresolved", "error", err)
		idx = graph.BuildFromNotes(nil, nil)
	} else {
		log.Info("vault graph built")
	}

	mux := http.NewServeMux()
	note.NewHandler(cfg.root, render.New(cfg.root, idx), statusSvc, log).Register(mux)
	status.NewHandler(statusSvc, log).Register(mux)
	asset.Register(mux)

	// Browser-only hardening, deepening wall 2: a same-origin form POST
	// triggers no CORS preflight, so any website can otherwise fire a
	// cross-site POST at 127.0.0.1's /status. CrossOriginProtection blocks
	// that class of request. It does NOT and cannot address the
	// audit-boundary limit for local, non-browser processes (curl, an
	// agent) — that is documented policy, not something to engineer
	// around (decisions D17).
	handler := http.NewCrossOriginProtection().Handler(mux)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Wall 2: loopback is hardcoded; only the port is configurable.
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
