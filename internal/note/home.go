package note

import (
	"cmp"
	"fmt"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

// homeRecentLimit keeps the landing page a trailhead rather than another
// browse surface. The complete vault remains reachable through navigation and
// search.
const homeRecentLimit = 7

// homeReadmePath is the note Home shows as its own introduction. It is named
// once because the lookup and the render need the same answer: the renderer
// resolves a relative image against the note's directory, so a body fetched
// under one path and rendered under another would address the wrong files.
const homeReadmePath = "README.md"

// home renders the four landing blocks from one coherent snapshot, followed by
// the vault README through the same markdown pipeline used by a note page. It
// is a read face: no status forms or write capability enter the view.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	lang := wording.LanguageFromRequest(r)
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	// Home links to the folder's own introduction rather than reprinting it,
	// so nothing here renders it and its absence is not news.
	_, hasReadme := snap.Note(homeReadmePath)
	// One reading of the folder's state, used by both halves of the notice
	// below. Read live, it can change between two questions, and the count in
	// the sentence would then be answered by a different set of paths than the
	// technical detail beside it lists.
	fresh := snap.Freshness()
	artifactPolicy := snap.ArtifactPolicy()
	pageShell := shell.Project(authority, snap)
	lifecycle, unstated, lifecycleClosed := h.lifecycle(authority, snap, lang)
	// The lifecycle block is derived from the write authority while the counts
	// under it come from the snapshot's own artifact sample, and the two are
	// taken at different instants. This does not fire today: the request-local
	// view binds its policy and its search index to one authority, so the count
	// refuses first and closes the block on the way out. It stands as the guard
	// for the day that binding is loosened, and the binding itself is what the
	// snapshot package pins.
	if !lifecycleClosed && !artifactPolicy.Trustworthy() {
		lifecycle = nil
		lifecycleClosed = true
	}
	visibleNav := pageShell.Nav
	// The recent list is plain reading — scanner-captured names and times — so
	// no closure gates it: the navigation model builds it in every contract
	// state, degraded to the all-inclusive, layer-citation-free answer when a
	// declaration could not be honoured. A vault whose contract broke must not
	// show less than one that never carried a contract, because mending the
	// toml is done while reading the vault it governs.
	recent, recentOrdered := recentHomeNotes(visibleNav.KnowledgeNotes(), pageShell.Governed, authority)
	pathsClosed := visibleNav.NavigationClosure().Closed() || visibleNav.ArtifactClosure().Closed()
	var paths []pages.HomePath
	if !pathsClosed {
		paths = homePaths(visibleNav.Paths())
	}
	content := homeContent{
		recent:    len(recent) > 0,
		lifecycle: pageShell.Governed && !lifecycleClosed && len(lifecycle) > 0,
		paths:     !pathsClosed && len(paths) > 0,
		withheld:  lifecycleClosed || pathsClosed,
	}
	view := pages.HomeView{
		Governed: pageShell.Governed,
		Subtitle: content.subtitle(lang),
		StandIn:  homeStandIn(snap, content),
		// The reason a block is missing, stated where the reader is looking. One
		// cause reaches several blocks — a contract that cannot be read closes
		// the lifecycle and the study paths alike — and
		// repeating its sentence per block is what buried the reader's own
		// content under a column of apologies. Each closed block renders
		// nothing; the cause is stated once, here.
		//
		// The navigation rail states it too, on every page rather than only this
		// one. That is not a duplicate to remove: the rail collapses behind a
		// toggle at narrow widths, and a fault only the wide layout can show is
		// a fault the reader does not get — which is also why every cause that
		// closed a block here has to reach this column, not only the one the
		// write authority happens to know about.
		Fault: statedOnce(
			authority.Diagnostic(),
			visibleNav.NavigationClosure().Diagnostic(),
			visibleNav.ArtifactClosure().Diagnostic(),
		),
		PrivacyFault:   snap.PrivacyPolicy().Diagnostic(),
		Degraded:       degradedNotice(&fresh, lang),
		DegradedDetail: blockedDetail(fresh.Blocked),
		Recent:         recent,
		RecentOrdered:  recentOrdered,
		RecentScoped:   visibleNav.KnowledgeScoped(),
		Lifecycle:      lifecycle,
		Unstated:       unstated,
		Paths:          paths,
		ShowRecent:     content.recent,
		ShowLifecycle:  content.lifecycle,
		ShowPaths:      content.paths,
		ReadmeMissing:  !hasReadme,
		Sidebar:        pages.NewSidebar(visibleNav, ""),
	}
	if err := pages.Home(view, layouts.ChromeFromRequest(r, wording.HomeTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.sources.Log.Error("write home page", "error", err)
	}
}

// lifecycle assembles Home's Lifecycle block: the vault's status
// distribution, one entry per status at least one note currently carries, in
// the contract's toml order. A closed result means the block was withheld —
// the vault declared a vocabulary yomihon could not read — never that the
// vault has no statuses. A vault whose notes carry none yields an open,
// empty block.
func (h *Handler) lifecycle(
	authority status.Authority,
	snap *snapshot.Generation,
	lang wording.Lang,
) (items, unstated []pages.LifecycleItem, closed bool) {
	if !authority.Governed() {
		return nil, nil, false
	}
	if authority.Closed() {
		return nil, nil, true
	}
	counts, err := snap.Search().CountByTypeStatus()
	if err != nil {
		return nil, nil, true
	}
	// The block states what the notes carry and claims nothing more: owner
	// lists play no part, and a terminal status with notes at it is as much a
	// fact of the vault as any other — a distribution that hid a bucket would
	// disagree with its own total. Every indexed note is accounted for,
	// including the ones holding no status at all: they leave this loop and
	// arrive in the two cells below, which are kept apart from the statuses
	// because neither of them is one.
	byStatus := make(map[string]int, len(counts))
	// declared records whether some type carrying the status declares it. A
	// status no carrier declares is outside every relevant enum, and its chip
	// says so with the note page's own flag instead of passing as vocabulary.
	declared := make(map[string]bool, len(counts))
	// Notes with no readable status used to leave the block entirely, which is
	// how a distribution came to disagree with the number of notes it claimed
	// to be a distribution of. They are counted here and given their own two
	// cells below.
	withoutStatus := 0
	for ts, n := range counts {
		if ts.Status == "" {
			withoutStatus += n
			continue
		}
		byStatus[ts.Status] += n
		if authority.KnownStatus(ts.Type, ts.Status) {
			declared[ts.Status] = true
		}
	}
	items = make([]pages.LifecycleItem, 0, len(byStatus))
	add := func(s string) {
		items = append(items, pages.LifecycleItem{
			Name:    s,
			Count:   byStatus[s],
			Sealed:  s == schema.SealStatus,
			Unknown: !declared[s],
		})
	}
	for _, s := range authority.Order() {
		if byStatus[s] == 0 {
			continue
		}
		add(s)
		delete(byStatus, s)
	}
	// Whatever is still here sits at a value the default vocabulary does not
	// list: the contract routes that kind of note to another group of statuses,
	// or the note carries a value no group declares at all. The total above
	// counted those notes, so leaving them out is exactly how a number stops
	// agreeing with its own breakdown. Nothing declares an order across groups,
	// so they follow in a stable one.
	for _, s := range slices.Sorted(maps.Keys(byStatus)) {
		add(s)
	}
	// The notes with no readable status divide in two, and the division
	// matters to a reader: one kind cannot be judged at all until its
	// frontmatter is repaired, the other declared no status and may be
	// perfectly entitled not to. Both counts come from the index that produced
	// the tally above, so the cells add up to the notes it counted rather than
	// to a second reckoning of the folder.
	unreadable, err := snap.Search().CountUnreadableFrontmatter()
	if err != nil {
		return items, nil, false
	}
	if unreadable > 0 {
		unstated = append(unstated, pages.LifecycleItem{
			Label: wording.LifecycleUnreadable.In(lang),
			Count: unreadable,
			Href:  "/health",
		})
	}
	// The rest declared no status at all. The cell takes its words from where a
	// blank status has always taken them, which is a sentence this product
	// already wrote for exactly this square and then stopped reaching.
	if declaredNone := withoutStatus - unreadable; declaredNone > 0 {
		unstated = append(unstated, pages.LifecycleItem{Count: declaredNone})
	}
	return items, unstated, false
}

// homeContent records which of Home's content blocks this folder actually
// fills, and whether any of them was withheld rather than empty. Both the line
// under the title and the line that stands in for the blocks are derived from
// it: a bordered box with one sentence of apology in it costs a quarter of the
// first screen and gives back nothing, and on a folder that declares no
// contract there will never be a typed note or a study path to put in it.
type homeContent struct {
	recent    bool
	lifecycle bool
	paths     bool
	withheld  bool
}

// subtitle names the blocks that are on the page, and nothing else. The three
// together produce the sentence this page has always carried; fewer of them
// produce a shorter true one, and none produces silence.
func (c homeContent) subtitle(lang wording.Lang) string {
	parts := make([]string, 0, 3)
	if c.recent {
		parts = append(parts, wording.HomeSubtitleRecent.In(lang))
	}
	if c.lifecycle {
		parts = append(parts, wording.HomeSubtitleLifecycle.In(lang))
	}
	if c.paths {
		parts = append(parts, wording.HomeSubtitlePaths.In(lang))
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf(wording.HomeSubtitleOneFmt.In(lang), parts[0])
	case 2:
		return fmt.Sprintf(wording.HomeSubtitleTwoFmt.In(lang), parts[0], parts[1])
	default:
		return fmt.Sprintf(wording.HomeSubtitleThreeFmt.In(lang), parts[0], parts[1], parts[2])
	}
}

// homeStandIn builds the line that opens Home when none of its content blocks
// has anything to show. It answers the two questions someone opening a folder
// actually has — how much is in here, and what changed last — and links the
// newest file, which is the shortest path to the thing most likely wanted. It
// never names what the folder did not declare: a reader who will not write a
// contract is not missing a feature.
//
// A withheld block counts as content. Its absence already has a reason stated
// once for the whole page, and a cheerful fact beside that reason would be a
// second, contradictory account of the same hole.
func homeStandIn(snap *snapshot.Generation, content homeContent) pages.HomeStandIn {
	if content.recent || content.lifecycle || content.paths || content.withheld {
		return pages.HomeStandIn{}
	}
	files := snap.Files()
	standIn := pages.HomeStandIn{Shown: true, Files: len(files)}
	var newest vaultfs.Entry
	for _, entry := range files {
		if newest.Path() == "" || entry.ModTime().After(newest.ModTime()) {
			newest = entry
		}
	}
	if newest.Path() == "" {
		return standIn
	}
	standIn.NewestRelPath = newest.Path()
	standIn.NewestName = path.Base(newest.Path())
	standIn.NewestDate = newest.ModTime().Format("2006-01-02")
	standIn.NewestAt = newest.ModTime().Format(time.RFC3339)
	return standIn
}

// degradedNotice states, in the reader's language, that the snapshot behind
// the page could not read everything, so the content may be incomplete or held
// at an older generation. Empty when the snapshot is whole and current, which
// is the ordinary case and renders nothing.
func degradedNotice(fresh *snapshot.Freshness, lang wording.Lang) string {
	n := len(fresh.Blocked)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return fmt.Sprintf(wording.DegradedNoticeOne.In(lang), n)
	}
	return fmt.Sprintf(wording.DegradedNoticeMany.In(lang), n)
}

