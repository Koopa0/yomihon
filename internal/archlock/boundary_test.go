package archlock

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

// The boundary walk reads every shipped package of this module with its types,
// on each system the release is built for, and holds it to what a reader trusts
// yomihon for: it writes only in the two packages that may, reads one variable
// of its environment, opens no connection but its loopback listener, and hands
// an agent only what the privacy gate sealed. Every rule is judged on what a use
// resolves to, since a name says what a programmer called something and a type
// says what it is.
//
// The modules the program links are not read: go.mod and go.sum pin them and
// govulncheck checks them, and what they do inside is theirs. Reflection is not
// read either, so reflect and the template packages are forbidden imports, with
// C, unsafe and go:linkname.

const (
	// rawWriter writes the reader's own files, inside the configuration
	// directory, and may name anything that changes a file.
	rawWriter = module + "/internal/mark/confdir"
	// machineFileWriter writes this machine's file of marks in the vault.
	machineFileWriter = module + "/internal/mark/machinefile"
	// systemCalls holds the system calls syscall no longer adds, and its names
	// are judged the way syscall's are.
	systemCalls = "golang.org/x/sys"
	// mainFile is where the command reads its port, binds its listener and
	// hands each command its standard output.
	mainFile = "cmd/yomihon/main.go"
)

// writer is a package that may write, and the type each of its exported
// functions that writes is handed, so a write goes where that type was pointed.
type writer struct {
	dir string
	// may reports what the package may name beyond readOnly; nil is anything.
	may func(key string) bool
}

var writers = map[string]writer{
	rawWriter: {dir: "Dir"},
	// machinefile writes through the root it opens and the files that root
	// opens, and so names no write by a path. Which file it writes is held by
	// its own tests.
	machineFileWriter: {dir: "Folder", may: func(key string) bool {
		return key == "os.OpenRoot" || strings.HasPrefix(key, "os.Root.") || strings.HasPrefix(key, "os.File.") ||
			strings.HasPrefix(key, "os.O_") || strings.HasPrefix(key, systemCalls+"/windows.")
	}},
}

// reachesFilesystem reports whether a package can change a file or run a
// program that can, which is what holds its names to readOnly.
func reachesFilesystem(path string) bool {
	switch path {
	case "os", "os/exec", "io/ioutil", "syscall", "plugin":
		return true
	}
	return strings.HasPrefix(path, systemCalls+"/")
}

// readOnly are the names of those packages that code outside the writers may
// use. A name that is not here is refused, so one the standard library adds is
// refused the day it appears. OpenFile is absent however it is called: whether
// it writes depends on flags nothing here reads.
var readOnly = []string{
	"os.Args", "os.Stdin", "os.Stdout", "os.Stderr", "os.Interrupt", "os.Exit",
	// The handle types, so a value can be declared; each method is a name of its
	// own.
	"os.File", "os.Root",
	"os.Getenv", "os.Getpid", "os.Getwd", "os.UserHomeDir",
	"os.Stat", "os.ReadFile", "os.DirFS", "os.OpenRoot", "os.SameFile",
	"os.DevNull", "os.ModeCharDevice", "os.ModeSymlink",
	"os.File.Close", "os.File.Read", "os.File.Seek", "os.File.Stat",
	"os.Root.Close", "os.Root.FS", "os.Root.Lstat", "os.Root.Open", "os.Root.OpenRoot", "os.Root.Stat",
	"syscall.EPIPE", "syscall.ECONNRESET", "syscall.SIGTERM",
}

// writerQueries are the names rawWriter uses that change nothing, so an
// exported function of it that uses only these may take no Dir: whether a
// process is still there, and where the configuration directory is.
var writerQueries = []string{
	"os.FindProcess", "os.Process.Signal", "syscall.Signal", "syscall.EPERM",
	systemCalls + "/windows.OpenProcess", systemCalls + "/windows.PROCESS_QUERY_LIMITED_INFORMATION", systemCalls + "/windows.SYNCHRONIZE",
	systemCalls + "/windows.WaitForSingleObject", systemCalls + "/windows.WAIT_TIMEOUT", systemCalls + "/windows.CloseHandle",
	"os.UserConfigDir",
}

