package judge

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Consumers and writers are inventoried independently: an unused declaration
// cannot cover a golden, and deleting a writer row cannot erase its consumer.
func TestRegenerateGoldensCoversCheckConsumers(t *testing.T) {
	t.Parallel()
	consumers := readGoldenInventory(t, "check_test.go", checkGoldenConsumers)
	writers := readGoldenInventory(t, "regen_goldens_test.go", engineGoldenWriters)
	for _, golden := range slices.Sorted(maps.Keys(consumers)) {
		if !writers[golden] {
			t.Errorf("caught: golden regeneration missing %s", golden)
		}
	}
}

func readGoldenInventory(t *testing.T, name string, inventory func(*token.FileSet, *ast.File) (map[string]bool, error)) map[string]bool {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, name, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	paths, err := inventory(positions, file)
	if err != nil {
		t.Fatalf("inventory %s: %v", name, err)
	}
	if len(paths) == 0 {
		t.Fatalf("inventory %s: no consumed goldens", name)
	}
	return paths
}

func checkGoldenConsumers(positions *token.FileSet, file *ast.File) (map[string]bool, error) {
	paths := make(map[string]bool)
	var failure error
	var ancestors []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return true
		}
		ancestors = append(ancestors, node)
		call, ok := node.(*ast.CallExpr)
		if !ok || failure != nil {
			return true
		}
		if !goldenCall(call, "os", "ReadFile") {
			if unsupportedGoldenCall(call) {
				failure = goldenSourceError(positions, call, "unsupported golden reader call")
			}
			return true
		}
		if len(call.Args) != 1 {
			failure = goldenSourceError(positions, call, "unsupported golden reader")
			return true
		}
		if literal, ok := call.Args[0].(*ast.BasicLit); ok {
			failure = addGoldenPath(positions, paths, literal)
			return true
		}
		rows, err := rangedGoldenConsumers(positions, ancestors, call.Args[0])
		if err != nil {
			failure = err
			return true
		}
		for path := range rows {
			if paths[path] {
				failure = goldenSourceError(positions, call, "duplicate consumer "+path)
				break
			}
			paths[path] = true
		}
		return true
	})
	return paths, failure
}

func unsupportedGoldenCall(call *ast.CallExpr) bool {
	if goldenCall(call, "t", "Errorf") || goldenCall(call, "t", "Fatalf") || goldenCall(call, "t", "Logf") {
		return false
	}
	for _, argument := range call.Args {
		if selector, ok := argument.(*ast.SelectorExpr); ok && selector.Sel.Name == "golden" {
			return true
		}
		literal, ok := argument.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			continue
		}
		path, err := strconv.Unquote(literal.Value)
		if err == nil && strings.HasPrefix(path, "testdata/golden/") && strings.HasSuffix(path, ".jsonl") {
			return true
		}
	}
	return false
}

func rangedGoldenConsumers(positions *token.FileSet, ancestors []ast.Node, argument ast.Expr) (map[string]bool, error) {
	for i, ancestor := range slices.Backward(ancestors) {
		loop, ok := ancestor.(*ast.RangeStmt)
		if !ok {
			continue
		}
		row, ok := loop.Value.(*ast.Ident)
		if !ok || loop.Tok != token.DEFINE || !goldenField(argument, row.Name, "golden") {
			return nil, goldenSourceError(positions, argument, "unresolved ranged golden reader")
		}
		if err := checkGoldenCallback(positions, loop, row.Name); err != nil {
			return nil, err
		}
		return goldenRows(positions, ancestors[:i], loop)
	}
	return nil, goldenSourceError(positions, argument, "unresolved golden reader")
}

