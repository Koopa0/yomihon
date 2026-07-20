// Package note owns the general reading surface: Home, rendered notes, raw
// bytes, and the honest fallback page for vault files without a dedicated
// reader. Status mutation remains in internal/status.
package note

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

// typeLesson is the one note type the reading page enriches with lesson-body
// interactions (the TTS speak buttons today; the slot machine and concept
// drawer next). It names in one place the single spot the handler treats a
// type specially, and it duplicates no enum: the type
// vocabulary lives in the schema contract's enums.type (vault-schema.toml),
// the single source of schema truth. render.HTML stays generic — the lesson decision is
// made here, so a non-lesson note never grows lesson affordances.
const typeLesson = "lesson"

// homeRecentLimit keeps the landing page a trailhead rather than another
// browse surface. The complete vault remains reachable through navigation and
// search.
const homeRecentLimit = 7

// Dependencies is everything the reading feature reads from. Grouping the providers in a
// struct keeps the constructor within the parameter budget. Snapshot is one
// closure because a request must read the atomic pointer once and derive its
// navigation and counts from that coherent value. Status captures one immutable
// lifecycle view for the request. Source changes affect the next request;
// write publication still revalidates current authority under the lifecycle
// lock. Provenance is a closure over the read-only git query owned by the write
// package and binds the result to the exact note bytes this request read.
type Dependencies struct {
	Source     *vault.Reader
	Status     func() status.View
	Snapshot   func() *snapshot.View
	Provenance func(ctx context.Context, rel string, content [sha256.Size]byte) (string, error)
	Log        *slog.Logger
}

// Handler serves reading pages from one rooted vault capability and its
// coherently published snapshots.
type Handler struct {
	deps Dependencies
}

// New wires the reading feature. It defensively copies the startup-owned
// dependency record so later field reassignment by the caller cannot rewire a
// live handler. Every required reference and function must be non-nil: a wiring
// bug fails here, not on the first request three calls deep inside show(). A
// fail-closed write face still provides a closed status.View.
func New(d *Dependencies) *Handler {
	if d == nil {
		panic("note: New requires non-nil Dependencies")
	}
	if d.Source == nil {
		panic("note: New requires a non-nil Source")
	}
	if d.Status == nil {
		panic("note: New requires a non-nil Status")
	}
	if d.Snapshot == nil {
		panic("note: New requires a non-nil Snapshot provider")
	}
	if d.Provenance == nil {
		panic("note: New requires a non-nil Provenance provider")
	}
	if d.Log == nil {
		panic("note: New requires a non-nil Log")
	}
	return &Handler{deps: *d}
}

// Register mounts the feature's routes.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /raw/{path...}", h.raw)
	mux.HandleFunc("GET /{$}", h.home)
}

// home renders the four landing blocks from one coherent snapshot, followed by
// the vault README through the same markdown pipeline used by a note page. It
// is a read face: no status forms or write capability enter the view.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	statusView := h.deps.Status()
	snap := h.deps.Snapshot().Capture()
	readme, ok := snap.Note("README.md")
	readmeHTML := ""
	if !ok {
		h.deps.Log.Warn("home README is absent from the request snapshot")
	} else {
		readmeHTML = snap.Render(readme.Body).HTML
	}
	artifactPolicy := snap.ArtifactPolicy()
	pageShell := shell.Project(statusView, artifactPolicy, snap)
	lifecycle, lifecycleDiagnostic := h.lifecycle(statusView, snap, "")
	if lifecycleDiagnostic == "" && !artifactPolicy.Available() {
		lifecycle = nil
		lifecycleDiagnostic = artifactPolicy.Diagnostic()
	}
	visibleNav := pageShell.Nav
	recent := recentHomeNotes(visibleNav.KnowledgeNotes())
	recentDiagnostic := ""
	if visibleNav.ArtifactDiagnostic() != "" {
		recent = nil
		recentDiagnostic = visibleNav.ArtifactDiagnostic()
	}
	pathDiagnostics := make([]string, 0, 2)
	if visibleNav.NavigationDiagnostic() != "" {
		pathDiagnostics = append(pathDiagnostics, visibleNav.NavigationDiagnostic())
	}
	if visibleNav.ArtifactDiagnostic() != "" {
		pathDiagnostics = append(pathDiagnostics, visibleNav.ArtifactDiagnostic())
	}
	view := pages.HomeView{
		Recent:              recent,
		RecentDiagnostic:    recentDiagnostic,
		Lifecycle:           lifecycle,
		LifecycleDiagnostic: lifecycleDiagnostic,
		Paths:               homePaths(visibleNav.Paths()),
		PathDiagnostics:     pathDiagnostics,
		ReadmeHTML:          readmeHTML,
		ReadmeMissing:       !ok,
		Sidebar:             pages.NewSidebar(visibleNav, ""),
	}
	if err := pages.Home(view, pageShell.Chrome(r, "首頁")).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write home page", "error", err)
	}
}

