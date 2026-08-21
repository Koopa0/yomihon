// Package note owns the general reading surface: Home, rendered notes, raw
// bytes, and the honest fallback page for vault files without a dedicated
// reader. Status mutation remains in internal/status.
package note

import (
	"bytes"
	"cmp"
	"context"
	"log/slog"
	"maps"
	"net/http"
	"path"
	"slices"
	"strconv"
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

// homeReadmePath is the note Home shows as its own introduction. It is named
// once because the lookup and the render need the same answer: the renderer
// resolves a relative image against the note's directory, so a body fetched
// under one path and rendered under another would address the wrong files.
const homeReadmePath = "README.md"

// Dependencies is everything the reading feature reads from. Grouping the providers in a
// struct keeps the constructor within the parameter budget. Snapshot is one
// closure because a request must read the atomic pointer once and derive its
// navigation and counts from that coherent value. Status captures one immutable
// lifecycle view for the request. Source changes affect the next request; a
// write still revalidates current authority under the lifecycle lock.
type Dependencies struct {
	Source   *vault.Reader
	Status   func() status.View
	Snapshot func() *snapshot.View
	// ObservedStatus is a closure over the write package's read of the note's
	// own status line. The rest of the page comes from a scan that lags the
	// folder by a couple of seconds, which a body and a link graph can afford
	// and an adjudication state cannot: the reader arrives here straight from a
	// write, and a status that lags is one they have already changed.
	ObservedStatus func(rel string) (string, error)
	Log            *slog.Logger
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
	if d.ObservedStatus == nil {
		panic("note: New requires a non-nil ObservedStatus provider")
	}
	if d.Log == nil {
		panic("note: New requires a non-nil Log")
	}
	return &Handler{deps: *d}
}

// Register mounts the feature's routes. The bare "GET /" is the last pattern
// any request can match, and it exists so that none of them reaches the
// router's own fallback, which answers in English and offers nowhere to go.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /raw/{path...}", h.raw)
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /folders/{path...}", h.folder)
	mux.HandleFunc("GET /", h.notFound)
}

// notFound answers a path the vault has nothing at. It is a page rather than a
// line of text because the reader is mid-navigation and needs the way onward
// they were already using: the folder tree, the search, and home.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.showNotFound(w, r, r.URL.Path)
}

// showNotFound renders the not-found page with the status code that belongs to
// it. The path is echoed so the reader can see their own typo; it reaches the
// page as text and is escaped there like any other note content.
func (h *Handler) showNotFound(w http.ResponseWriter, r *http.Request, asked string) {
	h.showMissing(w, r, asked, false)
}

// showUnreadable answers a note the generation captured but could not read:
// the file exists on disk, so the plain not-found page — whose repair is a
// typo or an unwritten note — would send the reader the wrong way.
func (h *Handler) showUnreadable(w http.ResponseWriter, r *http.Request, asked string) {
	h.showMissing(w, r, asked, true)
}

