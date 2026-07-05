package judge

import (
	"os"
	"path/filepath"
	"strings"
)

// The disk-reference rule resolves markdown [text](path) links and backticked
// path tokens against the filesystem, the only check that does. A relative path
// that stays inside the vault root but has no file is a real dead link and a
// warning; a path that escapes the root cannot be checked the same way on every
// machine, so it is reported informational — "external, not stat'd" — and never
// as broken, which keeps the check from ever claiming a false dead link.

// checkDiskRefs stats every note's path references against root and returns the
// findings.
func checkDiskRefs(notes []note, root string) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		noteDir := ""
		if idx := strings.LastIndexByte(n.path, '/'); idx >= 0 {
			noteDir = n.path[:idx]
		}
		for _, pref := range n.pathRefs {
			if f, ok := classifyPathRef(n, noteDir, pref, root); ok {
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
func classifyPathRef(n *note, noteDir string, pref pathRef, root string) (Finding, bool) {
	if pref.code {
		rootRel, rootOK := resolveWithinRoot("", pref.target)
		noteRel, noteOK := resolveWithinRoot(noteDir, pref.target)
		if (rootOK && fileExists(root, rootRel)) || (noteOK && fileExists(root, noteRel)) {
			return Finding{}, false
		}
		if !rootOK {
			return Finding{}, false
		}
		return deadInRoot(n, pref, rootRel), true
	}
	rel, ok := resolveWithinRoot(noteDir, pref.target)
	if !ok {
		return externalRef(n, pref), true
	}
	if fileExists(root, rel) {
		return Finding{}, false
	}
	return deadInRoot(n, pref, rel), true
}

// resolveWithinRoot resolves dest against baseDir — both vault-relative and
// slash-separated — collapsing "." and "..". It returns the normalized
// vault-relative path, or false when the reference climbs above the root.
func resolveWithinRoot(baseDir, dest string) (string, bool) {
	var comps []string
	for _, c := range strings.Split(baseDir, "/") {
		if c != "" {
			comps = append(comps, c)
		}
	}
	for _, part := range strings.Split(dest, "/") {
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
	return strings.Join(comps, "/"), true
}

// fileExists reports whether rel names an existing path under root.
func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- rel is a vault-relative reference resolved within the operator's own local vault root, not untrusted input
	return err == nil
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
		SourceRule:      "vault-schema.toml#rules",
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
		SourceRule:      "vault-schema.toml#rules",
		Target:          new(pref.target),
		Fingerprint:     fingerprint("link.broken.path", n.path, pref.target),
	}
}
