package archlock

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// yomihon writes in two places, and each is held to what it may write there by
// the types of its code. One is outside the vault: the reader's own records, in
// files of yomihon's own under the platform's configuration directory, which
// internal/mark/confdir writes. The other is inside it, and is one file: this
// machine's file of marks in the vault's reader folder, which
// internal/mark/machinefile writes and nothing else may (writers_test.go). No
// route calls it yet, so today nothing writes into a vault. The sentences the
// privacy inventory and the threat model make to a stranger rest on that being
// true of the code rather than of anyone's memory of it, so the code is read
// with its types: a name says what a programmer called something, and a type
// says what it is.
//
// The rule has three parts, and none is a list of writes.
//
//   - What may be named. Outside the packages that write, code may name only
//     the symbols of the packages that reach the filesystem or run a program
//     (os, syscall, io/ioutil, os/exec, golang.org/x/sys, plugin) that readOnly
//     lists. Everything else they hold is refused by being absent, so a function
//     the standard library adds tomorrow is refused the day it appears rather
//     than the day somebody remembers a list. Nor may code import a package of
//     the standard library that starts a program for it, as net/http/cgi does.
//   - What it may be pointed at. Each package that writes works on a type only
//     it makes, and takes one for everything that changes a file, so a write goes
//     where that type was pointed: confdir's Dir, which only Home and VaultDir
//     make, and machinefile's Folder, which only Open makes. Home and VaultDir
//     make a Dir only inside a ConfigDir, which only Platform, from what the
//     platform names, and At, for a test, make. shape and dirIsOpaque hold each
//     package to that: a function or method that changes a file is handed one,
//     and nothing but its makers makes one, sets what one holds or hands one
//     out. judge refuses At in shipped code. machinefile is held further, to the
//     few names its permits list and to what each of their calls is made with.
//   - What is linked. The program is more than this module: the modules it
//     links are read by the same names, may be only those on reviewedModules,
//     may not be swapped by a replace, and may not hold a file in a language the
//     guard does not read. The packages of golang.org/x/sys are not read:
//     like syscall, they are the system calls, so what is judged is each use of
//     one of their names, by this module or another, and not what they do
//     inside. What a call may be made with is held for the few calls whose harm
//     is in an argument: a signal other than 0, and a network other than tcp.
//
// What this does not read: whether the directory the platform names and a
// folder being read are one inside the other, which is a question about the
// disk and not the code, and which confdir's ConfigDir.Apart answers as the
// command runs;
// whether a function of confdir that is handed a Dir writes inside it, which
// confdir's own name checks and tests answer; what a reviewed module does with
// a path it is handed; and a write through a descriptor that was open already,
// such as standard input on a terminal opened read-write.
// Reflection is not read either, so reflect and the template packages are
// forbidden imports, and so are C, unsafe and go:linkname.

// rawWriter is the package that may name anything that changes a file, inside
// the directory it is handed. Everything else in internal/mark goes through it
// or through machinefile, and this guard checks that they do: the survey that
// made this list found no write outside it.
const rawWriter = module + "/internal/mark/confdir"

// target is a platform the release is built for.
type target struct{ goos, goarch string }

func (t target) String() string { return t.goos + "/" + t.goarch }

// releaseTargets are the platforms .github/workflows/release.yml builds, and so
// the platforms whose files have to be read. A file chosen by a build constraint
// reaches the guard only on the platform that builds it.
var releaseTargets = []target{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"windows", "amd64"}}

// fixtureTarget is the platform the fixtures are type-checked against.
var fixtureTarget = target{"linux", "amd64"}

// reachesFilesystem reports whether a package is one that can change a file or
// run a program that can, which is the line between the packages whose symbols
// are held to readOnly and the ones that are not.
func reachesFilesystem(path string) bool {
	switch path {
	case "os", "os/exec", "io/ioutil", "syscall", "plugin":
		return true
	}
	return strings.HasPrefix(path, systemCalls+"/")
}

// systemCalls is the module that holds the system calls syscall no longer adds,
// such as LockFileEx. Its packages are read the way syscall is: every name of
// theirs is held to readOnly where it is used, and their own code, which is
// those calls and the tables they are made from, is not read for names, any
// more than the standard library's is.
const systemCalls = "golang.org/x/sys"

// readOnly is every symbol of those packages that code outside the writers may
// name, each with what it is for. A symbol that is not here is refused, so
// adding one is a decision somebody makes and a reviewer reads, and the thing it
// has to be is something that cannot change a file.
//
// Opening a file for reading is os.Root.Open and os.ReadFile, and OpenFile is
// absent however it is called: whether it writes depends on flags that are
// written at the call and read nowhere else, and a name that cannot be read
// from its arguments is one this list does not have to judge.
var readOnly = map[string]bool{}

func init() {
	for _, name := range []string{
		// The process's own streams, arguments and identity.
		"os.Args", "os.Stdin", "os.Stdout", "os.Stderr", "os.Interrupt", "os.Exit",
		// The types of a handle, named so a value can be declared and passed; what a
		// handle can do is judged by each method below, since a method of the type
		// is a name of its own.
		"os.File", "os.Root",
		"os.Getenv", "os.Getpid", "os.Getwd", "os.UserHomeDir",
		// Looking at the filesystem without changing it.
		"os.Stat", "os.ReadFile", "os.DirFS", "os.OpenRoot", "os.SameFile",
		"os.DevNull", "os.ModeCharDevice", "os.ModeSymlink",
		"os.File.Close", "os.File.Read", "os.File.Seek", "os.File.Stat",
		"os.Root.Close", "os.Root.FS", "os.Root.Lstat", "os.Root.Open", "os.Root.OpenRoot", "os.Root.Stat",
		// What a closed connection and a termination look like.
		"syscall.EPIPE", "syscall.ECONNRESET", "syscall.SIGTERM",
	} {
		readOnly[name] = true
	}
}

// processQueries are the names the writer asks with whether a process is still
// there: on Unix, finding it by its id and sending it signal 0, which delivers
// nothing; on Windows, opening it to ask about it, waiting on it for no time and
// closing the handle. None changes a file or ends a process, so shape lets an
// exported function of the writer that uses them take no Dir; that is
// ProcessAlive, which tells a record of a running yomihon that its process left
// behind from one whose process still runs. Outside the writer they are refused
// like any name not on readOnly, since nothing else asks.
var processQueries = []string{
	"os.FindProcess", "os.Process.Signal", "syscall.Signal", "syscall.EPERM",
	"golang.org/x/sys/windows.OpenProcess", "golang.org/x/sys/windows.PROCESS_QUERY_LIMITED_INFORMATION", "golang.org/x/sys/windows.SYNCHRONIZE",
	"golang.org/x/sys/windows.WaitForSingleObject", "golang.org/x/sys/windows.WAIT_TIMEOUT", "golang.org/x/sys/windows.CloseHandle",
}