// writeFamilies are the method names a handle that writes, or ends a process,
// answers to. A call through an interface is judged by them, since what
// answers it is not known here.
var writeFamilies = []string{
	"Chmod", "Chown", "Chtimes", "Create", "Lchown", "Link", "Mkdir", "OpenFile",
	"Remove", "Rename", "Symlink", "Truncate", "WriteFile", "Kill", "Signal",
}

// alsoRefused write files of their own outside the packages above.
var alsoRefused = []string{
	"runtime/debug.WriteHeapDump",
	"runtime/coverage.WriteMetaDir", "runtime/coverage.WriteCountersDir",
	"mime/multipart.Reader.ReadForm",
	// A form that is not urlencoded is parsed as a multipart one, whose large
	// parts go to a temporary file. A handler reads r.PostForm after ParseForm.
	"net/http.Request.ParseMultipartForm", "net/http.Request.FormFile",
	"net/http.Request.FormValue", "net/http.Request.PostFormValue",
}

// forbiddenImports reach past the types this walk reads: cgo calls any C,
// unsafe breaks the types, and reflect and the templates call a method by a
// name that is a string.
var forbiddenImports = []string{"C", "unsafe", "reflect", "text/template", "html/template"}

// programStarters start or load another program, whose writes are no name in
// this source. A package of the standard library that imports one, however far
// down, is refused as well.
var programStarters = []string{"os/exec", "plugin"}

// placeRule refuses a name outside the files or directories (ending in "/") it
// is allowed in, or everywhere when allowedIn is empty.
type placeRule struct {
	match     func(key string) bool
	allowedIn []string
	why       string
}

// named matches the keys given; one ending in "*" is a prefix.
func named(keys ...string) func(string) bool {
	return func(key string) bool {
		return slices.ContainsFunc(keys, func(k string) bool {
			prefix, isPrefix := strings.CutSuffix(k, "*")
			return key == k || isPrefix && strings.HasPrefix(key, prefix)
		})
	}
}

var placeRules = []placeRule{
	{named("os.Getenv", "os.LookupEnv"), []string{mainFile}, "is read in main alone, for YOMIHON_PORT"},
	{named("os.UserConfigDir"), []string{"internal/mark/confdir/configdir.go"}, "says where the reader's files are kept, and is asked in one place"},
	{named("os.UserHomeDir"), []string{"internal/vault/tilde.go"}, "is read only to spell a path under it as ~"},
	{named("os.Environ", "os.ExpandEnv", "os.Expand", "os.UserCacheDir", "syscall.Getenv", "syscall.Environ"), nil,
		"reads the environment past the one variable main reads"},
	{named("net.Dial*", "net.Lookup*", "net.Resolver*", "net.DefaultResolver",
		"net/http.Client*", "net/http.DefaultClient", "net/http.DefaultTransport", "net/http.Transport*",
		"net/http.Get", "net/http.Head", "net/http.Post", "net/http.PostForm", "net/http.NewRequest*",
		"crypto/tls.Dial*", "crypto/tls.Client", "crypto/tls.Listen"), nil,
		"opens a connection, and yomihon makes no outbound call"},
	{func(key string) bool { return strings.HasPrefix(key, "net.Listen") && key != "net.Listener" }, []string{mainFile},
		"opens a listener, and the one listener is the loopback one main binds"},
	{named("os.Stdout", "os.NewFile"), []string{mainFile}, "is standard output, and only main hands it to the commands, as a writer"},
	{named(module+"/internal/schema.PrivacyPolicy.EgressAllowed", module+"/internal/schema.PrivacyPolicy.WithheldDirs"),
		[]string{"internal/egress/"}, "asks what is withheld, and only the gate decides that"},
	{named("go.yaml.in/yaml/v3.Unmarshal", "go.yaml.in/yaml/v3.NewDecoder"), []string{"internal/vault/", "internal/drill/slot.go"},
		"decodes YAML, and a note's frontmatter is decoded once, in internal/vault"},
	{named(rawWriter + ".At"), []string{"internal/mark/confdir/"},
		"makes a configuration directory at any path, for a test; shipped code takes confdir.Platform"},
	{named("net/http.NotFound"), nil, "writes the standard library's own sentence; take one from internal/wording"},
}

