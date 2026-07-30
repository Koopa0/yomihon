package judge

import (
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

// The graph rules turn the resolved link graph into findings: a wikilink that
// resolves to no note (broken, or resolving only to a title), an alias two
// notes both claim, a provenance reference that names nothing, and a syllabus
// that disagrees with the lessons on disk. They read the notes and the resolver
// the same way the vault's linker does; the diagnostic strings are frozen.

// normalizeKey folds a name to the key its resolution is stored under, so the
// title, alias, and slug indexes built here agree with the resolver about what
// a written name matches. It is the resolver's own function rather than a
// local reproduction of its steps: the two were identical by maintenance, and
// any drift between them would show up as this judge and the reading page
// disagreeing about a name neither of them could see was ambiguous.
func normalizeKey(name string) string {
	return graph.NormalizeKey(name)
}

// runGraphRules runs the graph rules over the notes and the resolver, in the
// order the wire format ties break on: link health, alias collisions,
// provenance, then map-vs-disk.
func runGraphRules(notes []note, idx *graph.Index, authority scanAuthority) []Finding {
	titles := titleIndex(notes, authority)
	slugs := slugIndex(notes, authority)
	planned := plannedNamesSet(notes, authority)
	return slices.Concat(
		linkHealth(notes, idx, titles, planned),
		collisionAlias(notes, authority),
		provenanceUnresolved(notes, idx, slugs, authority.contract),
		mapDiskMismatch(notes, idx),
		supersessionFindings(notes, idx, authority),
	)
}

// titleIndex maps a normalized note title to the paths that declare it, sorted.
// Titles are not resolution keys, so a link to a title that is neither a
// filename nor an alias does not resolve; this index recovers which note the
// author meant.
func titleIndex(notes []note, authority scanAuthority) map[string][]string {
	idx := make(map[string][]string)
	for i := range notes {
		n := &notes[i]
		if n.title == "" || !authority.egressAllowed(n.path) {
			continue
		}
		key := normalizeKey(n.title)
		if key == "" {
			continue
		}
		idx[key] = append(idx[key], n.path)
	}
	for key := range idx {
		slices.Sort(idx[key])
	}
	return idx
}

// slugIndex maps a lesson slug to its note path. Supersession references name a
// slug, not a filename, so they resolve here rather than through the resolver.
// A repeated slug keeps the last note, matching the reference reading.
func slugIndex(notes []note, authority scanAuthority) map[string]string {
	idx := make(map[string]string)
	for i := range notes {
		if n := &notes[i]; n.slug != "" && authority.egressAllowed(n.path) {
			idx[n.slug] = n.path
		}
	}
	return idx
}

// plannedNamesSet is the set of every concept name listed as planned anywhere
// in the public corpus, harvested from the names this scan already collected
// per note. A contract-private note is not a source: a name planned only there
// must not soften a public broken link, or a reader of the finding could infer
// that private content names that target. That narrowing is this face's alone —
// see NewPlanned for why the reading page draws from the whole vault instead.
func plannedNamesSet(notes []note, authority scanAuthority) Planned {
	set := make(Planned)
	for i := range notes {
		if !authority.egressAllowed(notes[i].path) {
			continue
		}
		set.add(notes[i].plannedNames)
	}
	return set
}

// linkHealth classifies every note's unresolved wikilinks. A study-path's links
// are its course list, owned by the map rule, so they are not double-reported
// here. A link whose target is some note's title is the title case; any other
// unresolved link is broken.
func linkHealth(
	notes []note,
	idx *graph.Index,
	titles map[string][]string,
	planned Planned,
) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		if n.noteType == "study-path" {
			continue
		}
		for _, link := range n.wikilinks {
			if idx.Resolve(link.target).Kind != graph.Unresolved {
				continue
			}
			if targetNotes, ok := titles[normalizeKey(link.target)]; ok {
				out = append(out, titleNotAlias(n, link, targetNotes))
			} else {
				out = append(out, brokenLink(n, link, planned))
			}
		}
	}
	return out
}

