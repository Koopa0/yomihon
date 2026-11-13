// Package note owns the general reading surface — every route a reader
// reaches that is not one of the dedicated faces. Register mounts Home, one
// rendered note, an external-editor thought stub, two notes read side by side, one folder, the
// maps mode page, the folders mode page, the whole-vault health page, a vault
// file's raw bytes, the freshness poll a page keeps open on the note it is
// showing, the excerpt a hover card shows of the note under the reader's
// pointer, and the catch-all that
// answers a path the vault has nothing at. The last is deliberately last: it
// gives an unknown GET path a page with a way onward.
// LocalizedRouter handles the mux's separate method refusal.
//
// A vault file with no dedicated reader is shown here too, as an honest
// stand-in page rather than as something this package pretends to render.
// Nothing here writes into the vault.
package note

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// Sources names the authorities one reading request draws on, and the log the
// routes report an operational fault to. Snapshot is one closure because a
// request must read the atomic pointer once and derive its navigation and
// counts from that coherent value. Authority captures one immutable contract
// view for the request; source changes affect the next request.
type Sources struct {
	// Uncertainties reads the separate location-mark file. Nil means this
	// process has no mark storage; a read error remains visible on the desk.
	Uncertainties func() ([]mark.Uncertainty, error)
	Source        *vault.Reader
	// Contract supplies the field vocabulary a thought stub is written from;
	// the note type it is for comes from the snapshot's roles.
	Contract *schema.Contract
	// VaultName is the opened folder's startup name, fixed even when the
	// selecting pathname changes. The scanner reports divergence separately.
	VaultName string
	Authority func() schema.Authority
	Snapshot  func() *snapshot.Generation
	// Continuation is a closure over the place the reader deliberately left
	// off at, answering false when they have left none. The desk reads it once
	// per request and resolves what it names inside the same generation the
	// rest of that page was built from, so the row's sentence about the note
	// having changed is decided against the version the page is showing.
	Continuation func() (mark.Continuation, bool)
	// MarkAddress is where a reading page posts a place to, and empty where
	// this process keeps none — no configuration directory to hold the file.
	// The page renders no control without it, so a reader is never invited to
	// keep something that has nowhere to go.
	MarkAddress        string
	UncertaintyAddress string
	// FigureApproved reports whether the owner approved a figure's exact
	// bytes to run their inline scripts. Nil means this process keeps no
	// approvals, and every figure is served without script.
	FigureApproved func(rel string, sum [sha256.Size]byte) (render.Approval, error)
	Log            *slog.Logger
}

// Handler serves reading pages from one rooted vault capability and its
// coherently published snapshots.
type Handler struct {
	sources Sources
	// freshnessFailures is the only state this handler keeps between
	// requests: what it last said about a note it could not read, so a page
	// polling every few seconds does not repeat one fault into the log.
	freshnessFailures freshnessLog
}

// New wires the reading feature. It defensively copies the startup-owned
// record so later field reassignment by the caller cannot rewire a live
// handler. Every required reference and function must be non-nil: a wiring
// bug fails here, not on the first request three calls deep inside show().
func New(d *Sources) *Handler {
	if d == nil {
		panic("note: New requires a non-nil Sources")
	}
	if d.Source == nil {
		panic("note: New requires a non-nil Source")
	}
	if d.Authority == nil {
		panic("note: New requires a non-nil Authority")
	}
	if d.Snapshot == nil {
		panic("note: New requires a non-nil Snapshot provider")
	}
	if d.Continuation == nil {
		panic("note: New requires a non-nil Continuation provider")
	}
	if d.Log == nil {
		panic("note: New requires a non-nil Log")
	}
	return &Handler{sources: *d}
}

// Register mounts the feature's routes. The bare "GET /" is the last pattern
// a GET can match, so an unknown path keeps the reading shell. Method
// refusals are answered separately by LocalizedRouter.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /thought/{path...}", h.thought)
	mux.HandleFunc("GET "+openThoughtsAddress, h.openThoughts)
	mux.HandleFunc("GET /compare/{path...}", h.compare)
	mux.HandleFunc("GET /raw/{path...}", h.raw)
	mux.HandleFunc("GET /embed/{path...}", h.figureDocument)
	mux.HandleFunc("GET /freshness/{path...}", h.freshness)
	mux.HandleFunc("GET /preview/{path...}", h.preview)
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /maps", h.maps)
	mux.HandleFunc("GET /journal", h.journal)
	// The mode index sits beside the subtree pattern that serves one folder.
	// Without a pattern of its own the bare address redirects into that
	// subtree, where an empty path is not a folder, and the desk's own link
	// answers with the not-found page.
	mux.HandleFunc("GET /folders", h.folders)
	mux.HandleFunc("GET /folders/{path...}", h.folder)
	mux.HandleFunc("GET /", h.notFound)
}

