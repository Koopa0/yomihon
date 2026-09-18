package archlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// yomihon writes in exactly two places, and this file is what holds that
// number at two.
//
// One is the status field inside a note, which internal/status alone rewrites.
// The other is the reader's own marks, which internal/mark alone writes, in a
// file of yomihon's own outside the vault. Everything else this program does
// is reading. The sentence the privacy inventory and the threat model both
// make to a stranger rests on that being true of the code rather than of
// anyone's memory of it, so the syntax tree is asked directly.
var writingPackages = []string{
	"internal/status/",
	"internal/mark/",
}

// inventoryFirstTable opens the table of what yomihon leaves on this machine
// outside the browser, which is where the marks file has to be named. It is
// matched exactly and has to appear once, so a renamed or duplicated table
// fails here rather than leaving the check below reading nothing. The
// browser-storage table has an anchor of its own elsewhere; these two must not
// be held to each other's.
const inventoryFirstTable = "| Data | Where it lives | How long | Who can reach it |"

// marksFileRow is what the inventory has to say about the second written
// place. The file's own name is the part a person can look for on their disk,
// so that is what is held here rather than a sentence that can be reworded.
const marksFileRow = "reader.json"

// osWriteFamilies are the prefixes of every os function that changes a
// directory or a file rather than reading one. They are matched by prefix
// because the standard library grows: Mkdir became MkdirAll and MkdirTemp, and
// a variant added tomorrow is refused the day it appears rather than the day
// somebody remembers this list.
//
// Open is deliberately absent: it opens for reading. OpenFile is the one
// spelling that is a write or a read depending on the flags it is handed, and
// it is judged by those flags below rather than by its name.
var osWriteFamilies = []string{
	"Chmod", "Chown", "Chtimes", "Create", "Lchown", "Link", "Mkdir",
	"Remove", "Rename", "Symlink", "Truncate", "WriteFile",
}

// methodWrites are the same operations reached as methods, which is how a
// confined write reaches them: an *os.Root or an *os.File names the thing it
// is writing to and then one of these. A method call on a value cannot be
// resolved back to its package by a walk that only reads import names, so
// these are matched on the name alone — which over-reports rather than
// under-reports, and an over-report is a line somebody reads.
//
// Write, WriteString and Truncate are absent, and their absence is the one
// judgement in this list. They are how every in-memory builder and buffer in
// the standard library is filled, so including them reported a dozen lines
// that assemble a string and not one that touched a disk — and a check whose
// every result is a false alarm is one a reader learns to skip. What reaches
// a file still has to be named by one of the spellings above before those two
// can be called on it: a handle is opened, created or renamed into place, and
// that is the line this catches.
var methodWrites = []string{
	"Chmod", "Chown", "Create", "Link", "Mkdir", "MkdirAll", "Remove",
	"RemoveAll", "Rename", "Symlink", "WriteFile",
}

// unixPackage is the raw-syscall wrapper this repository writes its
// platform-specific halves against, and lowLevelPackages are it and the
// standard library's own. Both reach the filesystem underneath os, so a write
// spelled through either is a write the check over os alone cannot see — and
// the two calls that swap a note into place, unix.Renameat2 and
// unix.RenameatxNp, are exactly that.
const unixPackage = "golang.org/x/sys/unix"

var lowLevelPackages = map[string]bool{unixPackage: true, "syscall": true}

// lowLevelWriteFamilies are the prefixes of those wrappers that change a file
// or a directory. Prefixes again, so the at-relative and f-prefixed variants of
// each are covered by the name they are built from.
//
// Open is among them with no flag reading, unlike os.OpenFile above. The flag
// argument sits in a different position in each of these and one of them takes
// none at all, so a reader that tried to judge them would be guessing; counting
// every raw open as a write over-reports in a place that has none today, which
// is the direction to be wrong in.
//
// The errno and flag constants these files also name — ENOATTR, RENAME_SWAP,
// SIGTERM — share no prefix with any of these, and the set equality is not what
// keeps them out: they are screaming-case and these are not.
var lowLevelWriteFamilies = []string{
	"Chmod", "Chown", "Creat", "Fchmod", "Fchown", "Fremovexattr", "Fsetxattr",
	"Ftruncate", "Lchown", "Link", "Lremovexattr", "Lsetxattr", "Mkdir", "Mkfifo",
	"Mknod", "Open", "Removexattr", "Rename", "Rmdir", "Setxattr", "Symlink",
	"Truncate", "Unlink", "Write",
}

