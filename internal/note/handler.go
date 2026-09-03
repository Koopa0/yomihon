// Package note owns the general reading surface — every route a reader
// reaches that is not one of the dedicated faces. Register mounts nine of
// them: Home, one rendered note, one folder, the whole-vault health page, a
// vault file's raw bytes, the freshness poll a page keeps open on the note it
// is showing, the excerpt a hover card shows of the note under the reader's
// pointer, the language switch every page's footer posts to, and the
// catch-all that answers a path the vault has nothing at. The last is
// deliberately last: it exists so no request reaches the router's own
// fallback, which answers in English and offers nowhere to go.
//
// A vault file with no dedicated reader is shown here too, as an honest
// stand-in page rather than as something this package pretends to render.
// Status mutation remains in internal/status.
package note

import (
	"bytes"
	"cmp"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

// typeLesson is the one note type the reading page enriches with lesson-body
// interactions (the TTS speak buttons today; the slot machine and concept
// drawer next). It names in one place the single spot the handler treats a
// type specially, and it duplicates no enum: the type
// vocabulary lives in the schema contract's enums.type (vault-schema.toml),
// the single source of schema truth. render.HTML stays generic — the lesson decision is
// made here, so a non-lesson note never grows lesson affordances.
const typeLesson = "lesson"

// Sources names the authorities one reading request draws on, and the log the
// routes report an operational fault to. Snapshot is one closure because a
// request must read the atomic pointer once and derive its navigation and
// counts from that coherent value. Status captures one immutable lifecycle
// view for the request. Source changes affect the next request; a write still
// revalidates current authority under the lifecycle lock.
type Sources struct {
	Source   *vaultfs.Reader
	Status   func() status.Authority
	Snapshot func() *snapshot.Generation
	// ObservedStatus is a closure over the write package's read of the note's
	// own status line. The rest of the page comes from a scan that lags the
	// folder by a couple of seconds, which a body and a link graph can afford
	// and an adjudication state cannot: the reader arrives here straight from a
	// write, and a status that lags is one they have already changed.
	//
	// It takes the request's context because the read queues behind the write
	// face's own lock, which one flip can hold across two synchronizations.
	ObservedStatus func(ctx context.Context, rel string) (string, error)
	// ConsumeReceipt is a closure over the write face's attestation that it
	// recently flipped the note at rel out of status from, spending the
	// attestation when it answers true. The transition receipt renders only
	// on that answer, so the page's sentence about a change is backed by the
	// one component that performed it rather than by whatever a URL claims.
	ConsumeReceipt func(rel, from string) bool
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
// bug fails here, not on the first request three calls deep inside show(). A
// fail-closed write face still provides a closed status.Authority.
func New(d *Sources) *Handler {
	if d == nil {
		panic("note: New requires a non-nil Sources")
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
	if d.ConsumeReceipt == nil {
		panic("note: New requires a non-nil ConsumeReceipt provider")
	}
	if d.Log == nil {
		panic("note: New requires a non-nil Log")
	}
	return &Handler{sources: *d}
}

// Register mounts the feature's routes. The bare "GET /" is the last pattern
// any request can match, and it exists so that none of them reaches the
// router's own fallback, which answers in English and offers nowhere to go.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /notes/{path...}", h.show)
	mux.HandleFunc("GET /raw/{path...}", h.raw)
	mux.HandleFunc("GET /freshness/{path...}", h.freshness)
	mux.HandleFunc("GET /preview/{path...}", h.preview)
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /maps", h.maps)
	// The mode index sits beside the subtree pattern that serves one folder.
	// Without a pattern of its own the bare address redirects into that
	// subtree, where an empty path is not a folder, and the desk's own link
	// answers with the not-found page.
	mux.HandleFunc("GET /folders", h.folders)
	mux.HandleFunc("GET /folders/{path...}", h.folder)
	mux.HandleFunc("POST /lang", h.language)
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
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(h.sources.Status(), snap)
	view := pages.NotFoundView{
		Asked:      asked,
		Unreadable: unreadable,
		Sidebar:    pages.NewSidebar(pageShell.Nav, ""),
	}
	lang := origin.Language(r)
	title := wording.NotFoundKicker.In(lang)
	if unreadable {
		title = wording.NotReadableKicker.In(lang)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := pages.NotFound(view, layouts.ChromeFromRequest(r, title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write not-found page", "path", asked, "error", err)
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
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(h.sources.Status(), snap)
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
	if err := pages.Folder(view, layouts.ChromeFromRequest(r, view.Name)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write folder page", "path", dir, "error", err)
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
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !servable(rel) {
		h.showNotFound(w, r, r.URL.Path)
		return
	}
	authority := h.sources.Status()
	snap := h.sources.Snapshot().Capture()
	if !vault.IsMarkdown(rel) {
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
			h.showUnreadable(w, r, r.URL.Path)
			return
		}
		h.sources.Log.Warn("note is absent from the request snapshot", "path", rel)
		h.showNotFound(w, r, r.URL.Path)
		return
	}

	state := h.governance(r.Context(), &n, snap, authority, lang)
	// render.Pipeline.HTML never fails the whole render: a content-level
	// problem becomes a Diagnostic, not an error — no error path left to handle.
	result := snap.Render(rel, n.Body, lang)

	// Governed lesson bodies get the read-aloud affordance: wrap each ruby-bearing
	// sentence with a speak button whose text has the furigana stripped
	// server-side (render.InjectTTS). Both governed-instance classification and
	// the lesson type are required here before any lesson affordance is added. A
	// governed lesson with a slot sidecar also gets its sentence-pattern machine,
	// and its concept wikilinks become in-app sheet triggers. This happens only
	// after the request's captured authority has classified the note, so every
	// projection in this response uses one coherent lifecycle view.
	var concepts []lesson.ConceptDoc
	if state.instance() && n.Type == typeLesson {
		result.HTML = render.InjectTTS(result.HTML, lang)
		pageChrome := layouts.ChromeFromRequest(r, n.Title)
		result.HTML = h.injectSlotMachine(r.Context(), snap.Slots(), rel, n.Slug, result.HTML, pageChrome.Nonce, lang)
		var refs []string
		result.HTML, refs = render.InjectConceptTriggers(result.HTML, snap.Concepts().IDForPath)
		concepts = h.loadConcepts(snap, refs, lang)
	}
	if n.LanguageDiagnostic != "" {
		h.sources.Log.Warn("invalid article language; the article carries no language of its own", "path", rel, "error", n.LanguageDiagnostic)
	}

	// The status face and the status shown beside the title are the same
	// claim, so they come from the same read.
	noteStatus := cmp.Or(state.status, n.Status)
	// One resolved rail answers both the navigation and the article's own way
	// onward, so the step under the prose and the folder list beside it can
	// never disagree about what follows this note.
	sidebar := pages.NewSidebar(state.shell.Nav, n.RelPath)
	footPrev, footNext, footLabel, footCourse := pages.FooterSequence(state.shell.Nav, n.RelPath, lang)
	flippedFrom := vouchedOrigin(authority, h.sources.ConsumeReceipt, rel, n.Type,
		transition{from: r.URL.Query().Get("from"), to: noteStatus})
	updatedDisplay, updatedMachine, updatedFromFile := metarowDate(n.Updated, snap, rel)
	view := pages.NoteView{
		Title:             n.Title,
		RelPath:           n.RelPath,
		Language:          n.Language,
		Type:              n.Type,
		Status:            noteStatus,
		Updated:           updatedDisplay,
		UpdatedAt:         updatedMachine,
		UpdatedFromFile:   updatedFromFile,
		ObsidianHref:      pages.ObsidianHref(h.sources.Source.Name(), n.RelPath),
		Diagnostic:        n.FMDiagnostic,
		Unsearchable:      !n.Searchable,
		Stale:             n.Stale,
		RenderDiagnostics: noteFaults(result.Diagnostics, snap, n.RelPath, n.Title, lang),
		CitedBy:           snap.CitedBy(rel),
		VaultHasLinks:     snap.AnyCitations(),
		Prev:              footPrev,
		Next:              footNext,
		StepsLabel:        footLabel,
		StepsCourse:       footCourse,
		TOC:               result.TOC,
		BodyHTML:          result.HTML,
		TitleAnchor:       result.TitleAnchor,
		Sidebar:           sidebar,
		Governed:          state.shell.Governed,
		NonInstance:       state.nonInstance(),
		WriteDiagnostic:   state.writeDiagnostic,
		Concepts:          concepts,
		Transitions:       state.transitions,
		ContentIdentity:   hex.EncodeToString(n.ContentIdentity[:]),
		// The identity above covers the note's own bytes; what the render
		// pulled in from other notes is bound by its own stamp, so an edit to
		// an embedded source can reach this page while it is open.
		TranscludedIdentity: result.TranscludedIdentity,
		NoFrontmatter:       state.noFrontmatter,
		StatusUnknown:       state.statusUnknown,
		SchemaNotices:       schemaNotices(snap.SchemaFindings(rel), n.RelPath, lang),
		FlippedFrom:         flippedFrom,
		// The layer that withheld the transition set, when that is why it is
		// empty, so the page names it instead of the schema.
		OutsideKnowledgeScope: state.outsideLayer(),
		// The receipt for a change the face cannot walk back carries the
		// recovery sentence; a reversible one leaves undoing to the controls
		// already on the page.
		FlipNoReturn: flippedFrom != "" && !authority.CanReturn(n.Type, flippedFrom, noteStatus),
	}

	pageChrome := layouts.ChromeFromRequest(r, n.Title)
	// The furigana control switches readings off. A page with none has nothing
	// to switch, so it does not carry the button — which is most pages in a
	// folder that holds no Japanese at all.
	pageChrome.HasRuby = strings.Contains(result.HTML, "<ruby")
	if err := pages.Note(view, pageChrome).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write note page", "path", rel, "error", err)
	}
}

// metarowDate picks the one date the reading page shows and the strings the
// time element carries. The note's declared update wins when it is readable:
// it is the author's own claim, shown as the date it is. Otherwise the file's
// recorded change time answers, keeping its clock in the machine-readable
// value because a clock is what was observed — and fromFile then picks the
// label naming that different claim.
//
// Every note this page serves was observed by the generation's scan, so the
// fallback is expected to answer; should the scan hold no identity for the
// path anyway, the page goes dateless rather than dated by invention.
func metarowDate(updated time.Time, snap *snapshot.Generation, rel string) (display, machine string, fromFile bool) {
	if !updated.IsZero() {
		return updated.Format(time.DateOnly), updated.Format(time.DateOnly), false
	}
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
//
// The folder handed over is the one the domain rule actually compares — the
// first one under the configured root, not the folder the note sits in. For a
// note nested deeper the two differ, and naming the wrong one would be a fresh
// falsehood in a sentence written to end one.
func schemaNotices(findings []judge.Finding, relPath string, lang wording.Lang) [][]wording.SchemaPart {
	if len(findings) == 0 {
		return nil
	}
	folder := ""
	if seg := strings.Split(relPath, "/"); len(seg) >= 3 {
		folder = seg[1]
	}
	notices := make([][]wording.SchemaPart, 0, len(findings))
	for i := range findings {
		f := &findings[i]
		notices = append(notices, wording.SchemaSentence(lang, f.RuleID, deref(f.Field), deref(f.Target), folder))
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

// noteFaults is every fault this page states about the note: what the render
// met, filtered against what the vault knows, plus the one observation that
// needs the note's own name beside its title. It takes those two strings
// rather than the note, because those two are all it reads and a projection
// of a whole note is a wider coupling than this asks for.
func noteFaults(diags []render.Diagnostic, snap *snapshot.Generation, relPath, title string, lang wording.Lang) []render.Diagnostic {
	out := faults(diags, snap)
	if d, ok := titleTruncatedAtHash(relPath, title, lang); ok {
		out = append(out, d)
	}
	return out
}

// titleTruncatedAtHash reports that a note's title is exactly its filename cut
// where YAML starts a comment: at whitespace followed by "#".
//
// The condition mirrors YAML's own, which is why it cannot fire on a title
// that survived. A hash with no space before it starts no comment, so
// "trailing#nospace" arrives whole and is not this.
//
// It is an observation, not an accusation. A title written short on purpose,
// in quotes, produces the identical coincidence, and nothing in the parsed
// frontmatter separates the two; the sentence therefore reports what is
// visible and leaves the author to recognise their own case.
func titleTruncatedAtHash(relPath, title string, lang wording.Lang) (render.Diagnostic, bool) {
	stem := vault.NormalizeNFC(strings.TrimSuffix(path.Base(relPath), ".md"))
	written := vault.NormalizeNFC(title)
	if written == "" || written == stem || !strings.HasPrefix(stem, written) {
		return render.Diagnostic{}, false
	}
	rest := stem[len(written):]
	trimmed := strings.TrimLeft(rest, " \t")
	if len(trimmed) == len(rest) || !strings.HasPrefix(trimmed, "#") {
		return render.Diagnostic{}, false
	}
	return render.Diagnostic{
		Kind:    render.DiagTitleTruncatedAtHash,
		Target:  title,
		Message: fmt.Sprintf(wording.TitleTruncatedAtHashFmt.In(lang), title),
	}, true
}

// governance is where the request's two authorities put one note: the folder
// governs it, the folder declared a knowledge layer this note sits outside of,
// the folder holds it outside the lifecycle, or neither authority can be asked.
// The four answers are exclusive and exhaustive, which is what a set of
// booleans could not state — booleans also admit both-true, and read on an
// unanswerable request as "not an artifact, therefore an instance".
type governance uint8

const (
	// governanceUnavailable is a note nothing can be said about, because one
	// of the two authorities has closed. It is the zero value because it is
	// the answer that asserts least.
	governanceUnavailable governance = iota
	// governedInstance is a note the folder's lifecycle governs: the page
	// offers its status face and, for a lesson, its lesson affordances.
	governedInstance
	// outsideKnowledgeLayer is a note the contract never placed under its state
	// machine: it sits outside the directories scan.knowledge_dirs declares, so
	// the page names the layer rather than claiming the schema defines nothing
	// onward. It is still a note, and reads as one.
	outsideKnowledgeLayer
	// readableArtifact is a note the folder holds outside its lifecycle —
	// readable, never adjudicated. The page says so rather than apologising
	// for a status face that was never meant to be there.
	readableArtifact
)

// classifyGovernance places one note against two authority samples taken at
// different instants: the request's captured lifecycle view, and the
// snapshot's own artifact capture. A note is placed only while both still
// answer, whichever was taken first. The knowledge layer is the contract's own
// and does not move, so the lifecycle view answers for it alone.
func classifyGovernance(lifecycle status.Authority, policy schema.ArtifactPolicy, relPath string) governance {
	if lifecycle.Closed() || !policy.Available() {
		return governanceUnavailable
	}
	if policy.IsNonInstance(relPath) {
		return readableArtifact
	}
	if errors.Is(lifecycle.WhyUngoverned(relPath), status.ErrOutsideKnowledgeScope) {
		return outsideKnowledgeLayer
	}
	return governedInstance
}

type governanceState struct {
	shell nav.Shell
	// placement is where the request's authorities put this note. Everything
	// below is read for a governed instance and left empty for the other two.
	placement governance
	// status is what the note's own file says, read for this request. It is
	// empty unless the write face applies to this note; the page falls back to
	// the scan's value, which is the only answer available when nothing may be
	// written and is then never contradicted by anything.
	status          string
	transitions     []pages.Transition
	writeDiagnostic string
	noFrontmatter   bool
	// statusUnknown is set when the note's non-empty status value is not in
	// the contract's declared list for its type, so the page can state that
	// fact instead of implying the schema defines nothing onward from it.
	statusUnknown bool
}

// instance reports a note the folder holds under the lifecycle's vocabulary
// rather than as a readable artifact, which is what decides whether the page
// reads a status and dresses a lesson. The declared knowledge layer decides
// where the state machine runs, not what a note is, so a note outside it is
// still one.
func (s *governanceState) instance() bool {
	return s.placement == governedInstance || s.placement == outsideKnowledgeLayer
}

// nonInstance reports a note the folder holds outside its lifecycle. It is
// not the negation of instance: a note neither authority could be asked about
// is neither.
func (s *governanceState) nonInstance() bool { return s.placement == readableArtifact }

// outsideLayer reports a note the declared knowledge layer withheld, which is
// why its transition set is empty and how the page knows to name the layer.
func (s *governanceState) outsideLayer() bool { return s.placement == outsideKnowledgeLayer }

func (h *Handler) governance(
	ctx context.Context,
	n *snapshot.Reading,
	snap *snapshot.Generation,
	authority status.Authority,
	lang wording.Lang,
) governanceState {
	policy := snap.ArtifactPolicy()
	state := governanceState{
		shell:     shell.Project(authority, snap),
		placement: classifyGovernance(authority, policy, n.RelPath),
	}
	state.writeDiagnostic = authority.WriteDiagnostic(lang)
	if state.writeDiagnostic == "" && !policy.Available() {
		state.writeDiagnostic = policy.Diagnostic()
	}
	if state.instance() && state.writeDiagnostic == "" {
		switch {
		case n.FMDiagnostic != "":
			// Bad YAML: diagnostic only, no keys — read isn't reliable enough to
			// write.
		case !n.HasFrontmatter:
			// Legally no frontmatter (e.g. drills): no keys either.
			state.noFrontmatter = true
		default:
			state.status, state.writeDiagnostic = h.observedStatus(ctx, n.RelPath, lang)
			if state.writeDiagnostic == "" {
				state.transitions = offeredTransitions(authority, n.RelPath, n.Type, state.status)
				state.statusUnknown = state.status != "" &&
					authority.DeclaresStatuses(n.Type) &&
					!authority.KnownStatus(n.Type, state.status)
			}
		}
	}
	return state
}

// vouchedOrigin is the status a transition receipt may name, or empty. The
// value arrives in the URL, so anything can put anything there: a hand-typed
// address could otherwise have the page announce a transition that never
// happened, from a status the contract does not declare, or to one this write
// face refuses to perform at all.
//
// The page may state only what it can stand behind, and that is two separate
// facts. The contract has to admit the sentence: the named origin is a
// declared status for this note's type, and the move from it to the status
// the note now carries is one the contract legalises. And the write face has
// to attest the event: consume answers true only while it holds an unspent
// record of a flip it recently performed out of the named origin on this
// note, and spends that record on answering. So the receipt appears on the
// one reading the redirect lands, and a reload, a bookmark, or a hand-typed
// address finds nothing left to vouch for it. The contract checks run first,
// so a claim the page could never repeat spends nothing.
func vouchedOrigin(
	authority status.Authority,
	consume func(rel, from string) bool,
	rel, noteType string,
	move transition,
) string {
	if move.from == "" || move.to == "" || move.from == move.to {
		return ""
	}
	if !authority.KnownStatus(noteType, move.from) {
		return ""
	}
	if !authority.LegalTransition(noteType, move.from, move.to) {
		return ""
	}
	if !consume(rel, move.from) {
		return ""
	}
	return move.from
}

// transition is one move through the lifecycle as a page states it: the
// status the note left, and the status it carries now.
//
// The two travel as one value because they are drawn from one vocabulary and
// read alike, so a check taking them as adjacent parameters can be handed
// them the wrong way round and still compile — and the check below, whose
// whole purpose is to refuse a move the contract does not legalise, would
// then be asking about the return journey and passing whatever it found.
type transition struct {
	from string
	to   string
}

// offeredTransitions pairs each legal target with whether the face could
// walk the note back from it to the status it carries now — which is what
// decides between the single-press control and the two-step confirm.
func offeredTransitions(authority status.Authority, relPath, noteType, current string) []pages.Transition {
	targets := authority.Transitions(relPath, noteType, current)
	if len(targets) == 0 {
		return nil
	}
	offered := make([]pages.Transition, 0, len(targets))
	for _, to := range targets {
		offered = append(offered, pages.Transition{To: to, NoReturn: !authority.CanReturn(noteType, current, to)})
	}
	return offered
}

// observedStatus asks the write package what the note's status line says right
// now, and turns a failure into a closed write face rather than a guess.
//
// Falling back to the scan's value would be the wrong repair: the value it
// holds may be exactly the one the reader has already moved away from, and a
// transition offered from it is refused on arrival. Whatever prevented this
// read is the same thing that would prevent the write.
func (h *Handler) observedStatus(ctx context.Context, rel string, lang wording.Lang) (current, blocked string) {
	current, err := h.sources.ObservedStatus(ctx, rel)
	if err != nil {
		// A reader who navigated away is not a fault to report: the read was
		// refused because nobody is waiting for it, and logging that as a
		// failure teaches an operator to distrust a log that is telling the
		// truth about everything else.
		if ctx.Err() == nil {
			h.sources.Log.Warn("read the note's own status for the reading page", "path", rel, "error", err)
		}
		return "", wording.NoteStatusUnreadable.In(lang)
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
func (h *Handler) loadConcepts(
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
		renderBody := func(rel, body string) string { return snap.RenderIn(region, rel, body, lang).HTML }
		if d, ok := snap.Concepts().Document(renderBody, rel); ok {
			docs = append(docs, d)
		}
	}
	return docs
}