// platformQueries are the names the writer asks the platform with where the
// configuration directory is: os.UserConfigDir, which reads the environment and
// changes nothing. shape lets an exported function of the writer that uses it
// take no Dir; that is Platform, which makes the ConfigDir every Dir is made in.
// Outside the writer it is refused like any name not on readOnly, so where the
// reader's files are kept is asked in one package, and the environment test in
// cmd/yomihon holds the read to one file of it.
var platformQueries = []string{"os.UserConfigDir"}

// writerQueries are the names the writer may use in an exported function that
// takes no Dir, since none of them changes a file or ends a process.
var writerQueries = slices.Concat(processQueries, platformQueries)

// writeFamilies are the prefixes of the names a handle that writes answers to.
// They are prefixes because the standard library grows (Mkdir became MkdirAll
// and MkdirTemp), and they judge a method reached through an interface, where
// the type that answers it cannot be read: a call to Remove on a value whose type
// is an interface may be a call to os.Root.Remove, and so is refused.
var writeFamilies = []string{
	"Chmod", "Chown", "Chtimes", "Create", "Lchown", "Link", "Mkdir", "OpenFile",
	"Remove", "Rename", "Symlink", "Truncate", "WriteFile",
}

// processFamilies are the names a handle that ends or signals a process answers
// to. They are refused through an interface for the same reason writeFamilies
// are: what answers the call is not known, and a process of the reader's is not
// something yomihon touches except to ask whether it is there.
var processFamilies = []string{"Kill", "Signal"}

// listenerFamilies are the names a listen config answers to. Through an
// interface the network it is called with is not judged, and a listener on a
// unix socket makes a file that stays after it is closed.
var listenerFamilies = []string{"Listen"}

// listens reports whether a function of net, or a method of its ListenConfig,
// opens a listener. They are matched by prefix, as the network test matches
// them, so a listener the standard library adds is refused the day it appears;
// only the constrainedCalls among them are allowed, and only on a tcp port.
func listens(key string) bool {
	name, isNet := strings.CutPrefix(key, "net.")
	if !isNet {
		return false
	}
	name = strings.TrimPrefix(name, "ListenConfig.")
	return strings.HasPrefix(name, "Listen") && !strings.Contains(name, ".")
}

// alsoRefused are the symbols outside those packages that write to disk
// anyway: they make files of their own, which the packages above would be
// refused for. None is used, and each is one more way a reader would not look.
var alsoRefused = []string{
	"runtime/debug.WriteHeapDump",
	"net/http.Request.ParseMultipartForm", "net/http.Request.FormFile",
	"mime/multipart.Reader.ReadForm",
	"runtime/coverage.WriteMetaDir", "runtime/coverage.WriteCountersDir",
	// A form that is not urlencoded is parsed as a multipart one, and a part
	// larger than the memory it is given is written to a temporary file. Each
	// handler reads r.PostForm after ParseForm, which never does.
	"net/http.Request.FormValue", "net/http.Request.PostFormValue",
}

// constrainedCalls are the calls that are refused unless made with the one
// argument that leaves them harmless: a network that is tcp, which is a port and
// not a file, and, among the processQueries only the writer may name, a signal of
// 0, which asks whether a process is there and sends nothing. A use that is not
// a call, or whose argument is not a constant of that kind, is refused, since
// nothing then says what it is called with.
var constrainedCalls = map[string]struct {
	argument int
	allowed  func(value constant.Value) bool
}{
	"os.Process.Signal":       {0, func(v constant.Value) bool { return v.Kind() == constant.Int && constant.Sign(v) == 0 }},
	"net.Listen":              {0, isTCP},
	"net.ListenConfig.Listen": {1, isTCP},
}

func isTCP(v constant.Value) bool {
	if v.Kind() != constant.String {
		return false
	}
	switch constant.StringVal(v) {
	case "tcp", "tcp4", "tcp6":
		return true
	}
	return false
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
			if !ok {
				return true
			}
			rule, constrained := constrainedCalls[sym.key]
			if !constrained || len(call.Args) <= rule.argument {
				return true
			}
			if value := u.info.Types[call.Args[rule.argument]].Value; value != nil && rule.allowed(value) {
				permitted[name] = true
			}
			return true
		})
	}
	return permitted
}

// forbiddenImports are the ways a program reaches the disk, or the memory the
// type checker reasons about, that no list of symbols can fence: cgo calls any
// C function, unsafe breaks the types this guard rests on, and reflect and the
// template packages call a method by a name that is a string, so that a Remove
// on a root nobody named is not a use of os.Root.Remove anywhere in the source.
// No shipped package imports one of them; one that needs to is a decision.
var forbiddenImports = []string{"C", "unsafe", "reflect", "text/template", "html/template"}

// programStarters are the packages that start or load another program, whose
// writes are no name in this module's source. Their own symbols are judged by
// name like those of os; a package of the standard library that imports one,
// however far down, is refused by startsAProgram, since what it runs on its
// caller's behalf is not a name its caller writes.
var programStarters = []string{"os/exec", "plugin"}

// startersAllowed are the packages of the standard library that reach a
// program starter and may be imported all the same, each with why. There are
// none: nothing yomihon ships starts a program.
var startersAllowed = map[string]string{}

// startersReached says, for every package of the standard library the world
// holds, which program starter it reaches through its own imports, or "" for
// none. It is read from the packages that were loaded, so a package the
// standard library adds, or one that starts importing os/exec, is judged the day
// it does.
func startersReached(byPath map[string]*packages.Package) map[string]string {
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
			if imported := p.Imports[path]; whose(imported) == standardLibrary {
				if starter := visit(imported); starter != "" {
					reached[p.PkgPath] = starter
					return starter
				}
			}
		}
		return ""
	}
	for _, p := range byPath {
		if whose(p) == standardLibrary {
			visit(p)
		}
	}
	return reached
}

// startsAProgram reports each import of a standard-library package that reaches
// a program starter, such as net/http/cgi, go/build or go/importer: each can run
// another program from a call that names nothing this guard refuses.
func startsAProgram(u *unit, reached map[string]string) []site {
	var found []site
	for _, file := range u.files {
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			starter := reached[path]
			if starter == "" || starter == path || startersAllowed[path] != "" {
				continue
			}
			name, line := u.rel(imp.Pos())
			found = append(found, site{path: name, line: line, text: "imports " + path + ", which starts a program through " + starter + ", and what that program changes is no name this guard can read"})
		}
	}
	return found
}

func hasPrefixIn(name string, families []string) bool {
	for _, family := range families {
		if strings.HasPrefix(name, family) {
			return true
		}
	}
	return false
}

// symbol is what a use of a name resolves to: the package that declares it, the
// key readOnly is written in, and whether it is a method of an interface, which
// is a call whose answer is not known here.
type symbol struct {
	path        string
	key         string
	viaIfaceFor string
}