// writeFlags are the os constants that make an OpenFile a write.
var writeFlags = []string{"O_WRONLY", "O_RDWR", "O_CREATE", "O_APPEND", "O_TRUNC"}

// mayWrite reports whether path is one of the two packages allowed to.
func mayWrite(path string) bool {
	for _, pkg := range writingPackages {
		if strings.HasPrefix(path, pkg) {
			return true
		}
	}
	return false
}

// openFileArity is how many arguments the filesystem's OpenFile takes, in both
// the package function and the rooted method: a name, the flags, and the
// permission bits. It is what tells that function from another method of the
// same name — this repository has one, the vault reader's, which takes a
// context and an already-validated entry and opens it to be read.
//
// The count is the discriminator rather than a list of receivers to excuse,
// because a receiver held in a field or handed over as a value is invisible to
// a walk that reads import names, while the shape of the call is written on
// the page. A standard library that grew a fourth parameter would stop
// matching, and the fixtures below are where that would be noticed.
const openFileArity = 3

// openFileWrites reports whether an OpenFile call's flag argument asks for a
// write, and whether that argument could be read at all.
//
// A flag expression this cannot resolve is reported as a write rather than
// skipped. Skipping it is how a check goes quiet: the one call that matters
// would be the one spelled in a way nobody anticipated, and a silent pass
// there is worse than a line naming a read.
func openFileWrites(call *ast.CallExpr) (writes, resolved bool) {
	found := false
	unresolved := false
	ast.Inspect(call.Args[1], func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if slices.Contains(writeFlags, node.Sel.Name) {
				found = true
			} else if !strings.HasPrefix(node.Sel.Name, "O_") {
				unresolved = true
			}
			// Its parts are the package name and the flag name, and reading
			// them on their own would report the package as something this
			// could not resolve.
			return false
		case *ast.Ident:
			// A bare name standing where a flag belongs is a variable, a
			// constant declared elsewhere, or a dot-imported flag. None of the
			// three can be read here, and the last is refused at the import.
			if slices.Contains(writeFlags, node.Name) {
				found = true
			} else {
				unresolved = true
			}
		case *ast.BasicLit:
			// Zero is the read-only flag written as a number. Any other
			// literal is a bit pattern this does not decode.
			if node.Value != "0" {
				unresolved = true
			}
		case *ast.BinaryExpr, *ast.ParenExpr:
		case nil:
		default:
			unresolved = true
		}
		return true
	})
	if unresolved {
		return true, false
	}
	return found, true
}

