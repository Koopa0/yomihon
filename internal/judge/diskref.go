package judge

import (
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// The disk-reference rule resolves markdown [text](path) links and backticked
// path tokens against the action's captured file and directory membership. A
// relative path inside the root but absent from that observation is a dead link
// and a warning; one that escapes the root cannot be checked the same way on
// every machine, so it is reported informational and never as broken.

// checkDiskRefs resolves every note's path references against the action's
// complete captured membership and returns the findings.
func checkDiskRefs(notes []note, scan vault.Scan, authority scanAuthority, idx *graph.Index) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		noteDir := ""
		if idx := strings.LastIndexByte(n.path, '/'); idx >= 0 {
			noteDir = n.path[:idx]
		}
		for _, pref := range n.pathRefs {
			if f, ok := classifyCapturedPathRef(n, noteDir, pref, diskRefContext{index: idx, authority: authority, contains: scan.Contains}); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

type diskRefContext struct {
	index     *graph.Index
	authority scanAuthority
	contains  func(string) bool
}

func classifyCapturedPathRef(n *note, noteDir string, pref pathRef, observation diskRefContext) (Finding, bool) {
	if pref.code {
		return classifyCodeRef(n, noteDir, pref, observation.authority, observation.contains)
	}
	if !observation.authority.egressAllowed(n.path) {
		return Finding{}, false
	}
	result := observation.index.ResolveMarkdown(n.path, pref.target, observation.authority.egressAllowed, observation.contains)
	if !result.Local || result.Invalid || result.Withheld {
		return Finding{}, false
	}
	if result.Outside {
		return externalRef(n, pref), true
	}
	if result.Kind == graph.KindUnique {
		return Finding{}, false
	}
	if result.Kind == graph.KindAmbiguous {
		finding := deadInRoot(n, pref, result.Relative)
		finding.Message = "link to " + pref.target + " names several files"
		finding.Evidence = "several targets: " + strings.Join(result.Candidates, ", ")
		finding.CollisionMembers = result.Candidates
		return finding, true
	}
	return deadInRoot(n, pref, result.Relative), true
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
		SourceRule:      sourceYomihon,
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
		SourceRule:      sourceYomihon,
		Target:          new(pref.target),
		Fingerprint:     fingerprint("link.broken.path", n.path, pref.target),
	}
}
