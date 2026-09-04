package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

// The front end of the three adjudicating commands — check, coverage and
// exists. It reads the words typed after the command, decides which folder to
// judge, hands the engine already-resolved options, and turns what comes back
// into stdout bytes, stderr sentences and a process exit code.
//
// It lives here rather than in the engine because this is where the usage
// strings live, and a parser that disagrees with the usage string it is
// documented by fails nowhere. The engine keeps the judging and the frozen
// output; every word that is only a word because a person typed it at a shell —
// the flag spellings, the working directory, the paragraph after a refusal —
// belongs to the binary.

// commandArgs holds the common flags parsed from a check, coverage, or exists
// invocation and the positionals owned by that invocation.
type commandArgs struct {
	root        string
	format      *judge.Format
	all         bool
	deny        []string
	baseline    string
	positionals []string
}

// runCommand parses and runs one adjudicating command. It writes only the
// command's frozen payload to stdout, writes usage or tool failures to stderr,
// and returns the process exit code: 0 for success, 1 for a gate hit or missing
// note, and 2 for an invalid invocation or tool failure.
func runCommand(
	ctx context.Context,
	command string,
	args []string,
	stdout, stderr io.Writer,
	stdoutIsTerminal bool,
) int {
	parsed, err := parseCommandArgs(args)
	if err != nil {
		return commandError(stderr, err)
	}
	if err = validateCommandArgs(command, &parsed); err != nil {
		return commandError(stderr, err)
	}
	if parsed.root == "" {
		parsed.root, err = os.Getwd()
		if err != nil {
			return commandError(stderr, fmt.Errorf("resolve working directory: %w", err))
		}
	}
	format := judge.ResolveFormat(parsed.format, stdoutIsTerminal)

	var payload []byte
	var exit int
	switch command {
	case "check":
		payload, exit, err = judge.RunCheck(ctx, &judge.CheckOptions{
			Root:     parsed.root,
			Paths:    parsed.positionals,
			All:      parsed.all,
			Deny:     parsed.deny,
			Baseline: parsed.baseline,
			Format:   format,
		})
	case "coverage":
		payload, exit, err = judge.RunCoverage(ctx, &judge.CoverageOptions{Root: parsed.root, Format: format})
	case "exists":
		payload, exit, err = judge.RunExists(ctx, &judge.ExistsOptions{
			Root: parsed.root, Name: parsed.positionals[0], Format: format,
		})
	default:
		return commandError(stderr, fmt.Errorf("unknown command %q", command))
	}
	if err != nil {
		// check is the only command whose positionals name scope inside the
		// vault rather than the vault itself, so only its error can earn the
		// extra sentence checkError adds; coverage and exists get the plain
		// refusal.
		if command == "check" {
			return checkError(stderr, err, parsed.root, parsed.positionals)
		}
		return commandError(stderr, err)
	}
	if _, err := stdout.Write(payload); err != nil {
		return commandError(stderr, fmt.Errorf("write output: %w", err))
	}
	return exit
}

