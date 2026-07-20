// yomihon is the local reading-and-adjudication interface for the Obsidian
// vault. Its HTTP server listens only on 127.0.0.1 and writes exactly one
// frontmatter field (status); explicit semantic CLI actions are the separately
// ruled path that may contact the embedding provider.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/search/agent"
	"github.com/koopa0/yomihon/internal/status"
)

func main() {
	if handled, exit := status.RunGitChild(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	if text, handled, err := helpRequest(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintf(os.Stderr, "yomihon: %v\n", err)
			os.Exit(2)
		}
		if _, err := fmt.Fprint(os.Stdout, text); err != nil {
			fmt.Fprintf(os.Stderr, "yomihon: write help: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "yomihon: usage: yomihon <serve|search|search-index|check|coverage|exists> [options]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "yomihon: usage: yomihon serve")
			os.Exit(2)
		}
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		if err := run(log); err != nil {
			log.Error("yomihon exited", "error", err)
			os.Exit(1)
		}
	case "check", "coverage", "exists":
		os.Exit(judge.RunCommand(os.Args[1], os.Args[2:], os.Stdout, os.Stderr, stdoutIsTerminal()))
	case "search", "search-index":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exit := (agent.CLI{
			DefaultRoot:      configuredVaultRoot,
			Stdout:           os.Stdout,
			Stderr:           os.Stderr,
			ReadEmbeddingKey: func() string { return os.Getenv("YOMIHON_EMBED_KEY") },
		}).Run(ctx, os.Args[1], os.Args[2:])
		stop()
		os.Exit(exit)
	default:
		fmt.Fprintf(os.Stderr, "yomihon: unknown command %q; use serve, search, search-index, check, coverage, or exists\n", os.Args[1])
		os.Exit(2)
	}
}

func configuredVaultRoot() (string, error) {
	if root := os.Getenv("YOMIHON_ROOT"); root != "" {
		return root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "obsidian"), nil
}

// stdoutIsTerminal reports whether stdout is a terminal rather than a pipe or a
// file, so the output format can default to the human view for a person and the
// machine view for an agent reading a pipe.
func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type config struct {
	root string
	port string
}

func loadConfig() (config, error) {
	root, err := configuredVaultRoot()
	if err != nil {
		return config{}, fmt.Errorf("resolve vault root: %w", err)
	}
	cfg := config{root: root, port: os.Getenv("YOMIHON_PORT")}
	if cfg.port == "" {
		cfg.port = "9610"
	}
	info, err := os.Stat(cfg.root) // #nosec G703 -- root is the operator's own vault path from local config
	if err != nil {
		return config{}, fmt.Errorf("vault root: %w", err)
	}
	if !info.IsDir() {
		return config{}, fmt.Errorf("vault root %q is not a directory", cfg.root)
	}
	return cfg, nil
}

func run(log *slog.Logger) (resultErr error) {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	site, err := newReadingSite(ctx, cfg.root, log)
	if err != nil {
		return fmt.Errorf("build reading site: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, site.close())
	}()
	// Loopback is hardcoded; only the port is configurable — yomihon and
	// everything it derives from the vault must never be reachable from
	// another machine.
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", cfg.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	srv := newHTTPServer(site)
	log.Info("yomihon serving", "addr", listener.Addr().String(), "vault", cfg.root)
	if err := serveHTTP(ctx, srv, listener); err != nil {
		return err
	}
	log.Info("yomihon stopped")
	return nil
}
