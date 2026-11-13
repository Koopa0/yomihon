package main

import "fmt"

const topLevelHelp = `Usage:
  yomihon [<dir>]                       read a folder (default: this one)
  yomihon serve [<dir>]                 read a folder (or --root <dir>)

About the vault:
  yomihon check [options] [path...]      check a vault (--root <dir>; path narrows)
  yomihon coverage [options]
  yomihon exists [options] <name>

About the reader:
  yomihon ` + openCommand + ` [options]
  yomihon ` + approveFigureCommand + ` [--root <dir>] <path>   let a figure's scripts run
  yomihon ` + revokeFigureCommand + ` [--root <dir>] <path>    withdraw that

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
		"means another yomihon, on another port.\n" +
		"\n" +
		"A file larger than 1 MiB is not read as characters: the page names the\n" +
		"bytes and search leaves it out. An in-place rewrite that keeps the same\n" +
		"inode, mode, size and mtime can stay stale for about an hour; an ordinary\n" +
		"save that changes size or mtime rebuilds at once.\n",
	"check": "Usage: yomihon check [--root <dir>] [--format json|human|md] [--all] [--deny <severity|rule-id>]... [--baseline <file>] [path...]\n" +
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
		"When folders.checked names directories, the frontmatter schema rules\n" +
		"judge those files; a file outside them is still scanned for links, but is\n" +
		"not held to the schema. When the list is omitted or empty, every note is\n" +
		"held to the schema.\n" +
		"\n" +
		"The default check keeps findings that touch that same layer; --all restores\n" +
		"findings that only touch paths outside it. Privacy is a different cut: a\n" +
		"withheld path is never reported, even with --all.\n" +
		"\n" +
		"Writes one JSON object per line when the output is not a terminal, and a\n" +
		"human summary when it is. --format decides instead of the terminal.\n" +
		"\n" +
		"A file the scan saw and the read could not open is reported as an error\n" +
		"finding named scan.unreadable, and the rest of the vault is judged. The\n" +
		"rules that conclude something is nowhere in the vault report nothing for\n" +
		"that run, since the file nobody read may hold it, and the finding says so.\n" +
		"\n" +
		"Exits 0 when nothing named by --deny was found, 1 when something was, and\n" +
		"2 when the command itself could not run. Findings alone do not fail the\n" +
		"command: without --deny it reports and exits 0, an unreadable file\n" +
		"included.\n",
	"coverage": "Usage: yomihon coverage [--root <dir>] [--format json|human|md]\n" +
		"\n" +
		"Writes a compact JSON object when the output is not a terminal, and a\n" +
		"human summary when it is; --format decides instead of the terminal, and\n" +
		"md falls back to the human view.\n" +
		"\n" +
		"When folders.checked names directories, only that layer is tallied.\n" +
		"A concept or map outside it cannot change a public concept's mount\n" +
		"state. When the list is omitted or empty, the whole vault is counted.\n" +
		"A withheld path is never reported.\n" +
		"\n" +
		"A census is an answer about the whole vault, so a file the read could not\n" +
		"open stops this command where it lets check report and go on: there is no\n" +
		"count that means \"except for the part I could not read\".\n" +
		"\n" +
		"Exits 0 — coverage reports state, it never gates — and 2 when the\n" +
		"command itself could not run, an unreadable file included.\n",
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
		"A note that answers still answers when some other file could not be read.\n" +
		"\"None does\" is the answer that needs the whole vault, so where a read\n" +
		"failed it is refused with 2 rather than given, and nothing gates a write\n" +
		"on an absence nobody could establish.\n" +
		"\n" +
		"A note inside a directory the contract withholds from agent-facing output\n" +
		"is never described here — no path, no matched field. It still answers:\n" +
		"the exit stays 0, so a write-if-absent gated on it does not create a\n" +
		"second note under a withheld note's own name.\n",
	openCommand: "Usage: yomihon " + openCommand + " [--root <dir>] [--format json|human|md]\n" +
		"\n" +
		"Lists what you left open in a vault: the locations you marked \"Not sure\n" +
		"yet\", and the notes of the contract's answer type that still declare\n" +
		"draft: true — newest first, as the reading desk shows them.\n" +
		"\n" +
		"Marks are kept on this machine, one set per folder, found by the folder's\n" +
		"path as it is spelled: this reads the set that yomihon serve kept for the\n" +
		"same folder. A mark whose note or heading is gone is still listed, with lost\n" +
		"saying which and its anchor left empty; nothing here removes it. A mark in a\n" +
		"folder the contract leaves out of the scan is not listed.\n" +
		"\n" +
		"Writes one JSON object per line when the output is not a terminal, and a\n" +
		"human list when it is; --format decides instead of the terminal, and md\n" +
		"falls back to the human view.\n" +
		"\n" +
		"Nothing under a directory the contract withholds from agent-facing output\n" +
		"is listed, nor an item whose title or heading links there in any link\n" +
		"syntax or spells a withheld directory's name and a slash, read past case,\n" +
		"fullwidth forms, character references, percent-escapes, and characters\n" +
		"that draw nothing or only mark another. A look-alike letter of another\n" +
		"script is not recognised. Plain words that only share a withheld note's\n" +
		"file name are listed as written.\n" +
		"Nothing says how much was left out. When no location is listed, one line\n" +
		"on stderr says only whether a marks file was found. The contract has to\n" +
		"declare [privacy] before anything is listed at all.\n" +
		"\n" +
		"A list that leaves out what could not be read would read as complete, so an\n" +
		"unreadable marks file stops this command, and so does a note outside\n" +
		"withheld ground that could not be read or whose frontmatter could not be.\n" +
		"That is any such note, whatever its type: the command names it, and once\n" +
		"it is mended the command runs again.\n" +
		"\n" +
		"Exits 0 with the list, an empty one included, and 2 when the command itself\n" +
		"could not run, an unreadable marks file or note included.\n",
	approveFigureCommand: "Usage: yomihon " + approveFigureCommand + " [--root <dir>] <path>\n" +
		"\n" +
		"Approves one figure, by its exact bytes, to run its scripts on the reading\n" +
		"page. <path> is the .html file as the vault spells it, relative to its root,\n" +
		"such as \"Notes/figures/replication.html\".\n" +
		"\n" +
		"Shows what it is about to approve, then asks you to type the first 8\n" +
		"characters of its SHA-256. The record is kept on this machine, outside the\n" +
		"vault. Any change to the file, one byte included, lapses the approval, and the\n" +
		"figure goes back to running without its scripts.\n" +
		"\n" +
		"An approved figure's scripts run in your browser and can reach the network;\n" +
		"yomihon itself still makes no network request.\n" +
		"\n" +
		"Needs a person at a terminal: when standard input or output is not a\n" +
		"terminal it approves nothing. Exits 0 when the figure was approved, 1 when\n" +
		"you did not confirm or there is nothing to approve, and 2 when the command\n" +
		"itself could not run.\n",
	revokeFigureCommand: "Usage: yomihon " + revokeFigureCommand + " [--root <dir>] <path>\n" +
		"\n" +
		"Removes the approval for one figure, so its scripts stop running the next\n" +
		"time its page is loaded. The file itself is not touched. A figure with no\n" +
		"approval is reported as such, and the command still exits 0.\n" +
		"\n" +
		"Exits 0, and 2 when the command itself could not run.\n",
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
