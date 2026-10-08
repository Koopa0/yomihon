package archlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type blockGrammarInventory map[string][]string

// The complete direct-call set makes a removed, bypassed, or newly added
// consumer visible. Grammar values are tested by behavior, not by this walk.
func supportedBlockGrammarInventory() blockGrammarInventory {
	return blockGrammarInventory{
		"owners":   {"internal/render/blockanchor.go:BlockAddress"},
		"adapters": {"internal/render/blockanchor.go:blockAddressIn"},
		"grammars": {"internal/render/blockanchor.go:blockMarkerTail"},
		"patterns": {"internal/render/blockanchor.go:blockMarkerTail"},
		"readers":  {"internal/render/blockanchor.go:BlockAddress"},
		"callers": {
			"internal/judge/fragment.go:collectBlockAddresses -> BlockAddress",
			"internal/render/blockanchor.go:CodeSpanOwnedAddresses -> blockAddressIn",
			"internal/render/blockanchor.go:blockAddressIn -> BlockAddress",
			"internal/render/blockanchor.go:markBlockAnchor -> blockAddressIn",
			"internal/render/blockanchor.go:markOwnedAddresses -> blockAddressIn",
			"internal/render/section.go:blockMarkerLine -> BlockAddress",
			"internal/render/tts.go:stripTrailingBlockAddress -> BlockAddress",
			"internal/render/wikilink.go:calloutOpeningTitle -> blockAddressIn",
		},
		"suffixes": nil,
		"aliases":  nil,
	}
}

func TestSupportedBlockGrammarHasOneOwnerAndEveryCaller(t *testing.T) {
	t.Parallel()
	t.Log("hit: complete production block grammar inventory")
	paths := productionFiles(t, ".go")
	if len(paths) == 0 {
		t.Fatal("no production Go source was discovered")
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path the repository source walk produced
		if err != nil {
			t.Fatalf("read block grammar source %q: %v", path, err)
		}
		sources[path] = string(data)
	}
	got, err := inspectBlockGrammar(sources)
	if err != nil {
		t.Fatalf("inspect production block grammar: %v", err)
	}
	if violations := blockGrammarViolations(got); len(violations) != 0 {
		t.Errorf("%s\ncomplete inventory (-want +got):\n%s", strings.Join(violations, "\n"), cmp.Diff(supportedBlockGrammarInventory(), got))
	}
}

func blockGrammarViolations(got blockGrammarInventory) []string {
	want := supportedBlockGrammarInventory()
	var violations []string
	for _, key := range []string{"owners", "adapters", "grammars", "patterns", "readers", "callers", "suffixes", "aliases"} {
		if !slices.Equal(want[key], got[key]) {
			violations = append(violations, "caught: block grammar "+key+" changed")
		}
	}
	return violations
}

// inspectBlockGrammar reads syntax without relying on emitted whitespace or
// import aliases. It bounds the copied-pattern check to caret-tail regexes and
// rejects suffix matching in the two known lookup functions; it does not claim
// to recognize arbitrary future handwritten token parsers.
func inspectBlockGrammar(sources map[string]string) (blockGrammarInventory, error) {
	inv := blockGrammarInventory{}
	for _, key := range []string{"owners", "adapters", "grammars", "patterns", "readers", "callers", "suffixes", "aliases"} {
		inv[key] = nil
	}
	for path, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		imports := map[string]string{}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("unquote import in %s: %w", path, err)
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name == "." && (importPath == "github.com/koopa0/yomihon/internal/render" || importPath == "regexp" || importPath == "strings") {
				inv["aliases"] = append(inv["aliases"], path+":dot import of "+importPath)
			}
			imports[name] = importPath
		}
		for _, declaration := range file.Decls {
			switch decl := declaration.(type) {
			case *ast.FuncDecl:
				caller := path + ":" + decl.Name.Name
				if decl.Name.Name == "BlockAddress" {
					owner := caller
					if !blockAddressSignature(decl) {
						owner += ":invalid signature"
					}
					inv["owners"] = append(inv["owners"], owner)
				}
				if decl.Name.Name == "blockAddressIn" {
					inv["adapters"] = append(inv["adapters"], caller)
				}
				if decl.Body != nil {
					inspectBlockGrammarNode(inv, decl.Body, file.Name.Name, caller, imports)
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range value.Names {
						if name.Name == "blockMarkerTail" {
							inv["grammars"] = append(inv["grammars"], path+":"+name.Name)
						}
					}
					caller := path + ":" + value.Names[0].Name
					for _, expression := range value.Values {
						inspectBlockGrammarNode(inv, expression, file.Name.Name, caller, imports)
					}
				}
			}
		}
	}
	for _, entries := range inv {
		slices.Sort(entries)
	}
	return inv, nil
}

