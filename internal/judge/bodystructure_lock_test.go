package judge

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The lock reads every production file, including future files. Allocation
// measurements cannot distinguish another parse from regexp pool noise.
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

func checkBodyStructureLock(files map[string]*ast.File) error {
	if len(files) == 0 {
		return fmt.Errorf("no production judge source")
	}
	functions := make(map[string]*ast.FuncDecl)
	for name, file := range files {
		if file.Name.Name != "judge" {
			return fmt.Errorf("%s has package %s, want judge", name, file.Name.Name)
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if functions[fn.Name.Name] != nil {
				return fmt.Errorf("duplicate declaration %s", fn.Name.Name)
			}
			functions[fn.Name.Name] = fn
		}
	}
	want := map[string]map[string]int{
		"structure":   {"inspectBody": 1},
		"inspectBody": {"readNote": 1, "extractWikilinksWith": 1, "extractPlannedNamesWith": 1},
		"readNote":    {"parseNoteWithMarks": 1, "parseFrontmatter": 1},
	}
	from := []string{"extractWikilinksFrom", "extractPathRefsFrom", "extractPlannedNamesFrom", "extractCalloutTitlesFrom", "anchorSurfaceFrom"}
	wrappers := map[string]bool{"extractWikilinksWith": true, "extractPlannedNamesWith": true}
	guardedValues := map[string]bool{"structure": true, "inspectBody": true, "readNote": true}
	for name := range wrappers {
		guardedValues[name] = true
	}
	for _, name := range from {
		guardedValues[name] = true
	}
	required := []string{"structure", "inspectBody", "parseNoteWithMarks", "parseFrontmatter", "readNote", "extractWikilinksWith", "extractPlannedNamesWith"}
	required = append(required, from...)
	for _, name := range required {
		if functions[name] == nil || functions[name].Body == nil {
			return fmt.Errorf("missing function body %s", name)
		}
	}
	observed := map[string]map[string]int{"structure": {}, "inspectBody": {}, "readNote": {}}
	calls := make(map[string]map[string]int)
	var functionValues []struct{ caller, name string }
	var violation error
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
			if !ok {
				return true
			}
			// A qualified selector names another package or a receiver, not
			// one of this package's ordinary functions.
			if selector, ok := parent.(*ast.SelectorExpr); ok && selector.Sel == identifier {
				return true
			}
			if declaration, ok := parent.(*ast.FuncDecl); ok && declaration.Name == identifier {
				return true
			}
			if field, ok := parent.(*ast.Field); ok {
				for _, name := range field.Names {
					if name == identifier {
						return true
					}
				}
			}
			caller := "<package>"
			repeated := false
			for _, ancestor := range stack {
				switch ancestor := ancestor.(type) {
				case *ast.FuncDecl:
					caller = ancestor.Name.Name
				case *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt, *ast.GoStmt, *ast.DeferStmt:
					repeated = true
				}
			}
			call, direct := parent.(*ast.CallExpr)
			direct = direct && call.Fun == identifier
			if !direct {
				// Refuse guarded function values rather than guessing their
				// eventual caller or resolving dynamically assigned aliases.
				if functions[identifier.Name] != nil {
					functionValues = append(functionValues, struct{ caller, name string }{caller, identifier.Name})
				}
				return true
			}
			if calls[caller] == nil {
				calls[caller] = make(map[string]int)
			}
			calls[caller][identifier.Name]++
			if counts, guarded := observed[identifier.Name]; guarded {
				counts[caller]++
				if repeated {
					violation = fmt.Errorf("%s: %s calls %s in a loop, closure, go or defer", filename, caller, identifier.Name)
				}
			}
			return true
		})
	}
	if violation != nil {
		return violation
	}
	// Include aliases of helpers that eventually rebuild a structure, not just
	// aliases of inspectBody itself. Unrelated callback functions remain free.
	rebuilders := map[string]bool{"inspectBody": true, "structure": true}
	for changed := true; changed; {
		changed = false
		for caller, callees := range calls {
			for callee := range callees {
				if rebuilders[callee] && !rebuilders[caller] {
					rebuilders[caller] = true
					changed = true
				}
			}
		}
	}
	// Follow all local helpers, including calls inside closures. A standalone
	// wrapper reached through any helper still rebuilds the shared structure.
	visited := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		visited[name] = true
		for callee := range calls[name] {
			if functions[callee] == nil {
				continue
			}
			if wrappers[callee] {
				return fmt.Errorf("shared path %s calls standalone wrapper %s", name, callee)
			}
			if err := visit(callee); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit("readNote"); err != nil {
		return err
	}
	for _, value := range functionValues {
		name := value.name
		if guardedValues[name] || ((visited[value.caller] || value.caller == "<package>") && rebuilders[name]) {
			return fmt.Errorf("guarded function value %s", name)
		}
	}
	if diff := cmp.Diff(want, observed); diff != "" {
		return fmt.Errorf("body inspection callers differ (-want +got):\n%s", diff)
	}
	if !bodyReaderReturn(functions["parseNoteWithMarks"], true) || !bodyReaderReturn(functions["parseFrontmatter"], false) {
		return fmt.Errorf("body readers must return readNote(rel, data, &marks) and readNote(rel, data, nil)")
	}
	// Pin the one optional body branch and the actual value lent to all five
	// harvesters, rather than only counting an inspectBody token.
	var branch *ast.IfStmt
	for _, statement := range functions["readNote"].Body.List {
		candidate, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		condition, ok := candidate.Cond.(*ast.BinaryExpr)
		if ok && condition.Op == token.NEQ && bodyIdentifier(condition.X, "marks") && bodyIdentifier(condition.Y, "nil") {
			if branch != nil || candidate.Init != nil || candidate.Else != nil {
				return fmt.Errorf("body marks branch must occur once without init or else")
			}
			branch = candidate
		}
	}
	if branch == nil {
		return fmt.Errorf("missing marks != nil body branch")
	}
	borrowed := make(map[string]int)
	inspection := 0
	for _, statement := range branch.Body.List {
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 {
			continue
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok {
			continue
		}
		if name.Name == "inspectBody" {
			if assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || !bodyIdentifier(assignment.Lhs[0], "facts") || len(call.Args) != 2 || !bodyIdentifier(call.Args[0], "body") || !bodySelector(call.Args[1], "marks", "heading") {
				return fmt.Errorf("body inspection must declare facts from body and marks.heading")
			}
			inspection++
		}
		for _, harvester := range from {
			if name.Name != harvester {
				continue
			}
			if inspection != 1 || len(call.Args) < 2 || !bodyIdentifier(call.Args[0], "body") {
				return fmt.Errorf("%s must borrow the inspected body", harvester)
			}
			last := call.Args[len(call.Args)-1]
			if harvester == "extractWikilinksFrom" || harvester == "extractPlannedNamesFrom" {
				address, ok := last.(*ast.UnaryExpr)
				if !ok || address.Op != token.AND || !bodyIdentifier(address.X, "facts") {
					return fmt.Errorf("%s must borrow &facts", harvester)
				}
			} else if !bodySelector(last, "facts", "comments") {
				return fmt.Errorf("%s must borrow facts.comments", harvester)
			}
			borrowed[harvester]++
		}
	}
	wantBorrowed := make(map[string]int)
	for _, name := range from {
		wantBorrowed[name] = 1
		if calls["readNote"][name] != 1 {
			return fmt.Errorf("readNote must call %s once", name)
		}
	}
	if inspection != 1 {
		return fmt.Errorf("body branch inspections = %d, want 1", inspection)
	}
	if diff := cmp.Diff(wantBorrowed, borrowed); diff != "" {
		return fmt.Errorf("shared harvests differ (-want +got):\n%s", diff)
	}
	return nil
}

func bodyIdentifier(expression ast.Expr, name string) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == name
}

