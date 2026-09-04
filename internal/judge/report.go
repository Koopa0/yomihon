package judge

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// The human and markdown reports pack the same findings into a scannable
// triage view: a per-domain debt scoreboard, a most-leveraged callout, the
// actionable findings grouped by domain and folded by identity, and a hidden
// count of the informational ones. Both renderers share one packing, so they
// never disagree on the numbers, and both are part of the frozen output.

// domainRoots are the folders under which a note's own folder names its
// knowledge domain. The contract declares them, under the same key the
// frontmatter rule reads, so the report groups a vault the way that vault says
// it is arranged rather than the way this repository's own vault happens to be.
// A vault that files its lessons flat has no domain in those paths, and they
// land under the no-domain heading — which is the truth about the path, not a
// gap in the report.
type domainRoots []string

// nestedLessonRoot is a second root the report reads, and no contract declares
// it. The contract's key takes a first path segment — its loader refuses an
// entry carrying a slash — so a vault filing its lessons under Writing/lessons/
// has no way to say so, and a report reading only the declaration files every
// one of them under the no-domain heading. That is a real loss on the vault this
// product was built for: findings that arrived under two domain headings
// collapse into one.
//
// So it stays, and stays as what it is: a folder layout written down here
// because the declaration cannot carry it. What removes it is a contract key
// that can express a nested root, which is a change to the vocabulary and not
// to this file.
const nestedLessonRoot = "Writing/lessons"

// of infers a finding's knowledge domain from its vault path, and reports
// "(other)" when the path carries no domain. The declared entry is the first
// path segment and the domain is the one under it, matching how the
// frontmatter rule reads the same declaration.
func (r domainRoots) of(path string) string {
	for _, root := range append(slices.Clone(r), nestedLessonRoot) {
		if rest, ok := strings.CutPrefix(path, root+"/"); ok {
			seg, _, _ := strings.Cut(rest, "/")
			return seg
		}
	}
	return "(other)"
}

// packedItem is one folded group of identical findings — same rule and target
// within a domain — carrying the running blast radius (fan-in) of the group.
type packedItem struct {
	severity   Severity
	message    string
	blast      int
	samplePath string
}

// leverageEntry is one high-leverage target: creating it would resolve count
// references, across the listed domains. planned is true only when every
// reference to it is itself a tracked forward-reference.
type leverageEntry struct {
	target  string
	count   int
	planned bool
	domains []string
}

// scoreRow is one domain's error and warn tally on the debt scoreboard.
type scoreRow struct {
	domain string
	errors int
	warns  int
}

// domainSection is one domain's blast-sorted folded items.
type domainSection struct {
	domain string
	items  []packedItem
}

// packed is the report ready to render: the counts, the scoreboard, the
// leverage callout, the per-domain sections, and the hidden split of tracked
// forward-references (planned) and external paths.
type packed struct {
	errors     int
	warns      int
	scoreboard []scoreRow
	leverage   []leverageEntry
	sections   []domainSection
	planned    int
	external   int
}

// pack computes the shared view once. The hidden count is derived from the
// total minus the errors and warns, and the external slice is subtracted from
// it to leave the planned count, so planned plus external always equals hidden
// and the count line can never under-report a hidden finding.
func pack(findings []Finding, roots domainRoots) packed {
	errors, warns := 0, 0
	for i := range findings {
		switch findings[i].Severity {
		case SeverityError:
			errors++
		case SeverityWarn:
			warns++
		case SeverityInfo:
		}
	}
	external := 0
	for i := range findings {
		if findings[i].RuleID == "link.broken.path" && findings[i].Severity == SeverityInfo {
			external++
		}
	}
	hidden := len(findings) - errors - warns
	return packed{
		errors:     errors,
		warns:      warns,
		scoreboard: scoreboard(findings, roots),
		leverage:   leverage(findings, roots),
		sections:   domainSections(findings, roots),
		planned:    hidden - external,
		external:   external,
	}
}