func checkGoldenCallback(positions *token.FileSet, loop *ast.RangeStmt, row string) error {
	if len(loop.Body.List) != 1 {
		return goldenSourceError(positions, loop, "unsupported consumer loop")
	}
	statement, ok := loop.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return goldenSourceError(positions, loop, "missing consumer callback")
	}
	call, ok := statement.X.(*ast.CallExpr)
	if !ok || !goldenCall(call, "t", "Run") || len(call.Args) != 2 {
		return goldenSourceError(positions, loop, "missing consumer callback")
	}
	if !goldenField(call.Args[0], row, "name") && !goldenString(call.Args[0]) {
		return goldenSourceError(positions, call, "unsupported consumer callback name")
	}
	callback, ok := call.Args[1].(*ast.FuncLit)
	if !ok {
		return goldenSourceError(positions, call, "unsupported consumer callback")
	}
	for _, parameter := range callback.Type.Params.List {
		for _, name := range parameter.Names {
			if name.Name == row {
				return goldenSourceError(positions, name, "shadowed callback row")
			}
		}
	}
	if goldenDeclares(callback.Body, row) != 0 {
		return goldenSourceError(positions, callback, "shadowed consumer row")
	}
	if !goldenConsumerBody(callback.Body, row) {
		return goldenSourceError(positions, callback, "unsupported consumer statement sequence")
	}
	reader, _, err := goldenPipelineCall(positions, callback.Body, "os", "ReadFile")
	if err != nil {
		return err
	}
	if len(reader.Args) != 1 || !goldenField(reader.Args[0], row, "golden") {
		return goldenSourceError(positions, reader, "unbound consumer golden read")
	}
	check, _, err := goldenPipelineCall(positions, callback.Body, "", "runCheck")
	if err != nil {
		return err
	}
	if len(check.Args) != 2 || !goldenIdent(check.Args[0], "t") || !goldenField(check.Args[1], row, "fixture") {
		return goldenSourceError(positions, check, "unbound consumer fixture")
	}
	return nil
}

func goldenConsumerBody(body *ast.BlockStmt, row string) bool {
	if len(body.List) != 5 || !goldenStatementCall(body.List[0], "t", "Parallel") {
		return false
	}
	reader, ok := body.List[1].(*ast.AssignStmt)
	if !ok || reader.Tok != token.DEFINE || len(reader.Lhs) != 2 || !goldenIdent(reader.Lhs[0], "want") || !goldenIdent(reader.Lhs[1], "err") {
		return false
	}
	guard, ok := body.List[2].(*ast.IfStmt)
	if !ok || guard.Init != nil || guard.Else != nil || len(guard.Body.List) != 1 || !goldenErrCondition(guard.Cond) {
		return false
	}
	fatal, ok := goldenStatementExpression(guard.Body.List[0]).(*ast.CallExpr)
	if !ok || !goldenCall(fatal, "t", "Fatalf") || len(fatal.Args) != 2 || !goldenString(fatal.Args[0]) || !goldenIdent(fatal.Args[1], "err") {
		return false
	}
	check, ok := body.List[3].(*ast.AssignStmt)
	if !ok || check.Tok != token.DEFINE || len(check.Lhs) != 1 || !goldenIdent(check.Lhs[0], "got") {
		return false
	}
	comparison, ok := body.List[4].(*ast.IfStmt)
	if !ok || comparison.Init != nil || comparison.Else != nil || len(comparison.Body.List) != 1 {
		return false
	}
	negation, ok := comparison.Cond.(*ast.UnaryExpr)
	if !ok || negation.Op != token.NOT {
		return false
	}
	equal, ok := negation.X.(*ast.CallExpr)
	if !ok || !goldenCall(equal, "bytes", "Equal") || len(equal.Args) != 2 || !goldenIdent(equal.Args[0], "got") || !goldenIdent(equal.Args[1], "want") {
		return false
	}
	diagnostic, ok := goldenStatementExpression(comparison.Body.List[0]).(*ast.CallExpr)
	if !ok || !goldenCall(diagnostic, "t", "Errorf") || len(diagnostic.Args) != 6 {
		return false
	}
	return goldenString(diagnostic.Args[0]) && goldenField(diagnostic.Args[1], row, "golden") && goldenIdent(diagnostic.Args[2], "got") && goldenIdent(diagnostic.Args[3], "want") && goldenDump(diagnostic.Args[4], "got") && goldenDump(diagnostic.Args[5], "want")
}

