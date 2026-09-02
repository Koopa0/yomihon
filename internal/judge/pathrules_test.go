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
		if !slices.Contains(ruleIDs, id) {
			t.Errorf("rule %q fires but --deny rejects it: the registry is missing it", id)
		}
	}
	for _, id := range ruleIDs {
		if _, emitted := seen[id]; !emitted {
			t.Errorf("rule %q is registered for --deny but no golden exercises it: add a fixture that makes it fire, or drop the dead entry", id)
		}
	}
}

// TestEveryStudyPathRuleHasAnAction is the other direction: the grammar owns
// the rule names, and this face owes each of them a sentence saying what to do
// next. The set is asked of the grammar rather than written out here, because a
// list written by hand and checked against a list written by hand agrees with
// itself and with nothing else — which is how the registry drifted before.
//
// A rule with no advice is no longer fatal, so this is what keeps the advice
// complete: without it, the author of a note tripping the newest rule would be
// told what is wrong and left with no sentence saying what to do about it.
func TestEveryStudyPathRuleHasAnAction(t *testing.T) {
	t.Parallel()

	rules := sequence.Rules()
	if len(rules) == 0 {
		t.Fatal("the grammar reports no rules at all; this check would pass over an empty set and prove nothing")
	}
	for _, rule := range rules {
		if pathRuleAction[rule] == "" {
			t.Errorf("rule %q has no suggested action", rule)
		}
		if !slices.Contains(ruleIDs, string(rule)) {
			t.Errorf("rule %q is not in the --deny registry", rule)
		}
	}
}

// TestARuleThisFaceHasNoAdviceForIsStillReported pins the answer to the case
// the completeness check above cannot cover: a rule name arriving from the
// grammar that this table has never seen. The two tables are derived from the
// grammar now, so the only way to reach it is a rule value built somewhere
// else — which is exactly what a check run must not die on. The author gets the
// grammar's own sentence about what is wrong, and an empty line where the
// advice would be, because inventing advice for a rule nobody here understands
// would be worse than admitting there is none.
func TestARuleThisFaceHasNoAdviceForIsStillReported(t *testing.T) {
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

	if got.RuleID != string(invented) {
		t.Errorf("RuleID = %q, want %q", got.RuleID, invented)
	}
	if got.Message != "the grammar's own account of what is wrong" {
		t.Errorf("Message = %q, want the grammar's sentence carried through", got.Message)
	}
	if got.SuggestedAction != "" {
		t.Errorf("SuggestedAction = %q, want none; no advice was written for this rule", got.SuggestedAction)
	}
	if got.Severity != SeverityWarn {
		t.Errorf("Severity = %v, want a warning like every other study-path rule", got.Severity)
	}
}

// TestACoursesLessonsAreFoundUnderAPartThatOnlyGroupsThem holds the reading of
// a course whose parts are headings that list nothing themselves. Both of this
// vault's real courses are written that way — a level-2 part holding a level-3
// module that carries the rows — so the walk that collects a course's lesson
// rows has to descend through a branch that lists none of its own.
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
		"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+
			"## Part\n\n### Module {sequence=primary}\n\n- [[L01]]\n")

	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	var rules []string
	for _, f := range findings {
		rules = append(rules, f.RuleID)
	}
	if diff := cmp.Diff([]string{"map.disk_mismatch"}, rules); diff != "" {
		t.Errorf("rules reported (-want +got):\n%s", diff)
	}
	if slices.Contains(rules, "link.broken") {
		t.Error("the lesson row was read as loose prose, so the part that only groups modules was never descended into")
	}
}
