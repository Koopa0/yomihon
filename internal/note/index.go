package note

import (
	"net/http"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

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

// folders is the page of every folder, two levels deep. A declared knowledge
// layer narrows the top-level folders; without a usable declaration the full
// tree shows, because mending a broken contract is done while reading the vault
// it governs. Root files and direct folder reading stay available in every
// contract state.
func (h *Handler) folders(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Authority()
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	model := pageShell.Nav

	query := pages.ParseListQuery(r.URL.Query())
	view := pages.NewFolderIndexView(model, pages.ContractStateFrom(pageShell.Governed, snap), query, time.Now(), lang)
	// Two of the three causes the desk states can degrade something drawn
	// here: the contract and the artifact policy. The navigation declaration is
	// not one of them — it decides which notes are courses and maps, and this
	// page draws neither — so stating it here would hand the reader a fault
	// about another page.
	view.Fault = statedOnce(
		authority.Diagnostic(),
		model.ArtifactClosure().Diagnostic(),
	)
	chrome := layouts.ChromeFromRequest(r, view.Title)
	if err := pages.FolderIndex(view, chrome).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write folder index", "error", err)
	}
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
