package judge

import (
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// ruleIDToken matches an inline-code span shaped like a rule id: two or more
// lowercase words joined by dots, such as `link.broken.path`.
var ruleIDToken = regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)+)`")

// TestAuthoringSkillNamesExactlyTheRulesTheChecksEmit holds the rule ids the
// authoring skill names to the rules the checks emit, both ways: a renamed rule
// leaves the skill pointing at a finding nobody sees, and a new rule without a
// line there is a finding with nowhere to look it up.
func TestAuthoringSkillNamesExactlyTheRulesTheChecksEmit(t *testing.T) {
	t.Parallel()
	skill := os.DirFS("../../skills")
	mentioned := map[string]bool{}
	err := fs.WalkDir(skill, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := fs.ReadFile(skill, path)
		for _, m := range ruleIDToken.FindAllStringSubmatch(string(data), -1) {
			if !strings.HasSuffix(m[1], ".md") {
				mentioned[m[1]] = true
			}
		}
		return err
	})
	if err != nil {
		t.Fatalf("read the authoring skill: %v", err)
	}
	emitted := map[string]bool{}
	for _, id := range ruleIDs {
		emitted[string(id)] = true
	}
	if diff := keysOnlyIn(emitted, mentioned); len(diff) > 0 {
		t.Errorf("the checks emit rules the authoring skill never names: %v", diff)
	}
	if diff := keysOnlyIn(mentioned, emitted); len(diff) > 0 {
		t.Errorf("the authoring skill names rules no check emits: %v", diff)
	}
}

// keysOnlyIn is the sorted keys of a that b lacks.
func keysOnlyIn(a, b map[string]bool) []string {
	var out []string
	for key := range a {
		if !b[key] {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}