func goldenStatementExpression(statement ast.Stmt) ast.Expr {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	return expression.X
}

func goldenStatementCall(statement ast.Stmt, receiver, name string) bool {
	return goldenZeroArgCall(goldenStatementExpression(statement), receiver, name)
}

func goldenDump(expression ast.Expr, value string) bool {
	call, ok := expression.(*ast.CallExpr)
	return ok && goldenCall(call, "hex", "Dump") && len(call.Args) == 1 && goldenIdent(call.Args[0], value)
}

func goldenErrCondition(expression ast.Expr) bool {
	condition, ok := expression.(*ast.BinaryExpr)
	return ok && condition.Op == token.NEQ && goldenIdent(condition.X, "err") && goldenIdent(condition.Y, "nil")
}

func engineGoldenWriters(positions *token.FileSet, file *ast.File) (map[string]bool, error) {
	var function *ast.FuncDecl
	for _, declaration := range file.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if !ok || candidate.Name.Name != "TestRegenerateGoldens" {
			continue
		}
		if function != nil {
			return nil, goldenSourceError(positions, candidate, "duplicate regeneration function")
		}
		function = candidate
	}
	if function == nil || function.Body == nil {
		return nil, goldenSourceError(positions, file, "missing regeneration function")
	}
	if !goldenEnginePrefix(function.Body) {
		return nil, goldenSourceError(positions, function, "unsupported engine declaration-to-loop prefix")
	}
	var engine *ast.RangeStmt
	for _, statement := range function.Body.List {
		loop, ok := statement.(*ast.RangeStmt)
		if !ok || !goldenIdent(loop.X, "engine") {
			continue
		}
		if engine != nil {
			return nil, goldenSourceError(positions, loop, "duplicate engine loop")
		}
		engine = loop
	}
	if engine == nil {
		return nil, goldenSourceError(positions, function, "missing active engine loop")
	}
	if err := checkEnginePipeline(positions, engine); err != nil {
		return nil, err
	}
	return goldenRows(positions, []ast.Node{function.Body}, engine)
}

func goldenEnginePrefix(body *ast.BlockStmt) bool {
	if len(body.List) < 3 {
		return false
	}
	guard, ok := body.List[0].(*ast.IfStmt)
	if !ok || guard.Init != nil || guard.Else != nil || len(guard.Body.List) != 1 {
		return false
	}
	condition, ok := guard.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.EQL {
		return false
	}
	getenv, ok := condition.X.(*ast.CallExpr)
	if !ok || !goldenCall(getenv, "os", "Getenv") || len(getenv.Args) != 1 || !goldenLiteral(getenv.Args[0], "YOMIHON_REGEN_GOLDENS") || !goldenLiteral(condition.Y, "") {
		return false
	}
	skip, ok := goldenStatementExpression(guard.Body.List[0]).(*ast.CallExpr)
	if !ok || !goldenCall(skip, "t", "Skip") || len(skip.Args) != 1 || !goldenString(skip.Args[0]) {
		return false
	}
	declaration, ok := body.List[1].(*ast.AssignStmt)
	if !ok || declaration.Tok != token.DEFINE || len(declaration.Lhs) != 1 || !goldenIdent(declaration.Lhs[0], "engine") || len(declaration.Rhs) != 1 {
		return false
	}
	_, literal := declaration.Rhs[0].(*ast.CompositeLit)
	loop, ok := body.List[2].(*ast.RangeStmt)
	return literal && ok && goldenIdent(loop.X, "engine")
}

func goldenLiteral(expression ast.Expr, want string) bool {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	return err == nil && value == want
}

