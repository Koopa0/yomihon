package archlock

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// yomihon writes in exactly one place, and that place is outside the vault: the
// reader's own records, in files of yomihon's own under the platform's
// configuration directory. Nothing writes into a vault. The sentence the privacy
// inventory and the threat model both make to a stranger rests on that being true
// of the code rather than of anyone's memory of it, so the code is read with its
// types: a name says what a programmer called something, and a type says what it
// is.
//
// The rule has two halves, and neither is a list of writes.
//
//   - What may be named. Outside the one package that writes, code may name only
//     the symbols of the packages that reach the filesystem or run a program
//     (os, syscall, io/ioutil, os/exec, golang.org/x/sys, plugin) that readOnly
//     lists. Everything else they hold is refused by being absent, so a function
//     the standard library adds tomorrow is refused the day it appears rather
//     than the day somebody remembers a list.
//   - What it may be pointed at. The package that writes, internal/mark/confdir,
//     works on a Dir, which only confdir makes and only from the configuration
//     directory, and takes a Dir for everything that changes a file. A caller that
//     holds none cannot aim a write at a vault, and the compiler is what says so.
//     shape and dirIsOpaque hold the package to it: a function that changes a
//     file takes a Dir, and the Dir stays a type only Home and VaultDir make.
//
// What this does not read: what third-party code does with a path it is handed,
// reflection, and the one string the configuration directory begins as, which
// the command takes from the platform and hands to the stores. A write that
// reaches the disk by any of those is not one a check over this tree can see.

// rawWriter is the one package that may name what changes a file. Everything
// else in internal/mark goes through it, and this guard checks that they do: the
// survey that made this list found no write outside it.
const rawWriter = module + "/internal/mark/confdir"

// releaseSystems are the platforms the release is built for, and so the
// platforms whose files have to be read. A file chosen by a build constraint
// reaches the guard only on the platform that builds it.
var releaseSystems = []string{"darwin", "linux", "windows"}

// reachesFilesystem reports whether a package is one that can change a file or
// run a program that can, which is the line between the packages whose symbols
// are held to readOnly and the ones that are not.
func reachesFilesystem(path string) bool {
	switch path {
	case "os", "os/exec", "io/ioutil", "syscall", "plugin":
		return true
	}
	return strings.HasPrefix(path, "golang.org/x/sys/")
}

// readOnly is every symbol of those packages that code outside rawWriter may
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
		"os.Getenv", "os.Getpid", "os.Getwd", "os.UserConfigDir",
		// Looking at the filesystem without changing it.
		"os.Stat", "os.ReadFile", "os.DirFS", "os.OpenRoot", "os.SameFile",
		"os.DevNull", "os.ModeCharDevice", "os.ModeSymlink",
		"os.File.Close", "os.File.Read", "os.File.Seek", "os.File.Stat",
		"os.Root.Close", "os.Root.FS", "os.Root.Lstat", "os.Root.Open", "os.Root.OpenRoot", "os.Root.Stat",
		// Asking whether a process is still there.
		"os.FindProcess", "os.Process.Signal", "os.Process.Release",
		"syscall.EPERM",
		// What a closed connection and a termination look like.
		"syscall.EPIPE", "syscall.ECONNRESET", "syscall.SIGTERM",
	} {
		readOnly[name] = true
	}
}

// writeFamilies are the prefixes of the names a handle that writes answers to.
// They are prefixes because the standard library grows (Mkdir became MkdirAll
// and MkdirTemp), and they judge a method reached through an interface, where
// the type that answers it cannot be read: a call to Remove on a value whose type
// is an interface may be a call to os.Root.Remove, and so is refused.
var writeFamilies = []string{
	"Chmod", "Chown", "Chtimes", "Create", "Lchown", "Link", "Mkdir", "OpenFile",
	"Remove", "Rename", "Symlink", "Truncate", "WriteFile",
}

// alsoRefused are the symbols outside those packages that write to disk
// anyway: they make files of their own, which the packages above would be
// refused for. None is used, and each is one more way a reader would not look.
var alsoRefused = []string{
	"runtime/debug.WriteHeapDump",
	"net/http.Request.ParseMultipartForm", "net/http.Request.FormFile",
	"mime/multipart.Reader.ReadForm",
	"net.ListenUnix", "net.ListenUnixgram",
}

