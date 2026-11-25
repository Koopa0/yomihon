package archlock

import (
	"cmp"
	"errors"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// machineFileWriter is the one package that may write inside a vault, and only
// this machine's file of marks in the reader folder. It is held to the few
// names machineFilePermits lists, each to what every use of it must be, on top
// of the rules any writer is held to: what changes a file is handed a Folder,
// and only Open makes one.
const machineFileWriter = module + "/internal/mark/machinefile"

// opaqueType is a type of a writer that only its makers make: what a write is
// pointed with, or a name a write may use.
type opaqueType struct {
	name   string
	makers []string
	// place is where a value of it holds a write to, for a message.
	place string
}

// writer is a package that may name what changes a file, and what holds it.
type writer struct {
	// carrier is the type every exported function of it that changes a file is
	// handed, so a write goes where the carrier was pointed.
	carrier opaqueType
	// names are further types that only their makers make.
	names []opaqueType
	// permits are the names it may use beyond readOnly, each with what every use
	// of it must be. Nil is any name at all, which is what confdir may use.
	permits map[string]permit
}

// writers are the packages that write. confdir writes anything, inside a Dir;
// machinefile writes one file of the reader folder, inside a Folder, by the
// names it is permitted.
var writers = map[string]writer{
	rawWriter: {carrier: opaqueType{name: "Dir", makers: dirMakers, place: "the configuration directory"}},
	machineFileWriter: {
		carrier: opaqueType{name: "Folder", makers: []string{"Open"}, place: "the reader folder"},
		names: []opaqueType{
			{name: "tempName", makers: []string{"newTemp", "staleTemps"}, place: "a temporary file"},
			{name: "machineFileName", makers: []string{"machineFile"}, place: "this machine's file"},
		},
		permits: machineFilePermits,
	},
}

// readerFolders are the folders machinefile may make and enter, each through
// the one above it: the reader folder, .yomihon inside it and marks inside that.
// They are written here and not read from the package, so a folder added there
// is one a reviewer decides on here.
var readerFolders = []string{"reader-notes", ".yomihon", "marks"}

// windowsSystemCalls is the package of systemCalls the Windows directory flush
// names.
const windowsSystemCalls = systemCalls + "/windows"

// flushFile is the one file of machinefile that may name the Windows system
// calls: what reopens a folder for writing so that it can be flushed, as
// confdir's sync_windows.go does, and nothing else.
const flushFile = "sync_windows.go"

// machineFilePermits is every name machinefile may use beyond readOnly, with
// what each use must be. A name that is not here or on readOnly is refused, so
// Link, WriteFile, RemoveAll, MkdirAll, Create, Truncate, Chmod, Chtimes,
// Symlink and every function of os that writes by a path are refused by being
// absent.
//
// It is what makes the reader folder safe by its types: a temporary file is the
// only name opened to write or removed, and the only rename moves a temporary
// file over this machine's file. So nothing in the folder but this machine's
// file is ever made, opened to write, removed or renamed, and the reader's own
// Markdown least of all. Which name machineFile gives this machine's file, and
// which vault Open opens, are not read here; machinefile's tests answer those.
var machineFilePermits = map[string]permit{
	// The vault, opened again from the path the server pinned, and only where a
	// Folder is made.
	"os.OpenRoot": {in: "Open", call: true, args: []argument{free}},
	// The folders on the way, each made and entered by one of readerFolders.
	"os.Root.OpenRoot": {call: true, args: []argument{oneOf(readerFolders...)}},
	"os.Root.Mkdir":    {call: true, args: []argument{oneOf(readerFolders...), free}},
	// A temporary file, made and never opened again: the flags create it, and
	// fail if anything is there already.
	"os.Root.OpenFile": {call: true, args: []argument{fieldOf("tempName"), constantOf("os", "O_WRONLY", "O_CREATE", "O_EXCL"), free}},
	"os.O_WRONLY":      {},
	"os.O_CREATE":      {},
	"os.O_EXCL":        {},
	// The one rename: a temporary file over this machine's file.
	"os.Root.Rename": {call: true, args: []argument{fieldOf("tempName"), fieldOf("machineFileName")}},
	// The one removal: a temporary file.
	"os.Root.Remove": {call: true, args: []argument{fieldOf("tempName")}},
	// Writing the temporary file it opened, and syncing it and the folders.
	"os.File.Write": {call: true, args: []argument{free}},
	"os.File.Sync":  {call: true},
	// Windows flushes a folder only through a handle that may write, which os
	// does not open, so the flush reopens it by the path of the handle os.Root
	// opened. It may create nothing: the open is of a folder that exists.
	"os.File.Fd":                       {file: flushFile, call: true},
	windowsSystemCalls + ".CreateFile": {file: flushFile, call: true, args: []argument{free, free, free, free, constantOf(windowsSystemCalls, "OPEN_EXISTING"), free, free}},
}

func init() {
	for _, name := range []string{
		"ByHandleFileInformation", "CloseHandle", "ERROR_FILENAME_EXCED_RANGE", "FILE_FLAG_BACKUP_SEMANTICS",
		"FILE_FLAG_OPEN_REPARSE_POINT", "FILE_SHARE_DELETE", "FILE_SHARE_READ", "FILE_SHARE_WRITE", "FlushFileBuffers",
		"GENERIC_WRITE", "GetFileInformationByHandle", "GetFinalPathNameByHandle", "Handle", "MAX_LONG_PATH",
		"OPEN_EXISTING", "UTF16ToString",
	} {
		machineFilePermits[windowsSystemCalls+"."+name] = permit{file: flushFile}
	}
}

// permit is what every use of a name a writer is permitted must be.
type permit struct {
	// in is the one function the name may be used in, and file the one file;
	// empty is anywhere.
	in, file string
	// call says every use is a call, each argument held to the rule at its
	// place in args, so a use as a value, whose arguments are not read, is
	// refused.
	call bool
	args []argument
}

// argument is what one argument of a permitted call must be.
type argument struct {
	says  string
	holds func(u *unit, e ast.Expr) bool
}

// free is an argument a permit leaves to the caller.
var free = argument{}

// fieldOf is an argument that is the field of a value of the writer's type
// typeName, read from the value: a type only its makers make, so a value of it
// holds what one of them put there and nothing else.
func fieldOf(typeName string) argument {
	return argument{says: "the field of a " + typeName, holds: func(u *unit, e ast.Expr) bool {
		sel, isSelector := ast.Unparen(e).(*ast.SelectorExpr)
		if !isSelector {
			return false
		}
		selection, ok := u.info.Selections[sel]
		if !ok || selection.Kind() != types.FieldVal {
			return false
		}
		object := u.pkg.Scope().Lookup(typeName)
		if object == nil {
			return false
		}
		structure, isStruct := object.Type().Underlying().(*types.Struct)
		field, isField := selection.Obj().(*types.Var)
		return isStruct && isField && slices.Contains(slices.Collect(structure.Fields()), field)
	}}
}

// oneOf is an argument that is a constant string, one of values.
func oneOf(values ...string) argument {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	return argument{says: "a constant, one of " + strings.Join(quoted, ", "), holds: func(u *unit, e ast.Expr) bool {
		value := u.info.Types[e].Value
		return value != nil && value.Kind() == constant.String && slices.Contains(values, constant.StringVal(value))
	}}
}

// constantOf is an argument that is a constant equal to the named constants of
// the package at path, or'd together: what they are on the platform the code is
// read for, since flags differ between them.
func constantOf(path string, names ...string) argument {
	return argument{says: "a constant that is exactly " + strings.Join(names, "|"), holds: func(u *unit, e ast.Expr) bool {
		value := u.info.Types[e].Value
		if value == nil || value.Kind() != constant.Int {
			return false
		}
		i := slices.IndexFunc(u.pkg.Imports(), func(p *types.Package) bool { return p.Path() == path })
		if i < 0 {
			return false
		}
		want := constant.MakeInt64(0)
		for _, name := range names {
			named, isConst := u.pkg.Imports()[i].Scope().Lookup(name).(*types.Const)
			if !isConst {
				return false
			}
			want = constant.BinaryOp(want, token.OR, named.Val())
		}
		return constant.Compare(value, token.EQL, want)
	}}
}

// useSites is where each use of a name in a unit stands: the call it is the
// function of, if any, and the declaration of the function it is in.
type useSites struct {
	callOf map[*ast.Ident]*ast.CallExpr
	funcOf map[*ast.Ident]*ast.FuncDecl
}

// sitesOf reads useSites from a unit whose writer has permits, and is nil for
// any other.
func sitesOf(u *unit) *useSites {
	if writers[u.path].permits == nil || u.info == nil {
		return nil
	}
	at := &useSites{callOf: map[*ast.Ident]*ast.CallExpr{}, funcOf: map[*ast.Ident]*ast.FuncDecl{}}
	for _, file := range u.files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			ast.Inspect(decl, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.Ident:
					if isFunc {
						at.funcOf[n] = fn
					}
				case *ast.CallExpr:
					switch fun := n.Fun.(type) {
					case *ast.SelectorExpr:
						at.callOf[fun.Sel] = n
					case *ast.Ident:
						at.callOf[fun] = n
					}
				}
				return true
			})
		}
	}
	return at
}