func checkEnginePipeline(positions *token.FileSet, loop *ast.RangeStmt) error {
	row, ok := loop.Value.(*ast.Ident)
	if !ok || loop.Tok != token.DEFINE {
		return goldenSourceError(positions, loop, "unsupported engine row binding")
	}
	if err := checkEngineBody(positions, loop.Body, row.Name); err != nil {
		return err
	}
	if goldenDeclares(loop.Body, row.Name) != 0 {
		return goldenSourceError(positions, loop, "shadowed engine row")
	}
	setup, root, err := goldenPipelineCall(positions, loop.Body, "", "judgeFixtureRootWithPrivacy")
	if err != nil {
		return err
	}
	if root != "root" || len(setup.Args) != 3 || !goldenIdent(setup.Args[0], "t") || !goldenField(setup.Args[1], row.Name, "fixture") || !goldenField(setup.Args[2], row.Name, "private") || !setup.Ellipsis.IsValid() {
		return goldenSourceError(positions, setup, "unbound engine fixture preparation")
	}
	check, findings, err := goldenPipelineCall(positions, loop.Body, "", "Check")
	if err != nil {
		return err
	}
	if findings != "findings" || len(check.Args) != 2 || !goldenZeroArgCall(check.Args[0], "t", "Context") || !goldenIdent(check.Args[1], root) || check.Pos() <= setup.Pos() {
		return goldenSourceError(positions, check, "unbound engine findings")
	}
	serializer, _, err := goldenPipelineCall(positions, loop.Body, "", "WriteJSONL")
	if err != nil {
		return err
	}
	if len(serializer.Args) != 2 || !goldenIdent(serializer.Args[1], findings) || serializer.Pos() <= check.Pos() {
		return goldenSourceError(positions, serializer, "unbound engine serialization")
	}
	address, ok := serializer.Args[0].(*ast.UnaryExpr)
	if !ok || address.Op != token.AND {
		return goldenSourceError(positions, serializer, "missing engine buffer address")
	}
	buffer, ok := address.X.(*ast.Ident)
	if !ok || buffer.Name != "buf" || goldenDeclares(loop.Body, buffer.Name) != 1 {
		return goldenSourceError(positions, serializer, "ambiguous engine buffer")
	}
	writer, _, err := goldenPipelineCall(positions, loop.Body, "os", "WriteFile")
	if err != nil {
		return err
	}
	if len(writer.Args) != 3 || !goldenField(writer.Args[0], row.Name, "golden") || !goldenBufferBytes(writer.Args[1], buffer.Name) || !goldenWriteMode(writer.Args[2]) || writer.Pos() <= serializer.Pos() {
		return goldenSourceError(positions, writer, "unbound engine golden write")
	}
	return nil
}

// The supported body has no branch that can skip a row or replace its inputs
// between preparation and writing. New control flow needs an explicit review.
func checkEngineBody(positions *token.FileSet, body *ast.BlockStmt, row string) error {
	if len(body.List) != 7 {
		return goldenSourceError(positions, body, "unsupported engine statement sequence")
	}
	setup, ok := body.List[0].(*ast.AssignStmt)
	if !ok || len(setup.Lhs) != 1 || !goldenIdent(setup.Lhs[0], "root") {
		return goldenSourceError(positions, body.List[0], "unsupported engine root binding")
	}
	check, ok := body.List[1].(*ast.AssignStmt)
	if !ok || len(check.Lhs) != 2 || !goldenIdent(check.Lhs[0], "findings") || !goldenIdent(check.Lhs[1], "err") {
		return goldenSourceError(positions, body.List[1], "unsupported engine findings binding")
	}
	if !goldenErrorGuard(body.List[2], row, "fixture", false) {
		return goldenSourceError(positions, body.List[2], "unsupported Check error guard")
	}
	declaration, ok := body.List[3].(*ast.DeclStmt)
	if !ok || !goldenBufferDeclaration(declaration) {
		return goldenSourceError(positions, body.List[3], "unsupported engine buffer declaration")
	}
	if !goldenErrorGuard(body.List[4], row, "fixture", true) || !goldenErrorGuard(body.List[5], row, "golden", true) {
		return goldenSourceError(positions, body, "unsupported serialization or write error guard")
	}
	log, ok := body.List[6].(*ast.ExprStmt)
	if !ok {
		return goldenSourceError(positions, body.List[6], "unsupported engine completion statement")
	}
	call, ok := log.X.(*ast.CallExpr)
	if !ok || !goldenCall(call, "t", "Logf") || len(call.Args) != 3 || !goldenString(call.Args[0]) || !goldenField(call.Args[1], row, "golden") || !goldenZeroArgCall(call.Args[2], "buf", "Len") {
		return goldenSourceError(positions, log, "unsupported engine completion log")
	}
	return nil
}

