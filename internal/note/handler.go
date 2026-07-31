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
	"path"
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

// homeReadmePath is the note Home shows as its own introduction. It is named
// once because the lookup and the render need the same answer: the renderer
// resolves a relative image against the note's directory, so a body fetched
// under one path and rendered under another would address the wrong files.
const homeReadmePath = "README.md"

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
	// WriteBlock is a closure over the write package's read-only working-tree
	// query. It answers why a transition on this note would be refused, so the
	// reading page states it beside the controls rather than after a press.
	WriteBlock func(ctx context.Context, rel string) (string, error)
	// ObservedStatus is a closure over the write package's read of the note's
	// own status line. The rest of the page comes from a scan that lags the
	// folder by a couple of seconds, which a body and a link graph can afford
	// and an adjudication state cannot: the reader arrives here straight from a
	// write, and a status that lags is one they have already changed.
	ObservedStatus func(ctx context.Context, rel string) (status.Observed, error)
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
	if d.Provenance == nil {
		panic("note: New requires a non-nil Provenance provider")
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
	snap := h.deps.Snapshot().Capture()
	pageShell := shell.Project(h.deps.Status(), snap.ArtifactPolicy(), snap)
	view := pages.NotFoundView{
		Asked:   asked,
		Sidebar: pages.NewSidebar(pageShell.Nav, ""),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := pages.NotFound(view, pageShell.Chrome(r, "找不到")).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write not-found page", "path", asked, "error", err)
	}
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
	if !recentClosed {
		recent = recentHomeNotes(visibleNav.KnowledgeNotes(), pageShell.Governed)
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
		Recent:          recent,
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
		h.deps.Log.Warn("note is absent from the request snapshot", "path", rel)
		h.showNotFound(w, r, r.URL.Path)
		return
	}

	artifactPolicy := snap.ArtifactPolicy()
	governance := h.governance(r.Context(), &n, snap, statusView, artifactPolicy)
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
		RenderDiagnostics: faults(result.Diagnostics, snap),
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		Sidebar:           pages.NewSidebar(governance.shell.Nav, n.RelPath),
		Governed:          governance.shell.Governed,
		NonInstance:       governance.nonInstance,
		WriteDiagnostic:   governance.writeDiagnostic,
		Concepts:          concepts,
		Transitions:       governance.transitions,
		NoFrontmatter:     governance.noFrontmatter,
	}

	h.addWriteBlock(r.Context(), rel, &view)
	// The receipt is checked against the bytes the status face just read, not
	// against the scan's capture. A write lands and redirects here inside the
	// scan interval, so the captured bytes are the ones from before it — and a
	// note whose seal is the only thing anyone will look at showed no commit at
	// all on the one page load that mattered.
	sealedAgainst := cmp.Or(governance.contentHash, n.ContentHash)
	h.addSealProvenance(r.Context(), rel, sealedAgainst, r.URL.Query().Get("sealed") == "1", &view)

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
	status string
	// contentHash is of the same bytes status came from. The seal receipt is
	// checked against it, so the commit shown belongs to the file the page is
	// describing rather than to whatever the scan last captured.
	contentHash     [sha256.Size]byte
	transitions     []string
	writeDiagnostic string
	instance        bool
	nonInstance     bool
	noFrontmatter   bool
}

func (h *Handler) governance(
	ctx context.Context,
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
			observed, blocked := h.observedStatus(ctx, n.RelPath)
			state.status, state.contentHash, state.writeDiagnostic = observed.Status, observed.ContentHash, blocked
			if state.writeDiagnostic == "" {
				state.transitions = statusView.Transitions(n.RelPath, n.Type, state.status)
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
func (h *Handler) observedStatus(ctx context.Context, rel string) (observed status.Observed, blocked string) {
	observed, err := h.deps.ObservedStatus(ctx, rel)
	if err != nil {
		h.deps.Log.Warn("read the note's own status for the reading page", "path", rel, "error", err)
		return status.Observed{}, status.ReadBlockReason
	}
	return observed, ""
}

// addWriteBlock asks the write package whether a transition on this note would
// be refused, so the page states it beside the controls instead of leaving the
// operator to discover it by pressing. It runs only when transitions are on
// offer.
//
// The two refusals differ in kind and are presented differently. An uncommitted
// edit is something the reader can clear and retry, so the controls stay and the
// reason sits beside them. A working tree that cannot be read at all — most often
// a folder that is not a git repository — cannot be cleared from inside yomihon,
// and every transition there is recorded as a commit that can never be made, so
// the whole write face closes and says why. Offering a control that can only fail
// is worse than offering none.
func (h *Handler) addWriteBlock(ctx context.Context, rel string, view *pages.NoteView) {
	if len(view.Transitions) == 0 || h.deps.WriteBlock == nil {
		return
	}
	reason, err := h.deps.WriteBlock(ctx, rel)
	if err != nil {
		h.deps.Log.Warn("write block check", "path", rel, "error", err)
		if reason == "" {
			reason = status.GitBlockReason
		}
		view.WriteDiagnostic = reason
		view.Transitions = nil
		return
	}
	view.WriteBlock = reason
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
	renderBody := func(rel, body string) string { return snap.Render(rel, body).HTML }
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
	// from, counted the same way the header counts them — so this block is that
	// number broken down rather than a second, differently-derived list. The
	// vocabulary is not the subject: a status nothing sits at, and a status
	// whose only onward edge is retirement, say nothing about what is waiting.
	waiting := make(map[string]int, len(counts))
	for ts, n := range counts {
		if statusView.Advanceable(ts.Type, ts.Status) {
			waiting[ts.Status] += n
		}
	}
	items = make([]pages.LifecycleItem, 0, len(waiting))
	for _, s := range statusView.Order() {
		if waiting[s] == 0 {
			continue
		}
		items = append(items, pages.LifecycleItem{
			Name:   s,
			Count:  waiting[s],
			Active: s == current,
			Sealed: s == schema.SealStatus,
		})
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
func recentHomeNotes(notes []nav.NoteSummary, governed bool) []pages.HomeNote {
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
	return out
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
func homePaths(paths []nav.Map) []pages.HomePath {
	out := make([]pages.HomePath, 0, len(paths))
	for i := range paths {
		studyPath := &paths[i]
		_, total := studyPath.EntryCounts(schema.SealStatus)
		out = append(out, pages.HomePath{
			Title:   studyPath.Title,
			RelPath: studyPath.RelPath,
			Total:   total,
		})
	}
	return out
}