// inPlace reports whether a repository file is one of places.
func inPlace(file string, places []string) bool {
	return slices.ContainsFunc(places, func(place string) bool {
		return file == place || strings.HasSuffix(place, "/") && strings.HasPrefix(file, place)
	})
}

// constrainedCalls are refused unless called with the one argument that keeps
// them to what they are for. A use that is not such a call is refused, since
// nothing then says what it is called with.
var constrainedCalls = map[string]struct {
	argument int
	allowed  func(u *unit, arg ast.Expr) bool
	says     string
}{
	"os.Process.Signal": {0, isConstant(func(v constant.Value) bool { return v.Kind() == constant.Int && constant.Sign(v) == 0 }),
		"signal 0, which asks whether a process is there and sends nothing"},
	"os.Getenv":    {0, isConstant(isPort), `"YOMIHON_PORT"`},
	"os.LookupEnv": {0, isConstant(isPort), `"YOMIHON_PORT"`},
	module + "/internal/egress.Open": {2, resolvesTo(module + "/internal/judge.LinkTargets"),
		"judge.LinkTargets, the extractor the page and the check read links with"},
}

func isPort(v constant.Value) bool {
	return v.Kind() == constant.String && constant.StringVal(v) == "YOMIHON_PORT"
}

func isConstant(allowed func(constant.Value) bool) func(*unit, ast.Expr) bool {
	return func(u *unit, arg ast.Expr) bool {
		value := u.info.Types[arg].Value
		return value != nil && allowed(value)
	}
}

func resolvesTo(key string) func(*unit, ast.Expr) bool {
	return func(u *unit, arg ast.Expr) bool {
		var id *ast.Ident
		switch arg := ast.Unparen(arg).(type) {
		case *ast.Ident:
			id = arg
		case *ast.SelectorExpr:
			id = arg.Sel
		default:
			return false
		}
		sym, ok := resolve(u.info.Uses[id])
		return ok && sym.key == key
	}
}

// permittedCalls are the identifiers of calls to a constrainedCalls name that
// are made with the argument it allows.
func permittedCalls(u *unit) map[*ast.Ident]bool {
	permitted := map[*ast.Ident]bool{}
	for _, file := range u.files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			var name *ast.Ident
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fun.Sel
			case *ast.Ident:
				name = fun
			default:
				return true
			}
			sym, ok := resolve(u.info.Uses[name])
			rule, constrained := constrainedCalls[sym.key]
			if ok && constrained && len(call.Args) > rule.argument && rule.allowed(u, call.Args[rule.argument]) {
				permitted[name] = true
			}
			return true
		})
	}
	return permitted
}

// symbol is what a use resolves to: the package that declares it and the key
// the rules are written in, or, for a method of an interface, its name alone.
type symbol struct {
	path        string
	key         string
	viaIfaceFor string
}

// resolve names what an object is, or reports that it is none of the kinds
// this judges: a field, a local, a package name.
func resolve(obj types.Object) (symbol, bool) {
	if obj == nil || obj.Pkg() == nil {
		return symbol{}, false
	}
	path := obj.Pkg().Path()
	switch o := obj.(type) {
	case *types.Func:
		recv := o.Signature().Recv()
		if recv == nil {
			return symbol{path: path, key: path + "." + o.Name()}, true
		}
		t := types.Unalias(recv.Type())
		if pointer, isPointer := t.(*types.Pointer); isPointer {
			t = types.Unalias(pointer.Elem())
		}
		if types.IsInterface(t) {
			return symbol{path: path, viaIfaceFor: o.Name()}, true
		}
		if named, isNamed := t.(*types.Named); isNamed && named.Obj().Pkg() != nil {
			path = named.Obj().Pkg().Path()
			return symbol{path: path, key: path + "." + named.Obj().Name() + "." + o.Name()}, true
		}
	case *types.Var:
		if !o.IsField() && o.Parent() == o.Pkg().Scope() {
			return symbol{path: path, key: path + "." + o.Name()}, true
		}
	case *types.Const:
		return symbol{path: path, key: path + "." + o.Name()}, true
	case *types.TypeName:
		if o.Parent() == o.Pkg().Scope() {
			return symbol{path: path, key: path + "." + o.Name()}, true
		}
	}
	return symbol{}, false
}

