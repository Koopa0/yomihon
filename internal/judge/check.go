package judge

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
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
//
// A cancelled ctx stops the scan: the contract load, the file walk and every
// note read are checked against it, so a caller that gives up is not left
// waiting on a whole vault.
func Check(ctx context.Context, root string) ([]Finding, error) {
	return runCheckAction(ctx, root, nil, false)
}

// runCheckAction is the whole-engine scan, with the two knobs a command line
// can turn on top of what Check describes: a scope filter keeping only findings
// that touch one of the given paths, and all, which keeps the findings touching
// nothing outside System/. Neither widens what is read — the graph is built
// from the whole vault either way — and neither reaches a path the contract
// withholds, which is dropped ahead of both.
func runCheckAction(ctx context.Context, root string, paths []string, all bool) ([]Finding, error) {
	a, err := openAction(ctx, root, actionHooks{})
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
	findings = append(findings, checkKnowledgeScope(a.scan, a.authority.contract)...)

	findings = dropEgressDenied(findings, a.authority)
	if !all {
		findings = dropSystemScoped(findings)
	}
	if len(paths) > 0 {
		filtered, ferr := filterByPaths(findings, paths, a.scan, a.authority)
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
		inputs[i] = graph.NoteInput{RelPath: notes[i].path, Aliases: notes[i].aliases}
	}
	return graph.BuildFromNotes(inputs, resources)
}

// dropSystemScoped removes findings every path of which — the citing path and
// any collision member — lies under System/. A finding is kept when at least
// one path it touches is outside System/.
func dropSystemScoped(findings []Finding) []Finding {
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return !touchesOutsideSystem(&f)
	})
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
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return touchesEgressDenied(&f, authority)
	})
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
// prefix or lies beneath it. Each prefix is normalized the way the scan
// canonicalizes what it observed — forward slashes, no trailing slash, no
// leading "./", composed form — so a path typed at a shell matches the vault's
// own spelling of it, including one written in decomposed form by a Mac
// keyboard.
//
// A prefix that cannot match anything is refused rather than filtered with. An
// argument that is empty or only slashes names no path at all; one that names
// something the scan never observed is a scope this command cannot see; and one
// the contract withholds from agent-facing output is a scope this command may
// see but may report nothing from. All three would otherwise return no findings
// and exit as though the scope were clean, which is the one answer an
// adjudication face must never give about ground it did not cover — a typo, or
// a private directory, would read as a verdict. The vault root is the exception
// it looks like: it names everything, so it filters nothing out, and a whole
// folder read carries no claim about any one directory inside it.
//
// The withheld check runs before the observation check on purpose. Answering
// "names nothing in this vault" for a path inside a private directory would
// make the pair of refusals an existence oracle over exactly the directory the
// contract closed; one uniform answer for every path under it tells the caller
// only what their own contract already says.
func filterByPaths(findings []Finding, paths []string, scan vault.Scan, authority scanAuthority) ([]Finding, error) {
	prefixes := make([]string, len(paths))
	for i, p := range paths {
		prefixes[i] = canonicalPathFilter(p)
		if prefixes[i] == "" {
			return nil, fmt.Errorf("path filter %q resolves to no path; give a vault-relative path", p)
		}
		if prefixes[i] != vaultRoot && !authority.egressAllowed(prefixes[i]) {
			return nil, fmt.Errorf("path filter %q lies under a directory this vault's contract withholds from agent-facing output; the scope was scanned but nothing from it can be reported, and an empty answer would read as a clean verdict", p)
		}
		if !scan.Contains(prefixes[i]) {
			return nil, fmt.Errorf("path filter %q names nothing in this vault; give a vault-relative path such as %s, or drop it to judge the whole vault", p, vaultRelativeExample)
		}
	}
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return !anyTouchedPath(&f, func(p string) bool { return underAnyPrefix(p, prefixes) })
	}), nil
}

// vaultRelativeExample stands in for a real path in the refusal above. It is a
// shape rather than a name taken from any particular vault: the directories a
// reader keeps notes in are theirs to name, and quoting one they do not have
// would send them looking for it.
const vaultRelativeExample = `"Notes" or "Notes/topic.md"`

// vaultRoot is how a scan spells the folder it was opened on. A reader who
// types it means the whole vault.
const vaultRoot = "."

// canonicalPathFilter rewrites a path typed on the command line into the form
// the scan gives what it observed, so the two can be compared as strings.
// Windows separators become slashes, a trailing slash and a leading "./" are
// dropped as the shell's punctuation rather than part of the name, and the
// result is composed: a Mac keyboard hands over CJK filenames in decomposed
// form, which is a different string for the same folder.
func canonicalPathFilter(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimRight(p, "/")
	for strings.HasPrefix(p, "./") {
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimRight(p, "/")
	}
	return vault.NormalizeNFC(p)
}

// underAnyPrefix reports whether p equals one of the prefixes or lies directly
// beneath it (the character after the prefix is a slash), so a prefix of
// "Concepts/go" matches "Concepts/go" and "Concepts/go/x.md" but not
// "Concepts/golang". The vault root is above every path and so matches all of
// them; no finding's path is spelled with it, so comparing as a prefix would
// match none.
func underAnyPrefix(p string, prefixes []string) bool {
	for _, w := range prefixes {
		if w == vaultRoot || p == w || (strings.HasPrefix(p, w) && p[len(w)] == '/') {
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
