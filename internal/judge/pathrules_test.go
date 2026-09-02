package judge

import (
	"os"
	"slices"
	"strings"
	"testing"

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
// the rule names, and a rule with no tailored advice would reach the author
// with only the generic fallback sentence. The enumeration is the grammar's
// own, not a copy kept here, so a rule added to the grammar is checked by this
// test without anyone editing it.
func TestEveryStudyPathRuleHasAnAction(t *testing.T) {
	t.Parallel()

	rules := sequence.Rules()
	if len(rules) == 0 {
		t.Fatal("sequence.Rules() is empty; the checks below would pass over nothing")
	}
	for _, rule := range rules {
		if pathRuleAction[rule] == "" {
			t.Errorf("rule %q has no suggested action", rule)
		}
		if !slices.Contains(ruleIDs, rule) {
			t.Errorf("rule %q is not in the --deny registry", rule)
		}
	}
}

// TestAGrammarRuleTheJudgeHasNoAdviceForStillFlowsThrough pins the judge's
// side of the vocabulary boundary: the grammar owns the rule names, so a rule
// it reports that this package has no tailored advice for must still become a
// well-formed finding — deniable by its id, carrying the grammar's own message
// and a generic action — rather than stopping the whole check run. The judge
// used to panic here, which turned one new grammar rule into a crash on an
// ordinary vault.
func TestAGrammarRuleTheJudgeHasNoAdviceForStillFlowsThrough(t *testing.T) {
	t.Parallel()

	n := &note{path: "Maps/course.md"}
	d := sequence.Diagnostic{
		Rule:     "path.test_unknown",
		Line:     3,
		Message:  "the grammar found something new",
		Evidence: "a row the grammar refused",
	}
	f := pathFinding(n, d)
	if f.RuleID != d.Rule {
		t.Errorf("RuleID = %q, want %q", f.RuleID, d.Rule)
	}
	if f.Severity != SeverityWarn {
		t.Errorf("Severity = %v, want SeverityWarn", f.Severity)
	}
	if f.SuggestedAction == "" {
		t.Error("SuggestedAction is empty; a finding must always tell the author something to do")
	}
	if f.Message != d.Message {
		t.Errorf("Message = %q, want the grammar's own %q", f.Message, d.Message)
	}
	if !strings.HasPrefix(f.Fingerprint, "v1:") {
		t.Errorf("Fingerprint = %q, want a versioned value", f.Fingerprint)
	}
}
