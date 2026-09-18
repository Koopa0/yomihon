package judge

import "slices"

// A file the scan listed and the read could not open is reported here, at the
// weight of the gravest thing this face has, and the judgement goes on over
// everything it did read. The reading room has always shown such a file as a
// row of its own; before this the command answered the same fact by ending the
// run, so two faces gave two answers about one file.
//
// Going on is only honest about part of what the rules conclude. A finding
// about one note is still true when another note could not be read. A finding
// about something being *absent* — a name no file answers to, a lesson no
// course lists — is not: the file nobody could read may hold the name or the
// list. Those rules therefore say nothing at all over such a run, and the file
// that cost them says so in its own sentence, so a reader meets the reason
// rather than a report that is quietly shorter than it looks.

// unreadableRule is the rule id of a file that could not be opened.
const unreadableRule RuleID = "scan.unreadable"

// withheldOnPartialCorpus are the rules that answer from the whole vault: each
// concludes that something is nowhere, and the file that could not be read is
// somewhere the search never reached.
//
//   - link.broken and link.title_not_alias conclude that a written name matches
//     no filename and no alias. The unread file's filename is known — the scan
//     gave it — but its aliases and its title are not, and neither is whether it
//     lists the name as a planned one, which decides the finding's weight.
//   - provenance.unresolved concludes the same about a reference, which may
//     also answer to the unread file's lesson slug.
//   - map.disk_mismatch concludes that a course entry names no note, which is
//     the broken link above under the course's own rule.
//   - map.disk_unlisted concludes that no course of a domain lists a lesson,
//     and the unread file may be the course that lists it.
//
// Every other rule concludes only from material this run holds: a note it read
// whole, or the file membership, which the scan completed before any read was
// attempted. A failed read can leave such a rule with less to report — two
// notes sharing an alias go unnoticed when one of them did not open — but never
// with a wrong verdict, and an under-report is what an unread file is. The
// rules that ask what another note says answer for no note they could not read:
// a live course linking an archived lesson, and a fragment addressing a section
// or a block, each decline that one target rather than guess at it.
var withheldOnPartialCorpus = []RuleID{
	"link.broken",
	"link.title_not_alias",
	"map.disk_mismatch",
	"map.disk_unlisted",
	"provenance.unresolved",
}

// dropWithheldOnPartialCorpus removes every finding whose rule answers from the
// whole vault. The caller applies it only to a run that could not read one, and
// what it removes is stated by the finding the same run carries for the file
// that caused it.
func dropWithheldOnPartialCorpus(findings []Finding) []Finding {
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return slices.Contains(withheldOnPartialCorpus, f.RuleID)
	})
}

// checkUnreadable reports each file the read could not open. A file the
// contract keeps out of agent-facing output cannot be named, and the reason the
// machine gave for it describes the same closed ground, so every such file
// collapses into one fixed sentence that says a hole exists without saying
// where: a run that is missing the whole-vault rules has to say so even when it
// may not say about what.
func checkUnreadable(entries []unreadableEntry, authority scanAuthority) []Finding {
	out := make([]Finding, 0, len(entries))
	withheld := false
	for _, entry := range entries {
		if !authority.egressAllowed(entry.path) {
			withheld = true
			continue
		}
		out = append(out, unreadableFinding(entry.path, entry.cause.Error()))
	}
	if withheld {
		out = append(out, withheldUnreadableFinding())
	}
	return out
}

// unreadableFinding names one file nothing was read from and carries the
// machine's own account of what stopped. The cause is evidence and not part of
// the fingerprint: a file that cannot be opened is one finding whichever way
// the operating system words it, and hashing the wording would move a
// consumer's baseline the day a release reworded an error.
func unreadableFinding(path, cause string) Finding {
	return Finding{
		RuleID:          unreadableRule,
		Severity:        SeverityError,
		Path:            path,
		Message:         "this file could not be read, so nothing was judged from it, and the rules that answer from the whole vault were withheld from this run",
		Evidence:        "the scan observed the path and the read could not open it: " + cause,
		SuggestedAction: "restore read access to the file, or remove it if nothing needs it, then judge the vault again",
		SourceRule:      sourceYomihon,
		Fingerprint:     fingerprint(unreadableRule, path, ""),
	}
}

// withheldUnreadableFinding is the whole of what may be said about a file the
// contract closes: that one exists and could not be read. It carries no path,
// because the path is the closed ground, and the sentence is fixed, so every
// such file and every cause read the same. One stands for all of them — a count
// would describe the folder by arithmetic.
func withheldUnreadableFinding() Finding {
	return Finding{
		RuleID:          unreadableRule,
		Severity:        SeverityError,
		Message:         "a file under a directory this vault's contract withholds from agent-facing output could not be read, so the rules that answer from the whole vault were withheld from this run",
		Evidence:        "naming the file, or the reason the machine gave for it, would describe ground the contract closed",
		SuggestedAction: "check read access under the withheld directories, which only someone at the vault itself can see, then judge the vault again",
		SourceRule:      sourceYomihon,
		Fingerprint:     fingerprint(unreadableRule, "", ""),
	}
}