// permitKey is how a permit that was used is noted, apart from readOnly's keys.
func permitKey(writer, key string) string { return writer + " " + key }

// refuses says what a use of a permitted name breaks of its permit, or that it
// breaks nothing.
func (p permit) refuses(u *unit, key string, id *ast.Ident, at *useSites) (string, bool) {
	refuse := func(why string) (string, bool) { return key + ", which " + why, true }
	if p.file != "" && filepath.Base(u.fset.Position(id.Pos()).Filename) != p.file {
		return refuse("this writer may name only in " + p.file)
	}
	if p.in != "" {
		if fn := at.funcOf[id]; fn == nil || fn.Recv != nil || fn.Name.Name != p.in {
			return refuse("this writer may use only in " + p.in)
		}
	}
	if !p.call {
		return "", false
	}
	call := at.callOf[id]
	if call == nil {
		return refuse("is used here other than called, so what it would be called with is not read")
	}
	if sel, isSelector := call.Fun.(*ast.SelectorExpr); isSelector {
		if selection, isMethod := u.info.Selections[sel]; isMethod && selection.Kind() != types.MethodVal {
			return refuse("is called here as a method expression, so its arguments are not the ones its permit reads")
		}
	}
	if call.Ellipsis.IsValid() || len(call.Args) != len(p.args) {
		return refuse("is called here with other arguments than the ones its permit reads")
	}
	for i, rule := range p.args {
		if rule.holds != nil && !rule.holds(u, call.Args[i]) {
			return refuse("is called here with argument " + strconv.Itoa(i+1) + " other than " + rule.says)
		}
	}
	return "", false
}

