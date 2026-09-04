package judge

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// Check scans the vault rooted at root for corpus-level findings and returns
// them in the deterministic wire order. The graph is always built from the
// whole vault; findings touching a contract-declared private path are dropped
// in every scope, and the default scope also drops findings touching only
// System/ files. A missing or privacy-incomplete contract is an error, because
// agent-facing output has no authority without one. A cancelled ctx stops the
// scan at the contract load, the walk, or any note read.
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
	findings = append(findings, checkSkipped(a.scan)...)

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
// Each is canonicalized the way the scan canonicalizes what it observed, so a
// decomposed spelling still matches the vault's own. A prefix that cannot match
// anything is refused rather than filtered with, since an empty answer would
// read as a clean verdict over ground never covered; the refusals are ordered
// — shape, then withheld, then unobserved — so none becomes an existence oracle.
func filterByPaths(findings []Finding, paths []string, scan vaultfs.Scan, authority scanAuthority) ([]Finding, error) {
	if err := scopeIsWrittenFromTheVaultRoot(paths); err != nil {
		return nil, err
	}
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

// scopeIsWrittenFromTheVaultRoot refuses a scope written as an absolute path. A
// scope names part of the vault the way the vault spells it, from the vault's
// own root, so an absolute one names nothing this face can hold. It reads the
// argument's shape and not what is on disk at it, which keeps it truthful
// wherever it is called and keeps this face out of the filesystem.
func scopeIsWrittenFromTheVaultRoot(scopes []string) error {
	for _, p := range scopes {
		if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
			continue
		}
		return fmt.Errorf(
			"path filter %q is an absolute path, and a filter names part of the vault from the vault's own root, such as %s",
			p, vaultRelativeExample)
	}
	return nil
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
