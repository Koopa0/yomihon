// yomihon is the local reading-and-adjudication interface for the Obsidian
// vault. Its HTTP server listens only on 127.0.0.1 and writes exactly one
// frontmatter field (status). Nothing here contacts a network.
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
		// A whole-vault scan is the slow part of these three, and the reader
		// who presses Ctrl-C in the middle of one is asking for it to stop.
		// The scan checks this at the contract load, the file walk and every
		// note read, so the interrupt lands rather than being noticed once the
		// work is already done.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exit := runCommand(ctx, command, args, os.Stdout, os.Stderr, stdoutIsTerminal())
		stop()
		os.Exit(exit)
	default:
		fmt.Fprintf(os.Stderr, "yomihon: %q is neither a command nor a folder; commands are serve, check, coverage, and exists\n", command)
		os.Exit(2)
	}
}

// dispatch decides what the arguments ask for. A first argument naming a
// command is that command; anything else is the folder to read, so
// `yomihon ~/notes` needs no flag. A word that is neither is reported rather
// than serving the current folder under a name the reader did not mean.
func dispatch(argv []string) (command string, args []string) {
	if len(argv) == 0 {
		return "serve", nil
	}
	switch argv[0] {
	case "serve", "check", "coverage", "exists":
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

// serveRoot resolves which folder to read: the one just typed, or the one the
// reader is standing in. There is deliberately no third answer — no compiled-in
// path and no environment variable — because those two already answer the
// question between them, and a third is only somewhere for them to disagree.
func serveRoot(args []string) (string, error) {
	switch {
	case len(args) == 0:
		return os.Getwd()
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

// stdoutIsTerminal reports whether stdout is a terminal rather than a pipe or a
// file, so the output format can default to the human view for a person and the
// machine view for an agent reading a pipe.
func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// defaultPort is the port the listener binds when the environment names none.
// The help text builds its line from this constant rather than repeating the
// number, because a help line naming a port the server does not listen on is
// worse than no help line at all: the operator trusts it over the source.
const defaultPort = "9610"

type config struct {
	root string
	port string
}

func loadConfig(root string) (config, error) {
	cfg := config{root: root, port: os.Getenv("YOMIHON_PORT")}
	if cfg.port == "" {
		cfg.port = defaultPort
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