func (h *Handler) showMissing(w http.ResponseWriter, r *http.Request, asked string, unreadable bool) {
	snap := h.deps.Snapshot().Capture()
	pageShell := shell.Project(h.deps.Status(), snap.ArtifactPolicy(), snap)
	view := pages.NotFoundView{
		Asked:      asked,
		Unreadable: unreadable,
		Sidebar:    pages.NewSidebar(pageShell.Nav, ""),
	}
	title := "找不到"
	if unreadable {
		title = "讀不進來"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := pages.NotFound(view, pageShell.Chrome(r, title)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write not-found page", "path", asked, "error", err)
	}
}

// folder shows one folder whole. The rail reaches any note one disclosure at a
// time, which is not the same as seeing a term's lessons in order or a month of
// entries at once — and the breadcrumb, which reads as a trail out of where you
// are, had nowhere to lead until now.
func (h *Handler) folder(w http.ResponseWriter, r *http.Request) {
	dir := path.Clean(strings.Trim(r.PathValue("path"), "/"))
	if dir == "." || dir == ".." || strings.HasPrefix(dir, "../") {
		h.showNotFound(w, r, r.URL.Path)
		return
	}
	dir = vault.NormalizeNFC(dir)
	snap := h.deps.Snapshot().Capture()
	pageShell := shell.Project(h.deps.Status(), snap.ArtifactPolicy(), snap)
	notes, subfolders, ok := pageShell.Nav.Directory(dir)
	if !ok {
		h.showNotFound(w, r, r.URL.Path)
		return
	}
	view := pages.FolderView{
		Dir:        dir,
		Name:       nav.Label(dir),
		Crumbs:     pages.Breadcrumb(dir),
		Subfolders: subfolders,
		Notes:      notes,
		Sidebar:    pages.NewSidebar(pageShell.Nav, ""),
	}
	if err := pages.Folder(view, pageShell.Chrome(r, view.Name)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write folder page", "dir", dir, "error", err)
	}
}

// health renders the whole-folder view of what needs attention. Every fact on
// it is already computed for the single-note pages; nobody opens every note, so
// gathering them is the only way they are ever seen.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	snap := h.deps.Snapshot().Capture()
	pageShell := shell.Project(h.deps.Status(), snap.ArtifactPolicy(), snap)
	health := snap.Health()
	fresh := snap.Freshness()
	view := pages.HealthView{
		Unwritten:    healthLinks(health.Unwritten),
		TitleOnly:    healthTitleLinks(health.TitleOnly),
		Islands:      healthIslands(health.Islands),
		IslandCount:  healthIslandCount(health.Islands),
		Collisions:   healthCollisions(health.Collisions),
		Blocked:      healthBlocked(fresh.Blocked),
		LastComplete: lastCompleteBuild(&fresh),
		Sidebar:      pages.NewSidebar(pageShell.Nav, ""),
	}
	if err := pages.Health(view, pageShell.Chrome(r, "整體狀況")).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write health page", "error", err)
	}
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
		candidates := make([]nav.NoteRef, 0, len(collision.Candidates))
		for _, candidate := range collision.Candidates {
			candidates = append(candidates, nav.NoteRef{Name: nav.Label(candidate), RelPath: candidate})
		}
		out = append(out, pages.HealthCollision{Name: collision.Name, Candidates: candidates})
	}
	return out
}