// scoreboard tallies error and warn counts per domain, dropping domains with
// neither, and orders them by total debt: most findings first, then most
// errors, then domain name.
func scoreboard(findings []Finding, roots domainRoots) []scoreRow {
	board := make(map[string][2]int)
	for i := range findings {
		d := roots.of(findings[i].Path)
		slot := board[d]
		switch findings[i].Severity {
		case SeverityError:
			slot[0]++
		case SeverityWarn:
			slot[1]++
		case SeverityInfo:
		}
		board[d] = slot
	}
	var rows []scoreRow
	for _, d := range slices.Sorted(maps.Keys(board)) {
		if board[d][0]+board[d][1] > 0 {
			rows = append(rows, scoreRow{domain: d, errors: board[d][0], warns: board[d][1]})
		}
	}
	slices.SortStableFunc(rows, func(a, b scoreRow) int {
		if c := cmp.Compare(b.errors+b.warns, a.errors+a.warns); c != 0 {
			return c
		}
		if c := cmp.Compare(b.errors, a.errors); c != 0 {
			return c
		}
		return strings.Compare(a.domain, b.domain)
	})
	return rows
}

// leverage aggregates every link finding by its normalized target, keeping only
// targets more than one reference names. A target is planned only when every
// reference to it is planned. The result is ordered by fan-in, then by target.
func leverage(findings []Finding, roots domainRoots) []leverageEntry {
	type agg struct {
		target  string
		count   int
		planned bool
		domains []string
	}
	byTarget := make(map[string]*agg)
	for i := range findings {
		f := &findings[i]
		if f.RuleID != "link.broken" && f.RuleID != "link.title_not_alias" {
			continue
		}
		if f.Target == nil {
			continue
		}
		key := normalizeKey(*f.Target)
		a := byTarget[key]
		if a == nil {
			a = &agg{target: *f.Target, planned: true}
			byTarget[key] = a
		}
		a.count++
		a.planned = a.planned && f.Severity == SeverityInfo
		if d := roots.of(f.Path); !slices.Contains(a.domains, d) {
			a.domains = append(a.domains, d)
		}
	}
	var out []leverageEntry
	for _, key := range slices.Sorted(maps.Keys(byTarget)) {
		a := byTarget[key]
		if a.count <= 1 {
			continue
		}
		slices.Sort(a.domains)
		out = append(out, leverageEntry{target: a.target, count: a.count, planned: a.planned, domains: a.domains})
	}
	slices.SortStableFunc(out, func(a, b leverageEntry) int {
		if c := cmp.Compare(b.count, a.count); c != 0 {
			return c
		}
		return strings.Compare(a.target, b.target)
	})
	return out
}

// domainSections groups the actionable findings — every severity but info — by
// domain, folding identical findings (same rule and target) and summing their
// blast radius. Domains are ordered by total blast; within a domain, items are
// ordered error-first, then by blast, then by message.
func domainSections(findings []Finding, roots domainRoots) []domainSection {
	type folded struct {
		domain string
		item   packedItem
	}
	groups := make(map[string]*folded)
	for i := range findings {
		f := &findings[i]
		if f.Severity == SeverityInfo {
			continue
		}
		d := roots.of(f.Path)
		target := f.Path
		if f.Target != nil {
			target = *f.Target
		}
		key := d + "\x1f" + f.RuleID + "\x1f" + target
		blast := max(len(f.CollisionMembers), 1)
		g := groups[key]
		if g == nil {
			g = &folded{domain: d, item: packedItem{severity: f.Severity, message: f.Message, samplePath: f.Path}}
			groups[key] = g
		}
		g.item.blast += blast
	}
	byDomain := make(map[string][]packedItem)
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		g := groups[key]
		byDomain[g.domain] = append(byDomain[g.domain], g.item)
	}
	out := make([]domainSection, 0, len(byDomain))
	for _, d := range slices.Sorted(maps.Keys(byDomain)) {
		items := byDomain[d]
		slices.SortStableFunc(items, func(a, b packedItem) int {
			if c := cmp.Compare(int(b.severity), int(a.severity)); c != 0 {
				return c
			}
			if c := cmp.Compare(b.blast, a.blast); c != 0 {
				return c
			}
			return strings.Compare(a.message, b.message)
		})
		out = append(out, domainSection{domain: d, items: items})
	}
	slices.SortStableFunc(out, func(a, b domainSection) int {
		if c := cmp.Compare(sumBlast(b.items), sumBlast(a.items)); c != 0 {
			return c
		}
		return strings.Compare(a.domain, b.domain)
	})
	return out
}

// sumBlast totals the blast radius of a domain's items, the key domains order by.
func sumBlast(items []packedItem) int {
	total := 0
	for _, i := range items {
		total += i.blast
	}
	return total
}