// writerFixture is a package written as machinefile, or as a package that uses
// it, for the guard to read, with what it must say: the files of the package
// by name, so a permit held to one file can be shown both in it and out of it.
type writerFixture struct {
	name string
	// as is the import path the fixture stands in for; machinefile when empty.
	as string
	// prelude says the package starts from machineFilePrelude, edited by edit.
	prelude bool
	edit    [2]string
	files   map[string]string
	stubs   map[string]string
	// want and noCompile are what fixture's are.
	want      []string
	noCompile string
}

// machineFilePrelude is machinefile in small, written the way the guard allows:
// a Folder only Open makes, the two name types and their makers, and a write
// that makes the reader folder, a temporary file in it, and renames it over
// this machine's file. Each fixture of machinefile adds one thing to it.
const machineFilePrelude = `package machinefile

import (
	"errors"
	"os"
)

type Folder struct{ vault *os.Root }

type (
	tempName        struct{ temp string }
	machineFileName struct{ file string }
)

func Open(path string) (*Folder, error) {
	root, err := os.OpenRoot(path)
	return &Folder{vault: root}, err
}

func machineFile(id string) machineFileName { return machineFileName{file: id + ".jsonl"} }

func newTemp(marks *os.Root, data []byte) (tempName, error) {
	t := tempName{temp: ".tmp-0"}
	f, err := marks.OpenFile(t.temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return tempName{}, err
	}
	_, err = f.Write(data)
	return t, errors.Join(err, f.Sync(), f.Close())
}

func staleTemps(names []string) []tempName {
	var stale []tempName
	for _, name := range names {
		stale = append(stale, tempName{temp: name})
	}
	return stale
}

func (f *Folder) ReplaceMachineFile(id string, data []byte) error {
	if err := f.vault.Mkdir("reader-notes", 0o755); err != nil {
		return err
	}
	notes, err := f.vault.OpenRoot("reader-notes")
	if err != nil {
		return err
	}
	t, err := newTemp(notes, data)
	if err != nil {
		return err
	}
	if err := notes.Rename(t.temp, machineFile(id).file); err != nil {
		return errors.Join(err, notes.Remove(t.temp))
	}
	for _, old := range staleTemps(nil) {
		_ = notes.Remove(old.temp)
	}
	return nil
}
`

