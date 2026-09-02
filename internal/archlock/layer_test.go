package archlock

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/koopa0/yomihon"

// enginePackages are the packages that hold what yomihon knows about the
// vault: the model, the reader, the graph and the projections built from them.
// Every one of them is built or rebuilt behind the generation store, which is
// the first thing the command constructs and the thing every served face reads
// from.
var enginePackages = []string{
	"internal/graph",
	"internal/judge",
	"internal/lesson",
	"internal/lexical",
	"internal/nav",
	"internal/render",
	"internal/schema",
	"internal/sequence",
	"internal/snapshot",
	"internal/vault",
	"internal/vaultfs",
}

// presentationPackages are what an engine package must not reach: the
// templates, the page shell, and the middleware that decides a served
// response's security headers.
var presentationPackages = []string{
	"internal/ui/",
	"internal/origin",
}

// TestTheEnginePackagesCannotSeeTheReadingInterface keeps the direction of the
// dependency between what yomihon knows and what it shows.
//
// The engine is built once per reading generation, before any request exists,
// and it is the layer a change to the vault model is reasoned about in. An
// engine package that imports the presentation layer, even transitively, drags
// the whole template set and the response middleware into that reasoning: the
// generation store could no longer be built or tested without them, and every
// import later added to a page would land inside the store's build graph
// without anyone deciding it should. Nothing about that breaks at runtime,
// which is exactly why it needs a test to be visible at all.
func TestTheEnginePackagesCannotSeeTheReadingInterface(t *testing.T) {
	t.Parallel()

	// A prefix that names nothing would pass every row without looking at
	// anything, so ask first whether the forbidden layer is still there under
	// the name this test spells.
	all := dependencies(t, module+"/cmd/yomihon")
	for _, forbidden := range presentationPackages {
		if !slices.ContainsFunc(all, func(dep string) bool {
			return strings.HasPrefix(strings.TrimPrefix(dep, module+"/"), forbidden)
		}) {
			t.Fatalf("nothing in this module is named %s any more, so every row below passes for the wrong reason", forbidden)
		}
	}

	for _, pkg := range enginePackages {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			for _, dep := range dependencies(t, module+"/"+pkg) {
				rel := strings.TrimPrefix(dep, module+"/")
				for _, forbidden := range presentationPackages {
					if strings.HasPrefix(rel, forbidden) {
						t.Errorf("%s reaches %s; the engine must not depend on the reading interface", pkg, rel)
					}
				}
			}
		})
	}
}

// TestTheNoteModelIsReachableWithoutTheReadCapability keeps the two halves of
// the vault split apart from the side that is easy to lose.
//
// A package that only asks what a note says has no business acquiring the
// ability to open one: the reading capability's whole value is that the set of
// packages holding it is small enough to audit. The three below ask the model
// alone today, and the way that quietly stops being true is a helper moving
// across the line — a path predicate, a normalizer — because it looked like it
// belonged nearer the disk.
func TestTheNoteModelIsReachableWithoutTheReadCapability(t *testing.T) {
	t.Parallel()

	for _, pkg := range []string{"internal/graph", "internal/lesson", "internal/render"} {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			deps := dependencies(t, module+"/"+pkg)
			if !slices.Contains(deps, module+"/internal/vault") {
				t.Fatalf("%s no longer depends on the note model at all, so this row asserts nothing", pkg)
			}
			if slices.Contains(deps, module+"/internal/vaultfs") {
				t.Errorf("%s reaches the vault read capability; it needs the note model only", pkg)
			}
		})
	}
}

// dependencies lists every package of this module that building pkg links,
// transitively. The question is asked of the build tool rather than answered by
// reading import lines, because an import one package away is exactly the way
// this property is lost without any file that names the forbidden package.
func dependencies(t *testing.T, pkg string) []string {
	t.Helper()

	format := `{{if .Module}}{{if eq .Module.Path "` + module + `"}}{{.ImportPath}}{{"\n"}}{{end}}{{end}}`
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", format, pkg) // #nosec G204 -- fixed Go invocation over a package path this file spells out
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("go list -deps %s: %v\n%s", pkg, err, exit.Stderr)
		}
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	var deps []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" && line != pkg {
			deps = append(deps, line)
		}
	}
	return deps
}
