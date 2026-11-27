package archlock

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/koopa0/yomihon"

// TestTheEngineCannotSeeARequest keeps everything the reading generation is
// built from free of the server and the templates. Every engine package is in
// what internal/snapshot links, so a new one is held here by being used, and
// an import a page needs never lands in the store's build.
func TestTheEngineCannotSeeARequest(t *testing.T) {
	t.Parallel()

	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", "{{.ImportPath}}", module+"/internal/snapshot").Output()
	if err != nil {
		t.Fatalf("go list -deps internal/snapshot: %v", err)
	}
	for dep := range strings.FieldsSeq(string(out)) {
		if dep == "net/http" || dep == "github.com/a-h/templ" || strings.HasPrefix(dep, "github.com/a-h/templ/") {
			t.Errorf("internal/snapshot links %s; the engine is built before any request exists", dep)
		}
	}
}
