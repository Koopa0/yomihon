package pages

import (
	"cmp"
	"fmt"
	"net/url"
	"slices"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// HealthColumn is one column of the findings table, and the closed set a reader
// may order the table by. The word a link carries and the column it orders by
// are the same string, so there is no second spelling to keep in step.
type HealthColumn string

const (
	// HealthByFinding keeps each kind of finding together. It is the order the
	// page has always listed findings in, and the one an unreadable request
	// falls back to.
	HealthByFinding HealthColumn = "finding"
	// HealthByFile brings every finding about one file together, which is the
	// question a reader deciding what to open next is asking.
	HealthByFile HealthColumn = "file"
	// HealthBySeverity puts the heaviest findings first. Rows the judge weighs
	// nothing by follow them, in their own order.
	HealthBySeverity HealthColumn = "severity"
	// HealthByCount puts the files carrying the most of one kind first.
	HealthByCount HealthColumn = "count"
)

// healthColumns is every column, in the order the header row prints them.
var healthColumns = []HealthColumn{HealthByFile, HealthByFinding, HealthBySeverity, HealthByCount}

// ParseHealthColumn reads the ordering a request asked for. The set is closed:
// a word outside it leaves the table in its default order rather than in one
// nobody can name, because a request that cannot be honoured is better answered
// with the page than with an argument about it.
func ParseHealthColumn(value string) HealthColumn {
	if slices.Contains(healthColumns, HealthColumn(value)) {
		return HealthColumn(value)
	}
	return HealthByFinding
}

// name is what the header row calls this column.
func (c HealthColumn) name(lang wording.Lang) string {
	switch c {
	case HealthByFile:
		return wording.HealthColumnFile.In(lang)
	case HealthByFinding:
		return wording.HealthColumnFinding.In(lang)
	case HealthBySeverity:
		return wording.HealthColumnSeverity.In(lang)
	case HealthByCount:
		return wording.HealthColumnCount.In(lang)
	}
	return string(c)
}

// href is the link the header carries. It names only the ordering, so following
// one from any state of the page reaches the same table ordered that way — the
// first page of it, because reordering the table makes a page number name
// different rows and carrying one over would leave the reader somewhere they
// did not ask to be.
func (c HealthColumn) href() string { return "?sort=" + string(c) }

// pageHref is the address of one stretch of the table in the ordering in
// force, so stepping through a long report keeps the order it was being read
// in. It is relative, like the ordering links beside it, which leaves the
// page's own route spelled where routes are spelled.
func (c HealthColumn) pageHref(n PageNumber) string {
	return "?" + url.Values{"sort": {string(c)}, "page": {n.String()}}.Encode()
}

// direction is what a reader is told the active column is ordered by. The two
// numeric columns put the largest first, because a page about what needs repair
// opens on the worst of it; the file column reads from the top down. Grouping
// by kind is neither: it is the order the kinds are declared in, which ascends
// by nothing a reader could name, and the vocabulary's word for an order it
// cannot describe is the honest answer rather than a direction that is not
// true.
func (c HealthColumn) direction() string {
	switch c {
	case HealthBySeverity, HealthByCount:
		return "descending"
	case HealthByFile:
		return "ascending"
	default:
		return "other"
	}
}

// healthSortedAttrs marks the header of the column the table is actually
// ordered by, and says which way, so the ordering is announced rather than left
// to be inferred from the rows. Every other header carries nothing.
func healthSortedAttrs(col, active HealthColumn) templ.Attributes {
	if col != active {
		return nil
	}
	return templ.Attributes{"aria-sort": col.direction()}
}

// healthKind is one sort of finding the page gathers. The order these are
// declared in is the order the table lists them in when nothing reorders it,
// and the order the guide under the table explains them in.
type healthKind int

const (
	healthBlocked healthKind = iota
	healthSkipped
	healthUnwritten
	healthTitleOnly
	healthIsland
	healthUnreadableFrontmatter
	healthSchemaFault
	healthStatusOutsideEnum
	healthStatusUnreachable
	healthCollision
)

// healthKinds is every kind, in that order.
var healthKinds = []healthKind{
	healthBlocked, healthSkipped, healthUnwritten, healthTitleOnly, healthIsland,
	healthUnreadableFrontmatter, healthSchemaFault, healthStatusOutsideEnum,
	healthStatusUnreachable, healthCollision,
}

// title is what the finding column calls this kind, and what the guide lists it
// under.
func (k healthKind) title(lang wording.Lang) string {
	switch k {
	case healthBlocked:
		return wording.BlockedTitle.In(lang)
	case healthSkipped:
		return wording.SkippedTitle.In(lang)
	case healthUnwritten:
		return wording.UnwrittenTitle.In(lang)
	case healthTitleOnly:
		return wording.TitleOnlyTitle.In(lang)
	case healthIsland:
		return wording.IslandsTitle.In(lang)
	case healthUnreadableFrontmatter:
		return wording.HealthFrontmatterTitle.In(lang)
	case healthSchemaFault:
		return wording.HealthSchemaTitle.In(lang)
	case healthStatusOutsideEnum:
		return wording.StatusOutsideEnumTitle.In(lang)
	case healthStatusUnreachable:
		return wording.StatusUnreachableTitle.In(lang)
	case healthCollision:
		return wording.CollisionsTitle.In(lang)
	}
	return ""
}

// healthRule is the judging face's account of a kind of finding: the rule it
// reports the same thing under, and the weight that rule gives it. The page
// states the weight so a reader can sort by it, and names the rule so the two
// faces can be put side by side and shown to agree.
type healthRule struct {
	id       judge.RuleID
	severity judge.Severity
}

// healthRules is that account, for the kinds whose every row weighs the same.
// What it records is the rule a kind of finding is reported under and the
// weight that rule gives it, not a claim that the two faces list the same
// files: the reading gathers what it could not open or would not index over
// the whole folder, and the judging face reads a narrower corpus.
//
// Two kinds are weighed without being here. Frontmatter that cannot be read and
// frontmatter the schema rejects both carry the weight the judging face gave
// that note, which differs from note to note, so it is set where the row is
// made instead.
//
// One kind carries no weight at all: the note nothing cites. No rule reports
// it, so its rows say nothing rather than a weight this page invented for them.
//
// The broken-link entry is the weight that rule gives an untracked target. The
// list this page gathers holds only those: a target under a gap heading or in
// the planned ledger is tracked, is weighed lighter, and never reaches a row
// here.
var healthRules = map[healthKind]healthRule{
	healthBlocked:           {"scan.unreadable", judge.SeverityError},
	healthSkipped:           {"scan.skipped", judge.SeverityWarn},
	healthUnwritten:         {"link.broken", judge.SeverityWarn},
	healthTitleOnly:         {"link.title_not_alias", judge.SeverityWarn},
	healthStatusOutsideEnum: {"schema.enum", judge.SeverityError},
	healthStatusUnreachable: {"schema.status_unreachable", judge.SeverityError},
	healthCollision:         {"collision.name", judge.SeverityWarn},
}

// healthDetail is one piece of a finding's evidence: words whose author wrote
// them — a link target, a status value, the error a read returned — and the
// note they point at where there is one.
type healthDetail struct {
	// Text is shown as written, in whatever language its author wrote it.
	Text string
	// Machine marks text a machine produced, which is set in the machinery's
	// own face rather than in the reading one.
	Machine bool
	// Link is a note this piece of evidence names. A zero relative path means
	// it names none.
	Link nav.NoteRef
}

// healthRow is one line of the findings table: one kind of finding about one
// file, however many of that kind the file carries.
type healthRow struct {
	Kind healthKind
	// File is the note the finding is about. A zero relative path means the
	// finding is about a path that is no note, and FilePath carries it.
	File     nav.NoteRef
	FilePath string
	Detail   []healthDetail
	// Severity is the weight the judging face gives a finding of this kind.
	// Weighed is false where no rule covers it, and the cell is then empty.
	Severity judge.Severity
	Weighed  bool
	// Count is how many findings of this kind the file carries, never less
	// than one: a row exists because something was found.
	Count int
}

// subject is the words the file column shows, which is also what ordering by
// that column compares.
func (r *healthRow) subject() string {
	if r.File.RelPath != "" {
		return r.File.Name
	}
	return r.FilePath
}

// weight orders a row against another by severity. A row no rule weighs sorts
// after every row one does, rather than posing as the lightest kind of finding.
func (r *healthRow) weight() int {
	if !r.Weighed {
		return -1
	}
	return int(r.Severity)
}

// healthFileKey identifies the file a row is about, for counting distinct
// files rather than for display. A note keeps its vault-relative path; a path
// that is no note keeps the string the row already carries. The two live in
// separate fields rather than one shared string so a source path can never be
// counted as the same file as a note whose relative path happens to read the
// same.
type healthFileKey struct {
	relPath  string
	filePath string
}

// fileKey is this row's identity for that count.
func (r *healthRow) fileKey() healthFileKey {
	return healthFileKey{relPath: r.File.RelPath, filePath: r.FilePath}
}

// healthTally is one kind of finding present on the page, with how many of it
// there are. The guide under the table is made of these.
type healthTally struct {
	Kind  healthKind
	Count int
}

// healthPageSize is how many findings one page of the table holds. A finding
// row is one line at a desk and a stack of four on a phone, so twenty-five is
// about two screen-heights either way: the strip under the table is reached in
// one scroll, and a folder carrying a few hundred findings comes apart into
// pages a reader can walk instead of one page nobody reaches the foot of.
const healthPageSize = 25

// divide is the stretch of the table this request asked for, and the strip
// that leads to the rest of it. The rows are gathered and ordered whole before
// they are divided, so every tally taken beside them — the shape line above,
// the guide below — answers for the report rather than for the page, the way
// the search page's divisions answer for the whole search.
func (v *HealthView) divide(rows []healthRow) ([]healthRow, Pager) {
	strip := NewPager(v.Page, healthPageSize, len(rows), v.Sort.pageHref)
	return rows[strip.First:strip.Last], strip
}

// healthRange names which rows of the table are on this page, and how many
// there are in all. strip is read, never kept, so the parameter is a pointer
// only to avoid copying the pager's own address list on every call.
func healthRange(strip *Pager, lang wording.Lang) string {
	return fmt.Sprintf(wording.HealthRangeFmt.In(lang), strip.First+1, strip.Last, strip.Total)
}

// rows is the whole table: every finding the view holds, one row per file per
// kind, ordered by what the request asked for. Several findings of one kind
// about one file share a row and are counted there, so a file carrying twelve
// broken links is one line a reader can act on rather than twelve.
func (v *HealthView) rows(lang wording.Lang) []healthRow {
	out := v.gather(lang)
	switch v.Sort {
	case HealthByFile:
		slices.SortStableFunc(out, func(a, b healthRow) int { return cmp.Compare(a.subject(), b.subject()) })
	case HealthBySeverity:
		slices.SortStableFunc(out, func(a, b healthRow) int { return cmp.Compare(b.weight(), a.weight()) })
	case HealthByCount:
		slices.SortStableFunc(out, func(a, b healthRow) int { return cmp.Compare(b.Count, a.Count) })
	case HealthByFinding:
	}
	return out
}

// gather builds the rows in the page's own order, which is the order the kinds
// are declared in and, inside a kind, the order the vault reading produced.
func (v *HealthView) gather(lang wording.Lang) []healthRow {
	out := slices.Concat(
		v.sourceRows(lang),
		v.citationRows(lang),
		v.schemaRows(),
		v.statusRows(lang),
		v.collisionRows(lang),
	)
	// The weight a kind carries is the same on every row of it, so it is put
	// on once here rather than repeated at each place a row is made — where
	// one of them would eventually be the one that forgot.
	for i := range out {
		if rule, ok := healthRules[out[i].Kind]; ok {
			out[i].Severity, out[i].Weighed = rule.severity, true
		}
	}
	return out
}

// sourceRows are the paths no note was read out of: one the reading could not
// open, and one it saw and left out. Each is a row of its own, because nothing
// was read out of it and the read's own account is all there is to say.
func (v *HealthView) sourceRows(lang wording.Lang) []healthRow {
	out := make([]healthRow, 0, len(v.Blocked)+len(v.Skipped))
	for _, source := range v.Blocked {
		out = append(out, healthRow{Kind: healthBlocked, FilePath: source.Path, Detail: machineDetail(source.Reason), Count: 1})
	}
	for _, source := range v.Skipped {
		var detail []healthDetail
		if source.Size > 0 {
			detail = append(detail, healthDetail{Text: humanSize(source.Size, lang)})
		}
		detail = append(detail, machineDetail(source.Reason)...)
		out = append(out, healthRow{Kind: healthSkipped, FilePath: source.Path, Detail: detail, Count: 1})
	}
	return out
}

// citationRows are what the links say: targets nothing answers to, targets
// naming a title rather than a name the vault resolves by, and notes no text
// cites. The first two fold every citation of one note into that note's row —
// the repair is opening that note once — while an uncited note is one row,
// carrying the folder, because the shape of a folder full of them is what a
// reader judges the group by and the file column names the note alone.
func (v *HealthView) citationRows(lang wording.Lang) []healthRow {
	var out []healthRow
	unwrittenAt := make(map[string]int, len(v.Unwritten))
	for _, link := range v.Unwritten {
		detail := healthDetail{Text: fmt.Sprintf(wording.LinkedToFmt.In(lang), link.Target)}
		if i, ok := unwrittenAt[link.From.RelPath]; ok {
			out[i].Detail = append(out[i].Detail, detail)
			out[i].Count++
			continue
		}
		unwrittenAt[link.From.RelPath] = len(out)
		out = append(out, healthRow{Kind: healthUnwritten, File: link.From, Detail: []healthDetail{detail}, Count: 1})
	}
	titleOnlyAt := make(map[string]int, len(v.TitleOnly))
	for _, link := range v.TitleOnly {
		detail := healthDetail{Text: fmt.Sprintf(wording.TitleOnlyMeansTo.In(lang), link.Target), Link: link.Note}
		if i, ok := titleOnlyAt[link.From.RelPath]; ok {
			out[i].Detail = append(out[i].Detail, detail)
			out[i].Count++
			continue
		}
		titleOnlyAt[link.From.RelPath] = len(out)
		out = append(out, healthRow{Kind: healthTitleOnly, File: link.From, Detail: []healthDetail{detail}, Count: 1})
	}
	for _, group := range v.Islands {
		for _, ref := range group.Notes {
			out = append(out, healthRow{Kind: healthIsland, File: ref, Detail: []healthDetail{{Text: group.Name}}, Count: 1})
		}
	}
	return out
}

// schemaRows are the notes the schema had something to say about. What it said
// stays on each note's own page — two accounts of one file in two places is how
// the two start disagreeing — so the row carries how many things were said and
// how heavy the heaviest was, and the reader opens the note to read them.
func (v *HealthView) schemaRows() []healthRow {
	out := make([]healthRow, 0, len(v.FrontmatterUnreadable)+len(v.SchemaFaults))
	for _, found := range v.FrontmatterUnreadable {
		out = append(out, healthRow{Kind: healthUnreadableFrontmatter, File: found.Note, Severity: found.Severity, Weighed: true, Count: found.Count})
	}
	for _, found := range v.SchemaFaults {
		out = append(out, healthRow{Kind: healthSchemaFault, File: found.Note, Severity: found.Severity, Weighed: true, Count: found.Count})
	}
	return out
}

// statusRows are the notes carrying a status their type cannot be in. The value
// and the type travel together: the value is the word the reader edits and the
// type is why it failed.
func (v *HealthView) statusRows(lang wording.Lang) []healthRow {
	out := make([]healthRow, 0, len(v.StatusOutsideEnum)+len(v.StatusUnreachable))
	for _, kind := range []struct {
		kind  healthKind
		found []HealthStatusNote
	}{
		{healthStatusOutsideEnum, v.StatusOutsideEnum},
		{healthStatusUnreachable, v.StatusUnreachable},
	} {
		for _, found := range kind.found {
			detail := healthDetail{Text: fmt.Sprintf(wording.StatusAndTypeFmt.In(lang), found.Status, found.Type)}
			out = append(out, healthRow{Kind: kind.kind, File: found.Note, Detail: []healthDetail{detail}, Count: 1})
		}
	}
	return out
}

func (v *HealthView) collisionRows(lang wording.Lang) []healthRow {
	out := make([]healthRow, 0, len(v.Collisions))
	for _, collision := range v.Collisions {
		out = append(out, healthCollisionRow(collision, lang))
	}
	return out
}

// healthCollisionRow is one shared name as a row. The file column names the
// first claimant, which is the file the judging face reports the collision
// against, and the rest of the claimants follow the name in the evidence — the
// name alone is no file, and the column is files.
func healthCollisionRow(collision HealthCollision, lang wording.Lang) healthRow {
	row := healthRow{Kind: healthCollision, Count: 1}
	shared := fmt.Sprintf(wording.CollisionSharedWith.In(lang), collision.Name)
	if len(collision.Candidates) == 0 {
		row.FilePath = collision.Name
		row.Detail = []healthDetail{{Text: shared}}
		return row
	}
	row.File = collision.Candidates[0]
	// The sentence introducing the others travels with the first of them, so
	// the separators between claimants fall between claimants.
	for i, candidate := range collision.Candidates[1:] {
		detail := healthDetail{Link: candidate}
		if i == 0 {
			detail.Text = shared
		}
		row.Detail = append(row.Detail, detail)
	}
	if len(row.Detail) == 0 {
		row.Detail = []healthDetail{{Text: shared}}
	}
	return row
}

// machineDetail carries a machine's own words into the evidence, and nothing
// where there were none.
func machineDetail(text string) []healthDetail {
	if text == "" {
		return nil
	}
	return []healthDetail{{Text: text, Machine: true}}
}

// healthTallies is what the guide explains: every kind of finding present,
// in the page's own order, with how many of it the table holds. The numbers
// are counted off the rows rather than off the lists behind them, so the
// heading a reader checks against and the lines they count cannot disagree.
func healthTallies(rows []healthRow) []healthTally {
	total := make(map[healthKind]int, len(healthKinds))
	for _, row := range rows {
		total[row.Kind] += row.Count
	}
	out := make([]healthTally, 0, len(healthKinds))
	for _, kind := range healthKinds {
		if count := total[kind]; count > 0 {
			out = append(out, healthTally{Kind: kind, Count: count})
		}
	}
	return out
}

// healthWeightTally is one weight the judge names, and how many findings in
// the table carry it — the same number a reader summing the count column by
// hand over every row of that weight would reach.
type healthWeightTally struct {
	Severity judge.Severity
	Count    int
}

// healthShape is the report's own shape, stated before a reader scrolls the
// table it is counted from: the findings gathered by the weight the judge
// gives them, heaviest first, and how many distinct files any row of the
// table names. A kind no rule weighs contributes no entry to Weights — it has
// no word in the judge's vocabulary to be counted under — and its file still
// counts toward Files, because the table still lists it. Total includes every
// finding, whether or not a rule weighs it.
type healthShape struct {
	Total   int
	Weights []healthWeightTally
	Files   int
}

// healthShapeOf tallies the table's own rows rather than the lists behind
// them, so this line and what a reader counts down the table can never
// disagree.
func healthShapeOf(rows []healthRow) healthShape {
	var total int
	var byWeight [judge.SeverityError + 1]int
	files := make(map[healthFileKey]struct{}, len(rows))
	for _, row := range rows {
		total += row.Count
		if row.Weighed {
			byWeight[row.Severity] += row.Count
		}
		files[row.fileKey()] = struct{}{}
	}
	weights := make([]healthWeightTally, 0, len(byWeight))
	for s := judge.SeverityError; s >= judge.SeverityInfo; s-- {
		if count := byWeight[s]; count > 0 {
			weights = append(weights, healthWeightTally{Severity: s, Count: count})
		}
	}
	return healthShape{Total: total, Weights: weights, Files: len(files)}
}

// kindLede is what the guide says a kind of finding means. All but one are a
// fixed sentence; the unreadable files also say how old the rest of the page
// is, which only this view knows.
func (v *HealthView) kindLede(kind healthKind, lang wording.Lang) string {
	switch kind {
	case healthBlocked:
		return v.blockedLede(lang)
	case healthSkipped:
		return wording.SkippedLede.In(lang)
	case healthUnwritten:
		return wording.UnwrittenLede.In(lang)
	case healthTitleOnly:
		return wording.TitleOnlyLede.In(lang)
	case healthIsland:
		return wording.IslandsLede.In(lang)
	case healthUnreadableFrontmatter:
		return wording.HealthFrontmatterLede.In(lang)
	case healthSchemaFault:
		return wording.HealthSchemaLede.In(lang)
	case healthStatusOutsideEnum:
		return wording.StatusOutsideEnumLede.In(lang)
	case healthStatusUnreachable:
		return wording.StatusUnreachableLede.In(lang)
	case healthCollision:
		return wording.CollisionsLede.In(lang)
	}
	return ""
}

// clean reports whether the folder has nothing to answer for.
func (v *HealthView) clean() bool {
	return len(v.Unwritten) == 0 && len(v.TitleOnly) == 0 && v.IslandCount == 0 &&
		len(v.Collisions) == 0 && len(v.Blocked) == 0 && len(v.Skipped) == 0 &&
		len(v.StatusOutsideEnum) == 0 &&
		len(v.StatusUnreachable) == 0 &&
		len(v.FrontmatterUnreadable) == 0 && len(v.SchemaFaults) == 0 &&
		v.InstanceScopeUnknown == "" && v.SchemaScopeUnknown == ""
}

// blockedLede states what the blocked list means for the reader, and how
// current the page behind it is.
func (v *HealthView) blockedLede(lang wording.Lang) string {
	lede := wording.BlockedLede.In(lang)
	if v.LastComplete == "" {
		return lede + wording.BlockedNeverComplete.In(lang)
	}
	return lede + fmt.Sprintf(wording.BlockedLastCompleteFmt.In(lang), v.LastComplete)
}