// titleNotAlias is a link whose target is a note's title but not its filename
// or alias, which the vault's linker fails to resolve silently.
func titleNotAlias(n *note, link wikiLink, targetNotes []string) Finding {
	targetNote := ""
	if len(targetNotes) > 0 {
		targetNote = targetNotes[0]
	}
	return Finding{
		RuleID:          "link.title_not_alias",
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(link.line),
		Message:         "[[" + link.target + "]] resolves to no filename or alias",
		Evidence:        "the target is the title of " + targetNote + " but not one of its aliases",
		SuggestedAction: "add the title to " + targetNote + "'s aliases, or link an existing filename/alias",
		SourceRule:      "Note-Schema.md#aliases",
		Target:          new(link.target),
		Fingerprint:     fingerprint("link.title_not_alias", n.path, link.target),
	}
}

// brokenLink is a link that resolves to nothing. It is informational when it is
// a tracked forward-reference — under a gap heading, or naming a planned
// concept — and a warning otherwise.
func brokenLink(n *note, link wikiLink, planned Planned) Finding {
	tracked := link.underGapHeading || planned.Has(link.target)
	severity := SeverityWarn
	evidence := "no filename or alias matches the target"
	action := "create the target note, or change the link to an existing filename/alias"
	if tracked {
		severity = SeverityInfo
		evidence = "a tracked forward-reference (under a gap heading or listed as a planned concept)"
		action = "if it is written, check the filename/alias matches; otherwise leave it tracked"
	}
	return Finding{
		RuleID:          "link.broken",
		Severity:        severity,
		Path:            n.path,
		Line:            new(link.line),
		Message:         "[[" + link.target + "]] resolves to no note",
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      "Note-Schema.md#aliases",
		Target:          new(link.target),
		Fingerprint:     fingerprint("link.broken", n.path, link.target),
	}
}

// collisionAlias reports each alias more than one note declares in frontmatter.
// [[alias]] then resolves to only one of them and the others silently lose
// inbound links. Matching is case-insensitive and NFC, across all note kinds;
// only the aliases field counts, not prose mentions.
func collisionAlias(notes []note, authority scanAuthority) []Finding {
	byAlias := make(map[string][]string)
	for i := range notes {
		n := &notes[i]
		if !authority.egressAllowed(n.path) {
			continue
		}
		for _, alias := range n.aliases {
			key := normalizeKey(alias)
			if key == "" {
				continue
			}
			if !slices.Contains(byAlias[key], n.path) {
				byAlias[key] = append(byAlias[key], n.path)
			}
		}
	}
	aliasKeys := make([]string, 0, len(byAlias))
	for key, members := range byAlias {
		if len(members) > 1 {
			aliasKeys = append(aliasKeys, key)
		}
	}
	slices.Sort(aliasKeys)
	out := make([]Finding, 0, len(aliasKeys))
	for _, key := range aliasKeys {
		members := slices.Clone(byAlias[key])
		slices.Sort(members)
		out = append(out, collisionFinding(key, members))
	}
	return out
}

// collisionFinding builds one alias-collision finding. The citing path is the
// smallest member, and the fingerprint keys on the alias alone — not a member
// path — so a new note joining the collision does not read as a new finding.
func collisionFinding(alias string, members []string) Finding {
	path := ""
	if len(members) > 0 {
		path = members[0]
	}
	return Finding{
		RuleID:           "collision.alias",
		Severity:         SeverityWarn,
		Path:             path,
		Field:            new("aliases"),
		Message:          "alias \"" + alias + "\" is declared by " + strconv.Itoa(len(members)) + " notes, so [[" + alias + "]] cannot resolve deterministically",
		Evidence:         "shared alias across: " + strings.Join(members, ", "),
		SuggestedAction:  "give the alias a single owner note, or qualify the duplicates",
		SourceRule:       "vault-schema.toml#rules",
		Target:           new(alias),
		CollisionMembers: members,
		Fingerprint:      fingerprint("collision.alias", "", alias),
	}
}