// flushStub is the part of golang.org/x/sys/windows the flush names, which the
// platform the fixtures are checked against does not build.
const flushStub = `package windows
type Handle uintptr
type SecurityAttributes struct{}
const (
	GENERIC_WRITE = 0x40000000
	FILE_SHARE_READ = 1
	OPEN_EXISTING = 3
	CREATE_ALWAYS = 2
	FILE_FLAG_BACKUP_SEMANTICS = 0x02000000
)
func CreateFile(name *uint16, access uint32, mode uint32, sa *SecurityAttributes, createmode uint32, attrs uint32, templatefile Handle) (Handle, error) { return 0, nil }
func FlushFileBuffers(h Handle) error { return nil }
func CloseHandle(h Handle) error { return nil }`

// one is a fixture file of machinefile with a body that needs os at most.
func one(body string) map[string]string {
	imports := ""
	if strings.Contains(body, "os.") {
		imports = "import \"os\"\n\n"
	}
	return map[string]string{"fixture.go": "package machinefile\n\n" + imports + body + "\n"}
}

var writerFixtures = []writerFixture{
	{name: "machinefile as it is meant to be written", prelude: true},

	// What the permits refuse: every name but the temporary file opened to write
	// or removed, every rename but the one, and every folder but the three.
	{
		name: "a temporary file renamed over a reader's note", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName, note string) error { return marks.Rename(t.temp, note) }`),
		want:  []string{"os.Root.Rename, which is called here with argument 2 other than the field of a machineFileName"},
	},
	{
		name: "this machine's file renamed over a temporary name", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName) error { return marks.Rename(machineFile("1f3a9c07").file, t.temp) }`),
		want:  []string{"os.Root.Rename, which is called here with argument 1 other than the field of a tempName"},
	},
	{
		name: "a rename of plain strings", prelude: true,
		files: one(`func bad(marks *os.Root, from, to string) error { return marks.Rename(from, to) }`),
		want:  []string{"os.Root.Rename, which is called here with argument 1"},
	},
	{
		name: "this machine's file removed", prelude: true,
		files: one(`func bad(marks *os.Root) error { return marks.Remove(machineFile("1f3a9c07").file) }`),
		want:  []string{"os.Root.Remove, which is called here with argument 1 other than the field of a tempName"},
	},
	{
		name: "a reader's note removed", prelude: true,
		files: one(`func bad(marks *os.Root, note string) error { return marks.Remove(note) }`),
		want:  []string{"os.Root.Remove, which is called here with argument 1"},
	},
	{
		name: "a reader's note opened to write and truncated", prelude: true,
		files: one(`func bad(marks *os.Root, note string) { f, _ := marks.OpenFile(note, os.O_WRONLY|os.O_TRUNC, 0o644); _ = f }`),
		want:  []string{"os.Root.OpenFile, which is called here with argument 1", "os.O_TRUNC"},
	},
	{
		name: "a temporary file opened with a flag that truncates", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName) { f, _ := marks.OpenFile(t.temp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644); _ = f }`),
		want:  []string{"os.Root.OpenFile, which is called here with argument 2 other than a constant that is exactly O_WRONLY|O_CREATE|O_EXCL", "os.O_TRUNC"},
	},
	{
		name: "a temporary file opened with flags that are not a constant", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName) { flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL; f, _ := marks.OpenFile(t.temp, flags, 0o644); _ = f }`),
		want:  []string{"os.Root.OpenFile, which is called here with argument 2"},
	},
	{
		name: "a reader's note made by a link", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName, note string) error { return marks.Link(t.temp, note) }`),
		want:  []string{"os.Root.Link"},
	},
	{
		name: "RemoveAll through the vault", prelude: true,
		files: one(`func (f *Folder) bad() error { return f.vault.RemoveAll("reader-notes") }`),
		want:  []string{"os.Root.RemoveAll"},
	},
	{
		name: "the other ways a root writes", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName) {
	_ = marks.WriteFile(t.temp, nil, 0o644)
	_ = marks.MkdirAll("marks", 0o755)
	_, _ = marks.Create(t.temp)
	_ = marks.Chmod(t.temp, 0o600)
	_ = marks.Symlink(t.temp, "x")
}`),
		want: []string{"os.Root.WriteFile", "os.Root.MkdirAll", "os.Root.Create", "os.Root.Chmod", "os.Root.Symlink"},
	},
	{
		name: "a path renamed by os", prelude: true,
		files: one(`func bad(from, to string) error { return os.Rename(from, to) }`),
		want:  []string{"os.Rename, which is not one this guard knows"},
	},
	{
		name: "a folder other than the three entered", prelude: true,
		files: one(`func (f *Folder) bad() (*os.Root, error) { return f.vault.OpenRoot("courses") }`),
		want:  []string{"os.Root.OpenRoot, which is called here with argument 1 other than a constant, one of"},
	},
	{
		name: "a folder entered by a name that is not a constant", prelude: true,
		files: one(`func (f *Folder) bad(name string) (*os.Root, error) { return f.vault.OpenRoot(name) }`),
		want:  []string{"os.Root.OpenRoot, which is called here with argument 1"},
	},
	{
		name: "a folder other than the three made", prelude: true,
		files: one(`func (f *Folder) bad() error { return f.vault.Mkdir("courses/new", 0o755) }`),
		want:  []string{"os.Root.Mkdir, which is called here with argument 1"},
	},
	{
		name: "a vault opened by path somewhere other than Open", prelude: true,
		files: one(`func elsewhere(path string) *os.Root { root, _ := os.OpenRoot(path); return root }`),
		want:  []string{"os.OpenRoot, which this writer may use only in Open"},
	},
	{
		name: "the writes of a file taken as values", prelude: true,
		files: one(`func bad(f *os.File) { w, s := f.Write, f.Sync; _, _ = w, s }`),
		want:  []string{"os.File.Write, which is used here other than called", "os.File.Sync, which is used here other than called"},
	},

	// The five ways round a rule that judged only a call spelled out, each
	// refused by judging every use.
	{
		name: "a method value of Rename", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName, note string) error { rename := marks.Rename; return rename(t.temp, note) }`),
		want:  []string{"os.Root.Rename, which is used here other than called"},
	},
	{
		name: "Rename called as a method expression", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName) error { return (*os.Root).Rename(marks, t.temp, machineFile("1f3a9c07").file) }`),
		want:  []string{"os.Root.Rename, which is called here as a method expression"},
	},
	{
		name: "Rename reached through an interface", prelude: true,
		files: one(`type renamer interface{ Rename(from, to string) error }
func bad(marks *os.Root, t tempName, note string) error { var r renamer = marks; return r.Rename(t.temp, note) }`),
		want: []string{"through an interface to Rename"},
	},
	{
		name: "a closure in a package variable", prelude: true,
		files: one(`var install = func(marks *os.Root, t tempName, m machineFileName) error { return marks.Rename(t.temp, m.file) }
func bad(marks *os.Root, t tempName) error { return install(marks, t, machineFile("1f3a9c07")) }`),
		want: []string{"install is a variable that holds a function"},
	},
	{
		name: "a plain string made a machineFileName, then renamed over", prelude: true,
		files: one(`func bad(marks *os.Root, t tempName, note string) error { return marks.Rename(t.temp, machineFileName{file: note}.file) }`),
		want:  []string{"a machineFileName is made here, and only machineFile make one"},
	},
	{
		name: "a plain string made a tempName, then removed", prelude: true,
		files: one(`func bad(marks *os.Root, note string) error { return marks.Remove(tempName{temp: note}.temp) }`),
		want:  []string{"a tempName is made here, and only newTemp and staleTemps make one"},
	},

	// The types that carry the rule stay what they say.
	{
		name: "a tempName aimed at a reader's note", prelude: true,
		files: one(`func bad(t *tempName, note string) { t.temp = note }`),
		want:  []string{"temp, a field of tempName, is set here"},
	},
	{
		name: "a machineFileName converted from a type of the same shape", prelude: true,
		files: one(`type raw struct{ file string }
func bad(marks *os.Root, t tempName, r raw) error { return marks.Rename(t.temp, machineFileName(r).file) }`),
		want: []string{"a struct with the fields of machineFileName", "a value is converted to a machineFileName here"},
	},
	{
		name: "a Folder with an exported field", prelude: true,
		edit: [2]string{"type Folder struct{ vault *os.Root }", "type Folder struct{ vault, Vault *os.Root }"},
		want: []string{"Folder has an exported field, Vault"},
	},
	{
		name: "a second way to make a Folder", prelude: true,
		files: one(`func Elsewhere(root *os.Root) *Folder { return &Folder{vault: root} }`),
		want:  []string{"Elsewhere returns a Folder", "a Folder is made here"},
	},
	{
		name: "an exported function that removes and takes no Folder", prelude: true,
		files: one(`func Clean(marks *os.Root, names []string) error { return marks.Remove(staleTemps(names)[0].temp) }`),
		want:  []string{"Clean changes a file, or a process, and takes no Folder"},
	},

	// The Windows flush, in its file and out of it.
	{
		name: "the Windows flush, in the one file that may name it", prelude: true,
		stubs: map[string]string{windowsSystemCalls: flushStub},
		files: map[string]string{flushFile: `package machinefile
import (
	"errors"
	"golang.org/x/sys/windows"
)
func flush(path *uint16) error {
	h, err := windows.CreateFile(path, windows.GENERIC_WRITE, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	return errors.Join(windows.FlushFileBuffers(h), windows.CloseHandle(h))
}`},
	},
	{
		name: "the Windows names in another file", prelude: true,
		stubs: map[string]string{windowsSystemCalls: flushStub},
		files: map[string]string{"fixture.go": `package machinefile
import "golang.org/x/sys/windows"
func flush(h windows.Handle) error { return windows.FlushFileBuffers(h) }`},
		want: []string{windowsSystemCalls + ".Handle, which this writer may name only in " + flushFile, windowsSystemCalls + ".FlushFileBuffers, which this writer may name only in " + flushFile},
	},
	{
		name: "a file's handle taken outside the file that flushes", prelude: true,
		files: one(`func bad(f *os.File) uintptr { return f.Fd() }`),
		want:  []string{"os.File.Fd, which this writer may name only in " + flushFile},
	},
	{
		name: "a file made by path in the file that flushes", prelude: true,
		stubs: map[string]string{windowsSystemCalls: flushStub},
		files: map[string]string{flushFile: `package machinefile
import "golang.org/x/sys/windows"
func make(path *uint16) (windows.Handle, error) {
	return windows.CreateFile(path, windows.GENERIC_WRITE, 0, nil, windows.CREATE_ALWAYS, 0, 0)
}`},
		want: []string{windowsSystemCalls + ".CreateFile, which is called here with argument 5 other than a constant that is exactly OPEN_EXISTING", windowsSystemCalls + ".CREATE_ALWAYS"},
	},

	// What the compiler is left to say: outside machinefile its names cannot be
	// made at all.
	{
		name: "this machine's file named outside the package", as: elsewhere,
		files:     map[string]string{"fixture.go": "package fixture\nimport \"" + machineFileWriter + "\"\nvar name = machinefile.machineFileName{}"},
		noCompile: "machineFileName",
	},
	{
		name: "a Folder filled in outside the package", as: elsewhere,
		files:     map[string]string{"fixture.go": "package fixture\nimport (\n\t\"os\"\n\t\"" + machineFileWriter + "\"\n)\nvar folder = machinefile.Folder{vault: (*os.Root)(nil)}"},
		noCompile: "vault",
	},
}

