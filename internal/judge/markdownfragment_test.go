package judge

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// fragmentRuleFindings checks a vault laid out the way the other Markdown tests
// lay one out and keeps the section and block findings, as "rule path target".
func fragmentRuleFindings(t *testing.T, files map[string]string, withheld ...string) []string {
	t.Helper()
	return fragmentRuleFindingsSealed(t, files, withheld, nil)
}

// fragmentRuleFindingsSealed is fragmentRuleFindings with some of the vault's
// files made unreadable before the check runs.
func fragmentRuleFindingsSealed(t *testing.T, files map[string]string, withheld, sealed []string) []string {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		write(t, root, path, body)
	}
	writeTestContract(t, root, withheld)
	for _, path := range sealed {
		unreadable(t, filepath.Join(root, filepath.FromSlash(path)))
	}
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var got []string
	for i := range findings {
		if rule := findings[i].RuleID; rule == "link.section_missing" || rule == "link.block_missing" {
			got = append(got, string(rule)+" "+findings[i].Path+" "+*findings[i].Target)
		}
	}
	slices.Sort(got)
	return got
}

func TestMarkdownLinkFragmentThatReachesNothingIsReported(t *testing.T) {
	t.Parallel()
	got := fragmentRuleFindings(t, map[string]string{
		"Concepts/golang/Target.md": "# Target\n\n## Real\n\ntext ^kept\n",
		"Concepts/golang/Source.md": "[a](Target.md#real) [b](Target.md#nope) [c](Target.md#^kept) [d](Target.md#^lost)\n\n[e](#gone)\n",
	})
	want := []string{
		"link.block_missing Concepts/golang/Source.md Target.md#^lost",
		"link.section_missing Concepts/golang/Source.md #gone",
		"link.section_missing Concepts/golang/Source.md Target.md#nope",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fragment findings (-want +got):\n%s", diff)
	}
}

// The check asks the page's lookup, so every spelling the page follows is quiet:
// the heading's words, GitHub's anchor, and the id a repeated heading is stamped.
func TestMarkdownLinkFragmentTheReadingPageFollowsIsQuiet(t *testing.T) {
	t.Parallel()
	got := fragmentRuleFindings(t, map[string]string{
		"Concepts/golang/Target.md": "## What's new?\n\n## Intro\n\n## Intro\n\n## Intro\n",
		"Concepts/golang/Source.md": "[a](Target.md#whats-new) [b](Target.md#What%27s%20new%3F) [c](Target.md#intro-2) [d](Target.md#intro-3) [e](Target.md#intro-1) [f](Target.md#top) [g](Target.md#)\n\n## Here\n\n[h](#here) [i](#top)\n",
	})
	if len(got) != 0 {
		t.Errorf("a fragment the page follows was reported: %v", got)
	}
}

func TestMarkdownLinkFragmentOfANoteTheContractWithholdsIsNotJudged(t *testing.T) {
	t.Parallel()
	got := fragmentRuleFindings(t, map[string]string{
		"Restricted/Target.md":      "## Real\n",
		"Restricted/Source.md":      "[a](Target.md#nope) [b](#gone)\n",
		"Concepts/golang/Source.md": "[c](../../Restricted/Target.md#nope)\n",
	}, "Restricted")
	if len(got) != 0 {
		t.Errorf("withheld ground was written about: %v", got)
	}
}

// A link whose file is missing is the broken-path rule's to report, and one to
// something that is not a note has no sections.
func TestMarkdownLinkFragmentOfAMissingOrNonNoteTargetIsLeftToOtherRules(t *testing.T) {
	t.Parallel()
	got := fragmentRuleFindings(t, map[string]string{
		"Concepts/golang/Source.md":   "[a](Nothing.md#part) [b](picture.png#page=2)\n",
		"Concepts/golang/picture.png": "not really a picture",
	})
	if len(got) != 0 {
		t.Errorf("fragment findings = %v, want none", got)
	}
}

