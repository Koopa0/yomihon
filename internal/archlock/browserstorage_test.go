package archlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// inventoryPath is the record of what yomihon leaves on the machine of the
// person running it. The checks below keep its browser-storage table equal to
// what the code actually stores, in both directions: a key with no row fails,
// and a row with no key fails. Prose rots quietly, and this is the one document
// a stranger checks the product's central claim against.
const inventoryPath = "docs/privacy/data-inventory.md"

// inventoryTableHeader opens the browser-storage table inside that document.
// It is matched exactly and has to appear once, so a renamed or duplicated
// table fails here rather than leaving the checks below reading nothing.
const inventoryTableHeader = "| Kept in the browser | What it holds | How long |"

// cookiePrefix is what every cookie yomihon writes is named with. The name is
// a Go string literal in every case — the server writes all six, and the
// browser only ever rewrites names the server already answers to — so the Go
// declarations are the authority for the set and the JavaScript is not.
const cookiePrefix = "yomihon_"

// The two files that each carry a copy of the reading-preference set: one
// resolves a stored choice for the page being rendered, the other offers the
// same choices on the settings page. Keeping a second copy of a closed set is
// how a set silently splits, so the two are compared against each other below.
const (
	readingPreferencesFile = "internal/ui/layouts/preferences.go"
	settingsPageFile       = "internal/preference/preference.go"
	languageCookieFile     = "internal/wording/wording.go"
)

// webStorageRoot is the only directory holding hand-written client modules, and
// mermaidDir is the vendored diagram runtime inside it. Minified third-party
// code is not something the key reader below can read a key out of, so it is
// skipped — and the skip is checked against the directory existing, because a
// skip aimed at a directory that moved would quietly stop skipping anything and
// quietly stop failing.
const (
	webStorageRoot = "assets/js"
	mermaidDir     = "assets/js/mermaid"
)

// webStorageUse finds every mention of the two Web Storage areas. Each one has
// to be a get, set or remove whose key the reader below can resolve; any other
// shape — a clear, an index expression, an alias — fails rather than passing
// unread. A mention inside a comment is counted and fails too, which is the
// cheaper mistake: skipping what looks like a comment could skip a real use and
// pass, while a failure naming a comment line says what it found.
var webStorageUse = regexp.MustCompile(`\b(localStorage|sessionStorage)\b(\.(getItem|setItem|removeItem)\()?`)

// cookieAssignment finds every write to document.cookie, and cookieWrite the
// subset whose name this can read: the text between the opening quote and the
// "=" that ends the name, both inside one quoted run. A name joined together
// from pieces reaches its "=" in a later piece, so it matches the first and not
// the second, and comparing the two counts is what keeps such a write from
// going unnoticed with a truncated name read off its first piece.
var (
	cookieAssignment = regexp.MustCompile(`document\.cookie\s*=[^=]`)
	cookieWrite      = regexp.MustCompile("document\\.cookie\\s*=\\s*[`'\"]([^=`'\"]*)=")
)

