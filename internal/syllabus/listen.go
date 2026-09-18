package syllabus

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// listen renders one course as the paragraphs its lessons are marked to be
// read aloud in, lesson by lesson, in course order. It refuses an unknown
// course exactly as the course page does, because it is the same name.
func (h *Handler) listen(w http.ResponseWriter, r *http.Request) {
	rel := vault.NormalizeNFC(r.PathValue("path"))
	lang := origin.Language(r)

	snap := h.current()
	current := snap.Shell.Nav.Path(rel)
	if current == nil {
		view := pages.NotFoundView{Asked: r.URL.Path, Sidebar: pages.NewSidebar(snap.Shell, "")}
		chrome := layouts.ChromeFromRequest(r, wording.PathNotFound.In(lang))
		if err := pages.WriteNotFound(r.Context(), w, view, chrome); err != nil {
			h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write listening not-found page", "path", rel, "error", err)
		}
		return
	}

	view := listenView(current, &snap, lang)
	if err := pages.Listen(view, layouts.ChromeFromRequest(r, view.Title)).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write listening page", "path", rel, "error", err)
	}
}

// listenView reads every lesson the course teaches and keeps the paragraphs
// its author marked. The snapshot holds a note's markdown and no HTML, and the
// marker survives only into rendered HTML, so each lesson is rendered here and
// most of what comes back is dropped.
func listenView(current *nav.Path, snap *RequestSnapshot, lang wording.Lang) pages.ListenView {
	view := pages.ListenView{
		Title:    fmt.Sprintf(wording.ListenTitleFmt.In(lang), current.Title),
		PathHref: pages.VaultHref("/syllabus/", current.RelPath),
	}
	for _, entry := range taught(current) {
		// A lesson the course plans but nobody has written carries no vault
		// path, so the generation answers for nothing and the row falls out
		// here. Asking a second predicate whether the row was openable would be
		// asking the same question twice and inviting the two to disagree.
		note, ok := snap.Generation.Note(entry.RelPath)
		if !ok || !snap.isLesson(note.Type) {
			continue
		}
		// One region per lesson, taken from where the lesson stands in the
		// course, so two readers of one course are served the same bytes. The
		// lessons share a page, and footnote ids minted without it would
		// collide between them.
		region := "l" + strconv.Itoa(len(view.Lessons)+1) + "-"
		marked := render.MarkedParagraphs(snap.Generation.RenderIn(region, entry.RelPath, note.Body, lang).HTML, lang)
		if len(marked) == 0 {
			continue
		}
		title := note.Title
		if title == "" {
			title = entry.Text
		}
		view.Lessons = append(view.Lessons, pages.ListenLesson{
			Title:      title,
			Href:       pages.VaultHref("/notes/", entry.RelPath),
			Paragraphs: marked,
		})
	}
	return view
}

// taught walks the course in the order it is read and returns the rows it
// teaches: a branch the grammar projects and a row it accepted, which is the
// same question the course page draws with. Neither half is re-derived here, so
// a branch drawn but not sequenced, and a heading that declared nothing, teach
// nothing on this page either.
func taught(current *nav.Path) []*nav.PathEntry {
	var found []*nav.PathEntry
	var walk func(group *nav.PathGroup)
	walk = func(group *nav.PathGroup) {
		for _, item := range group.Items {
			switch {
			case item.Entry != nil:
				if group.Teaches(item.Entry) {
					found = append(found, item.Entry)
				}
			case item.Group != nil:
				walk(item.Group)
			}
		}
	}
	for _, group := range current.Groups {
		walk(group)
	}
	return found
}

// isLesson asks the contract whether a note's declared type is the one this
// vault files course members as. A face reading a generation that carries no
// status authority teaches nothing rather than guessing at the name.
func (s *RequestSnapshot) isLesson(noteType string) bool {
	return s.Status != nil && s.Status.IsLessonType(noteType)
}
