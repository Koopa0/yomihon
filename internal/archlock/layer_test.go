package archlock

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
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
	"internal/wording",
}

// Presentation membership also owns the forbidden dependency roots. UI
// packages share a directory boundary, so an engine cannot reach a newly
// added UI descendant before its explicit membership decision is made.
var presentationLayerPackages = []string{
	"internal/origin",
	"internal/ui/layouts",
	"internal/ui/pages",
}

// Faces and adapters include the served endpoints and the build-time checks.
// They may join engine and presentation concerns without making that join a
// dependency of the vault's reading generation.
var faceAdapterPackages = []string{
	"internal/archlock",
	"internal/asset",
	"internal/mark",
	"internal/note",
	"internal/preference",
	"internal/report",
	"internal/search",
	"internal/shell",
	"internal/sourcebytes",
	"internal/status",
	"internal/syllabus",
}

type packageLayer struct {
	name     string
	packages []string
}

func presentationRoots(packages []string) []string {
	roots := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if pkg == "internal/ui" || strings.HasPrefix(pkg, "internal/ui/") {
			pkg = "internal/ui"
		}
		roots = append(roots, pkg)
	}
	slices.Sort(roots)
	return slices.Compact(roots)
}

func presentationDependencies(deps, roots []string) []string {
	var forbidden []string
	for _, dep := range deps {
		rel := strings.TrimPrefix(dep, module+"/")
		if slices.ContainsFunc(roots, func(root string) bool {
			return rel == root || strings.HasPrefix(rel, root+"/")
		}) {
			forbidden = append(forbidden, rel)
		}
	}
	return forbidden
}

func packageLayers() []packageLayer {
	return []packageLayer{
		{"engine", enginePackages},
		{"presentation", presentationLayerPackages},
		{"faces/adapters", faceAdapterPackages},
	}
}

// An import check over named engines cannot see a package nobody classified.
// Every internal package must have exactly one declared role before the
// direction of its dependencies can be reviewed.
func TestEveryInternalPackageBelongsToExactlyOneLayer(t *testing.T) {
	t.Parallel()
	for _, problem := range layerMembershipProblems(internalPackagePaths(t, repoRoot), packageLayers()) {
		t.Errorf("caught: %s", problem)
	}
}

func layerMembershipProblems(packages []string, layers []packageLayer) []string {
	actual := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		actual[pkg] = true
	}
	var problems []string
	membership := make(map[string][]string)
	for _, layer := range layers {
		for _, pkg := range layer.packages {
			membership[pkg] = append(membership[pkg], layer.name)
			if !actual[pkg] {
				problems = append(problems, fmt.Sprintf("%s is declared in %s but is not an internal package", pkg, layer.name))
			}
		}
	}
	for _, pkg := range packages {
		switch len(membership[pkg]) {
		case 0:
			problems = append(problems, pkg+" in no layer")
		case 1:
		default:
			problems = append(problems, fmt.Sprintf("%s declared %d times (%s); want exactly one layer", pkg, len(membership[pkg]), strings.Join(membership[pkg], ", ")))
		}
	}
	slices.Sort(problems)
	return problems
}

// A wildcard go list drops packages whose files target another operating
// system. Discover source directories first, then name each one explicitly;
// -e retains those packages even when the host cannot build them. Compilation
// errors are the build gate's concern; this check owns package membership.
func internalPackagePaths(t *testing.T, root string) []string {
	t.Helper()
	dirs := make(map[string]bool)
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(name) != ".go" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		rel, relErr := filepath.Rel(root, filepath.Dir(path))
		if relErr != nil {
			return relErr
		}
		dirs["./"+filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal package sources: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("no internal package source was found, so layer membership checks nothing")
	}
	paths := make([]string, 0, len(dirs))
	for dir := range dirs {
		paths = append(paths, dir)
	}
	slices.Sort(paths)
	args := append([]string{"list", "-e", "-f", "{{.ImportPath}}"}, paths...)
	cmd := exec.CommandContext(t.Context(), "go", args...) // #nosec G204 -- fixed Go inventory command over directories discovered beneath the supplied repository root
	cmd.Dir = root
	out, listErr := cmd.Output()
	if listErr != nil {
		if exit, ok := errors.AsType[*exec.ExitError](listErr); ok {
			t.Fatalf("go list internal packages: %v\n%s", listErr, exit.Stderr)
		}
		t.Fatalf("go list internal packages: %v", listErr)
	}
	var packages []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		rel, internal := strings.CutPrefix(line, module+"/internal/")
		if !internal || rel == "" {
			t.Fatalf("go list returned %q, want a package under %s/internal/", line, module)
		}
		packages = append(packages, "internal/"+rel)
	}
	if len(packages) != len(paths) {
		t.Fatalf("caught: go list returned %d packages for %d source directories", len(packages), len(paths))
	}
	return packages
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
	roots := presentationRoots(presentationLayerPackages)
	all := dependencies(t, module+"/cmd/yomihon")
	for _, forbidden := range roots {
		if len(presentationDependencies(all, []string{forbidden})) == 0 {
			t.Fatalf("nothing in this module is named %s any more, so every row below passes for the wrong reason", forbidden)
		}
	}

	for _, pkg := range enginePackages {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			for _, dep := range presentationDependencies(dependencies(t, module+"/"+pkg), roots) {
				t.Errorf("%s reaches %s; the engine must not depend on the reading interface", pkg, dep)
			}
		})
	}
}

// TestTheDictionaryNeverReadsARequest keeps what yomihon says separable from
// who is asking.
//
// The dictionary is a pair of strings per sentence and nothing else, which is
// what lets a report rendered from the command line and a page rendered for a
// browser reach for the same words. One function that took a request was enough
// to put the whole server library inside the closure of every package that only
// wanted a noun, and the reading of a cookie belongs where the request already
// is. Reading one header looks harmless at the moment it is written, so the
// cost has to be asserted rather than remembered.
func TestTheDictionaryNeverReadsARequest(t *testing.T) {
	t.Parallel()

	const server = "net/http"
	if !slices.Contains(allDependencies(t, module+"/internal/origin"), server) {
		t.Fatalf("nothing in this module reaches %s any more, so this check passes for the wrong reason", server)
	}
	if slices.Contains(allDependencies(t, module+"/internal/wording"), server) {
		t.Errorf("the dictionary reaches %s; a sentence is answered without a request, and reading one belongs in internal/origin", server)
	}
}

// dependencies lists every package of this module that building pkg links,
// transitively. The question is asked of the build tool rather than answered by
// reading import lines, because an import one package away is exactly the way
// this property is lost without any file that names the forbidden package.
func dependencies(t *testing.T, pkg string) []string {
	t.Helper()

	return listDeps(t, "..", pkg, `{{if .Module}}{{if eq .Module.Path "`+module+`"}}{{.ImportPath}}{{"\n"}}{{end}}{{end}}`)
}

// allDependencies lists every package building pkg links, this module's and the
// standard library's alike, for a check about what a package drags in from
// outside the module.
func allDependencies(t *testing.T, pkg string) []string {
	t.Helper()

	return listDeps(t, "..", pkg, `{{.ImportPath}}{{"\n"}}`)
}

func listDeps(t *testing.T, root, pkg, format string) []string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", format, pkg) // #nosec G204 -- fixed Go invocation over a package path this file spells out
	cmd.Dir = root
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
