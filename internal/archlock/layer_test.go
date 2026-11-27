package archlock

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const module = "github.com/koopa0/yomihon"

// TestTheEngineCannotSeeARequest keeps everything the reading generation is
// built from free of the server and the templates, on every release system.
// Every engine package is in what internal/snapshot links, so a new one is held
// here by being used, and an import a page needs never lands in the store's
// build.
func TestTheEngineCannotSeeARequest(t *testing.T) {
	t.Parallel()

	all, err := worlds()
	if err != nil {
		t.Fatal(err)
	}
	links := map[string][]string{}
	for _, at := range releaseTargets {
		snapshot := all[at].graph[module+"/internal/snapshot"]
		if snapshot == nil {
			t.Fatalf("internal/snapshot is not in the build on %s", at)
		}
		packages.Visit([]*packages.Package{snapshot}, nil, func(p *packages.Package) {
			if p.PkgPath == "net/http" || p.PkgPath == "github.com/a-h/templ" || strings.HasPrefix(p.PkgPath, "github.com/a-h/templ/") {
				links[p.PkgPath] = append(links[p.PkgPath], at.String())
			}
		})
	}
	for _, dep := range slices.Sorted(maps.Keys(links)) {
		t.Errorf("internal/snapshot links %s on %v; the engine is built before any request exists", dep, links[dep])
	}
}