// validateCommandArgs refuses flags and positionals a command does not own.
// Each of the three takes a different subset of one flag set, and the usage
// strings beside this file are where a reader learns which.
func validateCommandArgs(command string, args *commandArgs) error {
	switch command {
	case "check":
		return scopeIsNotTheVaultItself(args.positionals)
	case "coverage":
		if args.all {
			return errors.New("coverage does not accept --all")
		}
		if len(args.deny) > 0 {
			return errors.New("coverage does not accept --deny")
		}
		if args.baseline != "" {
			return errors.New("coverage does not accept --baseline")
		}
		if len(args.positionals) > 0 {
			return errors.New("coverage takes no positional arguments")
		}
		return nil
	case "exists":
		if args.all {
			return errors.New("exists does not accept --all")
		}
		if len(args.deny) > 0 {
			return errors.New("exists does not accept --deny")
		}
		if args.baseline != "" {
			return errors.New("exists does not accept --baseline")
		}
		if len(args.positionals) != 1 {
			return errors.New("exists takes exactly one name argument")
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

// scopeIsNotTheVaultItself refuses a scope written as an absolute path, before
// any folder is opened, and names the word to move. The command takes two
// directory-shaped words that read alike: the vault, after --root, and a
// filter, spelled from the vault's own root. Refusing here rather than in the
// engine keeps a reader outside their vault from being told it has no contract.
func scopeIsNotTheVaultItself(positionals []string) error {
	for _, p := range positionals {
		if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
			continue
		}
		return fmt.Errorf(
			"path filter %q is an absolute path, and a filter names part of the vault from the vault's own root, such as %s; the vault itself goes after --root",
			p, vaultRelativeScope)
	}
	return nil
}

// vaultRelativeScope stands in for a real path in the refusal above. It is a
// shape rather than a name taken from any particular vault: the directories a
// reader keeps notes in are theirs to name, and quoting one they do not have
// would send them looking for it. The engine's own refusals reach for an
// example of the same shape; neither is authority for the other, since an
// example of a well-formed path is not vocabulary anything has to agree on.
const vaultRelativeScope = `"Notes" or "Notes/topic.md"`

// parseCommandArgs accepts both --flag value and --flag=value spellings. An
// unknown flag, a missing value, or an empty value is an error.
func parseCommandArgs(args []string) (commandArgs, error) {
	var parsed commandArgs
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		name, inline, hasInline := strings.Cut(arg, "=")
		switch name {
		case "--all":
			if hasInline {
				return parsed, fmt.Errorf("flag %s takes no value", name)
			}
			parsed.all = true
		case "--root", "--baseline", "--deny", "--format":
			value, rest, err := commandFlagValue(name, inline, hasInline, args)
			if err != nil {
				return parsed, err
			}
			args = rest
			if err := parsed.setFlag(name, value); err != nil {
				return parsed, err
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return parsed, fmt.Errorf("unknown flag %q", arg)
			}
			parsed.positionals = append(parsed.positionals, arg)
		}
	}
	return parsed, nil
}

func commandFlagValue(name, inline string, hasInline bool, rest []string) (value string, remaining []string, err error) {
	switch {
	case hasInline:
		value, remaining = inline, rest
	case len(rest) == 0:
		return "", rest, fmt.Errorf("flag %s needs a value", name)
	default:
		value, remaining = rest[0], rest[1:]
	}
	if value == "" {
		return "", remaining, fmt.Errorf("flag %s needs a non-empty value", name)
	}
	return value, remaining, nil
}

func (args *commandArgs) setFlag(name, value string) error {
	switch name {
	case "--root":
		args.root = value
	case "--baseline":
		args.baseline = value
	case "--deny":
		args.deny = append(args.deny, value)
	case "--format":
		format, ok := judge.ParseFormat(value)
		if !ok {
			return fmt.Errorf("invalid --format %q; use json, human, or md", value)
		}
		args.format = &format
	}
	return nil
}

// contractGuidance is the paragraph a person needs after a refusal a program
// only needs the first line of. Naming where the contract belongs is not the
// redaction these commands refuse: what they withhold is an account of an
// existing file's keys, which is vault content, and a path is not.
func contractGuidance(err error) string {
	switch {
	case errors.Is(err, judge.ErrNoVaultContract):
		return "  yomihon reads " + schema.ContractRelPath + " for the note types, fields and\n" +
			"  lifecycle that check, coverage and exists judge against, and for the directories\n" +
			"  whose contents must never leave this machine. A folder carrying no such file has\n" +
			"  declared nothing, and these three commands have no vocabulary to answer in.\n" +
			"  Reading and search need none of it: yomihon <dir>\n"
	case errors.Is(err, judge.ErrPrivacyAuthorityUnavailable):
		return "  The contract is at " + schema.ContractRelPath + " and yomihon could not use it.\n" +
			"  The reason is not printed here: this command's output is written for a program to\n" +
			"  read, and stating the reason would quote the contract back out under exactly the\n" +
			"  policy that is missing. Read it where reading is the point: yomihon serve\n" +
			"  --root <dir> states the cause on the page, and the server logs it at startup.\n"
	default:
		return ""
	}
}

// misplacedVault reports the first positional that is itself a folder
// carrying a vault contract, so a reader who typed the vault in check's scope
// position is told which word to move rather than only that the folder they
// stood in declared nothing. It stats for the contract file's existence and
// nothing more — never opens or reads it — so a folder already refused for
// having no contract is asked nothing more than that. Its positionals always
// arrive already refused for the absolute-path shape scopeIsNotTheVaultItself
// owns, since checkError, its only caller, runs after that refusal has had
// its chance.
func misplacedVault(root string, positionals []string) (string, bool) {
	for _, p := range positionals {
		info, err := os.Stat(filepath.Join(root, p, schema.ContractRelPath)) // #nosec G703 -- root and the positional are both the operator's own CLI arguments; checked only for existence to shape a refusal, never opened
		if err == nil && !info.IsDir() {
			return p, true
		}
	}
	return "", false
}

func commandError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "yomihon: %v\n", err) //nolint:errcheck // an already-failing command has no second channel for a stderr failure
	if guidance := contractGuidance(err); guidance != "" {
		_, _ = fmt.Fprint(stderr, guidance) //nolint:errcheck // the same already-failing channel
	}
	return 2
}

// checkError is commandError's counterpart for check: the only command whose
// positionals name scope inside the vault rather than the vault itself, so a
// judge.ErrNoVaultContract from it may mean one of them was the vault all
// along. root and positionals are check's own resolved invocation, needed for
// nothing this function does beyond that one guess.
func checkError(stderr io.Writer, err error, root string, positionals []string) int {
	exit := commandError(stderr, err)
	if errors.Is(err, judge.ErrNoVaultContract) {
		if p, ok := misplacedVault(root, positionals); ok {
			hint := "  " + p + " carries a contract of its own, which makes it a vault rather\n" +
				"  than a path inside one: pass it as --root " + p + " instead.\n"
			_, _ = fmt.Fprint(stderr, hint) //nolint:errcheck // the same already-failing channel
		}
	}
	return exit
}