// forbiddenImports are the ways a program reaches the disk, or the memory the
// type checker reasons about, that no list of symbols can fence: cgo calls any
// C function, unsafe breaks the types this guard rests on, and reflect and the
// template packages call a method by a name that is a string, so that a Remove
// on a root nobody named is not a use of os.Root.Remove anywhere in the source.
// No shipped package imports one of them; one that needs to is a decision.
var forbiddenImports = []string{"C", "unsafe", "reflect", "text/template", "html/template"}

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
	}
	return symbol{}, false
}

// refusal says what to call a use of obj that code outside rawWriter may not
// make, or that there is none.
func refusal(obj types.Object) (string, bool) {
	sym, ok := resolve(obj)
	if !ok {
		return "", false
	}
	if sym.viaIfaceFor != "" {
		if hasPrefixIn(sym.viaIfaceFor, writeFamilies) {
			return "a call through an interface to " + sym.viaIfaceFor + ", which a handle that writes answers", true
		}
		return "", false
	}
	if slices.Contains(alsoRefused, sym.key) {
		return sym.key + ", which makes files of its own", true
	}
	if !reachesFilesystem(sym.path) || readOnly[sym.key] {
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
	for id, obj := range u.info.Uses {
		if what, refused := refusal(obj); refused {
			uses = append(uses, use{id.Pos(), what})
		} else if sym, ok := resolve(obj); ok && readOnly[sym.key] && u.path != rawWriter {
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

// shape is the rule that makes the writer's own signatures carry the other half:
// an exported function of rawWriter that changes a file, however many of its
// own helpers away, takes a Dir or is a method of a type this package makes.
// Without it a new function that took a path would be one more way to write
// into a vault, and nothing but its reader would notice.
func shape(u *unit) []site {
	if u.path != rawWriter {
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
	for id, obj := range u.info.Uses {
		for _, fn := range functions {
			if id.Pos() < fn.decl.Body.Pos() || id.Pos() >= fn.decl.Body.End() {
				continue
			}
			if _, refused := refusal(obj); refused {
				fn.writes = true
			}
			if callee, isFunc := obj.(*types.Func); isFunc && callee.Pkg() == u.pkg {
				fn.calls = append(fn.calls, callee)
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
	var found []site
	for _, fn := range functions {
		if !fn.writes || !fn.obj.Exported() || !receiverExported(fn.decl) {
			continue
		}
		sig, isSig := fn.obj.Type().(*types.Signature)
		if !isSig || sig.Recv() != nil || takesADir(sig, u.pkg) {
			continue
		}
		path, line := u.rel(fn.decl.Pos())
		found = append(found, site{path: path, line: line, text: fn.obj.Name() + " changes a file and takes no Dir, so a caller could point it anywhere"})
	}
	return found
}

// dirMakers are the only functions of rawWriter that return a Dir: one from the
// configuration directory the command resolved, and one inside it. A function
// that made a Dir from any other string, or one that moved a Dir to another
// place, would be a way to aim a write at a vault with every signature still
// taking a Dir.
var dirMakers = []string{"Home", "VaultDir"}

// dirIsOpaque holds the type that carries the other half of the rule to what it
// says: a struct nothing outside the package can fill, and made by dirMakers
// alone. A Dir that a string converts to, or that has a field to set, would
// leave the compiler saying nothing about where a write goes.
func dirIsOpaque(u *unit) []site {
	if u.path != rawWriter {
		return nil
	}
	at := func(pos token.Pos, text string) site {
		path, line := u.rel(pos)
		return site{path: path, line: line, text: text}
	}
	object := u.pkg.Scope().Lookup("Dir")
	if object == nil {
		return []site{at(u.files[0].Pos(), "there is no type Dir, so no write is held to the configuration directory by its type")}
	}
	var found []site
	named, isNamed := object.Type().(*types.Named)
	if !isNamed {
		return []site{at(object.Pos(), "Dir is not a defined type")}
	}
	structure, isStruct := named.Underlying().(*types.Struct)
	if !isStruct {
		return []site{at(object.Pos(), "Dir is not a struct, so a string converts to it and a Dir can be made from any path")}
	}
	for field := range structure.Fields() {
		if field.Exported() {
			found = append(found, at(field.Pos(), "Dir has an exported field, "+field.Name()+", so a Dir can be filled in from outside the package"))
		}
	}
	returnsDir := func(sig *types.Signature) bool {
		for result := range sig.Results().Variables() {
			typ := types.Unalias(result.Type())
			if pointer, isPointer := typ.(*types.Pointer); isPointer {
				typ = types.Unalias(pointer.Elem())
			}
			if got, ok := typ.(*types.Named); ok && got.Obj() == object {
				return true
			}
		}
		return false
	}
	for _, name := range u.pkg.Scope().Names() {
		switch member := u.pkg.Scope().Lookup(name).(type) {
		case *types.Func:
			if sig, ok := member.Type().(*types.Signature); ok && member.Exported() && returnsDir(sig) && !slices.Contains(dirMakers, member.Name()) {
				found = append(found, at(member.Pos(), member.Name()+" returns a Dir and is not one of "+strings.Join(dirMakers, " and ")+", which are the only ways to make one"))
			}
		case *types.TypeName:
			typed, ok := member.Type().(*types.Named)
			if !ok {
				continue
			}
			for method := range typed.Methods() {
				if sig, isSig := method.Type().(*types.Signature); isSig && method.Exported() && returnsDir(sig) {
					found = append(found, at(method.Pos(), typed.Obj().Name()+"."+method.Name()+" returns a Dir, which would put one somewhere else in the configuration directory or outside it"))
				}
			}
		}
	}
	return found
}

func receiverExported(decl *ast.FuncDecl) bool {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return true
	}
	expr := decl.Recv.List[0].Type
	if star, isStar := expr.(*ast.StarExpr); isStar {
		expr = star.X
	}
	if index, isIndex := expr.(*ast.IndexExpr); isIndex {
		expr = index.X
	}
	id, isIdent := expr.(*ast.Ident)
	return isIdent && id.IsExported()
}

func takesADir(sig *types.Signature, pkg *types.Package) bool {
	for param := range sig.Params().Variables() {
		typ := types.Unalias(param.Type())
		if pointer, isPointer := typ.(*types.Pointer); isPointer {
			typ = types.Unalias(pointer.Elem())
		}
		if named, isNamed := typ.(*types.Named); isNamed && named.Obj().Pkg() == pkg && named.Obj().Name() == "Dir" {
			return true
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
	root       string
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
	"context", "io", "net", "net/http", "os", "os/exec", "io/ioutil", "plugin", "syscall",
}

var (
	worldsMu sync.Mutex
	worlds   = map[string]*sync.Once{}
	loaded   = map[string]*world{}
	loadErrs = map[string]error{}
)

// loadWorld reads the module for one platform, once. CGO is off as the release
// is built, so what is read is what is shipped and no C compiler is needed.
func loadWorld(goos string) (*world, error) {
	worldsMu.Lock()
	once, ok := worlds[goos]
	if !ok {
		once = new(sync.Once)
		worlds[goos] = once
	}
	worldsMu.Unlock()
	once.Do(func() {
		w, err := readWorld(goos)
		worldsMu.Lock()
		loaded[goos], loadErrs[goos] = w, err
		worldsMu.Unlock()
	})
	worldsMu.Lock()
	defer worldsMu.Unlock()
	return loaded[goos], loadErrs[goos]
}

func readWorld(goos string) (*world, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	// Every package of the module, so that one added in a new directory is read
	// and not left for the check that every file was read to find.
	patterns := []string{"./..."}
	if goos == "linux" {
		patterns = append(patterns, fixtureImports...)
	}
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:   root,
		Env:   append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0"),
		Tests: false,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("read the module for %s: %w", goos, err)
	}
	w := &world{byPath: map[string]*packages.Package{}, root: root}
	packages.Visit(pkgs, nil, func(p *packages.Package) { w.byPath[p.PkgPath] = p })
	for _, path := range slices.Sorted(maps.Keys(w.byPath)) {
		p := w.byPath[path]
		// A package that did not load is not read, so it fails the test whatever it
		// holds; what is checked for first is the error and not the files.
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("read the module for %s: %s did not load: %w", goos, p.PkgPath, p.Errors[0])
		}
		// A directory that holds only tests has no package to read.
		if len(p.GoFiles) == 0 {
			continue
		}
		if strings.HasPrefix(path, module) && len(p.Syntax) > 0 {
			w.production = append(w.production, p)
		}
	}
	return w, nil
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

	results := make([]systemWalk, len(releaseSystems))
	var reading sync.WaitGroup
	for i, goos := range releaseSystems {
		reading.Go(func() { results[i] = walkSystem(goos) })
	}
	reading.Wait()

	var (
		systemsOf = map[site][]string{}
		order     []site
		analysed  = map[string]bool{}
		used      = map[string]bool{}
	)
	for _, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		// The writer has to have been seen writing. A walk that never reached it
		// would report a tree with no writes in it at all, which is the one result
		// that would look like success and mean nothing.
		if result.written == 0 {
			t.Fatalf("no write was seen in %s on %s, so this walk never reached the package it is meant to permit", rawWriter, result.goos)
		}
		for _, one := range result.found {
			if _, seen := systemsOf[one]; !seen {
				order = append(order, one)
			}
			systemsOf[one] = append(systemsOf[one], result.goos)
		}
		maps.Copy(analysed, result.files)
		maps.Copy(used, result.used)
	}
	var found []site
	for _, one := range order {
		if systems := systemsOf[one]; len(systems) < len(releaseSystems) {
			one.text += " (on " + strings.Join(systems, ", ") + " only)"
		}
		found = append(found, one)
	}
	report(t, "this may change a file or reach past what this guard can read, and yomihon writes in exactly one place, outside the vault: the reader's own files, through internal/mark/confdir", found)

	t.Logf("%d of %d readOnly names are used: %v", len(used), len(readOnly), slices.Sorted(maps.Keys(used)))
	// A permission nobody uses is one more name that may be written later without
	// anyone having decided it.
	for _, key := range slices.Sorted(maps.Keys(readOnly)) {
		if !used[key] {
			t.Errorf("%s is on readOnly and no shipped package outside %s names it; remove it", key, rawWriter)
		}
	}
	// A file that no platform builds is a file nothing here read.
	for _, path := range productionFiles(t, ".go") {
		if !analysed[path] {
			t.Errorf("%s was not read on any of %v: a build constraint keeps it out of every load, so what it does is not checked", path, releaseSystems)
		}
	}
}

// systemWalk is what reading the tree for one platform found.
type systemWalk struct {
	goos    string
	found   []site
	written int
	used    map[string]bool
	files   map[string]bool
	err     error
}

func walkSystem(goos string) systemWalk {
	out := systemWalk{goos: goos, used: map[string]bool{}, files: map[string]bool{}}
	w, err := loadWorld(goos)
	if err != nil {
		out.err = err
		return out
	}
	for _, u := range w.units() {
		sites, uses, usedHere := judge(u)
		out.found = append(out.found, sites...)
		out.written += uses
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
	for key := range readOnly {
		name := key[strings.LastIndexByte(key, '.')+1:]
		if hasPrefixIn(name, writeFamilies) || strings.HasPrefix(name, "Write") {
			t.Errorf("%s is on readOnly and its name is one a write answers to", key)
		}
		if !reachesFilesystem(key[:strings.LastIndexByte(key, '.')]) && !strings.Contains(key[:strings.LastIndexByte(key, '.')], ".") {
			// A method key is package.Type.Method; its package is what is left of
			// the type name.
			pkg, _, _ := strings.Cut(key, ".")
			if !reachesFilesystem(pkg) {
				t.Errorf("%s is on readOnly and names a package that does not reach the filesystem, so the list is judging nothing", key)
			}
		}
	}
}