// jsIdentifier is a bare name standing where a key would be, which is resolved
// to the literal it was declared with.
var jsIdentifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// TestBrowserStorageKeysAreDocumented holds the browser-storage table of the
// privacy inventory equal to what the code stores. The cookie names come from
// the Go declarations, the Web Storage keys from the client modules, and the
// table has to name each one exactly once and name nothing else.
//
// A build constraint such as yomihon_nodurable shares the cookie prefix and is
// not a cookie. It cannot reach this check: a constraint is a comment, and what
// is collected here is string literals from the syntax tree. The set equality
// below is what proves that structurally — if one ever did leak in, the set
// would differ from the declared one and this would go red rather than growing
// a row nobody wrote.
func TestBrowserStorageKeysAreDocumented(t *testing.T) {
	t.Parallel()

	cookies := declaredCookies(t)
	keys := webStorageKeys(t)
	rows := documentedStorageRows(t)

	declared := make(map[string]string, len(cookies)+len(keys))
	maps.Copy(declared, cookies)
	maps.Copy(declared, keys)

	summary := fmt.Sprintf("examined %d cookies, %d Web Storage keys, %d documented rows",
		len(cookies), len(keys), len(rows))
	t.Log(summary)

	for _, name := range slices.Sorted(maps.Keys(declared)) {
		if _, ok := rows[name]; !ok {
			t.Errorf("%s is stored in the reader's browser (%s) and has no row in the privacy inventory; %s",
				name, declared[name], summary)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(rows)) {
		if _, ok := declared[name]; !ok {
			t.Errorf("%s:%d documents %q, which nothing in this repository stores; %s",
				inventoryPath, rows[name], name, summary)
		}
	}
}

// declaredCookies is every cookie yomihon writes, taken from the Go string
// literals that name them. It also holds the two copies of the reading
// preferences equal to each other: the settings page and the page chrome each
// carry the set, and a cookie added to one and not the other is a choice the
// reader can make in one place and not see honoured in the other.
func declaredCookies(t *testing.T) map[string]string {
	t.Helper()

	found := make(map[string]string)
	perFile := make(map[string][]string)
	for _, path := range productionFiles(t, ".go") {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, path), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil || !strings.HasPrefix(value, cookiePrefix) {
				return true
			}
			if _, seen := found[value]; !seen {
				found[value] = fmt.Sprintf("%s:%d", path, fset.Position(lit.Pos()).Line)
			}
			if !slices.Contains(perFile[path], value) {
				perFile[path] = append(perFile[path], value)
			}
			return true
		})
	}
	if len(found) == 0 {
		t.Fatal("no cookie name was found in any Go source, so every check over the set passes for the wrong reason")
	}

	reading := sorted(perFile[readingPreferencesFile])
	settings := sorted(perFile[settingsPageFile])
	language := sorted(perFile[languageCookieFile])
	if len(reading) == 0 {
		t.Fatalf("%s declares no cookie, so the two copies of the reading preferences cannot be compared", readingPreferencesFile)
	}
	// The settings page reaches the language cookie through the package that
	// owns the interface's words, so that one name arrives as a reference
	// rather than as a literal and is not part of this comparison. What has to
	// match is the reading preferences the two files both spell out.
	if !slices.Equal(reading, settings) {
		t.Errorf("the reading preferences are declared twice and the copies differ:\n\t%s: %v\n\t%s: %v",
			readingPreferencesFile, reading, settingsPageFile, settings)
	}
	if len(language) != 1 {
		t.Errorf("%s names %d cookies, want exactly the interface language: %v", languageCookieFile, len(language), language)
	}

	want := sorted(append(slices.Clone(reading), language...))
	if got := slices.Sorted(maps.Keys(found)); !slices.Equal(got, want) {
		t.Errorf("cookie names in Go source = %v, want exactly the declared set %v; a name outside it is a cookie nobody declared", got, want)
	}
	return found
}

// webStorageKeys is every localStorage and sessionStorage key the client
// modules use. A key written as a literal is read from the call; a key held in
// a name is resolved from the declaration of that name in the same file. A call
// whose key is neither fails, because a key this cannot read is a key the
// inventory cannot be checked against.
func webStorageKeys(t *testing.T) map[string]string {
	t.Helper()

	if info, err := os.Stat(filepath.Join(repoRoot, mermaidDir)); err != nil || !info.IsDir() {
		t.Fatalf("%s is not a directory, so skipping the vendored diagram runtime skips nothing: %v", mermaidDir, err)
	}

	found := make(map[string]string)
	examined := 0
	for _, path := range clientModules(t) {
		data, err := os.ReadFile(filepath.Join(repoRoot, path)) // #nosec G304 -- a path this walk produced under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(data)
		examined++
		for _, m := range webStorageUse.FindAllStringSubmatchIndex(source, -1) {
			line := 1 + strings.Count(source[:m[0]], "\n")
			area := source[m[2]:m[3]]
			if m[6] == -1 {
				t.Errorf("%s:%d uses %s in a shape this check cannot read a key out of; keep it to getItem, setItem and removeItem so the privacy inventory stays checkable", path, line, area)
				continue
			}
			key, ok := resolveStorageKey(source, source[m[4]:])
			if !ok {
				t.Errorf("%s:%d stores under a key this check cannot resolve to a literal; the privacy inventory cannot be checked against it", path, line)
				continue
			}
			if _, seen := found[key]; !seen {
				found[key] = fmt.Sprintf("%s:%d", path, line)
			}
		}
		writes := cookieWrite.FindAllStringSubmatch(source, -1)
		if assignments := len(cookieAssignment.FindAllString(source, -1)); len(writes) != assignments {
			t.Errorf("%s: %d of %d writes to document.cookie name a cookie this check can read; a name assembled at run time cannot be held against the privacy inventory", path, len(writes), assignments)
		}
		for _, m := range writes {
			name := m[1]
			if !strings.HasPrefix(name, cookiePrefix) {
				t.Errorf("%s writes a cookie named %q, which is outside the %q namespace the Go declarations enumerate", path, name, cookiePrefix)
			}
		}
	}
	if examined == 0 {
		t.Fatal("no client module was read, so every check over browser storage passes for the wrong reason")
	}
	if len(found) == 0 {
		t.Fatal("no Web Storage key was found in any client module, so the check against the inventory compares nothing")
	}
	return found
}

