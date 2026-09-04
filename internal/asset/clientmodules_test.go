package asset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryClientModuleIsServed closes the gap between a module that exists and
// a module that can be fetched. A file under assets/js that no embed directive
// names and no registry entry serves answers 404, and one 404 inside an ES
// module graph stops every module in it — so the symptom is a page whose
// scripting silently does nothing at all, which no test that only renders HTML
// can see. The list is read from the directory rather than restated here,
// because a second copy of the list would need the same reminder this test is.
func TestEveryClientModuleIsServed(t *testing.T) {
	t.Parallel()

	const dir = "../../assets/js"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	examined := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".js" {
			continue
		}
		examined++
		if _, served := registry[entry.Name()]; !served {
			t.Errorf("assets/js/%s is not served: add it to the embed directive in assets/assets.go and to the registry", entry.Name())
		}
	}
	if examined == 0 {
		t.Fatal("no client module was examined, so this test asserts nothing")
	}
	// The entry point imports the others, so its absence would be the one 404
	// that costs the page everything.
	if _, served := registry["yomihon.js"]; !served {
		t.Error("the client entry point is not served")
	}
	if !strings.Contains(registry["yomihon.js"].contentType, "javascript") {
		t.Errorf("the client entry point is served as %q, want a JavaScript type", registry["yomihon.js"].contentType)
	}
}

// TestEveryFontIsServedAndNothingBesideThemIs closes the same gap on the other
// wildcard. Two things reach the served set: a name written in Go source, and a
// suffix match over a directory. A name in source is visible in review; a
// suffix match is not, so a face vendored into the directory and left out of
// the embed directive is a @font-face that 404s and a page that quietly falls
// back to a system font — legible, and not the typography anybody chose.
//
// The other direction is checked in the same pass, because the licence, the
// readme and the checksum list sit in that directory and none of them is meant
// to be fetchable.
func TestEveryFontIsServedAndNothingBesideThemIs(t *testing.T) {
	t.Parallel()

	const dir = "../../assets/fonts"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	faces, beside := 0, 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := "fonts/" + entry.Name()
		if filepath.Ext(entry.Name()) != ".woff2" {
			beside++
			if _, served := registry[name]; served {
				t.Errorf("assets/fonts/%s is served; only the faces themselves are meant to be fetchable", entry.Name())
			}
			continue
		}
		faces++
		served, ok := registry[name]
		if !ok {
			t.Errorf("assets/fonts/%s is not served: the directive embeds this directory whole, so either the name starts with a dot or an underscore and embed skipped it, or embedFonts no longer walks here", entry.Name())
			continue
		}
		if !strings.Contains(served.contentType, "woff2") {
			t.Errorf("assets/fonts/%s is served as %q, want a woff2 type", entry.Name(), served.contentType)
		}
	}
	if faces == 0 {
		t.Fatal("no font was examined, so the half of this test about serving them asserts nothing")
	}
	if beside == 0 {
		t.Fatal("nothing sits beside the fonts, so the half about not serving it asserts nothing")
	}
}
