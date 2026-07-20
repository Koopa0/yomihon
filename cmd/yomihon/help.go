package main

import "fmt"

const topLevelHelp = `Usage:
  yomihon serve
  yomihon search [options] <query...>
  yomihon search-index build [options]
  yomihon check [options] [path...]
  yomihon coverage [options]
  yomihon exists [options] <name>

Use "yomihon <command> --help" for command help.
`

var commandHelp = map[string]string{
	"serve":        "Usage: yomihon serve\n",
	"search":       "Usage: yomihon search [--json] [--semantic] [--root <dir>] [--limit <1..1000>] [--] <query...>\n",
	"search-index": "Usage: yomihon search-index build [--json] [--renew-attempt-budget] [--root <dir>]\n",
	"check":        "Usage: yomihon check [--root <dir>] [--format json|human|md] [--all] [--deny <severity|rule-id>]... [--baseline <file>] [path...]\n",
	"coverage":     "Usage: yomihon coverage [--root <dir>] [--format json|human|md]\n",
	"exists":       "Usage: yomihon exists [--root <dir>] [--format json|human|md] <name>\n",
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
