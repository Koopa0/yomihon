package judge

import (
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// The disk-reference rule resolves markdown [text](path) links and backticked
// path tokens against the action's complete captured file and directory
// membership. A relative path that stays inside the vault root but was absent
// from that observation is a real dead link and a warning; a path that escapes
// the root cannot be checked the same way on every machine, so it is reported
// informational — "external, not stat'd" — and never as broken.

// checkDiskRefs resolves every note's path references against the action's
// complete captured membership and returns the findings.
func checkDiskRefs(notes []note, scan vault.Scan, authority scanAuthority) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		noteDir := ""
		if idx := strings.LastIndexByte(n.path, '/'); idx >= 0 {
			noteDir = n.path[:idx]
		}
		for _, pref := range n.pathRefs {
			if f, ok := classifyPathRef(n, noteDir, pref, scan, authority); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// classifyPathRef resolves one reference and, when it is dead, returns the
// finding. A backticked token is written vault-root-relative and accepted if it
// exists either root-relative or note-relative; a markdown link is
// note-relative, and one that escapes the root is reported external rather than
// stat'd.
func classifyPathRef(
	n *note,
	noteDir string,
	pref pathRef,
	scan vault.Scan,
	authority scanAuthority,
) (Finding, bool) {
	return classifyPathRefWithContains(n, noteDir, pref, authority, scan.Contains)
}

// classifyPathRefWithContains keeps privacy authorization ahead of every
// membership observation. The injected predicate lets the lock test prove a
// denied target is rejected without consulting the captured file domain.
func classifyPathRefWithContains(
	n *note,
	noteDir string,
	pref pathRef,
	authority scanAuthority,
	contains func(string) bool,
) (Finding, bool) {
	if pref.code {
		return classifyCodeRef(n, noteDir, pref, authority, contains)
	}
	return classifyProseRef(n, noteDir, pref, authority, contains)
}

// classifyCodeRef judges a reference written inside code, which may name either
// the vault root or the note's own directory, so it is broken only when neither
// reading finds anything.
func classifyCodeRef(
	n *note,
	noteDir string,
	pref pathRef,
	authority scanAuthority,
	contains func(string) bool,
) (Finding, bool) {
	rootRel, rootOK := resolveWithinRoot("", pref.target)
	noteRel, noteOK := resolveWithinRoot(noteDir, pref.target)
	// The privacy gate runs before membership is inspected at all, so a
	// restricted path is never even looked up.
	if (rootOK && !authority.egressAllowed(rootRel)) ||
		(noteOK && !authority.egressAllowed(noteRel)) {
		return Finding{}, false
	}
	if (rootOK && contains(rootRel)) || (noteOK && contains(noteRel)) {
		return Finding{}, false
	}
	if !rootOK || vault.OutsideScan(rootRel) {
		return Finding{}, false
	}
	return deadInRoot(n, pref, rootRel), true
}

// classifyProseRef judges a reference written in prose, which names a path
// relative to the note that carries it.
func classifyProseRef(
	n *note,
	noteDir string,
	pref pathRef,
	authority scanAuthority,
	contains func(string) bool,
) (Finding, bool) {
	rel, ok := resolveWithinRoot(noteDir, pref.target)
	if !ok {
		return externalRef(n, pref), true
	}
	if !authority.egressAllowed(rel) || contains(rel) {
		return Finding{}, false
	}
	// The scan never visits a hidden path, so it holds no evidence about one.
	// Calling such a link broken would report the scan's own boundary as a
	// missing file — and the file it named may be sitting right there.
	if vault.OutsideScan(rel) {
		return Finding{}, false
	}
	return deadInRoot(n, pref, rel), true
}

// resolveWithinRoot resolves dest against baseDir — both vault-relative and
// slash-separated — collapsing "." and "..". It returns the normalized
// vault-relative path, or false when the reference climbs above the root.
func resolveWithinRoot(baseDir, dest string) (string, bool) {
	var comps []string
	for c := range strings.SplitSeq(baseDir, "/") {
		if c != "" {
			comps = append(comps, c)
		}
	}
	for part := range strings.SplitSeq(dest, "/") {
		switch part {
		case "", ".":
			// A no-op segment.
		case "..":
			if len(comps) == 0 {
				return "", false
			}
			comps = comps[:len(comps)-1]
		default:
			comps = append(comps, part)
		}
	}
	return vault.NormalizeNFC(strings.Join(comps, "/")), true
}

// deadInRoot is a relative path that stays inside the vault but has no file.
func deadInRoot(n *note, pref pathRef, resolved string) Finding {
	return Finding{
		RuleID:          "link.broken.path",
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(pref.line),
		Message:         "link to " + pref.target + " resolves to no file",
		Evidence:        resolved + " does not exist in the vault",
		SuggestedAction: "fix the path, restore the file, or remove the reference",
		SourceRule:      sourceContractRules,
		Target:          new(pref.target),
		Fingerprint:     fingerprint("link.broken.path", n.path, pref.target),
	}
}

// externalRef is a path that escapes the vault root — reported but not stat'd,
// to stay deterministic across machines.
func externalRef(n *note, pref pathRef) Finding {
	return Finding{
		RuleID:          "link.broken.path",
		Severity:        SeverityInfo,
		Path:            n.path,
		Line:            new(pref.line),
		Message:         "link to " + pref.target + " points outside the vault root",
		Evidence:        "external path, not stat'd (existence varies by environment)",
		SuggestedAction: "if it should be in the vault, fix the path; otherwise informational",
		SourceRule:      sourceContractRules,
		Target:          new(pref.target),
		Fingerprint:     fingerprint("link.broken.path", n.path, pref.target),
	}
}
