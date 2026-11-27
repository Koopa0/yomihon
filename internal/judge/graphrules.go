package judge

import (
	"iter"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// The graph rules turn the resolved link graph into findings: a wikilink that
// resolves to no note (broken, or resolving only to a title), an alias two
// notes both claim, a provenance reference that names nothing, and a course
// that disagrees with the units on disk. They read the notes and the resolver
// the same way the vault's linker does.

// runGraphRules runs the graph rules over the notes and the resolver, in the
// order the wire format ties break on: link health, fragments, alias
// collisions, name collisions, provenance, alias-resolved citations, then
// course against disk and course structure.
func runGraphRules(a *action, idx *graph.Index, paths *fragmentPaths) []Finding {
	notes, authority := a.notes, a.authority
	titles := titleIndex(notes, authority)
	slugs := slugIndex(notes, authority)
	planned := plannedNamesSet(notes, authority)
	aliases := aliasCollisions(notes, authority)
	roles := authority.contract.NavigationRoles()
	unitType, _ := authority.contract.LessonType()
	return slices.Concat(
		linkHealth(notes, idx, titles, planned, authority, a.partialCorpus()),
		paths.fragmentFindings(notes),
		collisionAlias(aliases),
		collisionName(notes, idx, aliases, authority, a.partialCorpus()),
		provenanceUnresolved(notes, idx, slugs),
		aliasResolvedBasedOn(notes, idx, authority),
		courseDiskMismatch(notes, idx, roles, unitType),
		pathFindings(notes, roles),
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
		if n.title == "" || !authority.gate.Allows(n.path) {
			continue
		}
		key := graph.NormalizeKey(n.title)
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

// slugIndex maps a unit slug to its note path. A reference may name a slug,
// not a filename, so it resolves here rather than through the resolver.
// A repeated slug keeps the last note, matching the reference reading.
func slugIndex(notes []note, authority scanAuthority) map[string]string {
	idx := make(map[string]string)
	for i := range notes {
		if n := &notes[i]; n.slug != "" && authority.gate.Allows(n.path) {
			idx[n.slug] = n.path
		}
	}
	return idx
}

// plannedNamesSet is every concept name listed as planned anywhere in the
// public corpus. A contract-private note is not a source: a name planned only
// there must not soften a public broken link, or a reader of the finding could
// infer that private content names that target.
func plannedNamesSet(notes []note, authority scanAuthority) Planned {
	set := Planned{names: make(map[string]bool)}
	for i := range notes {
		if !authority.gate.Allows(notes[i].path) {
			continue
		}
		set.add(notes[i].plannedNames)
	}
	return set
}

// linkHealth classifies every note's unresolved or suffix-ambiguous wikilinks.
// A course's unit rows are reconciled against disk by the course rule, so they
// are not reported twice here. An embed naming a file is the image rule's; a
// link whose target is some note's title is the title case; any other
// unresolved link is broken. partial says a file in the vault could not be
// read.
func linkHealth(
	notes []note,
	idx *graph.Index,
	titles map[string][]string,
	planned Planned,
	authority scanAuthority,
	partial bool,
) []Finding {
	roles := authority.contract.NavigationRoles()
	var out []Finding
	for i := range notes {
		n := &notes[i]
		// Every link in a course other than its unit rows is ordinary prose.
		units := map[int]bool{}
		if roles.IsPathType(n.noteType) {
			units = courseUnitLinks(n)
		}
		for l := range n.wikilinks {
			link := &n.wikilinks[l]
			if units[link.offset] {
				continue
			}
			targetNotes := titles[graph.NormalizeKey(link.target)]
			standing := Standing(Citation{Target: link.target, Owed: link.underGapHeading}, idx, planned, len(targetNotes) > 0)
			if standing == LinkResolves {
				continue
			}
			if link.embed && render.NamesAFile(link.target) {
				out = appendMissingEmbed(out, n, link, authority, partial)
				continue
			}
			if standing == LinkTitleOnly {
				out = append(out, titleNotAlias(n, link, targetNotes))
			} else {
				out = append(out, brokenLink(n, link, standing == LinkOwed))
			}
		}
	}
	return out
}

// titleNotAlias is a link whose target is a note's title but not its filename
// or alias, which the vault's linker fails to resolve silently. A single holder
// is named directly; several holders make it a collision finding that names
// none of them. Withheld notes are dropped by the title index before anything
// here counts them, so a title held only publicly once reads as singly held.
func titleNotAlias(n *note, link *wikiLink, targetNotes []string) Finding {
	f := Finding{
		RuleID:      RuleLinkTitleNotAlias,
		Severity:    SeverityWarn,
		Path:        n.path,
		Line:        new(link.line),
		Message:     "[[" + link.target + "]] resolves to no filename or alias",
		SourceRule:  sourceYomihon,
		Target:      new(link.target),
		Fingerprint: fingerprint(RuleLinkTitleNotAlias, n.path, link.target),
	}
	if len(targetNotes) > 1 {
		f.Evidence = "the target is the title of " + strconv.Itoa(len(targetNotes)) + " notes: " + strings.Join(targetNotes, ", ")
		f.SuggestedAction = "link the intended note by its full vault-relative path, or add the title to one note's aliases"
		f.CollisionMembers = targetNotes
		return f
	}
	targetNote := targetNotes[0]
	f.Evidence = "the target is the title of " + targetNote + " but not one of its aliases"
	f.SuggestedAction = "add the title to " + targetNote + "'s aliases, or link an existing filename/alias"
	return f
}

// brokenLink is a link that resolves to nothing. It is informational when it is
// owed, a tracked forward-reference, and a warning otherwise.
func brokenLink(n *note, link *wikiLink, tracked bool) Finding {
	severity := SeverityWarn
	evidence := "no filename or alias matches the target"
	action := "create the target note, or change the link to an existing filename/alias"
	if tracked {
		severity = SeverityInfo
		evidence = "a tracked forward-reference (under a gap heading or listed as a planned concept)"
		action = "if it is written, check the filename/alias matches; otherwise leave it tracked"
	}
	return Finding{
		RuleID:          RuleLinkBroken,
		Severity:        severity,
		Path:            n.path,
		Line:            new(link.line),
		Message:         "[[" + link.target + "]] resolves to no note",
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      sourceYomihon,
		Target:          new(link.target),
		Fingerprint:     fingerprint(RuleLinkBroken, n.path, link.target),
	}
}

// appendMissingEmbed adds the image rule's finding for an unresolved embed
// naming a file. The page shows an embedded picture or document in place, so
// one it cannot find is a missing file, not a missing note. The name can still
// be an unread file's alias, so a run that could not read one says nothing
// about it, as it says nothing about a broken link; and a name under ground the
// contract withholds is not judged, because its absence would describe that
// ground.
func appendMissingEmbed(out []Finding, n *note, link *wikiLink, authority scanAuthority, partial bool) []Finding {
	if partial || !authority.gate.Allows(vault.NormalizeNFC(link.target)) {
		return out
	}
	return append(out, missingImage(n, link.line, link.target))
}

// aliasCollisions maps each alias more than one note declares in frontmatter to
// the paths declaring it, sorted. Matching is case-insensitive and NFC; only
// the aliases field counts. A contract-private note is filtered out before the
// count, because censoring a member afterwards would describe it by arithmetic.
func aliasCollisions(notes []note, authority scanAuthority) map[string][]string {
	byAlias := make(map[string][]string)
	for i := range notes {
		n := &notes[i]
		if !authority.gate.Allows(n.path) {
			continue
		}
		for _, alias := range n.aliases {
			key := graph.NormalizeKey(alias)
			if key == "" {
				continue
			}
			if !slices.Contains(byAlias[key], n.path) {
				byAlias[key] = append(byAlias[key], n.path)
			}
		}
	}
	out := make(map[string][]string, len(byAlias))
	for key, members := range byAlias {
		if len(members) > 1 {
			out[key] = slices.Sorted(slices.Values(members))
		}
	}
	return out
}

// collisionAlias reports each alias more than one note declares. [[alias]] then
// resolves to only one of them and the others silently lose inbound links.
func collisionAlias(byAlias map[string][]string) []Finding {
	out := make([]Finding, 0, len(byAlias))
	for _, key := range slices.Sorted(maps.Keys(byAlias)) {
		out = append(out, collisionFinding(key, byAlias[key]))
	}
	return out
}

// collisionName reports each resolvable name more than one file answers to and
// some link is written against, which the resolver refuses to choose between.
// Files sharing a name that no link uses break nothing: a course vault repeats
// main.go and its captured output in every lab, linked by path, and a finding
// per repeated name would bury the rest. The alias rule owns the names it
// already reported, so one repair is not stated twice. It warns rather than
// errors, since a standing condition should not become a red gate by itself.
// Privacy filters the members before anything is counted, and the restatement
// question is asked afterwards, so one repair cannot be split into two rows.
//
// The links that make a name used are read from describable notes alone, for
// the reason a withheld note is no source of planned names: whether a finding
// exists would otherwise say that some withheld note cites the name. A run that
// could not read a note cannot say no link uses a name, so it reports every
// shared name.
func collisionName(notes []note, idx *graph.Index, byAlias map[string][]string, authority scanAuthority, partial bool) []Finding {
	describable := make(map[string][]string)
	for name, members := range idx.Collisions() {
		// The index sorts each name's members, so the kept order is theirs.
		kept := make([]string, 0, len(members))
		for _, member := range members {
			if authority.gate.Allows(member) {
				kept = append(kept, member)
			}
		}
		if len(kept) > 1 {
			describable[name] = kept
		}
	}
	if !partial {
		describable = graph.LinkedNames(describable, citedTargets(notes, authority))
	}
	distinct := graph.WithoutRestatedNames(describable)
	out := make([]Finding, 0, len(distinct))
	for _, name := range slices.Sorted(maps.Keys(distinct)) {
		if _, reported := byAlias[name]; reported {
			continue
		}
		out = append(out, nameCollisionFinding(name, distinct[name]))
	}
	return out
}

// citedTargets are the wikilink and embed targets written in the notes the
// contract lets out, in scan order. A name is cited when any of them folds to
// it.
func citedTargets(notes []note, authority scanAuthority) iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := range notes {
			if !authority.gate.Allows(notes[i].path) {
				continue
			}
			for l := range notes[i].wikilinks {
				if !yield(notes[i].wikilinks[l].target) {
					return
				}
			}
		}
	}
}

// nameCollisionFinding builds one name-collision finding. Like the alias rule's,
// the citing path is the smallest member and the fingerprint keys on the name
// alone, so a third file joining the collision does not read as a new finding.
func nameCollisionFinding(name string, members []string) Finding {
	return Finding{
		RuleID:           RuleCollisionName,
		Severity:         SeverityWarn,
		Path:             members[0],
		Message:          "\"" + name + "\" is the name of " + strconv.Itoa(len(members)) + " files, so [[" + name + "]] cannot resolve deterministically",
		Evidence:         "shared resolution name across: " + strings.Join(members, ", "),
		SuggestedAction:  "rename one of the files, or link each one by its full vault-relative path",
		SourceRule:       sourceYomihon,
		Target:           new(name),
		CollisionMembers: members,
		Fingerprint:      fingerprint(RuleCollisionName, "", name),
	}
}

// collisionFinding builds one alias-collision finding. The citing path is the
// smallest member, and the fingerprint keys on the alias alone — not a member
// path — so a new note joining the collision does not read as a new finding.
func collisionFinding(alias string, members []string) Finding {
	return Finding{
		RuleID:           RuleCollisionAlias,
		Severity:         SeverityWarn,
		Path:             members[0],
		Field:            new("aliases"),
		Message:          "alias \"" + alias + "\" is declared by " + strconv.Itoa(len(members)) + " notes, so [[" + alias + "]] cannot resolve deterministically",
		Evidence:         "shared alias across: " + strings.Join(members, ", "),
		SuggestedAction:  "give the alias a single owner note, or qualify the duplicates",
		SourceRule:       sourceYomihon,
		Target:           new(alias),
		CollisionMembers: members,
		Fingerprint:      fingerprint(RuleCollisionAlias, "", alias),
	}
}

type referenceField struct {
	name   string
	values []string
}

// provenanceUnresolved reports provenance references that resolve to no note.
// The requirement that based_on and related resolve is the product's own,
// declared by no contract key, so the findings cite the product.
func provenanceUnresolved(
	notes []note,
	idx *graph.Index,
	slugs map[string]string,
) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		for _, field := range noteReferenceFields(n) {
			for _, value := range field.values {
				if !provenanceResolves(idx, slugs, value) {
					out = append(out, provenanceFinding(n, field.name, value))
				}
			}
		}
	}
	return out
}