// refusal is what a use of obj outside the writers is refused as, or that it
// is not.
func refusal(sym symbol, obj types.Object) (string, bool) {
	if sym.viaIfaceFor != "" {
		if hasPrefixIn(sym.viaIfaceFor, writeFamilies) {
			return "a call through an interface to " + sym.viaIfaceFor + ", which a handle that writes or ends a process answers", true
		}
		return "", false
	}
	if slices.Contains(alsoRefused, sym.key) {
		return sym.key + ", which makes files of its own", true
	}
	if !reachesFilesystem(sym.path) || slices.Contains(readOnly, sym.key) {
		return "", false
	}
	// An error value of os names a failure and holds no handle.
	if _, isVar := obj.(*types.Var); isVar && sym.path == "os" && strings.HasPrefix(obj.Name(), "Err") {
		return "", false
	}
	return sym.key + ", which is not one this guard knows to leave a file as it was", true
}

// shortKey is a key as a message says it, without this module's path.
func shortKey(key string) string { return strings.TrimPrefix(key, module+"/") }

func hasPrefixIn(name string, families []string) bool {
	return slices.ContainsFunc(families, func(family string) bool { return strings.HasPrefix(name, family) })
}

// unit is one package read with its types: the tree's, or a fixture's.
type unit struct {
	path  string
	fset  *token.FileSet
	files []*ast.File
	info  *types.Info
	pkg   *types.Package
	// root is the directory paths are reported relative to.
	root string
}

func (u *unit) rel(pos token.Pos) (path string, line int) {
	position := u.fset.Position(pos)
	name := position.Filename
	if u.root != "" {
		if rel, err := filepath.Rel(u.root, name); err == nil {
			name = rel
		}
	}
	return filepath.ToSlash(name), position.Line
}

func (u *unit) at(pos token.Pos, text string) site {
	path, line := u.rel(pos)
	return site{path: path, line: line, text: text}
}

// check is everything the walk says about one package.
func check(u *unit, starters map[string]string) []site {
	found := forbidden(u)
	found = append(found, startsAProgram(u, starters)...)
	if u.info == nil {
		return found
	}
	found = append(found, refusedUses(u)...)
	found = append(found, shape(u)...)
	found = append(found, payloadOnly(u)...)
	found = append(found, writtenSentences(u)...)
	slices.SortFunc(found, func(a, b site) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.line, b.line), cmp.Compare(a.text, b.text))
	})
	return found
}

// refusedUses reports every use in the unit of a name its package may not use
// there.
func refusedUses(u *unit) []site {
	w, isWriter := writers[u.path]
	permitted := permittedCalls(u)
	var found []site
	for id, obj := range u.info.Uses {
		sym, ok := resolve(obj)
		if !ok {
			continue
		}
		file, _ := u.rel(id.Pos())
		if i := slices.IndexFunc(placeRules, func(r placeRule) bool { return sym.key != "" && r.match(sym.key) }); i >= 0 && !inPlace(file, placeRules[i].allowedIn) {
			found = append(found, u.at(id.Pos(), shortKey(sym.key)+", which "+placeRules[i].why))
			continue
		}
		if rule, constrained := constrainedCalls[sym.key]; constrained && !permitted[id] {
			found = append(found, u.at(id.Pos(), shortKey(sym.key)+", which is called only with "+rule.says))
			continue
		}
		if isWriter && (w.may == nil || w.may(sym.key)) {
			continue
		}
		if what, refused := refusal(sym, obj); refused {
			found = append(found, u.at(id.Pos(), what))
		}
	}
	return found
}