// show serves one entry of the browse tree. Every file the tree lists opens
// here; the presentation follows the kind. Markdown is the note page — the
// reading surface with its status face, table of contents and diagnostics.
// Everything else is a read-only view of a file, built by showFile.
//
// A note's body reaches this first-party page through the markdown pipeline's
// inert authored-markup subset. Ruby reading aids survive; executable,
// navigating, and automatically loading tags are visible text, never browser
// authority. Other file kinds are likewise shown as escaped source or handed
// to the browser through the sandboxed raw endpoint, never poured into this
// page as live markup.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !servable(rel) {
		http.Error(w, "找不到指定的 vault 項目", http.StatusNotFound)
		return
	}
	statusView := h.deps.Status()
	snap := h.deps.Snapshot().Capture()
	if !strings.HasSuffix(rel, ".md") {
		h.showFile(w, r, rel, statusView, snap)
		return
	}

	n, ok := snap.Note(rel)
	if !ok {
		h.deps.Log.Warn("note is absent from the request snapshot", "path", rel)
		http.Error(w, "找不到指定的筆記", http.StatusNotFound)
		return
	}

	artifactPolicy := snap.ArtifactPolicy()
	governance := h.governance(&n, snap, statusView, artifactPolicy)
	// render.Pipeline.HTML never fails the whole render: a content-level
	// problem becomes a Diagnostic, not an error — no error path left to handle.
	result := snap.Render(n.Body)

	// Governed lesson bodies get the read-aloud affordance: wrap each ruby-bearing
	// sentence with a speak button whose text has the furigana stripped
	// server-side (render.InjectTTS). Both governed-instance classification and
	// the lesson type are required here before any lesson affordance is added. A
	// governed lesson with a slot sidecar also gets its sentence-pattern machine,
	// and its concept wikilinks become in-app sheet triggers. This happens only
	// after the request's captured authority has classified the note, so every
	// projection in this response uses one coherent lifecycle view.
	var concepts []lesson.ConceptDoc
	if governance.instance && n.Type == typeLesson {
		result.HTML = render.InjectTTS(result.HTML)
		pageChrome := governance.shell.Chrome(r, n.Title)
		result.HTML = h.injectSlotMachine(r.Context(), snap.Slots(), rel, n.Slug, result.HTML, pageChrome.Nonce)
		var refs []string
		result.HTML, refs = render.InjectConceptTriggers(result.HTML, snap.Concepts().IDForPath)
		concepts = h.loadConcepts(snap, refs)
	}
	if n.LanguageDiagnostic != "" {
		h.deps.Log.Warn("invalid article language; using und", "path", rel, "error", n.LanguageDiagnostic)
	}

	view := pages.NoteView{
		Title:             n.Title,
		RelPath:           n.RelPath,
		Language:          n.Language,
		Type:              n.Type,
		Status:            n.Status,
		SealTarget:        schema.SealStatus,
		Sealed:            governance.instance && n.Status == schema.SealStatus,
		Diagnostic:        n.FMDiagnostic,
		RenderDiagnostics: result.Diagnostics,
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		Sidebar:           pages.NewSidebar(governance.shell.Nav, n.RelPath),
		NonInstance:       governance.nonInstance,
		WriteDiagnostic:   governance.writeDiagnostic,
		Concepts:          concepts,
		Transitions:       governance.transitions,
		NoFrontmatter:     governance.noFrontmatter,
	}

	h.addSealProvenance(r.Context(), rel, n.ContentHash, r.URL.Query().Get("sealed") == "1", &view)

	pageChrome := governance.shell.Chrome(r, n.Title)
	if err := pages.Note(view, pageChrome).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write note page", "path", rel, "error", err)
	}
}

type governanceState struct {
	shell           pages.Shell
	transitions     []string
	writeDiagnostic string
	instance        bool
	nonInstance     bool
	noFrontmatter   bool
}

func (h *Handler) governance(
	n *snapshot.Note,
	snap *snapshot.View,
	statusView status.View,
	policy schema.ArtifactPolicy,
) governanceState {
	pageShell := shell.Project(statusView, policy, snap)
	authorityAvailable := statusView.Diagnostic() == "" && policy.Available()
	state := governanceState{
		shell:       pageShell,
		instance:    authorityAvailable && !policy.IsNonInstance(n.RelPath),
		nonInstance: authorityAvailable && policy.IsNonInstance(n.RelPath),
	}
	state.writeDiagnostic = statusView.WriteDiagnostic()
	if state.writeDiagnostic == "" && !policy.Available() {
		state.writeDiagnostic = policy.Diagnostic()
	}
	if state.instance && state.writeDiagnostic == "" {
		switch {
		case n.FMDiagnostic != "":
			// Bad YAML: diagnostic only, no keys — read isn't reliable enough to
			// write.
		case !n.HasFrontmatter:
			// Legally no frontmatter (e.g. drills): no keys either.
			state.noFrontmatter = true
		default:
			state.transitions = statusView.Transitions(n.RelPath, n.Type, n.Status)
		}
	}
	return state
}

