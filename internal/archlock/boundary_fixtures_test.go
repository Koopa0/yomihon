package archlock

import (
	"cmp"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"testing"
)

// fixture is a package written for the walk to read, with what it must say.
// It is type-checked against the packages the tree is, so each row is a shape
// the walk cannot be talked out of by a spelling.
type fixture struct {
	name string
	// as is the import path the fixture stands in for, and file the repository
	// file it is read as.
	as, file string
	src      string
	// stubs are packages the fixture system does not build, in the shape the
	// walk judges by their path.
	stubs map[string]string
	// parsed is read for its syntax alone, since it cannot be type-checked.
	parsed bool
	// want is a part of what each refusal says, one for each, in any order.
	want []string
}

// fixtureImporter lets a fixture import what the world holds, and its stubs.
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
	src, isStub := i.stubs[path]
	if !isStub {
		return i.world.Import(path)
	}
	u, err := i.check(path, path+".go", src)
	if err != nil {
		return nil, err
	}
	i.made[path] = u.pkg
	return u.pkg, nil
}

func (i *fixtureImporter) check(path, name, src string) (*unit, error) {
	file, err := parser.ParseFile(i.fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	pkg, err := (&types.Config{Importer: i}).Check(path, i.fset, []*ast.File{file}, info)
	if err != nil {
		return nil, err
	}
	return &unit{path: path, fset: i.fset, files: []*ast.File{file}, info: info, pkg: pkg}, nil
}

func (f *fixture) read(w *world) (*unit, error) {
	path := cmp.Or(f.as, module+"/internal/fixture")
	file := cmp.Or(f.file, "internal/fixture/fixture.go")
	fset := token.NewFileSet()
	if f.parsed {
		parsed, err := parser.ParseFile(fset, file, f.src, parser.ParseComments)
		return &unit{path: path, fset: fset, files: []*ast.File{parsed}}, err
	}
	return (&fixtureImporter{world: w, fset: fset, stubs: f.stubs, made: map[string]*types.Package{}}).check(path, file, f.src)
}

var boundaryFixtures = []fixture{
	// A write, however it is spelt.
	{
		name: "a write of os under another name",
		src: `package fixture
import operating "os"
func save(name string) error { return operating.Remove(name) }`,
		want: []string{"os.Remove"},
	},
	{
		name: "a method of a root as a value and as an expression",
		src: `package fixture
import "os"
func wipe(root *os.Root) error {
	remove, again := root.Remove, (*os.Root).Remove
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
		name: "a write through an interface and through a type parameter",
		src: `package fixture
type remover interface{ Remove(string) error }
func wipe(r remover) error { return r.Remove("x") }
func wipeAny[T interface{ Remove(string) error }](x T) error { return x.Remove("y") }`,
		want: []string{"through an interface to Remove", "through an interface to Remove"},
	},
	{
		name: "the raw wrappers of x/sys",
		src: `package fixture
import "golang.org/x/sys/unix"
func swap(from, to string) error { return unix.Renameat2(0, from, 0, to, unix.RENAME_EXCHANGE) }`,
		stubs: map[string]string{"golang.org/x/sys/unix": `package unix
const RENAME_EXCHANGE = 2
func Renameat2(a int, b string, c int, d string, e uint) error { return nil }`},
		want: []string{"golang.org/x/sys/unix.Renameat2", "golang.org/x/sys/unix.RENAME_EXCHANGE"},
	},
	{
		name: "a program run, and a plugin loaded",
		src: `package fixture
import ("os/exec"; "plugin")
func run() error { _, _ = plugin.Open("x.so"); return exec.Command("rm", "x").Run() }`,
		want: []string{"plugin.Open", "os/exec.Command", "os/exec.Cmd.Run"},
	},
	{
		name: "a program started by a package of the standard library",
		src: `package fixture
import "net/http/cgi"
var handler = &cgi.Handler{Path: "/bin/sh"}`,
		want: []string{"imports net/http/cgi, which starts a program through os/exec"},
	},
	{
		name: "a form kept in temporary files",
		src: `package fixture
import "net/http"
func parse(r *http.Request) error { return r.ParseMultipartForm(1 << 20) }`,
		want: []string{"net/http.Request.ParseMultipartForm"},
	},
	{
		name:   "unsafe, and a linkname",
		parsed: true,
		src: `package fixture
import _ "unsafe"
//go:linkname write os.write
func write(fd int, p []byte) (int, error)`,
		want: []string{"imports unsafe", "go:linkname"},
	},
	{
		name:   "a method called by a name that is a string",
		parsed: true,
		src: `package fixture
import "reflect"
func call(v any) { reflect.ValueOf(v).MethodByName("Remove").Call(nil) }`,
		want: []string{"imports reflect"},
	},
	{
		name: "machinefile writing by a path",
		as:   machineFileWriter,
		file: "internal/mark/machinefile/fixture.go",
		src: `package machinefile
import "os"
func move(root *os.Root, from, to string) error {
	if err := root.Rename(from, to); err != nil {
		return err
	}
	return os.Rename(from, to)
}`,
		want: []string{"os.Rename"},
	},

	// What a writer is handed.
	{
		name: "an exported function of confdir that removes a path, a helper away",
		as:   rawWriter,
		file: "internal/mark/confdir/fixture.go",
		src: `package confdir
import "os"
type Dir struct{ path string }
func Wipe(path string) error { return wipe(path) }
func wipe(path string) error { return os.Remove(path) }`,
		want: []string{"Wipe changes a file, or a process, and takes no Dir"},
	},
	{
		name: "an exported function of machinefile that removes and takes no Folder",
		as:   machineFileWriter,
		file: "internal/mark/machinefile/fixture.go",
		src: `package machinefile
import "os"
type Folder struct{ root *os.Root }
func (f *Folder) Clear(name string) error { return f.root.Remove(name) }
func Clean(root *os.Root, name string) error { return root.Remove(name) }`,
		want: []string{"Clean changes a file, or a process, and takes no Folder"},
	},
	{
		name: "a signal other than 0",
		as:   rawWriter,
		file: "internal/mark/confdir/fixture.go",
		src: `package confdir
import ("os"; "syscall")
func Stop(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.Signal(9))
}`,
		want: []string{"os.Process.Signal, which is called only with signal 0"},
	},

	// The environment, the network and standard output.
	{
		name: "the environment read where it may not be",
		src: `package fixture
import ("os"; "syscall")
func read() {
	_ = os.Getenv("YOMIHON_PORT")
	_, _ = os.UserConfigDir()
	_, _ = os.UserHomeDir()
	_ = os.Environ()
	_, _ = syscall.Getenv("HOME")
}`,
		want: []string{"os.Getenv, which is read in main alone", "os.UserConfigDir, which", "os.UserHomeDir, which", "os.Environ, which", "syscall.Getenv, which"},
	},
	{
		name: "a variable other than the port, read in main",
		file: mainFile,
		src: `package fixture
import "os"
func home() string { return os.Getenv("HOME") }`,
		want: []string{`os.Getenv, which is called only with "YOMIHON_PORT"`},
	},
	{
		name: "an outbound connection",
		src: `package fixture
import ("context"; "crypto/tls"; "net"; "net/http")
func reach(ctx context.Context) {
	_, _ = http.Get("http://example.invalid")
	_, _ = net.Dial("tcp", "example.invalid:80")
	_, _ = (&net.Dialer{}).DialContext(ctx, "tcp", "example.invalid:80")
	_, _ = tls.Dial("tcp", "example.invalid:443", nil)
}`,
		want: []string{"net/http.Get", "net.Dial,", "net.Dialer,", "net.Dialer.DialContext", "crypto/tls.Dial,"},
	},
	{
		name: "a listener outside main",
		src: `package fixture
import "net"
func listen() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }`,
		want: []string{"net.Listen, which opens a listener"},
	},
	{
		name: "standard output outside main",
		src: `package fixture
import ("fmt"; "os")
func say() { fmt.Fprintln(os.Stdout, "x") }`,
		want: []string{"os.Stdout, which is standard output"},
	},

	// What leaves for an agent.
	{
		name: "what is withheld asked outside the gate",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/schema"
func withheld(p schema.PrivacyPolicy) []string { return p.WithheldDirs() }`,
		want: []string{"schema.PrivacyPolicy.WithheldDirs, which asks what is withheld"},
	},
	{
		name: "a gate opened with an extractor of its own",
		src: `package fixture
import (
	"context"
	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/vault"
)
func open(ctx context.Context, r *vault.Reader, links egress.LinkTargets) { _, _ = egress.Open(ctx, r, links) }`,
		want: []string{"egress.Open, which is called only with judge.LinkTargets"},
	},
	{
		name: "a payload's writer, kept under another name and handed on, printed to as well",
		src: `package fixture
import (
	"fmt"
	"io"
	"github.com/koopa0/yomihon/internal/egress"
)
func run(w io.Writer, p egress.Payload) {
	out := w
	hand(out, p)
	fmt.Fprintln(w, "and a word of its own")
}
func hand(out io.Writer, p egress.Payload) { _, _ = p.WriteTo(out) }`,
		want: []string{"w is written an egress.Payload and is used here for something else"},
	},

	// The source's own words and ways.
	{
		name: "frontmatter decoded outside the vault",
		src: `package fixture
import "go.yaml.in/yaml/v3"
func decode(b []byte) error { var v any; return yaml.Unmarshal(b, &v) }`,
		want: []string{"go.yaml.in/yaml/v3.Unmarshal, which decodes YAML"},
	},
	{
		name: "a configuration directory made at a path in shipped code",
		src: `package fixture
import "github.com/koopa0/yomihon/internal/mark/confdir"
var at = confdir.At("/vault")`,
		want: []string{"confdir.At, which makes a configuration directory at any path"},
	},
	{
		name: "a response body written in the source",
		src: `package fixture
import "net/http"
const sorry = "nothing here"
func h(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "boom: "+r.URL.Path, http.StatusInternalServerError)
	http.Error(w, sorry, http.StatusNotFound)
	http.NotFound(w, r)
}`,
		want: []string{"net/http.Error, which is handed a sentence", "net/http.Error, which is handed a sentence", "net/http.NotFound, which writes the standard library's own sentence"},
	},

	// And what must stay quiet.
	{
		name: "reading, and names that only look like writes",
		src: `package fixture
import ("errors"; "fmt"; "io"; "net"; "net/http"; "os")
type notes struct{}
func (notes) Remove(string) error { return nil }
func look(root *os.Root, name string, w io.Writer, l net.Listener, rw http.ResponseWriter, n notes, text fmt.Stringer) error {
	_, _ = os.Stat(name)
	_, _ = os.ReadFile(name)
	_, _ = root.Open(name)
	_, _ = w.Write(nil)
	_ = l.Close()
	http.Error(rw, text.String(), http.StatusInternalServerError)
	fmt.Fprintln(os.Stderr, "x")
	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		return n.Remove(name)
	}
	return nil
}`,
	},
}

// TestTheBoundaryWalkRefusesEachWayAcross reads each fixture and holds what the
// walk says to what it must: each way across a boundary refused, as itself, and
// the quiet shapes left alone.
func TestTheBoundaryWalkRefusesEachWayAcross(t *testing.T) {
	t.Parallel()

	w, err := loadWorld(fixtureTarget)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range boundaryFixtures {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			u, err := f.read(w)
			if err != nil {
				t.Fatalf("the fixture does not read: %v", err)
			}
			var said []string
			for _, one := range check(u, w.starters) {
				said = append(said, one.text)
			}
			if len(said) != len(f.want) {
				t.Fatalf("the walk said %d things, want %d:\n%s", len(said), len(f.want), strings.Join(said, "\n"))
			}
			for _, want := range f.want {
				if !slices.ContainsFunc(said, func(text string) bool { return strings.Contains(text, want) }) {
					t.Errorf("no refusal mentions %q; the walk said:\n%s", want, strings.Join(said, "\n"))
				}
			}
		})
	}
}
