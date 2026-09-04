package wording

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var verb = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z]`)

// pair is one phrase as it is written, named by the variable it is assigned to.
type pair struct {
	name   string
	zhHant string
	en     string
}

// TestBothLanguagesTakeTheSameValues holds every phrase that carries a value to
// the same shape in both languages. A phrase whose two sides disagree about how
// many values they take, or about what kind, formats correctly in one language
// and prints a formatting error into the page in the other — for the reader who
// chose that language, and only for them, which is the reader least likely to
// be looked at.
func TestBothLanguagesTakeTheSameValues(t *testing.T) {
	t.Parallel()
	phrases := writtenPhrases(t)
	carrying := 0
	for _, p := range phrases {
		zh, en := takes(p.zhHant), takes(p.en)
		if len(zh) > 0 {
			carrying++
		}
		if !reflect.DeepEqual(zh, en) {
			t.Errorf("%s takes %v in Traditional Chinese and %v in English; one of them prints a formatting error into the page", p.name, zh, en)
		}
	}
	if carrying == 0 {
		t.Fatal("no phrase carries a value, so the comparison above never ran on one")
	}
	t.Logf("%d phrases, %d of them carrying a value", len(phrases), carrying)
}

// TestNoPhraseIsHalfWritten catches the one thing the constructor cannot. Two
// arguments are what the compiler counts; whether both hold words is not.
func TestNoPhraseIsHalfWritten(t *testing.T) {
	t.Parallel()
	for _, p := range writtenPhrases(t) {
		if strings.TrimSpace(p.zhHant) == "" && strings.TrimSpace(p.en) != "" {
			t.Errorf("%s has no Traditional Chinese", p.name)
		}
		if strings.TrimSpace(p.en) == "" && strings.TrimSpace(p.zhHant) != "" {
			t.Errorf("%s has no English", p.name)
		}
	}
}

// writtenPhrases reads every phrase this package declares out of its own
// source. A list written by hand is a list a new phrase can be left off, and
// nothing would say so; the source is the only account that cannot be.
func writtenPhrases(t *testing.T) []pair {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the wording package's own directory: %v", err)
	}
	var out []pair
	read := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		read++
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, value := range spec.Values {
				call, ok := value.(*ast.CallExpr)
				if !ok {
					continue
				}
				if fn, ok := call.Fun.(*ast.Ident); !ok || fn.Name != "both" || len(call.Args) != 2 {
					continue
				}
				zh, zhOK := literal(call.Args[0])
				en, enOK := literal(call.Args[1])
				if !zhOK || !enOK {
					t.Errorf("%s is built from something other than two written strings, which this check cannot read", spec.Names[i].Name)
					continue
				}
				out = append(out, pair{name: spec.Names[i].Name, zhHant: zh, en: en})
			}
			return true
		})
	}
	if read < 2 {
		t.Fatalf("only %d source files were read, so this walked almost nothing", read)
	}
	if len(out) < 100 {
		t.Fatalf("only %d phrases were found in this package's source; the reader below cannot have walked it", len(out))
	}
	return out
}

// takes reads the kinds of value a phrase's verbs consume rather than their
// spelling. A value the sentence quotes in one language and the verb quotes in
// the other is the same value either way: %q and %s both take a string, and
// holding them to the same letter reports a difference that is not there.
func takes(format string) []string {
	var kinds []string
	for _, v := range verb.FindAllString(format, -1) {
		switch v[len(v)-1] {
		case 's', 'q', 'v':
			kinds = append(kinds, "string")
		case 'd':
			kinds = append(kinds, "int")
		case 'f', 'e', 'g':
			kinds = append(kinds, "float")
		default:
			kinds = append(kinds, v)
		}
	}
	return kinds
}

func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// Known is the refusing half of the pair, so what matters is which values it
// turns away. FromCookieValue beside it answers with the default for the same
// inputs, and a reader who lands on one when they asked for the other gets
// either a page in a language they did not choose or a receipt for a change
// nothing made.
func TestKnownAcceptsOnlyTheTwoSpokenLanguages(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		value string
		want  Lang
		known bool
	}{
		{"the default", "zh-Hant", ZhHant, true},
		{"the other one", "en", En, true},
		{"a language yomihon does not speak", "ja", ZhHant, false},
		{"a broader tag for one it does", "zh", ZhHant, false},
		{"a narrower tag for one it does", "en-GB", ZhHant, false},
		{"different case", "EN", ZhHant, false},
		{"nothing at all", "", ZhHant, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, known := Known(tt.value)
			if known != tt.known {
				t.Errorf("Known(%q) known = %v, want %v", tt.value, known, tt.known)
			}
			if got != tt.want {
				t.Errorf("Known(%q) = %q, want %q", tt.value, got, tt.want)
			}
			if normalised := FromCookieValue(tt.value); !tt.known && normalised != ZhHant {
				t.Errorf("FromCookieValue(%q) = %q; the normalising half must still fall to the default", tt.value, normalised)
			}
		})
	}
}

// Other is the complement of a two-value set, so it has to be a complement:
// never the language it was given, and its own inverse.
func TestOtherIsTheLanguageThisOneIsNot(t *testing.T) {
	t.Parallel()

	for _, lang := range []Lang{ZhHant, En} {
		if other := lang.Other(); other == lang {
			t.Errorf("%q.Other() = %q, which is the language it was asked about", lang, other)
		}
		if back := lang.Other().Other(); back != lang {
			t.Errorf("%q.Other().Other() = %q, want %q", lang, back, lang)
		}
	}
}
