package archlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestContributionGuideStatesTestPrerequisites keeps the compiler, test user,
// and JSON reader requirements at both starting points for contributors. Each
// fact belongs where the command is introduced, before a failed run is the
// reader's first account of the environment it needs.
func TestContributionGuideStatesTestPrerequisites(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(repoRoot, "CONTRIBUTING.md"))
	if err != nil {
		t.Fatalf("read contributor guide: %v", err)
	}
	guide := strings.ReplaceAll(string(data), "\r\n", "\n")
	t.Log("invoked: contributor guide test prerequisites")
	if strings.Contains(guide, "`make test` needs only a Go toolchain") {
		t.Error("caught: contributor guide says race-enabled permission tests need only Go")
	}

	for _, tt := range []struct {
		name  string
		start string
		end   string
	}{
		{name: "build", start: "## Build and run it\n", end: "## Run the tests\n"},
		{name: "gate", start: "### The gate\n", end: "## File an issue\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if strings.Count(guide, tt.start) != 1 || strings.Count(guide, tt.end) != 1 {
				t.Fatalf("caught: contributor guide has no unique %q section ending at %q", tt.start, tt.end)
			}
			_, rest, _ := strings.Cut(guide, tt.start)
			section, _, found := strings.Cut(rest, tt.end)
			if !found {
				t.Fatalf("caught: contributor guide section %q ends before it starts", tt.name)
			}
			// Read the whole declarations: a list of keywords would also
			// accept a sentence saying the required tools are unnecessary.
			for _, requirement := range []struct {
				name string
				text string
			}{
				{name: "race compiler", text: "- **Race-enabled tests:** On Linux and Windows, `make test` needs cgo enabled and a C compiler for the race detector; macOS needs neither for that detector."},
				{name: "permission fixture user", text: "- **Test user:** Run the tests as an unprivileged user: some permission fixtures fail rather than skip when the process can bypass file permissions."},
				{name: "gate JSON reader", text: "- **Gate JSON:** The full gate also needs `jq` to read its JSON contract."},
			} {
				prefix, _, _ := strings.Cut(requirement.text, ":** ")
				var declarations []string
				for paragraph := range strings.SplitSeq(section, "\n\n") {
					declaration := strings.Join(strings.Fields(paragraph), " ")
					if strings.HasPrefix(declaration, prefix+":** ") {
						declarations = append(declarations, declaration)
					}
				}
				if diff := cmp.Diff([]string{requirement.text}, declarations); diff != "" {
					t.Errorf("caught: contributor %s section differs for %s declarations (-want +got):\n%s", tt.name, requirement.name, diff)
				}
			}
		})
	}
}
