package archlock

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// These are declarations, not files allowed to contain arbitrary live regions.
// The template function and element class, or the JS receiver and the element
// it creates, identify each owner. Search counts, Japanese practice, speech
// controls and freshness have their own contracts; only action replies share
// Reply. Missing declarations fail just as added ones do.
type liveRegionOwner struct {
	path, owner string
	attributes  []string
}

var liveRegionOwners = []liveRegionOwner{
	{"internal/ui/layouts/reply.templ", "Reply/p.y-reply", []string{"role=status", "aria-live=polite"}},
	{"internal/ui/layouts/livesearch.templ", "LiveSearchStatus/p.y-live-search__status", []string{"role=status", "aria-live=polite"}},
	{"internal/ui/pages/slotmachine.templ", "slotCard/p.y-slotlive.y-offscreen", []string{"role=status", "aria-live=polite"}},
	{"assets/js/lesson.js", "speechStatus/span.y-ttsbar__status", []string{"aria-live=polite"}},
	{"internal/ui/pages/thought.templ", "Thought/p.y-thought__said", []string{"role=status"}},
	{"assets/js/uncertainty.js", "said/p.y-uncertainty__said", []string{"role=status"}},
	{"assets/js/uncertainty.js", "sectionSaid/p.y-uncertainty__said", []string{"role=status"}},
	{"internal/ui/pages/shelf.templ", "shelfRemoval/p.y-uncertainty__said", []string{"role=status"}},
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
	sources := make(map[string]string)
	for _, path := range liveRegionSourceFiles(t) {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- repository paths enumerated above
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sources[path] = string(data)
	}
	for _, problem := range liveRegionInventoryProblems(liveRegionOwners, sources) {
		t.Error(problem)
	}
}

// liveRegionInventoryProblems compares the live regions declared in sources,
// keyed by repository path, with the owners' declarations and reports every
// difference: an element that holds too few or too many, a declaration nobody
// owns, a declaration an owner lost, and a mention the scan cannot read.
func liveRegionInventoryProblems(owners []liveRegionOwner, sources map[string]string) []string {
	want := make(map[string]int)
	for _, owner := range owners {
		for _, attribute := range owner.attributes {
			want[owner.path+" "+owner.owner+" "+attribute]++
		}
	}
	found := make(map[string]int)
	elements := make(map[string]int)
	var problems []string
	for _, path := range slices.Sorted(maps.Keys(sources)) {
		problems = append(problems, scanLiveRegionDeclarations(path, withoutLiveRegionComments(sources[path]), found, elements)...)
	}
	// The two attributes must belong to one element. Two lookalike elements
	// with one attribute each cannot satisfy a single owner's declaration.
	for _, owner := range owners {
		if strings.HasSuffix(owner.path, ".templ") {
			key := owner.path + " " + owner.owner
			if elements[key] != 1 {
				problems = append(problems, fmt.Sprintf("live-region owner %s has %d elements, want exactly one", key, elements[key]))
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(found)) {
		if found[key] != want[key] {
			problems = append(problems, fmt.Sprintf("added live-region declaration: %s (found %d, want %d)", key, found[key], want[key]))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(want)) {
		if found[key] < want[key] {
			problems = append(problems, fmt.Sprintf("missing live-region declaration: %s (found %d, want %d)", key, found[key], want[key]))
		}
	}
	return problems
}

// Every template under internal/ui participates, including any future fixture
// subdirectory. The declaration inventory must not silently inherit the Go
// source walk's testdata/hidden-directory exclusions.
func liveRegionSourceFiles(t *testing.T) []string {
	t.Helper()
	paths := clientModules(t)
	root := filepath.Join(repoRoot, "internal", "ui")
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

func scanLiveRegionDeclarations(path, source string, found, elements map[string]int) []string {
	var problems []string
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
			problems = append(problems, fmt.Sprintf("%s:%d unreadable live-region declaration near %q; name its element and use a literal attribute", path, line, source[token[0]:token[1]]))
		}
	}
	return problems
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

const (
	syntheticReplyTemplate = "templ Reply() {\n\t<p class=\"y-reply\" role=\"status\" aria-live=\"polite\"></p>\n}\n"
	syntheticSpeechScript  = "const speechStatus = document.createElement('span');\nspeechStatus.className = 'y-tts';\nspeechStatus.setAttribute('aria-live', 'polite');\n"
)

var syntheticLiveRegionOwners = []liveRegionOwner{
	{"reply.templ", "Reply/p.y-reply", []string{"role=status", "aria-live=polite"}},
	{"speech.js", "speechStatus/span.y-tts", []string{"aria-live=polite"}},
}

// TestLiveRegionInventoryRejectsAddedAndMissingDeclarations feeds the
// inventory synthetic sources, so no repository file has to be edited to show
// that it fails closed in each direction.
func TestLiveRegionInventoryRejectsAddedAndMissingDeclarations(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		change func(sources map[string]string)
		want   []string
	}{
		{
			name:   "the declared regions alone",
			change: func(map[string]string) {},
			want:   nil,
		},
		{
			name: "a template gains a live region",
			change: func(sources map[string]string) {
				sources["probe.templ"] = "templ Probe() {\n\t<p class=\"y-probe\" role=\"status\"></p>\n}\n"
			},
			want: []string{"added live-region declaration: probe.templ Probe/p.y-probe role=status (found 1, want 0)"},
		},
		{
			name: "a script gains a live region",
			change: func(sources map[string]string) {
				sources["probe.js"] = "const probe = document.createElement('p');\nprobe.className = 'y-probe';\nprobe.setAttribute('aria-live', 'polite');\n"
			},
			want: []string{"added live-region declaration: probe.js probe/p.y-probe aria-live=polite (found 1, want 0)"},
		},
		{
			name: "a template loses its role",
			change: func(sources map[string]string) {
				sources["reply.templ"] = strings.Replace(sources["reply.templ"], ` role="status"`, "", 1)
			},
			want: []string{"missing live-region declaration: reply.templ Reply/p.y-reply role=status (found 0, want 1)"},
		},
		{
			name: "a script loses its live attribute",
			change: func(sources map[string]string) {
				sources["speech.js"] = strings.Replace(sources["speech.js"], "speechStatus.setAttribute('aria-live', 'polite');\n", "", 1)
			},
			want: []string{"missing live-region declaration: speech.js speechStatus/span.y-tts aria-live=polite (found 0, want 1)"},
		},
		{
			name: "a role moves to another element",
			change: func(sources map[string]string) {
				sources["reply.templ"] = "templ Reply() {\n\t<p class=\"y-reply\" aria-live=\"polite\"></p>\n\t<p class=\"y-gloss\" role=\"status\"></p>\n}\n"
			},
			want: []string{
				"added live-region declaration: reply.templ Reply/p.y-gloss role=status (found 1, want 0)",
				"missing live-region declaration: reply.templ Reply/p.y-reply role=status (found 0, want 1)",
			},
		},
		{
			name: "a script sets the attribute in a shape the scan cannot read",
			change: func(sources map[string]string) {
				sources["speech.js"] += "speechStatus.ariaLive = 'polite';\n"
			},
			want: []string{`speech.js:4 unreadable live-region declaration near "ariaLive"; name its element and use a literal attribute`},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sources := map[string]string{"reply.templ": syntheticReplyTemplate, "speech.js": syntheticSpeechScript}
			tt.change(sources)
			if diff := cmp.Diff(tt.want, liveRegionInventoryProblems(syntheticLiveRegionOwners, sources)); diff != "" {
				t.Errorf("inventory problems mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
