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
