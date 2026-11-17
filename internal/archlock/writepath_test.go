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

// yomihon writes in exactly one place, and that place is outside the vault:
// the reader's own records — the marks a reader keeps and the figures the
// owner approved — which internal/mark alone writes, in files of yomihon's own
// under the platform's configuration directory. Nothing writes into a vault.
// The sentence the privacy inventory and the threat model both make to a
// stranger rests on that being true of the code rather than of anyone's memory
// of it, so the syntax tree is asked directly.
var writingPackages = []string{
	"internal/mark/",
}

// inventoryFirstTable opens the table of what yomihon leaves on this machine
// outside the browser, which is where the marks file has to be named. It is
// matched exactly and has to appear once, so a renamed or duplicated table
// fails here rather than leaving the check below reading nothing. The
// browser-storage table has an anchor of its own elsewhere; these two must not
// be held to each other's.
const inventoryFirstTable = "| Data | Where it lives | How long | Who can reach it |"

// marksFileRow is what the inventory has to say about the one written place. The file's own name is the part a person can look for on their disk,
// so that is what is held here rather than a sentence that can be reworded.
const marksFileRow = "marks.json"

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
// The one exception is a receiver the file itself shows to come from a package
// under a writing tree (shownToComeFromAWriter): a call on it reaches code that
// this same walk judges where it is written, and that code may write only
// through the tree that is allowed to. Nothing else is excused, however
// plausible: a receiver of another package, an interface, a local type, or a
// value whose origin the file does not show is still read as the write its
// name says.
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
// the calls that swap a file into place, unix.Renameat2 and unix.RenameatxNp,
// are exactly that.
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

// mayWrite reports whether path is in the one package allowed to write.
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
	// The local names bound to a package under a writing tree.
	fromWriter := map[string]bool{}
	for _, imp := range file.Imports {
		canonical, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		local := canonical[strings.LastIndexByte(canonical, '/')+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		packageName[local] = true
		if inWritingTree(canonical) && local != "." && local != "_" {
			fromWriter[local] = true
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

	origins := writerOrigins(file, fromWriter)
	fromAWriter := func(receiver ast.Expr) bool {
		return shownToComeFromAWriter(receiver, fromWriter, origins, packageName)
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
			case !qualifierIsPackage(selector, packageName) && slices.Contains(methodWrites, selector.Sel.Name) && !fromAWriter(selector.X):
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
		case !throughOS && slices.Contains(methodWrites, name) && !fromAWriter(selector.X):
			record(selector.Pos(), "a call to "+name+", which changes what it is called on")
		}
		return true
	})
	return found, permitted
}

// inWritingTree reports whether an import path is a package under one of the
// trees that may write. The set comes from writingPackages and this module's
// own path, so a tree added there is excused here without a second list, and a
// package that only shares a prefix with one — internal/markdown against
// internal/mark — is not.
func inWritingTree(importPath string) bool {
	for _, tree := range writingPackages {
		if strings.HasPrefix(importPath+"/", module+"/"+tree) {
			return true
		}
	}
	return false
}

// writerOrigins says, for every name the file declares as a variable, whether
// every one of its declarations is shown to come from a writing tree: a
// parameter, result or var of a type spelled through such a package, or a name
// bound to a call of one of its functions, to its composite literal, or to a
// new of its type. A name declared even once in a way the file does not show —
// a range variable, a type switch, a local type, a package of another tree —
// is not in the set, and nor is a name the file does not declare, such as a
// field or a variable of another file of the package. Declarations are
// gathered across the whole file rather than by scope, so a name declared
// differently in two functions is judged by its weaker declaration.
func writerOrigins(file *ast.File, fromWriter map[string]bool) map[string]bool {
	allFromWriter := map[string]bool{}
	declare := func(name string, fromTree bool) {
		if name == "_" {
			return
		}
		if seen, declared := allFromWriter[name]; declared {
			allFromWriter[name] = seen && fromTree
			return
		}
		allFromWriter[name] = fromTree
	}
	fields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				declare(name.Name, typeFromWriter(field.Type, fromWriter))
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			fields(node.Recv)
		case *ast.FuncType:
			fields(node.Params)
			fields(node.Results)
		case *ast.ValueSpec:
			for i, name := range node.Names {
				switch {
				case node.Type != nil:
					declare(name.Name, typeFromWriter(node.Type, fromWriter))
				case len(node.Values) == len(node.Names):
					declare(name.Name, exprFromWriter(node.Values[i], fromWriter))
				case len(node.Values) == 1:
					declare(name.Name, exprFromWriter(node.Values[0], fromWriter))
				default:
					declare(name.Name, false)
				}
			}
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				return true
			}
			for i, lhs := range node.Lhs {
				name, isIdent := lhs.(*ast.Ident)
				if !isIdent {
					continue
				}
				switch {
				case len(node.Rhs) == len(node.Lhs):
					declare(name.Name, exprFromWriter(node.Rhs[i], fromWriter))
				case len(node.Rhs) == 1:
					declare(name.Name, exprFromWriter(node.Rhs[0], fromWriter))
				default:
					declare(name.Name, false)
				}
			}
		case *ast.RangeStmt:
			if node.Tok != token.DEFINE {
				return true
			}
			for _, expr := range []ast.Expr{node.Key, node.Value} {
				if name, isIdent := expr.(*ast.Ident); isIdent {
					declare(name.Name, false)
				}
			}
		}
		return true
	})
	return allFromWriter
}