// shape holds each exported function or method of a writer that changes a
// file, however many of its own helpers away, to being handed the writer's
// type, as a parameter or in a receiver that holds one: one that took a path
// could be pointed anywhere, a vault included.
func shape(u *unit) []site {
	w, isWriter := writers[u.path]
	if !isWriter {
		return nil
	}
	type function struct {
		decl   *ast.FuncDecl
		obj    *types.Func
		writes bool
		calls  []*types.Func
	}
	var functions []*function
	for _, file := range u.files {
		for _, decl := range file.Decls {
			if fn, isFunc := decl.(*ast.FuncDecl); isFunc && fn.Body != nil {
				if obj, isDef := u.info.Defs[fn.Name].(*types.Func); isDef {
					functions = append(functions, &function{decl: fn, obj: obj})
				}
			}
		}
	}
	for id, obj := range u.info.Uses {
		i := slices.IndexFunc(functions, func(fn *function) bool {
			return fn.decl.Body.Pos() <= id.Pos() && id.Pos() < fn.decl.Body.End()
		})
		if i < 0 {
			continue
		}
		if sym, ok := resolve(obj); ok && !slices.Contains(writerQueries, sym.key) {
			if _, refused := refusal(sym, obj); refused {
				functions[i].writes = true
			}
		}
		if callee, isFunc := obj.(*types.Func); isFunc && callee.Pkg() == u.pkg {
			functions[i].calls = append(functions[i].calls, callee.Origin())
		}
	}
	for changed := true; changed; {
		changed = false
		for _, fn := range functions {
			if !fn.writes && slices.ContainsFunc(fn.calls, func(callee *types.Func) bool {
				return slices.ContainsFunc(functions, func(other *function) bool { return other.obj == callee && other.writes })
			}) {
				fn.writes, changed = true, true
			}
		}
	}
	dir := u.pkg.Scope().Lookup(w.dir)
	var found []site
	for _, fn := range functions {
		if !fn.writes || !fn.obj.Exported() || handed(fn.obj.Signature(), dir) {
			continue
		}
		name := fn.obj.Name()
		if recv := fn.obj.Signature().Recv(); recv != nil {
			name = typeName(recv.Type()) + "." + name
		}
		found = append(found, u.at(fn.decl.Pos(), name+" changes a file, or a process, and takes no "+w.dir+", so a caller could point it anywhere"))
	}
	return found
}

// handed says whether a function's receiver or one of its parameters is dir or
// holds one.
func handed(sig *types.Signature, dir types.Object) bool {
	if dir == nil {
		return false
	}
	if recv := sig.Recv(); recv != nil && holds(recv.Type(), dir, map[types.Type]bool{}) {
		return true
	}
	for param := range sig.Params().Variables() {
		if holds(param.Type(), dir, map[types.Type]bool{}) {
			return true
		}
	}
	return false
}

// holds says whether a value of t is dir, or holds one in what it points to,
// its elements or its fields.
func holds(t types.Type, dir types.Object, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	switch typ := t.(type) {
	case *types.Named:
		return typ.Obj() == dir || holds(typ.Underlying(), dir, seen)
	case *types.Pointer:
		return holds(typ.Elem(), dir, seen)
	case *types.Slice:
		return holds(typ.Elem(), dir, seen)
	case *types.Array:
		return holds(typ.Elem(), dir, seen)
	case *types.Map:
		return holds(typ.Key(), dir, seen) || holds(typ.Elem(), dir, seen)
	case *types.Struct:
		for field := range typ.Fields() {
			if holds(field.Type(), dir, seen) {
				return true
			}
		}
	}
	return false
}

// typeName is the name of a receiver's type, for a message.
func typeName(t types.Type) string {
	t = types.Unalias(t)
	if pointer, isPointer := t.(*types.Pointer); isPointer {
		t = types.Unalias(pointer.Elem())
	}
	if named, isNamed := t.(*types.Named); isNamed {
		return named.Obj().Name()
	}
	return t.String()
}

