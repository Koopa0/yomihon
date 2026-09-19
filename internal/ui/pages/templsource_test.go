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

// leastTagsRead is a floor on the tags the handler scan below opens, for the
// same reason leastTemplSources is one: a scanner whose needle stopped
// matching reads a clean tree and a broken one alike.
const leastTagsRead = 400

// attributeNames returns the attribute names written in one template source,
// each with the line it sits on. Values are blanked before the names are read
// — first the Go between braces, then the text between quotes — so that what
// is left is markup the browser will be given, and a word inside a value can
// never be mistaken for the name of an attribute. tagsRead is how many tags
// were opened, which is what tells a clean answer from a blind one.
func attributeNames(source string) (names []struct {
	line int
	name string
}, tagsRead int,
) {
	for number, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		rest := line
		for {
			open := strings.Index(rest, "<")
			if open < 0 {
				break
			}
			rest = rest[open+1:]
			end := strings.Index(rest, ">")
			if end < 0 {
				break
			}
			tag := rest[:end]
			rest = rest[end+1:]
			if tag == "" || tag[0] == '/' || tag[0] == '!' {
				continue
			}
			tagsRead++
			for field := range strings.FieldsSeq(blankValues(tag)) {
				name, _, ok := strings.Cut(field, "=")
				if !ok || name == "" {
					continue
				}
				names = append(names, struct {
					line int
					name string
				}{number + 1, name})
			}
		}
	}
	return names, tagsRead
}

// blankValues replaces every attribute value with spaces of the same width, so
// the positions of what remains are the positions in the original line.
func blankValues(tag string) string {
	out := []byte(tag)
	depth := 0
	quoted := false
	for i, b := range out {
		switch {
		case depth > 0:
			switch b {
			case '}':
				depth--
			case '{':
				depth++
			}
			out[i] = ' '
		case quoted:
			if b == '"' {
				quoted = false
			}
			out[i] = ' '
		case b == '{':
			depth++
			out[i] = ' '
		case b == '"':
			quoted = true
			out[i] = ' '
		}
	}
	return string(out)
}

// TestNoTemplateWiresBehaviorIntoAnAttribute keeps behavior out of the markup's
// attributes. What a press does is said one of two ways here: the browser's own
// vocabulary, where the press names the surface it opens and the act to perform
// on it, or a listener in a client module the page loads. The third way — the
// act written into the element as a value the browser evaluates — is neither,
// and the served pages already forbid it at the response's edge, so a template
// that grew one would ship a control that silently does nothing.
func TestNoTemplateWiresBehaviorIntoAnAttribute(t *testing.T) {
	t.Parallel()

	tags := 0
	for _, dir := range templSourceDirs(t) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".templ" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path) // #nosec G304 -- a template name read out of this repository's own source tree
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", path, err)
			}
			names, read := attributeNames(string(data))
			tags += read
			for _, attribute := range names {
				if isHandlerAttribute(attribute.name) {
					t.Errorf("%s:%d writes what a press does into the attribute %s\n\tname the surface and the act with command/commandfor or popovertarget, or listen for it in a client module",
						filepath.ToSlash(path), attribute.line, attribute.name)
				}
			}
		}
	}
	if tags < leastTagsRead {
		t.Fatalf("opened %d tags under %s, want at least %d — the scan is reading something other than markup and would report nothing whatever the templates said",
			tags, templSourceRoot, leastTagsRead)
	}
}

// isHandlerAttribute reports the shape rather than a list of names: every
// attribute the browser evaluates as behavior is spelled on followed by the
// name of a happening, and a list would go stale the day a new happening is
// defined.
func isHandlerAttribute(name string) bool {
	if !strings.HasPrefix(name, "on") || len(name) < 4 {
		return false
	}
	for _, r := range name[2:] {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// TestAttributeScanSeesBothAnswers is the reader's own kill test. A scan that
// found nothing has two explanations, and the table below is what tells them
// apart: the same markup with the handler and without it, and the values a
// name could be mistaken for if they were not blanked first.
func TestAttributeScanSeesBothAnswers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		source  string
		tags    int
		handler string
	}{
		{
			name:   "a press that names the surface and the act",
			source: `<button type="button" command="show-modal" commandfor="search-dialog">`,
			tags:   1,
		},
		{
			name:    "the same press with the act written in",
			source:  `<button type="button" onclick="document.querySelector('dialog').showModal()">`,
			tags:    1,
			handler: "onclick",
		},
		{
			name:   "a value that reads like a name until it is blanked",
			source: "<html data-ruby=\"on\" aria-pressed={ strconv.FormatBool(c.Ruby == \"on\") }>",
			tags:   1,
		},
		{
			name:   "a comment naming one is not one",
			source: "\t\t// onclick=\"x\" is what this element does not do\n\t\t<button type=\"button\">",
			tags:   1,
		},
		{
			name:   "a closing tag carries no attributes",
			source: "</button>",
			tags:   0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			names, tags := attributeNames(tt.source)
			if tags != tt.tags {
				t.Errorf("opened %d tags, want %d", tags, tt.tags)
			}
			found := ""
			for _, attribute := range names {
				if isHandlerAttribute(attribute.name) {
					found = attribute.name
				}
			}
			if found != tt.handler {
				t.Errorf("found handler %q, want %q (names read: %v)", found, tt.handler, names)
			}
		})
	}
}