// typeFromWriter reports whether a type expression is spelled through a package
// under a writing tree, behind any pointers, parentheses and type arguments.
func typeFromWriter(expr ast.Expr, fromWriter map[string]bool) bool {
	for {
		switch node := expr.(type) {
		case *ast.StarExpr:
			expr = node.X
		case *ast.ParenExpr:
			expr = node.X
		case *ast.IndexExpr:
			expr = node.X
		case *ast.IndexListExpr:
			expr = node.X
		case *ast.SelectorExpr:
			qualifier, isIdent := node.X.(*ast.Ident)
			return isIdent && fromWriter[qualifier.Name]
		default:
			return false
		}
	}
}

// exprFromWriter reports whether an expression is shown to produce a value of a
// writing tree: a call of one of its functions, a literal of one of its types,
// or a new of one, behind an address-of or parentheses. It reads the spelling
// and not the result type, so a function of the tree that returned a handle on
// the filesystem would pass here; that is what the tree being the only place
// allowed to write is trusted not to do.
func exprFromWriter(expr ast.Expr, fromWriter map[string]bool) bool {
	for {
		switch node := expr.(type) {
		case *ast.ParenExpr:
			expr = node.X
		case *ast.UnaryExpr:
			if node.Op != token.AND {
				return false
			}
			expr = node.X
		case *ast.CompositeLit:
			return node.Type != nil && typeFromWriter(node.Type, fromWriter)
		case *ast.CallExpr:
			switch fun := node.Fun.(type) {
			case *ast.SelectorExpr:
				qualifier, isIdent := fun.X.(*ast.Ident)
				return isIdent && fromWriter[qualifier.Name]
			case *ast.Ident:
				return fun.Name == "new" && len(node.Args) == 1 && typeFromWriter(node.Args[0], fromWriter)
			default:
				return false
			}
		default:
			return false
		}
	}
}

// shownToComeFromAWriter reports whether the receiver of a method is shown, by
// this file alone, to be a value of a writing tree: a variable every one of
// whose declarations is (writerOrigins), or the result of a call of its
// functions. A package name is not a receiver, and neither is a field or a
// chain of them: what the file does not show is not assumed.
func shownToComeFromAWriter(receiver ast.Expr, fromWriter, origins, packageName map[string]bool) bool {
	for {
		switch node := receiver.(type) {
		case *ast.ParenExpr:
			receiver = node.X
		case *ast.StarExpr:
			receiver = node.X
		case *ast.Ident:
			return origins[node.Name] && !packageName[node.Name]
		case *ast.CallExpr:
			return exprFromWriter(node, fromWriter)
		default:
			return false
		}
	}
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

// TestTheOnlyPlaceProductionCodeWritesIsTheReadersOwnFiles reads every
// shipped Go file for a spelling that changes the filesystem and permits
// exactly one package: the one that keeps the reader's own marks in files of
// yomihon's own, outside the vault.
//
// A second is what this exists to catch, and above all one that opens a vault
// file for writing: yomihon is read-only toward the vault. Nothing else here
// writes, and the two documents that tell a stranger so are held to the same
// tree by the check below them.
//
// It judges a call rather than a name, unlike the outbound-connection check,
// because the names here are ordinary English words that appear as methods on
// types that have nothing to do with the filesystem. What that costs is a
// method named Remove on some other type reading as a write; what it buys is
// that a confined write through an *os.Root, which is how this repository
// writes, cannot hide behind a receiver.
func TestTheOnlyPlaceProductionCodeWritesIsTheReadersOwnFiles(t *testing.T) {
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
	// The writing package has to have been seen writing. A walk that never
	// reached it would report a tree with no writes in it at all, which is the
	// one result that would look like success and mean nothing.
	for _, pkg := range writingPackages {
		if permitted[pkg] == 0 {
			t.Fatalf("no write was seen in %s, so this walk never reached the package it is meant to permit", pkg)
		}
	}
	t.Logf("examined the shipped tree and found writes in %d permitted packages: %v", len(permitted), permitted)
	report(t, "this changes the filesystem, and yomihon writes in exactly one place, outside the vault: the reader's own marks files", found)
}

// TestNoPackageReachesTheRawSyscallWrappers is the structural half of the
// check above, and it exists because the other half is a list of names.
//
// The prefix families are a good list and they are still a list: golang.org/x/
// sys/unix is a few thousand symbols wide, it grows, and a name like
// RenameatxNp is one no general rule would have produced. What can be held
// without a list is who may reach that package at all, which is the same shape
// the layering check uses: nothing does, since the one writing package needs
// no more than os. A package that cannot import it cannot call anything in it,
// whatever it is called.
//
// syscall is deliberately not held this way. The command reads a termination
// signal from it and the request-shutdown path reads two errnos, neither of
// which writes anything, so a ban would refuse honest code; those two are held
// by the families instead.
func TestNoPackageReachesTheRawSyscallWrappers(t *testing.T) {
	t.Parallel()

	var found []site
	examined := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		examined++
		for _, imp := range file.Imports {
			canonical, err := strconv.Unquote(imp.Path.Value)
			if err != nil || canonical != unixPackage {
				continue
			}
			found = append(found, site{
				path: path,
				line: fset.Position(imp.Pos()).Line,
				text: "imports " + canonical,
			})
		}
	})
	if examined == 0 {
		t.Fatal("no production file was examined, so this walk proves nothing")
	}
	t.Logf("examined %d shipped files for an import of %s", examined, unixPackage)
	report(t, "no package may reach the raw syscall wrappers, whose surface no list of names can cover", found)
}

