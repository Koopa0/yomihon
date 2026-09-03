package main

import "fmt"

const topLevelHelp = `Usage:
  yomihon [<dir>]                       read a folder (default: this one)
  yomihon serve [<dir>]                 read a folder (or --root <dir>)
  yomihon check [options] [path...]      judge a vault (--root <vault>; path narrows)
  yomihon coverage [options]
  yomihon exists [options] <name>

Use "yomihon <command> --help" for command help.
`

var commandHelp = map[string]string{
	"serve": "Usage: yomihon [<dir>]  —  or  yomihon serve [<dir>]\n" +
		"\n" +
		"Reads the folder named on the line, or the one you are standing in.\n" +
		"yomihon serve --root <dir> reads the same folder as yomihon serve <dir>.\n" +
		"Serves it on 127.0.0.1:$YOMIHON_PORT (default " + defaultPort + ").\n" +
		"\n" +
		"The folder is fixed for the life of the process: reading another one\n" +
		"means another yomihon, on another port.\n",
	"check": "Usage: yomihon check [--root <vault>] [--format json|human|md] [--all] [--deny <severity|rule-id>]... [--baseline <file>] [path...]\n" +
		"\n" +
		"--root is the vault to judge; without it, the folder you are standing in is\n" +
		"the vault. A [path...] is not a second way to say that: each one narrows the\n" +
		"judging to part of that vault and is written the way the vault spells it,\n" +
		"relative to its root — \"Notes\" or \"Notes/topic.md\". With none, the whole\n" +
		"vault is judged.\n" +
		"\n" +
		"  yomihon check --root ~/vault              the whole vault\n" +
		"  yomihon check --root ~/vault Notes        one folder of it\n" +
		"\n" +
		"A path inside a directory the contract withholds from agent-facing output\n" +
		"is refused rather than judged: that ground is scanned, but nothing from it\n" +
		"can be reported, and an empty answer would read as a clean verdict.\n" +
		"\n" +
		"The frontmatter schema rules judge only files inside the directories the\n" +
		"contract's scan.knowledge_dirs declares; a file outside them is still\n" +
		"scanned for links, but is not held to the schema.\n" +
		"\n" +
		"Writes one JSON object per line when the output is not a terminal, and a\n" +
		"human summary when it is. --format decides instead of the terminal.\n" +
		"\n" +
		"Exits 0 when nothing named by --deny was found, 1 when something was, and\n" +
		"2 when the command itself could not run. Findings alone do not fail the\n" +
		"command: without --deny it reports and exits 0.\n",
	"coverage": "Usage: yomihon coverage [--root <dir>] [--format json|human|md]\n" +
		"\n" +
		"Writes a compact JSON object when the output is not a terminal, and a\n" +
		"human summary when it is; --format decides instead of the terminal, and\n" +
		"md falls back to the human view.\n" +
		"\n" +
		"Exits 0 — coverage reports state, it never gates — and 2 when the\n" +
		"command itself could not run.\n",
	"exists": "Usage: yomihon exists [--root <dir>] [--format json|human|md] <name>\n" +
		"\n" +
		"Writes a compact JSON object when the output is not a terminal, and a\n" +
		"human answer when it is; --format decides instead of the terminal, and\n" +
		"md falls back to the human view.\n" +
		"\n" +
		"Exits 0 when a note for the name exists and 1 when none does, so a\n" +
		"caller can gate a write-if-absent on the exit code alone; 2 when the\n" +
		"command itself could not run.\n" +
		"\n" +
		"A note inside a directory the contract withholds from agent-facing output\n" +
		"is never described here — no path, no matched field. It still answers:\n" +
		"the exit stays 0, so a write-if-absent gated on it does not create a\n" +
		"second note under a withheld note's own name.\n",
}

func helpRequest(args []string) (text string, handled bool, err error) {
	topic, handled := helpTopic(args)
	if !handled {
		return "", false, nil
	}
	if topic == "" {
		return topLevelHelp, true, nil
	}
	text, ok := commandHelp[topic]
	if !ok {
		return "", true, fmt.Errorf("unknown help topic %q", topic)
	}
	return text, true, nil
}

func helpTopic(args []string) (string, bool) {
	switch len(args) {
	case 1:
		return rootHelpTopic(args[0])
	case 2:
		return commandHelpTopic(args[0], args[1])
	default:
		return "", false
	}
}

func rootHelpTopic(arg string) (string, bool) {
	switch arg {
	case "--help", "-h", "help":
		return "", true
	default:
		return "", false
	}
}

func commandHelpTopic(first, second string) (string, bool) {
	if first == "help" {
		return second, true
	}
	if !isHelpFlag(second) {
		return "", false
	}
	_, known := commandHelp[first]
	return first, known
}

func isHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h"
}
