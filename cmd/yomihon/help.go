package main

import "fmt"

const topLevelHelp = `Usage:
  yomihon [<dir>]                       read a folder (default: this one)
  yomihon serve [<dir>]
  yomihon search [options] <query...>
  yomihon search-index build [options]
  yomihon check [options] [path...]      judge a vault (--root <vault>; path narrows)
  yomihon coverage [options]
  yomihon exists [options] <name>

Use "yomihon <command> --help" for command help.
`

var commandHelp = map[string]string{
	"serve": "Usage: yomihon [<dir>]  —  or  yomihon serve [<dir>]\n" +
		"\n" +
		"Reads the folder named on the line, or the one you are standing in.\n" +
		"Serves it on 127.0.0.1:$YOMIHON_PORT (default 9610).\n" +
		"\n" +
		"The folder is fixed for the life of the process: reading another one\n" +
		"means another yomihon, on another port.\n",
	"search":       "Usage: yomihon search [--json] [--semantic] [--root <dir>] [--limit <1..1000>] [--] <query...>\n",
	"search-index": "Usage: yomihon search-index build [--json] [--renew-attempt-budget] [--root <dir>]\n",
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
		"Writes one JSON object per line when the output is not a terminal, and a\n" +
		"human summary when it is. --format decides instead of the terminal.\n" +
		"\n" +
		"Exits 0 when nothing named by --deny was found, 1 when something was, and\n" +
		"2 when the command itself could not run. Findings alone do not fail the\n" +
		"command: without --deny it reports and exits 0.\n",
	"coverage": "Usage: yomihon coverage [--root <dir>] [--format json|human|md]\n",
	"exists":   "Usage: yomihon exists [--root <dir>] [--format json|human|md] <name>\n",
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
	case 3:
		return searchIndexHelpTopic(args[0], args[1], args[2])
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

func searchIndexHelpTopic(first, second, third string) (string, bool) {
	if first == "help" && second == "search-index" && third == "build" {
		return "search-index", true
	}
	if first == "search-index" && second == "build" && isHelpFlag(third) {
		return "search-index", true
	}
	return "", false
}

func isHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h"
}