// noteReferenceFields is the shared selection and authority for reference
// resolution and YAML-shape diagnostics.
func noteReferenceFields(n *note) []referenceField {
	return []referenceField{
		{name: "based_on", values: n.basedOn},
		{name: "related", values: n.related},
	}
}

// provenanceResolves reports whether a provenance value resolves, either as a
// wikilink (filename or alias) or as a unit slug. The value may be a bare slug
// or a [[wikilink]] carrying |display, #heading, or ^block, which are stripped
// the same way a body link is. A value that strips to nothing counts as
// resolved, so a bare anchor never invents a finding.
func provenanceResolves(idx *graph.Index, slugs map[string]string, value string) bool {
	target, ok := referenceTarget(value)
	if !ok {
		return true
	}
	if idx.Names(target) {
		return true
	}
	if _, listed := slugs[target]; listed {
		return true
	}
	_, listed := slugs[graph.NormalizeKey(target)]
	return listed
}

func provenanceFinding(n *note, field, value string) Finding {
	return Finding{
		RuleID:          RuleProvenanceUnresolved,
		Severity:        SeverityWarn,
		Path:            n.path,
		Field:           new(field),
		Message:         field + " -> " + value + " resolves to nothing",
		Evidence:        "no note, alias, or unit slug matches the reference",
		SuggestedAction: "fix the reference, or create the target note",
		SourceRule:      sourceYomihon,
		Target:          new(value),
		Fingerprint:     fingerprint(RuleProvenanceUnresolved, n.path, value),
	}
}