func goldenErrorGuard(statement ast.Stmt, row, field string, initialized bool) bool {
	guard, ok := statement.(*ast.IfStmt)
	if !ok || guard.Else != nil || len(guard.Body.List) != 1 || (guard.Init != nil) != initialized {
		return false
	}
	if !goldenErrCondition(guard.Cond) {
		return false
	}
	if initialized {
		assignment, assignmentOK := guard.Init.(*ast.AssignStmt)
		if !assignmentOK || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || !goldenIdent(assignment.Lhs[0], "err") {
			return false
		}
	}
	statementCall, ok := guard.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := statementCall.X.(*ast.CallExpr)
	return ok && goldenCall(call, "t", "Fatalf") && len(call.Args) == 3 && goldenString(call.Args[0]) && goldenField(call.Args[1], row, field) && goldenIdent(call.Args[2], "err")
}

func goldenBufferDeclaration(statement *ast.DeclStmt) bool {
	declaration, ok := statement.Decl.(*ast.GenDecl)
	if !ok || declaration.Tok != token.VAR || len(declaration.Specs) != 1 {
		return false
	}
	buffer, ok := declaration.Specs[0].(*ast.ValueSpec)
	return ok && len(buffer.Names) == 1 && buffer.Names[0].Name == "buf" && len(buffer.Values) == 0 && goldenField(buffer.Type, "bytes", "Buffer")
}

func goldenString(expression ast.Expr) bool {
	literal, ok := expression.(*ast.BasicLit)
	return ok && literal.Kind == token.STRING
}

func goldenWriteMode(expression ast.Expr) bool {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return false
	}
	mode, err := strconv.ParseUint(literal.Value, 0, 32)
	return err == nil && mode == 0o600
}

func goldenZeroArgCall(expression ast.Expr, receiver, name string) bool {
	call, ok := expression.(*ast.CallExpr)
	return ok && goldenCall(call, receiver, name) && len(call.Args) == 0
}

