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
	"strings"
	"syscall"
	"time"

	"github.com/koopa0/kurodo/internal/asset"
	"github.com/koopa0/kurodo/internal/judge"
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
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "kurodo: usage: kurodo <serve|check|coverage|exists> [options]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		if err := run(log); err != nil {
			log.Error("kurodo exited", "error", err)
			os.Exit(1)
		}
	case "check", "coverage", "exists":
		os.Exit(runJudge(os.Args[1], os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "kurodo: unknown command %q; use serve, check, coverage, or exists\n", os.Args[1])
		os.Exit(2)
	}
}

// judgeArgs holds the flags parsed off a check, coverage, or exists invocation:
// the global root and format, check's own all/deny/baseline, and the positional
// arguments (check's paths or exists's name).
type judgeArgs struct {
	root        string
	format      *judge.Format
	all         bool
	deny        []string
	baseline    string
	positionals []string
}

// runJudge parses one judge subcommand's arguments, runs it, prints its output,
// and returns the process exit code. A parse or tool error prints to stderr with
// the kurodo prefix and exits 2, distinct from a gate hit or a "does not exist",
// which come back as exit 1 from the command itself.
func runJudge(command string, args []string) int {
	p, err := parseJudgeArgs(args)
	if err != nil {
		return judgeError(err)
	}
	root := p.root
	if root == "" {
		wd, werr := os.Getwd()
		if werr != nil {
			return judgeError(fmt.Errorf("resolve working directory: %w", werr))
		}
		root = wd
	}
	format := judge.ResolveFormat(p.format, stdoutIsTerminal())

	var stdout []byte
	var exit int
	switch command {
	case "check":
		stdout, exit, err = judge.RunCheck(&judge.CheckOptions{
			Root:     root,
			Paths:    p.positionals,
			All:      p.all,
			Deny:     p.deny,
			Baseline: p.baseline,
			Format:   format,
		})
	case "coverage":
		stdout, exit, err = judge.RunCoverage(&judge.CoverageOptions{Root: root, Format: format})
	case "exists":
		if len(p.positionals) != 1 {
			return judgeError(errors.New("exists takes exactly one name argument"))
		}
		stdout, exit, err = judge.RunExists(&judge.ExistsOptions{Root: root, Name: p.positionals[0], Format: format})
	}
	if err != nil {
		return judgeError(err)
	}
	if _, werr := os.Stdout.Write(stdout); werr != nil {
		return judgeError(fmt.Errorf("write output: %w", werr))
	}
	return exit
}

// parseJudgeArgs reads the global root and format flags, check's all, deny
// (repeatable), and baseline flags, and the positional arguments, accepting
// both the "--flag value" and "--flag=value" spellings. An unknown flag or a
// flag missing its value is an error.
func parseJudgeArgs(args []string) (judgeArgs, error) {
	var p judgeArgs
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		name, inline, hasInline := strings.Cut(arg, "=")
		switch name {
		case "--all":
			if hasInline {
				return p, fmt.Errorf("flag %s takes no value", name)
			}
			p.all = true
		case "--root", "--baseline", "--deny", "--format":
			value, rest, err := takeValue(name, inline, hasInline, args)
			if err != nil {
				return p, err
			}
			args = rest
			if err := p.setFlag(name, value); err != nil {
				return p, err
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return p, fmt.Errorf("unknown flag %q", arg)
			}
			p.positionals = append(p.positionals, arg)
		}
	}
	return p, nil
}

// takeValue returns a value flag's value and the arguments left after it: the
// text after "=" when given inline, otherwise the next argument. A flag that
// ends the argument list with no value is an error.
func takeValue(name, inline string, hasInline bool, rest []string) (value string, remaining []string, err error) {
	switch {
	case hasInline:
		value, remaining = inline, rest
	case len(rest) == 0:
		return "", rest, fmt.Errorf("flag %s needs a value", name)
	default:
		value, remaining = rest[0], rest[1:]
	}
	// An empty value — from --flag= or --flag "" — is never meaningful for
	// these flags, and silently accepting it would let an unset variable in a
	// caller's --root=$VAR quietly scan the working directory instead of failing.
	if value == "" {
		return "", remaining, fmt.Errorf("flag %s needs a non-empty value", name)
	}
	return value, remaining, nil
}

// setFlag records one value flag on the parsed arguments, validating the output
// format.
func (p *judgeArgs) setFlag(name, value string) error {
	switch name {
	case "--root":
		p.root = value
	case "--baseline":
		p.baseline = value
	case "--deny":
		p.deny = append(p.deny, value)
	case "--format":
		f, ok := judge.ParseFormat(value)
		if !ok {
			return fmt.Errorf("invalid --format %q; use json, human, or md", value)
		}
		p.format = &f
	}
	return nil
}

// judgeError reports a tool error on stderr with the kurodo prefix and returns
// the tool-error exit code. Stderr is the one surface that names kurodo rather
// than reproducing the reference tool's bytes, since no pipeline reads it.
func judgeError(err error) int {
	fmt.Fprintf(os.Stderr, "kurodo: %v\n", err)
	return 2
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
	typeStatusCountsProvider := func() map[search.TypeStatus]int { return store.Current().Search.CountByTypeStatus() }

	mux := http.NewServeMux()
	note.NewHandler(note.Deps{
		Root:             cfg.root,
		Renderer:         renderer,
		Status:           statusSvc,
		Nav:              navProvider,
		Counts:           countsProvider,
		TypeStatusCounts: typeStatusCountsProvider,
		Provenance:       statusSvc.LastCommitHash,
		Log:              log,
		Slots:            slots,
		Concepts:         concepts,
	}).Register(mux)
	status.NewHandler(statusSvc, log).Register(mux)
	search.NewHandler(searchProvider, navProvider, log).Register(mux)
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
