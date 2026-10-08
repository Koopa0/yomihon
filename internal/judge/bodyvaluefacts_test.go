package judge_test

import (
	"crypto/sha256"
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

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Literal page IDs and contents words are checked before the public judge's
// fragment verdict, so agreement between two wrong producers cannot pass.
func TestRichHeadingValuesReachThePageAndCheck(t *testing.T) {
	t.Parallel()
	bodyRichHeadingControls(t)
}

func bodyRichHeadingControls(t *testing.T) {
	t.Helper()
	for _, tt := range []struct {
		name string
		body string
		want []render.TOCEntry
	}{
		{
			name: "comment join keeps code",
			body: "## A%%hidden%%B `Code`\n",
			want: []render.TOCEntry{{Level: 2, Text: "AB Code", ID: "ab-code"}},
		},
		{
			name: "ruby and branch instruction",
			body: "## <ruby>Base<rt>reading</rt></ruby> `Code` {sequence=primary}\n",
			want: []render.TOCEntry{{Level: 2, Text: "Base Code", ID: "base-code"}},
		},
		{
			name: "setext words",
			body: "Underlined `Code`\n---\n",
			want: []render.TOCEntry{{Level: 2, Text: "Underlined Code", ID: "underlined-code"}},
		},
		{
			name: "used footnote placement",
			body: "## Main\n\nref[^used]\n\n[^unused]:\n    ## Hidden\n\n[^used]:\n    ## Shown\n",
			want: []render.TOCEntry{{Level: 2, Text: "Main", ID: "main"}, {Level: 2, Text: "Shown", ID: "shown"}},
		},
	} {
		page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
		result := page.HTML("Notes/Reading.md", "", tt.body, wording.En)
		if len(result.Diagnostics) != 0 {
			t.Fatalf("not-applied: rich heading page diagnostics=%+v", result.Diagnostics)
		}
		if diff := cmp.Diff(tt.want, result.TOC); diff != "" {
			t.Errorf("caught: F3 rich-page-contents (-want +got):\n%s", diff)
		}
		actual := agreementObserve(t, result.HTML)
		ids := make([]string, len(tt.want))
		for i := range tt.want {
			ids[i] = tt.want[i].ID
		}
		if diff := cmp.Diff(ids, actual.Headings); diff != "" {
			t.Errorf("caught: F3 rich-page-id (-want +got):\n%s", diff)
		}
		failures := agreementFragmentFailures(t, []agreementCase{{Name: tt.name, Body: tt.body}}, []agreementHTML{actual})
		if len(failures) != 1 {
			t.Fatalf("not-applied: rich heading verdict batches=%d", len(failures))
		}
		if len(failures[0]) != 0 {
			t.Errorf("caught: F3 rich-public-check failures=%+v", failures[0])
		}
	}
}

// Both consumers borrow body facts. A parser hidden behind a local helper or
// receiver method still owns a second tree and is rejected by this control.
func TestConsumerBodyTreesStayGraphOwned(t *testing.T) {
	t.Parallel()
	bodyConsumerOwnershipControl(t, "")
}

func bodyConsumerOwnershipControl(t *testing.T, alternatePath string) {
	t.Helper()
	for _, target := range []struct {
		path  string
		roots []string
	}{
		{path: "../sequence", roots: []string{"Parse", "ParseFacts"}},
		{path: ".", roots: []string{"anchorSurfaceFrom", "collectParsedHeadings"}},
	} {
		files := bodyConsumerFiles(t, target.path, alternatePath)
		if target.path == "../sequence" {
			bodyConsumerImportsControl(t, files)
		}
		for _, violation := range bodyConsumerViolations(files, target.roots) {
			t.Errorf("caught: F3 consumer-whole-body-parse %s", violation)
		}
	}
}

func bodyConsumerImportsControl(t *testing.T, files []*ast.File) {
	t.Helper()
	for _, file := range files {
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("not-applied: decode consumer import: %v", err)
			}
			if path == "github.com/yuin/goldmark" || strings.HasPrefix(path, "github.com/yuin/goldmark/") {
				t.Errorf("caught: F3 consumer-whole-body-parse imported=%s", path)
			}
		}
	}
}

