package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The server writes data-* attributes and the client reads them; between the
// two there was nothing at all — no list, no type, no check — so renaming one
// broke a page in a way only a browser could notice, and the repository has
// already lost a client module to exactly that kind of silence.
//
// This is the list, derived rather than kept: every name is read out of the
// sources on both sides, so it cannot fall behind them.

// dataName matches one data-* attribute name written out in full.
var dataName = regexp.MustCompile(`\bdata-[a-z0-9]+(?:-[a-z0-9]+)*`)

// datasetName matches the other spelling the client uses for the same thing:
// element.dataset.freshnessPath is the attribute data-freshness-path.
var datasetName = regexp.MustCompile(`dataset\.([a-zA-Z0-9]+)`)

// clientOwned are the attributes the client sets on elements it created or on
// the document root, so no server template ever writes one. Each is a piece of
// runtime state rather than a fact about a note: whether scripting arrived,
// whether the voice is available or speaking, whether the drawer is open,
// whether the outline is mid-travel, what a freshness notice currently says,
// whether a diagram failed to draw, which speaking rate a button stands for on
// a toolbar the client builds, and the label a speak button wore before the
// client borrowed it — kept on the button so the client has one fewer copy of
// a sentence the server already wrote there.
var clientOwned = []string{
	"data-freshness",
	"data-js",
	"data-mermaid-error",
	"data-nav",
	"data-readaloud-idle",
	"data-speaking",
	"data-speech",
	"data-speech-rate",
	"data-traveling",
}

// TestEveryDataAttributeCrossesTheGapWithAReaderOnTheOtherSide holds the
// contract between the served HTML and the client. A name the server writes
// that nobody reads is either a rename that landed on one side only or a hook
// nothing ever used; a name the client reads that nobody writes is a page that
// silently does nothing.
func TestEveryDataAttributeCrossesTheGapWithAReaderOnTheOtherSide(t *testing.T) {
	t.Parallel()

	const repoRoot = "../../.."
	emitted := dataAttributesIn(t, "the server's own sources",
		goSources(t, filepath.Join(repoRoot, "internal")),
		filesUnder(t, filepath.Join(repoRoot, "internal"), ".templ"))
	script := clientDataAttributes(t, filepath.Join(repoRoot, "assets", "js"))
	readers := dataAttributesIn(t, "everything that reads them",
		filesUnder(t, filepath.Join(repoRoot, "assets", "js"), ".js"),
		filesUnder(t, filepath.Join(repoRoot, "assets", "css"), ".css"),
		filesUnder(t, filepath.Join(repoRoot, ".github", "e2e"), ".mjs"),
		testSources(t, filepath.Join(repoRoot, "internal")),
		testSources(t, filepath.Join(repoRoot, "cmd")))
	for name := range script {
		readers[name] = struct{}{}
	}

	// A sentinel on each side, so a scan that read the wrong tree fails here
	// rather than reporting that everything agrees.
	for _, present := range []struct {
		set  map[string]struct{}
		name string
		side string
	}{
		{emitted, "data-freshness-path", "the server's sources"},
		{readers, "data-freshness-path", "the client and the probes"},
		{script, "data-nav-toggle", "the client scripts"},
	} {
		if _, ok := present.set[present.name]; !ok {
			t.Fatalf("%s does not mention %s, so this scan is reading the wrong files", present.side, present.name)
		}
	}
	if len(emitted) < 50 || len(readers) < 50 {
		t.Fatalf("scanned %d written and %d read attribute names; both sides should be in the dozens, so a scan this thin found the wrong tree", len(emitted), len(readers))
	}

	for _, name := range sortedKeys(emitted) {
		if _, read := readers[name]; !read {
			t.Errorf("%s is written by the server and read by nothing: no script, no stylesheet, no probe, no test", name)
		}
	}
	for _, name := range sortedKeys(script) {
		if _, written := emitted[name]; written || slices.Contains(clientOwned, name) {
			continue
		}
		t.Errorf("%s is read by a client script and written by nothing: add it where the page is rendered, or name it in clientOwned with the reason it is the client's", name)
	}
	for _, name := range clientOwned {
		if _, written := emitted[name]; written {
			t.Errorf("%s is named as the client's own and the server writes it too", name)
		}
		if _, read := script[name]; !read {
			t.Errorf("%s is named as the client's own and no script mentions it", name)
		}
	}
}

// dataAttributesIn gathers every data-* name written out in the given files.
func dataAttributesIn(t *testing.T, what string, groups ...[]string) map[string]struct{} {
	t.Helper()
	found := map[string]struct{}{}
	files := slices.Concat(groups...)
	if len(files) == 0 {
		t.Fatalf("no files were scanned for %s", what)
	}
	for _, path := range files {
		for _, name := range dataName.FindAllString(readSource(t, path), -1) {
			found[name] = struct{}{}
		}
	}
	return found
}

// clientDataAttributes gathers what the scripts read, in both spellings.
func clientDataAttributes(t *testing.T, dir string) map[string]struct{} {
	t.Helper()
	found := map[string]struct{}{}
	for _, path := range filesUnder(t, dir, ".js") {
		source := readSource(t, path)
		for _, name := range dataName.FindAllString(source, -1) {
			found[name] = struct{}{}
		}
		for _, match := range datasetName.FindAllStringSubmatch(source, -1) {
			found["data-"+camelToKebab(match[1])] = struct{}{}
		}
	}
	return found
}

func camelToKebab(property string) string {
	var b strings.Builder
	for _, r := range property {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('-')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// filesUnder lists every file below dir carrying the extension.
func filesUnder(t *testing.T, dir, extension string) []string {
	t.Helper()
	var found []string
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == extension {
			found = append(found, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walking %s for %s: %v", dir, extension, err)
	}
	if len(found) == 0 {
		t.Fatalf("no %s file under %s", extension, dir)
	}
	return found
}

// goSources lists the hand-written, non-test Go below dir. Generated files are
// left out because they are a projection of the templates already scanned.
func goSources(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	for _, path := range filesUnder(t, dir, ".go") {
		name := filepath.Base(path)
		if strings.HasSuffix(name, "_templ.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		found = append(found, path)
	}
	return found
}

func testSources(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	for _, path := range filesUnder(t, dir, ".go") {
		if strings.HasSuffix(filepath.Base(path), "_test.go") {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		t.Fatalf("no test file under %s", dir)
	}
	return found
}

func readSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a path this test listed out of the repository's own tree
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(data)
}

func sortedKeys(set map[string]struct{}) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
