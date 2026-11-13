package note

import (
	"net/http"
	"slices"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
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
	authority := h.sources.Authority()
	// The names and counts below and the language each row is declared in come
	// from this one reading of the published pointer. Taken twice, a rebuild
	// between them puts one note's title beside another version's language, and
	// the row describes something the vault never held.
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	model := pageShell.Nav
	view := pages.NewMapIndex(model.Maps(), snap.NavigationRoles(), model.DeclaredClosure(), pages.ContractStateFrom(pageShell.Governed, snap), lang, pages.ArticleLanguageFromSnapshot(snap))
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, view.Shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write map index", "error", err)
	}
}

// folders lists the vault's folder shelf, and beneath it the two things a
// reader asks about how the shelf is kept: what changed last, and where every
// indexed note sits.
//
// A declared knowledge layer narrows the shelf's top-level folders. Without a
// usable declaration the shelf shows the full tree, because mending a broken
// contract is done while reading the vault it governs. Root files and direct
// folder reading remain available in every contract state.
func (h *Handler) folders(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Authority()
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	model := pageShell.Nav
	// The recent list is plain reading — scanner-captured names and times — so
	// no closure gates it: the navigation model builds it in every contract
	// state, degraded to the all-inclusive, layer-citation-free answer when a
	// declaration could not be honoured.
	articleLang := articleLanguageLookup(snap)
	recent, recentOrdered := recentShelfNotes(model.KnowledgeNotes(), articleLang)

	view := pages.NewFolderIndex(model, pages.ContractStateFrom(pageShell.Governed, snap), lang, articleLang)
	// Two of the three causes the desk states can degrade something drawn
	// here: the contract and the artifact policy, which takes the knowledge
	// layer off the recent list.
	// The navigation declaration is not one of them — the roles it carries
	// decide which notes are courses and maps, and this page draws neither — so
	// stating it here would hand the reader a fault about another page.
	view.Fault = statedOnce(
		authority.Diagnostic(),
		model.ArtifactClosure().Diagnostic(),
	)
	recentBlock := pages.NewRecentBlock(recent, recentOrdered, model.KnowledgeScoped(), lang)
	// The recent list is never withheld, only reworded — it is plain reading,
	// and the block it is built into degrades its own sentence rather than
	// disappearing.
	chrome := layouts.ChromeFromRequest(r, view.Shelf.Title)
	if err := pages.FolderIndex(view, recentBlock, chrome).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write folder index", "error", err)
	}
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
	articleLang pages.ArticleLanguageFor,
) (recent []pages.HomeNote, ordered bool) {
	notes = slices.Clone(notes)
	slices.SortStableFunc(notes, func(a, b nav.NoteSummary) int {
		switch {
		case a.Modified.Equal(b.Modified):
			return vault.ComparePaths(a.RelPath, b.RelPath)
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
		item := pages.HomeNote{Title: n.Title, RelPath: n.RelPath, Type: n.Type, Draft: n.Draft}
		if articleLang != nil {
			item.Language = articleLang(n.RelPath)
		}
		if !n.Modified.IsZero() {
			item.Modified = n.Modified.Format("2006-01-02")
			item.ModifiedAt = n.Modified.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out, ordered
}

func articleLanguageLookup(snap *snapshot.Generation) pages.ArticleLanguageFor {
	return func(relPath string) string {
		if snap == nil {
			return ""
		}
		note, ok := snap.Note(relPath)
		if !ok {
			return ""
		}
		return note.Language
	}
}