// notFound answers a path the vault has nothing at. It is a page rather than a
// line of text because the reader is mid-navigation and needs the way onward
// they were already using: the folder tree, the search, and home.
func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	authority := h.sources.Authority()
	snap := h.sources.Snapshot().Capture()
	h.showNotFound(w, r, r.URL.Path, authority, snap)
}

// showNotFound renders the not-found page with the status code that belongs to
// it. The path is echoed so the reader can see their own typo; it reaches the
// page as text and is escaped there like any other note content. The captured
// pair is the same generation that decided the path is missing, so the sidebar
// cannot describe a vault that already holds it.
func (h *Handler) showNotFound(
	w http.ResponseWriter,
	r *http.Request,
	asked string,
	authority schema.Authority,
	snap *snapshot.Generation,
) {
	h.showMissing(w, r, asked, false, snapshot.BlockedSource{}, authority, snap)
}

// showUnreadable answers a note the generation captured but could not read:
// the file exists on disk, so the plain not-found page — whose repair is a
// typo or an unwritten note — would send the reader the wrong way.
func (h *Handler) showUnreadable(
	w http.ResponseWriter,
	r *http.Request,
	asked, rel string,
	authority schema.Authority,
	snap *snapshot.Generation,
) {
	h.showMissing(w, r, asked, true, blockedSource(snap, rel), authority, snap)
}

// blockedSource names the captured path's failure without treating the words
// an error wrote as its type. A retained reading can carry the same failure.
func blockedSource(snap *snapshot.Generation, rel string) snapshot.BlockedSource {
	for _, source := range snap.Freshness().Blocked {
		if source.Path == rel {
			return source
		}
	}
	return snapshot.BlockedSource{}
}

// blockedAt finds a captured failure at the path or any folder above it.
// A folder covers whole path segments, never a similarly named sibling.
func blockedAt(snap *snapshot.Generation, rel string) snapshot.BlockedSource {
	for _, source := range snap.Freshness().Blocked {
		if source.Path == rel || strings.HasPrefix(rel, source.Path+"/") {
			return source
		}
	}
	return snapshot.BlockedSource{}
}

func (h *Handler) showMissing(
	w http.ResponseWriter,
	r *http.Request,
	asked string,
	unreadable bool,
	cause snapshot.BlockedSource,
	authority schema.Authority,
	snap *snapshot.Generation,
) {
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	view := pages.NotFoundView{
		Asked:      asked,
		Unreadable: unreadable,
		ParsePanic: cause.ParsePanic,
		Reason:     cause.Reason,
		Sidebar:    pages.NewSidebar(pageShell, ""),
	}
	lang := origin.Language(r)
	title := wording.NotFoundKicker.In(lang)
	if unreadable {
		title = wording.NotReadableKicker.In(lang)
		if cause.ParsePanic {
			title = wording.ParseFailedKicker.In(lang)
		}
	}
	if err := pages.WriteNotFound(r.Context(), w, view, layouts.ChromeFromRequest(r, title)); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write not-found page", "path", asked, "error", err)
	}
}

// folder shows one folder whole. The rail reaches any note one disclosure at a
// time, which is not the same as seeing a term's lessons in order or a month of
// entries at once — and the breadcrumb, which reads as a trail out of where you
// are, had nowhere to lead until now.
func (h *Handler) folder(w http.ResponseWriter, r *http.Request) {
	dir := path.Clean(strings.Trim(r.PathValue("path"), "/"))
	authority := h.sources.Authority()
	snap := h.sources.Snapshot().Capture()
	if dir == "." || dir == ".." || strings.HasPrefix(dir, "../") {
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}
	dir = vault.NormalizeNFC(dir)
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	notes, subfolders, ok := pageShell.Nav.Directory(dir)
	if !ok {
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}
	name := nav.Label(dir)
	view := pages.NewFolderLevel(dir, name, notes, subfolders, origin.Language(r), articleLanguageLookup(snap))
	if err := pages.Folder(view, layouts.ChromeFromRequest(r, name)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write folder page", "path", dir, "error", err)
	}
}

