package judge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var goldenSourceRule = regexp.MustCompile(`"source_rule":"([^"]*)"`)

// TestEveryEmittedSourceRuleIsOneThatCanBeOpened is the lock the two invented
// anchors got past. A finding's source_rule tells a reader where its rule is
// written down, and nothing checked that the thing it named existed: for forty
// findings it was a heading Note-Schema.md does not have, and for two more a
// contract table nothing declares. The field read as authority in every one of
// them.
//
// Every value a golden carries has to be one of the declared set, and every
// value in that set has to be a file this test can find and — where the value
// carries an anchor — a table that file really opens. The repository's own
// authoring contract is checked as a file here; the two vault artifacts are
// checked against the fixture contracts committed under testdata, because a
// clean clone has no vault and a test that skipped when the vault was absent
// would be the check that never runs.
func TestEveryEmittedSourceRuleIsOneThatCanBeOpened(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{
		sourceContract:             true,
		sourceContractRules:        true,
		sourceContractSupersession: true,
		sourceNoteSchema:           true,
		sourceAuthoring:            true,
	}

	goldens, err := filepath.Glob(filepath.Join("testdata", "golden", "*.jsonl"))
	if err != nil {
		t.Fatalf("glob goldens: %v", err)
	}
	if len(goldens) == 0 {
		t.Fatal("no goldens found; this test would pass over nothing")
	}
	seen := map[string]bool{}
	for _, g := range goldens {
		data, readErr := os.ReadFile(g) // #nosec G304 -- a fixed testdata path
		if readErr != nil {
			t.Fatalf("read %s: %v", g, readErr)
		}
		for _, m := range goldenSourceRule.FindAllStringSubmatch(string(data), -1) {
			seen[m[1]] = true
			if !declared[m[1]] {
				t.Errorf("%s emits source_rule %q, which is not one of the declared sources", g, m[1])
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no source_rule value appears in any golden; the scan matched nothing")
	}

	// Now the other direction: each declared source names something openable.
	contracts, err := filepath.Glob(filepath.Join("testdata", "*", "System", "schemas", "vault-schema.toml"))
	if err != nil {
		t.Fatalf("glob fixture contracts: %v", err)
	}
	if len(contracts) == 0 {
		t.Fatal("no fixture contract found; the anchor check would pass over nothing")
	}
	var contract strings.Builder
	for _, c := range contracts {
		data, readErr := os.ReadFile(c) // #nosec G304 -- a fixed testdata path
		if readErr != nil {
			t.Fatalf("read %s: %v", c, readErr)
		}
		contract.Write(data)
	}
	declaredTables := contract.String()
	for value := range declared {
		file, anchor, hasAnchor := strings.Cut(value, "#")
		switch file {
		case sourceAuthoring:
			if _, err := os.Stat(filepath.Join("..", "..", file)); err != nil {
				t.Errorf("source_rule %q names a repository file that is not there: %v", value, err)
			}
		case sourceContract:
			// Named without a directory the way the vault names it; the
			// committed fixture is the copy this test can open.
		case sourceNoteSchema:
			// A vault artifact with no committed copy. Its existence is the
			// vault's to keep; what this test can hold is that no anchor is
			// invented onto it, which is exactly how it went wrong.
			if hasAnchor {
				t.Errorf("source_rule %q anchors into a vault document this repository cannot check; name the file alone", value)
			}
		default:
			t.Errorf("declared source %q has no rule for checking it", value)
		}
		if hasAnchor && file == sourceContract {
			if !strings.Contains(declaredTables, "["+anchor+"]") {
				t.Errorf("source_rule %q names contract table [%s], which the fixture contract does not declare", value, anchor)
			}
		}
	}
}
