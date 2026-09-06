package judge

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var goldenSourceRule = regexp.MustCompile(`"source_rule":"([^"]*)"`)

// TestJudgeRuleInventory keeps the contributor lookup complete against the
// registered IDs and literal authority pairs in the frozen findings. A link
// to an existing source file is checked here; its ownership still needs review.
func TestJudgeRuleInventory(t *testing.T) {
	t.Parallel()

	repo, err := os.OpenRoot("../..")
	if err != nil {
		t.Fatalf("inventory repository: open root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := repo.Close(); closeErr != nil {
			t.Errorf("inventory repository: close root: %v", closeErr)
		}
	})
	const inventoryPath = "docs/judge-rules.md"
	data, err := repo.ReadFile(inventoryPath)
	if err != nil {
		t.Fatalf("inventory document: read %s: %v", inventoryPath, err)
	}
	const heading = "\n## Judge rule inventory\n\n"
	if count := strings.Count(string(data), heading); count != 1 {
		t.Fatalf("inventory heading: found %d, want exactly one %q", count, heading)
	}
	const header = "| RuleID | SourceRule | Finding source file |"
	if count := strings.Count(string(data), header); count != 1 {
		t.Fatalf("inventory header: found %d, want exactly one %q", count, header)
	}
	_, section, _ := strings.Cut(string(data), heading)
	table, _, _ := strings.Cut(section, "\n\n")
	lines := strings.Split(table, "\n")
	if len(lines) < 2 || lines[0] != header || lines[1] != "|---|---|---|" {
		t.Fatal("inventory table: want the exact header and three-column separator immediately after the heading")
	}
	if len(lines) == 2 {
		t.Fatal("inventory empty: want at least one rule row")
	}
	// Only this table's three-cell format is supported. Every actual row must
	// match, so malformed content cannot silently disappear from the check.
	documentedIDs := map[string]bool{}
	documentedPairs := map[string]bool{}
	firstLine := strings.Count(string(data[:len(data)-len(section)]), "\n") + 1
	for i, line := range lines[2:] {
		cells := strings.Split(line, " | ")
		if len(cells) != 3 || !strings.HasPrefix(cells[0], "| `") || !strings.HasSuffix(cells[0], "`") ||
			!strings.HasPrefix(cells[1], "`") || !strings.HasSuffix(cells[1], "`") ||
			!strings.HasPrefix(cells[2], "[`") || !strings.HasSuffix(cells[2], ") |") {
			t.Fatalf("inventory row: %s:%d: malformed three-column row %q", inventoryPath, firstLine+i+2, line)
		}
		id := strings.TrimSuffix(strings.TrimPrefix(cells[0], "| `"), "`")
		authority := strings.TrimSuffix(strings.TrimPrefix(cells[1], "`"), "`")
		label, target, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(cells[2], "[`"), ") |"), "`](")
		if id == "" || authority == "" || label == "" || target == "" || !ok ||
			strings.ContainsAny(id+authority+label+target, "`|") {
			t.Fatalf("inventory cells: %s:%d: want nonempty inline-code ID, authority, and source link", inventoryPath, firstLine+i+2)
		}
		pair := id + " / " + authority
		if documentedPairs[pair] {
			t.Fatalf("inventory duplicate pair: %s:%d: %s", inventoryPath, firstLine+i+2, pair)
		}
		documentedIDs[id] = true
		documentedPairs[pair] = true
		if strings.HasPrefix(target, "/") {
			t.Fatalf("inventory source target: %s:%d: %q must be relative", inventoryPath, firstLine+i+2, target)
		}
		sourcePath := path.Join(path.Dir(inventoryPath), target)
		if label != sourcePath {
			t.Fatalf("inventory source label: %s:%d: %q does not match target path %q", inventoryPath, firstLine+i+2, label, sourcePath)
		}
		info, statErr := repo.Stat(sourcePath)
		if statErr != nil {
			t.Fatalf("inventory source stat: %s:%d: %q: %v", inventoryPath, firstLine+i+2, target, statErr)
		}
		if !info.Mode().IsRegular() || path.Ext(sourcePath) != ".go" {
			t.Fatalf("inventory source file: %s:%d: %q must name a regular Go file", inventoryPath, firstLine+i+2, target)
		}
	}
	for i, line := range strings.Split(string(data), "\n") {
		if i+1 >= firstLine && i+1 < firstLine+len(lines) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			t.Fatalf("inventory unchecked row: %s:%d: table-shaped lines belong only in the inventory table", inventoryPath, i+1)
		}
	}

	registeredIDs := map[string]bool{}
	for _, id := range allRuleIDs() {
		registeredIDs[string(id)] = true
	}
	if len(registeredIDs) == 0 {
		t.Fatal("inventory registry: allRuleIDs returned no rules")
	}
	goldens, err := filepath.Glob(filepath.Join("testdata", "golden", "*.jsonl"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("inventory goldens: want frozen JSONL files (glob: %v)", err)
	}
	goldenPairs := map[string]bool{}
	for _, name := range goldens {
		golden, readErr := os.ReadFile(name) // #nosec G304 -- a fixed testdata path
		if readErr != nil {
			t.Fatalf("inventory golden read: %s: %v", name, readErr)
		}
		// JSONL is LF-delimited. Raw Unicode line separators are valid inside
		// the frozen JSON strings and must not split a record.
		for i, record := range strings.Split(string(golden), "\n") {
			if record == "" {
				continue
			}
			var finding struct {
				RuleID     string `json:"rule_id"`
				SourceRule string `json:"source_rule"`
			}
			if decodeErr := json.Unmarshal([]byte(record), &finding); decodeErr != nil {
				t.Fatalf("inventory golden JSON: %s:%d: %v", name, i+1, decodeErr)
			}
			if finding.RuleID == "" || finding.SourceRule == "" {
				t.Fatalf("inventory golden pair: %s:%d: want nonempty rule_id and source_rule", name, i+1)
			}
			goldenPairs[finding.RuleID+" / "+finding.SourceRule] = true
		}
	}
	if len(goldenPairs) == 0 {
		t.Fatal("inventory golden coverage: no rule/authority pairs found")
	}
	for _, check := range []struct {
		name      string
		got, want map[string]bool
	}{
		{"IDs", documentedIDs, registeredIDs},
		{"authority pairs", documentedPairs, goldenPairs},
	} {
		var missing, extra []string
		for value := range check.want {
			if !check.got[value] {
				missing = append(missing, value)
			}
		}
		for value := range check.got {
			if !check.want[value] {
				extra = append(extra, value)
			}
		}
		slices.Sort(missing)
		slices.Sort(extra)
		if len(missing)+len(extra) != 0 {
			t.Errorf("inventory %s mismatch: missing %v; extra %v", check.name, missing, extra)
		}
	}
}