// home renders the four landing blocks from one coherent snapshot, followed by
// the vault README through the same markdown pipeline used by a note page. It
// is a read face: no status forms or write capability enter the view.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	statusView := h.deps.Status()
	snap := h.deps.Snapshot().Capture()
	// Home links to the folder's own introduction rather than reprinting it,
	// so nothing here renders it and its absence is not news.
	_, hasReadme := snap.Note(homeReadmePath)
	// One reading of the folder's state, used by both halves of the notice
	// below. Read live, it can change between two questions, and the count in
	// the sentence would then be answered by a different set of paths than the
	// technical detail beside it lists.
	fresh := snap.Freshness()
	artifactPolicy := snap.ArtifactPolicy()
	pageShell := shell.Project(statusView, artifactPolicy, snap)
	lifecycle, lifecycleClosed := h.lifecycle(statusView, snap, "")
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
	recentClosed := visibleNav.InstanceProjectionsClosed()
	var recent []pages.HomeNote
	recentOrdered := false
	if !recentClosed {
		recent, recentOrdered = recentHomeNotes(visibleNav.KnowledgeNotes(), pageShell.Governed)
	}
	pathsClosed := visibleNav.NavigationClosure().Closed() || visibleNav.ArtifactClosure().Closed()
	var paths []pages.HomePath
	if !pathsClosed {
		paths = homePaths(visibleNav.Paths())
	}
	content := homeContent{
		recent:    !recentClosed && len(recent) > 0,
		lifecycle: pageShell.Governed && !lifecycleClosed,
		paths:     !pathsClosed && len(paths) > 0,
		withheld:  recentClosed || lifecycleClosed || pathsClosed,
	}
	view := pages.HomeView{
		Governed: pageShell.Governed,
		Subtitle: content.subtitle(),
		StandIn:  homeStandIn(snap, content),
		// The reason a block is missing, stated where the reader is looking. One
		// cause reaches several blocks — a contract that cannot be read closes
		// the lifecycle, the recent list, and the study paths alike — and
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
			statusView.Diagnostic(),
			visibleNav.NavigationDiagnostic(),
			visibleNav.ArtifactDiagnostic(),
		),
		Degraded:        degradedNotice(&fresh),
		DegradedDetail:  blockedDetail(fresh.Blocked),
		Recent:          recent,
		RecentOrdered:   recentOrdered,
		RecentClosed:    recentClosed,
		Lifecycle:       lifecycle,
		LifecycleClosed: lifecycleClosed,
		Paths:           paths,
		PathsClosed:     pathsClosed,
		ReadmeMissing:   !hasReadme,
		Sidebar:         pages.NewSidebar(visibleNav, ""),
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
		h.showNotFound(w, r, r.URL.Path)
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
		// The scan observed a regular file here and this generation has no
		// body for it: the file exists and could not be read, which is a
		// different fact — and a different repair — from a path that names
		// nothing. Asking for the file rather than for anything at the path
		// is what keeps that repair honest: a folder whose name ends in .md
		// is observed by the scan too, and no permission on it can be the one
		// the reader would be sent to clear.
		if _, isFile := snap.Entry(rel); isFile {
			h.deps.Log.Warn("note captured in scan but unreadable in this generation", "path", rel)
			h.showUnreadable(w, r, r.URL.Path)
			return
		}
		h.deps.Log.Warn("note is absent from the request snapshot", "path", rel)
		h.showNotFound(w, r, r.URL.Path)
		return
	}

	artifactPolicy := snap.ArtifactPolicy()
	governance := h.governance(&n, snap, statusView, artifactPolicy)
	// render.Pipeline.HTML never fails the whole render: a content-level
	// problem becomes a Diagnostic, not an error — no error path left to handle.
	result := snap.Render(rel, n.Body)

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

	// The status face and the status shown beside the title are the same
	// claim, so they come from the same read.
	noteStatus := cmp.Or(governance.status, n.Status)
	// One resolved rail answers both the navigation and the article's own way
	// onward, so the step under the prose and the folder list beside it can
	// never disagree about what follows this note.
	sidebar := pages.NewSidebar(governance.shell.Nav, n.RelPath)
	view := pages.NoteView{
		Title:             n.Title,
		RelPath:           n.RelPath,
		Language:          n.Language,
		Type:              n.Type,
		Status:            noteStatus,
		SealTarget:        schema.SealStatus,
		Sealed:            governance.instance && noteStatus == schema.SealStatus,
		Diagnostic:        n.FMDiagnostic,
		Unsearchable:      !n.Searchable,
		Stale:             n.Stale,
		RenderDiagnostics: faults(result.Diagnostics, snap),
		CitedBy:           snap.CitedBy(rel),
		VaultHasLinks:     snap.AnyCitations(),
		Prev:              sidebar.FooterPrev,
		Next:              sidebar.FooterNext,
		StepsLabel:        sidebar.FooterLabel,
		StepsCourse:       sidebar.FooterCourse,
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		TitleAnchor:       result.TitleAnchor,
		Sidebar:           sidebar,
		Governed:          governance.shell.Governed,
		NonInstance:       governance.nonInstance,
		WriteDiagnostic:   governance.writeDiagnostic,
		Concepts:          concepts,
		Transitions:       governance.transitions,
		WithheldByOwner:   governance.withheldByOwner,
		NoFrontmatter:     governance.noFrontmatter,
	}

	// The redirect after a seal carries a one-shot signal; only a sealed note
	// plays it.
	view.JustSealed = view.Sealed && r.URL.Query().Get("sealed") == "1"

	pageChrome := governance.shell.Chrome(r, n.Title)
	// The furigana control switches readings off. A page with none has nothing
	// to switch, so it does not carry the button — which is most pages in a
	// folder that holds no Japanese at all.
	pageChrome.HasRuby = strings.Contains(result.HTML, "<ruby")
	if err := pages.Note(view, pageChrome).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write note page", "path", rel, "error", err)
	}
}