// forbidden reports what reaches past the type checker: one of
// forbiddenImports, and a go:linkname, which names a symbol of another package
// without importing it.
func forbidden(u *unit) []site {
	var found []site
	for _, file := range u.files {
		for _, imp := range file.Imports {
			if path, err := strconv.Unquote(imp.Path.Value); err == nil && slices.Contains(forbiddenImports, path) {
				found = append(found, u.at(imp.Pos(), "imports "+path+", which reaches past what this guard can read"))
			}
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:linkname") {
					found = append(found, u.at(comment.Pos(), "a go:linkname, which names a symbol of another package without importing it"))
				}
			}
		}
	}
	return found
}

// startersReached says, for each package of the standard library in the
// graph, which of programStarters it reaches through its own imports, or ""
// for none. It is read from the graph that was loaded, so a package that starts
// importing os/exec is judged the day it does.
func startersReached(graph map[string]*packages.Package) map[string]string {
	reached := map[string]string{}
	var visit func(p *packages.Package) string
	visit = func(p *packages.Package) string {
		if starter, done := reached[p.PkgPath]; done {
			return starter
		}
		reached[p.PkgPath] = ""
		if slices.Contains(programStarters, p.PkgPath) {
			reached[p.PkgPath] = p.PkgPath
			return p.PkgPath
		}
		for _, path := range slices.Sorted(maps.Keys(p.Imports)) {
			if imported := p.Imports[path]; imported.Module == nil {
				if starter := visit(imported); starter != "" {
					reached[p.PkgPath] = starter
					return starter
				}
			}
		}
		return ""
	}
	for _, p := range graph {
		if p.Module == nil {
			visit(p)
		}
	}
	return reached
}

// startsAProgram reports each import of a standard-library package that runs
// another program for its caller, such as net/http/cgi or go/build.
func startsAProgram(u *unit, reached map[string]string) []site {
	var found []site
	for _, file := range u.files {
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if starter := reached[path]; err == nil && starter != "" && starter != path {
				found = append(found, u.at(imp.Pos(), "imports "+path+", which starts a program through "+starter+", and what that program changes is no name this guard can read"))
			}
		}
	}
	return found
}

// payloadWrite is how a sealed payload leaves for an agent.
const payloadWrite = module + "/internal/egress.Payload.WriteTo"

// payloadOnly holds a writer that an egress.Payload is written to, which is a
// command's standard output, to that and nothing else. A writer is a variable,
// known by what it is and not by what it is called, and one handed on to
// another variable, or to a parameter of another function of the package, is
// held there too: a command that printed text of its own to it would put out
// what no gate sealed.
func payloadOnly(u *unit) []site {
	local := func(obj types.Object) *types.Var {
		if v, isVar := obj.(*types.Var); isVar && !v.IsField() && v.Pkg() == u.pkg && v.Parent() != u.pkg.Scope() {
			return v
		}
		return nil
	}
	// A use of a variable is a payload written to it, the variable it is handed
	// on to, or anything else.
	type use struct {
		id      *ast.Ident
		written bool
		into    *types.Var
	}
	handed := map[*ast.Ident]use{}
	handOn := func(from ast.Expr, into *types.Var) {
		if id, isIdent := ast.Unparen(from).(*ast.Ident); isIdent && into != nil {
			handed[id] = use{id: id, into: into}
		}
	}
	for _, file := range u.files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i, lhs := range n.Lhs {
						if id, isIdent := lhs.(*ast.Ident); isIdent {
							handOn(n.Rhs[i], local(u.info.ObjectOf(id)))
						}
					}
				}
			case *ast.ValueSpec:
				if len(n.Names) == len(n.Values) {
					for i, name := range n.Names {
						handOn(n.Values[i], local(u.info.Defs[name]))
					}
				}
			case *ast.CallExpr:
				callee := typeutil.StaticCallee(u.info, n)
				if callee == nil {
					return true
				}
				sym, _ := resolve(callee)
				sig := callee.Origin().Signature()
				for i, arg := range n.Args {
					id, isIdent := ast.Unparen(arg).(*ast.Ident)
					switch {
					case !isIdent:
					case sym.key == payloadWrite:
						handed[id] = use{id: id, written: true}
					case callee.Pkg() == u.pkg && !n.Ellipsis.IsValid() && i < sig.Params().Len() && (!sig.Variadic() || i < sig.Params().Len()-1):
						handOn(id, sig.Params().At(i))
					}
				}
			}
			return true
		})
	}
	uses := map[*types.Var][]use{}
	for id, obj := range u.info.Uses {
		if v := local(obj); v != nil {
			one, isHanded := handed[id]
			if !isHanded {
				one = use{id: id}
			}
			uses[v] = append(uses[v], one)
		}
	}
	bound := map[*types.Var]bool{}
	held := func(one use) bool { return one.written || bound[one.into] }
	for changed := true; changed; {
		changed = false
		for v, all := range uses {
			if !bound[v] && slices.ContainsFunc(all, held) {
				bound[v], changed = true, true
			}
		}
	}
	var found []site
	for v, all := range uses {
		for _, one := range all {
			if bound[v] && !held(one) {
				found = append(found, u.at(one.id.Pos(), v.Name()+" is written an egress.Payload and is used here for something else, so what leaves is not only what the gate sealed"))
			}
		}
	}
	return found
}

