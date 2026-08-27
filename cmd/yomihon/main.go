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
	"strings"
	"syscall"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/search/agent"
)

func main() {
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
	command, args := dispatch(os.Args[1:])
	switch command {
	case "serve":
		root, err := serveRoot(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yomihon: %v\n", err)
			os.Exit(2)
		}
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		if err := run(log, root); err != nil {
			log.Error("yomihon exited", "error", err)
			os.Exit(1)
		}
	case "check", "coverage", "exists":
		os.Exit(judge.RunCommand(command, args, os.Stdout, os.Stderr, stdoutIsTerminal()))
	case "search", "search-index":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exit := (agent.CLI{
			DefaultRoot:      configuredVaultRoot,
			Stdout:           os.Stdout,
			Stderr:           os.Stderr,
			ReadEmbeddingKey: func() string { return os.Getenv("YOMIHON_EMBED_KEY") },
		}).Run(ctx, command, args)
		stop()
		os.Exit(exit)
	default:
		fmt.Fprintf(os.Stderr, "yomihon: %q is neither a command nor a folder; commands are serve, search, search-index, check, coverage, and exists\n", command)
		os.Exit(2)
	}
}

// dispatch decides what the arguments ask for. Reading a folder is what this
// program is for, so it is what bare `yomihon` does — in the folder you are
// standing in, the way every other tool that serves a directory behaves, and
// the way this program's own check and search commands already behaved while
// serve alone insisted on a flag.
//
// A first argument that names a command is that command. Anything else is
// taken as the folder to read, so `yomihon ~/notes` needs no flag; and when it
// is neither a command nor a directory, saying so beats silently serving the
// current folder under a name the reader clearly did not mean.
func dispatch(argv []string) (command string, args []string) {
	if len(argv) == 0 {
		return "serve", nil
	}
	switch argv[0] {
	case "serve", "search", "search-index", "check", "coverage", "exists":
		return argv[0], argv[1:]
	}
	// #nosec G703 -- the operator's own shell argument, asked only whether it
	// names a directory so a typo can be told from a folder. Nothing is opened
	// here; the vault capability is taken later by os.OpenRoot, which is what
	// actually bounds every read.
	if info, err := os.Stat(argv[0]); err == nil && info.IsDir() {
		return "serve", argv
	}
	return argv[0], argv[1:]
}

// serveRoot resolves which folder to read, in the order a reader would expect:
// what they just typed, then what their environment says, then the default. It
// exists because the folder is the one thing a first run must be able to state
// without reading the source — every other command already takes --root, and
// serve silently reading a different directory than the one the operator was
// standing in is the failure this closes.
func serveRoot(args []string) (string, error) {
	switch {
	case len(args) == 0:
		return configuredVaultRoot()
	case len(args) == 1 && !strings.HasPrefix(args[0], "-"):
		return args[0], nil
	case len(args) == 2 && args[0] == "--root":
		if args[1] == "" {
			return "", errors.New("--root needs a directory")
		}
		return args[1], nil
	default:
		return "", errors.New("usage: yomihon [dir] — or yomihon serve [dir] — or yomihon serve --root <dir>")
	}
}

// configuredVaultRoot answers which folder to read when the arguments named
// none: the folder the reader is standing in.
//
// It used to end at a path compiled into the binary — one particular person's
// vault — which is the author's own setup wearing the costume of a default.
// Anyone else's first run read a directory they had never mentioned, and the
// program's own check and search commands had meanwhile always read the
// current folder, so serve was the one face disagreeing. An environment
// variable naming the folder went the same way afterwards: where you are
// standing and what you name on the line already answer the question between
// them, and a third answer is only somewhere else for the two to disagree.
func configuredVaultRoot() (string, error) {
	return os.Getwd()
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

func loadConfig(root string) (config, error) {
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

func run(log *slog.Logger, root string) (resultErr error) {
	cfg, err := loadConfig(root)
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
	err = serveHTTP(ctx, srv, listener, shutdownGrace(srv))
	// The deferred close above waits without a deadline for in-flight handlers
	// and the scanner, deliberately, so an uncertain status write is not
	// abandoned. Announcing that turns the wait from an apparent hang into a
	// visible state, which is what the reader who pressed Ctrl-C is owed.
	// While the notify handler is installed that wait also swallows a repeated
	// interrupt; restoring default signal delivery the moment serving ends
	// lets a second Ctrl-C terminate a stuck shutdown.
	log.Info("yomihon no longer serving; finishing work already accepted")
	stop()
	if err != nil {
		return err
	}
	log.Info("yomihon stopped")
	return nil
}
