package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestARefusedNavigationTableFailsTheGateOfTheRulesItSilenced is the lock on
// the silence: a contract whose [navigation] table is refused used to skip
// every course and gate nothing, so a run denying a course rule exited 0 with
// the course never read.
//
// The control is the same vault under the contract that names its courses
// correctly. It has to gate on the same rule for the refused case to prove
// anything: a gate that fired on both would be the finding doing nothing.
func TestARefusedNavigationTableFailsTheGateOfTheRulesItSilenced(t *testing.T) {
	t.Parallel()

	const (
		refused = "testdata/vault-navigation-unavailable"
		control = "testdata/vault-course"
	)
	tests := []struct {
		name string
		root string
		deny []string
		want int
	}{
		{name: "control gates on the course rule it reports", root: control, deny: []string{"path.role_missing"}, want: 1},
		{name: "refused table gates on the course rule it silenced", root: refused, deny: []string{"path.role_missing"}, want: 1},
		{name: "refused table gates on a map rule it silenced", root: refused, deny: []string{"map.disk_mismatch"}, want: 1},
		{name: "refused table gates on its own rule", root: refused, deny: []string{"schema.navigation_unavailable"}, want: 1},
		{name: "refused table gates on the error severity", root: refused, deny: []string{"error"}, want: 1},
		{name: "refused table does not gate on a rule it did not silence", root: refused, deny: []string{"link.broken"}, want: 0},
		{name: "refused table without a gate reports and exits 0", root: refused, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, exit, err := RunCheck(t.Context(), &CheckOptions{Root: tt.root, Format: FormatHuman, All: true, Deny: tt.deny})
			if err != nil {
				t.Fatalf("RunCheck(%q, deny %v) error = %v", tt.root, tt.deny, err)
			}
			if exit != tt.want {
				t.Errorf("RunCheck(%q, deny %v) exit = %d, want %d", tt.root, tt.deny, exit, tt.want)
			}
		})
	}
}

// TestTheNavigationFindingSurvivesEveryScope holds the finding past the
// filters a scope applies: it names the contract, which lies outside the
// knowledge layer and outside any directory a run is narrowed to, and dropping
// it there would leave a scoped run reading as a clean verdict over courses
// that were never read.
func TestTheNavigationFindingSurvivesEveryScope(t *testing.T) {
	t.Parallel()

	root := judgeFixtureRoot(t, "testdata/vault-navigation-unavailable")
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
			findings, err := runCheckAction(t.Context(), root, tt.paths, tt.all)
			if err != nil {
				t.Fatalf("runCheckAction() error = %v", err)
			}
			var got int
			for i := range findings {
				if findings[i].RuleID == navigationUnavailableRule {
					got++
					if findings[i].Path != schema.ContractRelPath {
						t.Errorf("finding path = %q, want the contract file %q", findings[i].Path, schema.ContractRelPath)
					}
				}
			}
			if got != 1 {
				t.Errorf("runCheckAction() carried %d %s findings, want 1", got, navigationUnavailableRule)
			}
		})
	}
}

// TestNoNavigationFindingWhenTheTableIsUsable keeps the finding from being a
// permanent fixture of every report: a contract whose table is read cleanly
// never carries it.
func TestNoNavigationFindingWhenTheTableIsUsable(t *testing.T) {
	t.Parallel()

	findings, err := runCheckAction(t.Context(), judgeFixtureRoot(t, "testdata/vault-course"), nil, true)
	if err != nil {
		t.Fatalf("runCheckAction() error = %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == navigationUnavailableRule {
			t.Errorf("a usable [navigation] table drew %s", navigationUnavailableRule)
		}
	}
}

// TestAContractWithoutANavigationTableIsReportedToo pins the other way the
// roles go missing. A contract that exists and leaves the table out declared
// nothing about which types are courses, and the course rules skip every note
// just as they do for a refused one; the finding's sentence says "missing"
// for this case, so the case has to draw it.
func TestAContractWithoutANavigationTableIsReportedToo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/vault-course")); err != nil {
		t.Fatalf("CopyFS() error = %v", err)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	data, err := os.ReadFile(contractPath) // #nosec G304 -- a path under this test's own temp directory
	if err != nil {
		t.Fatalf("ReadFile(contract) error = %v", err)
	}
	const table = "[navigation]\npath_types = [\"course\"]\nmap_types = [\"topic-map\"]\n"
	text := strings.Replace(string(data), table, "", 1)
	if text == string(data) {
		t.Fatalf("fixture drift: the contract no longer carries %q", table)
	}
	if err := os.WriteFile(contractPath, []byte(text), 0o600); err != nil { // #nosec G703 -- contractPath is the fixed contract location under this test's own t.TempDir
		t.Fatalf("WriteFile(contract) error = %v", err)
	}

	findings, err := runCheckAction(t.Context(), root, nil, true)
	if err != nil {
		t.Fatalf("runCheckAction() error = %v", err)
	}
	var got int
	for i := range findings {
		if findings[i].RuleID == navigationUnavailableRule {
			got++
		}
	}
	if got != 1 {
		t.Errorf("a contract with no [navigation] table drew %d %s findings, want 1", got, navigationUnavailableRule)
	}
}
