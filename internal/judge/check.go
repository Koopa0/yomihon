package judge

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/schema"
	"github.com/koopa0/kurodo/internal/vault"
)

// Check scans the vault rooted at root for corpus-level findings — broken and
// title-only links, alias collisions, unresolved provenance, syllabus-vs-disk
// mismatches, dead file references, and frontmatter schema violations — and
// returns them in the deterministic wire order. The graph is always built from
// the whole vault; findings touching the private daily journal are dropped in
// every scope, and the default scope additionally drops findings that touch
// only System/ files, which cite reference material rather than carry live
// links. A missing schema contract skips the frontmatter checks; a malformed
// one is an error.
func Check(root string) ([]Finding, error) {
	return check(root, nil, false)
}

// check is the whole-engine scan behind the check command. The graph is always
// built from the entire vault; paths and all only decide which findings are
// kept. Findings that touch the private daily journal are always dropped,
// whatever all is. Without all, findings that touch only System/ are dropped,
// since those files cite reference material rather than carry live links. When
// paths are given, a finding is kept only if one of the paths it touches lies at
// or below one of them. The findings are returned in the deterministic wire
// order.
func check(root string, paths []string, all bool) ([]Finding, error) {
	notes, resources, err := collectVault(root)
	if err != nil {
		return nil, err
	}
	idx := buildIndex(notes, resources)

	findings := runGraphRules(notes, idx)
	findings = append(findings, checkDiskRefs(notes, root)...)
	switch s, serr := schema.Load(root); {
	case serr == nil:
		schemaFindings, cerr := checkSchema(notes, s)
		if cerr != nil {
			return nil, cerr
		}
		findings = append(findings, schemaFindings...)
	case errors.Is(serr, fs.ErrNotExist):
		// A vault with no contract skips the frontmatter checks.
	default:
		return nil, fmt.Errorf("load contract: %w", serr)
	}

	findings = dropDiaryScoped(findings)
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

// dropDiaryScoped removes every finding that touches the private daily journal:
// a finding is dropped when its citing path or any collision member begins with
// Diary/. The journal is still scanned so its links resolve for other notes,
// but its contents never surface in a finding a reader would see, and the drop
// holds even when the full, unfiltered set is requested.
func dropDiaryScoped(findings []Finding) []Finding {
	out := findings[:0]
	for i := range findings {
		if !touchesDiary(&findings[i]) {
			out = append(out, findings[i])
		}
	}
	return out
}

// touchesDiary reports whether a finding touches the private daily journal —
// its citing path or any collision member beginning with Diary/.
func touchesDiary(f *Finding) bool {
	return anyTouchedPath(f, func(p string) bool {
		return strings.HasPrefix(p, "Diary/")
	})
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

// collectVault walks the vault once, parsing every markdown file into a note
// and keeping every other file as a linkable resource. It reuses the shared
// walk, so the scan boundary is the resolver's: dot-prefixed directories are
// skipped, a .gitignore is never consulted, and every path is NFC-normalized.
func collectVault(root string) (notes []note, resources []string, err error) {
	paths, err := vault.List(root)
	if err != nil {
		return nil, nil, err
	}
	for _, rel := range paths {
		if filepath.Ext(rel) != ".md" {
			resources = append(resources, rel)
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- rel is a vault-relative path from the shared walk over the operator's own local vault, not untrusted input
		if err != nil {
			return nil, nil, fmt.Errorf("read note %s: %w", rel, err)
		}
		notes = append(notes, parseNote(rel, data))
	}
	return notes, resources, nil
}