// faults keeps the rendering diagnostics that describe something wrong with the
// note, dropping the unresolved links the vault is deliberately writing toward.
//
// Nearly every unresolved link in a vault written this way is one of those, so
// listing them all makes the count beside the page a number the reader learns
// to ignore, and the two genuine faults hide behind fifty-six deliberate ones.
// The links themselves are still marked where they sit in the prose, which is
// where a forward reference is worth seeing: the target is not written yet, and
// the reader is looking straight at the sentence that wants it.
func faults(diags []render.Diagnostic, snap *snapshot.View) []render.Diagnostic {
	kept := make([]render.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Kind == render.DiagWikilinkBroken && snap.TrackedForwardReference(d.Target) {
			continue
		}
		kept = append(kept, d)
	}
	return kept
}

type governanceState struct {
	shell pages.Shell
	// status is what the note's own file says, read for this request. It is
	// empty unless the write face applies to this note; the page falls back to
	// the scan's value, which is the only answer available when nothing may be
	// written and is then never contradicted by anything.
	status          string
	transitions     []string
	writeDiagnostic string
	instance        bool
	nonInstance     bool
	noFrontmatter   bool
	// withheldByOwner distinguishes an empty transition list whose onward
	// steps the contract defines for other owners from one where the schema
	// defines nothing onward; the page words the two differently.
	withheldByOwner bool
}

func (h *Handler) governance(
	n *snapshot.Note,
	snap *snapshot.View,
	statusView status.View,
	policy schema.ArtifactPolicy,
) governanceState {
	pageShell := shell.Project(statusView, policy, snap)
	// Two authority samples taken at different instants: the request's captured
	// write view, and the snapshot's own artifact capture. A note counts as a
	// governed instance only while both still answer, whichever was taken first.
	authorityAvailable := !statusView.Closed() && policy.Available()
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
			state.status, state.writeDiagnostic = h.observedStatus(n.RelPath)
			if state.writeDiagnostic == "" {
				state.transitions = statusView.Transitions(n.RelPath, n.Type, state.status)
				if len(state.transitions) == 0 {
					state.withheldByOwner = statusView.WithheldByOwner(n.Type, state.status)
				}
			}
		}
	}
	return state
}

// observedStatus asks the write package what the note's status line says right
// now, and turns a failure into a closed write face rather than a guess.
//
// Falling back to the scan's value would be the wrong repair: the value it
// holds may be exactly the one the reader has already moved away from, and a
// transition offered from it is refused on arrival. Whatever prevented this
// read is the same thing that would prevent the write.
func (h *Handler) observedStatus(rel string) (current, blocked string) {
	current, err := h.deps.ObservedStatus(rel)
	if err != nil {
		h.deps.Log.Warn("read the note's own status for the reading page", "path", rel, "error", err)
		return "", status.NoteUnreadableDiagnostic
	}
	return current, ""
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
	docs := make([]lesson.ConceptDoc, 0, len(refs))
	for ordinal, rel := range refs {
		// Each sheet is cloned into the lesson's own document when the reader
		// opens it, so its ids share one space with the lesson body's. The
		// region is the sheet's place in the order this page cites its
		// concepts — the same for every reader of the page, and derived from
		// the page rather than from anything the process is counting.
		region := "c" + strconv.Itoa(ordinal+1) + "-"
		renderBody := func(rel, body string) string { return snap.RenderIn(region, rel, body).HTML }
		if d, ok := snap.Concepts().Document(renderBody, rel); ok {
			docs = append(docs, d)
		}
	}
	return docs
}

