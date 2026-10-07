package judge_test

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

func agreementFixtures(t *testing.T) []agreementCase {
	t.Helper()
	repo, openErr := os.OpenRoot("../..")
	if openErr != nil {
		t.Fatalf("open fixture repository: %v", openErr)
	}
	t.Cleanup(func() {
		if closeErr := repo.Close(); closeErr != nil {
			t.Errorf("close fixture repository: %v", closeErr)
		}
	})
	var paths []string
	for _, root := range []string{"../../internal", "../../.github/e2e/vault", "../../examples/vault"} {
		before := len(paths)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return fmt.Errorf("read symlink fixture %q: %w", path, err)
				}
				t.Logf("fixture-excluded=%s role=symlink-not-regular-body target=%q sha256=%x", filepath.ToSlash(path), target, sha256.Sum256([]byte(target)))
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("nonregular fixture %q: %v", path, entry.Type())
			}
			if root == "../../internal" && !strings.Contains(filepath.ToSlash(path), "/testdata/") {
				return nil
			}
			if strings.HasSuffix(path, ".md") || strings.Contains(filepath.ToSlash(path), "/testdata/fuzz/") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("discover %q: %v", root, err)
		}
		if len(paths) == before {
			t.Fatalf("empty fixture root %q", root)
		}
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		t.Fatal("empty fixture inventory")
	}
	cases := make([]agreementCase, 0, len(paths))
	for _, path := range paths {
		name, relErr := filepath.Rel("../..", path)
		if relErr != nil {
			t.Fatalf("resolve fixture %q: %v", path, relErr)
		}
		data, err := repo.ReadFile(name)
		if err != nil {
			t.Fatalf("read fixture %q: %v", path, err)
		}
		split, _ := vault.SplitFrontmatter(data)
		body := string(split.Body)
		if strings.Contains(filepath.ToSlash(path), "/testdata/fuzz/") {
			var included bool
			body, included = agreementFuzzBody(t, path, string(data))
			if !included {
				continue
			}
		}
		t.Logf("fixture=%s sha256=%x", filepath.ToSlash(path), sha256.Sum256([]byte(body)))
		cases = append(cases, agreementCase{Name: filepath.ToSlash(path), Body: body})
	}
	t.Logf("fixture-count=%d", len(cases))
	return cases
}

// Saved Fuzz signatures identify the body argument; remaining arguments are
// query/indices/status choices, not Markdown. Schema inputs have no body role.
func agreementFuzzBody(t *testing.T, path, text string) (string, bool) {
	t.Helper()
	packagePath, remainder, ok := strings.Cut(filepath.ToSlash(path), "/testdata/fuzz/")
	if !ok {
		t.Fatalf("fuzz path has no target: %q", path)
	}
	target, _, ok := strings.Cut(remainder, "/")
	if !ok {
		t.Fatalf("fuzz path has no seed: %q", path)
	}
	identity := filepath.Base(packagePath) + "/" + target
	var types []string
	wholeNote := false
	switch identity {
	case "render/FuzzStripObsidianComments":
		types = []string{"string"}
	case "lexical/FuzzSnippet":
		types = []string{"string", "string", "int", "int"}
	case "status/FuzzRewriteStatusLine":
		types = []string{"[]byte", "byte"}
		wholeNote = true
	case "schema/FuzzDecodeContractDeterministic", "schema/FuzzParseLanguageTag":
		t.Logf("fuzz-excluded=%s target=%s role=contract-or-language-not-Markdown sha256=%x", path, identity, sha256.Sum256([]byte(text)))
		return "", false
	default:
		t.Fatalf("unclassified saved fuzz target %q", identity)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) < 2 || lines[0] != "go test fuzz v1" {
		t.Fatalf("unsupported fuzz envelope %q: %q", path, text)
	}
	var arguments []string
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			arguments = append(arguments, line)
		}
	}
	if len(arguments) != len(types) {
		t.Fatalf("fuzz %q argument count = %d, want %d", identity, len(arguments), len(types))
	}
	var body string
	for index, encoded := range arguments {
		literal := agreementFuzzArgument(t, encoded, types[index])
		if index == 0 {
			decoded, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatalf("decode fuzz body %q: %v", path, err)
			}
			body = decoded
		}
	}
	if wholeNote {
		split, _ := vault.SplitFrontmatter([]byte(body))
		body = string(split.Body)
	}
	t.Logf("fuzz-included=%s target=%s body-argument=0 nonbody-arguments=%d", path, identity, len(arguments)-1)
	return body, true
}

func agreementFuzzArgument(t *testing.T, encoded, wantType string) *ast.BasicLit {
	t.Helper()
	expr, err := parser.ParseExpr(encoded)
	if err != nil {
		t.Fatalf("parse fuzz argument %q: %v", encoded, err)
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		t.Fatalf("unsupported fuzz argument %q", encoded)
	}
	actualType := ""
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		actualType = fn.Name
	case *ast.ArrayType:
		ident, isIdent := fn.Elt.(*ast.Ident)
		if fn.Len == nil && isIdent && ident.Name == "byte" {
			actualType = "[]byte"
		}
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || actualType != wantType {
		t.Fatalf("fuzz argument %q type = %q, want %q", encoded, actualType, wantType)
	}
	wantKind := token.STRING
	switch wantType {
	case "int":
		wantKind = token.INT
	case "byte":
		wantKind = token.CHAR
	}
	if literal.Kind != wantKind {
		t.Fatalf("fuzz argument %q literal kind = %v, want %v", encoded, literal.Kind, wantKind)
	}
	return literal
}

func agreementEnvelope(t *testing.T, body string) []byte {
	t.Helper()
	wrapped := []byte("---\n---\n" + body)
	split, _ := vault.SplitFrontmatter(wrapped)
	if got := string(split.Body); got != body {
		t.Fatalf("synthetic envelope changed body: got=%q want=%q", got, body)
	}
	return wrapped
}
