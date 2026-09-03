package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// namedTitle matches the class attribute of an element carrying the title
// class, whatever else it carries alongside.
var namedTitle = regexp.MustCompile(`class="[^"]*\by-title\b[^"]*"`)

// TestNoPageNamesTheTitleTransitionTwice holds an invariant the stylesheet
// depends on and nothing else states. A transition name has to be unique in
// its document; where two elements claim one, the engine skips the whole
// transition and reports nothing anywhere. So a future page rendering two
// titled sections would silently turn off all eleven of the declarations that
// make one page dissolve into the next, and the only symptom would be that
// navigation stopped feeling continuous.
//
// It is checked against the recorded documents rather than against the
// templates, because the templates give the wrong answer: the reading page
// writes the title in two mutually exclusive branches and the not-found page
// does the same, so counting occurrences in the source reports two where one
// is rendered. A page with no title at all is not a duplicate and is allowed.
func TestNoPageNamesTheTitleTransitionTwice(t *testing.T) {
	t.Parallel()

	const stylesheet = "../../../assets/css/components.css"
	css, err := os.ReadFile(stylesheet)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", stylesheet, err)
	}
	// If the title stops taking part in the transition there is nothing here
	// to protect, and this check would go on passing while guarding nothing.
	if !strings.Contains(string(css), "view-transition-name: y-title") {
		t.Fatalf("%s no longer gives the title a transition name; either this check has outlived its reason or the transition was dropped by accident", stylesheet)
	}

	dir := filepath.Join("testdata", "render")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	documents := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".html" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path) // #nosec G304 -- a recording read out of this package's own testdata
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		// The recordings hold fragments as well as whole pages, and a
		// transition name is only ever unique within a document.
		if !strings.Contains(string(body), "<html") {
			continue
		}
		documents++
		if named := len(namedTitle.FindAllString(string(body), -1)); named > 1 {
			t.Errorf("%s names the title %d times; a repeated transition name makes the engine skip every transition on the page, with nothing anywhere saying so", entry.Name(), named)
		}
	}
	if documents < 8 {
		t.Fatalf("examined %d whole documents under %s, want at least 8 — this scan is reading the wrong place and would report nothing whatever the recordings said", documents, dir)
	}
}
