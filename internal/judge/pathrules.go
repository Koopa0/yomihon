package judge

import (
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// The study-path rules carry the authoring contract's own diagnostics to the
// author. They are the judge's face on one interpretation: the same grammar
// navigation reads decides what a course lists, so a course page and a check
// run can never disagree about it.
//
// Every one is a warning. Reading is unaffected — the prose still renders and
// the note still opens — but the structured projection stops until the author
// decides, and a decision only the author can make is not information.

// pathRuleAction names what to do about each rule, in the author's terms.
var pathRuleAction = map[string]string{
	sequence.RuleRoleMissing:        "declare the branch {sequence=primary}, {sequence=local} or {sequence=none}",
	sequence.RuleRoleDuplicate:      "keep one sequence declaration on the branch",
	sequence.RuleRoleConflict:       "move the branch out from under the one declared none, or declare it none too",
	sequence.RuleLocalOrphan:        "nest the side branch under the lesson it belongs to",
	sequence.RuleNestingTooDeep:     "keep a side branch one level below what it hangs from",
	sequence.RuleRoleOnEntry:        "give the branch its own row above the list it opens",
	sequence.RuleRoleInvalid:        "use exactly one of primary, local or none",
	sequence.RuleRoleMisplaced:      "put the declaration at the end of a heading, or of a row that opens a list",
	sequence.RuleEntryOutsideBranch: "put the lesson under a level-2 heading",
	sequence.RuleEntryMultiTarget:   "give each lesson its own row, or move the commentary to a paragraph below it",
	sequence.RuleEntryNoncanonical:  "start the row with the lesson's link",
}

// pathFindings reports every study path's structural diagnostics. A note whose
// type the contract does not list for path behaviour is prose here: the same
// marker in it means nothing, so nothing about it is reported.
func pathFindings(notes []note, roles schema.NavigationRoles) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		if !roles.IsPathType(n.noteType) {
			continue
		}
		for _, d := range n.sequence.Diagnostics {
			out = append(out, pathFinding(n, d))
		}
	}
	return out
}

// pathFinding turns one grammar diagnostic into a wire finding. The message is
// the grammar's own sentence: one rule, one wording, wherever it is read.
func pathFinding(n *note, d sequence.Diagnostic) Finding {
	action, ok := pathRuleAction[d.Rule]
	if !ok {
		panic("judge: unknown study-path rule: " + d.Rule)
	}
	evidence := d.Evidence
	if evidence == "" {
		evidence = "the branch lists rows but declares no part in the course"
	}
	return Finding{
		RuleID:          d.Rule,
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(d.Line),
		Message:         d.Message,
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      "AUTHORING.md",
		Fingerprint:     fingerprint(d.Rule, n.path, evidence),
	}
}

// courseRowLines are the source lines a course's own lesson rows sit on. A
// rule that reconciles a course against disk reads the note's extracted links
// and keeps the ones on those lines, so the link's own context — whether it
// sits under a heading marking the section as planned — travels with it. Two
// rows naming the same note stay distinguishable, which a set of targets could
// not do.
func courseRowLines(n *note) map[int]bool {
	lines := make(map[int]bool)
	var walk func(g *sequence.Group)
	walk = func(g *sequence.Group) {
		projectable := g.Projectable()
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				if projectable && item.Entry.State == sequence.EntryAccepted {
					lines[item.Entry.Line] = true
				}
			case item.Branch != nil:
				if projectable || (g.Role == sequence.RoleStructural && !g.Invalid) {
					walk(item.Branch)
				}
			}
		}
	}
	for _, g := range n.sequence.Groups {
		walk(g)
	}
	return lines
}
