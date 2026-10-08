package judge

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Read the whole production package, including platform-specific and future
// files. Allocation measurements cannot identify an extra body inspection.
func judgeBodyStructureLock(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read judge source directory: %w", err)
	}
	files := make(map[string]*ast.File)
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(directory, name), nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		files[name] = file
	}
	return checkBodyStructureLock(files)
}

func bodyStructureCallers() map[string]map[string]int {
	return map[string]map[string]int{
		"structure":               {"inspectBody": 1},
		"inspectBody":             {"readNote": 1, "extractWikilinksWith": 1, "extractPlannedNamesWith": 1},
		"readNote":                {"parseNoteWithMarks": 1, "parseFrontmatter": 1},
		"extractWikilinksWith":    {"extractWikilinks": 1},
		"extractPlannedNamesWith": {"NewPlanned": 1, "extractPlannedNames": 1},
		"extractWikilinks":        {"LinkTargets": 1},
		"extractPlannedNames":     {},
		"LinkTargets":             {},
		"NewPlanned":              {},
	}
}

func checkBodyStructureLock(files map[string]*ast.File) error {
	want := bodyStructureCallers()
	observed := make(map[string]map[string]int, len(want))
	for name := range want {
		observed[name] = make(map[string]int)
	}
	var violations []string
	for filename, file := range files {
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			var parent ast.Node
			if len(stack) != 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, node)
			identifier, ok := node.(*ast.Ident)
			if !ok || observed[identifier.Name] == nil {
				return true
			}
			if selector, ok := parent.(*ast.SelectorExpr); ok && selector.Sel == identifier {
				return true
			}
			if declaration, ok := parent.(*ast.FuncDecl); ok && declaration.Name == identifier {
				return true
			}
			if field, ok := parent.(*ast.Field); ok && slices.Contains(field.Names, identifier) {
				return true
			}
			caller := "<package>"
			repeated := false
			for _, ancestor := range stack {
				switch ancestor := ancestor.(type) {
				case *ast.FuncDecl:
					caller = ancestor.Name.Name
				case *ast.FuncLit:
					caller = "<literal>"
				case *ast.ForStmt, *ast.RangeStmt, *ast.GoStmt, *ast.DeferStmt:
					repeated = true
				}
			}
			call, direct := parent.(*ast.CallExpr)
			if !direct || call.Fun != identifier {
				violations = append(violations, fmt.Sprintf("%s: %s references guarded function value %s", filename, caller, identifier.Name))
				return true
			}
			observed[identifier.Name][caller]++
			if repeated && (identifier.Name == "structure" || identifier.Name == "inspectBody" || identifier.Name == "readNote") {
				violations = append(violations, fmt.Sprintf("%s: %s calls %s in a loop, go or defer", filename, caller, identifier.Name))
			}
			return true
		})
	}
	if diff := cmp.Diff(want, observed); diff != "" {
		violations = append(violations, "body inspection callers differ (-want +got):\n"+diff)
	}
	slices.Sort(violations)
	if len(violations) != 0 {
		return errors.New(strings.Join(violations, "\n"))
	}
	return nil
}

