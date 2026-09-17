package note

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// home renders the desk: four ways into the library, built from one coherent
// snapshot, with whatever is wrong with the vault stated below the seam. It is
// a read face: no status forms or write capability enter the view.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	// The desk links to the folder's own introduction rather than reprinting
	// it, so nothing here renders it and its absence is not news.
	_, hasReadme := snap.Entry(pages.HomeReadmeRelPath)
	// One reading of the folder's state, used by both halves of the notice
	// below. Read live, it can change between two questions, and the count in
	// the sentence would then be answered by a different set of files than the
	// technical detail beside it lists.
	fresh := snap.Freshness()
	visible := shell.Project(authority, snap)
	visibleNav := visible.Nav
	blocks := pages.NewDeskBlocks(visibleNav, snap.NavigationRoles(), pages.ContractStateFrom(visible.Governed, snap), lang, pages.ArticleLanguageFromSnapshot(snap))
	// The reason a way in is empty, stated once at the foot of the page. The
	// desk draws all four ways in, so what it says here is the union of what
	// the four pages behind them say: the write authority, which empties the
	// distribution the folder mode carries; the navigation declaration, which
	// empties the courses and the maps; and the artifact policy, which does
	// both and also takes the knowledge layer off the recent list. The reports
	// are a listing of a directory that no declaration can close, so they add
	// nothing here. A mode added later without its reason reaching this line
	// would empty a block on this page and never say why.
	//
	// The rail no longer carries this on every page, because the rail is now
	// the book being read, which makes this the only place it is said.
	fault := statedOnce(
		authority.Diagnostic(lang),
		visibleNav.NavigationClosure().Diagnostic(),
		visibleNav.ArtifactClosure().Diagnostic(),
	)
	kept, hasMark := h.sources.Continuation()
	view := pages.HomeView{
		Fault:          fault,
		PrivacyFault:   snap.PrivacyPolicy().Diagnostic(),
		Degraded:       degradedNotice(&fresh, lang),
		DegradedDetail: blockedDetail(fresh.Blocked),
		Blocks:         blocks,
		ReadmeMissing:  !hasReadme,
		Continue:       continueRow(&kept, hasMark, snap, lang),
	}
	if err := pages.Home(view, layouts.ChromeFromRequest(r, wording.HomeTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write home page", "error", err)
	}
}

// continueRow is the desk's way back to the place the reader kept.
//
// The note it names is resolved inside snap — the same generation the rest of
// the page was built from — so the row's sentence about the note having
// changed is decided against the version this desk is describing, rather than
// against whatever the folder holds a moment later. An identity that no longer
// matches still goes to the mark: it is the reader's own best pointer, and the
// sentence says why the place may have moved rather than withholding it.
func continueRow(kept *mark.Continuation, hasMark bool, snap *snapshot.Generation, lang wording.Lang) pages.ContinueRow {
	if !hasMark {
		return pages.ContinueRow{}
	}
	reading, found := snap.Note(kept.RelPath)
	if !found {
		// Nothing is cleared. The reader did not ask for that, and a note can
		// be missing from one generation because the folder was mid-write.
		return pages.ContinueRow{Show: true, Title: kept.RelPath, Notice: wording.MarkNoteGone.In(lang)}
	}
	row := pages.ContinueRow{
		Show:  true,
		Title: cmp.Or(reading.Title, kept.RelPath),
		Href:  continueHref(kept),
	}
	if hex.EncodeToString(reading.ContentIdentity[:]) != kept.Identity {
		row.Notice = wording.MarkNoteChanged.In(lang)
	}
	return row
}

// continueHref is where that row leads: the note, the anchor the mark named,
// and how far below it the reader was.
//
// The anchor is the fragment, so a browser running nothing lands on the
// heading or block the reader stopped under — which is the whole of the
// promise a mark can keep without a script. The distance rides as a query the
// reading page's own module spends and then removes from the address; a
// fragment carrying it would name no element and drop the reader at the top.
func continueHref(kept *mark.Continuation) string {
	address := pages.VaultHref("/notes/", kept.RelPath)
	if kept.Offset > 0 {
		address += "?" + url.Values{continueOffsetParam: {strconv.Itoa(kept.Offset)}}.Encode()
	}
	if kept.Anchor != "" {
		address += "#" + url.PathEscape(kept.Anchor)
	}
	return address
}

// continueOffsetParam carries the distance below the anchor. The page that
// reads it back is drawn by this package, so the name is written once here and
// stamped into the address the reading page receives.
const continueOffsetParam = "at"

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
// read closes the distribution and the study paths alike — and printing its
// sentence once per empty block is what buried the reader's own content. A
// rejected navigation declaration closes the courses and the maps, and its
// sentence is a different one, so a page that carries only the write
// authority's reason empties a block without ever saying why.
//
// Which causes a page passes here is settled by one rule: a page states the
// reasons that could empty or degrade something it actually draws, and no
// others. A reason for a projection the page never shows is a fault the reader
// is asked to hold against a page that has nothing to do with it. The desk is
// the one page that states several, because it draws all four ways in at once,
// and what it states is the union of what those four pages state.
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
