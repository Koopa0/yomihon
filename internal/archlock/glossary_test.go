package archlock

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// glossaryFile decides the words the product writes on screen. Two fenced
// blocks in it, retired-en and retired-zh, list a word and the word that
// replaces it, one `retired => replacement` to a line.
const glossaryFile = "internal/wording/glossary.md"

// glossaryDebtFile says, for each wording file and retired word, how many uses
// the screen text still has. A line is `file<TAB>word<TAB>count`. It exists so
// that a retired word cannot come back while the old uses are being replaced;
// it is not a list of words that are fine.
const glossaryDebtFile = "testdata/glossary-debt.txt"

// retiredWord is one line of a retired block.
type retiredWord struct {
	word, use string
	zh        bool
}

// debtKey names one retired word in one wording file.
type debtKey struct{ file, word string }

// TestScreenTextUsesTheGlossaryWords keeps the screen text on the words
// internal/wording/glossary.md decides. Two ideas called by two words is the drift that
// makes a reader wonder whether they are two ideas, and it happens in the quiet
// direction: each string reads fine alone.
//
// It reads the syntax rather than the text, because the bilingual strings are
// what a reader sees and a comment or an identifier is not: `VaultRoot` is a
// name the code is free to keep while its screen text says "Vault root". It
// reads every both(zh, en) call in internal/wording, whose two parameters the
// compiler guarantees, and compares the Chinese with the Chinese words and the
// English with the English ones.
//
// What it does not reach, said here so nobody reads a green run as more than it
// is. It reads only the wording: documentation, the command line's help and
// the pages' own template text are checked by a person. It matches a retired
// word and its simple plural, so an irregular plural needs a line of its own in
// the glossary. And it cannot tell a retired word used in the retired sense
// from the same word used for something else, which is why a word that is
// ordinary English elsewhere, such as the verb judge, is not retired here.
func TestScreenTextUsesTheGlossaryWords(t *testing.T) {
	t.Parallel()

	retired := retiredWords(t)
	allowed := glossaryDebt(t)
	uses, scanned := screenTextUses(t, retired)
	if scanned < 500 {
		t.Fatalf("only %d both(zh, en) calls were read, so this check read almost none of the screen text", scanned)
	}

	known := make(map[string]retiredWord, len(retired))
	for _, r := range retired {
		known[r.word] = r
	}
	keys := make(map[debtKey]bool)
	for k := range uses {
		keys[k] = true
	}
	for k := range allowed {
		keys[k] = true
	}
	ordered := make([]debtKey, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	slices.SortFunc(ordered, func(a, b debtKey) int {
		return strings.Compare(a.file+"\x00"+a.word, b.file+"\x00"+b.word)
	})

	for _, k := range ordered {
		r, isRetired := known[k.word]
		if !isRetired {
			t.Errorf("%s allows %q in %s, but %s no longer retires it; remove the line", glossaryDebtFile, k.word, k.file, glossaryFile)
			continue
		}
		got, want := len(uses[k]), allowed[k]
		switch {
		case got > want:
			report(t, fmt.Sprintf("uses %q, which %s retires; write %q (%d use(s) in this file, %d allowed)", k.word, glossaryFile, r.use, got, want), uses[k])
		case got < want:
			t.Errorf("%s allows %d use(s) of %q in %s but the screen text has %d; lower the allowance", glossaryDebtFile, want, k.word, k.file, got)
		}
	}
	if t.Failed() {
		t.Logf("the counts as they stand, in the form %s takes:\n%s", glossaryDebtFile, debtLines(uses))
	}
}

// retiredWords reads the two retired blocks out of the glossary.
func retiredWords(t *testing.T) []retiredWord {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, glossaryFile)) // #nosec G304 -- fixed repository source
	if err != nil {
		t.Fatalf("read %s: %v", glossaryFile, err)
	}
	var words []retiredWord
	block := ""
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if info, isFence := strings.CutPrefix(line, "```"); isFence {
			switch {
			case block != "":
				block = ""
			case info == "":
				block = "other"
			default:
				block = info
			}
			continue
		}
		if block != "retired-en" && block != "retired-zh" || line == "" {
			continue
		}
		word, use, found := strings.Cut(line, " => ")
		word, use = strings.TrimSpace(word), strings.TrimSpace(use)
		if !found || word == "" || use == "" {
			t.Fatalf("%s:%d is not `retired => replacement`: %q", glossaryFile, i+1, line)
		}
		words = append(words, retiredWord{word: word, use: use, zh: block == "retired-zh"})
	}
	var en, zh int
	for _, w := range words {
		if w.zh {
			zh++
		} else {
			en++
		}
	}
	if en == 0 || zh == 0 {
		t.Fatalf("%s names %d English and %d Chinese retired words, so this check would compare against nothing", glossaryFile, en, zh)
	}
	return words
}

