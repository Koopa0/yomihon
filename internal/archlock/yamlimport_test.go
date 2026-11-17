package archlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The two import paths of the YAML decoder every note is read through. The
// first is the one this module moved off: it is unmaintained and its last
// release panics on a merge key that sits beside a key which is a list or a
// mapping, so one such note in a vault ended the server. The second is the
// maintained line of the same package, which turns that input into an error.
const (
	retiredYAMLImport = "gopkg.in/yaml.v3"
	currentYAMLImport = "go.yaml.in/yaml/v3"
)

// TestNoSourceFileImportsTheRetiredYAMLDecoder keeps the decoder on its
// maintained module. It reads the import path itself, in every Go file of this
// module including the tests, so a file that reaches for the old path again
// fails here whichever package it lands in.
//
// The rule is about the import path and not about the module: go.sum keeps the
// retired module's hashes while another module in the build graph still
// requires it, and that is not an import.
//
// A tree with no import of the current path is not clean, it is unread: either
// the walk reached nothing or the decoder moved again, and in both cases this
// rule would pass over a tree it says nothing about.
func TestNoSourceFileImportsTheRetiredYAMLDecoder(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	var found []site
	current := 0
	for _, path := range everyGoFile(t) {
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: read import %s: %v", path, spec.Path.Value, err)
			}
			switch imported {
			case currentYAMLImport:
				current++
			case retiredYAMLImport:
				found = append(found, site{path: path, line: fset.Position(spec.Pos()).Line, text: imported})
			}
		}
	}
	if current == 0 {
		t.Fatalf("no file imports %s, so this walk either read nothing or the decoder moved and the rule below guards a path nobody uses", currentYAMLImport)
	}
	report(t, "this imports the unmaintained YAML decoder; import "+currentYAMLImport+" instead", found)
}

// The one file outside internal/vault that turns bytes into YAML. It reads a
// drill's slot sidecar, a file of its own that no note's frontmatter ever
// reaches.
const sidecarDecoder = "internal/drill/slot.go"

// TestOnlyTheVaultDecodesFrontmatter keeps a note's frontmatter read once.
// Two decodes of the same block can disagree about what it says, and one did: a
// single string for aliases was a name to the check and no name to the page.
// internal/vault decodes the block and hands out both views of the one
// document, the map and the tree, so no other package turns bytes into YAML.
//
// The entry points are yaml.Unmarshal and yaml.NewDecoder: a *yaml.Node, the
// only other way to hold a document, comes from one of them, so a package that
// reads the tree vault gave it decodes nothing. The sidecar above is the one
// exception, and the rule fails if it stops being one, because an exception
// that names nothing hides the next file that copies it.
func TestOnlyTheVaultDecodesFrontmatter(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	var found []site
	inVault, inSidecar := 0, 0
	for _, path := range productionFiles(t, ".go") {
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		local := yamlLocalName(t, path, file)
		if local == "" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != local || (selector.Sel.Name != "Unmarshal" && selector.Sel.Name != "NewDecoder") {
				return true
			}
			switch {
			case strings.HasPrefix(path, "internal/vault/"):
				inVault++
			case path == sidecarDecoder:
				inSidecar++
			default:
				found = append(found, site{path: path, line: fset.Position(call.Pos()).Line, text: pkg.Name + "." + selector.Sel.Name})
			}
			return true
		})
	}
	if inVault == 0 {
		t.Fatal("internal/vault decodes no YAML, so this walk read the wrong tree or the one decode moved and the rule guards a place nobody uses")
	}
	if inSidecar == 0 {
		t.Fatalf("%s decodes no YAML, so the exception this rule names is stale; remove it", sidecarDecoder)
	}
	report(t, "frontmatter is decoded in internal/vault only; read the document it returns (vault.ParseWithTree)", found)
}

// yamlLocalName is the name a file calls the YAML package by, or "" when it
// does not import it.
func yamlLocalName(t *testing.T, path string, file *ast.File) string {
	t.Helper()
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s: read import %s: %v", path, spec.Path.Value, err)
		}
		if imported != currentYAMLImport {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "yaml"
	}
	return ""
}
