package archlock

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// fixture is a package written for this guard to read, with what it must say. A
// fixture is type-checked against the same packages the tree is, so it says what
// a use of a name resolves to and not what it looks like: that is what makes
// each of these a shape the guard cannot be talked out of by a spelling.
type fixture struct {
	name string
	// as is the import path the fixture stands in for; it decides whether it is
	// the writer, and where an interface is reached.
	as  string
	src string
	// stubs are packages the tree does not build and so cannot be loaded, written
	// in the shape the guard judges by their path.
	stubs map[string]string
	// parsed is a fixture that cannot be type-checked at all, such as one that
	// imports C, and is read for its syntax alone.
	parsed bool
	// want is a part of what each refusal says, one for each, in any order. Empty
	// means the guard must say nothing.
	want []string
	// noCompile is a part of what the compiler must answer, for a fixture that
	// asserts something cannot be written at all.
	noCompile string
}

const elsewhere = module + "/cmd/fixture"

// fixtureImporter lets a fixture name any package the tree is built from, and
// the stubs it brings.
type fixtureImporter struct {
	world *world
	fset  *token.FileSet
	stubs map[string]string
	made  map[string]*types.Package
}

func (i *fixtureImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := i.made[path]; ok {
		return pkg, nil
	}
	if src, ok := i.stubs[path]; ok {
		pkg, _, _, err := i.check(path, path+".go", src)
		if err != nil {
			return nil, err
		}
		i.made[path] = pkg
		return pkg, nil
	}
	if path == "unsafe" {
		return importer.Default().Import(path)
	}
	return i.world.Import(path)
}

