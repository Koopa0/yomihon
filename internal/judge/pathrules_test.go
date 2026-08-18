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
// the rule names, and a rule the judge cannot advise on would reach the author
// as a panic during a check run.
func TestEveryStudyPathRuleHasAnAction(t *testing.T) {
	t.Parallel()

	for _, rule := range []string{
		sequence.RuleRoleMissing, sequence.RuleRoleDuplicate, sequence.RuleRoleConflict,
		sequence.RuleLocalOrphan, sequence.RuleNestingTooDeep, sequence.RuleRoleOnEntry,
		sequence.RuleRoleInvalid, sequence.RuleRoleMisplaced, sequence.RuleEntryOutsideBranch,
		sequence.RuleEntryMultiTarget, sequence.RuleEntryNoncanonical,
	} {
		if pathRuleAction[rule] == "" {
			t.Errorf("rule %q has no suggested action", rule)
		}
		if !slices.Contains(ruleIDs, rule) {
			t.Errorf("rule %q is not in the --deny registry", rule)
		}
	}
}
