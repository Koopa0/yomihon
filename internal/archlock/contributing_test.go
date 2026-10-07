package archlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// TestContributionGuideStatesTestPrerequisites keeps one declaration of the
// compiler, test user, and JSON reader requirements in the build section. The
// gate points back to them so a second copy cannot drift from the first.
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
	requirements := []struct {
		name string
		text string
	}{
		{name: "race compiler", text: "- **Race-enabled tests:** On Linux and Windows, `make test` needs cgo enabled and a C compiler for the race detector; macOS needs neither for that detector."},
		{name: "permission fixture user", text: "- **Test user:** Run the tests as an unprivileged user: some permission fixtures fail rather than skip when the process can bypass file permissions."},
		{name: "gate JSON reader", text: "- **Gate JSON:** The full gate also needs `jq` to read its JSON contract."},
	}
	normalizedGuide := strings.Join(strings.Fields(guide), " ")
	for _, requirement := range requirements {
		prefix, _, _ := strings.Cut(requirement.text, ":** ")
		if count := strings.Count(normalizedGuide, prefix+":** "); count != 1 {
			t.Errorf("caught: contributor guide has %d %s declarations, want 1", count, requirement.name)
		}
	}
	previousHeading := -1
	for _, heading := range []string{
		"## Build and run it\n",
		"## Run the tests\n",
		"### The gate\n",
		"## File an issue\n",
	} {
		position := strings.Index(guide, heading)
		if position <= previousHeading {
			t.Errorf("caught: contributor guide heading %q is missing or out of prerequisite order", heading)
			break
		}
		previousHeading = position
	}

	for _, tt := range []struct {
		name  string
		start string
		end   string
		level int
	}{
		{name: "build", start: "## Build and run it\n", end: "## Run the tests\n", level: 2},
		{name: "gate", start: "### The gate\n", end: "## File an issue\n", level: 3},
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
			document := goldmark.DefaultParser().Parse(text.NewReader([]byte(section)))
			for node := document.FirstChild(); node != nil; node = node.NextSibling() {
				if heading, ok := node.(*ast.Heading); ok && heading.Level <= tt.level {
					t.Fatalf("caught: contributor %s section contains an intervening level-%d heading", tt.name, heading.Level)
				}
			}
			if tt.name == "gate" {
				pointer := "The test prerequisites above also apply to `make verify`."
				var pointers []string
				for paragraph := range strings.SplitSeq(section, "\n\n") {
					declaration := strings.Join(strings.Fields(paragraph), " ")
					if declaration == pointer {
						pointers = append(pointers, declaration)
					}
				}
				if diff := cmp.Diff([]string{pointer}, pointers); diff != "" {
					t.Errorf("caught: contributor gate section differs for prerequisite pointer (-want +got):\n%s", diff)
				}
				return
			}
			// Read the whole declarations: a list of keywords would also
			// accept a sentence saying the required tools are unnecessary.
			for _, requirement := range requirements {
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