// writeSites reports every spelling in one file that changes the filesystem,
// and counts separately the ones its own package is allowed, so a caller can
// tell a clean tree from a walk that read nothing at all.
//
// Import names are resolved first, for the reason the outbound check already
// gives: a file reaching os under another name writes calls no fixed spelling
// would recognise, and one that dot-imports it hides them entirely.
func writeSites(path string, fset *token.FileSet, file *ast.File) (found []site, permitted int) {
	// Which local name binds to which of the packages that reach the
	// filesystem. A file reaching one under another name writes calls no fixed
	// spelling would recognise, and one that dot-imports it hides them
	// entirely, which is reported rather than audited.
	pkgOf := map[string]string{}
	// Every local import name, filesystem or not. A method taken as a value is
	// judged on its name alone below, and the names in that list are ordinary
	// English words that are also types elsewhere — ast.Link, sequence.Link.
	// A type is always reached through a package name and a bound method never
	// is, so knowing which qualifiers are packages is what tells them apart.
	packageName := map[string]bool{}
	for _, imp := range file.Imports {
		canonical, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if imp.Name != nil {
			packageName[imp.Name.Name] = true
		} else {
			packageName[canonical[strings.LastIndexByte(canonical, '/')+1:]] = true
		}
		if canonical != "os" && !lowLevelPackages[canonical] {
			continue
		}
		switch {
		case imp.Name == nil:
			pkgOf[canonical[strings.LastIndexByte(canonical, '/')+1:]] = canonical
		case imp.Name.Name == ".":
			found = append(found, site{
				path: path,
				line: fset.Position(imp.Pos()).Line,
				text: canonical + " is imported into this file's own namespace, so what it writes cannot be read here",
			})
		case imp.Name.Name == "_":
			// Imported for its initialisers; nothing here is callable through it.
		default:
			pkgOf[imp.Name.Name] = canonical
		}
	}

	record := func(pos token.Pos, what string) {
		if mayWrite(path) {
			permitted++
			return
		}
		found = append(found, site{path: path, line: fset.Position(pos).Line, text: what})
	}

	// A write handed over as a value carries no call for the walk to read —
	// var w = os.WriteFile, then w(name, data, perm) somewhere else entirely —
	// so the name is judged where it is written, as the environment guard
	// judges a reader referenced rather than called. Because the walk meets a
	// call before the selector inside it, a selector already judged as a call
	// is recognised when it comes round again.
	//
	// A confined write goes the same way round: swap := root.Rename hands over
	// a method whose receiver is already bound, and the call that follows names
	// no filesystem at all. So a method write is judged as a reference on the
	// name alone, which is the policy its call site already follows, and it
	// over-reports for the same reason and to the same end.
	judged := map[*ast.SelectorExpr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if selector, isSelector := n.(*ast.SelectorExpr); isSelector && !judged[selector] {
			pkg, name := selected(selector, pkgOf)
			switch {
			case writesByName(pkg, name):
				record(selector.Pos(), qualifierOf(selector)+"."+name+", referenced as a value")
			case !qualifierIsPackage(selector, packageName) && slices.Contains(methodWrites, selector.Sel.Name):
				record(selector.Pos(), "a reference to "+selector.Sel.Name+", which changes what it is called on")
			}
			return true
		}
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		judged[selector] = true
		name := selector.Sel.Name
		qualifier, isIdent := selector.X.(*ast.Ident)
		pkg := ""
		if isIdent {
			pkg = pkgOf[qualifier.Name]
		}
		throughOS := pkg == "os"

		if lowLevelPackages[pkg] {
			if hasPrefixIn(name, lowLevelWriteFamilies) {
				record(selector.Pos(), qualifier.Name+"."+name)
			}
			return true
		}
		if name == "OpenFile" && len(call.Args) == openFileArity {
			writes, resolved := openFileWrites(call)
			if !writes {
				return true
			}
			detail := "OpenFile with write flags"
			if !resolved {
				detail = "OpenFile whose flags this check could not read, so it is counted as a write"
			}
			record(selector.Pos(), detail)
			return true
		}
		switch {
		case throughOS && hasPrefixIn(name, osWriteFamilies):
			record(selector.Pos(), qualifier.Name+"."+name)
		case !throughOS && slices.Contains(methodWrites, name):
			record(selector.Pos(), "a call to "+name+", which changes what it is called on")
		}
		return true
	})
	return found, permitted
}

// selected reports the canonical package and symbol a selector names, for a
// selector whose qualifier is one of the imported filesystem packages.
func selected(selector *ast.SelectorExpr, pkgOf map[string]string) (pkg, name string) {
	qualifier, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return "", ""
	}
	return pkgOf[qualifier.Name], selector.Sel.Name
}

// qualifierIsPackage reports whether the selector is reached through an
// imported package name rather than through a value. A package qualifier means
// the selector names a function or a type, both of which are judged elsewhere;
// what is left is a method bound to a receiver.
func qualifierIsPackage(selector *ast.SelectorExpr, packageName map[string]bool) bool {
	qualifier, isIdent := selector.X.(*ast.Ident)
	return isIdent && packageName[qualifier.Name]
}

func qualifierOf(selector *ast.SelectorExpr) string {
	if qualifier, isIdent := selector.X.(*ast.Ident); isIdent {
		return qualifier.Name
	}
	return "?"
}

