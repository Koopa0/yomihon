package judge

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/sequence"
)

// authoringSkillPath is the rule vocabulary's other reader: a person makes
// sense of a `yomihon check` finding by looking up this guide's mention of the
// same rule id, so nothing but this test forces the two to move together. The
// path is relative to this package's own directory, the way go test always
// runs regardless of where the test command itself was invoked from.
const authoringSkillPath = "../../skills/yomihon-authoring/SKILL.md"

// ruleIDToken matches one inline-code span shaped like a rule id: two or more
// lowercase, underscore-only words joined by dots, and nothing else inside the
// backticks. Two was not enough — link.broken.path is a real emitted id, and
// while the pattern stopped at two segments the guide could name a
// three-segment rule that had since been renamed and this check, whose whole
// job is to catch that, saw nothing to compare.
var ruleIDToken = regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)+)`")

// TestAuthoringSkillNamesOnlyRulesThatExist pins every rule id the authoring
// guide mentions to the set this package and the study-path grammar actually
// emit. The guide names a working subset chosen for a reader rather than the
// whole rule set, so this checks containment, not equality: a rule renamed or
// removed in code would otherwise leave a stale id sitting in the guide,
// pointing a reader at a finding they will never see.
func TestAuthoringSkillNamesOnlyRulesThatExist(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(authoringSkillPath) // #nosec G304 -- a fixed path to this repository's own guide, not untrusted input
	if err != nil {
		t.Fatalf("read %s: %v", authoringSkillPath, err)
	}

	known := map[string]bool{}
	for _, id := range allRuleIDs() {
		known[id] = true
	}
	for _, rule := range sequence.Rules() {
		known[string(rule)] = true
	}

	mentioned := map[string]bool{}
	for _, m := range ruleIDToken.FindAllStringSubmatch(string(data), -1) {
		mentioned[m[1]] = true
	}
	if len(mentioned) == 0 {
		t.Fatal("found no rule-id-shaped token in the authoring skill; the extraction pattern may have drifted from how the guide spells one")
	}

	missing := map[string]bool{}
	for id := range mentioned {
		if !known[id] {
			missing[id] = true
		}
	}
	if len(missing) > 0 {
		t.Errorf("the authoring skill names rules no current check emits: %v", slices.Sorted(maps.Keys(missing)))
	}
}