// glossaryDebt reads the allowances.
func glossaryDebt(t *testing.T) map[debtKey]int {
	t.Helper()

	data, err := os.ReadFile(glossaryDebtFile) // #nosec G304 -- fixed repository source
	if err != nil {
		t.Fatalf("read %s: %v", glossaryDebtFile, err)
	}
	allowed := make(map[debtKey]int)
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		count, convErr := 0, error(nil)
		if len(fields) == 3 {
			count, convErr = strconv.Atoi(fields[2])
		}
		if len(fields) != 3 || convErr != nil || count < 1 {
			t.Fatalf("%s:%d is not `file<TAB>word<TAB>count` with a count of one or more: %q", glossaryDebtFile, i+1, line)
		}
		k := debtKey{file: fields[0], word: fields[1]}
		if _, dup := allowed[k]; dup {
			t.Fatalf("%s:%d repeats %q for %s", glossaryDebtFile, i+1, k.word, k.file)
		}
		allowed[k] = count
	}
	return allowed
}

// screenTextUses finds every use of a retired word in the bilingual strings of
// internal/wording, and says how many both(zh, en) calls it read.
func screenTextUses(t *testing.T, retired []retiredWord) (uses map[debtKey][]site, scanned int) {
	t.Helper()

	patterns := make(map[string]*regexp.Regexp)
	for _, r := range retired {
		if !r.zh {
			patterns[r.word] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(r.word) + `(?:'s|s|es)?\b`)
		}
	}
	files, err := filepath.Glob(filepath.Join(repoRoot, "internal", "wording", "*.go"))
	if err != nil {
		t.Fatalf("list the wording files: %v", err)
	}
	uses = make(map[debtKey][]site)
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		base := filepath.Base(path)
		ast.Inspect(file, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}
			name, isName := call.Fun.(*ast.Ident)
			if !isName || name.Name != "both" || len(call.Args) != 2 {
				return true
			}
			zh, zhOK := constantString(call.Args[0])
			en, enOK := constantString(call.Args[1])
			pos := fset.Position(call.Pos())
			if !zhOK || !enOK {
				t.Fatalf("%s:%d: a both(zh, en) call is not made of string constants, so its words cannot be read", base, pos.Line)
			}
			scanned++
			for _, r := range retired {
				k := debtKey{file: base, word: r.word}
				if r.zh {
					for range strings.Count(zh, r.word) {
						uses[k] = append(uses[k], site{path: "internal/wording/" + base, line: pos.Line, text: r.word + " in " + strconv.Quote(zh)})
					}
					continue
				}
				for range patterns[r.word].FindAllString(en, -1) {
					uses[k] = append(uses[k], site{path: "internal/wording/" + base, line: pos.Line, text: r.word + " in " + strconv.Quote(en)})
				}
			}
			return true
		})
	}
	return uses, scanned
}

// constantString reads an expression made only of string literals joined by +.
func constantString(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.ParenExpr:
		return constantString(v.X)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		left, leftOK := constantString(v.X)
		right, rightOK := constantString(v.Y)
		return left + right, leftOK && rightOK
	}
	return "", false
}

// debtLines renders the counts in the form the debt file takes.
func debtLines(uses map[debtKey][]site) string {
	var lines []string
	for k, sites := range uses {
		if len(sites) > 0 {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%d", k.file, k.word, len(sites)))
		}
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}
