package judge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

// Check scans the vault rooted at root for corpus-level findings — broken and
// title-only links, alias collisions, unresolved provenance, syllabus-vs-disk
// mismatches, dead file references, and frontmatter schema violations — and
// returns them in the deterministic wire order. The graph is always built from
// the whole vault; findings touching a contract-declared private path are
// dropped in every scope, and the default scope additionally drops findings
// that touch only System/ files, which cite reference material rather than
// carry live links. A missing, malformed, or privacy-incomplete schema contract
// is an error because agent-facing output has no authority without it.
func Check(root string) ([]Finding, error) {
	return runCheckAction(root, nil, false)
}

// check is the whole-engine scan behind the check command. The graph is always
// built from the entire vault; paths and all only decide which findings are
// kept. Findings that touch a contract-declared private path are always
// dropped, whatever all is. Without all, findings that touch only System/ are
// dropped, since those files cite reference material rather than carry live
// links. When paths are given, a finding is kept only if one of the paths it
// touches lies at or below one of them. The findings are returned in the
// deterministic wire order.
func check(root string, paths []string, all bool) ([]Finding, error) {
	return runCheckAction(root, paths, all)
}

func runCheckAction(root string, paths []string, all bool) ([]Finding, error) {
	a, err := openAction(root, actionHooks{})
	if err != nil {
		return nil, err
	}
	findings, err := checkAction(a, paths, all)
	if err != nil {
		return nil, a.abort(err)
	}
	if err := a.finish(); err != nil {
		return nil, err
	}
	return findings, nil
}

func checkAction(a *action, paths []string, all bool) ([]Finding, error) {
	idx := buildIndex(a.notes, a.resources)

	findings := runGraphRules(a.notes, idx, a.authority)
	findings = append(findings, checkDiskRefs(a.notes, a.scan, a.authority)...)
	schemaFindings, err := checkSchema(a.notes, a.authority.contract)
	if err != nil {
		return nil, err
	}
	findings = append(findings, schemaFindings...)

	findings = dropEgressDenied(findings, a.authority)
	if !all {
		findings = dropSystemScoped(findings)
	}
	if len(paths) > 0 {
		filtered, ferr := filterByPaths(findings, paths)
		if ferr != nil {
			return nil, ferr
		}
		findings = filtered
	}
	sortFindings(findings)
	return findings, nil
}

// buildIndex builds the wikilink resolver from the collected notes and
// resources, keying each note by its path forms and its genuine string aliases.
func buildIndex(notes []note, resources []string) *graph.Index {
	inputs := make([]graph.NoteInput, len(notes))
	for i := range notes {
		inputs[i] = graph.NoteInput{Path: notes[i].path, Aliases: notes[i].aliases}
	}
	return graph.BuildFromNotes(inputs, resources)
}

// dropSystemScoped removes findings every path of which — the citing path and
// any collision member — lies under System/. A finding is kept when at least
// one path it touches is outside System/.
func dropSystemScoped(findings []Finding) []Finding {
	out := findings[:0]
	for i := range findings {
		if touchesOutsideSystem(&findings[i]) {
			out = append(out, findings[i])
		}
	}
	return out
}

// touchesOutsideSystem reports whether a finding touches any path outside
// System/, counting its citing path and every collision member.
func touchesOutsideSystem(f *Finding) bool {
	return anyTouchedPath(f, func(p string) bool {
		return !strings.HasPrefix(p, "System/")
	})
}

// dropEgressDenied removes every finding whose resolution touches a path the
// contract forbids from agent-facing output. Private notes are still scanned so
// a public link resolves consistently, but their paths never surface in a
// finding and the drop holds even for the full, unfiltered set.
func dropEgressDenied(findings []Finding, authority scanAuthority) []Finding {
	out := findings[:0]
	for i := range findings {
		if !touchesEgressDenied(&findings[i], authority) {
			out = append(out, findings[i])
		}
	}
	return out
}

// touchesEgressDenied reports whether a finding's resolution touches a
// contract-private path: the note it cites, a collision member, or the note a
// link resolved to. The link's own target text is the citing author's words
// and is deliberately not counted.
func touchesEgressDenied(f *Finding, authority scanAuthority) bool {
	denied := func(relPath string) bool { return !authority.egressAllowed(relPath) }
	if anyTouchedPath(f, denied) {
		return true
	}
	return f.ResolvedTo != nil && denied(*f.ResolvedTo)
}

// filterByPaths keeps only findings that touch one of the given path prefixes.
// A finding is kept when its citing path or any collision member equals a
// prefix or lies beneath it. Each prefix is normalized to forward slashes with
// trailing slashes trimmed, matching how a path argument is given on the
// command line. A prefix that normalizes to nothing — an empty argument, or one
// that is only slashes — is rejected rather than silently matching nothing,
// so a caller cannot mistake an empty filter for a clean scan.
func filterByPaths(findings []Finding, paths []string) ([]Finding, error) {
	prefixes := make([]string, len(paths))
	for i, p := range paths {
		prefixes[i] = strings.TrimRight(strings.ReplaceAll(p, `\`, "/"), "/")
		if prefixes[i] == "" {
			return nil, fmt.Errorf("path filter %q resolves to no path; give a vault-relative path", p)
		}
	}
	out := findings[:0]
	for i := range findings {
		if anyTouchedPath(&findings[i], func(p string) bool { return underAnyPrefix(p, prefixes) }) {
			out = append(out, findings[i])
		}
	}
	return out, nil
}

// underAnyPrefix reports whether p equals one of the prefixes or lies directly
// beneath it (the character after the prefix is a slash), so a prefix of
// "Concepts/go" matches "Concepts/go" and "Concepts/go/x.md" but not
// "Concepts/golang".
func underAnyPrefix(p string, prefixes []string) bool {
	for _, w := range prefixes {
		if p == w || (strings.HasPrefix(p, w) && p[len(w)] == '/') {
			return true
		}
	}
	return false
}

// anyTouchedPath reports whether pred holds for any path the finding touches:
// its citing path or any collision member.
func anyTouchedPath(f *Finding, pred func(string) bool) bool {
	return pred(f.Path) || slices.ContainsFunc(f.CollisionMembers, pred)
}