// resolve names what an object is, or reports that it is none of the kinds this
// judges: a type, a field, a local, a package name.
func resolve(obj types.Object) (symbol, bool) {
	if obj.Pkg() == nil {
		return symbol{}, false
	}
	switch o := obj.(type) {
	case *types.Func:
		o = o.Origin()
		sig, ok := o.Type().(*types.Signature)
		if !ok || sig.Recv() == nil {
			return symbol{path: o.Pkg().Path(), key: o.Pkg().Path() + "." + o.Name()}, true
		}
		recv := types.Unalias(sig.Recv().Type())
		if pointer, isPointer := recv.(*types.Pointer); isPointer {
			recv = types.Unalias(pointer.Elem())
		}
		if types.IsInterface(recv) {
			return symbol{path: o.Pkg().Path(), viaIfaceFor: o.Name()}, true
		}
		if named, isNamed := recv.(*types.Named); isNamed && named.Obj().Pkg() != nil {
			path := named.Obj().Pkg().Path()
			return symbol{path: path, key: path + "." + named.Obj().Name() + "." + o.Name()}, true
		}
		return symbol{}, false
	case *types.Var:
		if o.IsField() || o.Parent() != o.Pkg().Scope() {
			return symbol{}, false
		}
		return symbol{path: o.Pkg().Path(), key: o.Pkg().Path() + "." + o.Name()}, true
	case *types.Const:
		return symbol{path: o.Pkg().Path(), key: o.Pkg().Path() + "." + o.Name()}, true
	case *types.TypeName:
		// A type of a package that reaches the filesystem is named only if it is
		// listed: a value of the type that writes can be made from its literal.
		if o.Parent() != o.Pkg().Scope() {
			return symbol{}, false
		}
		return symbol{path: o.Pkg().Path(), key: o.Pkg().Path() + "." + o.Name()}, true
	}
	return symbol{}, false
}

// refusal says what to call a use of obj that code outside rawWriter may not
// make, or that there is none.
func refusal(obj types.Object, dependency bool) (string, bool) {
	sym, ok := resolve(obj)
	if !ok {
		return "", false
	}
	if sym.viaIfaceFor != "" {
		// A package of another module calls the methods of its own types, whose
		// names are its own business; what it reaches the filesystem with is a
		// symbol of the packages above, which is judged.
		if !dependency && (hasPrefixIn(sym.viaIfaceFor, writeFamilies) || hasPrefixIn(sym.viaIfaceFor, processFamilies) || hasPrefixIn(sym.viaIfaceFor, listenerFamilies)) {
			return "a call through an interface to " + sym.viaIfaceFor + ", which a handle that writes, signals or listens answers", true
		}
		return "", false
	}
	if slices.Contains(alsoRefused, sym.key) {
		return sym.key + ", which makes files of its own", true
	}
	if _, isFunc := obj.(*types.Func); isFunc && listens(sym.key) {
		if _, constrained := constrainedCalls[sym.key]; !constrained {
			return sym.key + ", a listener other than the tcp one, which can make a socket file that stays after it is closed", true
		}
	}
	if !reachesFilesystem(sym.path) || readOnly[sym.key] || (dependency && dependencyReads[sym.key] != "") {
		return "", false
	}
	// An error value of os is a name for a failure and holds no handle.
	if v, isVar := obj.(*types.Var); isVar && sym.path == "os" && strings.HasPrefix(v.Name(), "Err") {
		return "", false
	}
	return sym.key + ", which is not one this guard knows to leave a file as it was", true
}

// forbidden reports what in a file reaches the disk past the type checker: an
// import of C or unsafe, and a //go:linkname, which names a symbol of another
// package without importing it and so without the use this guard reads.
func forbidden(u *unit) []site {
	var found []site
	for _, file := range u.files {
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, "\"`")
			if slices.Contains(forbiddenImports, path) {
				name, line := u.rel(imp.Pos())
				found = append(found, site{path: name, line: line, text: "imports " + path + ", which reaches past what this guard can read"})
			}
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:linkname") {
					name, line := u.rel(comment.Pos())
					found = append(found, site{path: name, line: line, text: "a go:linkname, which names a symbol of another package without importing it"})
				}
			}
		}
	}
	return found
}

// unit is one package, read with its types: the real ones, or a fixture's.
type unit struct {
	path  string
	fset  *token.FileSet
	files []*ast.File
	info  *types.Info
	pkg   *types.Package
	// root is the directory paths are reported relative to.
	root string
	// dependency is a package of another module that is linked into the program.
	dependency bool
}

func (u *unit) rel(pos token.Pos) (path string, line int) {
	position := u.fset.Position(pos)
	if u.dependency {
		// A file of another module is named by its package, since where the module
		// cache keeps it is nothing to the reader of the report.
		return u.path + "/" + filepath.Base(position.Filename), position.Line
	}
	name := position.Filename
	if u.root != "" {
		if rel, err := filepath.Rel(u.root, name); err == nil {
			name = rel
		}
	}
	return filepath.ToSlash(name), position.Line
}

// judge reports every use in the unit of a symbol outside rawWriter may not
// name. It counts the uses the writer made, so a caller can tell a clean tree
// from a walk that read nothing at all, and says which entries of readOnly were
// used, so a permission nobody uses can be found.
func judge(u *unit) (found []site, writerUses int, used map[string]bool) {
	used = map[string]bool{}
	type use struct {
		pos  token.Pos
		what string
	}
	var uses []use
	permitted := permittedCalls(u)
	permits, at := writers[u.path].permits, sitesOf(u)
	for id, obj := range u.info.Uses {
		sym, resolved := resolve(obj)
		if p, held := permits[sym.key]; resolved && held {
			// A name a writer is permitted is judged by its permit, use by use,
			// and by nothing else.
			if what, refused := p.refuses(u, sym.key, id, at); refused {
				uses = append(uses, use{id.Pos(), what})
			} else {
				writerUses++
				used[permitKey(u.path, sym.key)] = true
			}
			if readOnly[sym.key] {
				used[sym.key] = true
			}
			continue
		}
		if why := testOnly[sym.key]; resolved && why != "" && u.path != rawWriter {
			uses = append(uses, use{id.Pos(), sym.key + ", which " + why})
		} else if what, refused := refusedUse(obj, id, permitted, u.dependency); refused {
			uses = append(uses, use{id.Pos(), what})
			// The writer may name anything, so what it uses says nothing of readOnly;
			// it is what keeps writerQueries from holding a name nothing asks with.
			if u.path == rawWriter && resolved && slices.Contains(writerQueries, sym.key) {
				used[sym.key] = true
			}
		} else if resolved && readOnly[sym.key] && !u.dependency && u.path != rawWriter {
			used[sym.key] = true
		}
	}
	slices.SortFunc(uses, func(a, b use) int { return cmp.Compare(a.pos, b.pos) })
	for _, one := range uses {
		if u.path == rawWriter {
			writerUses++
			continue
		}
		path, line := u.rel(one.pos)
		found = append(found, site{path: path, line: line, text: one.what})
	}
	return found, writerUses, used
}

