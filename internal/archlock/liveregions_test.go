package archlock

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// These are declarations, not files allowed to contain arbitrary live regions.
// The template function and element class, or the JS receiver and the element
// it creates, identify each owner. Search counts, Japanese practice, speech
// controls and freshness have their own contracts; only action replies share
// Reply. Missing declarations fail just as added ones do.
var liveRegionOwners = []struct {
	path, owner string
	attributes  []string
}{
	{"internal/ui/layouts/reply.templ", "Reply/p.y-reply", []string{"role=status", "aria-live=polite"}},
	{"internal/ui/layouts/livesearch.templ", "LiveSearchStatus/p.y-live-search__status", []string{"role=status", "aria-live=polite"}},
	{"internal/ui/pages/slotmachine.templ", "slotCard/p.y-slotlive.y-offscreen", []string{"role=status", "aria-live=polite"}},
	{"assets/js/lesson.js", "speechStatus/span.y-ttsbar__status", []string{"aria-live=polite"}},
	{"assets/js/freshness.js", "banner/p.y-freshness", []string{"role=status"}},
}

var (
	liveRegionToken = regexp.MustCompile(`(?i)\b(?:role|aria-live|ariaLive)\b`)
	liveElement     = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9:-]*)\b[^<>]*>`)
	liveAttribute   = regexp.MustCompile(`(?i)\b(role|aria-live)\s*=\s*["']([^"']*)["']`)
	liveClass       = regexp.MustCompile(`\bclass\s*=\s*"([^"]*)"`)
	liveReplyClass  = regexp.MustCompile(`\bclass\s*=\s*\{\s*"y-reply "\s*\+\s*class\s*\}`)
	liveTempl       = regexp.MustCompile(`(?m)^templ\s+(\w+)\s*\(`)
	liveJSAttribute = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\.setAttribute\(\s*["']((?i:role|aria-live))["']\s*,\s*["']([^"']*)["']\s*\)`)
)

func TestLiveRegionOwnershipInventory(t *testing.T) {
	t.Parallel()
	want := make(map[string]int)
	for _, owner := range liveRegionOwners {
		for _, attribute := range owner.attributes {
			want[owner.path+" "+owner.owner+" "+attribute]++
		}
	}
	found := make(map[string]int)
	elements := make(map[string]int)
	for _, path := range liveRegionSourceFiles(t) {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- repository paths enumerated above
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		scanLiveRegionDeclarations(t, path, withoutLiveRegionComments(string(data)), found, elements)
	}
	// The two attributes must belong to one element. Two lookalike elements
	// with one attribute each cannot satisfy a single owner's declaration.
	for _, owner := range liveRegionOwners {
		if strings.HasSuffix(owner.path, ".templ") {
			key := owner.path + " " + owner.owner
			if elements[key] != 1 {
				t.Errorf("live-region owner %s has %d elements, want exactly one", key, elements[key])
			}
		}
	}
	for key, count := range found {
		if count != want[key] {
			t.Errorf("added live-region declaration: %s (found %d, want %d)", key, count, want[key])
		}
	}
	for key, count := range want {
		if found[key] < count {
			t.Errorf("missing live-region declaration: %s (found %d, want %d)", key, found[key], count)
		}
	}
}

// Every template under internal/ui participates, including any future fixture
// subdirectory. The declaration inventory must not silently inherit the Go
// source walk's testdata/hidden-directory exclusions.
func liveRegionSourceFiles(t *testing.T) []string {
	t.Helper()
	paths := clientModules(t)
	root := filepath.Join(repoRoot, "internal/ui")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".templ" {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk every UI template: %v", err)
	}
	slices.Sort(paths)
	return paths
}

// Ignore only standalone line comments. All other mentions must be read as a
// literal declaration; a new dynamic spelling, property assignment, attribute
// map or unfamiliar source shape fails closed instead of escaping the scan.
func withoutLiveRegionComments(source string) string {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			lines[i] = strings.Repeat(" ", len(line))
		}
	}
	return strings.Join(lines, "\n")
}

func scanLiveRegionDeclarations(t *testing.T, path, source string, found, elements map[string]int) {
	t.Helper()
	covered := make([]bool, len(source))
	record := func(start, end int, owner, attribute, value string) {
		for i := start; i < end; i++ {
			covered[i] = true
		}
		attribute = strings.ToLower(attribute)
		if attribute == "role" && !slices.Contains(strings.Fields(strings.ToLower(value)), "status") {
			return
		}
		found[path+" "+owner+" "+attribute+"="+value]++
	}
	for _, element := range liveElement.FindAllStringSubmatchIndex(source, -1) {
		tag := source[element[0]:element[1]]
		owner := liveTemplateOwner(source[:element[0]]) + "/" + source[element[2]:element[3]] + "." + liveElementClass(tag)
		elements[path+" "+owner]++
		for _, attribute := range liveAttribute.FindAllStringSubmatchIndex(tag, -1) {
			record(element[0]+attribute[0], element[0]+attribute[1], owner, tag[attribute[2]:attribute[3]], tag[attribute[4]:attribute[5]])
		}
	}
	for _, attribute := range liveJSAttribute.FindAllStringSubmatchIndex(source, -1) {
		receiver := source[attribute[2]:attribute[3]]
		name := source[attribute[4]:attribute[5]]
		value := source[attribute[6]:attribute[7]]
		owner := liveJSOwner(source[:attribute[0]], receiver)
		record(attribute[0], attribute[1], owner, name, value)
	}
	for _, token := range liveRegionToken.FindAllStringIndex(source, -1) {
		if !covered[token[0]] {
			line := 1 + strings.Count(source[:token[0]], "\n")
			t.Errorf("%s:%d unreadable live-region declaration near %q; name its element and use a literal attribute", path, line, source[token[0]:token[1]])
		}
	}
}

func liveTemplateOwner(before string) string {
	declarations := liveTempl.FindAllStringSubmatch(before, -1)
	if len(declarations) == 0 {
		return "unowned"
	}
	return declarations[len(declarations)-1][1]
}

func liveElementClass(tag string) string {
	if liveReplyClass.MatchString(tag) {
		return "y-reply"
	}
	if class := liveClass.FindStringSubmatch(tag); class != nil {
		return strings.Join(strings.Fields(class[1]), ".")
	}
	return "unowned"
}

// Adjacent creation/class declarations identify the actual element receiving
// the attribute, not merely its file. Moving it to another receiver fails even
// when the file's total number of live declarations stays unchanged.
func liveJSOwner(before, receiver string) string {
	name := regexp.QuoteMeta(receiver)
	creation := regexp.MustCompile(name + `\s*=\s*document\.createElement\(\s*["']([^"']+)["']\s*\);\s*` + name + `\.className\s*=\s*["']([^"']+)["'];\s*$`)
	if element := creation.FindStringSubmatch(before); element != nil {
		return fmt.Sprintf("%s/%s.%s", receiver, element[1], strings.Join(strings.Fields(element[2]), "."))
	}
	return receiver + "/unowned"
}
