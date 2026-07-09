// Package note is the reading feature: it loads a note from the vault,
// renders it, and serves the reading page.
package note

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// sealStatus is the one primary status — the koopa-only seal. Only a ready note
// carries the seal display, so the handler gates its git-provenance read and the
// one-shot seal signal on this value; it duplicates no schema enum.
const sealStatus = "ready"

// typeLesson is the one note type the reading page enriches with lesson-body
// interactions (the TTS speak buttons today; the slot machine and concept
// drawer next). Like sealStatus, it names in one place the single spot the
// handler treats a type specially, and it duplicates no enum: the type
// vocabulary lives in the schema contract's enums.type (vault-schema.toml),
// the single source of schema truth. render.HTML stays generic — the lesson decision is
// made here, so a non-lesson note never grows lesson affordances.
const typeLesson = "lesson"

// StatusPolicy is the read-only projection of the write face the reading page
// needs: whether the face is closed (Closed), which transition keys to offer
// (Transitions), the stable note-status axis the Lifecycle rail lists (Order),
// and whether a note still has an owner-held onward move (Advanceable), for the
// sidebar's pending-decision count. It is a genuine slice of *status.Service —
// never its write path: Flip, the single status write, stays out of the reading
// page's reach. *status.Service satisfies this.
type StatusPolicy interface {
	Closed() bool
	Transitions(noteType, current string) []string
	Order() []string
	Advanceable(noteType, current string) bool
}

// Deps is everything the reading feature reads from. Grouping the providers in a
// struct keeps the constructor within the parameter budget as the page grew a
// per-status count source (Counts, from the snapshot's search index) and a
// read-only git-provenance source (Provenance, from the write face). Counts,
// Provenance, and Nav are plain closures because "give me the current value" is
// a closure, not a method set — the models live behind the atomic snapshot and
// must be read fresh per request.
type Deps struct {
	Root     string
	Renderer *render.Renderer
	Status   StatusPolicy
	Nav      func() *nav.Model
	Counts   func() map[string]int
	// TypeStatusCounts tallies notes by (type, status) from the current
	// snapshot, so the sidebar can weigh each note's onward transitions against
	// the contract for its pending-decision count. Kept apart from Counts because
	// that pairing, not the flat status tally, is what the count needs.
	TypeStatusCounts func() map[search.TypeStatus]int
	Provenance       func(ctx context.Context, rel string) (string, error)
	Log              *slog.Logger
	// Slots is the lesson slot-machine sidecar index, loaded once at
	// startup. Unlike the closures above it is static (slot sidecars are never
	// indexed as notes and never enter the scanner's rebuilt-on-change snapshot
	// — they are a separate read path). A nil index is legal: it just
	// means no lesson carries a slot machine, so it is not a required dependency.
	Slots lesson.SlotIndex
	// Concepts indexes the grammar concept notes a lesson may link to, for the
	// in-app concept sheet. Also static and also optional: a nil index means no
	// wikilink ever becomes a sheet trigger, so lessons just navigate to concept
	// notes as usual.
	Concepts lesson.ConceptIndex
}

// Handler serves reading pages for a vault rooted at Deps.Root.
type Handler struct {
	deps Deps
}

// NewHandler wires the reading feature. Every dependency must be non-nil: a nil
// is a wiring bug that must fail here, not on the first request three calls deep
// inside show(). A fail-closed write face is still a non-nil Status whose Closed
// reports true, not a missing one.
//
//nolint:gocritic // hugeParam: Deps (80B) is composed once at startup wiring and passed by value to match the sibling constructor (syllabus.NewHandler); by-value keeps the single call site a literal struct composite, and this is not a hot path.
func NewHandler(d Deps) *Handler {
	if d.Renderer == nil {
		panic("note: NewHandler requires a non-nil Renderer")
	}
	if d.Status == nil {
		panic("note: NewHandler requires a non-nil Status")
	}
	if d.Nav == nil {
		panic("note: NewHandler requires a non-nil Nav provider")
	}
	if d.Counts == nil {
		panic("note: NewHandler requires a non-nil Counts provider")
	}
	if d.TypeStatusCounts == nil {
		panic("note: NewHandler requires a non-nil TypeStatusCounts provider")
	}
	if d.Provenance == nil {
		panic("note: NewHandler requires a non-nil Provenance provider")
	}
	if d.Log == nil {
		panic("note: NewHandler requires a non-nil Log")
	}
	return &Handler{deps: d}
}