// show serves one entry of the browse tree. Every file the tree lists opens
// here; the presentation follows the kind. Markdown is the note page — the
// reading surface with its table of contents and diagnostics.
// Everything else is a read-only view of a file, built by showFile.
//
// A note's body reaches this first-party page through the markdown pipeline's
// inert authored-markup subset. Ruby reading aids survive; executable,
// navigating, and automatically loading tags are visible text, never browser
// authority. Other file kinds are likewise shown as escaped source or handed
// to the browser through the sandboxed raw endpoint, never poured into this
// page as live markup.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	authority := h.sources.Authority()
	snap := h.sources.Snapshot().Capture()
	if !servable(rel) {
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}
	if !vault.IsMarkdown(rel) || snap.SkipsNote(rel) {
		h.showFile(w, r, rel, authority, snap)
		return
	}
	// A note over the source bound is skipped at scan, the same ceiling a
	// non-note already had. The file page names the size and offers the
	// bytes; the unreadable page would tell the reader to clear a permission
	// and wait for a reload that never shows it.
	if entry, isFile := snap.Entry(rel); isFile && entry.Size() > render.MaxSourceBytes {
		h.showFile(w, r, rel, authority, snap)
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
			h.sources.Log.Warn("note captured in scan but unreadable in this generation", "path", rel)
			h.showUnreadable(w, r, r.URL.Path, rel, authority, snap)
			return
		}
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}

	view, conceptRefs := h.reading(r, rel, &n, snap, authority, lang, "")
	view.Concepts = loadConcepts(snap, conceptRefs, lang)

	pageChrome := layouts.ChromeFromRequest(r, n.Title)
	if err := pages.Note(view, pageChrome).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write note page", "path", rel, "error", err)
	}
}

