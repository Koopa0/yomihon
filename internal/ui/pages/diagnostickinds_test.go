package pages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// declaredDiagnosticKinds reads every DiagnosticKind the renderer declares out
// of the renderer's own source. A list written by hand here is a list a new
// kind can be left off, and being left off is the whole failure this guards
// against — the source is the only account of them that cannot fall behind.
//
// The value is what the pages switch on, and the name is what a failure
// message has to say for anyone to find the declaration again, so both are
// carried.
func declaredDiagnosticKinds(t *testing.T) map[string]string {
	t.Helper()
	const source = "../../render/render.go"
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", source, err)
	}
	kinds := make(map[string]string)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		named, ok := spec.Type.(*ast.Ident)
		if !ok || named.Name != "DiagnosticKind" {
			return true
		}
		for i, value := range spec.Values {
			basic, ok := value.(*ast.BasicLit)
			if !ok || basic.Kind != token.STRING {
				continue
			}
			text, unquoteErr := strconv.Unquote(basic.Value)
			if unquoteErr != nil {
				t.Fatalf("%s: %s has a value this test cannot read: %v", source, spec.Names[i].Name, unquoteErr)
			}
			kinds[spec.Names[i].Name] = text
		}
		return true
	})
	return kinds
}

// TestEveryDiagnosticKindHasWordsOnThePage is the lock a diagnostic kind has
// to pass before a reader ever meets it. Both places that describe one fall
// back to something honest but useless when they have no sentence for a kind —
// the note page prints the kind's own internal name, the summary calls it
// unrecognised — and neither is a compile error, so a kind added without
// wording reaches the page silently. That has happened.
//
// The enumeration comes from the renderer's source rather than from a list
// here, so adding a kind is what makes this test start asking about it.
func TestEveryDiagnosticKindHasWordsOnThePage(t *testing.T) {
	t.Parallel()

	kinds := declaredDiagnosticKinds(t)
	if len(kinds) < 5 {
		t.Fatalf("the walk found only %d kinds, so it is not reading the declarations it thinks it is", len(kinds))
	}

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		unknown := renderDiagnosticSummary(render.DiagnosticKind("a-kind-nothing-declares"), lang)
		for name, value := range kinds {
			kind := render.DiagnosticKind(value)

			if label := diagKindLabel(kind, lang); label == value {
				t.Errorf("%s (%q) has no label in %s: the page would show its internal name", name, value, lang)
			}
			if summary := renderDiagnosticSummary(kind, lang); summary == unknown {
				t.Errorf("%s (%q) has no summary in %s: the page would call it unrecognised", name, value, lang)
			}
		}
	}
}

// TestTheDiagnosticKindWalkSeesAKindItWasNotToldAbout proves the enumeration
// above is reading declarations rather than reproducing a list. A walk that
// silently found nothing would let every assertion in the test beside it pass
// while checking no kind at all.
func TestTheDiagnosticKindWalkSeesAKindItWasNotToldAbout(t *testing.T) {
	t.Parallel()

	kinds := declaredDiagnosticKinds(t)
	// Two the walk must have found: one declared long before this test and one
	// added after it, named here so a rename is a failure rather than a
	// quietly shorter list.
	for _, name := range []string{"DiagWikilinkBroken", "DiagEmbedNotExpanded"} {
		if _, found := kinds[name]; !found {
			t.Errorf("the walk did not find %s; it is reading something other than the kind declarations", name)
		}
	}
	for name, value := range kinds {
		if !strings.HasPrefix(name, "Diag") || value == "" {
			t.Errorf("the walk collected %s = %q, which is not a diagnostic kind", name, value)
		}
	}
}