func bodyConsumerFiles(t *testing.T, directory, alternatePath string) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("not-applied: read consumer directory: %v", err)
	}
	var files []*ast.File
	consumed := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, readErr := os.ReadFile(path)
		if directory == "../sequence" && entry.Name() == "sequence.go" && alternatePath != "" {
			data, readErr = os.ReadFile(alternatePath)
			consumed = true
		}
		if readErr != nil {
			t.Fatalf("not-applied: read consumer source: %v", readErr)
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if parseErr != nil {
			t.Fatalf("not-applied: parse consumer source: %v", parseErr)
		}
		if directory == "../sequence" && entry.Name() == "sequence.go" {
			t.Logf("AGREEMENT-SOURCE-CONSUMED sha256=%x", sha256.Sum256(data))
		}
		files = append(files, file)
	}
	if len(files) == 0 || directory == "../sequence" && alternatePath != "" && !consumed {
		t.Fatal("not-applied: consumer source inventory is empty or alternate was not consumed")
	}
	return files
}

type bodyConsumerFunction struct {
	declaration *ast.FuncDecl
	imports     map[string]bool
}

type bodyConsumerSource struct {
	node    ast.Node
	imports map[string]bool
}

type bodyConsumerIndex struct {
	functions     map[string][]bodyConsumerFunction
	globals       map[string][]ast.Expr
	globalImports map[string]map[string]bool
	methods       map[string]bool
	fields        map[string]map[string]ast.Expr
}

func bodyConsumerViolations(files []*ast.File, roots []string) []string {
	index := bodyIndexConsumers(files)
	visited := make(map[string]bool)
	queue := slices.Clone(roots)
	var violations []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		functions, exists := index.functions[name]
		if !exists {
			violations = append(violations, "missing function="+name)
			continue
		}
		for _, function := range functions {
			next, found := index.references(name, function)
			queue = append(queue, next...)
			violations = append(violations, found...)
		}
	}
	return violations
}

func bodyIndexConsumers(files []*ast.File) bodyConsumerIndex {
	index := bodyConsumerIndex{
		functions:     make(map[string][]bodyConsumerFunction),
		globals:       make(map[string][]ast.Expr),
		globalImports: make(map[string]map[string]bool),
		methods:       make(map[string]bool),
		fields:        make(map[string]map[string]ast.Expr),
	}
	for _, file := range files {
		imports := make(map[string]bool)
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				continue
			}
			name := filepath.Base(path)
			if imported.Name != nil {
				name = imported.Name.Name
			}
			imports[name] = true
		}
		bodyIndexFields(file, index.fields)
		bodyIndexGlobals(file, index.globals, index.globalImports, imports)
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				index.methods[name] = true
				name = bodyConsumerTypeName(fn.Recv.List[0].Type) + "." + name
			}
			index.functions[name] = append(index.functions[name], bodyConsumerFunction{declaration: fn, imports: imports})
		}
	}
	return index
}

func bodyIndexGlobals(file *ast.File, globals map[string][]ast.Expr, globalImports map[string]map[string]bool, imports map[string]bool) {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			bodyBindValues(value, globals)
			for _, name := range value.Names {
				globalImports[name.Name] = imports
			}
		}
	}
}

func bodyBindValues(value *ast.ValueSpec, bindings map[string][]ast.Expr) {
	for i, name := range value.Names {
		if value.Type != nil {
			bindings[name.Name] = append(bindings[name.Name], value.Type)
		}
		if i < len(value.Values) {
			bindings[name.Name] = append(bindings[name.Name], value.Values[i])
		}
	}
}

func bodyIndexFields(file *ast.File, fields map[string]map[string]ast.Expr) {
	ast.Inspect(file, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structure, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		members := make(map[string]ast.Expr)
		for _, field := range structure.Fields.List {
			for _, name := range field.Names {
				members[name.Name] = field.Type
			}
		}
		fields[typeSpec.Name.Name] = members
		return false
	})
}

func bodyConsumerTypeName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return bodyConsumerTypeName(value.X)
	case *ast.ArrayType:
		return bodyConsumerTypeName(value.Elt)
	case *ast.MapType:
		return bodyConsumerTypeName(value.Value)
	case *ast.ParenExpr:
		return bodyConsumerTypeName(value.X)
	case *ast.SelectorExpr:
		return bodyConsumerTypeName(value.X) + "." + value.Sel.Name
	}
	return ""
}