// blockedDetail joins the blocked paths and their errors into the technical
// detail shown beside the notice, in the same shape other diagnostics use.
func blockedDetail(blocked []snapshot.BlockedSource) string {
	parts := make([]string, 0, len(blocked))
	for _, source := range blocked {
		if source.Reason == "" {
			parts = append(parts, source.Path)
			continue
		}
		parts = append(parts, source.Path+": "+source.Reason)
	}
	return strings.Join(parts, "; ")
}

// statedOnce joins the distinct reasons a page withheld something, in the order
// they were given, dropping the empty ones.
//
// A single cause usually closes several projections — a contract that cannot be
// read closes the lifecycle and the study paths alike — and
// printing its sentence once per closed block is what buried the reader's own
// content. A rejected navigation declaration closes only the study paths, and
// its sentence is a different one, so a page that carries only the write
// authority's reason drops a block without ever saying why.
func statedOnce(causes ...string) string {
	distinct := make([]string, 0, len(causes))
	for _, cause := range causes {
		if cause == "" || slices.Contains(distinct, cause) {
			continue
		}
		distinct = append(distinct, cause)
	}
	return strings.Join(distinct, "; ")
}

// recentHomeNotes selects the newest knowledge notes from the snapshot's
// scanner-captured timestamps. It sorts a clone, leaving the published model
// immutable for concurrent readers. Equal mtimes fall back to path order so a
// rebuild produces stable output.
// recentHomeNotes picks the notes the landing page leads with, and reports
// whether their recorded times actually order them. A fresh clone stamps every
// file with the checkout moment, so the block's tie-break — path order — starts
// deciding, and a heading that says "recently changed" leads with whatever name
// sorts first. The reader has no way to see that from the page, which is the
// kind of quiet wrong answer this interface is not allowed to give.
func recentHomeNotes(
	notes []nav.NoteSummary,
	governed bool,
	authority status.Authority,
) (recent []pages.HomeNote, ordered bool) {
	rules := !authority.Closed()
	notes = slices.Clone(notes)
	slices.SortStableFunc(notes, func(a, b nav.NoteSummary) int {
		switch {
		case a.Modified.Equal(b.Modified):
			return cmp.Compare(a.RelPath, b.RelPath)
		case a.Modified.After(b.Modified):
			return -1
		default:
			return 1
		}
	})
	if len(notes) > homeRecentLimit {
		notes = notes[:homeRecentLimit]
	}
	// One shared timestamp across everything shown means the times separated
	// nothing: what the reader is looking at is the tie-break, not recency.
	// A single note is ordered by itself — it is trivially the most recently
	// changed thing listed, and the tie notice would speak of files that are
	// not there.
	ordered = len(notes) < 2 || !notes[0].Modified.Equal(notes[len(notes)-1].Modified)

	out := make([]pages.HomeNote, 0, len(notes))
	for _, n := range notes {
		item := pages.HomeNote{Title: n.Title, RelPath: n.RelPath, Type: n.Type}
		// A status chip names a value from a declared vocabulary. Without a
		// contract there is no vocabulary, so raw frontmatter text is not
		// dressed up as a lifecycle state — and a view that holds no
		// vocabulary rules on nothing rather than calling every value a fault.
		if governed {
			item.Status = n.Status
			if rules && n.Status != "" {
				item.StatusOutsideEnum = !authority.KnownStatus(n.Type, n.Status)
			}
		}
		if !n.Modified.IsZero() {
			item.Modified = n.Modified.Format("2006-01-02")
			item.ModifiedAt = n.Modified.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out, ordered
}

// homePaths maps the snapshot's parsed study paths onto what Home says about
// them: how many lessons a course holds.
//
// It used to carry a second figure beside that, presented as how much of the
// course was finished. The figure counted lessons at the status the contract
// reserves for a human's final review, so it described a queue rather than any
// reading — and because publishing a lesson moves it out of that status, the
// number went down as the work was completed. A count that runs backwards
// cannot be repaired by renaming it.
func homePaths(paths []nav.Path) []pages.HomePath {
	out := make([]pages.HomePath, 0, len(paths))
	for i := range paths {
		studyPath := &paths[i]
		total := studyPath.Planned
		out = append(out, pages.HomePath{
			Title:   studyPath.Title,
			RelPath: studyPath.RelPath,
			Total:   total,
			// A zero with grammar diagnostics behind it is a fault to
			// repair; a zero without them is the author's answer. Only the
			// first is marked, so the two stop looking alike.
			Undetermined: total == 0 && len(studyPath.Diagnostics) > 0,
		})
	}
	return out
}
