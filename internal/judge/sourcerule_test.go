package judge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var goldenSourceRule = regexp.MustCompile(`"source_rule":"([^"]*)"`)

// TestEveryEmittedSourceRuleIsOneThatCanBeOpened is the lock the invented
// anchors got past. A finding's source_rule tells a reader where its rule's
// authority is written down, and nothing checked that the thing it named held
// it: findings anchored into a heading their document does not have, into a
// contract table nothing declares, and onto artifacts that never state the
// convention being enforced. The field read as authority in every one of
// them.
//
// Every value a golden carries has to be one of the declared set, and every
// value in that set has to answer for itself by its class. A repository file
// is checked as a file. The vault contract's anchors are checked against the
// fixture contracts committed under testdata, because a clean clone has no
// vault and a test that skipped when the vault was absent would be the check
// that never runs. The product's own name is not a file at all: its authority
// is the golden set this test is reading, so what it must never do is grow an
// anchor and pose as a document. And a value naming the vault's human note
// schema — a form no current rule earns, since that document states no link
// or collision convention — must never anchor into a document this repository
// cannot open, which is exactly how it went wrong the first time.
func TestEveryEmittedSourceRuleIsOneThatCanBeOpened(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{
		sourceContract:             true,
		sourceContractRules:        true,
		sourceContractScan:         true,
		sourceContractSupersession: true,
		sourceYomihon:              true,
		sourceAuthoring:            true,
		// A vault-kept document is an admissible source in principle, but no
		// rule cites one, so none is declared: reintroducing one means adding
		// it here deliberately, with a class check of its own below.
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
	if !seen[sourceYomihon] {
		t.Errorf("no golden emits source_rule %q; the product-dialect rules should be citing it", sourceYomihon)
	}

	// Now the other direction: each declared source answers for itself.
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
			// committed fixtures are the copies this test can open, and the
			// anchor check below holds each named table to them.
		case sourceYomihon:
			// The product itself. Its rules are declared by no artifact;
			// the goldens this test has already read are what pin them, so
			// the one thing this value must never do is anchor into a
			// document as if one held the rule.
			if hasAnchor {
				t.Errorf("source_rule %q anchors into the product name; the product is not a document", value)
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