// aliasResolvedBasedOn reports a thought's based_on that reaches its note only
// through an alias. A note that was replaced is deleted and its old name goes
// into its successor's aliases, so the citation still resolves; but the
// thought was written about the text of the note it named, and that text is
// gone. The reader may never have read what the name now leads to. Only a
// thought is asked: it is the one note whose reaction is evidence about the
// words it cites.
func aliasResolvedBasedOn(notes []note, idx *graph.Index, authority scanAuthority) []Finding {
	thought := authority.contract.NavigationRoles().AnswerType()
	if thought == "" {
		return nil
	}
	var out []Finding
	for i := range notes {
		n := &notes[i]
		if n.noteType != thought {
			continue
		}
		for _, value := range n.basedOn {
			target, ok := referenceTarget(value)
			if !ok {
				continue
			}
			if resolved, viaAlias := idx.ResolvesOnlyByAlias(target); viaAlias {
				out = append(out, aliasResolvedFinding(n, value, resolved))
			}
		}
	}
	return out
}

// referenceTarget is the name a frontmatter reference cites: the words inside
// its wikilink brackets, or the bare value, with any |display, #heading or
// ^block taken off the way a body link is read. ok is false for a value that
// strips to nothing.
func referenceTarget(value string) (string, bool) {
	v := strings.TrimSpace(value)
	inner := v
	if len(v) >= 4 && strings.HasPrefix(v, "[[") && strings.HasSuffix(v, "]]") {
		inner = v[2 : len(v)-2]
	}
	return stripTarget(inner)
}

