package judge

import "github.com/koopa0/yomihon/internal/vaultfs"

// checkSkipped reports the paths the scan saw and did not index. A vault that
// organises by symbolic link loses notes here without being told: the file
// opens in a file manager, the link resolves, and yet no reading face, no
// listing and no other rule in this command has ever seen it. The severity is
// a warning rather than an error because the folder is not malformed — it
// holds something a note cannot be read out of, and only the author can say
// whether that was meant.
func checkSkipped(scan vaultfs.Scan) []Finding {
	skipped := scan.Skipped()
	out := make([]Finding, 0, len(skipped))
	for _, entry := range skipped {
		out = append(out, skippedFinding(entry.Path(), entry.Kind()))
	}
	return out
}

// skippedFinding names one unindexed path. The kind is carried in the sentence
// and the advice rather than in the rule name, so a vault gains no new rule id
// the day it grows a socket: what a reader does about it is the part that
// differs. The kind's own spelling comes from the scan, which owns that closed
// set, so the bytes a consumer parses have one source.
func skippedFinding(path string, kind vaultfs.SkipKind) Finding {
	message := "this path is not a regular file, so nothing was read from it: it holds no note, answers no link, and appears in no listing"
	action := "replace it with a regular file, or remove it if nothing needs it"
	if kind == vaultfs.SkipSymlink {
		message = "this path is a symbolic link, so nothing was read from it: it holds no note, answers no link, and appears in no listing"
		action = "move the file itself into the vault, and cite it from a note with a wikilink rather than linking to it on disk"
	}
	return Finding{
		RuleID:          "scan.skipped",
		Severity:        SeverityWarn,
		Path:            path,
		Message:         message,
		Evidence:        "the scan observed the path and left it out, because a note is read only out of a regular file: " + kind.String(),
		SuggestedAction: action,
		SourceRule:      sourceYomihon,
		Fingerprint:     fingerprint("scan.skipped", path, kind.String()),
	}
}
