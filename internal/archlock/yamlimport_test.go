package archlock

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
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