type referenceField struct {
	name       string
	values     []string
	sourceRule string
}

// provenanceUnresolved reports configured provenance and replacement-ledger
// references that resolve to no note. The fixed based_on and related fields
// retain the inherited judge contract; replacement field names come only from
// the loaded vault contract.
func provenanceUnresolved(
	notes []note,
	idx *graph.Index,
	slugs map[string]string,
	contract *schema.Contract,
) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		fields := []referenceField{
			{name: "based_on", values: n.basedOn, sourceRule: "vault-schema.toml#provenance"},
			{name: "related", values: n.related, sourceRule: "vault-schema.toml#provenance"},
		}
		if vocabulary, ok := supersessionForNote(contract, n); ok {
			fields = appendConfiguredReferences(fields, n, vocabulary)
		}
		for _, field := range fields {
			for _, value := range field.values {
				if !provenanceResolves(idx, slugs, value) {
					out = append(out, provenanceFinding(n, field.name, value, field.sourceRule))
				}
			}
		}
	}
	return out
}

func supersessionForNote(contract *schema.Contract, n *note) (schema.Supersession, bool) {
	if contract == nil || contract.StatusGroup(n.noteType) == "" {
		return schema.Supersession{}, false
	}
	return contract.Supersession()
}

func appendConfiguredReferences(
	fields []referenceField,
	n *note,
	vocabulary schema.Supersession,
) []referenceField {
	configured := []string{vocabulary.GeneralLinkField}
	if n.noteType == "lesson" {
		configured = []string{vocabulary.PredecessorField, vocabulary.SuccessorField}
	}
	for _, field := range configured {
		if field == "based_on" || field == "related" {
			continue
		}
		fields = append(fields, referenceField{
			name:       field,
			values:     frontmatterStringValues(n, field),
			sourceRule: "vault-schema.toml#supersession",
		})
	}
	return fields
}

func frontmatterStringValues(n *note, field string) []string {
	value, ok := n.frontmatter[field]
	if !ok {
		return nil
	}
	return value.stringValues()
}

// provenanceResolves reports whether a provenance value resolves, either as a
// wikilink (filename or alias) or as a lesson slug. The value may be a bare slug
// or a [[wikilink]] carrying |display, #heading, or ^block, which are stripped
// the same way a body link is. A value that strips to nothing counts as
// resolved, so a bare anchor never invents a finding.
func provenanceResolves(idx *graph.Index, slugs map[string]string, value string) bool {
	v := strings.TrimSpace(value)
	inner := v
	if len(v) >= 4 && strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
		inner = v[2 : len(v)-2]
	}
	target, ok := stripTarget(inner)
	if !ok {
		return true
	}
	if idx.Resolve(target).Kind != graph.Unresolved {
		return true
	}
	if _, listed := slugs[target]; listed {
		return true
	}
	_, listed := slugs[normalizeKey(target)]
	return listed
}

func provenanceFinding(n *note, field, value, sourceRule string) Finding {
	return Finding{
		RuleID:          "provenance.unresolved",
		Severity:        SeverityWarn,
		Path:            n.path,
		Field:           new(field),
		Message:         field + " -> " + value + " resolves to nothing",
		Evidence:        "no note, alias, or lesson slug matches the reference",
		SuggestedAction: "fix the reference, or create the target note",
		SourceRule:      sourceRule,
		Target:          new(value),
		Fingerprint:     fingerprint("provenance.unresolved", n.path, value),
	}
}