// resolveStorageKey answers the key one call stores under. rest begins at the
// accessor, so the first parenthesis in it opens the call. A quoted first
// argument is the key; a bare name is looked up among the declarations of the
// same file.
func resolveStorageKey(source, rest string) (string, bool) {
	_, call, found := strings.Cut(rest, "(")
	if !found {
		return "", false
	}
	arg, _, ok := strings.Cut(call, ")")
	if !ok {
		return "", false
	}
	arg = strings.TrimSpace(firstArgument(arg))
	if len(arg) > 1 {
		if quote := arg[0]; (quote == '\'' || quote == '"' || quote == '`') && arg[len(arg)-1] == quote {
			body := arg[1 : len(arg)-1]
			if !strings.ContainsAny(body, "`'\"$") {
				return body, true
			}
			return "", false
		}
	}
	if !jsIdentifier.MatchString(arg) {
		return "", false
	}
	declaration := regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+` + regexp.QuoteMeta(arg) + "\\s*=\\s*['\"`]([^'\"`$]*)['\"`]\\s*;")
	m := declaration.FindStringSubmatch(source)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// firstArgument cuts an argument list at its first top-level comma.
func firstArgument(args string) string {
	depth := 0
	for i, r := range args {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				return args[:i]
			}
		}
	}
	return args
}

// clientModules lists the hand-written client modules, the vendored diagram
// runtime excluded.
func clientModules(t *testing.T) []string {
	t.Helper()

	var paths []string
	root := filepath.Join(repoRoot, webStorageRoot)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(repoRoot, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == mermaidDir {
				return fs.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(rel); ext == ".js" || ext == ".mjs" {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the client modules: %v", err)
	}
	slices.Sort(paths)
	return paths
}

// documentedStorageRows reads the browser-storage table of the privacy
// inventory and answers what it names, against the line each name sits on. The
// table's shape is matched exactly, so a malformed row fails here instead of
// disappearing from the comparison.
func documentedStorageRows(t *testing.T) map[string]int {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, inventoryPath)) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatalf("read the privacy inventory: %v", err)
	}
	document := string(data)
	if count := strings.Count(document, inventoryTableHeader+"\n"); count != 1 {
		t.Fatalf("%s: found %d browser-storage tables, want exactly one opening %q", inventoryPath, count, inventoryTableHeader)
	}
	before, section, _ := strings.Cut(document, inventoryTableHeader+"\n")
	headerLine := 1 + strings.Count(before, "\n")
	table, _, _ := strings.Cut(section, "\n\n")
	lines := strings.Split(strings.TrimRight(table, "\n"), "\n")
	if len(lines) < 2 || lines[0] != "|---|---|---|" {
		t.Fatalf("%s:%d: want a three-column separator and at least one row under the browser-storage table", inventoryPath, headerLine+1)
	}

	rows := make(map[string]int, len(lines)-1)
	for i, line := range lines[1:] {
		at := headerLine + 2 + i
		cells := strings.Split(line, " | ")
		if len(cells) != 3 || !strings.HasPrefix(cells[0], "| `") || !strings.HasSuffix(cells[0], "`") || !strings.HasSuffix(cells[2], " |") {
			t.Fatalf("%s:%d: malformed three-column row %q", inventoryPath, at, line)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(cells[0], "| `"), "`")
		if name == "" || strings.ContainsAny(name, "`|") {
			t.Fatalf("%s:%d: want a nonempty inline-code key in the first cell", inventoryPath, at)
		}
		if previous, duplicate := rows[name]; duplicate {
			t.Fatalf("%s:%d: %q is already documented at line %d", inventoryPath, at, name, previous)
		}
		rows[name] = at
	}
	if len(rows) == 0 {
		t.Fatalf("%s: the browser-storage table has no rows, so it documents nothing", inventoryPath)
	}
	return rows
}

// sorted answers a sorted copy, leaving the argument alone.
func sorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}
