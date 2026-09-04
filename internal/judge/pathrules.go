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

// pathRuleAction names what to do about each rule, in the author's terms. The
// wording belongs to this face — the grammar states what is wrong and this
// table states what to do next — but the key set belongs to the grammar, and a
// test holds this table to every rule the grammar can report.
var pathRuleAction = map[sequence.Rule]string{
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
// the grammar's own sentence: one rule, one wording, wherever it is read. A
// rule this table has no advice for still reaches the author, carrying that
// sentence with a general action, rather than dying on an ordinary vault.
func pathFinding(n *note, d sequence.Diagnostic) Finding {
	action, ok := pathRuleAction[d.Rule]
	if !ok {
		action = "resolve the study-path problem the message describes"
	}
	evidence := d.Evidence
	if evidence == "" {
		evidence = "the branch lists rows but declares no part in the course"
	}
	return Finding{
		RuleID:          string(d.Rule),
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(d.Line),
		Message:         d.Message,
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      sourceYomihon,
		Fingerprint:     fingerprint(string(d.Rule), n.path, evidence),
	}
}

// courseLessonLinks are the exact wikilinks a course lists as its lessons: the
// body offset of each accepted row's target, as the grammar read it, so the
// link's own context travels with it. The identity is the occurrence, not the
// name and not the line: a name can be written on two rows, and a row can hold
// an embed or a mention beside the lesson it names.
func courseLessonLinks(n *note) map[int]bool {
	offsets := make(map[int]bool)
	var walk func(g *sequence.Group)
	walk = func(g *sequence.Group) {
		projectable := g.Projectable()
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				// An accepted row always resolves to one written link, so its
				// span is set; the guard keeps an unset span from claiming
				// offset zero rather than trusting that it cannot happen.
				if projectable && item.Entry.State == sequence.EntryAccepted && !item.Entry.TargetSpan.Zero() {
					offsets[item.Entry.TargetSpan.Start] = true
				}
			case item.Branch != nil:
				if projectable || g.Carries() {
					walk(item.Branch)
				}
			}
		}
	}
	for _, g := range n.sequence.Groups {
		walk(g)
	}
	return offsets
}
