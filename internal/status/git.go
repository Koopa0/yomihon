package status

import (
	"fmt"
	"io"
	"strings"
)

const gitChildCommand = "__yomihon-status-git"

// RunGitChild recognizes and runs status's private same-binary git child.
// Command entry points and test TestMain functions must call it before ordinary
// argument parsing. The child can only execute git and requires an inherited
// directory descriptor supplied by its parent.
func RunGitChild(args []string, stderr io.Writer) (handled bool, exit int) {
	if len(args) == 0 || args[0] != gitChildCommand {
		return false, 0
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if len(args) == 1 {
		if _, err := fmt.Fprintln(stderr, "yomihon: status git child: missing git arguments"); err != nil {
			return true, 1
		}
		return true, 1
	}
	if err := execGitChild(args[1:]); err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "yomihon: status git child: %v\n", err); writeErr != nil {
			return true, 1
		}
		return true, 1
	}
	return true, 0
}

func gitChildEnvironment(environ []string) []string {
	clean := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") || strings.HasPrefix(name, "YOMIHON_") {
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}