// TestTheMarksFileIsNamedInThePrivacyInventory holds the one written place to
// the document a stranger checks the claim against. The code and the
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
		name: "a raw rename that swaps a file into place",
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
	{
		name: "Remove on a vaults store a parameter names, from outside the tree",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "github.com/koopa0/yomihon/internal/mark/vaults"
func forget(store *vaults.Store, id string) error { _, err := store.Remove(id); return err }`,
		clean: true,
	},
	{
		name: "Remove on a store a function of the tree returned, under another import name",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import v "github.com/koopa0/yomihon/internal/mark/vaults"
func forget(dir, id string) error {
	store, err := v.Open(dir, nil)
	if err != nil {
		return err
	}
	drop := store.Remove
	_, err = drop(id)
	return err
}`,
		clean: true,
	},
	{
		name: "Remove straight on the result of a call of the tree",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "github.com/koopa0/yomihon/internal/mark/vaults"
func forget(dir, id string) error { s, _ := vaults.Open(dir, nil); _, err := (*s).Remove(id); return err }`,
		clean: true,
	},
	{
		name: "os.Remove outside the tree",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "os"
func forget(path string) error { return os.Remove(path) }`,
		want: []string{"os.Remove"},
	},
	{
		name: "Remove on an *os.Root outside the tree",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "os"
func forget(root *os.Root, name string) error { return root.Remove(name) }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "os.WriteFile in a package that sits beside the tree",
		path: "internal/markdown/save.go",
		src: `package markdown
import "os"
func save(name string, data []byte) error { return os.WriteFile(name, data, 0o600) }`,
		want: []string{"os.WriteFile"},
	},
	{
		name: "Remove on a local type outside the tree",
		path: "internal/note/index.go",
		src: `package note
type index struct{ ids map[string]bool }
func (i *index) Remove(id string) { delete(i.ids, id) }
func drop(i *index, id string) { i.Remove(id) }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "Remove on a type of another package",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "github.com/koopa0/yomihon/internal/vault"
func forget(r *vault.Reader, name string) error { return r.Remove(name) }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "Remove on a package whose path only starts like the tree's",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "github.com/koopa0/yomihon/internal/markdown"
func forget(s *markdown.Store, id string) error { return s.Remove(id) }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "Remove on an interface of another package",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "io/fs"
type remover interface{ Remove(string) error }
func forget(r remover, name string) error { return r.Remove(name) }
var _ fs.FS`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "Remove on a receiver whose origin the file does not show",
		path: "cmd/yomihon/vaults.go",
		src: `package main
func forget(name string) error { return held.Remove(name) }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "Remove on a field, which the file does not show the type of",
		path: "cmd/yomihon/host.go",
		src: `package main
import "github.com/koopa0/yomihon/internal/mark/vaults"
type host struct{ store *vaults.Store }
func (h *host) forget(id string) error { _, err := h.store.Remove(id); return err }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "one name declared as a store in one function and a root in another",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import (
	"os"
	"github.com/koopa0/yomihon/internal/mark/vaults"
)
func first(store *vaults.Store, id string) error { _, err := store.Remove(id); return err }
func second(store *os.Root, name string) error { return store.Remove(name) }`,
		want: []string{"a call to Remove, which changes what it is called on"},
	},
	{
		name: "a store the loop gave, which the file does not show the origin of",
		path: "cmd/yomihon/vaults.go",
		src: `package main
import "github.com/koopa0/yomihon/internal/mark/vaults"
func forget(all []*vaults.Store, id string) {
	for _, store := range all {
		_, _ = store.Remove(id)
	}
}`,
		want: []string{"a call to Remove, which changes what it is called on"},
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
