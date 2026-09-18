package note

import (
	"net/http"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// health renders the whole-folder view of what needs attention. Every fact on
// it is already computed for the single-note pages; nobody opens every note, so
// gathering them is the only way they are ever seen.
//
// The gathering itself is shared, because the foot of every rail says how many
// of these there are and a second count worked out beside the rail would be the
// number that disagrees with this page. What is decided here is what each row
// is called and in which language, which is a question about this request.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	found := shell.GatherFindings(authority, snap)
	articleLang := articleLanguageLookup(snap)
	view := pages.HealthView{
		Unwritten:             healthLinks(found.Unwritten, articleLang),
		TitleOnly:             healthTitleLinks(found.TitleOnly, articleLang),
		Islands:               healthIslands(found.Islands, lang, articleLang),
		IslandCount:           healthIslandCount(found.Islands),
		Collisions:            healthCollisions(found.Collisions, articleLang),
		Blocked:               healthBlocked(found.Blocked),
		Skipped:               healthSkipped(found.Skipped),
		StatusOutsideEnum:     healthStatusNotes(found.StatusOutsideEnum, articleLang),
		StatusUnreachable:     healthStatusNotes(found.StatusUnreachable, articleLang),
		FrontmatterUnreadable: healthNoteFindings(found.FrontmatterUnreadable, articleLang),
		SchemaFaults:          healthNoteFindings(found.SchemaFaults, articleLang),
		InstanceScopeUnknown:  found.InstanceScopeUnknown,
		// A folder that declared no vocabulary has no schema findings to
		// report, and that is an answer rather than a failure — the view says
		// nothing in that case, which is why this reads the diagnostic instead
		// of the closed flag. What it carries is whatever actually failed: a
		// contract that could not be read, or one that read and named a
		// folder its artifacts section may not name.
		SchemaScopeUnknown: authority.Diagnostic(lang),
		LastComplete:       lastCompleteBuild(found.LastComplete),
		// A word the table cannot order by leaves the page in its default
		// order: the reader asked for this page, and the ordering is how it is
		// laid out rather than what it is about.
		Sort:    pages.ParseHealthColumn(r.URL.Query().Get("sort")),
		Sidebar: pages.NewSidebar(pageShell, ""),
	}
	if err := pages.Health(view, layouts.ChromeFromRequest(r, wording.HealthTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write health page", "error", err)
	}
}

// healthStatusNotes carries the gathered notes whose status their type never
// declared across to the page, each title marked with the language its author
// wrote it in. The rows arrive in the index's own path order, which is the
// order the rest of the page lists findings in.
func healthStatusNotes(found []shell.StatusNote, articleLang pages.ArticleLanguageFor) []pages.HealthStatusNote {
	out := make([]pages.HealthStatusNote, 0, len(found))
	for _, row := range found {
		out = append(out, pages.HealthStatusNote{
			Note:   noteRef(row.Note, articleLang),
			Type:   row.Type,
			Status: row.Status,
		})
	}
	return out
}

// healthNoteFindings carries the gathered notes the schema had something to say
// about across to the page. The rows carry no words of their own: each note's
// own page says which field and why, and one file described twice in two places
// is how two accounts of it start to disagree. What they do carry is how many
// things were said about the note and how heavy the heaviest was — a number and
// a weight the note's own page never states, and the only way the table can
// tell one note that drew a single complaint from one that drew nine.
func healthNoteFindings(found []snapshot.HealthNoteFindings, articleLang pages.ArticleLanguageFor) []pages.HealthNoteFindings {
	out := make([]pages.HealthNoteFindings, 0, len(found))
	for _, row := range found {
		out = append(out, pages.HealthNoteFindings{
			Note:     noteRef(row.Note, articleLang),
			Severity: row.Severity,
			Count:    row.Count,
		})
	}
	return out
}

// healthLinks carries the gathered citations with nowhere to land across to the
// page, each note marked with the language its author wrote it in.
func healthLinks(links []snapshot.HealthLink, articleLang pages.ArticleLanguageFor) []snapshot.HealthLink {
	if articleLang == nil {
		return links
	}
	out := make([]snapshot.HealthLink, len(links))
	for i, link := range links {
		out[i] = snapshot.HealthLink{From: noteRef(link.From, articleLang), Target: link.Target}
	}
	return out
}

func healthTitleLinks(links []snapshot.HealthTitleLink, articleLang pages.ArticleLanguageFor) []snapshot.HealthTitleLink {
	if articleLang == nil {
		return links
	}
	out := make([]snapshot.HealthTitleLink, len(links))
	for i, link := range links {
		out[i] = snapshot.HealthTitleLink{
			From:   noteRef(link.From, articleLang),
			Target: link.Target,
			Note:   noteRef(link.Note, articleLang),
		}
	}
	return out
}

func noteRefs(refs []nav.NoteRef, articleLang pages.ArticleLanguageFor) []nav.NoteRef {
	if articleLang == nil {
		return refs
	}
	out := make([]nav.NoteRef, len(refs))
	for i, ref := range refs {
		out[i] = noteRef(ref, articleLang)
	}
	return out
}

func noteRef(ref nav.NoteRef, articleLang pages.ArticleLanguageFor) nav.NoteRef {
	if ref.Language != "" || articleLang == nil {
		return ref
	}
	ref.Language = articleLang(ref.RelPath)
	return ref
}

// healthIslands names each folder for the reader in front of it. The folder at
// the top of the vault has no name of its own, and what stands in for it is a
// word rather than a path, so it is chosen here — where the request says which
// language to choose it in — rather than by the scan that grouped the notes.
func healthIslands(groups []snapshot.HealthIslandGroup, lang wording.Lang, articleLang pages.ArticleLanguageFor) []pages.HealthIslandGroup {
	out := make([]pages.HealthIslandGroup, 0, len(groups))
	for _, g := range groups {
		name := g.Dir
		if name == "" {
			name = wording.VaultRoot.In(lang)
		}
		out = append(out, pages.HealthIslandGroup{Dir: g.Dir, Name: name, Notes: noteRefs(g.Notes, articleLang)})
	}
	return out
}

func healthIslandCount(groups []snapshot.HealthIslandGroup) int {
	total := 0
	for _, g := range groups {
		total += len(g.Notes)
	}
	return total
}

// healthBlocked carries the freshness record's blocked sources across to the
// page as plain values, like every other health finding.
func healthBlocked(blocked []snapshot.BlockedSource) []pages.HealthBlockedSource {
	out := make([]pages.HealthBlockedSource, 0, len(blocked))
	for _, source := range blocked {
		out = append(out, pages.HealthBlockedSource{Path: source.Path, Reason: source.Reason})
	}
	return out
}

// healthSkipped carries the generation's unindexed paths across to the page.
// They are not a freshness fact like the blocked list: a later reading will
// skip them again, so the page states them as they are rather than as
// something that may recover.
func healthSkipped(skipped []snapshot.Skipped) []pages.HealthSkippedSource {
	out := make([]pages.HealthSkippedSource, 0, len(skipped))
	for _, source := range skipped {
		out = append(out, pages.HealthSkippedSource{Path: source.Path, Reason: source.Reason, Size: source.Size})
	}
	return out
}

// lastCompleteBuild formats when the folder was last read whole, which is not
// always when the generation behind this page was built: a generation
// published without the sources it could not re-read carries the time of the
// last one that did read everything. Empty means there has been no whole read
// since startup, and the page says that instead — which it may not say while
// one has happened, because a reader deciding whether to trust the page is
// then being told the folder has never been seen entire.
func lastCompleteBuild(lastComplete time.Time) string {
	if lastComplete.IsZero() {
		return ""
	}
	return lastComplete.Format("2006-01-02 15:04")
}

// healthCollisions maps each shared name onto the page type, which keeps
// Candidates as []nav.NoteRef rather than the generation's []string.
func healthCollisions(collisions []snapshot.HealthCollision, articleLang pages.ArticleLanguageFor) []pages.HealthCollision {
	out := make([]pages.HealthCollision, 0, len(collisions))
	for _, collision := range collisions {
		// Every row of one collision would otherwise read the same word:
		// nav.Label names a file by its base name, and these files collide
		// precisely because they share it. The path is the only thing that
		// separates them, and separating them is the whole point of the list.
		candidates := make([]nav.NoteRef, 0, len(collision.Candidates))
		for _, candidate := range collision.Candidates {
			candidates = append(candidates, noteRef(nav.NoteRef{Name: candidate, RelPath: candidate}, articleLang))
		}
		out = append(out, pages.HealthCollision{Name: collision.Name, Candidates: candidates})
	}
	return out
}
