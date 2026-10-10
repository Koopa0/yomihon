package archlock

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestOnlyTheSchemaPackageImportsTheTOMLDecoder keeps contract interpretation
// with its owner: another decoder import lets a package read a second answer
// instead of asking the schema. Generated Go ships too, while test imports
// only help fixtures describe the contract and do not enter the product.
// A tree with no decoder import would guard an unused path, so it fails too.
func TestOnlyTheSchemaPackageImportsTheTOMLDecoder(t *testing.T) {
	t.Parallel()

	const tomlImport = "github.com/BurntSushi/toml"
	fset := token.NewFileSet()
	var found []site
	imports := 0
	for _, path := range documentedGoFiles(t) {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: read import %s: %v", path, spec.Path.Value, err)
			}
			if imported != tomlImport {
				continue
			}
			imports++
			if filepath.ToSlash(filepath.Dir(path)) != "internal/schema" {
				found = append(found, site{path: path, line: fset.Position(spec.Pos()).Line, text: imported})
			}
		}
	}
	if imports == 0 {
		t.Fatalf("no production file imports %s, so this walk either read nothing or the decoder moved and this rule guards a path nobody uses", tomlImport)
	}
	report(t, "this imports the TOML decoder outside internal/schema; ask the schema for the contract instead", found)
}
