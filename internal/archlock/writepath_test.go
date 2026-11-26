package archlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// writingPackages are the packages whose files are the reader's records, so
// the ones the privacy inventory has to name. The guard of what writes is in
// writeguard_test.go.
var writingPackages = []string{
	"internal/mark/",
}

// inventoryFirstTable opens the table of what yomihon leaves on this machine
// outside the browser, which is where every file the reader's records are kept
// in has to be named. It is matched exactly and has to appear once, so a renamed
// or duplicated table fails here rather than leaving the check below reading
// nothing. The browser-storage table has an anchor of its own elsewhere; these
// two must not be held to each other's.
const inventoryFirstTable = "| Data | Where it lives | How long | Who can reach it |"

// storedFileName is what a string naming one of the reader's files looks like:
// a bare lower-case name that ends in .json, .jsonl or .lock. It is a shape and
// not a list, so a file added to internal/mark is found the day it is written.
var storedFileName = regexp.MustCompile(`^[a-z][a-z0-9.-]*\.(?:json|jsonl|lock)$`)

// TestEveryFileTheReadersRecordsAreKeptInIsNamedInThePrivacyInventory holds the
// places yomihon writes to the document a stranger checks the claim against. The
// code and the sentence about it are edited by different hands on different
// days, and a file the product can write without the inventory naming it is the
// one failure this repository has no other way to notice. The names are read
// from the package, from every string in it that is shaped like one, rather than
// listed here, so a file added there fails this the day it is added.
func TestEveryFileTheReadersRecordsAreKeptInIsNamedInThePrivacyInventory(t *testing.T) {
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

	named := map[string]string{}
	fset := token.NewFileSet()
	for _, path := range productionFiles(t, ".go") {
		if !slices.ContainsFunc(writingPackages, func(prefix string) bool { return strings.HasPrefix(path, prefix) }) {
			continue
		}
		found, parseErr := storedFileNames(fset, filepath.Join(repoRoot, path), nil)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		for _, name := range found {
			if _, seen := named[name]; !seen {
				named[name] = path
			}
		}
	}
	if len(named) == 0 {
		t.Fatalf("no file name was found in %v, so this check proves nothing", writingPackages)
	}
	t.Logf("found %d file names in %v", len(named), writingPackages)
	for _, name := range slices.Sorted(maps.Keys(named)) {
		if !slices.ContainsFunc(rows, func(row string) bool { return strings.Contains(row, name) }) {
			t.Errorf("%s names a file called %s and no row of %s names it; %d rows were read",
				named[name], name, inventoryPath, len(rows))
		}
	}
}

// storedFileNames is every string in a Go source that is shaped like the name of
// one of the reader's files, wherever it stands: a constant, an argument, a
// composite literal. src is the source when it is not on disk.
func storedFileNames(fset *token.FileSet, path string, src any) ([]string, error) {
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	var names []string
	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if value, unquoteErr := strconv.Unquote(lit.Value); unquoteErr == nil && storedFileName.MatchString(value) {
			names = append(names, value)
		}
		return true
	})
	return names, nil
}

// The control for the walk above: what it must find, and what it must leave
// alone, so that it cannot go on passing by reading nothing.
func TestTheFileNameWalkFindsWhatCouldBeWrittenAndNothingElse(t *testing.T) {
	t.Parallel()

	const src = `package fixture

import "github.com/koopa0/yomihon/internal/mark/confdir"

const declared = "newthing.json"

type doc struct {
	ID string ` + "`json:\"id\"`" + `
}

// "mentioned.json" in a comment is not a name in use.
func write() {
	_ = confdir.Replace("dir", "inline.lock", nil)
	_ = []string{"listed.json", ".json", "a/b.json", "Upper.json", "x.jsonl", ".jsonl", "x.jsonlx", "plain", "sp ace.json"}
}
`
	got, err := storedFileNames(token.NewFileSet(), "fixture.go", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"newthing.json", "inline.lock", "listed.json", "x.jsonl"}
	if !slices.Equal(got, want) {
		t.Errorf("found %q, want %q", got, want)
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