// Only calls made directly by the loop count; a nested dead branch or an
// uninvoked closure cannot supply the regeneration pipeline.
func goldenPipelineCall(positions *token.FileSet, body *ast.BlockStmt, receiver, name string) (*ast.CallExpr, string, error) {
	var found *ast.CallExpr
	var output string
	for _, statement := range body.List {
		if conditional, ok := statement.(*ast.IfStmt); ok {
			statement = conditional.Init
		}
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 {
			continue
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok || !goldenCall(call, receiver, name) {
			continue
		}
		if found != nil || assignment.Tok != token.DEFINE || len(assignment.Lhs) == 0 {
			return nil, "", goldenSourceError(positions, call, "ambiguous pipeline call "+name)
		}
		found = call
		if id, ok := assignment.Lhs[0].(*ast.Ident); ok {
			output = id.Name
		}
	}
	if found == nil {
		return nil, "", goldenSourceError(positions, body, "missing direct pipeline call "+name)
	}
	if output != "err" && goldenDeclares(body, output) != 1 {
		return nil, "", goldenSourceError(positions, found, "ambiguous pipeline output "+name)
	}
	return found, output, nil
}

func goldenBufferBytes(expression ast.Expr, buffer string) bool {
	call, ok := expression.(*ast.CallExpr)
	return ok && buffer != "" && goldenCall(call, buffer, "Bytes") && len(call.Args) == 0
}

func goldenRows(positions *token.FileSet, ancestors []ast.Node, loop *ast.RangeStmt) (map[string]bool, error) {
	name, ok := loop.X.(*ast.Ident)
	if !ok {
		return nil, goldenSourceError(positions, loop, "unsupported table expression")
	}
	var table *ast.CompositeLit
	for _, ancestor := range slices.Backward(ancestors) {
		block, ok := ancestor.(*ast.BlockStmt)
		if !ok {
			continue
		}
		for _, statement := range block.List {
			assignment, ok := statement.(*ast.AssignStmt)
			if !ok || len(assignment.Lhs) != 1 || !goldenIdent(assignment.Lhs[0], name.Name) {
				continue
			}
			if table != nil || assignment.Tok != token.DEFINE || len(assignment.Rhs) != 1 || statement.Pos() >= loop.Pos() {
				return nil, goldenSourceError(positions, statement, "ambiguous table binding")
			}
			table, ok = assignment.Rhs[0].(*ast.CompositeLit)
			if !ok {
				return nil, goldenSourceError(positions, assignment, "nonliteral golden table")
			}
		}
		if table != nil {
			if goldenDeclares(block, name.Name) != 1 {
				return nil, goldenSourceError(positions, table, "ambiguous table binding")
			}
			if !goldenTableReachesLoop(block, table, loop) {
				return nil, goldenSourceError(positions, table, "unsupported table-to-loop statements")
			}
			return literalGoldenRows(positions, table)
		}
	}
	return nil, goldenSourceError(positions, loop, "unresolved golden table")
}

func goldenTableReachesLoop(block *ast.BlockStmt, table *ast.CompositeLit, loop *ast.RangeStmt) bool {
	for i, statement := range block.List {
		if statement == loop {
			return i > 0 && block.List[i-1].Pos() < table.Pos() && block.List[i-1].End() == table.End()
		}
	}
	return false
}

func literalGoldenRows(positions *token.FileSet, table *ast.CompositeLit) (map[string]bool, error) {
	paths := make(map[string]bool)
	for _, element := range table.Elts {
		row, ok := element.(*ast.CompositeLit)
		if !ok {
			return nil, goldenSourceError(positions, element, "nonliteral golden row")
		}
		fields := make(map[string]ast.Expr)
		for _, element := range row.Elts {
			field, fieldOK := element.(*ast.KeyValueExpr)
			if !fieldOK {
				return nil, goldenSourceError(positions, element, "unkeyed golden row")
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok || fields[key.Name] != nil {
				return nil, goldenSourceError(positions, element, "duplicate or unsupported row field")
			}
			fields[key.Name] = field.Value
		}
		fixture, ok := fields["fixture"].(*ast.BasicLit)
		if !ok || fixture.Kind != token.STRING {
			return nil, goldenSourceError(positions, row, "nonliteral fixture")
		}
		golden, ok := fields["golden"].(*ast.BasicLit)
		if !ok {
			return nil, goldenSourceError(positions, row, "nonliteral golden")
		}
		if err := addGoldenPath(positions, paths, golden); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func addGoldenPath(positions *token.FileSet, paths map[string]bool, literal *ast.BasicLit) error {
	path, err := strconv.Unquote(literal.Value)
	if err != nil || literal.Kind != token.STRING || !strings.HasPrefix(path, "testdata/golden/") || !strings.HasSuffix(path, ".jsonl") {
		return goldenSourceError(positions, literal, "unsupported golden path")
	}
	if paths[path] {
		return goldenSourceError(positions, literal, "duplicate golden "+path)
	}
	paths[path] = true
	return nil
}

func goldenDeclares(body *ast.BlockStmt, name string) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.AssignStmt:
			for _, expression := range declaration.Lhs {
				if goldenIdent(expression, name) {
					count++
				}
			}
		case *ast.ValueSpec:
			for _, identifier := range declaration.Names {
				if identifier.Name == name {
					count++
				}
			}
		}
		return true
	})
	return count
}

func goldenIdent(expression ast.Expr, name string) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == name
}

func goldenField(expression ast.Expr, receiver, field string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == field && goldenIdent(selector.X, receiver)
}