// Register mounts the feature's routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /raw/{path...}", h.raw)
	mux.HandleFunc("GET /{$}", h.home)
}

// home redirects to the vault README. The Vault-Index home page is deferred by
// design; until it is built, / lands on README.md.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/notes/README.md", http.StatusFound)
}

// show serves one entry of the browse tree. Every file the tree lists opens
// here; the presentation follows the kind. Markdown is the note page — the
// reading surface with its status face, table of contents and diagnostics.
// Everything else is a read-only view of a file, built by showFile.
//
// A note's body reaches this first-party page through the markdown pipeline
// with WithUnsafe, which passes raw HTML — including <script> — through
// unchanged. That is why no other kind may take this path: a vault .html file
// rendered here would run its scripts against the whole vault-reading surface.
// The other kinds are shown as escaped source or handed to the browser through
// the sandboxed raw endpoint, never poured into this page.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if !servable(rel) {
		http.NotFound(w, r)
		return
	}
	if !strings.HasSuffix(rel, ".md") {
		h.showFile(w, r, rel)
		return
	}

	n, err := h.readNote(rel)
	switch {
	case errors.Is(err, errUnreadable):
		// The file was there and allowed; the read itself failed.
		h.deps.Log.Error("read note", "path", rel, "error", err)
		http.Error(w, "cannot read note", http.StatusInternalServerError)
		return
	case err != nil:
		// A note that vanished, and one the vault root turned away, answer
		// alike: neither is here, and which is which is not the caller's.
		h.deps.Log.Warn("read note", "path", rel, "error", err)
		http.NotFound(w, r)
		return
	}

	// render.Renderer.HTML never fails the whole render: a content-level
	// problem becomes a Diagnostic, not an error — no error path left to handle.
	result := h.deps.Renderer.HTML(n.Body)

	// Lesson bodies get the read-aloud affordance: wrap each ruby-bearing
	// sentence with a speak button whose text has the furigana stripped
	// server-side (render.InjectTTS). The gate is here, not in render, so
	// render.HTML stays a generic note renderer — a diary or concept note that
	// contains <ruby> never grows speaker buttons. A lesson with a slot sidecar
	// (joined by slug, never filename) also gets its sentence-pattern machine spliced in,
	// and its wikilinks to concept notes become in-app sheet triggers.
	var concepts []lesson.ConceptDoc
	if n.Type() == typeLesson {
		result.HTML = render.InjectTTS(result.HTML)
		result.HTML = h.injectSlotMachine(r.Context(), rel, n.Slug(), result.HTML)
		var refs []string
		result.HTML, refs = render.InjectConceptTriggers(result.HTML, h.deps.Concepts.SlugForPath)
		concepts = h.loadConcepts(refs)
	}

	pending, pendingKnown := h.pending()
	view := pages.NoteView{
		Title:             n.Title(),
		RelPath:           n.RelPath,
		Type:              n.Type(),
		Status:            n.Status(),
		Diagnostic:        n.FMDiagnostic,
		RenderDiagnostics: result.Diagnostics,
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		Sidebar:           pages.NewSidebar(h.deps.Nav(), n.RelPath, h.lifecycle(n.Status()), pending, pendingKnown),
		WriteClosed:       h.deps.Status.Closed(),
		Concepts:          concepts,
	}
	switch {
	case n.FMDiagnostic != "":
		// Bad YAML: diagnostic only, no keys — read isn't reliable enough to
		// write.
	case n.Frontmatter == nil:
		// Legally no frontmatter (e.g. drills): no keys either.
		view.NoFrontmatter = true
	default:
		view.Transitions = h.deps.Status.Transitions(n.Type(), n.Status())
	}

	// A ready note shows the seal, so only then spend a git read for the commit
	// short-hash and honor the one-shot ?sealed=1 ceremony signal.
	if n.Status() == sealStatus {
		view.JustSealed = r.URL.Query().Get("sealed") == "1"
		if hash, herr := h.deps.Provenance(r.Context(), rel); herr == nil {
			view.SealedHash = hash
		} else {
			h.deps.Log.Warn("seal provenance", "path", rel, "error", herr)
		}
	}

	if err := pages.Note(view, pages.ChromeFromRequest(r, n.Title())).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write note page", "path", rel, "error", err)
	}
}