// writtenSentences reports each http.Error handed a sentence written at the
// call, or held in a constant, rather than taken from internal/wording, which
// has both languages of it.
func writtenSentences(u *unit) []site {
	var found []site
	for _, file := range u.files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			if calleeKey(u.info, call) == "net/http.Error" && len(call.Args) > 1 && u.written(call.Args[1]) {
				found = append(found, u.at(call.Pos(), "net/http.Error, which is handed a sentence written in the source; take it from internal/wording"))
			}
			return true
		})
	}
	return found
}

// written says whether an expression is words written in the source: a
// constant string, one joined to something else, or a format string, which is
// the sentence however much is put into it.
func (u *unit) written(e ast.Expr) bool {
	if value := u.info.Types[e].Value; value != nil && value.Kind() == constant.String {
		return true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.BinaryExpr:
		return e.Op == token.ADD && (u.written(e.X) || u.written(e.Y))
	case *ast.CallExpr:
		return calleeKey(u.info, e) == "fmt.Sprintf" && len(e.Args) > 0 && u.written(e.Args[0])
	}
	return false
}

// calleeKey is the key of the function a call statically calls, or "".
func calleeKey(info *types.Info, call *ast.CallExpr) string {
	if callee := typeutil.StaticCallee(info, call); callee != nil {
		if sym, ok := resolve(callee); ok {
			return sym.key
		}
	}
	return ""
}

// world is the module read with its types for one system: its own packages
// from source, and what they import from export data.
type world struct {
	units []*unit
	// byPath is every package a fixture may import, the same packages the tree
	// was checked against.
	byPath   map[string]*types.Package
	starters map[string]string
}

// Import lets a fixture name a package the world holds.
func (w *world) Import(path string) (*types.Package, error) {
	if p, ok := w.byPath[path]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("the world holds no package %q; add it to fixtureImports", path)
}

// fixtureImports are the packages a fixture names that the tree does not
// import, loaded on top of it.
var fixtureImports = []string{"io/ioutil", "mime/multipart", "net/http/cgi", "os/exec", "plugin", "runtime/coverage", "runtime/debug"}

// loadWorld reads the module for one system, once.
var loadWorld = func() func(target) (*world, error) {
	loads := map[target]func() (*world, error){}
	for _, at := range releaseTargets {
		loads[at] = sync.OnceValues(func() (*world, error) { return readWorld(at) })
	}
	return func(at target) (*world, error) { return loads[at]() }
}()

// target is a system the release is built for.
type target struct{ goos, goarch string }

func (t target) String() string { return t.goos + "/" + t.goarch }