// reading is one note as a page shows it. The note's own page assembles one of
// these and the page holding two notes assembles two, so a column beside
// another note can never become a second idea of what a note looks like.
//
// idPrefix is the id space the article will occupy: empty where the page shows
// one note and every place inside it is the note's own, and a column's own name
// where two notes share a document. Every place the rendered body carries and
// every address reaching one is renamed under it, so a contents entry, a
// footnote marker, or an address an author wrote at one of their own sections
// stays inside the column it was written in.
//
// The concept references the body turned out to hold come back rather than the
// sheets themselves. One page assembles one set of sheets: two columns citing
// one concept share it instead of putting the same document into the page twice
// under one name.
func (h *Handler) reading(
	r *http.Request,
	rel string,
	n *snapshot.Reading,
	snap *snapshot.Generation,
	authority schema.Authority,
	lang wording.Lang,
	idPrefix string,
) (view pages.NoteView, conceptRefs []string) {
	state := h.governance(n, snap, authority)
	// render.Pipeline.HTML never fails the whole render: a content-level
	// problem becomes a Diagnostic, not an error — no error path left to handle.
	// On the compare page, which is neither note's own, a figure is its
	// stand-in; an approval runs a figure only where its note is read alone.
	var result render.Result
	if idPrefix == "" {
		result = snap.Render(rel, n.Body, lang)
	} else {
		result = snap.RenderExcerptIn("", rel, n.Body, lang)
	}
	declaredSources, sourceDiagnostics := snap.DeclaredSources(rel, lang)
	result.Diagnostics = append(result.Diagnostics, sourceDiagnostics...)

	// Governed lesson bodies get the read-aloud affordance: wrap each ruby-bearing
	// sentence with a speak button whose text has the furigana stripped
	// server-side (render.InjectTTS). Both governed-instance classification and
	// the lesson type are required here before any lesson affordance is added. A
	// governed lesson with a slot sidecar also gets its sentence-pattern machine,
	// and its concept wikilinks become in-app sheet triggers. This happens only
	// after the request's captured authority has classified the note, so every
	// projection in this response uses one coherent lifecycle view.
	// The lesson type is the one type this page enriches with lesson-body
	// interactions. What the contract layer supplies is the spelling, not yet a
	// vault's own choice of word: it is still one fixed name, so a folder
	// calling its course members something else gets a plain page. Asking for it
	// is what lets that stop being true in one place instead of four.
	// render.HTML stays generic, so the decision is made here.
	if state.instance() && authority.IsLessonType(n.Type) {
		result.HTML = render.InjectTTS(result.HTML, lang)
		pageChrome := layouts.ChromeFromRequest(r, n.Title)
		result.HTML = h.injectSlotMachine(r.Context(), snap.Slots(), rel, n.Slug, result.HTML, pageChrome.Nonce, lang)
		result.HTML, conceptRefs = render.InjectConceptTriggers(result.HTML, snap.Concepts().IDForPath)
	}
	if n.LanguageDiagnostic != "" {
		h.sources.Log.Warn("invalid article language; the article carries no language of its own", "path", rel, "error", n.LanguageDiagnostic)
	}
	// Last, so what a lesson's own controls were spliced in carrying is renamed
	// with everything the render itself wrote.
	render.Qualify(idPrefix, &result)
	result.HTML = annotateMarkAnchors(result.HTML)

	// One resolved rail answers both the navigation and the article's own way
	// onward, so the step under the prose and the folder list beside it can
	// never disagree about what follows this note.
	readingRail := pages.NewReadingRail(state.shell, n.RelPath, n.Domain)
	// The one place the reader kept, if any. Both the book beside this note and
	// the control for keeping a place read it from here, so the two cannot
	// disagree about where it is.
	kept, marked := h.sources.Continuation()
	if marked {
		readingRail.KeptNote = kept.RelPath
	}
	foot := pages.FooterSequence(&readingRail, lang)
	updatedDisplay, updatedMachine, updatedFromFile := metarowDate(snap, rel)
	contract := h.enumContract(authority)
	cause := blockedSource(snap, rel)
	view = pages.NoteView{
		Title:              n.Title,
		RelPath:            n.RelPath,
		Language:           n.Language,
		Type:               n.Type,
		Draft:              n.Draft,
		Updated:            updatedDisplay,
		UpdatedAt:          updatedMachine,
		UpdatedFromFile:    updatedFromFile,
		ObsidianHref:       obsidianOpenHref(h.sources.Source, n.RelPath),
		Diagnostic:         n.FMDiagnostic,
		Stale:              n.Stale,
		ParsePanic:         cause.ParsePanic,
		ParseReason:        cause.Reason,
		RenderDiagnostics:  faults(result.Diagnostics, snap),
		CitedBy:            snap.CitedBy(rel),
		BasedOn:            declaredSources,
		DeclaredBy:         declaredBy(snap, rel, &result, idPrefix, lang),
		Pair:               pairOffer(snap, state.shell.Nav, n),
		VaultHasLinks:      snap.AnyCitations(),
		Prev:               foot.Prev,
		Next:               foot.Next,
		StepsLabel:         foot.Label,
		StepsCourse:        foot.Course,
		StepsUnit:          foot.Unit,
		StepsPath:          foot.Path,
		StepsBack:          foot.Back,
		StepsOnward:        foot.Onward,
		StepsToContents:    foot.ToContents,
		StepsAsides:        foot.Asides,
		TOC:                result.Outline(),
		BodyHTML:           result.HTML,
		TitleAnchor:        result.TitleAnchor,
		ReadingRail:        readingRail,
		IDPrefix:           idPrefix,
		ContentIdentity:    hex.EncodeToString(n.ContentIdentity[:]),
		MarkAddress:        h.sources.MarkAddress,
		PlaceKept:          marked && kept.RelPath == n.RelPath,
		UncertaintyAddress: h.sources.UncertaintyAddress,
		ThoughtDoor:        snap.NavigationRoles().AnswerType() != "" && thoughtSourceAvailable(snap, rel),
		// The identity above covers the note's own bytes; what the render
		// pulled in from other notes is bound by its own stamp, so an edit to
		// an embedded source can reach this page while it is open.
		TranscludedIdentity: result.TranscludedIdentity,
		SchemaNotices:       schemaNotices(snap.SchemaFindings(rel), n, lang, contract),
		// Said whatever the vault governs: it is a fact about the bytes, and
		// it is the page's own to state, not a verdict borrowed from a contract.
		FrontmatterUnclosed: n.FrontmatterUnclosed,
	}
	return view, conceptRefs
}

// metarowDate picks the one date the reading page shows and the strings the
// time element carries: the file's recorded change time, kept with its clock in
// the machine-readable value because a clock is what was observed. A note
// writes no update date of its own, so fromFile is always the answer and the
// label names that claim.
//
// Every note this page serves was observed by the generation's scan, so the
// date is expected to answer; should the scan hold no identity for the path
// anyway, the page goes dateless rather than dated by invention.
func metarowDate(snap *snapshot.Generation, rel string) (display, machine string, fromFile bool) {
	entry, ok := snap.Entry(rel)
	if !ok || entry.ModTime().IsZero() {
		return "", "", false
	}
	mod := entry.ModTime()
	return mod.Format(time.DateOnly), mod.Format(time.RFC3339), true
}