// refusedUse is what a use of a name is refused as outside rawWriter, or that it
// is not: refusal, and for a constrainedCalls name, the argument it is called
// with.
func refusedUse(obj types.Object, id *ast.Ident, permitted map[*ast.Ident]bool, dependency bool) (string, bool) {
	if what, refused := refusal(obj, dependency); refused {
		return what, true
	}
	if sym, ok := resolve(obj); ok {
		if _, constrained := constrainedCalls[sym.key]; constrained && !permitted[id] {
			return sym.key + ", which is not called with the one argument it is allowed with", true
		}
	}
	return "", false
}

// shape is the rule that makes the writer's own signatures carry the other half:
// an exported function or method of rawWriter that changes a file, however many
// of its own helpers away, is handed a Dir, as a parameter or as a receiver that
// holds one. A method is judged whatever its type is called, since a value of an
// unexported type can still be handed out. Without it a new function that took
// a path would be one more way to write into a vault, and nothing but its reader
// would notice.
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
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Body == nil {
				continue
			}
			if obj, isDef := u.info.Defs[fn.Name].(*types.Func); isDef {
				functions = append(functions, &function{decl: fn, obj: obj})
			}
		}
	}
	permitted := permittedCalls(u)
	// What a use outside the writer would be refused for, it changes here too,
	// except asking whether a process is there with signal 0, or where the
	// configuration directory is.
	changes := func(id *ast.Ident, obj types.Object) bool {
		if _, refused := refusedUse(obj, id, permitted, false); !refused {
			return false
		}
		sym, _ := resolve(obj)
		if !slices.Contains(writerQueries, sym.key) {
			return true
		}
		_, constrained := constrainedCalls[sym.key]
		return constrained && !permitted[id]
	}
	for id, obj := range u.info.Uses {
		for _, fn := range functions {
			if id.Pos() < fn.decl.Body.Pos() || id.Pos() >= fn.decl.Body.End() {
				continue
			}
			if changes(id, obj) {
				fn.writes = true
			}
			if callee, isFunc := obj.(*types.Func); isFunc && callee.Pkg() == u.pkg {
				fn.calls = append(fn.calls, callee.Origin())
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, fn := range functions {
			if fn.writes {
				continue
			}
			for _, callee := range fn.calls {
				if i := slices.IndexFunc(functions, func(other *function) bool { return other.obj == callee }); i >= 0 && functions[i].writes {
					fn.writes, changed = true, true
					break
				}
			}
		}
	}
	dir := u.pkg.Scope().Lookup(w.carrier.name)
	var found []site
	for _, fn := range functions {
		if !fn.writes || !fn.obj.Exported() || handedADir(fn.obj.Signature(), dir) {
			continue
		}
		name := fn.obj.Name()
		if recv := fn.obj.Signature().Recv(); recv != nil {
			name = typeName(recv.Type()) + "." + name
		}
		path, line := u.rel(fn.decl.Pos())
		found = append(found, site{path: path, line: line, text: name + " changes a file, or a process, and takes no " + w.carrier.name + ", so a caller could point it anywhere"})
	}
	return found
}

// handedADir says whether a function's caller hands it a Dir: its receiver, or
// one of its parameters, is a Dir or holds one in any field.
func handedADir(sig *types.Signature, dir types.Object) bool {
	if dir == nil {
		return false
	}
	carries := holding{is: isType(dir), field: func(*types.Var) bool { return true }}
	if recv := sig.Recv(); recv != nil && carries.in(recv.Type()) {
		return true
	}
	for param := range sig.Params().Variables() {
		if carries.in(param.Type()) {
			return true
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

// dirMakers are the only functions of rawWriter that make a Dir or return one:
// the reader's directory, and one vault's directory inside it. A function that
// made a Dir from any other string, or one that moved a Dir to another place,
// would be a way to aim a write at a vault with every signature still taking a
// Dir.
var dirMakers = []string{"Home", "VaultDir"}

// configDirMakers are the only functions of rawWriter that make a ConfigDir,
// the configuration directory every Dir is made inside: Platform, from what the
// platform names, and At, for a test, which judge refuses in shipped code.
// Without ConfigDir, Home and VaultDir would take a string again, so a vault's
// root could be the directory a Dir is made in.
var configDirMakers = []string{"Platform", "At"}

// testOnly are the names of rawWriter that tests use and shipped code may not,
// each with why. judge refuses them outside rawWriter; a test is not read.
var testOnly = map[string]string{
	rawWriter + ".At": "makes a configuration directory of any path a test chooses, and shipped code takes the one confdir.Platform names",
}

// dirIsOpaque holds the types a writer is pointed with to what they say: the
// carrier and each name type of writers, a struct nothing outside the package
// can fill, that only its makers make, set or hand out, and that no variable of
// the package hands out either. A Dir that a string converts to, that has a
// field to set, or that a method could re-aim would leave the compiler saying
// nothing about where a write goes. So does a function held in a variable or
// handed back by an exported one, whose calls shape cannot follow.
func dirIsOpaque(u *unit) []site {
	w, isWriter := writers[u.path]
	if !isWriter {
		return nil
	}
	at := func(pos token.Pos, text string) site {
		path, line := u.rel(pos)
		return site{path: path, line: line, text: text}
	}
	function := holding{is: func(t types.Type) bool { _, isSig := t.(*types.Signature); return isSig }, field: func(*types.Var) bool { return true }}
	var found []site
	const handsOut = " returns a function, or something that holds one: what calling it changes is not something shape can follow"
	for _, name := range u.pkg.Scope().Names() {
		switch member := u.pkg.Scope().Lookup(name).(type) {
		case *types.Func:
			if member.Exported() && returns(member.Signature(), function) {
				found = append(found, at(member.Pos(), member.Name()+handsOut))
			}
		case *types.Var:
			if function.in(member.Type()) {
				found = append(found, at(member.Pos(), member.Name()+" is a variable that holds a function: what calling it changes is not something shape can follow, and anyone can call or set an exported one"))
			}
		case *types.TypeName:
			if typed, ok := member.Type().(*types.Named); ok {
				for method := range typed.Methods() {
					if method.Exported() && returns(method.Signature(), function) {
						found = append(found, at(method.Pos(), typed.Obj().Name()+"."+method.Name()+handsOut))
					}
				}
			}
		}
	}
	for _, kind := range append([]opaqueType{w.carrier}, w.names...) {
		found = append(found, opaque(u, kind)...)
	}
	return found
}

// returns says whether a function hands back something what accepts.
func returns(sig *types.Signature, what holding) bool {
	for result := range sig.Results().Variables() {
		if what.in(result.Type()) {
			return true
		}
	}
	return false
}

// opaque is dirIsOpaque for one type.
func opaque(u *unit, kind opaqueType) []site {
	at := func(pos token.Pos, text string) site {
		path, line := u.rel(pos)
		return site{path: path, line: line, text: text}
	}
	name, makers := kind.name, strings.Join(kind.makers, " and ")
	object := u.pkg.Scope().Lookup(name)
	if object == nil {
		if kind.optional {
			return nil
		}
		return []site{at(u.files[0].Pos(), "there is no type "+name+", so no write is held to "+kind.place+" by its type")}
	}
	var found []site
	named, isNamed := object.Type().(*types.Named)
	if !isNamed {
		return []site{at(object.Pos(), name+" is not a defined type")}
	}
	structure, isStruct := named.Underlying().(*types.Struct)
	if !isStruct {
		return []site{at(object.Pos(), name+" is not a struct, so a string converts to it and a "+name+" can be made from any path")}
	}
	for field := range structure.Fields() {
		if field.Exported() {
			found = append(found, at(field.Pos(), name+" has an exported field, "+field.Name()+", so a "+name+" can be filled in from outside the package"))
		}
	}
	// What a caller outside the package can reach: a value, what it points to or
	// holds, its exported fields, and what a function takes or returns.
	reached := holding{is: isType(object), field: (*types.Var).Exported, throughFuncs: true}
	for _, member := range u.pkg.Scope().Names() {
		switch member := u.pkg.Scope().Lookup(member).(type) {
		case *types.Func:
			if member.Exported() && returns(member.Signature(), reached) && !slices.Contains(kind.makers, member.Name()) {
				found = append(found, at(member.Pos(), member.Name()+" returns a "+name+", or something that holds one, and is not one of "+makers+", which are the only ways to make one"))
			}
		case *types.Var:
			if member.Exported() && reached.in(member.Type()) {
				found = append(found, at(member.Pos(), member.Name()+" is a variable that holds a "+name+", which anyone could set to a "+name+" of their own or read one from"))
			}
		case *types.TypeName:
			typed, ok := member.Type().(*types.Named)
			if !ok {
				continue
			}
			if other, isStruct := typed.Underlying().(*types.Struct); isStruct && member.Exported() && typed.Obj() != object {
				for field := range other.Fields() {
					if field.Exported() && reached.in(field.Type()) {
						found = append(found, at(field.Pos(), typed.Obj().Name()+"."+field.Name()+" holds a "+name+", which anyone could set to a "+name+" of their own"))
					}
				}
			}
			for method := range typed.Methods() {
				if method.Exported() && returns(method.Signature(), reached) {
					found = append(found, at(method.Pos(), typed.Obj().Name()+"."+method.Name()+" returns a "+name+", which would put one somewhere else in "+kind.place+" or outside it"))
				}
			}
		}
	}
	return append(found, dirMadeElsewhere(u, object, structure, kind)...)
}

// dirMadeElsewhere reports, outside the bodies of the type's makers, every
// place a value of it is made or what one holds is set: a literal of it, a
// conversion to it, a struct with its fields, which it is assignable from and
// converts from, and an assignment to one of its fields or a pointer taken to
// one. It is read from the syntax with the types, so a promoted field, an
// elided literal type and a type of another name are the type they are.
func dirMadeElsewhere(u *unit, dir types.Object, structure *types.Struct, kind opaqueType) []site {
	name, makers := kind.name, strings.Join(kind.makers, " and ")
	fields := map[types.Object]bool{}
	for field := range structure.Fields() {
		fields[field] = true
	}
	isDir := isType(dir)
	// setsAField says whether an expression written to names a field of the
	// type, however deep the selector is.
	setsAField := func(expr ast.Expr) (string, bool) {
		for {
			switch e := expr.(type) {
			case *ast.ParenExpr:
				expr = e.X
			case *ast.StarExpr:
				expr = e.X
			case *ast.IndexExpr:
				expr = e.X
			case *ast.SelectorExpr:
				if selection, ok := u.info.Selections[e]; ok && selection.Kind() == types.FieldVal && fields[selection.Obj()] {
					return selection.Obj().Name(), true
				}
				expr = e.X
			default:
				return "", false
			}
		}
	}
	var found []site
	report := func(node ast.Node, text string) {
		path, line := u.rel(node.Pos())
		found = append(found, site{path: path, line: line, text: text})
	}
	set := func(node ast.Node, expr ast.Expr) {
		if field, ok := setsAField(expr); ok {
			report(node, field+", a field of "+name+", is set here, and only "+makers+" set one")
		}
	}
	for _, file := range u.files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				if n.Recv == nil && slices.Contains(kind.makers, n.Name.Name) {
					return false
				}
			case *ast.TypeSpec:
				if u.info.Defs[n.Name] == dir {
					return false
				}
			case *ast.StructType:
				if tv, ok := u.info.Types[n]; ok && types.Identical(tv.Type, structure) {
					report(n, "a struct with the fields of "+name+", which is assignable and converts to one, so a "+name+" can be made from it outside "+makers)
				}
			case *ast.CompositeLit:
				if isDir(u.info.TypeOf(n)) {
					report(n, "a "+name+" is made here, and only "+makers+" make one")
				}
			case *ast.CallExpr:
				if tv, ok := u.info.Types[n.Fun]; ok && tv.IsType() && (isDir(tv.Type) || isDir(pointed(tv.Type))) {
					report(n, "a value is converted to a "+name+" here, and only "+makers+" make one")
				}
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					set(lhs, lhs)
				}
			case *ast.IncDecStmt:
				set(n, n.X)
			case *ast.RangeStmt:
				if n.Tok == token.ASSIGN {
					for _, target := range []ast.Expr{n.Key, n.Value} {
						if target != nil {
							set(target, target)
						}
					}
				}
			case *ast.UnaryExpr:
				if field, ok := setsAField(n.X); ok && n.Op == token.AND {
					report(n, "the address of "+field+", a field of "+name+", is taken here, so it can be set outside "+makers)
				}
			}
			return true
		})
	}
	return found
}

