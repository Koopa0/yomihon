package archlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
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
	osNames := map[string]bool{}
	for _, imp := range file.Imports {
		if imp.Path.Value != `"os"` {
			continue
		}
		switch {
		case imp.Name == nil:
			osNames["os"] = true
		case imp.Name.Name == ".":
			found = append(found, site{
				path: path,
				line: fset.Position(imp.Pos()).Line,
				text: "os is imported into this file's own namespace, so what it writes cannot be read here",
			})
		case imp.Name.Name == "_":
			// Imported for its initialisers; nothing here is callable through it.
		default:
			osNames[imp.Name.Name] = true
		}
	}

	record := func(pos token.Pos, what string) {
		if mayWrite(path) {
			permitted++
			return
		}
		found = append(found, site{path: path, line: fset.Position(pos).Line, text: what})
	}

	ast.Inspect(file, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		name := selector.Sel.Name
		qualifier, isIdent := selector.X.(*ast.Ident)
		throughOS := isIdent && osNames[qualifier.Name]

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
