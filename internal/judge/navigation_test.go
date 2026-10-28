package judge

import (
	"testing"
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
// filters a scope applies: it names no path, so a filter by directory has
// nothing to keep it by, and dropping it there would leave a scoped run
// reading as a clean verdict over courses that were never read.
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