// isType is a test for one defined type, through any alias of it.
func isType(object types.Object) func(types.Type) bool {
	return func(t types.Type) bool {
		named, isNamed := types.Unalias(t).(*types.Named)
		return isNamed && named.Obj() == object
	}
}

// pointed is what a pointer type points to, or nil.
func pointed(t types.Type) types.Type {
	if pointer, isPointer := types.Unalias(t).(*types.Pointer); isPointer {
		return pointer.Elem()
	}
	return nil
}

// holding is a question asked of a type: whether a value of it is one that is
// accepts, or holds one. A value holds what a pointer, slice, array, map or
// channel of it holds, what those of its struct fields that field accepts hold,
// and, when throughFuncs is set, what a function takes or returns.
type holding struct {
	is           func(types.Type) bool
	field        func(*types.Var) bool
	throughFuncs bool
}

func (h holding) in(t types.Type) bool { return h.walk(t, map[types.Type]bool{}) }

func (h holding) walk(t types.Type, seen map[types.Type]bool) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	if h.is(t) {
		return true
	}
	switch typ := t.(type) {
	case *types.Named:
		return h.walk(typ.Underlying(), seen)
	case *types.Pointer:
		return h.walk(typ.Elem(), seen)
	case *types.Slice:
		return h.walk(typ.Elem(), seen)
	case *types.Array:
		return h.walk(typ.Elem(), seen)
	case *types.Chan:
		return h.walk(typ.Elem(), seen)
	case *types.Map:
		return h.walk(typ.Key(), seen) || h.walk(typ.Elem(), seen)
	case *types.Struct:
		for field := range typ.Fields() {
			if h.field(field) && h.walk(field.Type(), seen) {
				return true
			}
		}
	case *types.Signature:
		if !h.throughFuncs {
			return false
		}
		for param := range typ.Params().Variables() {
			if h.walk(param.Type(), seen) {
				return true
			}
		}
		for result := range typ.Results().Variables() {
			if h.walk(result.Type(), seen) {
				return true
			}
		}
	}
	return false
}