// A note's embeds are drawn into it, so a heading or a block address its own
// body lacks may be one of theirs. Where something it embeds, at any depth, is
// withheld by the contract or could not be read, saying the address is missing
// would say something about words this run may not write about or never saw.
func TestMarkdownLinkFragmentThroughAnEmbedOfGroundNobodyMayReadIsNotJudged(t *testing.T) {
	t.Parallel()
	links := "[a](Relay.md#Absent) [b](Relay.md#^absent)\n"
	tests := []struct {
		name     string
		files    map[string]string
		withheld []string
		sealed   []string
	}{
		{
			name: "a withheld note it embeds",
			files: map[string]string{
				"Restricted/Sealed.md":     "## Absent\n",
				"Concepts/golang/Relay.md": "![[Sealed]]\n",
				"Concepts/golang/Cite.md":  links,
			},
			withheld: []string{"Restricted"},
		},
		{
			name: "a withheld note a note it embeds embeds",
			files: map[string]string{
				"Restricted/Sealed.md":     "## Absent\n",
				"Concepts/golang/Mid.md":   "![[Sealed]]\n",
				"Concepts/golang/Relay.md": "![[Mid]]\n",
				"Concepts/golang/Cite.md":  links,
			},
			withheld: []string{"Restricted"},
		},
		{
			name: "an unreadable note it embeds",
			files: map[string]string{
				"Concepts/golang/Sealed.md": "## Absent\n",
				"Concepts/golang/Relay.md":  "![[Sealed]]\n",
				"Concepts/golang/Cite.md":   links,
			},
			sealed: []string{"Concepts/golang/Sealed.md"},
		},
		{
			name: "an unreadable note a note it embeds embeds",
			files: map[string]string{
				"Concepts/golang/Sealed.md": "## Absent\n",
				"Concepts/golang/Mid.md":    "![[Sealed]]\n",
				"Concepts/golang/Relay.md":  "![[Mid]]\n",
				"Concepts/golang/Cite.md":   links,
			},
			sealed: []string{"Concepts/golang/Sealed.md"},
		},
		{
			name: "a link to a place in its own note that embeds a withheld note",
			files: map[string]string{
				"Restricted/Sealed.md":     "## Absent\n",
				"Concepts/golang/Relay.md": "![[Sealed]]\n\n[a](#Absent) [b](#^absent)\n",
			},
			withheld: []string{"Restricted"},
		},
		{
			name: "an unreadable note in a ring of embeds",
			files: map[string]string{
				"Concepts/golang/Sealed.md": "## Absent\n",
				"Concepts/golang/Mid.md":    "![[Relay]] ![[Sealed]]\n",
				"Concepts/golang/Relay.md":  "![[Mid]]\n",
				"Concepts/golang/Cite.md":   links,
			},
			sealed: []string{"Concepts/golang/Sealed.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fragmentRuleFindingsSealed(t, tt.files, tt.withheld, tt.sealed); len(got) != 0 {
				t.Errorf("an address was called missing where unseen ground is embedded: %v", got)
			}
		})
	}
}

// What is embedded and fully readable is still judged, in a ring of embeds as
// well, and the page's own reading of an embedded note's headings counts.
func TestMarkdownLinkFragmentThroughAnEmbedOfReadableGroundIsStillJudged(t *testing.T) {
	t.Parallel()
	got := fragmentRuleFindings(t, map[string]string{
		"Concepts/golang/Mid.md":   "![[Relay]]\n\n## Inside\n",
		"Concepts/golang/Relay.md": "![[Mid]]\n",
		"Concepts/golang/Cite.md":  "[a](Relay.md#Absent) [b](Relay.md#^absent)\n",
	})
	want := []string{
		"link.block_missing Concepts/golang/Cite.md Relay.md#^absent",
		"link.section_missing Concepts/golang/Cite.md Relay.md#Absent",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fragment findings (-want +got):\n%s", diff)
	}
}
