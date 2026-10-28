package judge

import (
	"slices"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// navigationUnavailableRule is the rule id of a contract whose [navigation]
// table could not be used.
const navigationUnavailableRule RuleID = "schema.navigation_unavailable"

// checkNavigation reports a contract whose navigation roles could not be
// honoured. Without them no note type is known to be a course or a map, so the
// rules that read those roles skip every note, and a run that says nothing
// about it reads as a clean verdict over ground it never judged. A contract
// that omits the table is reported the same way: the roles are missing either
// way, and the fix is in the same table.
func checkNavigation(roles schema.NavigationRoles) []Finding {
	if roles.Trustworthy() {
		return nil
	}
	return []Finding{navigationUnavailableFinding()}
}

// navigationUnavailableFinding is the whole of what is said about a refused
// [navigation] table. It carries no path, because the fault is in the contract
// as a whole and a contract file's own path is dropped from every run that did
// not ask for System, and the sentence is fixed: the decoder's account of why
// the table was refused quotes keys out of the contract, which this face
// does not hand to an agent.
func navigationUnavailableFinding() Finding {
	return Finding{
		RuleID:          navigationUnavailableRule,
		Severity:        SeverityError,
		Message:         "course and map rules did not run, because the contract's [navigation] table is missing or could not be used",
		Evidence:        "without it no note type is known to be a course or a map, so those rules skipped every note; the reason is not quoted here, because it would repeat the contract's own text",
		SuggestedAction: "correct the [navigation] table in the vault contract (yomihon serve logs the reason at startup), then judge the vault again",
		SourceRule:      sourceContract,
		Fingerprint:     fingerprint(navigationUnavailableRule, "", ""),
	}
}

// silencedWithoutNavigation reports whether a rule reads the navigation roles
// and so reports nothing when they are unavailable: the study-path grammar's
// rules, the two that reconcile a course against disk, and the one that finds a
// live course linking an archived lesson. A gate on any of them cannot be
// answered by their silence, so it is answered by the finding that says they
// did not run.
func silencedWithoutNavigation(rule string) bool {
	switch rule {
	case "map.disk_mismatch", "map.disk_unlisted", archivedNavigationRule:
		return true
	}
	return slices.Contains(sequence.Rules(), sequence.Rule(rule))
}