// world is every package of the module read with its types, for one platform,
// and every package they import: one universe, so that a package named twice is
// one package.
type world struct {
	byPath map[string]*packages.Package
	// production are the module's own packages that have source.
	production []*packages.Package
	// dependencies are the packages of other modules that are linked in, and
	// modules the modules they come from.
	dependencies []*packages.Package
	root         string
	target       target
	// problems are what is wrong with the code the program links that is not a
	// name a use of it makes.
	problems []string
	// starters is startersReached for the packages of this world.
	starters map[string]string
}

// Import lets a fixture name any package the world holds, so that a fixture is
// type-checked against the same packages the tree is.
func (w *world) Import(path string) (*types.Package, error) {
	if p, ok := w.byPath[path]; ok && p.Types != nil {
		return p.Types, nil
	}
	return nil, fmt.Errorf("the world holds no package %q; add it to fixtureImports", path)
}

// fixtureImports are the packages a fixture may name beyond the ones the tree
// imports itself. Each is loaded on top of the tree, so the two share packages.
var fixtureImports = []string{
	"context", "io", "net", "net/http", "os", "os/exec", "io/ioutil", "plugin", "runtime/coverage", "syscall",
	"net/http/cgi", "go/build", "go/importer", "mime/multipart", "runtime/debug",
}

var (
	worldsMu sync.Mutex
	worlds   = map[target]*sync.Once{}
	loaded   = map[target]*world{}
	loadErrs = map[target]error{}
)

// loadWorld reads the module for one platform, once. CGO is off as the release
// is built, so what is read is what is shipped and no C compiler is needed.
func loadWorld(at target) (*world, error) {
	worldsMu.Lock()
	once, ok := worlds[at]
	if !ok {
		once = new(sync.Once)
		worlds[at] = once
	}
	worldsMu.Unlock()
	once.Do(func() {
		w, err := readWorld(at)
		worldsMu.Lock()
		loaded[at], loadErrs[at] = w, err
		worldsMu.Unlock()
	})
	worldsMu.Lock()
	defer worldsMu.Unlock()
	return loaded[at], loadErrs[at]
}

func readWorld(at target) (*world, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	// Every package of the module, so that one added in a new directory is read
	// and not left for the check that every file was read to find.
	patterns := []string{"./..."}
	if at == fixtureTarget {
		patterns = append(patterns, fixtureImports...)
	}
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir: root,
		// What the release is built with and not what the shell that ran the test
		// has set: GOFLAGS could add files to or take them from the load, and a
		// workspace could swap a module for another. An empty GOFLAGS gives way to
		// one go env -w wrote, so the go env file is not read at all.
		Env:   append(os.Environ(), "GOOS="+at.goos, "GOARCH="+at.goarch, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off", "GOENV=off"),
		Tests: false,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("read the module for %s: %w", at, err)
	}
	w := &world{byPath: map[string]*packages.Package{}, root: root, target: at}
	packages.Visit(pkgs, nil, func(p *packages.Package) { w.byPath[p.PkgPath] = p })
	w.starters = startersReached(w.byPath)
	for _, path := range slices.Sorted(maps.Keys(w.byPath)) {
		p := w.byPath[path]
		// A package that did not load is not read, so it fails the test whatever it
		// holds; what is checked for first is the error and not the files.
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("read the module for %s: %s did not load: %w", at, p.PkgPath, p.Errors[0])
		}
		// A directory that holds only tests has no package to read.
		if len(p.GoFiles) == 0 {
			continue
		}
		switch whose(p) {
		case ownCode:
			w.production = append(w.production, p)
		case anotherModule:
			w.dependencies = append(w.dependencies, p)
		case standardLibrary:
			continue
		}
		w.problems = append(w.problems, linkProblems(p)...)
	}
	return w, nil
}

// owner is whose code a package is.
type owner int

const (
	standardLibrary owner = iota
	ownCode
	anotherModule
)

// whose says whose code a package is by the module it comes from, which is what
// go.mod says and not how its path is spelled: a module path need not have a dot
// in it, and one can begin with this module's path and be another module.
func whose(p *packages.Package) owner {
	switch {
	case p.Module == nil:
		return standardLibrary
	case p.Module.Path == module:
		return ownCode
	}
	return anotherModule
}

// linkProblems is what is wrong with a package of the program that the names its
// code uses do not show: that it comes from a module nobody has reviewed, that
// go.mod swaps the module for other code, or that it holds a file in a language
// this guard does not read.
func linkProblems(p *packages.Package) []string {
	var problems []string
	if whose(p) == anotherModule {
		if !slices.Contains(reviewedModules, p.Module.Path) {
			problems = append(problems, fmt.Sprintf("%s is a package of %s, which is not one of reviewedModules: this guard reads its code for names and no more, so linking another module is a decision", p.PkgPath, p.Module.Path))
		}
		if p.Module.Replace != nil {
			problems = append(problems, fmt.Sprintf("%s comes from %s, which go.mod replaces with %s", p.PkgPath, p.Module.Path, p.Module.Replace.Path))
		}
	}
	if len(p.OtherFiles) > 0 || slices.ContainsFunc(p.IgnoredFiles, func(name string) bool { return !strings.HasSuffix(name, ".go") }) {
		problems = append(problems, p.PkgPath+" holds a file that is not Go (assembly or C): this guard reads Go only")
	}
	return problems
}

