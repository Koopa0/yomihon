package judge

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
)

// commandArgs holds the common flags parsed from a check, coverage, or exists
// invocation and the positionals owned by that invocation.
type commandArgs struct {
	root        string
	format      *Format
	all         bool
	deny        []string
	baseline    string
	positionals []string
}

// RunCommand parses and runs one judge command. It writes only the command's
// frozen payload to stdout, writes usage or tool failures to stderr, and
// returns the process exit code: 0 for success, 1 for a gate hit or missing
// note, and 2 for an invalid invocation or tool failure.
func RunCommand(command string, args []string, stdout, stderr io.Writer, stdoutIsTerminal bool) int {
	return runCommand(command, args, stdout, stderr, stdoutIsTerminal, actionHooks{})
}

func runCommand(
	command string,
	args []string,
	stdout, stderr io.Writer,
	stdoutIsTerminal bool,
	hooks actionHooks,
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
	format := ResolveFormat(parsed.format, stdoutIsTerminal)

	var prepared preparedCommand
	switch command {
	case "check":
		prepared, err = prepareCheckWithHooks(&CheckOptions{
			Root:     root,
			Paths:    parsed.positionals,
			All:      parsed.all,
			Deny:     parsed.deny,
			Baseline: parsed.baseline,
			Format:   format,
		}, hooks)
	case "coverage":
		prepared, err = prepareCoverageWithHooks(&CoverageOptions{Root: root, Format: format}, hooks)
	case "exists":
		prepared, err = prepareExistsWithHooks(&ExistsOptions{
			Root: root, Name: parsed.positionals[0], Format: format,
		}, hooks)
	default:
		return commandError(stderr, fmt.Errorf("unknown judge command %q", command))
	}
	if err != nil {
		return commandError(stderr, err)
	}
	if hooks.beforeEmission != nil {
		hooks.beforeEmission()
	}
	if err := prepared.finish(); err != nil {
		return commandError(stderr, err)
	}
	if _, err := stdout.Write(prepared.stdout); err != nil {
		return commandError(stderr, fmt.Errorf("write output: %w", err))
	}
	return prepared.exit
}

func validateCommandArgs(command string, args *commandArgs) error {
	switch command {
	case "check":
		return scopeIsWrittenFromTheVaultRoot(args.positionals)
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
		return fmt.Errorf("unknown judge command %q", command)
	}
}

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
		format, ok := ParseFormat(value)
		if !ok {
			return fmt.Errorf("invalid --format %q; use json, human, or md", value)
		}
		args.format = &format
	}
	return nil
}

// scopeIsWrittenFromTheVaultRoot refuses a filter written as an absolute path.
//
// This command takes two directory-shaped words that read alike at a shell and
// mean opposite things: the vault to judge, which goes after --root, and a
// filter narrowing the judging to part of that vault, which is spelled the way
// the vault spells it — from the vault's own root, with no leading slash. A
// reader who types the vault where a filter goes has written something that
// cannot name a filter at all, and the refusal that followed talked about
// vault-relative paths without naming the word to move.
//
// The rule reads the shape of the argument, not what is on disk at it. That
// keeps it truthful — an absolute path is already refused further in, whether
// or not a vault happens to sit there — and it keeps this file out of the
// filesystem, which this face reaches through one rooted path only.
func scopeIsWrittenFromTheVaultRoot(positionals []string) error {
	for _, p := range positionals {
		if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
			continue
		}
		return fmt.Errorf(
			"path filter %q is an absolute path, and a filter names part of the vault from the vault's own root, such as %s; the vault itself goes after --root",
			p, vaultRelativeExample)
	}
	return nil
}

// contractGuidance is the paragraph a person needs after a refusal a program
// only needs the first line of. It is written for whoever typed the command:
// what is missing, where it belongs, why this face needs it, and the face that
// needs none of it.
//
// Naming the contract's path is not the redaction this face refuses. What it
// refuses is the decoder's account of an existing file's keys, which would send
// vault content out under the very policy that is missing. Where the file is
// supposed to live is not vault content, and a refusal that will not say what
// is missing leaves the reader with nothing to do.
func contractGuidance(err error) string {
	switch {
	case errors.Is(err, errNoVaultContract):
		return "  yomihon reads " + schema.ContractRelPath + " for the note types, fields and\n" +
			"  lifecycle that check, coverage and exists judge against, and for the directories\n" +
			"  whose contents must never leave this machine. A folder carrying no such file has\n" +
			"  declared nothing, and these three commands have no vocabulary to answer in.\n" +
			"  Reading and search need none of it: yomihon serve --root <dir>\n"
	case errors.Is(err, errPrivacyAuthorityUnavailable):
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
	_, _ = fmt.Fprintf(stderr, "yomihon: %v\n", err) //nolint:errcheck // an already-failing CLI has no second channel for a stderr failure
	if guidance := contractGuidance(err); guidance != "" {
		_, _ = fmt.Fprint(stderr, guidance) //nolint:errcheck // the same already-failing channel
	}
	return 2
}