func aliasResolvedFinding(n *note, value, resolved string) Finding {
	return Finding{
		RuleID:          RuleProvenanceAliasResolved,
		Severity:        SeverityWarn,
		Path:            n.path,
		Field:           new("based_on"),
		Message:         "based_on -> " + value + " resolves only through an alias",
		Evidence:        "the cited name is an alias of another note, not the name of a file, so the note it reaches may have replaced the one this thought was written from",
		SuggestedAction: "read the note the alias leads to and cite it by its own name, or rewrite the thought against it",
		SourceRule:      sourceYomihon,
		Target:          new(value),
		ResolvedTo:      new(resolved),
		Fingerprint:     fingerprint(RuleProvenanceAliasResolved, n.path, value),
	}
}

// courseDiskMismatch reconciles courses against the units on disk. Its first
// direction reports a course entry that resolves to nothing; its second
// reports a unit of a domain that no course of that domain lists. Listed is
// collected across every course of the domain before that second walk, so a
// unit another parallel course owns is not unlisted. The second walk emits one
// finding per such unit, attached to the first course of the domain in check
// order — the file the reader opens next to add the unit. A unit declared a
// draft is expected work in progress and is not reported at all.
func courseDiskMismatch(
	notes []note,
	idx *graph.Index,
	roles schema.NavigationRoles,
	unitType string,
) []Finding {
	byDomain := unitsByDomain(notes, unitType)
	listedByDomain := make(map[string]map[string]bool)
	var out []Finding
	for i := range notes {
		course := &notes[i]
		if !roles.IsPathType(course.noteType) {
			continue
		}
		listed, mismatches := reconcileCourse(course, idx)
		out = append(out, mismatches...)
		if course.domain == "" {
			continue
		}
		union := listedByDomain[course.domain]
		if union == nil {
			listedByDomain[course.domain] = listed
			continue
		}
		maps.Copy(union, listed)
	}
	seenDomain := make(map[string]bool)
	for i := range notes {
		course := &notes[i]
		if !roles.IsPathType(course.noteType) || course.domain == "" {
			continue
		}
		if seenDomain[course.domain] {
			continue
		}
		seenDomain[course.domain] = true
		out = append(out, unlistedUnits(
			course,
			byDomain[course.domain],
			listedByDomain[course.domain],
		)...)
	}
	return out
}