// notGoSources are the extensions of the files the go command compiles besides
// Go: assembly and objects, C and its relatives. A system call needs none of the
// Go this guard reads, so a package that has one is a package it cannot judge.
var notGoSources = []string{".s", ".S", ".sx", ".syso", ".c", ".h", ".hh", ".hpp", ".cc", ".cpp", ".cxx", ".m", ".f", ".F", ".for", ".f90", ".swig", ".swigcxx"}

// sourcesBeyondGo lists, under root, the files in a language the guard does not
// read, the go.mod and go.work files besides the root's, and a vendor directory,
// wherever they are: the go command skips a directory whose name begins with a
// dot or an underscore, but a module that go.mod replaces with a path there is
// built from it, and a vendored copy is built without being checked against
// go.sum.
func sourcesBeyondGo(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			switch name {
			case ".git", "node_modules":
				return fs.SkipDir
			case "vendor":
				found = append(found, rel+": a vendored copy of modules, which go builds without checking it against go.sum")
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case slices.Contains(notGoSources, filepath.Ext(name)):
			found = append(found, rel+": a file in a language this guard does not read")
		case name == "go.work" || name == "go.work.sum":
			found = append(found, rel+": a workspace, which can swap a module for another")
		case name == "go.mod" && rel != "go.mod":
			found = append(found, rel+": another module inside this one")
		}
		return nil
	})
	slices.Sort(found)
	return found, err
}

// replacesIn lists the lines of a go.mod that replace a module with other code.
func replacesIn(goMod string) []string {
	var found []string
	for line := range strings.SplitSeq(goMod, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && (fields[0] == "replace" || strings.Contains(line, "=>")) {
			found = append(found, strings.TrimSpace(line))
		}
	}
	return found
}

// dependencyUnits are the linked packages of other modules, to be read for the
// same names as this module's, except those of systemCalls.
func (w *world) dependencyUnits() []*unit {
	var units []*unit
	for _, p := range w.dependencies {
		if p.Module.Path == systemCalls {
			continue
		}
		units = append(units, &unit{path: p.PkgPath, fset: p.Fset, files: p.Syntax, info: p.TypesInfo, pkg: p.Types, root: w.root, dependency: true})
	}
	return units
}

func (w *world) units() []*unit {
	var units []*unit
	for _, p := range w.production {
		units = append(units, &unit{path: p.PkgPath, fset: p.Fset, files: p.Syntax, info: p.TypesInfo, pkg: p.Types, root: w.root})
	}
	return units
}

// TestTheOnlyPlaceProductionCodeWritesIsTheReadersOwnFiles reads every shipped
// package, on every platform the release is built for, for a name outside the
// package that writes that this guard does not know to leave a file as it was,
// and for a function of that package that takes no Dir.
//
// A second writer is what this exists to catch, and above all one that opens a
// vault file for writing: yomihon is read-only toward the vault. Nothing else
// here writes, and the two documents that tell a stranger so are held to the
// same tree by the checks beside it.
func TestTheOnlyPlaceProductionCodeWritesIsTheReadersOwnFiles(t *testing.T) {
	t.Parallel()

	results := make([]systemWalk, len(releaseTargets))
	var reading sync.WaitGroup
	for i, at := range releaseTargets {
		reading.Go(func() { results[i] = walkSystem(at) })
	}
	reading.Wait()

	var (
		systemsOf = map[site][]string{}
		order     []site
		analysed  = map[string]bool{}
		used      = map[string]bool{}
		problems  = map[string]bool{}
		linked    = map[string]bool{}
		imported  = map[string]bool{}
	)
	for _, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		// Each writer has to have been seen writing. A walk that never reached one
		// would report a tree with no writes in it at all, which is the one result
		// that would look like success and mean nothing.
		for _, writer := range slices.Sorted(maps.Keys(writers)) {
			if result.written[writer] == 0 {
				t.Fatalf("no write was seen in %s on %s, so this walk never reached the package it is meant to permit", writer, result.at)
			}
		}
		for _, one := range result.found {
			if _, seen := systemsOf[one]; !seen {
				order = append(order, one)
			}
			systemsOf[one] = append(systemsOf[one], result.at.String())
		}
		maps.Copy(analysed, result.files)
		maps.Copy(used, result.used)
		maps.Copy(linked, result.modules)
		maps.Copy(imported, result.imports)
		for _, problem := range result.problems {
			problems[problem] = true
		}
	}
	for _, problem := range slices.Sorted(maps.Keys(problems)) {
		t.Error(problem)
	}
	for _, module := range reviewedModules {
		if !linked[module] {
			t.Errorf("%s is on reviewedModules and no package of it is linked; remove it", module)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(startersAllowed)) {
		if !imported[path] {
			t.Errorf("%s is on startersAllowed and no package imports it; remove it", path)
		}
	}
	beyond, err := sourcesBeyondGo(repoRoot)
	if err != nil {
		t.Fatalf("walk the repository for what is not Go: %v", err)
	}
	for _, one := range beyond {
		t.Error(one)
	}
	goMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod")) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range replacesIn(string(goMod)) {
		t.Errorf("go.mod: %s, which builds the program from other code than the module it names", line)
	}
	var found []site
	for _, one := range order {
		if systems := systemsOf[one]; len(systems) < len(releaseTargets) {
			one.text += " (on " + strings.Join(systems, ", ") + " only)"
		}
		found = append(found, one)
	}
	report(t, "this may change a file or reach past what this guard can read, and yomihon writes only the reader's own files, through internal/mark/confdir, and this machine's file of marks, through internal/mark/machinefile", found)

	// A permission nobody uses is one more name that may be written later without
	// anyone having decided it. The writer's own uses do not count for readOnly,
	// since the writer may name anything.
	for _, key := range slices.Sorted(maps.Keys(readOnly)) {
		if !used[key] {
			t.Errorf("%s is on readOnly and no shipped package outside %s names it; remove it", key, rawWriter)
		}
	}
	for _, key := range writerQueries {
		if !used[key] {
			t.Errorf("%s is on writerQueries and %s does not name it; remove it", key, rawWriter)
		}
	}
	for _, writer := range slices.Sorted(maps.Keys(writers)) {
		for _, key := range slices.Sorted(maps.Keys(writers[writer].permits)) {
			if !used[permitKey(writer, key)] {
				t.Errorf("%s is permitted to %s and no use of it there is one its permit allows; remove it", key, writer)
			}
		}
	}
	// A file that no platform builds is a file nothing here read.
	for _, path := range productionFiles(t, ".go") {
		if !analysed[path] {
			t.Errorf("%s was not read on any of %v: a build constraint keeps it out of every load, so what it does is not checked", path, releaseTargets)
		}
	}
}

