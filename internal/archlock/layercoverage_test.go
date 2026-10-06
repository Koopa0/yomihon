package archlock

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLayerMembershipRejectsMissingOrRepeatedDeclarations(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		packages []string
		layers   []packageLayer
		want     []string
	}{
		{"engine", []string{"internal/vault"}, []packageLayer{{"engine", []string{"internal/vault"}}}, nil},
		{"presentation", []string{"internal/ui/pages"}, []packageLayer{{"presentation", []string{"internal/ui/pages"}}}, nil},
		{"adapter", []string{"internal/note"}, []packageLayer{{"faces/adapters", []string{"internal/note"}}}, nil},
		{"unlisted", []string{"internal/digest"}, nil, []string{"internal/digest in no layer"}},
		{"cross layer duplicate", []string{"internal/vault"}, []packageLayer{{"engine", []string{"internal/vault"}}, {"presentation", []string{"internal/vault"}}}, []string{"internal/vault declared 2 times (engine, presentation); want exactly one layer"}},
		{"same layer duplicate", []string{"internal/vault"}, []packageLayer{{"engine", []string{"internal/vault", "internal/vault"}}}, []string{"internal/vault declared 2 times (engine, engine); want exactly one layer"}},
		{"stale declaration", []string{"internal/vault"}, []packageLayer{{"engine", []string{"internal/vault", "internal/deleted"}}}, []string{"internal/deleted is declared in engine but is not an internal package"}},
		{"prefix is not membership", []string{"internal/ui/pagesextra"}, []packageLayer{{"presentation", []string{"internal/ui/pages"}}}, []string{"internal/ui/pages is declared in presentation but is not an internal package", "internal/ui/pagesextra in no layer"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, layerMembershipProblems(tt.packages, tt.layers)); diff != "" {
				t.Errorf("caught: layer membership (-want +got):\n%s", diff)
			}
		})
	}
}

// Removing any declared member must identify that member, whichever layer
// owns it. The loop derives its complete control set from the declarations.
func TestEveryLayerMemberHasAMissingMembershipControl(t *testing.T) {
	t.Parallel()
	layers := packageLayers()
	var packages []string
	for _, layer := range layers {
		packages = append(packages, layer.packages...)
	}
	for at, layer := range layers {
		for _, pkg := range layer.packages {
			t.Run(pkg, func(t *testing.T) {
				t.Parallel()
				removed := slices.Clone(layers)
				removed[at].packages = slices.DeleteFunc(slices.Clone(layer.packages), func(member string) bool { return member == pkg })
				if diff := cmp.Diff([]string{pkg + " in no layer"}, layerMembershipProblems(packages, removed)); diff != "" {
					t.Errorf("caught: omitted member %s (-want +got):\n%s", pkg, diff)
				}
			})
		}
	}
}

func TestInternalPackageInventoryIncludesEverySourceDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                                      "module " + module + "\n\ngo 1.27.0\n",
		"internal/plain/plain.go":                     "package plain\n",
		"internal/nested/deep/deep.go":                "package deep\n",
		"internal/generated/generated_templ.go":       "package generated\n",
		"internal/windowsonly/package_windows.go":     "package windowsonly\n",
		"internal/linuxonly/package_linux.go":         "package linuxonly\n",
		"internal/testonly/only_test.go":              "package testonly\n",
		"internal/twofiles/first.go":                  "package twofiles\n",
		"internal/twofiles/second.go":                 "package twofiles\n",
		"internal/_ignored/ignored.go":                "package ignored\n",
		"internal/.hidden/ignored.go":                 "package ignored\n",
		"internal/plain/testdata/fixture/fixture.go":  "package fixture\n",
		"internal/plain/vendor/dependency/package.go": "package dependency\n",
		"internal/notpackage/README.md":               "no Go source\n",
		"internal/ignoredfile/_ignored.go":            "package ignored\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { // #nosec G703 -- fixed fixture paths within this test's temporary directory
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G703 -- fixed fixture paths within this test's temporary directory
			t.Fatal(err)
		}
	}
	want := []string{
		"internal/generated",
		"internal/linuxonly",
		"internal/nested/deep",
		"internal/plain",
		"internal/testonly",
		"internal/twofiles",
		"internal/windowsonly",
	}
	if diff := cmp.Diff(want, internalPackagePaths(t, root)); diff != "" {
		t.Errorf("caught: complete internal package inventory (-want +got):\n%s", diff)
	}
}

// The expected roots are independent of the projection so dropping a
// declared presentation package cannot silently change engine permissions.
func TestPresentationRootsCoverTheDeclaredLayer(t *testing.T) {
	t.Parallel()
	if diff := cmp.Diff([]string{"internal/origin", "internal/ui"}, presentationRoots(presentationLayerPackages)); diff != "" {
		t.Errorf("caught: complete presentation roots (-want +got):\n%s", diff)
	}
	for _, tt := range []struct {
		name          string
		members, want []string
	}{
		{"empty", nil, []string{}},
		{"new ui descendant", []string{"internal/ui/fresh/deep"}, []string{"internal/ui"}},
		{"ui directory", []string{"internal/ui"}, []string{"internal/ui"}},
		{"duplicate ui roots", []string{"internal/ui/pages", "internal/ui/layouts", "internal/ui/pages"}, []string{"internal/ui"}},
		{"near prefix", []string{"internal/uiish", "internal/originextra"}, []string{"internal/originextra", "internal/uiish"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, presentationRoots(tt.members)); diff != "" {
				t.Errorf("caught: presentation root projection (-want +got):\n%s", diff)
			}
		})
	}
}

// A real dependency graph includes a new UI descendant that no declaration
// names. The UI directory boundary must still forbid it, while similarly
// named packages remain outside that boundary.
func TestPresentationBoundaryIncludesNewUIDescendants(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                              "module " + module + "\n\ngo 1.27.0\n",
		"internal/origin/origin.go":           "package origin\n",
		"internal/origin/child/child.go":      "package child\n",
		"internal/ui/fresh/deep/deep.go":      "package deep\n",
		"internal/uiish/uiish.go":             "package uiish\n",
		"internal/originextra/originextra.go": "package originextra\n",
		"internal/engine/engine.go": `package engine
import (
 _ "github.com/koopa0/yomihon/internal/origin"
 _ "github.com/koopa0/yomihon/internal/origin/child"
 _ "github.com/koopa0/yomihon/internal/ui/fresh/deep"
 _ "github.com/koopa0/yomihon/internal/uiish"
 _ "github.com/koopa0/yomihon/internal/originextra"
)
`,
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { // #nosec G703 -- fixed fixture paths in a test-owned directory
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G703 -- fixed fixture paths in a test-owned directory
			t.Fatal(err)
		}
	}
	deps := listDeps(t, root, module+"/internal/engine", `{{if .Module}}{{if eq .Module.Path "`+module+`"}}{{.ImportPath}}{{"\n"}}{{end}}{{end}}`)
	t.Log("invoked: real presentation dependency fixture")
	want := []string{"internal/origin", "internal/origin/child", "internal/ui/fresh/deep"}
	if diff := cmp.Diff(want, presentationDependencies(deps, presentationRoots(presentationLayerPackages))); diff != "" {
		t.Errorf("caught: complete presentation dependency boundary (-want +got):\n%s", diff)
	}
}
