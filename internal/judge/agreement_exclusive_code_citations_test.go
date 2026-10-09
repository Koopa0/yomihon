package judge_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
)

func agreementExclusiveHiddenCitationDifference(c agreementCase, f *agreementFailure, actual *agreementHTML, budget map[agreementCitation]int) (kind, authority, wrong string) {
	if c.Title != "" || len(c.Companions) != 0 || f.Property != "P1" || f.Identity != "citation-occurrences" || f.Direction != "page-only" || f.Tuple.Target == "" || f.Tuple.SourceRole != "" || f.Tuple.Section != "" || f.Tuple.State != "" || f.Cut != "" || f.Fragment != "" || f.PagePresent || f.JudgeAccepted || f.ExcerptFound || f.Multiplicity <= 0 {
		return "", "", ""
	}
	expected, found := make(map[agreementCitation]int), make(map[agreementCitation]int)
	count := 0
	for tuple, n := range budget {
		if tuple.Target == f.Tuple.Target {
			expected[tuple] = n
			count += n
		}
	}
	if count != f.Multiplicity {
		return "", "", ""
	}
	if slices.Contains(judge.LinkTargets(c.Body), f.Tuple.Target) {
		return "", "", ""
	}
	for _, tuple := range actual.Citations {
		if tuple.Target == f.Tuple.Target {
			found[tuple]++
		}
	}
	if !cmp.Equal(expected, found) {
		return "", "", ""
	}
	return "debt", "#1011 stage 5", "page"
}

func TestAgreementExclusiveCodeCitations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body string }{
		{"whole literal inventory", "`open\n[[A]] [[A#A]] [[A]] [[B|alias]]\nclose`\n"},
		{"container fence", "> ```\n> [[A]] [[A]] [[B]]\n> ```\n"},
		{"literal embed", "`open\n![[A]]\nclose`\n"},
		{"separate declarations", "`open\n[[A]]\nclose`\n\n`open\n[[B]]\nclose`\n"},
		{"remaining target", "`open\n[[A]] [[B]]\nclose`\n\n[[A]]\n"},
		{"raw target", "`open\n[[A\\]]\nclose`\n"},
		{"recorded independent markers", " `open\n[[A]]\nclose`## !\n``- > [!note] title\n%%"},
		{"recorded raw target and embed", "[[image.png]][[B|alias]]## A\n> A\n===\n\t```\n[[A\\|alias]][[#A]][[A\\]]    ## !\n![[A]]```\n`open\n[[A]]\nclose`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			budget := agreementExclusiveCodeBudget(c.Body)
			agreementAssertHiddenCitationProof(t, c, budget)
		})
	}
}

func agreementAssertHiddenCitationProof(t *testing.T, c agreementCase, budget map[agreementCitation]int) {
	t.Helper()
	r, actual := agreementIsolatedPage(t, c)
	failures := agreementPageFailures(c.Body, &r, &actual)
	targets := make(map[string]int)
	for tuple, n := range budget {
		targets[tuple.Target] += n
	}
	matched := 0
	for i := range failures {
		f := &failures[i]
		if f.Property != "P1" {
			continue
		}
		kind, authority, wrong := agreementExclusiveHiddenCitationDifference(c, f, &actual, budget)
		if targets[f.Tuple.Target] == 0 {
			if kind != "" {
				t.Fatal("caught: exclusive citation borrowed unowned target")
			}
			continue
		}
		if kind != "debt" || authority != "#1011 stage 5" || wrong != "page" {
			t.Fatalf("caught: exclusive citation public ownership %q %q %q %s", kind, authority, wrong, agreementSignature(f))
		}
		matched++
		for _, other := range []agreementCase{{Body: c.Body, Title: "title"}, {Body: c.Body, Companions: capturedBodies{"A.md": "body"}}} {
			if kind, _, _ := agreementExclusiveHiddenCitationDifference(other, f, &actual, budget); kind != "" {
				t.Fatal("caught: exclusive citation borrowed vault context")
			}
		}
		for _, change := range []func(*agreementFailure){
			func(f *agreementFailure) { f.Property = "unowned" }, func(f *agreementFailure) { f.Identity = "unowned" }, func(f *agreementFailure) { f.Direction = "unowned" }, func(f *agreementFailure) { f.Tuple.Target = "unowned" }, func(f *agreementFailure) { f.Tuple.Section = "unowned" }, func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" }, func(f *agreementFailure) { f.Tuple.State = "unowned" }, func(f *agreementFailure) { f.Multiplicity++ }, func(f *agreementFailure) { f.Fragment = "unowned" }, func(f *agreementFailure) { f.Cut = "unowned" }, func(f *agreementFailure) { f.PagePresent = true }, func(f *agreementFailure) { f.JudgeAccepted = true }, func(f *agreementFailure) { f.ExcerptFound = true },
		} {
			changed := *f
			change(&changed)
			if kind, _, _ := agreementExclusiveHiddenCitationDifference(c, &changed, &actual, budget); kind != "" {
				t.Fatalf("caught: exclusive citation borrowed signature %s", agreementSignature(&changed))
			}
		}
		for _, change := range []func(*agreementHTML, string){
			func(a *agreementHTML, target string) { a.Citations = nil },
			func(a *agreementHTML, target string) {
				for _, tuple := range a.Citations {
					if tuple.Target == target {
						a.Citations = append(a.Citations, tuple)
						break
					}
				}
			},
			func(a *agreementHTML, target string) {
				for i := range a.Citations {
					if a.Citations[i].Target == target {
						a.Citations[i].Section = "unowned"
						break
					}
				}
			},
			func(a *agreementHTML, target string) {
				for i := range a.Citations {
					if a.Citations[i].Target == target {
						a.Citations[i].State = "unowned"
						break
					}
				}
			},
			func(a *agreementHTML, target string) {
				for i := range a.Citations {
					if a.Citations[i].Target == target {
						a.Citations[i].SourceRole = "unowned"
						break
					}
				}
			},
		} {
			changed := actual
			changed.Citations = append([]agreementCitation(nil), actual.Citations...)
			change(&changed, f.Tuple.Target)
			if kind, _, _ := agreementExclusiveHiddenCitationDifference(c, f, &changed, budget); kind != "" {
				t.Fatal("caught: exclusive citation borrowed page inventory")
			}
		}
		// A stale source receipt cannot hide a live occurrence reported by check.
		changed := c
		changed.Body = "[[" + f.Tuple.Target + "]]\n"
		if slices.Contains(judge.LinkTargets(changed.Body), f.Tuple.Target) {
			if kind, _, _ := agreementExclusiveHiddenCitationDifference(changed, f, &actual, budget); kind != "" {
				t.Fatal("caught: exclusive citation borrowed check occurrence")
			}
		}
	}
	if matched != len(targets) {
		t.Fatalf("caught: exclusive citation public set got=%d want=%d", matched, len(targets))
	}
}