// reviewedModules are the modules other than this one whose packages are linked
// into the program. The guard reads them for the same names as this module's own,
// and no more: reflection, assembly and what they do with a path they are handed
// by this module are not read, so a module that is not on this list is code that
// nobody has decided to trust. systemCalls is on it and is not read at all, as
// syscall is not. It is every module with a package in the build, which is a
// smaller set than the ones go.mod requires.
var reviewedModules = []string{
	"github.com/BurntSushi/toml",
	"github.com/a-h/templ",
	"github.com/alecthomas/chroma/v3",
	"github.com/dlclark/regexp2/v2",
	"github.com/tats-u/goldmark-cjk-friendly/v2",
	"github.com/yuin/goldmark",
	"go.yaml.in/yaml/v3",
	"golang.org/x/net",
	systemCalls,
	"golang.org/x/text",
}

// TestTheSystemCallsAreLinkedAndNotRead holds the one exception to reading what
// is linked to what it is: the packages of systemCalls, and nothing else, are
// left unread. It is read on a platform that links one, so the rule is seen
// deciding something, and it reads that package anyway to show that leaving it
// out is what keeps its own code from being refused.
func TestTheSystemCallsAreLinkedAndNotRead(t *testing.T) {
	t.Parallel()
	w, err := loadWorld(target{"windows", "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	read := map[string]bool{}
	for _, u := range w.dependencyUnits() {
		read[u.path] = true
	}
	var unread []*packages.Package
	for _, p := range w.dependencies {
		if p.Module.Path == systemCalls {
			unread = append(unread, p)
		}
		if read[p.PkgPath] == (p.Module.Path == systemCalls) {
			t.Errorf("%s, a package of %s, is read: %v", p.PkgPath, p.Module.Path, read[p.PkgPath])
		}
	}
	if len(unread) == 0 {
		t.Fatalf("no package of %s is linked on windows/amd64, so nothing here is left unread", systemCalls)
	}
	for _, p := range unread {
		u := &unit{path: p.PkgPath, fset: p.Fset, files: p.Syntax, info: p.TypesInfo, pkg: p.Types, root: w.root, dependency: true}
		if found, _, _ := judge(u); len(found) == 0 {
			t.Errorf("%s, read for names, is refused nothing, so leaving it unread decides nothing", p.PkgPath)
		}
	}
}

// dependencyReads are the names of os that a package of another module may use
// besides the ones readOnly lists, each with what it is for: a file it is handed
// is opened to be read, and a directory for scratch files is named and not made.
var dependencyReads = map[string]string{
	"os.Open":    "the TOML decoder and templ's runtime open a file they are handed, to read",
	"os.TempDir": "templ's runtime names the directory for temporary files and makes none",
}

// systemWalk is what reading the tree for one platform found.
type systemWalk struct {
	at    target
	found []site
	// written is how many uses of what changes a file each writer made.
	written map[string]int
	used    map[string]bool
	files   map[string]bool
	modules map[string]bool
	// imports are the import paths the program's packages write.
	imports  map[string]bool
	problems []string
	err      error
}

func walkSystem(at target) systemWalk {
	out := systemWalk{at: at, written: map[string]int{}, used: map[string]bool{}, files: map[string]bool{}, modules: map[string]bool{}, imports: map[string]bool{}}
	w, err := loadWorld(at)
	if err != nil {
		out.err = err
		return out
	}
	out.problems = w.problems
	for _, p := range w.dependencies {
		out.modules[p.Module.Path] = true
	}
	for _, p := range append(slices.Clone(w.production), w.dependencies...) {
		for path := range p.Imports {
			out.imports[path] = true
		}
	}
	for _, u := range w.dependencyUnits() {
		sites, _, _ := judge(u)
		out.found = append(out.found, sites...)
		out.found = append(out.found, startsAProgram(u, w.starters)...)
	}
	for _, u := range w.units() {
		sites, uses, usedHere := judge(u)
		out.found = append(out.found, sites...)
		out.found = append(out.found, startsAProgram(u, w.starters)...)
		out.written[u.path] += uses
		maps.Copy(out.used, usedHere)
		out.found = append(out.found, shape(u)...)
		out.found = append(out.found, dirIsOpaque(u)...)
		out.found = append(out.found, forbidden(u)...)
		for _, file := range u.files {
			path, _ := u.rel(file.Pos())
			out.files[path] = true
		}
	}
	return out
}

// TestNoWriteIsOnTheReadOnlyList holds the list the guard permits to what it
// says it is. Someone adding a symbol is deciding it cannot change a file, and
// this is the check on a decision made in a hurry: a name that opens a family of
// writes is refused whatever package it is in.
func TestNoWriteIsOnTheReadOnlyList(t *testing.T) {
	t.Parallel()
	for _, key := range writerQueries {
		if readOnly[key] {
			t.Errorf("%s is on readOnly and on writerQueries, and belongs on one", key)
		}
	}
	for _, problem := range readOnlyProblems(slices.Sorted(maps.Keys(readOnly))) {
		t.Error(problem)
	}
}

// readOnlyProblems is what is wrong with names put on readOnly: one a write
// answers to, or one of a package that does not reach the filesystem, which the
// list would be judging for nothing.
func readOnlyProblems(keys []string) []string {
	var problems []string
	for _, key := range keys {
		name := key[strings.LastIndexByte(key, '.')+1:]
		if hasPrefixIn(name, writeFamilies) || strings.HasPrefix(name, "Write") {
			problems = append(problems, key+" is on readOnly and its name is one a write answers to")
		}
		if !reachesFilesystem(keyPackage(key)) {
			problems = append(problems, key+" is on readOnly and names a package that does not reach the filesystem, so the list is judging nothing")
		}
	}
	return problems
}

// keyPackage is the package a key of readOnly names: a key is package.Name or
// package.Type.Method, and the last element of a package path holds no dot.
func keyPackage(key string) string {
	slash := strings.LastIndexByte(key, '/') + 1
	dot := strings.IndexByte(key[slash:], '.')
	if dot < 0 {
		return key
	}
	return key[:slash+dot]
}

// The check on readOnly judges a method by the package of its type, so a method
// of a package that reaches no file is caught as a function of one is.
func TestTheReadOnlyCheckFindsWhatDoesNotBelong(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		key  string
		want int
	}{
		{"os.Stat", 0},
		{"os.Root.Open", 0},
		{"golang.org/x/sys/unix.Getpid", 0},
		{"golang.org/x/sys/windows.Handle.Close", 0},
		{"os.Root.WriteFile", 1},
		{"os.Chmod", 1},
		{"net.Dial", 1},
		{"net/http.Request.Context", 1},
		{"golang.org/x/net/html.Node.AppendChild", 1},
		{"net/http.Request.ParseMultipartForm", 1},
		{"os/exec.Cmd.Run", 0},
	} {
		if got := readOnlyProblems([]string{tt.key}); len(got) != tt.want {
			t.Errorf("readOnlyProblems(%q) = %q, want %d problems", tt.key, got, tt.want)
		}
	}
}