// releaseTargets are the systems the release is built for. No file of the
// module is chosen by architecture, so one each reads every file; the test
// says so when one is.
var releaseTargets = []target{{"darwin", "amd64"}, {"linux", "amd64"}, {"windows", "amd64"}}

// fixtureTarget is the system the fixtures are checked against.
var fixtureTarget = target{"linux", "amd64"}

func readWorld(at target) (*world, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	patterns := []string{"./..."}
	if at == fixtureTarget {
		patterns = append(patterns, fixtureImports...)
	}
	// What the release is built with, and not what the shell has set: GOFLAGS
	// could add files to the load and a workspace could swap a module.
	env := append(os.Environ(), "GOOS="+at.goos, "GOARCH="+at.goarch, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off", "GOENV=off")
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir: root,
		Env: env,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("read the module for %s: %w", at, err)
	}
	// The import graph alone, for what reaches a program starter.
	graph, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedModule, Dir: root, Env: env}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("read the import graph for %s: %w", at, err)
	}
	w := &world{byPath: map[string]*types.Package{}}
	var add func(p *types.Package)
	add = func(p *types.Package) {
		if _, seen := w.byPath[p.Path()]; !seen {
			w.byPath[p.Path()] = p
			for _, imported := range p.Imports() {
				add(imported)
			}
		}
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("read the module for %s: %s did not load: %w", at, p.PkgPath, p.Errors[0])
		}
		if p.Types != nil {
			add(p.Types)
		}
		if p.Module != nil && p.Module.Path == module && len(p.GoFiles) > 0 {
			w.units = append(w.units, &unit{path: p.PkgPath, fset: p.Fset, files: p.Syntax, info: p.TypesInfo, pkg: p.Types, root: root})
		}
	}
	nodes := map[string]*packages.Package{}
	packages.Visit(graph, nil, func(p *packages.Package) { nodes[p.PkgPath] = p })
	w.starters = startersReached(nodes)
	return w, nil
}

// TestShippedCodeKeepsToItsBoundaries reads every shipped package on each
// system the release is built for, and fails on each use that crosses one of
// the boundaries above.
func TestShippedCodeKeepsToItsBoundaries(t *testing.T) {
	t.Parallel()

	type walked struct {
		found []site
		read  []string
		err   error
	}
	results := make([]walked, len(releaseTargets))
	var reading sync.WaitGroup
	for i, at := range releaseTargets {
		reading.Go(func() {
			w, err := loadWorld(at)
			if err != nil {
				results[i].err = err
				return
			}
			for _, u := range w.units {
				results[i].found = append(results[i].found, check(u, w.starters)...)
				for _, file := range u.files {
					path, _ := u.rel(file.Pos())
					results[i].read = append(results[i].read, path)
				}
			}
		})
	}
	reading.Wait()

	on := map[site][]string{}
	var order []site
	read := map[string]bool{}
	for i, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		for _, one := range result.found {
			if _, seen := on[one]; !seen {
				order = append(order, one)
			}
			on[one] = append(on[one], releaseTargets[i].String())
		}
		for _, path := range result.read {
			read[path] = true
		}
	}
	for _, one := range order {
		if systems := on[one]; len(systems) < len(releaseTargets) {
			one.text += " (on " + strings.Join(systems, ", ") + " only)"
		}
		t.Errorf("%s:%d %s", one.path, one.line, one.text)
	}
	// A file no system builds is a file nothing here read.
	for _, path := range productionFiles(t, ".go") {
		if !read[path] {
			t.Errorf("%s is built on none of %v, so what it does is not checked", path, releaseTargets)
		}
	}
}

// TestNoWriteIsOnTheReadOnlyList holds readOnly to what it says: a name a write
// answers to is refused whatever package it is in.
func TestNoWriteIsOnTheReadOnlyList(t *testing.T) {
	t.Parallel()
	for _, key := range readOnly {
		if name := key[strings.LastIndexByte(key, '.')+1:]; hasPrefixIn(name, writeFamilies) || strings.HasPrefix(name, "Write") {
			t.Errorf("%s is on readOnly and its name is one a write answers to", key)
		}
	}
}
