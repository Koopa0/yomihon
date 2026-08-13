package judge

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/sequence"
)

// TestEveryEmittedRuleIsDeniable holds the registry honest. --deny takes a rule
// id, and a rule that fires but is not listed cannot be denied by name — the
// list drifted that way once already, silently, because nothing compared it
// against what the rules actually emit.
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
