package judge

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

type commandHooks struct {
	afterScan      func()
	afterNoteRead  func(string)
	beforeEmission func()
}

func (h commandHooks) actionHooks() actionHooks {
	return actionHooks{afterScan: h.afterScan, afterNoteRead: h.afterNoteRead}
}

// RunCommand parses and runs one judge command. It writes only the command's
// frozen payload to stdout, writes usage or tool failures to stderr, and
// returns the process exit code: 0 for success, 1 for a gate hit or missing
// note, and 2 for an invalid invocation or tool failure.
func RunCommand(command string, args []string, stdout, stderr io.Writer, stdoutIsTerminal bool) int {
	return runCommand(command, args, stdout, stderr, stdoutIsTerminal, commandHooks{})
}

func runCommand(
	command string,
	args []string,
	stdout, stderr io.Writer,
	stdoutIsTerminal bool,
	hooks commandHooks,
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
		}, hooks.actionHooks())
	case "coverage":
		prepared, err = prepareCoverageWithHooks(&CoverageOptions{Root: root, Format: format}, hooks.actionHooks())
	case "exists":
		prepared, err = prepareExistsWithHooks(&ExistsOptions{
			Root: root, Name: parsed.positionals[0], Format: format,
		}, hooks.actionHooks())
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
		return nil
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

func commandError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "yomihon: %v\n", err) //nolint:errcheck // an already-failing CLI has no second channel for a stderr failure
	return 2
}