// injectSlotMachine splices this lesson's slot-pattern machine into its rendered
// body when a sidecar joins by slug. It renders the templ component to a
// string and inserts it after the lesson's first table — the 文型骨架 pattern
// skeleton — matching the lesson's own pedagogy (practise the patterns before
// the reading passages), falling back to appending when a lesson has no table.
// A render failure is logged and the body returned unchanged: a broken machine
// must never blank the page.
func (h *Handler) injectSlotMachine(ctx context.Context, rel, slug, body string) string {
	sc, ok := h.deps.Slots.Lookup(slug)
	if !ok {
		return body
	}
	var buf bytes.Buffer
	if err := pages.SlotMachine(sc).Render(ctx, &buf); err != nil {
		h.deps.Log.Error("render slot machine", "path", rel, "slug", slug, "error", err)
		return body
	}
	machine := buf.String()
	if i := strings.Index(body, "</table>"); i >= 0 {
		end := i + len("</table>")
		return body[:end] + machine + body[end:]
	}
	return body + machine
}

// loadConcepts renders each referenced concept note into a sheet document. A
// concept body is rendered through the plain note pipeline (no concept post-pass
// of its own), so its wikilinks stay ordinary links and the sheet never nests. A
// concept that fails to load is skipped — its trigger stays a working link to
// the note, so no dead sheet ships: degrade, never break.
func (h *Handler) loadConcepts(refs []string) []lesson.ConceptDoc {
	if len(refs) == 0 {
		return nil
	}
	renderBody := func(body string) string { return h.deps.Renderer.HTML(body).HTML }
	docs := make([]lesson.ConceptDoc, 0, len(refs))
	for _, rel := range refs {
		if d, ok := lesson.LoadConcept(renderBody, h.deps.Concepts, h.deps.Root, rel); ok {
			docs = append(docs, d)
		}
	}
	return docs
}

// lifecycle assembles the status-first Lifecycle rail: the note group's statuses
// in the contract's toml order (Status.Order — never hardcoded), each with its
// live snapshot count and whether it is the current note's status. Empty when
// the write face is closed.
func (h *Handler) lifecycle(current string) []pages.LifecycleItem {
	order := h.deps.Status.Order()
	if len(order) == 0 {
		return nil
	}
	counts := h.deps.Counts()
	items := make([]pages.LifecycleItem, 0, len(order))
	for _, s := range order {
		items = append(items, pages.LifecycleItem{
			Name:   s,
			Count:  counts[s],
			Active: s == current,
		})
	}
	return items
}

// pending counts the notes still awaiting a decision: those whose (type, status)
// still has an owner-held onward move in the contract, excluding the seal itself
// — the final human act on a note is not something still pending. It returns
// known = false when the write face is closed, so the sidebar shows no figure
// rather than a misleading zero. The predicate reuses the write face's own
// contract reading; the seal is named in one place, never a second status list.
func (h *Handler) pending() (count int, known bool) {
	if h.deps.Status.Closed() {
		return 0, false
	}
	for ts, n := range h.deps.TypeStatusCounts() {
		if ts.Status == sealStatus {
			continue
		}
		if h.deps.Status.Advanceable(ts.Type, ts.Status) {
			count += n
		}
	}
	return count, true
}
