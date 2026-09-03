package pages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// templSourceRoot is the one directory this repository generates templates
// from, reached from the package the test runs in.
const templSourceRoot = ".."

// leastTemplSources is a floor, not a count: a scan that walked the wrong
// directory finds nothing and would otherwise report that nothing is wrong.
// The number is under today's total so adding a template does not fail this,
// and far enough above zero that a broken path cannot pass.
const leastTemplSources = 13

// TestNoGoIsWrittenInsideATemplate keeps hand-written Go out of the template
// sources. Go placed in a .templ file reaches the compiler only inside the
// generated output, which carries the header every linter in this repository
// is configured to skip — so a function written there is read by none of them
// and measured by nothing but a coverage total that counts generated
// statements. A template holds markup; its sibling .go file holds the Go the
// markup calls.
func TestNoGoIsWrittenInsideATemplate(t *testing.T) {
	t.Parallel()

	examined := 0
	for _, dir := range templSourceDirs(t) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".templ" {
				continue
			}
			examined++
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path) // #nosec G304 -- a template name read out of this repository's own source tree
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", path, err)
			}
			for number, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "func ") {
					t.Errorf("%s:%d declares Go inside a template, where no linter reads it: %s\n\tmove it to the sibling .go file of the same package",
						filepath.ToSlash(path), number+1, strings.TrimSpace(line))
				}
			}
		}
	}
	if examined < leastTemplSources {
		t.Fatalf("read %d template sources under %s, want at least %d — the scan is looking in the wrong place and would report nothing whatever the sources said",
			examined, templSourceRoot, leastTemplSources)
	}
}

// templSourceDirs lists every package under the template root, so a package
// added beside this one is scanned without anyone remembering to name it here.
func templSourceDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(templSourceRoot)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", templSourceRoot, err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(templSourceRoot, entry.Name()))
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("%s holds no package directories, so this scan reads nothing", templSourceRoot)
	}
	return dirs
}