func bodyConsumerBindings(fn *ast.FuncDecl) map[string][]ast.Expr {
	bindings := make(map[string][]ast.Expr)
	for _, list := range []*ast.FieldList{fn.Recv, fn.Type.Params} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				bindings[name.Name] = append(bindings[name.Name], field.Type)
			}
		}
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.AssignStmt:
			bodyBindAssignment(value, bindings)
		case *ast.RangeStmt:
			if name, ok := value.Value.(*ast.Ident); ok {
				bindings[name.Name] = append(bindings[name.Name], value.X)
			}
		case *ast.ValueSpec:
			bodyBindValues(value, bindings)
		}
		return true
	})
	return bindings
}

func bodyBindAssignment(value *ast.AssignStmt, bindings map[string][]ast.Expr) {
	if len(value.Lhs) != len(value.Rhs) {
		return
	}
	for i, lhs := range value.Lhs {
		if name, ok := lhs.(*ast.Ident); ok {
			bindings[name.Name] = append(bindings[name.Name], value.Rhs[i])
		}
	}
}

type bodyConsumerWalk struct {
	bindings   map[string][]ast.Expr
	locals     map[string]bool
	seen       map[string]bool
	pending    []bodyConsumerSource
	next       []string
	violations []string
}

func (index bodyConsumerIndex) references(name string, function bodyConsumerFunction) ([]string, []string) {
	walk := bodyConsumerWalk{
		bindings: bodyConsumerBindings(function.declaration),
		locals:   make(map[string]bool),
		seen:     make(map[string]bool),
		pending:  []bodyConsumerSource{{node: function.declaration.Body, imports: function.imports}},
	}
	for variable := range walk.bindings {
		walk.locals[variable] = true
	}
	for variable, sources := range index.globals {
		if !walk.locals[variable] {
			walk.bindings[variable] = sources
		}
	}
	for len(walk.pending) > 0 {
		root := walk.pending[0]
		walk.pending = walk.pending[1:]
		ast.Inspect(root.node, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.Ident:
				index.identifierReferences(value.Name, root.imports, &walk)
			case *ast.SelectorExpr:
				called, found, descend := index.selectorReferences(name, value, root.imports, walk.bindings)
				walk.next = append(walk.next, called...)
				walk.violations = append(walk.violations, found...)
				return descend
			}
			return true
		})
	}
	return walk.next, walk.violations
}

func (index bodyConsumerIndex) identifierReferences(name string, imports map[string]bool, walk *bodyConsumerWalk) {
	if _, exists := index.functions[name]; exists && !imports[name] && len(walk.bindings[name]) == 0 {
		walk.next = append(walk.next, name)
	}
	if walk.seen[name] || walk.locals[name] || imports[name] {
		return
	}
	walk.seen[name] = true
	for _, source := range index.globals[name] {
		walk.pending = append(walk.pending, bodyConsumerSource{node: source, imports: index.globalImports[name]})
	}
}

func (index bodyConsumerIndex) selectorReferences(name string, selector *ast.SelectorExpr, imports map[string]bool, bindings map[string][]ast.Expr) ([]string, []string, bool) {
	method := selector.Sel.Name
	var violations []string
	if method == "NewBodyMarkdown" || method == "NewBodyGrammar" || method == "Parser" || method == "Parse" || method == "Convert" {
		violations = append(violations, "function="+name+" selector="+method)
	}
	if receiver, ok := selector.X.(*ast.Ident); ok && imports[receiver.Name] && len(bindings[receiver.Name]) == 0 {
		return nil, violations, false
	}
	if !index.methods[method] {
		return nil, violations, true
	}
	var next []string
	resolved := false
	for _, receiver := range index.receiverTypes(selector.X, bindings, 0) {
		key := receiver + "." + method
		if _, exists := index.functions[key]; exists {
			next = append(next, key)
			resolved = true
		}
		prefix, _, qualified := strings.Cut(receiver, ".")
		if qualified && imports[prefix] {
			resolved = true
		}
	}
	if !resolved {
		violations = append(violations, "function="+name+" unresolved-local-method="+method)
	}
	return next, violations, true
}

