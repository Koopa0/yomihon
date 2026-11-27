package judge

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var goldenSourceRule = regexp.MustCompile(`"source_rule":"([^"]*)"`)

// TestEveryGoldenSourceRuleIsDeclared holds each source_rule a golden records
// to the declared sources, so a finding never names an authority the rules do
// not have.
func TestEveryGoldenSourceRuleIsDeclared(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{
		sourceContract:        true,
		sourceContractRules:   true,
		sourceContractFolders: true,
		sourceContractTypes:   true,
		sourceYomihon:         true,
	}
	goldens, err := filepath.Glob(filepath.Join("testdata", "golden", "*.jsonl"))
	if err != nil {
		t.Fatalf("glob goldens: %v", err)
	}
	if len(goldens) == 0 {
		t.Fatal("no goldens found")
	}
	for _, g := range goldens {
		data, readErr := os.ReadFile(g) // #nosec G304 -- a fixed testdata path
		if readErr != nil {
			t.Fatalf("read %s: %v", g, readErr)
		}
		for _, m := range goldenSourceRule.FindAllStringSubmatch(string(data), -1) {
			if !declared[m[1]] {
				t.Errorf("%s emits source_rule %q, which is not one of the declared sources", g, m[1])
			}
		}
	}
}