// addSealProvenance performs the git read only for a governed sealed note and
// carries the redirect's one-shot ceremony signal into the view.
func (h *Handler) addSealProvenance(
	ctx context.Context,
	rel string,
	contentHash [sha256.Size]byte,
	justSealed bool,
	view *pages.NoteView,
) {
	if !view.Sealed {
		return
	}
	view.JustSealed = justSealed
	hash, err := h.deps.Provenance(ctx, rel, contentHash)
	if err != nil {
		h.deps.Log.Warn("seal provenance", "path", rel, "error", err)
		return
	}
	view.SealedHash = hash
}

// injectSlotMachine splices this lesson's slot-pattern machine into its rendered
// body when a sidecar joins by slug. It renders the templ component to a
// string and inserts it after the lesson's first table — the 文型骨架 pattern
// skeleton — matching the lesson's own pedagogy (practise the patterns before
// the reading passages), falling back to appending when a lesson has no table.
// A render failure is logged and the body returned unchanged: a broken machine
// must never blank the page.
func (h *Handler) injectSlotMachine(
	ctx context.Context,
	slots lesson.SlotIndex,
	rel, slug, body, nonce string,
) string {
	sc, ok := slots.Lookup(slug)
	if !ok {
		return body
	}
	var buf bytes.Buffer
	if err := pages.SlotMachine(sc, nonce).Render(ctx, &buf); err != nil {
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
func (h *Handler) loadConcepts(
	snap *snapshot.View,
	refs []string,
) []lesson.ConceptDoc {
	if len(refs) == 0 {
		return nil
	}
	renderBody := func(body string) string { return snap.Render(body).HTML }
	docs := make([]lesson.ConceptDoc, 0, len(refs))
	for _, rel := range refs {
		if d, ok := snap.Concepts().Document(renderBody, rel); ok {
			docs = append(docs, d)
		}
	}
	return docs
}

// lifecycle assembles Home's Lifecycle block: the note group's statuses in the
// contract's toml order, each with its live snapshot count and whether it is
// current. The diagnostic distinguishes an unavailable write contract from an
// unavailable artifact-metadata capability, so Home never presents either as a
// successful empty projection.
func (h *Handler) lifecycle(
	statusView status.View,
	snap *snapshot.View,
	current string,
) (items []pages.LifecycleItem, diagnostic string) {
	if diagnostic := statusView.Diagnostic(); diagnostic != "" {
		return nil, diagnostic
	}
	order := statusView.Order()
	if order == nil {
		return nil, "Lifecycle is unavailable while the contract is closed."
	}
	if len(order) == 0 {
		return nil, "Lifecycle is unavailable because the contract declares no note statuses."
	}
	counts, err := snap.Search().CountByStatus()
	if err != nil {
		return nil, err.Error()
	}
	items = make([]pages.LifecycleItem, 0, len(order))
	for _, s := range order {
		items = append(items, pages.LifecycleItem{
			Name:   s,
			Count:  counts[s],
			Active: s == current,
			Sealed: s == schema.SealStatus,
		})
	}
	return items, ""
}

// recentHomeNotes selects the newest knowledge notes from the snapshot's
// scanner-captured timestamps. It sorts a clone, leaving the published model
// immutable for concurrent readers. Equal mtimes fall back to path order so a
// rebuild produces stable output.
func recentHomeNotes(notes []nav.NoteSummary) []pages.HomeNote {
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

	out := make([]pages.HomeNote, 0, len(notes))
	for _, n := range notes {
		item := pages.HomeNote{Title: n.Title, RelPath: n.RelPath, Type: n.Type, Status: n.Status}
		if !n.Modified.IsZero() {
			item.Modified = n.Modified.Format("2006-01-02")
			item.ModifiedAt = n.Modified.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out
}

// homePaths maps the snapshot's parsed study paths onto the small progress
// figures Home displays. nav owns the ready/total derivation shared with the
// full study-path page, so the two reading surfaces cannot drift.
func homePaths(paths []nav.Map) []pages.HomePath {
	out := make([]pages.HomePath, 0, len(paths))
	for i := range paths {
		path := &paths[i]
		ready, total := path.EntryCounts(schema.SealStatus)
		out = append(out, pages.HomePath{
			Title:   path.Title,
			RelPath: path.RelPath,
			Ready:   ready,
			Total:   total,
		})
	}
	return out
}
