package archlock

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// vaultVocabulary are the words of a vault's rules file that internal/schema
// owns and the product reasons about. A copy anywhere else is a second answer
// to what a vault calls a lesson, which stays true of this repository's vault
// and stops being true of anybody else's.
var vaultVocabulary = []string{"lesson", "concept", "draft"}

// TestOnlyTheSchemaPackageSpellsTheVaultVocabulary reads every string literal
// the module ships outside internal/schema, the Go a template generates
// included. A literal is compared whole, so a struct tag such as
// `yaml:"lesson"` is a different string and stays legal.
func TestOnlyTheSchemaPackageSpellsTheVaultVocabulary(t *testing.T) {
	t.Parallel()

	all, err := worlds()
	if err != nil {
		t.Fatal(err)
	}
	w := all[fixtureTarget]
	for _, u := range w.units {
		if u.path == module+"/internal/schema" {
			continue
		}
		for _, file := range u.files {
			ast.Inspect(file, func(node ast.Node) bool {
				lit, isLit := node.(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					return true
				}
				if value, err := strconv.Unquote(lit.Value); err == nil && slices.Contains(vaultVocabulary, value) {
					path, line := u.rel(lit.Pos())
					t.Errorf("%s:%d spells %s, a word internal/schema owns; ask the contract for it instead", path, line, lit.Value)
				}
				return true
			})
		}
	}
}