// schemaNotices turns what the schema said about a note into sentences a
// reader can read. The findings themselves are one program's verdict in one
// language for another program to parse; what a page owes a reader is the same
// verdict in their own words, with the note's own text kept intact inside it.
func schemaNotices(findings []judge.Finding, reading *snapshot.Reading, lang wording.Lang, contract *schema.Contract) [][]wording.SchemaPart {
	if len(findings) == 0 {
		return nil
	}
	notices := make([][]wording.SchemaPart, 0, len(findings))
	for i := range findings {
		f := &findings[i]
		// An unclosed fence already has its own notice, so the same finding is
		// not repeated in the schema notice list.
		if reading.FrontmatterUnclosed && f.RuleID == "schema.frontmatter" {
			continue
		}
		target := deref(f.Target)
		if f.RuleID == "schema.frontmatter" {
			target = reading.FMDiagnostic
		}
		sentence := wording.SchemaSentence(lang, string(f.RuleID), deref(f.Field), target)
		if f.RuleID == "schema.enum" {
			sentence = append(sentence, wording.AllowedEnumValues(lang, deref(f.Field), enumValues(contract, deref(f.Field)))...)
		}
		notices = append(notices, sentence)
	}
	return notices
}

// deref reads an optional finding field, which is absent for every rule that
// does not name one.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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
func faults(diags []render.Diagnostic, snap *snapshot.Generation) []render.Diagnostic {
	kept := make([]render.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Kind == render.DiagWikilinkBroken && snap.TrackedForwardReference(d.Target) {
			continue
		}
		kept = append(kept, d)
	}
	return kept
}

// governance is where the request's two authorities put one note: the folder
// holds it as a note instance, the folder holds it as a readable artifact, or
// neither authority can be asked. The three answers are exclusive and
// exhaustive, which is what a pair of booleans could not state — booleans also
// admit both-true, and read on an unanswerable request as "not an artifact,
// therefore an instance".
type governance uint8

const (
	// governanceUnavailable is a note nothing can be said about, because one
	// of the two authorities has closed. It is the zero value because it is
	// the answer that asserts least.
	governanceUnavailable governance = iota
	// governedInstance is a note the folder holds as one of its notes: for a
	// lesson, the page offers its lesson affordances.
	governedInstance
	// readableArtifact is a note the folder holds outside its instances, such
	// as a template — readable, never adjudicated.
	readableArtifact
)

// classifyGovernance places one note against two authority samples taken at
// different instants: the request's captured contract view, and the
// snapshot's own artifact capture. A note is placed only while both still
// answer, whichever was taken first.
func classifyGovernance(authority schema.Authority, policy schema.ArtifactPolicy, relPath string) governance {
	if authority.Closed() || !policy.Available() {
		return governanceUnavailable
	}
	if policy.IsNonInstance(relPath) {
		return readableArtifact
	}
	return governedInstance
}

type governanceState struct {
	shell nav.Shell
	// placement is where the request's authorities put this note.
	placement governance
}

// instance reports a note the folder holds as one of its notes rather than as
// a readable artifact, which is what decides whether the page dresses a
// lesson.
func (s *governanceState) instance() bool {
	return s.placement == governedInstance
}

func (h *Handler) governance(
	n *snapshot.Reading,
	snap *snapshot.Generation,
	authority schema.Authority,
) governanceState {
	return governanceState{
		shell:     shell.Project(h.sources.VaultName, authority, snap),
		placement: classifyGovernance(authority, snap.ArtifactPolicy(), n.RelPath),
	}
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
	lang wording.Lang,
) string {
	sc, ok := slots.Lookup(slug)
	if !ok {
		return body
	}
	var buf bytes.Buffer
	if err := pages.SlotMachine(sc, nonce, lang).Render(ctx, &buf); err != nil {
		h.sources.Log.Error("render slot machine", "path", rel, "slug", slug, "error", err)
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
func loadConcepts(
	snap *snapshot.Generation,
	refs []string,
	lang wording.Lang,
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
		// The sheet is the concept note's words inside the lesson's page, so a
		// figure in it is its stand-in.
		renderBody := func(rel, body string) string { return snap.RenderExcerptIn(region, rel, body, lang).HTML }
		if d, ok := snap.Concepts().Document(renderBody, rel); ok {
			docs = append(docs, d)
		}
	}
	return docs
}