// mapDiskMismatch reconciles each study-path against the lessons on disk. Its
// first direction reports a syllabus link that resolves to nothing; its second
// reports a lesson of the syllabus's domain that exists on disk but is not
// listed. A draft or curriculum-gap lesson is expected work-in-progress and is
// not reported at all.
func mapDiskMismatch(notes []note, idx *graph.Index) []Finding {
	byDomain := lessonsByDomain(notes)
	var out []Finding
	for i := range notes {
		if syllabus := &notes[i]; syllabus.noteType == "study-path" {
			out = append(out, reconcileSyllabus(syllabus, idx, byDomain)...)
		}
	}
	return out
}

// lessonsByDomain groups every lesson note by its domain. A supplementary file
// without a domain is not a lesson node and self-excludes.
func lessonsByDomain(notes []note) map[string][]*note {
	byDomain := make(map[string][]*note)
	for i := range notes {
		if n := &notes[i]; n.noteType == "lesson" && n.domain != "" {
			byDomain[n.domain] = append(byDomain[n.domain], n)
		}
	}
	return byDomain
}

// reconcileSyllabus reports one study-path's disagreements with disk: each of
// its links that resolves to nothing, then each lesson of its domain that
// exists but is not listed, skipping draft and curriculum-gap lessons.
func reconcileSyllabus(syllabus *note, idx *graph.Index, byDomain map[string][]*note) []Finding {
	var out []Finding
	listed := make(map[string]bool)
	for _, link := range syllabus.wikilinks {
		res := idx.Resolve(link.target)
		switch res.Kind {
		case graph.Unique:
			listed[res.Path] = true
		case graph.Ambiguous:
			// An ambiguous link resolves to some note; leave it to the collision rule.
		case graph.Unresolved:
			out = append(out, syllabusListsMissing(syllabus, link))
		default:
			panic("judge: unknown graph.Kind: " + strconv.Itoa(int(res.Kind)))
		}
	}
	if syllabus.domain == "" {
		return out
	}
	for _, lesson := range byDomain[syllabus.domain] {
		expected := lesson.status == "draft" || lesson.sourceKind == "curriculum-gap"
		if !listed[lesson.path] && !expected {
			out = append(out, diskUnlisted(syllabus, lesson))
		}
	}
	return out
}

// syllabusListsMissing is a study-path entry that resolves to nothing — a
// syllabus promising a note that does not exist. It is informational under a
// planned-gap heading and a warning otherwise.
func syllabusListsMissing(syllabus *note, link wikiLink) Finding {
	severity := SeverityWarn
	if link.underGapHeading {
		severity = SeverityInfo
	}
	return Finding{
		RuleID:          "map.disk_mismatch",
		Severity:        severity,
		Path:            syllabus.path,
		Line:            new(link.line),
		Message:         "syllabus links [[" + link.target + "]] but it resolves to nothing",
		Evidence:        "a study-path entry that resolves to no note on disk",
		SuggestedAction: "create the note, fix the entry, or mark it a planned gap",
		SourceRule:      "vault-schema.toml#rules",
		Target:          new(link.target),
		Fingerprint:     fingerprint("map.disk_mismatch", syllabus.path, link.target),
	}
}

// diskUnlisted is a non-draft, non-gap lesson on disk that the syllabus for its
// domain does not list. It is advisory and never gates, since writing a lesson
// before adding it to the syllabus is normal.
func diskUnlisted(syllabus, lesson *note) Finding {
	return Finding{
		RuleID:          "map.disk_unlisted",
		Severity:        SeverityWarn,
		Path:            lesson.path,
		Message:         "lesson is on disk but not listed in syllabus " + syllabus.path,
		Evidence:        "the lesson exists but the study-path for its domain does not list it",
		SuggestedAction: "add the lesson to the syllabus, or confirm it is intentionally excluded",
		SourceRule:      "vault-schema.toml#rules",
		Target:          new(lesson.path),
		ResolvedTo:      new(syllabus.path),
		Fingerprint:     fingerprint("map.disk_unlisted", lesson.path, syllabus.path),
	}
}