// humanReport renders the packed findings for a terminal reader.
func humanReport(findings []Finding, roots domainRoots) string {
	p := pack(findings, roots)
	var s strings.Builder
	fmt.Fprintf(&s, "%d findings: %d error, %d warn, %d hidden (%d planned forward-refs, %d external paths)\n",
		len(findings), p.errors, p.warns, p.planned+p.external, p.planned, p.external)

	if len(p.scoreboard) > 0 {
		s.WriteString("\ndebt by domain:\n")
		for _, r := range p.scoreboard {
			fmt.Fprintf(&s, "  %-20s %d error · %d warn\n", r.domain, r.errors, r.warns)
		}
	}

	if len(p.leverage) > 0 {
		s.WriteString("\nmost leveraged (create one, resolve many):\n")
		for _, l := range p.leverage[:min(5, len(p.leverage))] {
			fmt.Fprintf(&s, "  ×%d [[%s]] (%s) — %s\n", l.count, l.target, leverageTag(l.planned), strings.Join(l.domains, ", "))
		}
	}

	for _, sec := range p.sections {
		fmt.Fprintf(&s, "\n▌ %s\n", sec.domain)
		for _, i := range sec.items {
			fmt.Fprintf(&s, "  [%s] %s%s  (%s)\n", i.severity.String(), blastPrefix(i.blast), i.message, i.samplePath)
		}
	}
	return s.String()
}

// markdownReport renders the packed findings as a fileable markdown note body.
// The frontmatter is deterministic — no timestamp — so the caller stamps and
// routes it, and the tool identity names whatever produced the note.
func markdownReport(findings []Finding, roots domainRoots) string {
	p := pack(findings, roots)
	var s strings.Builder
	s.WriteString("---\ntype: report\ntool: yomihon\n---\n\n# yomihon check\n\n")
	fmt.Fprintf(&s, "%d findings — **%d error**, **%d warn**, %d hidden.\n\n", len(findings), p.errors, p.warns, p.planned+p.external)

	if len(p.scoreboard) > 0 {
		s.WriteString("## Debt by domain\n\n| domain | error | warn |\n|---|--:|--:|\n")
		for _, r := range p.scoreboard {
			fmt.Fprintf(&s, "| %s | %d | %d |\n", escapeMd(r.domain), r.errors, r.warns)
		}
		s.WriteByte('\n')
	}

	if len(p.leverage) > 0 {
		s.WriteString("## Most leveraged (create one, resolve many)\n\n")
		for _, l := range p.leverage[:min(5, len(p.leverage))] {
			fmt.Fprintf(&s, "- **×%d** `[[%s]]` (%s) — %s\n", l.count, l.target, leverageTag(l.planned), escapeMd(strings.Join(l.domains, ", ")))
		}
		s.WriteByte('\n')
	}

	for _, sec := range p.sections {
		fmt.Fprintf(&s, "## %s\n\n", escapeMd(sec.domain))
		for _, i := range sec.items {
			fmt.Fprintf(&s, "- `%s` %s%s — %s\n", i.severity.String(), blastPrefix(i.blast), escapeMd(i.message), escapeMd(i.samplePath))
		}
		s.WriteByte('\n')
	}

	if p.planned+p.external > 0 {
		fmt.Fprintf(&s, "<details><summary>%d tracked forward-references · %d external paths (info)</summary>\n\n", p.planned, p.external)
		for i := range findings {
			f := &findings[i]
			if f.Severity == SeverityInfo {
				fmt.Fprintf(&s, "- `%s` %s — %s\n", f.RuleID, escapeMd(f.Message), escapeMd(f.Path))
			}
		}
		s.WriteString("\n</details>\n")
	}
	return s.String()
}

// leverageTag labels a leverage target planned when every reference to it is a
// tracked forward-reference, and broken otherwise.
func leverageTag(planned bool) string {
	if planned {
		return "planned"
	}
	return "broken"
}

// blastPrefix is the "×N " marker a folded group of more than one carries, and
// empty for a single finding.
func blastPrefix(blast int) string {
	if blast > 1 {
		return fmt.Sprintf("×%d ", blast)
	}
	return ""
}

// escapeMd neutralizes the characters that would break a markdown table (a
// pipe), an HTML details block (angle brackets), or turn the report's own text
// into a live wikilink that pollutes the graph (the bracket pairs).
func escapeMd(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "[[", `[\[`)
	s = strings.ReplaceAll(s, "]]", `]\]`)
	return s
}