func (index bodyConsumerIndex) receiverTypes(expr ast.Expr, bindings map[string][]ast.Expr, depth int) []string {
	if depth > 12 {
		return nil
	}
	switch value := expr.(type) {
	case *ast.Ident:
		return index.identifierTypes(value.Name, bindings, depth)
	case *ast.UnaryExpr:
		return index.receiverTypes(value.X, bindings, depth+1)
	case *ast.StarExpr:
		return index.receiverTypes(value.X, bindings, depth+1)
	case *ast.ParenExpr:
		return index.receiverTypes(value.X, bindings, depth+1)
	case *ast.CompositeLit:
		return []string{bodyConsumerTypeName(value.Type)}
	case *ast.IndexExpr:
		return index.receiverTypes(value.X, bindings, depth+1)
	case *ast.SliceExpr:
		return index.receiverTypes(value.X, bindings, depth+1)
	case *ast.ArrayType:
		return []string{bodyConsumerTypeName(value.Elt)}
	case *ast.MapType:
		return []string{bodyConsumerTypeName(value.Value)}
	case *ast.SelectorExpr:
		return index.fieldTypes(value, bindings, depth)
	case *ast.CallExpr:
		return index.callResultTypes(value, bindings, depth)
	}
	return nil
}

func (index bodyConsumerIndex) identifierTypes(name string, bindings map[string][]ast.Expr, depth int) []string {
	sources, exists := bindings[name]
	if !exists {
		return []string{name}
	}
	var types []string
	for _, source := range sources {
		types = append(types, index.receiverTypes(source, bindings, depth+1)...)
	}
	return types
}

func (index bodyConsumerIndex) fieldTypes(value *ast.SelectorExpr, bindings map[string][]ast.Expr, depth int) []string {
	var types []string
	for _, receiver := range index.receiverTypes(value.X, bindings, depth+1) {
		if field, exists := index.fields[receiver][value.Sel.Name]; exists {
			types = append(types, bodyConsumerTypeName(field))
		}
	}
	if len(types) == 0 {
		return []string{bodyConsumerTypeName(value)}
	}
	return types
}

func (index bodyConsumerIndex) callResultTypes(call *ast.CallExpr, bindings map[string][]ast.Expr, depth int) []string {
	var keys []string
	switch callee := call.Fun.(type) {
	case *ast.Ident:
		if callee.Name == "new" && len(call.Args) == 1 {
			return index.receiverTypes(call.Args[0], bindings, depth+1)
		}
		keys = append(keys, callee.Name)
	case *ast.SelectorExpr:
		for _, receiver := range index.receiverTypes(callee.X, bindings, depth+1) {
			keys = append(keys, receiver+"."+callee.Sel.Name)
		}
	}
	return index.resultTypes(keys)
}

func (index bodyConsumerIndex) resultTypes(keys []string) []string {
	var types []string
	for _, key := range keys {
		for _, function := range index.functions[key] {
			if function.declaration.Type.Results == nil {
				continue
			}
			for _, field := range function.declaration.Type.Results.List {
				types = append(types, bodyConsumerTypeName(field.Type))
			}
		}
	}
	return types
}

func TestBodyConsumerOwnershipSourceControls(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   bool
	}{
		{name: "direct parser", source: `package p; func Root() { mdParser.Parse(reader) }`, want: true},
		{name: "method value aliases", source: `package p; func Root() { parse := mdParser.Parse; alias := parse; alias(reader) }`, want: true},
		{name: "package helper alias", source: `package p; func Root() { helper := parseBody; next := helper; next() }; func parseBody() { mdParser.Parse(reader) }`, want: true},
		{name: "global helper alias", source: `package p; var helper = parseBody; func Root() { helper() }; func parseBody() { mdParser.Parse(reader) }`, want: true},
		{name: "typed receiver", source: `package p; type worker struct{}; func Root(w *worker) { var copy worker; copy.read(); w.read() }; func (w worker) read() { mdParser.Parse(reader) }`, want: true},
		{name: "unresolved receiver refuses", source: `package p; type worker struct{}; func Root() { unknown.read() }; func (w worker) read() {}`, want: true},
		{name: "receiver dispatch", source: `package p; type worker struct{}; func Root() { w := &worker{}; run := w.read; run() }; func (w *worker) read() { mdParser.Parse(reader) }`, want: true},
		{name: "import alias is not local", source: `package p; import helper "example.invalid/other"; func Root() { helper.read() }; type worker struct{}; func (w worker) read() { mdParser.Parse(reader) }; func read() { mdParser.Parse(reader) }`, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), tc.name, tc.source, 0)
			if err != nil {
				t.Fatalf("source control parse: %v", err)
			}
			violations := bodyConsumerViolations([]*ast.File{file}, []string{"Root"})
			if got := len(violations) != 0; got != tc.want {
				t.Errorf("ownership source control violations=%q, want rejected=%t", violations, tc.want)
			}
		})
	}
}
