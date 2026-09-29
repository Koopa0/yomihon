package archlock

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	modulepath "golang.org/x/mod/module"
)

// TestTrackedPathsAreValidInAModuleZip holds the release path open: `go
// install pkg@version` builds the module zip from the tracked tree, and the zip
// refuses a whole module when any one path in it is malformed (a fullwidth
// colon, a reserved character, a Windows-reserved name). A fixture that needs
// such a name is written at test time instead of being tracked.
func TestTrackedPathsAreValidInAModuleZip(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not available: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), "git", "-c", "core.quotepath=off", "ls-files", "-z")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("no git work tree to list: %v: %s", err, strings.TrimSpace(stderr.String()))
	}

	var bad []string
	for path := range strings.SplitSeq(string(out), "\x00") {
		if path == "" {
			continue
		}
		if err := modulepath.CheckFilePath(path); err != nil {
			bad = append(bad, path+": "+err.Error())
		}
	}
	if len(bad) > 0 {
		t.Errorf("%d tracked path(s) are invalid in a module zip, so `go install module@version` fails:\n%s",
			len(bad), strings.Join(bad, "\n"))
	}
}