// unitsByDomain groups every unit note by its domain. A supplementary file
// without a domain is not a unit node and self-excludes.
func unitsByDomain(notes []note, unitType string) map[string][]*note {
	byDomain := make(map[string][]*note)
	for i := range notes {
		if n := &notes[i]; n.noteType == unitType && n.domain != "" {
			byDomain[n.domain] = append(byDomain[n.domain], n)
		}
	}
	return byDomain
}

// reconcileCourse reports one course's entries that resolve to nothing, and
// the set of units it lists. The unlisted walk waits until every course of the
// domain has been collected.
func reconcileCourse(course *note, idx *graph.Index) (map[string]bool, []Finding) {
	var out []Finding
	listed := make(map[string]bool)
	units := courseUnitLinks(course)
	for l := range course.wikilinks {
		link := &course.wikilinks[l]
		if !units[link.offset] {
			continue
		}
		res := idx.Resolve(link.target)
		switch res.Kind {
		case graph.KindUnique:
			listed[res.RelPath] = true
		case graph.KindAmbiguous:
			// A name two files share is the collision rule's to report. A path
			// suffix several files end with is a name none of them carries, so
			// no collision reports it and the row lists a note that is missing.
			if !idx.Claimed(link.target) {
				out = append(out, courseListsMissing(course, link))
			}
		case graph.KindUnresolved:
			out = append(out, courseListsMissing(course, link))
		default:
			panic("judge: unknown graph.Kind: " + res.Kind.String())
		}
	}
	return listed, out
}

// unlistedUnits reports each unit of the course's domain that no course of
// that domain lists and that is not a draft. The listed set is the domain
// union, already collected. The caller invokes this once per domain, against
// the first course in check order, so one unlisted unit is one finding.
func unlistedUnits(
	course *note,
	units []*note,
	listed map[string]bool,
) []Finding {
	var out []Finding
	for _, unit := range units {
		if !listed[unit.path] && !unit.draft {
			out = append(out, diskUnlisted(course, unit))
		}
	}
	return out
}

// courseListsMissing is a course entry that resolves to nothing — a course
// promising a note that does not exist. It is informational under a gap
// heading and a warning otherwise.
func courseListsMissing(course *note, link *wikiLink) Finding {
	severity := SeverityWarn
	if link.underGapHeading {
		severity = SeverityInfo
	}
	return Finding{
		RuleID:          RuleCourseDiskMismatch,
		Severity:        severity,
		Path:            course.path,
		Line:            new(link.line),
		Message:         "course links [[" + link.target + "]] but it resolves to nothing",
		Evidence:        "a course entry that resolves to no note on disk",
		SuggestedAction: "create the note, fix the entry, or mark it a planned gap",
		SourceRule:      sourceYomihon,
		Target:          new(link.target),
		Fingerprint:     fingerprint(RuleCourseDiskMismatch, course.path, link.target),
	}
}

// diskUnlisted is a unit on disk, not declared a draft, that no course of its
// domain lists. Writing a unit before adding it to a course is normal, so it is
// reported at warning level; whether a warning stops a run belongs to whoever
// starts it.
func diskUnlisted(course, unit *note) Finding {
	return Finding{
		RuleID:          RuleCourseDiskUnlisted,
		Severity:        SeverityWarn,
		Path:            unit.path,
		Message:         "unit is on disk but not listed in course " + course.path,
		Evidence:        "the unit exists but the course for its domain does not list it",
		SuggestedAction: "add the unit to the course, or confirm it is intentionally excluded",
		SourceRule:      sourceYomihon,
		Target:          new(unit.path),
		ResolvedTo:      new(course.path),
		Fingerprint:     fingerprint(RuleCourseDiskUnlisted, unit.path, course.path),
	}
}