func blockAddressSignature(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Body == nil || fn.Type.TypeParams != nil || fn.Type.Params == nil || fn.Type.Results == nil {
		return false
	}
	params, results := fn.Type.Params.List, fn.Type.Results.List
	if len(params) != 1 || len(params[0].Names) != 1 || len(results) != 1 || len(results[0].Names) > 1 {
		return false
	}
	param, paramOK := params[0].Type.(*ast.Ident)
	result, resultOK := results[0].Type.(*ast.Ident)
	return paramOK && resultOK && param.Name == "string" && result.Name == "string"
}

func blockGrammarReference(expression ast.Expr, pkg string, imports map[string]string) string {
	switch expr := expression.(type) {
	case *ast.Ident:
		if pkg == "render" && (expr.Name == "BlockAddress" || expr.Name == "blockAddressIn") {
			return expr.Name
		}
	case *ast.SelectorExpr:
		qualifier, ok := expr.X.(*ast.Ident)
		if ok && imports[qualifier.Name] == "github.com/koopa0/yomihon/internal/render" && expr.Sel.Name == "BlockAddress" {
			return expr.Sel.Name
		}
	}
	return ""
}

func blockGrammarSuffixCall(expression ast.Expr, imports map[string]string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "HasSuffix" {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && imports[qualifier.Name] == "strings"
}

func inspectBlockGrammarNode(inv blockGrammarInventory, node ast.Node, pkg, caller string, imports map[string]string) {
	if node == nil {
		return
	}
	called := map[ast.Expr]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if target := blockGrammarReference(call.Fun, pkg, imports); target != "" {
				inv["callers"] = append(inv["callers"], caller+" -> "+target)
				called[call.Fun] = true
			}
			lookup := caller == "internal/render/section.go:blockMarkerLine" || caller == "internal/judge/fragment.go:blockAddressed"
			if lookup && blockGrammarSuffixCall(call.Fun, imports) {
				inv["suffixes"] = append(inv["suffixes"], caller)
			}
		}
		if literal, ok := n.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			pattern, err := strconv.Unquote(literal.Value)
			if err != nil {
				inv["patterns"] = append(inv["patterns"], caller+":unreadable string")
			} else if strings.Contains(pattern, `\^`) && strings.Contains(pattern, `\z`) {
				inv["patterns"] = append(inv["patterns"], caller)
			}
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == "blockMarkerTail" {
			inv["readers"] = append(inv["readers"], caller)
		}
		if expression, ok := n.(ast.Expr); ok && !called[expression] {
			if target := blockGrammarReference(expression, pkg, imports); target != "" {
				inv["aliases"] = append(inv["aliases"], caller+" -> "+target)
			}
		}
		return true
	})
}

func blockGrammarControlSources() map[string]string {
	return map[string]string{
		"internal/render/blockanchor.go": "package render\nimport \"regexp\"\nvar blockMarkerTail = regexp.MustCompile(`(\\^x)\\z`)\n" +
			"func BlockAddress(line string) string { blockMarkerTail.FindString(line); return \"\" }\n" +
			"func blockAddressIn(line string) (string, []int) { BlockAddress(line); return \"\", nil }\n" +
			"func CodeSpanOwnedAddresses() { blockAddressIn(\"\") }\n" +
			"func markOwnedAddresses() { blockAddressIn(\"\") }\n" +
			"func markBlockAnchor() { blockAddressIn(\"\") }\n",
		"internal/render/section.go":  "package render\nfunc blockMarkerLine() { BlockAddress(\"\") }\n",
		"internal/render/tts.go":      "package render\nfunc stripTrailingBlockAddress() { BlockAddress(\"\") }\n",
		"internal/render/wikilink.go": "package render\nfunc calloutOpeningTitle() { blockAddressIn(\"\") }\n",
		"internal/judge/fragment.go": "package judge\nimport r \"github.com/koopa0/yomihon/internal/render\"\n" +
			"func collectBlockAddresses() { r.BlockAddress(\"\") }\nfunc blockAddressed() {}\n",
	}
}

func TestBlockGrammarInventoryRejectsUnreadableSource(t *testing.T) {
	t.Parallel()
	got, err := inspectBlockGrammar(map[string]string{"broken.go": "package render\nfunc broken("})
	if err == nil {
		t.Fatal("unreadable source returned no parse error")
	}
	if diff := cmp.Diff(blockGrammarInventory(nil), got); diff != "" {
		t.Errorf("unreadable source returned a partial inventory (-want +got):\n%s", diff)
	}
}

