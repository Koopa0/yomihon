package archlock

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/mod/modfile"
)

// TestDependencyAccountMatchesTheModule keeps the contributor's account of
// dependencies attached to the declared modules and their actual imports.
// Test-only libraries have no reason to enter a production binary unnoticed.
func TestDependencyAccountMatchesTheModule(t *testing.T) {
	t.Parallel()
	t.Log("invoked: complete documented module account")
	manifest, err := modfile.Parse("go.mod", readOwnedDocument(t, "go.mod"), nil)
	if err != nil {
		t.Fatalf("parse module declaration: %v", err)
	}
	type dependency struct {
		path     string
		testOnly bool
	}
	want := []dependency{
		{path: "github.com/BurntSushi/toml"},
		{path: "github.com/a-h/templ"},
		{path: "github.com/alecthomas/chroma/v3"},
		{path: "github.com/google/go-cmp", testOnly: true},
		{path: "github.com/yuin/goldmark"},
		{path: "go.yaml.in/yaml/v3"},
		{path: "golang.org/x/mod", testOnly: true},
		{path: "golang.org/x/net"},
		{path: "golang.org/x/sys"},
		{path: "golang.org/x/text"},
	}
	type uses struct{ runtime, tests bool }
	imports := make(map[string]uses)
	for _, path := range documentedGoFiles(t) {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, readOwnedDocument(t, path), parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse imports in %s: %v", path, parseErr)
		}
		for _, spec := range file.Imports {
			imported, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("read import in %s: %v", path, unquoteErr)
			}
			for _, requirement := range manifest.Require {
				name := requirement.Mod.Path
				if requirement.Indirect || (imported != name && !strings.HasPrefix(imported, name+"/")) {
					continue
				}
				use := imports[name]
				if strings.HasSuffix(path, "_test.go") {
					use.tests = true
				} else {
					use.runtime = true
				}
				imports[name] = use
			}
		}
	}
	var got []dependency
	for _, requirement := range manifest.Require {
		if requirement.Indirect {
			continue
		}
		use := imports[requirement.Mod.Path]
		if !use.runtime && !use.tests {
			t.Errorf("caught: direct module %s has no inspected import", requirement.Mod.Path)
		}
		got = append(got, dependency{path: requirement.Mod.Path, testOnly: !use.runtime})
	}
	slices.SortFunc(got, func(a, b dependency) int { return strings.Compare(a.path, b.path) })
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(dependency{})); diff != "" {
		t.Errorf("caught: documented complete dependency membership or test-only scope differs (-want +got):\n%s", diff)
	}

	guide := strings.ReplaceAll(string(readOwnedDocument(t, "CONTRIBUTING.md")), "\r\n", "\n")
	account := "- **A new dependency.** `go.mod` names ten direct requirements and each earned its place: goldmark and chroma to render, templ for the templates, a TOML decoder for the vault contract, a YAML decoder for frontmatter, three `golang.org/x` libraries for runtime behavior, and go-cmp plus `golang.org/x/mod` for the tests."
	var accounts []string
	for paragraph := range strings.SplitSeq(guide, "\n\n") {
		paragraph = strings.Join(strings.Fields(paragraph), " ")
		if !strings.HasPrefix(paragraph, "- **A new dependency.** ") {
			continue
		}
		declaration, _, found := strings.Cut(paragraph, " The client side")
		if !found {
			t.Fatal("caught: contributor dependency account has no client-side boundary")
		}
		accounts = append(accounts, declaration)
	}
	if diff := cmp.Diff([]string{account}, accounts); diff != "" {
		t.Errorf("caught: contributor dependency account omits or misclassifies declared modules (-want +got):\n%s", diff)
	}
}

// TestLanguageStatisticsNameOnlyCurrentGeneratedAssets keeps retired search
// inventories from looking like live generated output in repository metadata.
func TestLanguageStatisticsNameOnlyCurrentGeneratedAssets(t *testing.T) {
	t.Parallel()
	t.Log("invoked: complete language statistics declarations")
	var got []string
	for line := range strings.SplitSeq(string(readOwnedDocument(t, ".gitattributes")), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			got = append(got, strings.Join(strings.Fields(line), " "))
		}
	}
	want := []string{"assets/js/mermaid/** linguist-vendored", "*_templ.go linguist-generated"}
	slices.Sort(want)
	slices.Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: language statistics name missing or obsolete declarations (-want +got):\n%s", diff)
	}
}

// TestAssetCommentsTravelWithThePackage prevents client-asset documentation
// from relying on a private planning document a contributor cannot read.
func TestAssetCommentsTravelWithThePackage(t *testing.T) {
	t.Parallel()
	t.Log("invoked: asset comments available to contributors")
	file, err := parser.ParseFile(token.NewFileSet(), "assets/assets.go", readOwnedDocument(t, "assets/assets.go"), parser.ParseComments)
	if err != nil {
		t.Fatalf("parse asset comments: %v", err)
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			for _, privateName := range []string{"yomihon-dev", "D-brief"} {
				if strings.Contains(comment.Text, privateName) {
					t.Errorf("caught: asset comment relies on absent document %s", privateName)
				}
			}
		}
	}
}

// TestExampleLibraryOffersBothReadmeLanguages makes either example entry
// point lead to its translated peer, with the same language line as the root.
func TestExampleLibraryOffersBothReadmeLanguages(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		path   string
		line   string
		target string
	}{
		{path: "examples/README.md", line: "English | [繁體中文](README.zh-TW.md)", target: "examples/README.zh-TW.md"},
		{path: "examples/README.zh-TW.md", line: "[English](README.md) | 繁體中文", target: "examples/README.md"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			t.Log("invoked: example language line " + tt.path)
			readme := strings.ReplaceAll(string(readOwnedDocument(t, tt.path)), "\r\n", "\n")
			parts := strings.SplitN(readme, "\n\n", 3)
			if len(parts) != 3 || parts[1] != tt.line || strings.Count(readme, tt.line) != 1 {
				t.Errorf("caught: example entry %s lacks its unique reciprocal language line", tt.path)
			}
			if len(readOwnedDocument(t, tt.target)) == 0 {
				t.Errorf("caught: example language target %s is empty", tt.target)
			}
		})
	}
}

// readOwnedDocument reads the repository artifact itself; test expectations
// must not substitute a convenient constant for what a contributor receives.
func readOwnedDocument(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- fixed repository artifacts or paths produced by documentedGoFiles
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// documentedGoFiles includes generated Go because a module used by a generated
// template is a runtime dependency too. Fixture text and build outputs are not
// module source and cannot turn a test-only import into a production import.
func documentedGoFiles(t *testing.T) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != repoRoot && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "bin" || name == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk documented module sources: %v", err)
	}
	slices.Sort(paths)
	return paths
}
