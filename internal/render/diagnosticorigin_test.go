package render_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// truncatedTitleKind is the diagnostic whose origin this file holds, written as
// a string because the check reads source rather than values. Renaming or
// deleting the constant leaves this string naming nothing, which the check
// reports as its own failure rather than as an all-clear.
const truncatedTitleKind = "DiagTitleTruncatedAtHash"

// TestNothingHereRaisesTheTruncatedTitleDiagnostic holds a sentence the
// diagnostic set's doc makes about itself: every kind but this one is raised by
// a pass in this package, and this one is noticed where a note is assembled,
// because the coincidence it reports is between frontmatter and a filename.
// Nothing in the compiler stops a pass here from raising it, and the day one
// does, the doc goes on saying the opposite in the one place a reader looks it
// up.
//
// The syntax tree is read rather than the bytes, because the doc comments name
// the kind too and a reader of bytes cannot tell a sentence about it from a
// line that emits it.
func TestNothingHereRaisesTheTruncatedTitleDiagnostic(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	fset := token.NewFileSet()
	parsed := 0
	declaredAt := make(map[string]bool)
	var mentions []string
	// The wire value and where it is written are read out of the declaration
	// rather than repeated here, so this cannot end up guarding a string the
	// constant stopped carrying.
	wire, wireAt := "", ""
	literals := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		parsed++
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ValueSpec:
				for i, ident := range node.Names {
					if ident.Name != truncatedTitleKind {
						continue
					}
					declaredAt[fset.Position(ident.Pos()).String()] = true
					if i < len(node.Values) {
						if lit, ok := node.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if text, err := strconv.Unquote(lit.Value); err == nil {
								wire, wireAt = text, fset.Position(lit.Pos()).String()
							}
						}
					}
				}
			case *ast.Ident:
				if node.Name == truncatedTitleKind {
					mentions = append(mentions, fset.Position(node.Pos()).String())
				}
			case *ast.BasicLit:
				if node.Kind == token.STRING {
					if text, err := strconv.Unquote(node.Value); err == nil {
						literals[fset.Position(node.Pos()).String()] = text
					}
				}
			}
			return true
		})
	}

	// Both of these are the check failing, not the package: one says it read no
	// source at all, the other that the name it exists to watch is not declared
	// where it thought. Either way an "all clear" below would mean nothing.
	if parsed == 0 {
		t.Fatal("no shipped source in this package was parsed, so this check would pass over any tree")
	}
	if wire == "" {
		t.Fatalf("%s carries no string value this check could look for, so it watches the identifier and nothing else", truncatedTitleKind)
	}
	if len(declaredAt) != 1 {
		t.Fatalf("%s is declared %d times in this package, want exactly 1: the check cannot tell a use from the declaration it measures against", truncatedTitleKind, len(declaredAt))
	}

	// Every mention is an identifier, and the only one left standing should be
	// the declaration; anything else is shipped code reaching for the kind.
	var uses []string
	for _, at := range mentions {
		if !declaredAt[at] {
			uses = append(uses, at)
		}
	}
	// The identifier is not the only way to reach the kind: its value spells the
	// same thing, and a pass writing that literal raises it just as much.
	var spelled []string
	for at, text := range literals {
		if text == wire && at != wireAt {
			spelled = append(spelled, at)
		}
	}
	if len(spelled) > 0 {
		sort.Strings(spelled)
		t.Errorf("the value %q is written by shipped code here, at %s, which raises the kind as surely as naming it does; no pass in this package raises it — the coincidence is noticed where a note is assembled",
			wire, strings.Join(spelled, ", "))
	}
	if len(uses) > 0 {
		t.Errorf("%s is reached by shipped code here, at %s; no pass in this package raises it — a note's frontmatter and its filename are read where the note is assembled, and that is where the coincidence is noticed. Either move the emission there, or correct the set's doc, which tells a reader this kind does not come from here",
			truncatedTitleKind, strings.Join(uses, ", "))
	}
}