// These parser inputs exercise the caller contract, not judge semantics.
// The independent harvest comparison owns the values produced by the parser.
func TestBodyStructureLock(t *testing.T) {
	t.Parallel()
	const source = `package judge
func structure() {}
func inspectBody() { structure() }
func readNote() { if marks != nil { facts := inspectBody(); harvest(facts) }; return n }
func parseNoteWithMarks() { return readNote(rel, data, &marks) }
func parseFrontmatter() { return readNote(rel, data, nil) }
func extractWikilinksWith() { inspectBody() }
func extractPlannedNamesWith() { inspectBody() }
func extractWikilinks() { extractWikilinksWith() }
func extractPlannedNames() { extractPlannedNamesWith() }
func LinkTargets() { extractWikilinks() }
func NewPlanned() { extractPlannedNamesWith() }
func harvest(facts any) { first(facts); second(facts) }
`
	for _, tt := range []struct {
		name  string
		old   string
		new   string
		extra string
		want  string
	}{
		{name: "valid"},
		{name: "local rename", old: "facts := inspectBody(); harvest(facts)", new: "shared := inspectBody(); harvest(shared)"},
		{name: "parameter rename", old: "readNote(rel, data, &marks)", new: "readNote(path, data, &marks)"},
		{name: "reversed nil comparison", old: "marks != nil", new: "nil != marks"},
		{name: "helper extraction", old: "first(facts); second(facts)", new: "combined(facts)", extra: "func combined(shared any) { first(shared); second(shared) }"},
		{name: "ordinary parser hook", extra: "var parseHook = parseNote\nfunc parseNote() { parseNoteWithMarks() }"},
		{name: "duplicate platform helper", extra: "func platformHelper() {}\nfunc platformHelper() {}"},
		{name: "field declaration", extra: "type hooks struct { inspectBody func() }"},
		{name: "unrelated callbacks", extra: "var callback = harvest\nfunc other() { fn := harvest; fn() }"},
		{name: "strings comments and foreign selectors", extra: "// inspectBody()\nfunc other() { _ = \"structure()\"; foreign.inspectBody(); foreign.structure() }"},
		{name: "double inspection", old: "facts := inspectBody()", new: "facts := inspectBody(); inspectBody()", want: "body inspection callers differ"},
		{name: "direct wrapper", old: "harvest(facts)", new: "extractWikilinksWith()", want: "body inspection callers differ"},
		{name: "method wrapper", extra: "type linkHarvest struct{}\nfunc (linkHarvest) links() { extractWikilinksWith() }", want: "body inspection callers differ"},
		{name: "package literal wrapper", extra: "var relinks = func() { extractWikilinksWith() }", want: "body inspection callers differ"},
		{name: "local literal wrapper", extra: "func other() { relinks := func() { extractWikilinksWith() }; relinks() }", want: "body inspection callers differ"},
		{name: "helper wrapper", extra: "func helper() { extractWikilinksWith() }", want: "body inspection callers differ"},
		{name: "inspection moved into literal", old: "facts := inspectBody()", new: "facts := func() { inspectBody() }()", want: "body inspection callers differ"},
		{name: "package initializer", extra: "var extra = inspectBody()", want: "body inspection callers differ"},
		{name: "duplicate caller counts", extra: "func extractWikilinks() { extractWikilinksWith() }", want: "body inspection callers differ"},
		{name: "loop inspection", old: "facts := inspectBody()", new: "for { facts := inspectBody() }", want: "calls inspectBody in a loop, go or defer"},
		{name: "range inspection", old: "facts := inspectBody()", new: "for range values { facts := inspectBody() }", want: "calls inspectBody in a loop, go or defer"},
		{name: "go inspection", old: "facts := inspectBody()", new: "go inspectBody()", want: "calls inspectBody in a loop, go or defer"},
		{name: "deferred inspection", old: "facts := inspectBody()", new: "defer inspectBody()", want: "calls inspectBody in a loop, go or defer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := source
			if tt.old != "" {
				if count := strings.Count(input, tt.old); count != 1 {
					t.Fatalf("control edit sites = %d, want 1", count)
				}
				input = strings.Replace(input, tt.old, tt.new, 1)
			}
			checkBodyStructureControl(t, input+tt.extra, tt.want)
		})
	}
	var names []string
	for name := range bodyStructureCallers() {
		names = append(names, name)
	}
	slices.Sort(names)
	wantNames := []string{"LinkTargets", "NewPlanned", "extractPlannedNames", "extractPlannedNamesWith", "extractWikilinks", "extractWikilinksWith", "inspectBody", "readNote", "structure"}
	if diff := cmp.Diff(wantNames, names); diff != "" {
		t.Fatalf("ruled routes differ (-want +got):\n%s", diff)
	}
	// Every ruled route gets both an extra edge and a non-call reference control.
	for name := range bodyStructureCallers() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkBodyStructureControl(t, source+"func extra() { "+name+"() }", "body inspection callers differ")
			checkBodyStructureControl(t, source+"var alias = "+name, "guarded function value "+name)
		})
	}
	t.Run("all violations sorted", func(t *testing.T) {
		t.Parallel()
		file, err := parser.ParseFile(token.NewFileSet(), "control.go", source+"var z = structure\nvar a = inspectBody", 0)
		if err != nil {
			t.Fatalf("parse control: %v", err)
		}
		err = checkBodyStructureLock(map[string]*ast.File{"control.go": file})
		want := "control.go: <package> references guarded function value inspectBody\ncontrol.go: <package> references guarded function value structure"
		if err == nil || err.Error() != want {
			t.Errorf("source lock error = %v, want %q", err, want)
		}
	})
}

func checkBodyStructureControl(t *testing.T, input, want string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "control.go", input, 0)
	if err != nil {
		t.Fatalf("parse control: %v", err)
	}
	err = checkBodyStructureLock(map[string]*ast.File{"control.go": file})
	if want == "" {
		if err != nil {
			t.Errorf("valid source lock error = %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("source lock error = %v, want %q", err, want)
	}
}