// lifecycle assembles Home's Lifecycle block: the note group's statuses in the
// contract's toml order, each with its live snapshot count and whether it is
// current. A closed result means the block was withheld — the vault declared a
// vocabulary yomihon could not read — never that the vault has no statuses.
// A vault that declares none legitimately yields an open, empty block.
func (h *Handler) lifecycle(
	statusView status.View,
	snap *snapshot.View,
	current string,
) (items []pages.LifecycleItem, closed bool) {
	if !statusView.Governed() {
		return nil, false
	}
	if statusView.Closed() {
		return nil, true
	}
	counts, err := snap.Search().CountByTypeStatus()
	if err != nil {
		return nil, true
	}
	// One entry per status that has notes the operator can actually move on
	// from. The vocabulary is not the subject: a status nothing sits at, and a
	// status whose only onward edge is retirement, say nothing about what is
	// waiting.
	waiting := make(map[string]int, len(counts))
	for ts, n := range counts {
		if statusView.Advanceable(ts.Type, ts.Status) {
			waiting[ts.Status] += n
		}
	}
	items = make([]pages.LifecycleItem, 0, len(waiting))
	add := func(s string) {
		items = append(items, pages.LifecycleItem{
			Name:   s,
			Count:  waiting[s],
			Active: s == current,
			Sealed: s == schema.SealStatus,
		})
	}
	for _, s := range statusView.Order() {
		if waiting[s] == 0 {
			continue
		}
		add(s)
		delete(waiting, s)
	}
	// Whatever is still here sits at a value the default vocabulary does not
	// list: the contract routes that kind of note to another group of statuses,
	// or the note carries a value no group declares at all. The total above
	// counted those notes, so leaving them out is exactly how a number stops
	// agreeing with its own breakdown. Nothing declares an order across groups,
	// so they follow in a stable one.
	for _, s := range slices.Sorted(maps.Keys(waiting)) {
		add(s)
	}
	return items, false
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
func (c homeContent) subtitle() string {
	parts := make([]string, 0, 3)
	if c.recent {
		parts = append(parts, "最近變更")
	}
	if c.lifecycle {
		parts = append(parts, "待判讀內容")
	}
	if c.paths {
		parts = append(parts, "接下來的學習路徑")
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return "查看" + parts[0] + "。"
	case 2:
		return "查看" + parts[0] + "與" + parts[1] + "。"
	default:
		return "查看" + parts[0] + "、" + parts[1] + "，以及" + parts[2] + "。"
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
func homeStandIn(snap *snapshot.View, content homeContent) pages.HomeStandIn {
	if content.recent || content.lifecycle || content.paths || content.withheld {
		return pages.HomeStandIn{}
	}
	files := snap.Files()
	standIn := pages.HomeStandIn{Shown: true, Files: len(files)}
	var newest vault.Entry
	for _, entry := range files {
		if newest.Path() == "" || entry.ModTime().After(newest.ModTime()) {
			newest = entry
		}
	}
	if newest.Path() == "" {
		return standIn
	}
	standIn.NewestPath = newest.Path()
	standIn.NewestName = path.Base(newest.Path())
	standIn.NewestDate = newest.ModTime().Format("2006-01-02")
	standIn.NewestAt = newest.ModTime().Format(time.RFC3339)
	return standIn
}

// degradedNotice states, in the reader's language, that the snapshot behind
// the page could not read everything, so the content may be incomplete or held
// at an older generation. Empty when the snapshot is whole and current, which
// is the ordinary case and renders nothing.
func degradedNotice(fresh *snapshot.Freshness) string {
	if len(fresh.Blocked) == 0 {
		return ""
	}
	return "有 " + strconv.Itoa(len(fresh.Blocked)) + " 個檔案讀不進來，頁面內容可能不完整，或停在較舊的版本。詳細狀況見整體狀況頁。"
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
// read closes the lifecycle, the recent list and the study paths alike — and
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
func recentHomeNotes(notes []nav.NoteSummary, governed bool) (recent []pages.HomeNote, ordered bool) {
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
	ordered = len(notes) > 1 && !notes[0].Modified.Equal(notes[len(notes)-1].Modified)

	out := make([]pages.HomeNote, 0, len(notes))
	for _, n := range notes {
		item := pages.HomeNote{Title: n.Title, RelPath: n.RelPath, Type: n.Type}
		// A status chip names a value from a declared vocabulary. Without a
		// contract there is no vocabulary, so raw frontmatter text is not
		// dressed up as a lifecycle state.
		if governed {
			item.Status = n.Status
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
		})
	}
	return out
}
