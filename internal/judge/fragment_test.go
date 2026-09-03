package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The fragment rules must judge a link's "#section" and "#^block" half the
// way the reading page judges it: same fold, same generosity, same one level
// of transclusion, and silence about everything the page is silent about.
// These tests pin the agreement case by case; the golden fixture pins the
// wire bytes over a whole vault.

// fragmentRun parses the given files as a vault, resolves them, and returns
// the fragment findings alone.
func fragmentRun(t *testing.T, files map[string]string) []Finding {
	t.Helper()
	var notes []note
	var resources []string
	for path, body := range files {
		if strings.HasSuffix(path, ".md") {
			notes = append(notes, parseNote(path, []byte(body)))
		} else {
			resources = append(resources, path)
		}
	}
	return fragmentFindings(notes, buildIndex(notes, resources))
}

// ruleTargets reduces findings to "rule rule-target" strings, the identity the
// table tests compare.
func ruleTargets(findings []Finding) []string {
	var out []string
	for i := range findings {
		out = append(out, findings[i].RuleID+" "+*findings[i].Target)
	}
	return out
}

func TestFragmentSectionMatchingMirrorsTheReadingPage(t *testing.T) {
	t.Parallel()

	target := "# Top\n" +
		"## Real Section\n" +
		"words\n" +
		"## Twice\n" +
		"## Twice\n" +
		"> ## Quoted Heading\n" +
		"\n" +
		"Setext Name\n" +
		"===\n" +
		"\n" +
		"## About [[Other|Bar]]\n" +
		"## がん\n" +
		"\n" +
		"```\n" +
		"# Fenced Words\n" +
		"```\n" +
		"\n" +
		"%%\n" +
		"## Hidden Heading\n" +
		"%%\n"
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a missing section is reported", body: "[[Target#Nowhere]]\n", want: []string{"link.section_missing Target#Nowhere"}},
		{name: "an existing section is not", body: "[[Target#Real Section]]\n"},
		{name: "case folds the way the page's ids fold", body: "[[Target#REAL SECTION]]\n"},
		{name: "punctuation folds the way the page's ids fold", body: "[[Target#real, section!]]\n"},
		{name: "any one of a repeated heading answers", body: "[[Target#Twice]]\n"},
		{name: "a heading inside a quote answers", body: "[[Target#Quoted Heading]]\n"},
		{name: "an underlined heading answers", body: "[[Target#Setext Name]]\n"},
		{name: "a heading citing a note answers by its display words", body: "[[Target#About Bar]]\n"},
		{name: "the display words are the name, not the bracket text", body: "[[Target#About Other Bar]]\n", want: []string{"link.section_missing Target#About Other Bar"}},
		{name: "a decomposed spelling reaches a composed heading", body: "[[Target#がん]]\n"},
		{name: "a heading shape inside a fence stays out of reports", body: "[[Target#Fenced Words]]\n"},
		{name: "a heading hidden in a comment is not on the page", body: "[[Target#Hidden Heading]]\n", want: []string{"link.section_missing Target#Hidden Heading"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ruleTargets(fragmentRun(t, map[string]string{
				"Notes/Target.md": target,
				"Notes/Other.md":  "other\n",
				"Notes/Citer.md":  tt.body,
			}))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFragmentBlockMatchingMirrorsTheReadingPage(t *testing.T) {
	t.Parallel()

	target := "## Real Section\n" +
		"\n" +
		"some words ^blk1\n" +
		"\n" +
		"| cell | row | ^tabled\n" +
		"\n" +
		"```\n" +
		"code ^fenced\n" +
		"```\n" +
		"\n" +
		"> quoted words ^quoted\n"
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a missing block is reported", body: "[[Target#^ghost]]\n", want: []string{"link.block_missing Target#^ghost"}},
		{name: "an existing address is not", body: "[[Target#^blk1]]\n"},
		{name: "the bare caret spelling reads the same address", body: "[[Target^blk1]]\n"},
		{name: "the address folds case like every fragment", body: "[[Target#^BLK1]]\n"},
		{name: "an address at the end of a quoted line answers", body: "[[Target#^quoted]]\n"},
		{name: "an address on a table row is not an anchor", body: "[[Target#^tabled]]\n", want: []string{"link.block_missing Target#^tabled"}},
		{name: "an address inside a fence is code", body: "[[Target#^fenced]]\n", want: []string{"link.block_missing Target#^fenced"}},
		{name: "a block beats a section when both are written", body: "[[Target^ghost2#Real Section]]\n", want: []string{"link.block_missing Target^ghost2#Real Section"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ruleTargets(fragmentRun(t, map[string]string{
				"Notes/Target.md": target,
				"Notes/Citer.md":  tt.body,
			}))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFragmentRulesJudgeOnlyUniquelyResolvedNoteLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name: "a broken name is the broken-link rule's alone",
			files: map[string]string{
				"Notes/Citer.md": "[[Ghost#Section]]\n",
			},
		},
		{
			name: "an ambiguous name is the collision rules' alone",
			files: map[string]string{
				"Notes/Citer.md": "[[Dup#Section]]\n",
				"Notes/Dup.md":   "a\n",
				"Other/Dup.md":   "b\n",
			},
		},
		{
			name: "a picture has no sections",
			files: map[string]string{
				"Notes/Citer.md": "[[shot.png#Section]]\n",
				"Notes/shot.png": "",
			},
		},
		{
			name: "a same-file fragment is never resolved at all",
			files: map[string]string{
				"Notes/Citer.md": "[[#Nowhere]]\n",
			},
		},
		{
			name: "a transclusion is out of these rules' scope",
			files: map[string]string{
				"Notes/Citer.md":  "![[Target#Nowhere]]\n",
				"Notes/Target.md": "words\n",
			},
		},
		{
			name: "a fragment on a plain link still reports beside those",
			files: map[string]string{
				"Notes/Citer.md":  "[[Target#Nowhere]]\n",
				"Notes/Target.md": "words\n",
			},
			want: []string{"link.section_missing Target#Nowhere"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ruleTargets(fragmentRun(t, tt.files))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestATranscludedNoteAnswersForItsHostsSections(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"Notes/Citer.md":  "[[Host#Inner Heading]]\n[[Host#Deeper Heading]]\n",
		"Notes/Host.md":   "![[Inner]]\n",
		"Notes/Inner.md":  "## Inner Heading\n![[Deeper]]\n",
		"Notes/Deeper.md": "## Deeper Heading\n",
	}
	got := ruleTargets(fragmentRun(t, files))
	// The page expands one level, so Inner's headings are on Host's page and
	// Deeper's are not: a citation into the first stays quiet and a citation
	// into the second is a real miss.
	want := []string{"link.section_missing Host#Deeper Heading"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
	}
}

func TestFragmentFindingCarriesTheWireShape(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"Notes/Citer.md":  "intro\n\n[[Target#Nowhere|shown words]]\n[[Target#^ghost]]\n",
		"Notes/Target.md": "words\n",
	}
	findings := fragmentRun(t, files)
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2:\n%+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != SeverityWarn {
			t.Errorf("%s Severity = %v, want SeverityWarn", f.RuleID, f.Severity)
		}
		if f.SourceRule != sourceYomihon {
			t.Errorf("%s SourceRule = %q, want %q", f.RuleID, f.SourceRule, sourceYomihon)
		}
		if f.Path != "Notes/Citer.md" {
			t.Errorf("%s Path = %q, want the citing note", f.RuleID, f.Path)
		}
		if f.ResolvedTo == nil || *f.ResolvedTo != "Notes/Target.md" {
			t.Errorf("%s ResolvedTo = %v, want the resolved note", f.RuleID, f.ResolvedTo)
		}
		if f.Line == nil {
			t.Errorf("%s Line is nil, want the link's line", f.RuleID)
		}
		if f.Evidence == "" || f.SuggestedAction == "" || f.Message == "" {
			t.Errorf("%s is missing prose: %+v", f.RuleID, f)
		}
		if !strings.HasPrefix(f.Fingerprint, "v1:") {
			t.Errorf("%s Fingerprint = %q, want a versioned value", f.RuleID, f.Fingerprint)
		}
	}
	section, block := findings[0], findings[1]
	if section.RuleID != "link.section_missing" || *section.Target != "Target#Nowhere" {
		t.Errorf("section finding = %s %q, want link.section_missing on the address without the display alias", section.RuleID, *section.Target)
	}
	if *section.Line != 3 {
		t.Errorf("section Line = %d, want 3", *section.Line)
	}
	if block.RuleID != "link.block_missing" || *block.Target != "Target#^ghost" {
		t.Errorf("block finding = %s %q, want link.block_missing on the written address", block.RuleID, *block.Target)
	}
	if section.Fingerprint == block.Fingerprint {
		t.Error("the two findings share a fingerprint; the address must separate them")
	}
}

// TestRunCheckDeniesFragmentRules proves the new rules gate end to end: a
// deny token naming either one turns the fixture's findings into exit 1, so
// an operator who asks for the gate really gets it.
func TestRunCheckDeniesFragmentRules(t *testing.T) {
	t.Parallel()

	for _, ruleID := range []string{"link.section_missing", "link.block_missing"} {
		t.Run(ruleID, func(t *testing.T) {
			t.Parallel()

			root := judgeFixtureRoot(t, "testdata/vault-fragments")
			_, exit, err := RunCheck(t.Context(), &CheckOptions{
				Root:   root,
				Format: FormatJSON,
				Deny:   []string{ruleID},
			})
			if err != nil {
				t.Fatalf("RunCheck(--deny %q) error = %v", ruleID, err)
			}
			if exit != 1 {
				t.Errorf("RunCheck(--deny %q) exit = %d, want 1", ruleID, exit)
			}
		})
	}
}

// TestExtractKeepsTheFragmentBesideTheTarget pins the extraction half: the
// resolution target stays exactly what it always was, and the fragment
// arrives beside it split the way the resolver splits it.
func TestExtractKeepsTheFragmentBesideTheTarget(t *testing.T) {
	t.Parallel()

	links := extractWikilinks("[[Note#Sec|words]] and [[Note#^blk]] and ![[Note#Part]]\n", 1)
	want := []wikiLink{
		{target: "Note", address: "Note#Sec", heading: "Sec", offset: 0, line: 1},
		{target: "Note", address: "Note#^blk", block: "blk", offset: 23, line: 1},
		{target: "Note", address: "Note#Part", heading: "Part", embed: true, offset: 42, line: 1},
	}
	if diff := cmp.Diff(want, links, cmp.AllowUnexported(wikiLink{})); diff != "" {
		t.Errorf("extractWikilinks mismatch (-want +got):\n%s", diff)
	}
}
