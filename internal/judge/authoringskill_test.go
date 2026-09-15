package judge

import (
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/sequence"
)

// authoringSkillDir is the rule vocabulary's other reader: a person makes
// sense of a `yomihon check` finding by looking up the guide's mention of the
// same rule id, so nothing but this test forces the two to move together. The
// whole folder is read rather than its entry point alone, because the guide
// splits one subject per file and most rule ids are named in the reference
// that owns them. The path is relative to this package's own directory, the
// way go test always runs regardless of where the test command itself was
// invoked from.
const authoringSkillDir = "../../skills"

// ruleIDToken matches one inline-code span shaped like a rule id: two or more
// lowercase, underscore-only words joined by dots, and nothing else inside the
// backticks. Two was not enough — link.broken.path is a real emitted id, and
// while the pattern stopped at two segments the guide could name a
// three-segment rule that had since been renamed and this check, whose whole
// job is to catch that, saw nothing to compare.
var ruleIDToken = regexp.MustCompile("`([a-z_]+(?:\\.[a-z_]+)+)`")

// TestAuthoringSkillNamesExactlyTheRulesTheChecksEmit pins the rule ids the
// authoring guide mentions to the set this package and the study-path grammar
// actually emit, in both directions.
//
// A rule renamed or removed in code leaves a stale id sitting in the guide,
// pointing a reader at a finding they will never see. A rule added without a
// line in the guide is a finding a reader meets with nowhere to look it up:
// the id is the only handle a person has on it, and the guide is where that
// handle is supposed to resolve. Neither shows up in any other way, because
// prose and code are checked by different people at different times.
func TestAuthoringSkillNamesExactlyTheRulesTheChecksEmit(t *testing.T) {
	t.Parallel()

	known := map[string]bool{}
	for _, id := range allRuleIDs() {
		known[string(id)] = true
	}
	for _, rule := range sequence.Rules() {
		known[string(rule)] = true
	}

	mentioned := ruleIDsInAuthoringSkill(t)
	if len(mentioned) == 0 {
		t.Fatal("found no rule-id-shaped token anywhere in the authoring skill; the extraction pattern may have drifted from how the guide spells one")
	}

	var undocumented []string
	for id := range known {
		if !mentioned[id] {
			undocumented = append(undocumented, id)
		}
	}
	slices.Sort(undocumented)
	if len(undocumented) > 0 {
		t.Errorf("checks emit rules the authoring skill never names, so a reader who meets one has nowhere to look it up: %v", undocumented)
	}

	var unemittable []string
	for id := range mentioned {
		if !known[id] {
			unemittable = append(unemittable, id)
		}
	}
	slices.Sort(unemittable)
	if len(unemittable) > 0 {
		t.Errorf("the authoring skill names rules no current check emits, so a reader is pointed at a finding they will never see: %v", unemittable)
	}
}

// ruleIDsInAuthoringSkill collects every inline-code span shaped like a rule
// id from every Markdown file in the skill folder.
//
// A filename written in inline code has that same shape — `diagnostics.md` is
// two lowercase words joined by a dot, and the guide links to its own
// reference files that way — so a token whose last segment is a file
// extension is dropped. No rule id carries one. The extension here is the one
// the guide currently writes; a file of another kind named this way turns the
// check red rather than passing quietly, which is how it should be found.
//
// Reading goes through a directory handle so every path resolves inside the
// folder. A link planted under it cannot then pull in text from elsewhere on
// the machine and have this count the ids it finds there.
func ruleIDsInAuthoringSkill(t *testing.T) map[string]bool {
	t.Helper()

	root, err := os.OpenRoot(authoringSkillDir)
	if err != nil {
		t.Fatalf("open %s: %v", authoringSkillDir, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("close %s: %v", authoringSkillDir, closeErr)
		}
	})
	skill := root.FS()

	ids := map[string]bool{}
	files := 0
	err = fs.WalkDir(skill, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		files++
		data, readErr := fs.ReadFile(skill, path)
		if readErr != nil {
			return readErr
		}
		for _, m := range ruleIDToken.FindAllStringSubmatch(string(data), -1) {
			if isFilename(m[1]) {
				continue
			}
			ids[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read %s: %v", authoringSkillDir, err)
	}
	if files == 0 {
		t.Fatalf("found no Markdown under %s; the folder moved and this check is comparing against nothing", authoringSkillDir)
	}
	return ids
}

// isFilename reports whether an id-shaped token is a file the guide names
// rather than a rule the checks emit.
func isFilename(token string) bool {
	return strings.HasSuffix(token, ".md")
}
