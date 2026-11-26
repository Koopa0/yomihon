package machinemarks

import (
	_ "embed" // the program a reader runs by hand is the one the tests run
	"strings"
)

// readMarksJq is the program that reads the machine files with no help from
// Yomihon, as the tests run it against what Yomihon reads.
//
//go:embed read-marks.jq
var readMarksJq string

// ReadMarksCommand is a command that prints every mark of the machine files of a
// vault, run in the vault's own folder, which a reader without Yomihon can run
// with jq. It is the program the tests hold to the screen, its comments
// removed, so that it stands on one command line: it has no single quote.
func ReadMarksCommand() string {
	var program []string
	for line := range strings.SplitSeq(readMarksJq, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		program = append(program, line)
	}
	return "jq -c -n -R '" + strings.TrimSpace(strings.Join(program, "\n")) + "' reader-notes/.yomihon/marks/*.jsonl"
}
