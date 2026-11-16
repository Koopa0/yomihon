package judge

import (
	"slices"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// navigationUnavailableRule is the rule id of a contract whose type roles
// could not be used.
const navigationUnavailableRule RuleID = "schema.navigation_unavailable"

// checkNavigation reports a contract whose navigation roles could not be
// honoured. Without them no note type is known to be a course or a map, so the
// rules that read those roles skip every note, and a run that says nothing
// about it reads as a clean verdict over ground it never judged.
func checkNavigation(roles schema.NavigationRoles) []Finding {
	if roles.Trustworthy() {
		return nil
	}
	return []Finding{navigationUnavailableFinding()}
}

// navigationUnavailableFinding is the whole of what is said about a refused
// type role. It names the contract file, which sits at the same place in every
// vault, so the path carries nothing out of the vault. The sentence is fixed:
// the account of why a role was refused quotes type names out of the contract,
// which this face does not hand to an agent. The finding is added
// past the scope filters, so the contract living under System does not drop it.
func navigationUnavailableFinding() Finding {
	return Finding{
		RuleID:          navigationUnavailableRule,
		Severity:        SeverityError,
		Path:            schema.ContractRelPath,
		Message:         "course and map rules did not run, because a type role in the contract could not be used",
		Evidence:        "without the roles no note type is known to be a course or a map, so those rules skipped every note; the reason is not quoted here, because it would repeat the contract's own text",
		SuggestedAction: "correct the role of the type in the vault contract (yomihon serve logs the reason at startup), then judge the vault again",
		SourceRule:      sourceContract,
		Fingerprint:     fingerprint(navigationUnavailableRule, schema.ContractRelPath, ""),
	}
}

// silencedWithoutNavigation reports whether a rule reads the navigation roles
// and so reports nothing when they are unavailable: the study-path grammar's
// rules and the two that reconcile a course against disk. A gate on any of them
// cannot be answered by their silence, so it is answered by the finding that
// says they did not run.
func silencedWithoutNavigation(rule string) bool {
	switch rule {
	case "map.disk_mismatch", "map.disk_unlisted":
		return true
	}
	return slices.Contains(sequence.Rules(), sequence.Rule(rule))
}