// writesByName reports whether a symbol changes the filesystem judged by its
// name alone, which is all a reference hands over.
//
// OpenFile is absent: whether it writes is decided by the flags a call passes,
// and a reference carries none. What a bare reference to it could become is a
// call this check does read, at whatever site supplies those flags — and where
// that site is a variable it is already refused there as flags this cannot
// resolve.
func writesByName(pkg, name string) bool {
	switch {
	case pkg == "os":
		return name != "OpenFile" && hasPrefixIn(name, osWriteFamilies)
	case lowLevelPackages[pkg]:
		return hasPrefixIn(name, lowLevelWriteFamilies)
	default:
		return false
	}
}

func hasPrefixIn(name string, families []string) bool {
	for _, family := range families {
		if strings.HasPrefix(name, family) {
			return true
		}
	}
	return false
}

// TestTheOnlyPlacesProductionCodeWritesAreTheStatusFieldAndTheMarksFile reads
// every shipped Go file for a spelling that changes the filesystem and permits
// exactly two packages: the write face that rewrites a note's status line, and
// the one that keeps the reader's own marks in a file of yomihon's own.
//
// A third is what this exists to catch. Nothing else here writes, and the two
// documents that tell a stranger so are held to the same tree by the check
// below them.
//
// It judges a call rather than a name, unlike the outbound-connection check,
// because the names here are ordinary English words that appear as methods on
// types that have nothing to do with the filesystem. What that costs is a
// method named Remove on some other type reading as a write; what it buys is
// that a confined write through an *os.Root, which is how this repository
// writes, cannot hide behind a receiver.
func TestTheOnlyPlacesProductionCodeWritesAreTheStatusFieldAndTheMarksFile(t *testing.T) {
	t.Parallel()

	var found []site
	permitted := map[string]int{}
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		sites, allowed := writeSites(path, fset, file)
		found = append(found, sites...)
		if allowed > 0 {
			for _, pkg := range writingPackages {
				if strings.HasPrefix(path, pkg) {
					permitted[pkg] += allowed
				}
			}
		}
	})
	// Each writing package has to have been seen writing. A walk that reached
	// neither would report a tree with no writes in it at all, which is the
	// one result that would look like success and mean nothing.
	for _, pkg := range writingPackages {
		if permitted[pkg] == 0 {
			t.Fatalf("no write was seen in %s, so this walk never reached the package it is meant to permit", pkg)
		}
	}
	t.Logf("examined the shipped tree and found writes in %d permitted packages: %v", len(permitted), permitted)
	report(t, "this changes the filesystem, and yomihon writes in exactly two places: a note's status field and the reader's own marks file", found)
}

// TestOnlyTheWritingPackagesReachTheRawSyscallWrappers is the structural half
// of the check above, and it exists because the other half is a list of names.
//
// The prefix families are a good list and they are still a list: golang.org/x/
// sys/unix is a few thousand symbols wide, it grows, and the one that mattered
// here — RenameatxNp — is a name no general rule would have produced. What can
// be held without a list is who may reach that package at all, which is the
// same shape the layering check uses. A package that cannot import it cannot
// call anything in it, whatever it is called.
//
// syscall is deliberately not held this way. The command reads a termination
// signal from it and the request-shutdown path reads two errnos, neither of
// which writes anything, so a ban would refuse honest code; those two are held
// by the families instead.
func TestOnlyTheWritingPackagesReachTheRawSyscallWrappers(t *testing.T) {
	t.Parallel()

	var found []site
	reached := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		for _, imp := range file.Imports {
			canonical, err := strconv.Unquote(imp.Path.Value)
			if err != nil || canonical != unixPackage {
				continue
			}
			if mayWrite(path) {
				reached++
				continue
			}
			found = append(found, site{
				path: path,
				line: fset.Position(imp.Pos()).Line,
				text: "imports " + canonical,
			})
		}
	})
	if reached == 0 {
		t.Fatalf("no file was seen importing %s, so this walk never reached the package it is meant to permit", unixPackage)
	}
	t.Logf("examined the shipped tree and found %d permitted imports of %s", reached, unixPackage)
	report(t, "only the two packages that write may reach the raw syscall wrappers, whose surface no list of names can cover", found)
}