func bodySelector(expression ast.Expr, name, field string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && bodyIdentifier(selector.X, name) && selector.Sel.Name == field
}

func bodyReaderReturn(fn *ast.FuncDecl, marked bool) bool {
	if len(fn.Body.List) != 1 {
		return false
	}
	statement, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(statement.Results) != 1 {
		return false
	}
	call, ok := statement.Results[0].(*ast.CallExpr)
	if !ok || !bodyIdentifier(call.Fun, "readNote") || len(call.Args) != 3 || !bodyIdentifier(call.Args[0], "rel") || !bodyIdentifier(call.Args[1], "data") {
		return false
	}
	if !marked {
		return bodyIdentifier(call.Args[2], "nil")
	}
	address, ok := call.Args[2].(*ast.UnaryExpr)
	return ok && address.Op == token.AND && bodyIdentifier(address.X, "marks")
}

// These are parser inputs, not executable substitutes for judge behavior.
// Each rejected input names a distinct way the source lock could be bypassed.
func TestBodyStructureLock(t *testing.T) {
	t.Parallel()
	const source = `package judge
func structure(body string, marks []string) {}
func inspectBody(body string, marks []string) { structure(body, marks) }
func extractWikilinksWith() { inspectBody(body, marks) }
func extractPlannedNamesWith() { inspectBody(body, marks) }
func extractWikilinksFrom() {}
func extractPathRefsFrom() {}
func extractPlannedNamesFrom() {}
func extractCalloutTitlesFrom() {}
func anchorSurfaceFrom() {}
func parseNoteWithMarks() { return readNote(rel, data, &marks) }
func parseFrontmatter() { return readNote(rel, data, nil) }
func readNote() {
 if marks != nil {
  facts := inspectBody(body, marks.heading)
  n.wikilinks = extractWikilinksFrom(body, line, &facts)
  n.pathRefs = extractPathRefsFrom(body, line, facts.comments)
  n.plannedNames = extractPlannedNamesFrom(body, marks, &facts)
  n.calloutTitles = extractCalloutTitlesFrom(body, line, facts.comments)
  n.anchors = anchorSurfaceFrom(body, facts.comments)
 }
 return n
}
`
	for _, tt := range []struct {
		name  string
		old   string
		new   string
		extra string
		want  string
	}{
		{name: "valid"},
		{name: "public parser capture", extra: "func action() { parse := parseNoteWithMarks; parse() }"},
		{name: "unrelated with helper on shared path", old: "return n", new: "metadataWith(); return n", extra: "func metadataWith() {}"},
		{name: "unrelated from and with callbacks", extra: "func other() { from := callbackFrom; with := callbackWith; from(); with() }\nfunc callbackFrom() {}\nfunc callbackWith() {}"},
		{name: "unrelated package callback", extra: "var callback = callbackWith\nfunc callbackWith() {}"},
		{name: "parser hook field declaration", extra: "type actionHooks struct { parseNoteWithMarks func() }"},
		{name: "missing declaration", old: "func extractWikilinksFrom() {}", new: "", want: "missing function body extractWikilinksFrom"},
		{name: "duplicate declaration", extra: "func extractWikilinksFrom() {}", want: "duplicate declaration extractWikilinksFrom"},
		{name: "double inspection", old: "facts := inspectBody(body, marks.heading)", new: "facts := inspectBody(body, marks.heading); inspectBody(body, marks.heading)", want: "body inspection callers differ"},
		{name: "new structure caller", extra: "func other() { structure(body, marks) }", want: "body inspection callers differ"},
		{name: "new inspection caller", extra: "func other() { inspectBody(body, marks) }", want: "body inspection callers differ"},
		{name: "recursive reader", old: "return n", new: "readNote(rel, data, marks); return n", want: "body inspection callers differ"},
		{name: "from inspection", old: "func extractWikilinksFrom() {}", new: "func extractWikilinksFrom() { inspectBody(body, marks) }", want: "body inspection callers differ"},
		{name: "direct standalone wrapper", old: "extractWikilinksFrom(body, line, &facts)", new: "extractWikilinksWith(body, line, marks)", want: "shared path readNote calls standalone wrapper extractWikilinksWith"},
		{name: "indirect helper wrapper", old: "func extractWikilinksFrom() {}", new: "func extractWikilinksFrom() { helper() }", extra: "func helper() { extractWikilinksWith() }", want: "calls standalone wrapper extractWikilinksWith"},
		{name: "from closure wrapper", old: "func extractWikilinksFrom() {}", new: "func extractWikilinksFrom() { callback := func() { extractWikilinksWith() }; callback() }", want: "calls standalone wrapper extractWikilinksWith"},
		{name: "inspection alias", old: "facts := inspectBody(body, marks.heading)", new: "inspect := inspectBody; facts := inspect(body, marks.heading)", want: "guarded function value inspectBody"},
		{name: "wrapper alias", old: "func extractWikilinksFrom() {}", new: "func extractWikilinksFrom() { wrapper := extractWikilinksWith; wrapper() }", want: "guarded function value extractWikilinksWith"},
		{name: "planned wrapper alias", old: "func extractWikilinksFrom() {}", new: "func extractWikilinksFrom() { wrapper := extractPlannedNamesWith; wrapper() }", want: "guarded function value extractPlannedNamesWith"},
		{name: "owned harvester alias", extra: "func other() { harvest := extractPathRefsFrom; harvest() }", want: "guarded function value extractPathRefsFrom"},
		{name: "helper alias", old: "func extractWikilinksFrom() {}", new: "func extractWikilinksFrom() { wrapper := helper; wrapper() }", extra: "func helper() { extractWikilinksWith() }", want: "guarded function value helper"},
		{name: "package rebuilding helper alias", old: "return n", new: "redundantWikilinks(); return n", extra: "var redundantWikilinks = helper\nfunc helper() { extractWikilinksWith() }", want: "guarded function value helper"},
		{name: "loop inspection", old: "facts := inspectBody(body, marks.heading)", new: "for { facts := inspectBody(body, marks.heading) }", want: "calls inspectBody in a loop, closure, go or defer"},
		{name: "closure inspection", old: "facts := inspectBody(body, marks.heading)", new: "func() { facts := inspectBody(body, marks.heading) }()", want: "calls inspectBody in a loop, closure, go or defer"},
		{name: "go inspection", old: "facts := inspectBody(body, marks.heading)", new: "go inspectBody(body, marks.heading)", want: "calls inspectBody in a loop, closure, go or defer"},
		{name: "deferred inspection", old: "facts := inspectBody(body, marks.heading)", new: "defer inspectBody(body, marks.heading)", want: "calls inspectBody in a loop, closure, go or defer"},
		{name: "marks bypass", old: "return readNote(rel, data, &marks)", new: "return readNote(rel, data, nil)", want: "body readers must return"},
		{name: "frontmatter body extraction", old: "return readNote(rel, data, nil)", new: "return readNote(rel, data, &marks)", want: "body readers must return"},
		{name: "missing marks guard", old: "if marks != nil", new: "if true", want: "missing marks != nil body branch"},
		{name: "different structure", old: "extractWikilinksFrom(body, line, &facts)", new: "extractWikilinksFrom(body, line, &other)", want: "extractWikilinksFrom must borrow &facts"},
		{name: "different comments", old: "anchorSurfaceFrom(body, facts.comments)", new: "anchorSurfaceFrom(body, other.comments)", want: "anchorSurfaceFrom must borrow facts.comments"},
		{name: "missing harvest", old: "n.anchors = anchorSurfaceFrom(body, facts.comments)", new: "", want: "readNote must call anchorSurfaceFrom once"},
		{name: "comments strings and selectors", extra: "// inspectBody(body, marks)\nfunc unrelated() { text := \"structure(body, marks)\"; other.inspectBody(); other.structure(); callback := unrelatedCallback; callback(); _ = text }\nfunc unrelatedCallback() {}"},
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
			input += tt.extra
			file, err := parser.ParseFile(token.NewFileSet(), "control.go", input, 0)
			if err != nil {
				t.Fatalf("parse control: %v", err)
			}
			err = checkBodyStructureLock(map[string]*ast.File{"control.go": file})
			if tt.want == "" {
				if err != nil {
					t.Errorf("valid source lock error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("source lock error = %v, want %q", err, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		files map[string]*ast.File
		want  string
	}{
		{name: "empty package", files: map[string]*ast.File{}, want: "no production judge source"},
		{name: "wrong package", files: map[string]*ast.File{"other.go": {Name: ast.NewIdent("other")}}, want: "has package other, want judge"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := checkBodyStructureLock(tt.files); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("source lock error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBodyStructureSource(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		file string
		body string
		want string
	}{
		{name: "empty", want: "no production judge source"},
		{name: "test files excluded", file: "body_test.go", body: "not Go", want: "no production judge source"},
		{name: "parse failure", file: "body.go", body: "package judge\nfunc broken(", want: "parse body.go:"},
		{name: "wrong package", file: "body.go", body: "package other", want: "has package other, want judge"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if tt.file != "" {
				path := filepath.Join(directory, tt.file)
				if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil { // #nosec G703 -- path is inside this test's private TempDir
					t.Fatalf("write source control: %v", err)
				}
			}
			if err := judgeBodyStructureLock(directory); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("source lock error = %v, want %q", err, tt.want)
			}
		})
	}
}