func goldenCall(call *ast.CallExpr, receiver, name string) bool {
	if receiver == "" {
		return goldenIdent(call.Fun, name)
	}
	return goldenField(call.Fun, receiver, name)
}

func goldenSourceError(positions *token.FileSet, node ast.Node, message string) error {
	return fmt.Errorf("%s: %s", positions.Position(node.Pos()), message)
}

func TestGoldenInventoryReadsConsumedRows(t *testing.T) {
	t.Parallel()
	const consumer = `package judge
func TestCheckGolden(t *testing.T) {
 tests := []struct { fixture, golden string }{
  {fixture: "testdata/vault", golden: "testdata/golden/check.jsonl"},
 }
 for _, tt := range tests {
  t.Run("fixture", func(t *testing.T) {
   t.Parallel()
   want, err := os.ReadFile(tt.golden)
   if err != nil { t.Fatalf("read golden: %v", err) }
   got := runCheck(t, tt.fixture)
   if !bytes.Equal(got, want) { t.Errorf("check findings differ from golden %s", tt.golden, got, want, hex.Dump(got), hex.Dump(want)) }
  })
 }
 os.ReadFile("testdata/golden/direct.jsonl")
}`
	const writer = `package judge
func TestRegenerateGoldens(t *testing.T) {
 if os.Getenv("YOMIHON_REGEN_GOLDENS") == "" { t.Skip("set YOMIHON_REGEN_GOLDENS=1 to rewrite the JSONL goldens") }
 engine := []struct { fixture, golden string; private []string }{
  {fixture: "testdata/vault", golden: "testdata/golden/check.jsonl"},
 }
 for _, tt := range engine {
  root := judgeFixtureRootWithPrivacy(t, tt.fixture, tt.private...)
  findings, err := Check(t.Context(), root)
  if err != nil { t.Fatalf("Check(%q): %v", tt.fixture, err) }
  var buf bytes.Buffer
  if err := WriteJSONL(&buf, findings); err != nil { t.Fatalf("WriteJSONL(%q): %v", tt.fixture, err) }
  if err := os.WriteFile(tt.golden, buf.Bytes(), 0o600); err != nil { t.Fatalf("write %s: %v", tt.golden, err) }
  t.Logf("rewrote %s (%d bytes)", tt.golden, buf.Len())
 }
}`
	consumerPaths := map[string]bool{"testdata/golden/check.jsonl": true, "testdata/golden/direct.jsonl": true}
	writerPaths := map[string]bool{"testdata/golden/check.jsonl": true}
	tests := []struct {
		name        string
		source      string
		needle      string
		replacement string
		inventory   func(*token.FileSet, *ast.File) (map[string]bool, error)
		want        map[string]bool
		invalid     bool
	}{
		{name: "direct and callback readers", source: consumer, inventory: checkGoldenConsumers, want: consumerPaths},
		{name: "active engine writer", source: writer, inventory: engineGoldenWriters, want: writerPaths},
		{name: "unused literal and table", source: consumer, needle: "tests :=", replacement: `unused := "testdata/golden/unused.jsonl"
 dead := []struct { fixture, golden string }{{fixture: "dead", golden: "testdata/golden/dead.jsonl"}}
 tests :=`, inventory: checkGoldenConsumers, want: consumerPaths},
		{name: "dynamic golden", source: consumer, needle: `golden: "testdata/golden/check.jsonl"`, replacement: `golden: chooseGolden()`, inventory: checkGoldenConsumers, invalid: true},
		{name: "duplicate consumer", source: consumer, needle: `os.ReadFile("testdata/golden/direct.jsonl")`, replacement: `os.ReadFile("testdata/golden/check.jsonl")`, inventory: checkGoldenConsumers, invalid: true},
		{name: "unresolved reader", source: consumer, needle: "os.ReadFile(tt.golden)", replacement: "os.ReadFile(resolve(tt.golden))", inventory: checkGoldenConsumers, invalid: true},
		{name: "unknown reader helper", source: consumer, needle: "os.ReadFile(tt.golden)", replacement: "readGolden(tt.golden)", inventory: checkGoldenConsumers, invalid: true},
		{name: "duplicate table binding", source: consumer, needle: "for _, tt :=", replacement: "tests = replacement\n for _, tt :=", inventory: checkGoldenConsumers, invalid: true},
		{name: "duplicate engine row", source: writer, needle: `{fixture: "testdata/vault", golden: "testdata/golden/check.jsonl"},`, replacement: `{fixture: "testdata/vault", golden: "testdata/golden/check.jsonl"},
 {fixture: "testdata/other", golden: "testdata/golden/check.jsonl"},`, inventory: engineGoldenWriters, invalid: true},
		{name: "unused engine table", source: writer, needle: "range engine", replacement: "range other", inventory: engineGoldenWriters, invalid: true},
		{name: "unbound writer buffer", source: writer, needle: "buf.Bytes()", replacement: "other.Bytes()", inventory: engineGoldenWriters, invalid: true},
		{name: "dead serialization", source: writer, needle: `if err := WriteJSONL(&buf, findings); err != nil { t.Fatalf("WriteJSONL(%q): %v", tt.fixture, err) }`, replacement: "if false { WriteJSONL(&buf, findings) }", inventory: engineGoldenWriters, invalid: true},
		{name: "skipped engine row", source: writer, needle: "root :=", replacement: `if tt.golden == "testdata/golden/check.jsonl" { continue }
  root :=`, inventory: engineGoldenWriters, invalid: true},
		{name: "changed golden destination", source: writer, needle: "if err := os.WriteFile", replacement: `tt.golden = "testdata/golden/other.jsonl"
  if err := os.WriteFile`, inventory: engineGoldenWriters, invalid: true},
		{name: "changed fixture", source: writer, needle: "root :=", replacement: `tt.fixture = "testdata/other"
  root :=`, inventory: engineGoldenWriters, invalid: true},
		{name: "changed root", source: writer, needle: "findings, err :=", replacement: `root = "testdata/other"
  findings, err :=`, inventory: engineGoldenWriters, invalid: true},
		{name: "changed findings", source: writer, needle: "if err := WriteJSONL", replacement: "findings = nil\n  if err := WriteJSONL", inventory: engineGoldenWriters, invalid: true},
		{name: "reset serialized buffer", source: writer, needle: "if err := os.WriteFile", replacement: "buf.Reset()\n  if err := os.WriteFile", inventory: engineGoldenWriters, invalid: true},
		{name: "changed table golden before loop", source: writer, needle: "for _, tt := range engine", replacement: `engine[0].golden = "testdata/golden/other.jsonl"
 for _, tt := range engine`, inventory: engineGoldenWriters, invalid: true},
		{name: "early return before engine loop", source: writer, needle: "for _, tt := range engine", replacement: "if len(engine) > 0 { return }\n for _, tt := range engine", inventory: engineGoldenWriters, invalid: true},
		{name: "changed callback golden", source: consumer, needle: "want, err := os.ReadFile", replacement: `tt.golden = "testdata/golden/schema.jsonl"
   want, err := os.ReadFile`, inventory: checkGoldenConsumers, invalid: true},
		{name: "changed consumer table golden", source: consumer, needle: "for _, tt := range tests", replacement: `tests[0].golden = "testdata/golden/schema.jsonl"
 for _, tt := range tests`, inventory: checkGoldenConsumers, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := tt.source
			if tt.needle != "" {
				if count := strings.Count(source, tt.needle); count != 1 {
					t.Fatalf("not-applied: %s needle matches %d sites, want 1", tt.name, count)
				}
				source = strings.Replace(source, tt.needle, tt.replacement, 1)
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, "control.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse control: %v", err)
			}
			got, err := tt.inventory(positions, file)
			if tt.invalid {
				if err == nil {
					t.Fatalf("caught: golden inventory falsely accepted %s", tt.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("inventory control: %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("inventory() = %v, want %v", got, tt.want)
			}
		})
	}
}
