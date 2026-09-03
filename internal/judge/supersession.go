package judge

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

const (
	predecessorNotArchivedRule = "supersession.predecessor_not_archived"
	archivedNavigationRule     = "supersession.archived_navigation_target"
)

func supersessionFindings(notes []note, idx *graph.Index, authority scanAuthority) []Finding {
	vocabulary, ok := authority.contract.Supersession()
	if !ok {
		return nil
	}
	artifacts := authority.contract.ArtifactPolicy().ValidateSource()
	if !artifacts.Available() {
		return nil
	}

	findings := predecessorNotArchived(notes, authority, artifacts, vocabulary)
	roles := authority.roles()
	if !roles.Available() {
		return findings
	}
	return append(findings, archivedNavigationTargets(notes, idx, authority, roles, artifacts, vocabulary)...)
}

func predecessorNotArchived(
	notes []note,
	authority scanAuthority,
	artifacts schema.ArtifactPolicy,
	vocabulary schema.Supersession,
) []Finding {
	var findings []Finding
	for i := range notes {
		n := &notes[i]
		if n.noteType != "lesson" ||
			!authority.egressAllowed(n.path) ||
			artifacts.IsNonInstance(n.path) {
			continue
		}
		if !slices.Contains(authority.contract.Statuses(n.noteType), n.status) ||
			n.status == vocabulary.ArchivedStatus ||
			len(frontmatterStringValues(n, vocabulary.SuccessorField)) == 0 {
			continue
		}
		findings = append(findings, predecessorStatusFinding(n, vocabulary))
	}
	return findings
}

func predecessorStatusFinding(n *note, vocabulary schema.Supersession) Finding {
	field := vocabulary.SuccessorField
	return Finding{
		RuleID:          predecessorNotArchivedRule,
		Severity:        SeverityWarn,
		Path:            n.path,
		Field:           &field,
		Message:         field + ` is populated while status is "` + n.status + `", not "` + vocabulary.ArchivedStatus + `"`,
		Evidence:        "the configured successor ledger marks this lesson as superseded",
		SuggestedAction: "archive the predecessor, or clear the successor field until the replacement is authoritative",
		SourceRule:      sourceContractSupersession,
		Fingerprint:     fingerprint(predecessorNotArchivedRule, n.path, field),
	}
}

func archivedNavigationTargets(
	notes []note,
	idx *graph.Index,
	authority scanAuthority,
	roles schema.NavigationRoles,
	artifacts schema.ArtifactPolicy,
	vocabulary schema.Supersession,
) []Finding {
	byPath := make(map[string]*note, len(notes))
	for i := range notes {
		byPath[notes[i].path] = &notes[i]
	}

	var findings []Finding
	for i := range notes {
		source := &notes[i]
		if !authority.egressAllowed(source.path) ||
			artifacts.IsNonInstance(source.path) ||
			(!roles.IsPathType(source.noteType) && !roles.IsMapType(source.noteType)) ||
			!liveStatus(authority.contract, source, vocabulary.ArchivedStatus) {
			continue
		}
		links := navigationLinks(source, roles)
		for l := range links {
			link := &links[l]
			resolution := idx.Resolve(link.target)
			if resolution.Kind != graph.KindUnique {
				continue
			}
			target := byPath[resolution.RelPath]
			if target == nil ||
				!authority.egressAllowed(target.path) ||
				artifacts.IsNonInstance(target.path) ||
				authority.contract.StatusGroup(target.noteType) == "" ||
				target.status != vocabulary.ArchivedStatus {
				continue
			}
			findings = append(findings, archivedNavigationFinding(source, target, link, vocabulary))
		}
	}
	return findings
}

func liveStatus(contract *schema.Contract, n *note, archivedStatus string) bool {
	return n.status != archivedStatus && slices.Contains(contract.Statuses(n.noteType), n.status)
}

func archivedNavigationFinding(
	source, target *note,
	link *wikiLink,
	vocabulary schema.Supersession,
) Finding {
	return Finding{
		RuleID:          archivedNavigationRule,
		Severity:        SeverityWarn,
		Path:            source.path,
		Line:            new(link.line),
		Message:         "[[" + link.target + "]] resolves to an archived note",
		Evidence:        `the live path or map links ` + target.path + `, whose status is "` + vocabulary.ArchivedStatus + `"`,
		SuggestedAction: "replace or remove the link, or restore the target if it is current again",
		SourceRule:      sourceContractSupersession,
		Target:          new(link.target),
		ResolvedTo:      new(target.path),
		Fingerprint:     fingerprint(archivedNavigationRule, source.path, link.target),
	}
}

// navigationLinks are the links through which a note points at another. A
// course points only by listing a lesson, so a link in its prose or in a row
// the grammar refused points nowhere; a general map declares no sequence and
// keeps answering for every link it carries.
func navigationLinks(source *note, roles schema.NavigationRoles) []wikiLink {
	if !roles.IsPathType(source.noteType) {
		return source.wikilinks
	}
	lessons := courseLessonLinks(source)
	out := make([]wikiLink, 0, len(source.wikilinks))
	for _, link := range source.wikilinks {
		if lessons[link.offset] {
			out = append(out, link)
		}
	}
	return out
}