func TestBlockGrammarInventoryControls(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		path   string
		before string
		after  string
		want   []string
	}{
		{name: "positive source and aliased import"},
		{name: "adapter call removed", path: "internal/render/blockanchor.go", before: "BlockAddress(line);", after: "", want: []string{"caught: block grammar callers changed"}},
		{name: "code ownership call removed", path: "internal/render/blockanchor.go", before: "func CodeSpanOwnedAddresses() { blockAddressIn(\"\") }", after: "func CodeSpanOwnedAddresses() {}", want: []string{"caught: block grammar callers changed"}},
		{name: "run ownership call removed", path: "internal/render/blockanchor.go", before: "func markOwnedAddresses() { blockAddressIn(\"\") }", after: "func markOwnedAddresses() {}", want: []string{"caught: block grammar callers changed"}},
		{name: "page call removed", path: "internal/render/blockanchor.go", before: "func markBlockAnchor() { blockAddressIn(\"\") }", after: "func markBlockAnchor() {}", want: []string{"caught: block grammar callers changed"}},
		{name: "excerpt call removed", path: "internal/render/section.go", before: "BlockAddress(\"\")", after: "", want: []string{"caught: block grammar callers changed"}},
		{name: "speech call removed", path: "internal/render/tts.go", before: "BlockAddress(\"\")", after: "", want: []string{"caught: block grammar callers changed"}},
		{name: "callout call removed", path: "internal/render/wikilink.go", before: "blockAddressIn(\"\")", after: "", want: []string{"caught: block grammar callers changed"}},
		{name: "judge call removed", path: "internal/judge/fragment.go", before: "r.BlockAddress(\"\")", after: "", want: []string{"caught: block grammar callers changed"}},
		{name: "unapproved caller", path: "internal/render/section.go", before: "package render", after: "package render\nfunc secondReading() { BlockAddress(\"\") }", want: []string{"caught: block grammar callers changed"}},
		{name: "copied grammar", path: "internal/render/blockanchor.go", before: "func markBlockAnchor()", after: "var secondGrammar = regexp.MustCompile(`(\\^copy)\\z`)\nfunc markBlockAnchor()", want: []string{"caught: block grammar patterns changed"}},
		{name: "direct grammar bypass", path: "internal/render/section.go", before: "BlockAddress(\"\")", after: "blockMarkerTail.FindString(\"\")", want: []string{"caught: block grammar readers changed", "caught: block grammar callers changed"}},
		{name: "owner removed", path: "internal/render/blockanchor.go", before: "func BlockAddress(line string) string { blockMarkerTail.FindString(line); return \"\" }", after: "", want: []string{"caught: block grammar owners changed", "caught: block grammar readers changed"}},
		{name: "owner signature changed", path: "internal/render/blockanchor.go", before: "func BlockAddress(line string) string", after: "func BlockAddress(line []byte) string", want: []string{"caught: block grammar owners changed"}},
		{name: "grammar declaration removed", path: "internal/render/blockanchor.go", before: "var blockMarkerTail = regexp.MustCompile(`(\\^x)\\z`)", after: "", want: []string{"caught: block grammar grammars changed", "caught: block grammar patterns changed"}},
		{name: "adapter declaration removed", path: "internal/render/blockanchor.go", before: "func blockAddressIn(line string) (string, []int) { BlockAddress(line); return \"\", nil }", after: "", want: []string{"caught: block grammar adapters changed", "caught: block grammar callers changed"}},
		{name: "excerpt stale suffix", path: "internal/render/section.go", before: "func blockMarkerLine() { BlockAddress(\"\") }", after: "import \"strings\"\nfunc blockMarkerLine() { BlockAddress(\"\"); strings.HasSuffix(\"\", \"\") }", want: []string{"caught: block grammar suffixes changed"}},
		{name: "judge stale suffix", path: "internal/judge/fragment.go", before: "func collectBlockAddresses() { r.BlockAddress(\"\") }\nfunc blockAddressed() {}", after: "import \"strings\"\nfunc collectBlockAddresses() { r.BlockAddress(\"\") }\nfunc blockAddressed() { strings.HasSuffix(\"\", \"\") }", want: []string{"caught: block grammar suffixes changed"}},
		{name: "noncall owner alias", path: "internal/render/section.go", before: "BlockAddress(\"\")", after: "address := BlockAddress; address(\"\")", want: []string{"caught: block grammar callers changed", "caught: block grammar aliases changed"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("hit: block grammar inventory control %s", tt.name)
			sources := blockGrammarControlSources()
			if tt.path != "" {
				if count := strings.Count(sources[tt.path], tt.before); count != 1 {
					t.Fatalf("control %q mutation matches %d sites, want one", tt.name, count)
				}
				sources[tt.path] = strings.Replace(sources[tt.path], tt.before, tt.after, 1)
			}
			got, err := inspectBlockGrammar(sources)
			if err != nil {
				t.Fatalf("parse control %q: %v", tt.name, err)
			}
			if tt.name == "positive source and aliased import" {
				if diff := cmp.Diff(supportedBlockGrammarInventory(), got); diff != "" {
					t.Errorf("positive block grammar inventory (-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff(tt.want, blockGrammarViolations(got)); diff != "" {
				t.Errorf("block grammar control %q violations (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
