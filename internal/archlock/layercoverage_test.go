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
