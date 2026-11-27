package main

import (
	"fmt"
	"strconv"
)

// outputFormat is the shape a command prints its answer in.
type outputFormat int

const (
	// formatJSON is the machine format.
	formatJSON outputFormat = iota
	// formatHuman is the terminal format for a person.
	formatHuman
	// formatMarkdown is check's markdown report body; no other command has one.
	formatMarkdown
)

// String is the word --format takes for f.
func (f outputFormat) String() string {
	switch f {
	case formatJSON:
		return "json"
	case formatHuman:
		return "human"
	case formatMarkdown:
		return "md"
	default:
		panic("main: unknown outputFormat: " + strconv.Itoa(int(f)))
	}
}

// parseFormat maps a --format value to its format, reporting false for a value
// that is none of json, human or md.
func parseFormat(s string) (outputFormat, bool) {
	switch s {
	case "json":
		return formatJSON, true
	case "human":
		return formatHuman, true
	case "md":
		return formatMarkdown, true
	default:
		return 0, false
	}
}

// resolveFormat picks the output format: an explicit flag wins; otherwise the
// machine format for a pipe and the human format for a terminal, so an agent
// reading a pipe gets JSON and a person at a terminal gets the readable view.
func resolveFormat(explicit *outputFormat, isTTY bool) outputFormat {
	if explicit != nil {
		return *explicit
	}
	if isTTY {
		return formatHuman
	}
	return formatJSON
}

// refuseMarkdown is the refusal for --format md on a command that has no
// markdown view.
func refuseMarkdown(command string, explicit *outputFormat) error {
	if explicit != nil && *explicit == formatMarkdown {
		return fmt.Errorf("%s does not accept --format md; use json or human", command)
	}
	return nil
}
