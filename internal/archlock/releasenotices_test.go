package archlock

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// noticeFileName matches the files at the repository root that carry licence
// terms the binaries owe their readers: the project's own licence and the
// notices of what is compiled into it. The set a release has to publish is read
// from the root rather than spelled out here, so a notice file added tomorrow
// is held to the same rule as the two that exist today.
var noticeFileName = regexp.MustCompile(`(?i)licen[cs]e|notice`)

// releaseWorkflowCommands returns the commands of the release workflow in file
// order, as the fields of each non-comment line. A comment that names a file is
// not a command that copies it, so comment lines are dropped; the rest is read
// line by line, which is how the workflow's own shell reads it.
func releaseWorkflowCommands(t *testing.T) [][]string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "release.yml")) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatalf("read the release workflow: %v", err)
	}
	var commands [][]string
	for line := range strings.Lines(string(data)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		commands = append(commands, strings.Fields(trimmed))
	}
	return commands
}

// copiesInto reports whether a command is `cp` with dir as its destination, and
// which names it copies. Flags are not names.
func copiesInto(command []string, dir string) (names []string, ok bool) {
	if len(command) < 3 || command[0] != "cp" {
		return nil, false
	}
	if dest := strings.TrimSuffix(command[len(command)-1], "/"); dest != dir {
		return nil, false
	}
	for _, field := range command[1 : len(command)-1] {
		if !strings.HasPrefix(field, "-") {
			names = append(names, filepath.Base(field))
		}
	}
	return names, true
}

// TestReleasePublishesTheLicenceNotices holds the release to what the
// binaries owe: the notices reach the release page beside the binaries. The
// workflow publishes whatever dist/ holds, so the property is that every notice
// file is copied into dist/, that it is there before the sums are written (so
// that SHA256SUMS covers what the release publishes), and that the publish step
// still takes the whole directory.
func TestReleasePublishesTheLicenceNotices(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		t.Fatalf("read the repository root: %v", err)
	}
	var notices []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && noticeFileName.MatchString(entry.Name()) {
			notices = append(notices, entry.Name())
		}
	}
	// A root that lost its licence files would report nothing missing from the
	// release; the two the release has to carry are named so that cannot happen.
	for _, want := range []string{"LICENSE", "THIRD_PARTY_NOTICES.md"} {
		if !slices.Contains(notices, want) {
			t.Fatalf("the repository root has no %s; found %q", want, notices)
		}
	}

	commands := releaseWorkflowCommands(t)
	copied := make(map[string]int) // notice file -> index of the command that copies it into dist/
	sums, publish := -1, -1
	for i, command := range commands {
		if names, ok := copiesInto(command, "dist"); ok {
			for _, name := range names {
				if _, seen := copied[name]; !seen {
					copied[name] = i
				}
			}
		}
		if sums < 0 && slices.Contains(command, "sha256sum") {
			sums = i
		}
		if publish < 0 && slices.Contains(command, "release") && slices.Contains(command, "create") && slices.Contains(command, "dist/*") {
			publish = i
		}
	}
	if sums < 0 {
		t.Fatal("the release workflow no longer writes SHA256SUMS, so there is nothing to order the notices against")
	}
	if publish < 0 {
		t.Error("the release workflow's `gh release create` no longer takes dist/*, so it does not publish what dist/ holds")
	}
	for _, name := range notices {
		at, ok := copied[name]
		switch {
		case !ok:
			t.Errorf("the release workflow never copies %s into dist/, so the release publishes it nowhere", name)
		case at > sums:
			t.Errorf("the release workflow copies %s into dist/ after it writes SHA256SUMS, so the sums do not list it", name)
		}
	}
}
