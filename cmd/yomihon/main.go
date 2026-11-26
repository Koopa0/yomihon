// yomihon is the local reading-and-adjudication interface for the Obsidian
// vault. Its HTTP server listens only on 127.0.0.1 and writes into the vault
// only the reader's own marks, under reader-notes/.yomihon/marks/. Nothing here
// contacts a network.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/vaults"
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
		if err := run(log, root, os.Stdout); err != nil {
			if fault := operatorFault(err); fault != nil {
				fmt.Fprintf(os.Stderr, "yomihon: %v\n", fault)
				os.Exit(2)
			}
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
	case openCommand:
		// The reader's marks live under the platform's configuration directory,
		// which is resolved here, before the command runs. Without one the marks
		// cannot be found, and the command says so rather than list thoughts
		// alone.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exit := runOpenThoughts(ctx, args, userConfig(), os.Stdout, os.Stderr, stdoutIsTerminal())
		stop()
		os.Exit(exit)
	case vaultsCommand:
		// The list of vaults is the reader's own file under the configuration
		// directory, resolved here with the others.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exit := runVaults(ctx, args, userConfig(), os.Stdout, os.Stderr, stdoutIsTerminal())
		stop()
		os.Exit(exit)
	case approveFigureCommand, revokeFigureCommand:
		// These two speak about the reader, so they open the reader's own
		// directory, resolved here like the server's. Without one they say so,
		// whatever the reason the platform gave.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exit := runFigureCommand(ctx, command, args, &figureIO{
			in:          os.Stdin,
			out:         os.Stdout,
			errOut:      os.Stderr,
			interactive: isTerminal(os.Stdin) && isTerminal(os.Stdout),
		}, userConfig().configDir, time.Now)
		stop()
		os.Exit(exit)
	default:
		fmt.Fprintf(os.Stderr, "yomihon: %q is neither a command nor a folder; commands are serve, check, coverage, exists, %s, %s, %s, and %s\n",
			command, openCommand, vaultsCommand, approveFigureCommand, revokeFigureCommand)
		os.Exit(2)
	}
}

// userConfig is the reader's configuration directory as the platform names it,
// or why there is none. confdir.Platform is the one place it is asked, and the
// command asks it here and in loadConfig.
func userConfig() config {
	configDir, err := confdir.Platform()
	if err != nil {
		return config{noConfigDir: err.Error()}
	}
	return config{configDir: configDir}
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
	case "serve", "check", "coverage", "exists", openCommand, vaultsCommand, approveFigureCommand, revokeFigureCommand:
		return argv[0], argv[1:]
	case retiredOpenCommand:
		return openCommand, argv[1:]
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
	parsed, err := parseCommandArgs(args)
	if err != nil {
		return "", err
	}
	if parsed.format != nil || parsed.all || len(parsed.deny) > 0 || parsed.baseline != "" || parsed.adopt != "" || parsed.export ||
		parsed.rootCount > 1 || len(parsed.positionals) > 1 ||
		(parsed.rootCount > 0 && len(parsed.positionals) > 0) {
		return "", errors.New("usage: yomihon [dir] — or yomihon serve [dir] — or yomihon serve --root <dir>")
	}
	switch {
	case parsed.rootCount > 0:
		return parsed.root, nil
	case len(parsed.positionals) == 1:
		return parsed.positionals[0], nil
	default:
		return os.Getwd()
	}
}

// stdoutIsTerminal reports whether stdout is a terminal rather than a pipe or a
// file, so the output format can default to the human view for a person and the
// machine view for an agent reading a pipe.
func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// defaultPort is the starting port when the environment names none.
// The help text builds its line from this constant rather than repeating the
// number, because a help line naming a port the server does not listen on is
// worse than no help line at all: the operator trusts it over the source.
const defaultPort = "9610"

type config struct {
	root string
	port string
	// portSet is whether the environment named the port, which tells a reader's
	// own choice from the one used when they made none.
	portSet bool
	// configDir is where this platform keeps a program's own files, resolved
	// once at the start because reading the environment is the process's
	// business. The standard library reads HOME for it, XDG_CONFIG_HOME on
	// Linux and %AppData% on Windows, so this is the second thing outside the
	// arguments that decides where the binary looks — which is why
	// confdir.Platform is the one place that asks, and why what it answers is
	// held in this struct like the port and handed to the packages that need
	// it, as a type a vault's path cannot be passed as.
	//
	// The zero ConfigDir where the environment names none, or names one that
	// is not an absolute path. That is not a reason to refuse to start:
	// reading is the product and a kept reading place is a convenience on top
	// of it, so the room opens and the convenience is the thing that is
	// missing.
	configDir confdir.ConfigDir
	// noConfigDir is why none was resolved, for the one line that says so.
	noConfigDir string
}

// portError identifies a setting the operator can correct before startup,
// rather than a failure of the listener or the vault it was asked to read.
type portError struct{ value string }

func (e *portError) Error() string {
	return fmt.Sprintf("YOMIHON_PORT %q must be an integer from 0 to 65535", e.value)
}