// read type-checks the fixture against the world's packages.
func (f *writerFixture) read(w *world) (*unit, error) {
	path := cmp.Or(f.as, machineFileWriter)
	sources := maps.Clone(f.files)
	if sources == nil {
		sources = map[string]string{}
	}
	if f.prelude {
		prelude := machineFilePrelude
		if f.edit[0] != "" {
			if !strings.Contains(prelude, f.edit[0]) {
				return nil, errors.New("the prelude holds no " + strconv.Quote(f.edit[0]))
			}
			prelude = strings.Replace(prelude, f.edit[0], f.edit[1], 1)
		}
		sources["machinefile.go"] = prelude
	}
	fset := token.NewFileSet()
	imp := &fixtureImporter{world: w, fset: fset, stubs: f.stubs, made: map[string]*types.Package{}}
	var files []*ast.File
	for _, name := range slices.Sorted(maps.Keys(sources)) {
		file, err := parser.ParseFile(fset, name, sources[name], parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	var first error
	config := types.Config{Importer: imp, Error: func(err error) {
		if first == nil {
			first = err
		}
	}}
	pkg, err := config.Check(path, fset, files, info)
	if first == nil {
		first = err
	}
	if first != nil {
		return nil, first
	}
	return &unit{path: path, fset: fset, files: files, info: info, pkg: pkg}, nil
}

// TestTheMachineFileIsHeldToItsPairs reads each fixture of machinefile and
// holds what the guard says to what it should: the package as it is meant to be
// written says nothing, and each way a change could write a name other than
// this machine's file in the reader folder is refused, and refused as itself.
func TestTheMachineFileIsHeldToItsPairs(t *testing.T) {
	t.Parallel()
	w, err := loadWorld(fixtureTarget)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range writerFixtures {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			u, err := f.read(w)
			if f.noCompile != "" {
				if err == nil {
					t.Fatalf("the fixture compiled, and it has to be something that cannot be written")
				}
				if !strings.Contains(err.Error(), f.noCompile) {
					t.Fatalf("the compiler said %q, which does not mention %q", err, f.noCompile)
				}
				return
			}
			if err != nil {
				t.Fatalf("the fixture does not read: %v", err)
			}
			var said []string
			for _, found := range refusals(u, w) {
				said = append(said, found.text)
			}
			if len(said) != len(f.want) {
				t.Fatalf("the guard said %d things, want %d\n%s", len(said), len(f.want), strings.Join(said, "\n"))
			}
			for _, want := range f.want {
				if !anyContains(said, want) {
					t.Errorf("no refusal mentions %q; the guard said:\n%s", want, strings.Join(said, "\n"))
				}
			}
		})
	}
}

// Every permit that holds a use to something is shown refusing a use that is
// not, so a rule cannot stop being applied, and its fixture cannot be dropped,
// without this going red. The Windows names share one rule, the file they may
// be named in, and one fixture shows it.
func TestEveryPermitOfTheMachineFileIsShownRefusing(t *testing.T) {
	t.Parallel()
	var said []string
	seen := map[string]bool{}
	for _, f := range writerFixtures {
		if seen[f.name] {
			t.Errorf("two fixtures are called %q", f.name)
		}
		seen[f.name] = true
		said = append(said, f.want...)
	}
	for _, key := range slices.Sorted(maps.Keys(machineFilePermits)) {
		p := machineFilePermits[key]
		if !p.call && p.in == "" && p.file == "" {
			continue
		}
		if p.file != "" && !p.call {
			key = windowsSystemCalls + ".Handle"
		}
		if !anyContains(said, key+", which") {
			t.Errorf("no fixture shows a use of %s refused by its permit", key)
		}
	}
}