func (i *fixtureImporter) check(path, name, src string) (*types.Package, []*ast.File, *types.Info, error) {
	file, err := parser.ParseFile(i.fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	var first error
	config := types.Config{Importer: i, Error: func(err error) {
		if first == nil {
			first = err
		}
	}}
	pkg, err := config.Check(path, i.fset, []*ast.File{file}, info)
	if first == nil {
		first = err
	}
	return pkg, []*ast.File{file}, info, first
}

// read turns a fixture into the unit the guard reads, or says why it could not.
func (f *fixture) read(w *world) (*unit, error) {
	path := f.as
	if path == "" {
		path = elsewhere
	}
	fset := token.NewFileSet()
	if f.parsed {
		file, err := parser.ParseFile(fset, "fixture.go", f.src, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		return &unit{path: path, fset: fset, files: []*ast.File{file}}, nil
	}
	imp := &fixtureImporter{world: w, fset: fset, stubs: f.stubs, made: map[string]*types.Package{}}
	pkg, files, info, err := imp.check(path, "fixture.go", f.src)
	if err != nil {
		return nil, err
	}
	return &unit{path: path, fset: fset, files: files, info: info, pkg: pkg}, nil
}

// refusals is everything the guard says about a fixture read in w.
func refusals(u *unit, w *world) []site {
	if u.info == nil {
		return append(forbidden(u), startsAProgram(u, w.starters)...)
	}
	found, _, _ := judge(u)
	found = append(found, shape(u)...)
	found = append(found, dirIsOpaque(u)...)
	found = append(found, startsAProgram(u, w.starters)...)
	return append(found, forbidden(u)...)
}

var writeGuardFixtures = []fixture{
	// What changes a file, spelled every way.
	{
		name: "a function of os that writes",
		src: `package fixture
import "os"
func save(name string, data []byte) error { return os.WriteFile(name, data, 0o600) }`,
		want: []string{"os.WriteFile"},
	},
	{
		name: "the same, under a name the import was given",
		src: `package fixture
import operating "os"
func save(name string) error { return operating.Remove(name) }`,
		want: []string{"os.Remove"},
	},
	{
		name: "a method of a root",
		src: `package fixture
import "os"
func save(root *os.Root, name string) error { return root.Rename(name, name+".old") }`,
		want: []string{"os.Root.Rename"},
	},
	{
		name: "OpenFile, whatever it is opened for",
		src: `package fixture
import "os"
func read(root *os.Root, name string, flags int) { _, _ = root.OpenFile(name, flags, 0) }`,
		want: []string{"os.Root.OpenFile"},
	},
	{
		name: "a root held in a variable of the package, named store",
		src: `package fixture
import "os"
var store *os.Root
func wipe(name string) error { return store.Remove(name) }`,
		want: []string{"os.Root.Remove"},
	},
	{
		name: "a function of os held as a value",
		src: `package fixture
import "os"
var save = os.WriteFile
func keep(with func(string) error) error { return with("x") }
func run() error { return keep(os.Remove) }`,
		want: []string{"os.WriteFile", "os.Remove"},
	},
	{
		name: "a method taken as a value, and as an expression",
		src: `package fixture
import "os"
func wipe(root *os.Root) error {
	remove := root.Remove
	again := (*os.Root).Remove
	_ = again
	return remove("x")
}`,
		want: []string{"os.Root.Remove", "os.Root.Remove"},
	},
	{
		name: "a method promoted through embedding",
		src: `package fixture
import "os"
type wrapper struct{ *os.Root }
func wipe(w wrapper) error { return w.Remove("x") }`,
		want: []string{"os.Root.Remove"},
	},
	{
		name: "a package of another path under the name os",
		src: `package fixture
import os "io/ioutil"
func save() error { return os.WriteFile("a", nil, 0o600) }`,
		want: []string{"io/ioutil.WriteFile"},
	},
	{
		name: "the temporary files of ioutil",
		src: `package fixture
import "io/ioutil"
func scratch() { _, _ = ioutil.TempFile("", "x"); _, _ = ioutil.TempDir("", "x") }`,
		want: []string{"io/ioutil.TempFile", "io/ioutil.TempDir"},
	},
	{
		name: "the other ways os makes and changes files",
		src: `package fixture
import "os"
func scratch(name string) {
	_, _ = os.CreateTemp("", "x")
	_, _ = os.MkdirTemp("", "x")
	_ = os.Chmod(name, 0o600)
	_ = os.Symlink(name, name)
	_ = os.Truncate(name, 0)
	_ = os.Chdir(name)
	_ = os.Setenv("X", "y")
}`,
		want: []string{"os.CreateTemp", "os.MkdirTemp", "os.Chmod", "os.Symlink", "os.Truncate", "os.Chdir", "os.Setenv"},
	},
	{
		name: "a handle that writes, made from the process's own streams",
		src: `package fixture
import "os"
func quiet() { _, _ = os.Stdout.WriteString("x"); _ = os.Stdout.Truncate(0) }`,
		want: []string{"os.File.WriteString", "os.File.Truncate"},
	},
	{
		name: "a program run",
		src: `package fixture
import "os/exec"
func run() error { return exec.Command("rm", "x").Run() }`,
		want: []string{"os/exec.Command", "os/exec.Cmd.Run"},
	},
	{
		name: "the system calls",
		src: `package fixture
import "syscall"
func raw(name string) {
	_, _ = syscall.Open(name, 0, 0)
	_ = syscall.Unlink(name)
}`,
		want: []string{"syscall.Open", "syscall.Unlink"},
	},
	{
		name: "a plugin",
		src: `package fixture
import "plugin"
func load() { _, _ = plugin.Open("x.so") }`,
		want: []string{"plugin.Open"},
	},
	{
		name: "the raw wrappers of x/sys, a call and a constant",
		src: `package fixture
import "golang.org/x/sys/unix"
func swap(from, to string) error { return unix.Renameat2(0, from, 0, to, unix.RENAME_EXCHANGE) }`,
		stubs: map[string]string{
			"golang.org/x/sys/unix": `package unix
const RENAME_EXCHANGE = 2
func Renameat2(a int, b string, c int, d string, e uint) error { return nil }`,
		},
		want: []string{"golang.org/x/sys/unix.Renameat2", "golang.org/x/sys/unix.RENAME_EXCHANGE"},
	},
	{
		name: "a socket that is a file",
		src: `package fixture
import "net"
func listen() { _, _ = net.ListenUnix("unix", &net.UnixAddr{Name: "/x", Net: "unix"}) }`,
		want: []string{"net.ListenUnix"},
	},
	{
		name: "a form that is kept in files",
		src: `package fixture
import "net/http"
func parse(r *http.Request) error { return r.ParseMultipartForm(1 << 20) }`,
		want: []string{"net/http.Request.ParseMultipartForm"},
	},

	// What a type cannot say. A call through an interface is judged by the name
	// of the method, since what answers it is not known here.
	{
		name: "an interface of this module's own with a method that writes",
		as:   module + "/internal/mark/fixture",
		src: `package fixture
type remover interface{ Remove(string) error }
func wipe(r remover) error { return r.Remove("x") }`,
		want: []string{"through an interface to Remove"},
	},
	{
		name: "a type parameter whose constraint says Remove",
		src: `package fixture
func wipe[T interface{ Remove(string) error }](x T) error { return x.Remove("y") }`,
		want: []string{"through an interface to Remove"},
	},
	{
		name: "a generic type holding something that removes",
		src: `package fixture
type Handle[T interface{ Remove(string) error }] struct{ held T }
func (h Handle[T]) wipe() error { return h.held.Remove("y") }`,
		want: []string{"through an interface to Remove"},
	},
	{
		name: "every name a write answers to, through an interface",
		src: `package fixture
type handle interface {
	Chmod(); Chown(); Chtimes(); Create(); Lchown(); Link(); Mkdir()
	OpenFile(); Remove(); Rename(); Symlink(); Truncate(); WriteFile()
}
func use(h handle) {
	h.Chmod(); h.Chown(); h.Chtimes(); h.Create(); h.Lchown(); h.Link(); h.Mkdir()
	h.OpenFile(); h.Remove(); h.Rename(); h.Symlink(); h.Truncate(); h.WriteFile()
}`,
		want: []string{
			"through an interface to Chmod", "through an interface to Chown", "through an interface to Chtimes",
			"through an interface to Create", "through an interface to Lchown", "through an interface to Link",
			"through an interface to Mkdir", "through an interface to OpenFile", "through an interface to Remove",
			"through an interface to Rename", "through an interface to Symlink", "through an interface to Truncate",
			"through an interface to WriteFile",
		},
	},
	{
		name: "an interface method that creates",
		src: `package fixture
type maker interface{ Mkdir(string, uint32) error }
func make(m maker) error { return m.Mkdir("x", 0o700) }`,
		want: []string{"through an interface to Mkdir"},
	},

	// A package of the standard library that starts a program for its caller:
	// the program is what writes, and nothing it does is a name in the source.
	{
		name: "a program started through the CGI handler",
		src: `package fixture
import "net/http/cgi"
var handler = &cgi.Handler{Path: "/bin/sh"}`,
		want: []string{"imports net/http/cgi, which starts a program through os/exec"},
	},
	{
		name: "a program started by the package that finds packages",
		src: `package fixture
import "go/build"
func find() { _, _ = build.Import("x", ".", 0) }`,
		want: []string{"imports go/build, which starts a program through os/exec"},
	},
	{
		name: "a program started by the package that imports export data",
		src: `package fixture
import "go/importer"
var types = importer.Default()`,
		want: []string{"imports go/importer, which starts a program through os/exec"},
	},

	// What reaches the disk past what the types say.
	{
		name:   "cgo",
		parsed: true,
		src: `package fixture
import "C"`,
		want: []string{"imports C"},
	},
	{
		name:   "unsafe, and a linkname that needs it",
		parsed: true,
		src: `package fixture
import _ "unsafe"
//go:linkname write os.write
func write(fd int, p []byte) (int, error)`,
		want: []string{"imports unsafe", "go:linkname"},
	},

	{
		name:   "a method called by a name that is a string, through reflect",
		parsed: true,
		src: `package fixture
import "reflect"
func call(v any) { reflect.ValueOf(v).MethodByName("Remove").Call(nil) }`,
		want: []string{"imports reflect"},
	},
	{
		name:   "a method called by a name in a template",
		parsed: true,
		src: `package fixture
import "text/template"
var t = template.Must(template.New("x").Parse("{{.Remove}}"))`,
		want: []string{"imports text/template"},
	},
	{
		name:   "the same, in the template package that escapes",
		parsed: true,
		src: `package fixture
import "html/template"
var t = template.Must(template.New("x").Parse("{{.Remove}}"))`,
		want: []string{"imports html/template"},
	},

	// The writer's own signatures, which carry the other half of the rule.
	{
		name: "an exported function of the writer that takes a path",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
func Wipe(path string) error { return os.Remove(path) }`,
		want: []string{"Wipe changes a file, or a process, and takes no Dir"},
	},
	{
		name: "the same, a helper away",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
func Wipe(path string) error { return wipe(path) }
func wipe(path string) error { return os.Remove(path) }`,
		want: []string{"Wipe changes a file, or a process, and takes no Dir"},
	},
	{
		name: "an exported function of the writer that makes a directory from a path",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
func Prepare(path string) error { return os.MkdirAll(path, 0o700) }`,
		want: []string{"Prepare changes a file, or a process, and takes no Dir"},
	},

	// The type that carries the rule is what it says it is.
	{
		name: "a Dir that a string converts to",
		as:   rawWriter,
		src: `package confdir
type Dir string`,
		want: []string{"Dir is not a struct"},
	},
	{
		name: "a Dir with a field to fill in",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string; Path string }`,
		want: []string{"Dir has an exported field, Path"},
	},
	{
		name: "a third way to make a Dir, from a path",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
func DirOf(path string) Dir { return Dir{path: path} }`,
		want: []string{"DirOf returns a Dir", "a Dir is made here"},
	},
	{
		name: "a method that moves a Dir somewhere else",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
func (d Dir) Sub(name string) Dir { return Dir{path: d.path + "/" + name} }`,
		want: []string{"Dir.Sub returns a Dir", "a Dir is made here"},
	},
	{
		name: "a writer with no Dir at all",
		as:   rawWriter,
		src:  `package confdir`,
		want: []string{"there is no type Dir"},
	},

	// Only Home and VaultDir make a Dir or set what one holds, so no signature
	// that takes a Dir can be handed one aimed anywhere else.
	{
		name: "a pointer receiver that aims its Dir elsewhere",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
func (d *Dir) Aim(path string) { d.path = path }`,
		want: []string{"path, a field of Dir, is set here"},
	},
	{
		name: "a Dir handed back through a pointer the caller passes",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
func Into(out *Dir, path string) { *out = Dir{path: path} }`,
		want: []string{"a Dir is made here"},
	},
	{
		name: "Dirs made into a slice, and set through one the caller passes",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
var homes = []Dir{{path: "/vault"}}
func Dirs(paths ...string) []Dir {
	var all []Dir
	for _, p := range paths { all = append(all, Dir{path: p}) }
	return all
}
func Fill(all []Dir, path string) { for i := range all { all[i].path = path } }`,
		want: []string{"a Dir is made here", "Dirs returns a Dir", "a Dir is made here", "path, a field of Dir, is set here"},
	},
	{
		name: "an exported method that writes a path it is given",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
type saver struct{}
func (saver) Save(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }`,
		want: []string{"saver.Save changes a file, or a process, and takes no Dir"},
	},
	{
		name: "an exported variable that holds a function that writes",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
var Save = os.WriteFile`,
		want: []string{"Save is a variable that holds a function"},
	},
	{
		name: "a function that writes, held in a variable an exported function calls",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
var save = os.WriteFile
func Save(path string, data []byte) error { return save(path, data, 0o600) }`,
		want: []string{"save is a variable that holds a function"},
	},
	{
		name: "a struct with the fields of Dir, which is assignable to one",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
func of(path string) Dir { return struct{ path string }{path} }`,
		want: []string{"a struct with the fields of Dir"},
	},
	{
		name: "a Dir converted from a type of the same shape",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
type raw struct{ path string }
func aim(r raw) Dir { return Dir(r) }`,
		want: []string{"a struct with the fields of Dir", "converted to a Dir here"},
	},
	{
		name: "a field of Dir set through its address",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
func aim(d *Dir, path string) { p := &d.path; *p = path }`,
		want: []string{"the address of path, a field of Dir"},
	},

	// And what must stay quiet.
	{
		name: "reading",
		src: `package fixture
import "os"
func look(root *os.Root, name string) {
	_, _ = os.Stat(name)
	_, _ = os.ReadFile(name)
	_, _ = os.OpenRoot(name)
	_, _ = root.Open(name)
	_, _ = root.Stat(name)
	_ = os.Args
	_ = os.Getenv("X")
	os.Exit(0)
}`,
	},
	{
		name: "a type of this module that has methods a write is called",
		src: `package fixture
type notes struct{}
func (notes) OpenFile(string) error { return nil }
func (notes) Remove(string) error   { return nil }
func (notes) Rename(string) error   { return nil }
func use(n notes) error { return n.Remove("x") }`,
	},
	{
		name: "a function of this module called what os calls its own",
		src: `package fixture
func Remove(name string) error { return nil }
func WriteFile(name string) error { return nil }
func use() error { _ = WriteFile("x"); return Remove("y") }`,
	},
	{
		name: "a local called os",
		src: `package fixture
func use() {
	os := struct{ WriteFile func() }{WriteFile: func() {}}
	os.WriteFile()
}`,
	},
	{
		name: "taking a vault out of the list, which writes through confdir and not here",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/vaults"
func forget(store *vaults.Store, id string) error { _, err := store.Remove(id); return err }`,
	},
	{
		name: "an interface method that only reads or writes a stream",
		src: `package fixture
import "io"
type reader interface{ Read([]byte) (int, error) }
func copyAll(w io.Writer, r reader) { _, _ = w.Write(nil); _, _ = r.Read(nil) }`,
	},
	{
		name: "the errors os gives and the streams it holds",
		src: `package fixture
import (
	"errors"
	"fmt"
	"os"
)
func missing(err error) bool { fmt.Fprintln(os.Stderr, "x"); return errors.Is(err, os.ErrNotExist) }`,
	},
	{
		name: "the writer working on a Dir",
		as:   rawWriter,
		src: `package confdir
import "os"
type Dir struct{ path string }
func Home(configDir string) (Dir, error) { return Dir{path: configDir}, nil }
func VaultDir(configDir, vaultRoot string) (Dir, error) { return Dir{path: configDir + vaultRoot}, nil }
func (d Dir) Path() string { return d.path }
func Wipe(dir Dir, name string) error { return os.Remove(dir.path + name) }
func (d Dir) Clear(name string) error { return os.Remove(d.path + name) }
type Lock struct{ dir Dir; name string }
func (l *Lock) Release() error { return os.Remove(l.dir.path + l.name) }
func Size(path string) (int64, error) { info, err := os.Stat(path); if err != nil { return 0, err }; return info.Size(), nil }`,
	},

	// What the compiler is left to say: a Dir is made by confdir, from the
	// configuration directory, and by nothing else.
	{
		name: "a Dir from a path by conversion",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/confdir"
var vault = confdir.Dir("/vault")`,
		noCompile: "Dir",
	},
	{
		name: "a Dir from a path by literal",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/confdir"
var vault = confdir.Dir{"/vault"}`,
		noCompile: "Dir",
	},
	{
		name: "a Dir with its field set",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/confdir"
func aim(dir confdir.Dir) confdir.Dir { dir.path = "/vault"; return dir }`,
		noCompile: "path",
	},
	{
		name: "a function of the writer given a path",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/confdir"
func save() error { return confdir.Replace("/vault", "marks.json", nil) }`,
		noCompile: "Dir",
	},
	{
		name: "a Dir from the configuration directory",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/confdir"
func home(configDir string) error { _, err := confdir.Home(configDir); return err }`,
	},

	// A program started through a value that only says it can Run, and the
	// processes of the reader's own.
	{
		name: "a program started through an interface it is stored in",
		src: `package fixture
import "os/exec"
type runner interface{ Run() error }
func run() error { var r runner = &exec.Cmd{Path: "/bin/sh"}; return r.Run() }`,
		want: []string{"os/exec.Cmd, which is not one this guard knows"},
	},
	{
		name: "a process found and signalled outside the writer",
		src: `package fixture
import ("os"; "syscall")
func there(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}`,
		want: []string{"os.FindProcess", "os.Process.Signal", "syscall.Signal"},
	},
	{
		name: "an exported function of the writer that sends a signal other than 0",
		as:   rawWriter,
		src: `package confdir
import ("os"; "syscall")
type Dir struct{ path string }
func Stop(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil { return err }
	return p.Signal(syscall.Signal(9))
}`,
		want: []string{"Stop changes a file, or a process, and takes no Dir"},
	},
	{
		name: "an exported function of the writer that sends a signal it is given",
		as:   rawWriter,
		src: `package confdir
import ("os"; "syscall")
type Dir struct{ path string }
func Stop(pid int, n syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil { return err }
	return p.Signal(n)
}`,
		want: []string{"Stop changes a file, or a process, and takes no Dir"},
	},
	{
		name: "the writer asking whether a process is there, for any caller",
		as:   rawWriter,
		src: `package confdir
import ("errors"; "os"; "runtime"; "syscall")
type Dir struct{ path string }
func ProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil { return false }
	if runtime.GOOS == "windows" { return p.Release() == nil }
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}`,
	},
	{
		name: "a process ended through an interface",
		src: `package fixture
func stop(v any) error { return v.(interface{ Kill() error }).Kill() }`,
		want: []string{"through an interface to Kill"},
	},
	{
		name: "a signal sent through an interface",
		src: `package fixture
func stop(v any) error { return v.(interface{ Signal(int) error }).Signal(9) }`,
		want: []string{"through an interface to Signal"},
	},

	// A listener that makes a file, and the forms that make one of their own.
	{
		name: "a listener on a unix socket",
		src: `package fixture
import "net"
func listen() error { _, err := net.Listen("unix", "/vault/x.sock"); return err }`,
		want: []string{"net.Listen, which is not called with"},
	},
	{
		name: "a packet listener of a listen config on a unix socket",
		src: `package fixture
import ("context"; "net")
func listen(ctx context.Context) error { _, err := (&net.ListenConfig{}).ListenPacket(ctx, "unixgram", "/x"); return err }`,
		want: []string{"net.ListenConfig.ListenPacket"},
	},
	{
		name: "a listener reached through an interface a listen config answers",
		src: `package fixture
import ("context"; "net")
type packets interface{ ListenPacket(context.Context, string, string) (net.PacketConn, error) }
func listen(ctx context.Context, lc *net.ListenConfig) error { _, err := any(lc).(packets).ListenPacket(ctx, "unixgram", "/x"); return err }`,
		want: []string{"through an interface to ListenPacket"},
	},
	{
		name: "a listener whose network is a variable",
		src: `package fixture
import "net"
func listen(network string) error { _, err := net.Listen(network, "x"); return err }`,
		want: []string{"net.Listen, which is not called with"},
	},
	{
		name: "a listener of a listen config on a unix socket",
		src: `package fixture
import ("context"; "net")
func listen(ctx context.Context) error { _, err := (&net.ListenConfig{}).Listen(ctx, "unix", "/x"); return err }`,
		want: []string{"net.ListenConfig.Listen, which is not called with"},
	},
	{
		name: "a packet listener on a unix socket",
		src: `package fixture
import "net"
func listen() error { _, err := net.ListenPacket("unixgram", "/x"); return err }`,
		want: []string{"net.ListenPacket"},
	},
	{
		name: "listeners on a port",
		src: `package fixture
import ("context"; "net")
func listen(ctx context.Context) error {
	if _, err := net.Listen("tcp", "127.0.0.1:0"); err != nil { return err }
	_, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	return err
}`,
	},
	{
		name: "a form read in a way that can write a part to a temporary file",
		src: `package fixture
import "net/http"
func read(r *http.Request) string { return r.PostFormValue("a") + r.FormValue("b") }`,
		want: []string{"net/http.Request.PostFormValue", "net/http.Request.FormValue"},
	},
	{
		name: "a form read after the urlencoded one is parsed",
		src: `package fixture
import "net/http"
func read(r *http.Request) string { _ = r.ParseForm(); return r.PostForm.Get("a") }`,
	},
	{
		name: "the other names that make files of their own",
		src: `package fixture
import ("mime/multipart"; "net/http"; "runtime/coverage"; "runtime/debug")
func spill(r *http.Request, m *multipart.Reader) {
	_, _, _ = r.FormFile("a")
	_, _ = m.ReadForm(1 << 20)
	debug.WriteHeapDump(0)
	_ = coverage.WriteCountersDir("/x")
}`,
		want: []string{"net/http.Request.FormFile", "mime/multipart.Reader.ReadForm", "runtime/debug.WriteHeapDump", "runtime/coverage.WriteCountersDir"},
	},
	{
		name: "coverage data written by the program",
		src: `package fixture
import "runtime/coverage"
func dump() error { return coverage.WriteMetaDir("/x") }`,
		want: []string{"runtime/coverage.WriteMetaDir"},
	},

	// A Dir that something other than a function could hand out.
	{
		name: "an exported variable that holds a Dir",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
var Saved Dir`,
		want: []string{"Saved is a variable that holds a Dir"},
	},
	{
		name: "an exported variable that holds Dirs in a map",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
var Table = map[string]*Dir{}`,
		want: []string{"Table is a variable that holds a Dir"},
	},
	{
		name: "an exported field that holds a Dir",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
type Holder struct{ Where Dir }`,
		want: []string{"Holder.Where holds a Dir"},
	},
	{
		name: "a Dir held where only the package can reach it",
		as:   rawWriter,
		src: `package confdir
type Dir struct{ path string }
var saved Dir
type Lock struct{ dir Dir }
func (l *Lock) Path() string { return l.dir.path + saved.path }`,
	},
}

func TestTheWriteGuardCatchesEveryShapeOfWrite(t *testing.T) {
	t.Parallel()
	w, err := loadWorld(fixtureTarget)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range writeGuardFixtures {
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

func anyContains(texts []string, part string) bool {
	for _, text := range texts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

// A fixture that is meant to refuse and names nothing to refuse would pass by
// saying nothing, so every fixture that is not a quiet one has to say something.
// And every row of a list the guard refuses by is shown refused by a fixture, so
// a row cannot stop being refused, and its fixture cannot be dropped, without
// one of these going red.
func TestEveryFixtureOfTheWriteGuardSaysWhatItExpects(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	var said []string
	for _, f := range writeGuardFixtures {
		if seen[f.name] {
			t.Errorf("two fixtures are called %q", f.name)
		}
		seen[f.name] = true
		if f.noCompile != "" && len(f.want) > 0 {
			t.Errorf("%q cannot be compiled and also say what the guard refuses", f.name)
		}
		if strings.TrimSpace(f.src) == "" {
			t.Errorf("%q has no source", f.name)
		}
		said = append(said, f.want...)
	}
	var rows []string
	rows = append(rows, alsoRefused...)
	rows = append(rows, slices.Sorted(maps.Keys(constrainedCalls))...)
	for _, path := range forbiddenImports {
		rows = append(rows, "imports "+path)
	}
	for _, family := range slices.Concat(writeFamilies, processFamilies, listenerFamilies) {
		rows = append(rows, "through an interface to "+family)
	}
	for _, row := range rows {
		if !anyContains(said, row) {
			t.Errorf("no fixture shows %q refused", row)
		}
	}
}

// Whose a package is is read from the module it comes from, so a module that is
// named with no dot, one whose path begins with this module's, and one go.mod
// replaces are none of them this module's own, and so none of them goes unread.
func TestWhoseCodeAPackageIs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		module *packages.Module
		want   owner
	}{
		{"the standard library has no module", nil, standardLibrary},
		{"this module", &packages.Module{Path: module}, ownCode},
		{"a module that begins with this one's path", &packages.Module{Path: module + "-evil"}, anotherModule},
		{"a module of a version of this one", &packages.Module{Path: module + "/v2"}, anotherModule},
		{"a module whose path has no dot", &packages.Module{Path: "zzw", Replace: &packages.Module{Path: "./.zzw"}}, anotherModule},
		{"a module of a host", &packages.Module{Path: "github.com/natefinch/atomic"}, anotherModule},
	} {
		if got := whose(&packages.Package{PkgPath: "x", Module: tt.module}); got != tt.want {
			t.Errorf("%s: whose = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestTheLinkCheckSaysWhatItIsNotToldAbout(t *testing.T) {
	t.Parallel()
	reviewed := reviewedModules[0]
	for _, tt := range []struct {
		name string
		pkg  packages.Package
		want []string
	}{
		{"a package of a reviewed module", packages.Package{Module: &packages.Module{Path: reviewed}}, nil},
		{"a package of this module", packages.Package{Module: &packages.Module{Path: module}}, nil},
		{"a package of a module nobody reviewed", packages.Package{Module: &packages.Module{Path: "github.com/natefinch/atomic"}}, []string{"not one of reviewedModules"}},
		{"a module with no dot in its path, replaced by a folder", packages.Package{Module: &packages.Module{Path: "zzw", Replace: &packages.Module{Path: "./.zzw"}}}, []string{"not one of reviewedModules", "go.mod replaces"}},
		{"a reviewed module that go.mod replaces", packages.Package{Module: &packages.Module{Path: reviewed, Replace: &packages.Module{Path: "../other"}}}, []string{"go.mod replaces"}},
		{"assembly in a package", packages.Package{Module: &packages.Module{Path: module}, OtherFiles: []string{"zz_asm_arm64.s"}}, []string{"not Go"}},
		{"assembly for another platform", packages.Package{Module: &packages.Module{Path: module}, IgnoredFiles: []string{"zz_asm_other.s", "other.go"}}, []string{"not Go"}},
		{"a Go file for another platform", packages.Package{Module: &packages.Module{Path: module}, IgnoredFiles: []string{"other.go"}}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.pkg.PkgPath = "x/y"
			got := linkProblems(&tt.pkg)
			if len(got) != len(tt.want) {
				t.Fatalf("said %d things, want %d: %q", len(got), len(tt.want), got)
			}
			for _, want := range tt.want {
				if !anyContains(got, want) {
					t.Errorf("nothing says %q: %q", want, got)
				}
			}
		})
	}
}

func TestWhatIsNotGoIsFoundWhereverItIs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{
		"go.mod", "main.go", "README.md", ".zzw/w.go",
		"internal/note/zz_asm.s", "internal/note/zz_obj.syso", ".zzw/go.mod", "tools/go.work", "c/helper.h",
		".git/x.s", "node_modules/y/z.c", "vendor/modules.txt", "vendor/github.com/natefinch/atomic/atomic.go",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil { // #nosec G703 -- a file under t.TempDir
			t.Fatal(err)
		}
	}
	got, err := sourcesBeyondGo(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".zzw/go.mod", "c/helper.h", "internal/note/zz_asm.s", "internal/note/zz_obj.syso", "tools/go.work", "vendor"}
	var names []string
	for _, line := range got {
		name, _, _ := strings.Cut(line, ":")
		names = append(names, name)
	}
	if !slices.Equal(names, want) {
		t.Errorf("found %q, want %q", names, want)
	}
}

func TestAReplaceInGoModIsFound(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		text string
		want int
	}{
		{"none", "module x\n\ngo 1.27\n\nrequire y v1.0.0\n", 0},
		{"one line", "module x\n\nreplace zzw => ./.zzw\n", 1},
		{"a block", "module x\n\nreplace (\n\tzzw => ./.zzw\n)\n", 2},
		{"inside a require block", "module x\n\nrequire (\n\ty v1.0.0 // indirect\n)\n", 0},
	} {
		if got := replacesIn(tt.text); len(got) != tt.want {
			t.Errorf("%s: found %q, want %d", tt.name, got, tt.want)
		}
	}
}