// listenLoopback binds the one address yomihon listens on. Loopback is
// hardcoded; only the port is configurable — yomihon and everything it derives
// from the vault must never be reachable from another machine.
func listenLoopback(ctx context.Context, port string) (net.Listener, error) {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return nil, &listenError{err: err}
	}
	return listener, nil
}

// listenError is a port the operator can change before the next start: taken
// by another process, or one this user may not bind. The reader of the message
// is at a terminal, so it names the one setting that moves it.
type listenError struct{ err error }

func (e *listenError) Error() string {
	return fmt.Sprintf("%v; set YOMIHON_PORT to use another port", e.err)
}

func (e *listenError) Unwrap() error { return e.err }

// refusal is something the operator can correct, told in the words that say
// how. It ends the start the way a port that cannot be bound does.
type refusal struct {
	msg string
	// cause is the failure the refusal is about, when there is one.
	cause error
}

func (e *refusal) Error() string { return e.msg }

func (e *refusal) Unwrap() error { return e.cause }

// operatorFault is err when it is a setting the operator corrects before the
// next start, and nil for any other failure.
func operatorFault(err error) error {
	if fault, ok := errors.AsType[*refusal](err); ok {
		return fault
	}
	if fault, ok := errors.AsType[*portError](err); ok {
		return fault
	}
	if fault, ok := errors.AsType[*listenError](err); ok {
		return fault
	}
	return nil
}

func loadConfig(root string) (config, error) {
	cfg := config{root: root, port: os.Getenv("YOMIHON_PORT")}
	cfg.portSet = cfg.port != ""
	if cfg.port == "" {
		cfg.port = defaultPort
	}
	if configDir, dirErr := confdir.Platform(); dirErr != nil {
		cfg.noConfigDir = dirErr.Error()
	} else {
		cfg.configDir = configDir
	}
	info, err := os.Stat(cfg.root) // #nosec G703 -- root is the operator's own vault path from local config
	if err != nil {
		return config{}, fmt.Errorf("vault root: %w", err)
	}
	if !info.IsDir() {
		return config{}, fmt.Errorf("vault root %q is not a directory", cfg.root)
	}
	port, err := strconv.Atoi(cfg.port)
	if err != nil || port < 0 || port > 65535 {
		return config{}, &portError{value: cfg.port}
	}
	return cfg, nil
}

func run(log *slog.Logger, root string, out io.Writer) error {
	cfg, err := loadConfig(root)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveVault(ctx, stop, log, &cfg, out)
}

// serveVault serves the vault until ctx is done. stop gives the process back
// its default handling of an interrupt once serving has ended.
//
// With a configuration directory and a port of its own, the vault joins the
// list of vaults the reader has opened and is served beside them by the one
// running yomihon; otherwise it is served alone, and nothing is remembered.
func serveVault(ctx context.Context, stop func(), log *slog.Logger, cfg *config, out io.Writer) error {
	// A folder that overlaps the configuration directory would have what
	// yomihon keeps written into the folder being read, so it is refused before
	// anything is listed, bound or written. One that cannot be compared with it
	// is read with nothing kept: the process goes on with no configuration
	// directory, as though the platform named none, and the warning below says
	// why.
	switch relation, err := cfg.configDir.Classify(cfg.root); relation {
	case confdir.Overlaps:
		return &refusal{cause: err, msg: err.Error()}
	case confdir.Unknown:
		cfg.configDir, cfg.noConfigDir = confdir.ConfigDir{}, err.Error()
	case confdir.Apart:
	}
	if cfg.noConfigDir != "" {
		log.Warn("no configuration directory; reading is unaffected and no reading place can be kept",
			"reason", cfg.noConfigDir)
	}
	host := newHost(log, cfg.configDir, nil)
	if !cfg.remembers() {
		return host.serveOne(ctx, stop, cfg, out)
	}
	store, err := vaults.Open(cfg.configDir, func(port int) bool { return portFree(ctx, port) },
		vaults.Report(func(line string) { log.Warn(line) }))
	if err != nil {
		log.Warn("could not open the list of vaults; reading this folder alone", "error", err)
		return host.serveOne(ctx, stop, cfg, out)
	}
	host.store = store
	return host.serveListed(ctx, stop, cfg, out)
}

// startPort is the port the running yomihon holds. loadConfig has already
// refused one that is not a port.
func (c *config) startPort() int {
	port, err := strconv.Atoi(c.port)
	if err != nil {
		return 0
	}
	return port
}

// remembers says whether the vault joins the list. A port left to the system is
// not one a reader can come back to, and with no configuration directory there
// is nowhere to keep the list.
func (c *config) remembers() bool {
	return !c.configDir.IsZero() && c.startPort() != 0
}

// portFree says whether nothing is listening on the loopback port. It is asked
// by binding the one address yomihon listens on, which is the only way this
// process has to find out: it opens no connection.
func portFree(ctx context.Context, port int) bool {
	listener, err := listenLoopback(ctx, strconv.Itoa(port))
	if err != nil {
		return false
	}
	return listener.Close() == nil
}
