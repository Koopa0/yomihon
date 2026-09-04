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

// vaultVocabulary are the words a vault-schema.toml uses for the kinds of note
// and the stages of one. Each is spelled once, in internal/schema, and reached
// through that package: a second copy anywhere else is a second answer to
// "what does this vault call a lesson", and the two drift apart in the quiet
// direction — the copy keeps working on this repository's own vault and stops
// being true of anybody else's.
//
// The list is short on purpose. It holds the words the product reasons about,
// not every value an enum can carry: a vault names its own domains and its own
// map kinds, and nothing here compares against those.
// "archived" is deliberately absent. Nothing in internal/schema owns it: a
// vault names its own archived status under the supersession declaration, so a
// package comparing against the word reads it from there and this check would
// be asserting an owner that does not exist.
var vaultVocabulary = []string{"lesson", "concept", "inbox", "draft", "ready", "published"}

// What this check does not reach, said here so nobody reads a green run as
// more than it is. It compares whole string literals, so a word assembled from
// pieces or carried inside a longer sentence goes by unseen; it reads the list
// above, so a closed-set word nobody adds there is not looked for at all; and
// it walks the .go source this repository ships, so tests, fixtures and the Go
// inside a .templ file are outside it. Each of those could be closed and none of them is what went wrong: five
// comparisons against a bare "lesson" is the shape that was actually written,
// and it is the shape this catches.

// TestOnlyTheSchemaPackageSpellsTheVaultVocabulary walks the syntax rather than
// the text, because the same characters are two different things: a comparison
// against "lesson" is a copy of the contract's word, and a `yaml:"lesson"` tag
// is the name of a field in a sidecar file, which this rule has no opinion
// about. A grep cannot tell them apart and would either miss the first or
// forbid the second.
func TestOnlyTheSchemaPackageSpellsTheVaultVocabulary(t *testing.T) {
	t.Parallel()

	var found []site
	fset := token.NewFileSet()
	checked := 0
	for _, path := range productionFiles(t, ".go") {
		if strings.HasPrefix(path, "internal/schema/") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		checked++
		// A struct tag is a string literal too, and it is left alone by the
		// comparison rather than by a special case: a tag's literal is the
		// whole tag text — `yaml:"lesson"` — which is never equal to the bare
		// word. internal/lesson/slot.go carries exactly that tag, so this file
		// staying green is the demonstration.
		ast.Inspect(file, func(n ast.Node) bool {
			lit, isLit := n.(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for _, word := range vaultVocabulary {
				if value == word {
					pos := fset.Position(lit.Pos())
					found = append(found, site{path: path, line: pos.Line, text: lit.Value})
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no file outside internal/schema was parsed, so this check proved nothing")
	}
	report(t, "spells a word internal/schema owns; ask the contract for it instead", found)
}
