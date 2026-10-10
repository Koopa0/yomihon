package judge_test

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
)

// Repeated title fields need the complete reported and rendered roster. A
// matching target alone cannot identify which literal titles own its count.
func agreementMultipleTitleBudget(body string, captions []string, findings []judge.Finding, path string, firstBodyLine int) map[string]int {
	type titleReceipt struct {
		line  int
		title string
	}
	lines := strings.Split(body, "\n")
	seen := make(map[int]bool)
	reported := make(map[string]int)
	var receipts []titleReceipt
	targets, owners := make(map[string]int), make(map[string]int)
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID != "callout.title_markup" {
			continue
		}
		if finding.Path != path || finding.Line == nil || finding.Target == nil {
			return nil
		}
		line, title := *finding.Line-firstBodyLine, *finding.Target
		if line < 0 || line >= len(lines) || seen[line] || !render.IsCalloutOpening(lines[line]) {
			return nil
		}
		opening := strings.Index(lines[line], "[!")
		closing := strings.IndexByte(lines[line][opening:], ']')
		if closing < 0 {
			return nil
		}
		remainder := lines[line][opening+closing+1:]
		if strings.HasPrefix(remainder, "-") || strings.HasPrefix(remainder, "+") {
			remainder = remainder[1:]
		}
		if strings.TrimSpace(remainder) != title {
			return nil
		}
		seen[line] = true
		reported[title]++
		receipts = append(receipts, titleReceipt{line: line, title: title})
		owned := make(map[string]int)
		for _, target := range judge.LinkTargets(title) {
			owned[target]++
		}
		for target, n := range owned {
			targets[target] += n
			owners[target]++
		}
	}
	slices.SortFunc(receipts, func(a, b titleReceipt) int { return cmp.Compare(a.line, b.line) })
	var expected, actual []string
	for _, receipt := range receipts {
		expected = append(expected, receipt.title)
	}
	for _, caption := range captions {
		if reported[caption] > 0 {
			actual = append(actual, caption)
		} else if len(judge.LinkTargets(caption)) > 0 {
			return nil
		}
	}
	if !gocmp.Equal(expected, actual) {
		return nil
	}
	selected := make(map[string]int)
	for target, n := range targets {
		if owners[target] > 1 {
			selected[target] = n
		}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func agreementMultipleTitleDifferences(t agreementTB, c agreementCase, actual *agreementHTML, failures []agreementFailure) map[string]bool {
	t.Helper()
	allowed := make(map[string]bool)
	if len(actual.CalloutTitles) < 2 || len(c.Companions) != 0 {
		return allowed
	}
	candidateOwners := make(map[string]int)
	for _, caption := range actual.CalloutTitles {
		unique := make(map[string]bool)
		for _, target := range judge.LinkTargets(caption) {
			unique[target] = true
		}
		for target := range unique {
			candidateOwners[target]++
		}
	}
	shared := false
	for _, n := range candidateOwners {
		shared = shared || n > 1
	}
	if !shared {
		return allowed
	}
	eligible := false
	for i := range failures {
		failure := &failures[i]
		eligible = eligible || failure.Property == "P1" && failure.Identity == "citation-occurrences" && failure.Direction == "judge-only"
	}
	if !eligible {
		return allowed
	}
	root := agreementAttributionRoot(t)
	const path = "Notes/Reading.md"
	agreementWrite(t, root, path, agreementEnvelope(t, c.Body))
	findings, err := agreementPublicCheck(t, root)
	if err != nil {
		t.Fatalf("multiple title public Check setup/refusal: %v", err)
	}
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID == "scan.unreadable" || finding.RuleID == "scan.skipped" {
			t.Fatalf("multiple title incomplete Check: %+v", finding)
		}
	}
	firstBodyLine := 1 + strings.Count(string(agreementEnvelope(t, "")), "\n")
	budget := agreementMultipleTitleBudget(c.Body, actual.CalloutTitles, findings, path, firstBodyLine)
	for i := range failures {
		failure := &failures[i]
		if failure.Property == "P1" && failure.Identity == "citation-occurrences" && failure.Direction == "judge-only" && failure.Tuple.SourceRole == "" && failure.Tuple.Section == "" && failure.Tuple.State == "" && failure.Fragment == "" && failure.Cut == "" && !failure.PagePresent && !failure.JudgeAccepted && !failure.ExcerptFound && failure.Multiplicity > 0 && budget[failure.Tuple.Target] == failure.Multiplicity {
			allowed[agreementSignature(failure)] = true
		}
	}
	return allowed
}

