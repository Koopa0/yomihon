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
	root := parsed.root
	if root == "" {
		root, err = os.Getwd()
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
			Root:     root,
			Paths:    parsed.positionals,
			All:      parsed.all,
			Deny:     parsed.deny,
			Baseline: parsed.baseline,
			Format:   format,
		})
	case "coverage":
		payload, exit, err = judge.RunCoverage(ctx, &judge.CoverageOptions{Root: root, Format: format})
	case "exists":
		payload, exit, err = judge.RunExists(ctx, &judge.ExistsOptions{
			Root: root, Name: parsed.positionals[0], Format: format,
		})
	default:
		return commandError(stderr, fmt.Errorf("unknown command %q", command))
	}
	if err != nil {
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
// any folder is opened, and names the word to move.
//
// This command takes two directory-shaped words that read alike at a shell and
// mean opposite things: the vault to judge, which goes after --root, and a
// filter narrowing the judging to part of that vault, which is spelled the way
// the vault spells it — from the vault's own root, with no leading slash. A
// reader who types the vault where a filter goes has written something that
// cannot name a filter at all.
//
// The engine refuses the same shape for its own callers, in its own words. It
// cannot say this sentence, because the word to move is a flag and the engine
// takes no flags; and saying it only there would mean opening a folder first,
// so a reader standing outside their vault would be told their folder carries
// no contract rather than which word they misplaced.
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
// only needs the first line of. It is written for whoever typed the command:
// what is missing, where it belongs, why these commands need it, and the face
// that needs none of it.
//
// Naming the contract's path is not the redaction those commands refuse. What
// they refuse is the decoder's account of an existing file's keys, which would
// send vault content out under the very policy that is missing. Where the file
// is supposed to live is not vault content, and a refusal that will not say
// what is missing leaves the reader with nothing to do.
func contractGuidance(err error) string {
	switch {
	case errors.Is(err, judge.ErrNoVaultContract):
		return "  yomihon reads " + schema.ContractRelPath + " for the note types, fields and\n" +
			"  lifecycle that check, coverage and exists judge against, and for the directories\n" +
			"  whose contents must never leave this machine. A folder carrying no such file has\n" +
			"  declared nothing, and these three commands have no vocabulary to answer in.\n" +
			"  Reading and search need none of it: yomihon serve --root <dir>\n"
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

func commandError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "yomihon: %v\n", err) //nolint:errcheck // an already-failing command has no second channel for a stderr failure
	if guidance := contractGuidance(err); guidance != "" {
		_, _ = fmt.Fprint(stderr, guidance) //nolint:errcheck // the same already-failing channel
	}
	return 2
}