// TestTheMarksFileIsNamedInThePrivacyInventory holds the second written place
// to the document a stranger checks the claim against. The code and the
// sentence about it are edited by different hands on different days, and a
// file the product writes without the inventory naming it is the one failure
// this repository has no other way to notice.
func TestTheMarksFileIsNamedInThePrivacyInventory(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join(repoRoot, inventoryPath)) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatalf("read %s: %v", inventoryPath, err)
	}
	inventory := string(data)
	if got := strings.Count(inventory, inventoryFirstTable); got != 1 {
		t.Fatalf("%s opens the table of what yomihon leaves on this machine %d times, want exactly 1; the check below reads that table",
			inventoryPath, got)
	}
	rows, err := tableRows(inventory, inventoryFirstTable)
	if err != nil {
		t.Fatalf("read the table in %s: %v", inventoryPath, err)
	}
	if len(rows) < 4 {
		t.Fatalf("read %d rows out of the table in %s, which is too few to be that table", len(rows), inventoryPath)
	}
	if !slices.ContainsFunc(rows, func(row string) bool { return strings.Contains(row, marksFileRow) }) {
		t.Errorf("the reader's own marks are written to a file named %s and no row of %s names it; %d rows were read",
			marksFileRow, inventoryPath, len(rows))
	}
}

// tableRows returns the body rows of the markdown table opened by header.
func tableRows(document, header string) ([]string, error) {
	_, after, found := strings.Cut(document, header)
	if !found {
		return nil, fmt.Errorf("the table header %q is not in the document", header)
	}
	var rows []string
	for line := range strings.SplitSeq(after, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if len(rows) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "|---") || strings.HasPrefix(line, "| ---") {
			continue
		}
		rows = append(rows, line)
	}
	return rows, nil
}

