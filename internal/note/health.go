package note

import (
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// health renders the whole-folder view of what needs attention. Every fact on
// it is already computed for the single-note pages; nobody opens every note, so
// gathering them is the only way they are ever seen.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(authority, snap)
	health := snap.Health()
	fresh := snap.Freshness()
	unreadableFrontmatter, schemaFaults := schemaFaultLists(snap)
	view := pages.HealthView{
		Unwritten:             healthLinks(health.Unwritten),
		TitleOnly:             healthTitleLinks(health.TitleOnly),
		Islands:               healthIslands(health.Islands),
		IslandCount:           healthIslandCount(health.Islands),
		Collisions:            healthCollisions(health.Collisions),
		Blocked:               healthBlocked(fresh.Blocked),
		StatusOutsideEnum:     statusesOutsideEnum(authority, snap),
		FrontmatterUnreadable: unreadableFrontmatter,
		SchemaFaults:          schemaFaults,
		InstanceScopeUnknown:  health.InstanceScopeUnknown,
		// A folder that declared no vocabulary has no schema findings to
		// report, and that is an answer rather than a failure — the view says
		// nothing in that case, which is why this reads the diagnostic instead
		// of the closed flag. What it carries is whatever actually failed: a
		// contract that could not be read, or one that read and named a
		// folder its artifacts section may not name.
		SchemaScopeUnknown: authority.Diagnostic(),
		LastComplete:       lastCompleteBuild(&fresh),
		Sidebar:            pages.NewSidebar(pageShell.Nav, ""),
	}
	if err := pages.Health(view, layouts.ChromeFromRequest(r, wording.HealthTitle.In(lang))).Render(r.Context(), w); err != nil {
		h.sources.Log.Error("write health page", "error", err)
	}
}

// schemaFaultLists splits what the schema said about the whole folder into the
// two things a reader does differently about them: frontmatter that cannot be
// read at all, which has to be repaired before anything else about the note
// can be judged, and frontmatter that reads and carries something the schema
// does not accept, which has a named field to change.
//
// The split is on the rule that fired rather than on a guess about the note,
// because one of these findings is the judge's own statement that it could
// read nothing. The rows carry no detail: each note's own page says which
// field and why, and one file described twice in two places is how two
// accounts of it start to disagree.
func schemaFaultLists(snap *snapshot.Generation) (unreadable, faults []nav.NoteRef) {
	for _, entry := range snap.Files() {
		rel := entry.Path()
		findings := snap.SchemaFindings(rel)
		if len(findings) == 0 {
			continue
		}
		note, ok := snap.Note(rel)
		if !ok {
			continue
		}
		ref := nav.NoteRef{RelPath: rel, Name: note.Title}
		if slices.ContainsFunc(findings, func(f judge.Finding) bool { return f.RuleID == "schema.frontmatter" }) {
			unreadable = append(unreadable, ref)
			continue
		}
		faults = append(faults, ref)
	}
	return unreadable, faults
}

// statusesOutsideEnum names the notes whose status value is outside their
// type's declared list — the whole-folder gathering of the flag each note
// page and distribution chip already shows one at a time. It reads the same
// entries the distribution counts, so the two faces cannot disagree about
// which notes exist, and the page states its number by counting what this
// returns rather than by adding a second sum nothing reconciles against it.
// When the authority is closed or the entries are unavailable it names none
// and the page carries no line: an unknowable finding must not pose as one.
//
// The rows arrive in the index's own path order, which is the order the rest
// of the page lists findings in.
func statusesOutsideEnum(authority status.Authority, snap *snapshot.Generation) []pages.HealthStatusNote {
	if !authority.Governed() || authority.Closed() {
		return nil
	}
	holders, err := snap.Search().StatusHolders()
	if err != nil {
		return nil
	}
	out := make([]pages.HealthStatusNote, 0, len(holders))
	for _, h := range holders {
		if authority.KnownStatus(h.Type, h.Status) {
			continue
		}
		out = append(out, pages.HealthStatusNote{
			Note:   nav.NoteRef{Name: healthNoteName(h.RelPath), RelPath: h.RelPath},
			Type:   h.Type,
			Status: h.Status,
		})
	}
	return out
}

// healthNoteName is the words a health row shows for a note, derived the way
// navigation derives them: the file name without its extension. Every other
// section of this page names notes that way, so one note cannot appear as two
// different things on one screen. It is also the honest identifier here — a
// frontmatter title is not a name this vault resolves links by, which is a
// confusion the section above this one exists to report.
func healthNoteName(relPath string) string {
	return strings.TrimSuffix(path.Base(relPath), ".md")
}

// healthLinks and healthCollisions carry the snapshot's findings across to the
// page as plain values. The page package holds no feature types — it is what
// keeps a view from importing the generation it renders.
func healthLinks(links []snapshot.HealthLink) []pages.HealthLink {
	out := make([]pages.HealthLink, 0, len(links))
	for _, link := range links {
		out = append(out, pages.HealthLink{From: link.From, Target: link.Target})
	}
	return out
}

func healthIslands(groups []snapshot.HealthIslandGroup) []pages.HealthIslandGroup {
	out := make([]pages.HealthIslandGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, pages.HealthIslandGroup{Dir: g.Dir, Name: g.Name, Notes: g.Notes})
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

func healthTitleLinks(links []snapshot.HealthTitleLink) []pages.HealthTitleLink {
	out := make([]pages.HealthTitleLink, 0, len(links))
	for _, link := range links {
		out = append(out, pages.HealthTitleLink{From: link.From, Target: link.Target, Note: link.Note})
	}
	return out
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

// lastCompleteBuild formats when the folder was last read whole, which is not
// always when the generation behind this page was built: a generation
// published without the sources it could not re-read carries the time of the
// last one that did read everything. Empty means there has been no whole read
// since startup, and the page says that instead — which it may not say while
// one has happened, because a reader deciding whether to trust the page is
// then being told the folder has never been seen entire.
func lastCompleteBuild(fresh *snapshot.Freshness) string {
	if fresh.LastComplete.IsZero() {
		return ""
	}
	return fresh.LastComplete.Format("2006-01-02 15:04")
}

func healthCollisions(collisions []snapshot.HealthCollision) []pages.HealthCollision {
	out := make([]pages.HealthCollision, 0, len(collisions))
	for _, collision := range collisions {
		// Every row of one collision would otherwise read the same word:
		// nav.Label names a file by its base name, and these files collide
		// precisely because they share it. The path is the only thing that
		// separates them, and separating them is the whole point of the list.
		candidates := make([]nav.NoteRef, 0, len(collision.Candidates))
		for _, candidate := range collision.Candidates {
			candidates = append(candidates, nav.NoteRef{Name: candidate, RelPath: candidate})
		}
		out = append(out, pages.HealthCollision{Name: collision.Name, Candidates: candidates})
	}
	return out
}