func TestAgreementMultipleTitleReceipts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		want       map[string]int
		allowed    int
	}{
		{name: "same title in two source lines", body: "> [!note] [[A]]\n> [!note] [[A]]\n", want: map[string]int{"A": 2}, allowed: 1},
		{name: "different titles share target", body: "> [!note] one [[A]]\n> [!note] two [[A]]\n", want: map[string]int{"A": 2}, allowed: 1},
		{name: "whole shared target set", body: "> [!note] [[A]] [[B]]\n> [!note] [[A]] [[A]] [[B]]\n", want: map[string]int{"A": 3, "B": 2}, allowed: 2},
		{name: "folded title retains source field", body: "> [!note]- [[A]]\n> [!note]+ [[A]]\n", want: map[string]int{"A": 2}, allowed: 1},
		{name: "ordinary body stays separate", body: "> [!note] [[A]]\n> [[A]]\n> [!note] [[A]]\n", want: map[string]int{"A": 2}, allowed: 1},
		{name: "single title scope stays separate", body: "> [!note] [[A]] [[A]]\n"},
		{name: "unrelated title targets stay separate", body: "> [!note] [[A]]\n> [!note] [[B]]\n"},
		{name: "code owns opener text", body: "```\n> [!note] [[A]]\n> [!note] [[A]]\n```\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := agreementCase{Body: tc.body}
			result, actual := agreementIsolatedPage(t, c)
			failures := agreementPageFailures(c.Body, &result, &actual)
			root := agreementAttributionRoot(t)
			const path = "Notes/Reading.md"
			agreementWrite(t, root, path, agreementEnvelope(t, c.Body))
			findings, err := agreementPublicCheck(t, root)
			if err != nil {
				t.Fatal(err)
			}
			firstBodyLine := 1 + strings.Count(string(agreementEnvelope(t, "")), "\n")
			budget := agreementMultipleTitleBudget(c.Body, actual.CalloutTitles, findings, path, firstBodyLine)
			if diff := gocmp.Diff(tc.want, budget); diff != "" {
				t.Fatalf("caught: complete multiple title receipt inventory (-want +got):\n%s", diff)
			}
			allowed := agreementMultipleTitleDifferences(t, c, &actual, failures)
			if len(allowed) != tc.allowed {
				t.Fatalf("caught: multiple title public ownership allowed=%d want=%d", len(allowed), tc.allowed)
			}
			if tc.want == nil {
				return
			}
			for _, drift := range [][]string{
				actual.CalloutTitles[:len(actual.CalloutTitles)-1],
				append(slices.Clone(actual.CalloutTitles), "[[A]]"),
			} {
				if agreementMultipleTitleBudget(c.Body, drift, findings, path, firstBodyLine) != nil {
					t.Fatal("caught: partial or extra rendered title roster borrowed ownership")
				}
			}
			if agreementMultipleTitleBudget(c.Body, actual.CalloutTitles, findings, "different.md", firstBodyLine) != nil || agreementMultipleTitleBudget(c.Body, actual.CalloutTitles, findings, path, firstBodyLine+1) != nil {
				t.Fatal("caught: title receipt borrowed a different path or source line")
			}
			changedFields := slices.Clone(findings)
			changedCaptions := slices.Clone(actual.CalloutTitles)
			for i := range changedFields {
				if changedFields[i].RuleID == "callout.title_markup" {
					changedFields[i].Target = new("unowned " + *changedFields[i].Target)
				}
			}
			for i := range changedCaptions {
				changedCaptions[i] = "unowned " + changedCaptions[i]
			}
			if agreementMultipleTitleBudget(c.Body, changedCaptions, changedFields, path, firstBodyLine) != nil {
				t.Fatal("caught: matching title rosters borrowed different source fields")
			}
			for i, finding := range findings {
				if finding.RuleID != "callout.title_markup" {
					continue
				}
				for _, change := range []func(*judge.Finding){
					func(f *judge.Finding) { f.Path = "different.md" },
					func(f *judge.Finding) { f.Line = nil },
					func(f *judge.Finding) { f.Target = nil },
					func(f *judge.Finding) { f.Line = new(*f.Line + 999) },
					func(f *judge.Finding) { f.Target = new("unowned") },
				} {
					drift := slices.Clone(findings)
					change(&drift[i])
					if agreementMultipleTitleBudget(c.Body, actual.CalloutTitles, drift, path, firstBodyLine) != nil {
						t.Fatal("caught: malformed source title receipt borrowed ownership")
					}
				}
				duplicate := append(slices.Clone(findings), finding)
				moreCaptions := append(slices.Clone(actual.CalloutTitles), *finding.Target)
				if agreementMultipleTitleBudget(c.Body, moreCaptions, duplicate, path, firstBodyLine) != nil {
					t.Fatal("caught: duplicate source line manufactured title ownership")
				}
			}
			reversed := slices.Clone(actual.CalloutTitles)
			slices.Reverse(reversed)
			if !slices.Equal(reversed, actual.CalloutTitles) && agreementMultipleTitleBudget(c.Body, reversed, findings, path, firstBodyLine) != nil {
				t.Fatal("caught: reordered title roster borrowed source ownership")
			}
			withCompanion := c
			withCompanion.Companions = capturedBodies{"A.md": "body"}
			if len(agreementMultipleTitleDifferences(t, withCompanion, &actual, failures)) != 0 {
				t.Fatal("caught: title receipt borrowed a different vault context")
			}
			for _, failure := range failures {
				if !allowed[agreementSignature(&failure)] {
					continue
				}
				for _, change := range []func(*agreementFailure){
					func(f *agreementFailure) { f.Property = "P0" },
					func(f *agreementFailure) { f.Identity = "unowned" },
					func(f *agreementFailure) { f.Multiplicity++ },
					func(f *agreementFailure) { f.Tuple.Target = "unowned" },
					func(f *agreementFailure) { f.Tuple.SourceRole = "unowned" },
					func(f *agreementFailure) { f.Tuple.Section = "unowned" },
					func(f *agreementFailure) { f.Tuple.State = "unowned" },
					func(f *agreementFailure) { f.Direction = "page-only" },
					func(f *agreementFailure) { f.Fragment = "unowned" },
					func(f *agreementFailure) { f.Cut = "unowned" },
					func(f *agreementFailure) { f.PagePresent = true },
					func(f *agreementFailure) { f.JudgeAccepted = true },
					func(f *agreementFailure) { f.ExcerptFound = true },
				} {
					changed := failure
					change(&changed)
					eligible := failure
					eligible.PagePresent = true
					eligible.Cut = "unowned"
					if got := agreementMultipleTitleDifferences(t, c, &actual, []agreementFailure{changed, eligible}); len(got) != 0 {
						t.Fatalf("caught: unrelated multiple title signature accepted: %+v", changed)
					}
				}
			}
		})
	}
}
