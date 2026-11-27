package judge

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/sequence"
)

// TestEveryEmittedRuleIsDeniable holds the registry honest in both directions.
// --deny takes a rule id, and a rule that fires but is not listed cannot be
// denied by name — the list drifted that way once already, silently, because
// nothing compared it against what the rules actually emit. The reverse holds
// too: a registered id that no golden exercises is either a dead entry
// accumulating silently or a live rule with no fixture behind it, so every
// registry entry must appear in at least one golden line — landing a rule
// means landing the fixture that proves it fires.
func TestEveryEmittedRuleIsDeniable(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	entries, err := os.ReadDir("testdata/golden")
	if err != nil {
		t.Fatalf("read goldens: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		data, readErr := os.ReadFile("testdata/golden/" + entry.Name())
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			_, rest, found := strings.Cut(line, `"rule_id":"`)
			if !found {
				t.Fatalf("golden line carries no rule_id: %s", line)
			}
			id, _, _ := strings.Cut(rest, `"`)
			seen[id] = struct{}{}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no rule ids were read from the goldens; the scan proved nothing")
	}
	for id := range seen {
		if !slices.Contains(ruleIDs, RuleID(id)) {
			t.Errorf("rule %q fires but --deny rejects it: the registry is missing it", id)
		}
	}
	for _, id := range ruleIDs {
		if _, emitted := seen[string(id)]; !emitted {
			t.Errorf("rule %q is registered for --deny but no golden exercises it: add a fixture that makes it fire, or drop the dead entry", id)
		}
	}
}

// TestEveryCourseRuleHasAnAction holds the advice table to the grammar: each
// rule the grammar names has a sentence of its own saying what to do next,
// rather than the general direction a rule without one falls back to. The
// table is read directly, because the finding supplies the fallback and would
// never come back empty.
func TestEveryCourseRuleHasAnAction(t *testing.T) {
	t.Parallel()

	rules := sequence.Rules()
	if len(rules) == 0 {
		t.Fatal("the grammar reports no rules at all; this check would pass over an empty set and prove nothing")
	}
	for _, rule := range rules {
		if pathRuleAction[rule] == "" {
			t.Errorf("rule %q has no suggested action", rule)
		}
	}
}

// TestAGrammarRuleTheJudgeHasNoAdviceForStillFlowsThrough holds a rule name
// the advice table has never seen to a well-formed finding rather than a
// crash: deniable by its id, carrying the grammar's own sentence about what is
// wrong, and a general direction in place of the advice nobody wrote.
func TestAGrammarRuleTheJudgeHasNoAdviceForStillFlowsThrough(t *testing.T) {
	t.Parallel()

	const invented = sequence.Rule("path.a_rule_added_after_this_table")
	if _, known := pathRuleAction[invented]; known {
		t.Fatalf("%q is in the action table, so this test proves nothing about an unknown rule", invented)
	}

	n := &note{path: "Maps/Course.md"}
	got := pathFinding(n, sequence.Diagnostic{
		Rule:     invented,
		Line:     7,
		Message:  "the grammar's own account of what is wrong",
		Evidence: "the row it read",
	})

	if got.RuleID != RuleID(invented) {
		t.Errorf("RuleID = %q, want %q", got.RuleID, invented)
	}
	if got.Message != "the grammar's own account of what is wrong" {
		t.Errorf("Message = %q, want the grammar's sentence carried through", got.Message)
	}
	if got.SuggestedAction == "" {
		t.Error("SuggestedAction is empty; a finding must always tell the author something to do")
	}
	if got.Severity != SeverityWarn {
		t.Errorf("Severity = %v, want a warning like every other course rule", got.Severity)
	}
	if !strings.HasPrefix(got.Fingerprint, "v1:") {
		t.Errorf("Fingerprint = %q, want a versioned value", got.Fingerprint)
	}
}

// TestACoursesLessonsAreFoundUnderAPartThatOnlyGroupsThem holds the reading of
// a course whose parts are headings that list nothing themselves — a level-2
// part holding a level-3 module that carries the rows — so the walk that
// collects a course's unit rows has to descend through a branch that lists
// none of its own.
//
// The discrimination is which rule speaks about the missing lesson. A row the
// walk found is the course's own listing, reconciled against disk by the map
// rule and deliberately left out of ordinary link health; a row the walk missed
// falls back to being prose, and the reader is told a link is broken instead of
// that the course promises a note nobody wrote. Both are one finding on one
// line, so a count would not tell them apart.
func TestACoursesLessonsAreFoundUnderAPartThatOnlyGroupsThem(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "Maps/Course.md",
		"---\ntype: study-path\ncreated: 2026-01-01\n---\n# Course\n\n"+
			"## Part\n\n### Module {sequence=primary}\n\n- [[L01]]\n")

	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	var rules []string
	for _, f := range findings {
		rules = append(rules, string(f.RuleID))
	}
	if diff := cmp.Diff([]string{"course.disk_mismatch"}, rules); diff != "" {
		t.Errorf("rules reported (-want +got):\n%s", diff)
	}
	if slices.Contains(rules, "link.broken") {
		t.Error("the lesson row was read as loose prose, so the part that only groups modules was never descended into")
	}
}
