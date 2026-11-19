package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

const domainTableFixture = "testdata/vault-domain-table"

// domainTableCount is how many findings of the domains rule a run carried.
func domainTableCount(findings []Finding) (count int) {
	for i := range findings {
		if findings[i].RuleID == domainTableRule {
			count++
		}
	}
	return count
}

// TestARefusedDomainsTableGatesOnItsOwnRule holds that the finding is one a
// run can be told to fail on, and that it is only that: it silences no other
// rule, so a gate on one it did not silence stays quiet.
func TestARefusedDomainsTableGatesOnItsOwnRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		deny []string
		want int
	}{
		{name: "its own rule", deny: []string{"schema.domain_table"}, want: 1},
		{name: "the error severity", deny: []string{"error"}, want: 1},
		{name: "a rule it did not silence", deny: []string{"link.broken"}, want: 0},
		{name: "no gate reports and exits 0", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, exit, err := RunCheck(t.Context(), &CheckOptions{Root: domainTableFixture, Format: FormatHuman, All: true, Deny: tt.deny})
			if err != nil {
				t.Fatalf("RunCheck(deny %v) error = %v", tt.deny, err)
			}
			if exit != tt.want {
				t.Errorf("RunCheck(deny %v) exit = %d, want %d", tt.deny, exit, tt.want)
			}
		})
	}
}

// TestTheDomainsFindingSurvivesEveryScope keeps it past the filters a scope
// applies: it names the rules file, which lies outside the knowledge layer and
// outside any directory a run is narrowed to.
func TestTheDomainsFindingSurvivesEveryScope(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		all   bool
		paths []string
	}{
		{name: "default scope"},
		{name: "whole vault", all: true},
		{name: "narrowed to a directory", paths: []string{"Writing"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			findings, err := runCheckAction(t.Context(), domainTableFixture, tt.paths, tt.all)
			if err != nil {
				t.Fatalf("runCheckAction() error = %v", err)
			}
			if got := domainTableCount(findings); got != 1 {
				t.Fatalf("runCheckAction() carried %d %s findings, want 1", got, domainTableRule)
			}
			for i := range findings {
				if findings[i].RuleID == domainTableRule && findings[i].Path != schema.ContractRelPath {
					t.Errorf("finding path = %q, want the rules file %q", findings[i].Path, schema.ContractRelPath)
				}
			}
		})
	}
}

// TestADomainsTableThatIsUsableDrawsNoFinding is the control: the same vault
// with the second pin changed to an endpaper of its own, and the same vault with
// no table at all, never carry the finding.
func TestADomainsTableThatIsUsableDrawsNoFinding(t *testing.T) {
	t.Parallel()

	rulesFile := filepath.FromSlash(schema.ContractRelPath)
	data, err := os.ReadFile(filepath.Join(domainTableFixture, rulesFile)) // #nosec G304 -- the checked-in fixture, a constant path
	if err != nil {
		t.Fatalf("read the fixture's rules file: %v", err)
	}
	tests := []struct {
		name string
		edit func(string) string
	}{
		{"two different endpapers", func(text string) string {
			return strings.Replace(text, "[domains.japanese]\nendpaper = 5", "[domains.japanese]\nendpaper = 6", 1)
		}},
		{"no endpaper pinned", func(text string) string {
			return strings.ReplaceAll(text, "endpaper = 5\n", "")
		}},
		{"no table", func(text string) string {
			before, _, _ := strings.Cut(text, "# Two domains")
			return before + "[privacy]\nnever_egress_dirs = []\n"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(domainTableFixture)); err != nil {
				t.Fatalf("copy the fixture: %v", err)
			}
			edited := tt.edit(string(data))
			if edited == string(data) {
				t.Fatal("the edit changed nothing, so this case proves nothing")
			}
			if err := os.WriteFile(filepath.Join(root, rulesFile), []byte(edited), 0o600); err != nil {
				t.Fatalf("write the rules file: %v", err)
			}
			findings, err := runCheckAction(t.Context(), root, nil, true)
			if err != nil {
				t.Fatalf("runCheckAction() error = %v", err)
			}
			if got := domainTableCount(findings); got != 0 {
				t.Errorf("a usable table drew %d %s findings", got, domainTableRule)
			}
		})
	}
}