// writeGuardFixtures are this check's own controls. Each is one way a write
// could reach the tree while the check above stayed green, and they run with
// the ordinary suite so the guard is watched to fail on every test run rather
// than on the day somebody thinks to try it.
var writeGuardFixtures = []struct {
	name  string
	path  string
	src   string
	want  []string
	clean bool
}{
	{
		name: "a plain write from a third package",
		path: "internal/report/report.go",
		src: `package report
import "os"
func save(name string, data []byte) error { return os.WriteFile(name, data, 0o600) }`,
		want: []string{"os.WriteFile"},
	},
	{
		name: "os reached under another name",
		path: "internal/report/report.go",
		src: `package report
import operating "os"
func save(name string) error { return operating.Remove(name) }`,
		want: []string{"operating.Remove"},
	},
	{
		name: "a confined write through a rooted directory",
		path: "internal/report/report.go",
		src: `package report
import "os"
func save(root *os.Root, name string) error { return root.Rename(name, name+".old") }`,
		want: []string{"a call to Rename, which changes what it is called on"},
	},
	{
		name: "OpenFile asking to create",
		path: "internal/report/report.go",
		src: `package report
import "os"
func save(root *os.Root, name string) { _, _ = root.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600) }`,
		want: []string{"OpenFile with write flags"},
	},
	{
		name: "OpenFile whose flags cannot be read here",
		path: "internal/report/report.go",
		src: `package report
import "os"
func save(root *os.Root, name string, flags int) { _, _ = root.OpenFile(name, flags, 0o600) }`,
		want: []string{"OpenFile whose flags this check could not read, so it is counted as a write"},
	},
	{
		name: "an OpenFile that is not the filesystem's",
		path: "internal/report/report.go",
		src: `package report
import "context"
type reader struct{}
func (reader) OpenFile(ctx context.Context, entry string) error { return nil }
func read(ctx context.Context, r reader, entry string) error { return r.OpenFile(ctx, entry) }`,
		clean: true,
	},
	{
		// The receiver is bound where the method is named and the call that
		// follows mentions no filesystem, so the line below is the only one a
		// reader of this file would ever see it on.
		name: "a confined write handed over as a method value",
		path: "internal/report/report.go",
		src: `package report
import "os"
func swap(root *os.Root, name string) error {
	rename := root.Rename
	return rename(name, name+".old")
}`,
		want: []string{"a reference to Rename, which changes what it is called on"},
	},
	{
		// A type reached through a package name shares these names — this
		// repository's own graph has a Link and so does the markdown parser it
		// reads — and naming a type is not writing anything.
		name: "types whose names are also the names of writes",
		path: "internal/nav/nav.go",
		src: `package nav
import (
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/yuin/goldmark/ast"
)
func count(links []sequence.Link, node ast.Node) int {
	switch node.(type) {
	case *ast.Link:
		return len(links)
	}
	return 0
}`,
		clean: true,
	},
	{
		name: "the repository's own idiom for swapping a file into place",
		path: "internal/report/report.go",
		src: `package report
import "golang.org/x/sys/unix"
func swap(from, to string) error { return unix.Renameat2(0, from, 0, to, unix.RENAME_EXCHANGE) }`,
		want: []string{"unix.Renameat2"},
	},
	{
		name: "a raw syscall write under another name",
		path: "internal/report/report.go",
		src: `package report
import sys "golang.org/x/sys/unix"
func stamp(fd int, value []byte) error { return sys.Fsetxattr(fd, "user.x", value, 0) }`,
		want: []string{"sys.Fsetxattr"},
	},
	{
		name: "the errno and flag constants beside them are not writes",
		path: "internal/report/report.go",
		src: `package report
import (
	"syscall"
	"golang.org/x/sys/unix"
)
func quiet(err error) bool { return err == unix.ENOATTR || err == syscall.EPIPE }
const swap = unix.RENAME_SWAP
var stop = syscall.SIGTERM`,
		clean: true,
	},
	{
		name: "a write handed over as a value",
		path: "internal/report/report.go",
		src: `package report
import "os"
var save = os.WriteFile
func keep(name string, data []byte) error { return save(name, data, 0o600) }`,
		want: []string{"os.WriteFile, referenced as a value"},
	},
	{
		name: "a write passed to something else to call",
		path: "internal/report/report.go",
		src: `package report
import "os"
func keep(with func(string) error) error { return with("x") }
func run() error { return keep(os.Remove) }`,
		want: []string{"os.Remove, referenced as a value"},
	},
	{
		name: "OpenFile for reading only",
		path: "internal/report/report.go",
		src: `package report
import "os"
func read(root *os.Root, name string) { _, _ = root.OpenFile(name, os.O_RDONLY, 0) }`,
		clean: true,
	},
	{
		name: "the same write inside the package that may make it",
		path: "internal/mark/file.go",
		src: `package mark
import "os"
func save(name string, data []byte) error { return os.WriteFile(name, data, 0o600) }`,
		clean: true,
	},
}

// TestTheWriteGuardCatchesEveryShapeOfWrite runs those controls. A check whose
// every result is a pass teaches a reader to skip it, so this one is shown
// failing on each shape it claims to refuse, and passing on the two it must
// not refuse: a read, and a write in a package that is allowed one.
func TestTheWriteGuardCatchesEveryShapeOfWrite(t *testing.T) {
	t.Parallel()

	for _, fixture := range writeGuardFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, fixture.path, fixture.src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse the fixture: %v", err)
			}
			found, permitted := writeSites(fixture.path, fset, file)
			if fixture.clean {
				if len(found) != 0 {
					t.Errorf("the guard refused %q, which it must not: %+v", fixture.name, found)
				}
				if mayWrite(fixture.path) && permitted == 0 {
					t.Errorf("the write in %s was neither refused nor counted, so it was not seen at all", fixture.path)
				}
				return
			}
			texts := make([]string, 0, len(found))
			for _, s := range found {
				texts = append(texts, s.text)
			}
			for _, want := range fixture.want {
				if !slices.Contains(texts, want) {
					t.Errorf("the guard let %q past: found %v, want one of them to be %q", fixture.name, texts, want)
				}
			}
		})
	}
}