// TestEveryEmittedSourceRuleIsOneThatCanBeOpened is the lock the invented
// anchors got past. A finding's source_rule tells a reader where its rule's
// authority is written down, and nothing checked that the thing it named held
// it: findings anchored into a heading their document does not have, into a
// contract table nothing declares, and onto artifacts that never state the
// convention being enforced. The field read as authority in every one of
// them.
//
// Every value a golden carries has to be one of the declared set, and every
// value in that set has to answer for itself by its class. The vault
// contract's anchors are checked against the fixture contracts committed under
// testdata, because a clean clone has no vault and a test that skipped when
// the vault was absent would be the check that never runs. The product's own
// name is not a file at all: its authority is the golden set this test is
// reading, so what it must never do is grow an anchor and pose as a document.
// And a value naming the vault's human note schema — a form no current rule
// earns, since that document states no link or collision convention — must
// never anchor into a document this repository cannot open, which is exactly
// how it went wrong the first time.
func TestEveryEmittedSourceRuleIsOneThatCanBeOpened(t *testing.T) {
	t.Parallel()

	declared := map[string]bool{
		sourceContract:             true,
		sourceContractRules:        true,
		sourceContractScan:         true,
		sourceContractSupersession: true,
		sourceYomihon:              true,
		// A document is an admissible source in principle — one the vault
		// keeps, or one this repository ships — but no rule cites one, so none
		// is declared: reintroducing one means adding it here deliberately,
		// with a class check of its own below that opens it from a clean clone.
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
