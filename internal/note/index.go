package note

import (
	"cmp"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// shelfRecentLimit keeps the recent list a trailhead rather than another browse
// surface. The whole vault is one folder away, and reachable by search.
const shelfRecentLimit = 7

// maps lists every map the vault declares. Like the study-path index it refuses
// what the rail refused: a withheld declaration lists nothing and states its
// reason, so the page cannot become a second route to a projection the contract
// closed.
func (h *Handler) maps(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	model := shell.Project(h.sources.Status(), h.sources.Snapshot().Capture()).Nav
	closure := model.DeclaredClosure()
	var declared []nav.Map
	if !closure.Closed() {
		declared = model.Maps()
	}
	view := pages.NewMapIndex(declared, lang)
	view.Fault = closure.Diagnostic()
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, view.Shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write map index", "error", err)
	}
}

// folders lists the vault's own directory tree, and beneath it the two things a
// reader asks about how the shelf is kept: what changed last, and where every
// indexed note sits.
//
// The tree itself is plain reading — file names and where they are — so no
// declaration gates it: a vault whose contract broke must not show less of its
// own folders than one that never carried a contract, because mending the
// contract is done while reading the vault it governs.
func (h *Handler) folders(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(authority, snap)
	model := pageShell.Nav
	lifecycle, unstated, lifecycleClosed := h.lifecycle(authority, snap, lang)
	// The distribution is derived from the write authority while the counts
	// under it come from the snapshot's own artifact sample, and the two are
	// taken at different instants. This does not fire today: the request-local
	// view binds its policy and its search index to one authority, so the count
	// refuses first and closes the block on the way out. It stands as the guard
	// for the day that binding is loosened, and the binding itself is what the
	// snapshot package pins.
	if !lifecycleClosed && !snap.ArtifactPolicy().Trustworthy() {
		lifecycle = nil
		lifecycleClosed = true
	}
	// The recent list is plain reading — scanner-captured names and times — so
	// no closure gates it: the navigation model builds it in every contract
	// state, degraded to the all-inclusive, layer-citation-free answer when a
	// declaration could not be honoured.
	recent, recentOrdered := recentShelfNotes(model.KnowledgeNotes(), pageShell.Governed, authority)

	view := pages.NewFolderIndex(model, lang)
	view.Fault = statedOnce(
		authority.Diagnostic(lang),
		model.NavigationClosure().Diagnostic(),
		model.ArtifactClosure().Diagnostic(),
	)
	view.Recent = recent
	view.RecentOrdered = recentOrdered
	view.RecentScoped = model.KnowledgeScoped()
	view.Lifecycle = lifecycle
	view.Unstated = unstated
	view.ShowRecent = len(recent) > 0
	view.ShowLifecycle = pageShell.Governed && !lifecycleClosed && len(lifecycle) > 0
	if err := pages.FolderIndex(view, layouts.ChromeFromRequest(r, view.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write folder index", "error", err)
	}
}

// lifecycle assembles the distribution block: the vault's status distribution,
// one entry per status at least one note currently carries, in the contract's
// toml order. A closed result means the block was withheld — the vault declared
// a vocabulary yomihon could not read — never that the vault has no statuses. A
// vault whose notes carry none yields an open, empty block.
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

// recentShelfNotes selects the newest knowledge notes from the snapshot's
// scanner-captured timestamps, and reports whether their recorded times
// actually order them. It sorts a clone, leaving the published model immutable
// for concurrent readers. Equal mtimes fall back to path order so a rebuild
// produces stable output.
//
// A fresh clone stamps every file with the checkout moment, so the tie-break
// starts deciding and a heading that says "recently changed" leads with
// whatever name sorts first. The reader has no way to see that from the page,
// which is the kind of quiet wrong answer this interface is not allowed to
// give.
func recentShelfNotes(
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
	if len(notes) > shelfRecentLimit {
		notes = notes[:shelfRecentLimit]
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
