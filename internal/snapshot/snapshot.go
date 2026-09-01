// Package snapshot owns the reading server's coherent vault generations.
// One scanner observes the rooted vault capability, reads each Markdown file
// at most once for that generation, builds every derived projection from those
// captured notes, and publishes one coherent View with an atomic pointer swap.
// The initial View may omit a source that was unreadable during startup so the
// reading surface remains available; later incomplete rebuilds retain the last
// published generation until a complete retry succeeds.
package snapshot

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// scanInterval is the reconciliation cadence. It preserves the ruled
// approximately-two-second freshness bound without a second watcher or file
// identity model.
const scanInterval = 2 * time.Second

// maxRetryDelay caps the exponential backoff between full rebuild attempts
// while a wanted source stays unreadable. The cheap metadata scan still runs
// every scanInterval, so any visible change — a fixed permission, a new file —
// retries immediately; the cap bounds only how often an unchanged and still
// unreadable folder is fully re-read.
const maxRetryDelay = time.Minute

// degradeAfter is how many build attempts in a row may come back incomplete —
// counted from the last generation that read every source it wanted — before
// the folder publishes what it could read instead of holding the previous
// generation. The retry schedule puts those attempts about zero, two, and six
// seconds after the first failure, so the folder starts telling the truth
// about itself within a few seconds rather than after the schedule has
// stretched to its one-minute cap. Waiting longer means every note written
// after one unreadable file is answered with a 404, in a folder that is
// otherwise entirely fine — for a reading tool that is a worse trade than
// showing the last copy of the one file that cannot be re-read.
const degradeAfter = 3

// reconcileEvery is the number of scan ticks between unconditional rebuilds.
// The fast path compares only file identity and metadata, so an in-place edit
// that preserves inode, mode, size, and mtime is invisible to it; every
// reconcileEvery-th tick rebuilds without the short-circuit so such an edit is
// still republished eventually. The trade: those edits can stay stale for up
// to about five minutes, and a quiescent folder pays one full re-read (about
// 0.7 CPU-seconds at three thousand notes) per period.
const reconcileEvery = 150

// Source is the rooted read capability required to construct a generation.
// The interface is defined by its only consumer so tests can count scans and
// reads without weakening vault.Reader's production identity checks.
type Source interface {
	ScanAvailable(context.Context) (vault.Scan, error)
	ReadFile(context.Context, vault.Entry) ([]byte, error)
}

// Freshness is the published account of how the reading generation relates to
// the folder on disk. It exists because both degraded states are otherwise
// invisible: a startup view may omit a source so reading stays available, and
// a failing rebuild retains the previous generation — in either case the
// pages would present a partial or stale folder as current and whole. It is
// assembled at read time from a generation's fixed buildFacts and its owning
// Store's live liveAttempt; see View.Freshness.
type Freshness struct {
	// BuiltAt is when the generation being reported finished building.
	BuiltAt time.Time
	// Complete reports whether that generation read every source it wanted.
	// Startup may publish an incomplete view for availability; a rescan
	// publishes complete generations only, so a retained stale view keeps
	// Complete true while Blocked says what is holding the next one.
	Complete bool
	// Blocked lists the sources that generation never read and the ones the
	// latest build attempt could not observe or read, with the error each
	// returned. Empty means the reading surface is whole and current.
	Blocked []BlockedSource
	// FailedRetries counts the rebuild attempts that have come back incomplete
	// in a row behind the generation being reported.
	FailedRetries int
	// LastComplete is when the most recent generation that did read every
	// source it wanted was built, as that was known when the generation being
	// reported was published. Zero means no whole read has happened since
	// startup. It is separate from BuiltAt because a generation may be
	// published without sources it could not re-read, and "this page is a few
	// seconds old" and "the folder was last seen whole an hour ago" are then
	// two different facts, both of which the reader needs.
	LastComplete time.Time
}

// buildFacts is one generation's own fixed account of itself: when it
// finished building, whether it read every source it wanted, the sources it
// never got, and when a whole read last happened. It is set once when a
// generation is published and never changes afterward — unlike the owning
// Store's liveAttempt, which keeps moving while a later build is retried.
type buildFacts struct {
	builtAt      time.Time
	complete     bool
	blocked      []BlockedSource
	lastComplete time.Time
}

// liveAttempt is a Store's continuously updated account of its latest rebuild
// attempt: the sources it currently cannot have, and how many attempts in a
// row have come back incomplete. Every View published while a liveAttempt is
// current shares a pointer to the same value, which is how a page serving a
// retained generation can say the folder has moved on without it.
type liveAttempt struct {
	blocked       []BlockedSource
	failedRetries int
}

// BlockedSource is one vault path a generation build wanted and could not
// have, and the reason it could not.
type BlockedSource struct {
	Path   string
	Reason string
}

// View is one immutable reading generation. Every projection is built from the
// same scan and captured bytes and remains behind read-only package APIs. The
// artifact policy is a source-bound revocation capability: callers capture it
// once per request, so contract drift closes later requests without changing a
// response already being rendered.
type View struct {
	graph          *graph.Index
	navigation     *nav.Model
	search         *search.Index
	slots          lesson.SlotIndex
	concepts       lesson.ConceptIndex
	planned        judge.Planned
	backlinks      *Backlinks
	health         Health
	artifactPolicy schema.ArtifactPolicy
	privacyPolicy  schema.PrivacyPolicy

	scan     vault.Scan
	notes    map[string]Note
	markdown *render.Pipeline

	// schemaFindings is what the schema said about each note when this
	// generation read it. The verdict is reached once here rather than per
	// request, so a page and the check command answer for the same bytes.
	schemaFindings map[string][]judge.Finding

	// titles maps each declared title to every note declaring it, for the one
	// question the resolver is built not to answer: what a citation written
	// against a title was reaching for.
	titles map[string][]nav.NoteRef

	// parsed and sidecars are what a later build can fall back on for a source
	// it can no longer read: the frontmatter and body this generation parsed,
	// and the practice files a lesson is built from. Both are already held by
	// this generation's projections, so keeping them addressable by path costs
	// two maps rather than a second copy of the folder.
	parsed   map[string]*vault.Note
	sidecars map[string][]byte

	// built is this generation's own account of itself, fixed when it was
	// published: when the build finished, whether it read every source it
	// wanted, and the sources it never got. A response renders the generation
	// it captured, so these are the facts that belong beside that content even
	// after a later generation has replaced it.
	built buildFacts

	// freshness points at the owning Store's live account of the latest build
	// attempt. Unlike every other projection it is deliberately not frozen at
	// build time: while rebuilds fail, the retained generation is the one being
	// served, and it is exactly the view that must be able to say the folder
	// has moved on without it.
	freshness *atomic.Pointer[liveAttempt]
}

// Capture returns a request-local View bound to one point-in-time artifact
// authority. Immutable generation data remains shared; the search index is a
// shallow functional copy so its metadata operations use that same authority
// even if the contract changes while the response is being rendered.
func (v *View) Capture() *View {
	if v == nil {
		return nil
	}
	captured := *v
	captured.artifactPolicy = v.artifactPolicy.Capture()
	captured.privacyPolicy = v.privacyPolicy
	captured.search = v.search.WithArtifactPolicy(captured.artifactPolicy)
	return &captured
}

// Graph returns the immutable wikilink resolver for this generation.
func (v *View) Graph() *graph.Index {
	if v == nil {
		return nil
	}
	return v.graph
}

// CitedBy returns the notes citing relPath in this generation, sorted by the
// name each shows. Nothing citing a note is an answer rather than a gap: it is
// how a note nothing depends on becomes visible.
func (v *View) CitedBy(relPath string) []nav.NoteRef {
	if v == nil {
		return nil
	}
	return v.backlinks.To(relPath)
}

// Freshness reports how the generation this view holds relates to the folder
// on disk right now: when it was built, whether it read everything it wanted,
// and which sources it or the latest build attempt could not have. The build
// facts are this generation's own, so a response that captured it is never
// told a newer generation's build time or completeness beside content it is
// not showing. The blocked list is the union of what this generation never
// read and what the attempt running now cannot read, which lets the account
// name more trouble than the reader is suffering but never less. The returned
// value is the caller's own copy.
func (v *View) Freshness() Freshness {
	if v == nil {
		return Freshness{}
	}
	out := Freshness{
		BuiltAt:      v.built.builtAt,
		Complete:     v.built.complete,
		Blocked:      slices.Clone(v.built.blocked),
		LastComplete: v.built.lastComplete,
	}
	if v.freshness == nil {
		return out
	}
	attempt := v.freshness.Load()
	if attempt == nil {
		return out
	}
	out.FailedRetries = attempt.failedRetries
	for _, source := range attempt.blocked {
		if !slices.ContainsFunc(out.Blocked, func(known BlockedSource) bool { return known.Path == source.Path }) {
			out.Blocked = append(out.Blocked, source)
		}
	}
	return out
}

// Health returns the whole-folder view of what needs attention in this
// generation.
func (v *View) Health() Health {
	if v == nil {
		return Health{}
	}
	return v.health
}

// AnyCitations reports whether any note in this generation cites another.
func (v *View) AnyCitations() bool {
	return v != nil && v.backlinks.Any()
}

// TrackedForwardReference reports whether target is a name the vault is
// deliberately writing toward: it resolves to no file, and some note has
// declared it as a concept still owed. The vault is written forward — a note
// lists what it has yet to write and then links those names from wherever they
// belong — so such a link records intent rather than a fault, and the reading
// page must not count it as one.
//
// Both halves are required, and they are the same two the adjudicator applies
// in that order. Resolution is asked first because a target that does resolve
// can still fail to render for reasons that have nothing to do with planning —
// an embed whose body this generation did not capture, say — and a name-only
// test would quietly swallow that.
func (v *View) TrackedForwardReference(target string) bool {
	if v == nil || v.graph == nil {
		return false
	}
	return v.graph.Resolve(target).Kind == graph.Unresolved && v.planned.Has(target)
}

// Navigation returns the immutable navigation model for this generation.
func (v *View) Navigation() *nav.Model {
	if v == nil {
		return nil
	}
	return v.navigation
}

// Search returns the immutable lexical index for this generation.
func (v *View) Search() *search.Index {
	if v == nil {
		return nil
	}
	return v.search
}

// Slots returns the immutable lesson-slot index for this generation.
func (v *View) Slots() lesson.SlotIndex {
	if v == nil {
		return lesson.SlotIndex{}
	}
	return v.slots
}

// Concepts returns the immutable concept-sheet index for this generation.
func (v *View) Concepts() lesson.ConceptIndex {
	if v == nil {
		return lesson.ConceptIndex{}
	}
	return v.concepts
}

// ArtifactPolicy returns this View's point-in-time artifact authority. Request
// handlers must call Capture at entry before combining it with other View
// projections; on that request-local View this method returns the same frozen
// authority for the whole response.
func (v *View) ArtifactPolicy() schema.ArtifactPolicy {
	if v == nil {
		return schema.ArtifactPolicy{}
	}
	return v.artifactPolicy.Capture()
}

// PrivacyPolicy returns this View's egress authority, as the generation that
// read the contract found it.
//
// Nothing this View serves consults it: egress authority governs the
// adjudication commands, and a page is not one. It travels here so a reader
// can be told why those commands are unusable, which is a promise the commands
// themselves make and cannot keep — their output is written for a program, and
// naming the fault there would quote the vault back out under the very policy
// that is missing.
func (v *View) PrivacyPolicy() schema.PrivacyPolicy {
	if v == nil {
		return schema.PrivacyPolicy{}
	}
	return v.privacyPolicy
}

// Files returns the regular files captured in this generation, in canonical
// path order. The returned slice and entries are independent of the View.
func (v *View) Files() []vault.Entry {
	if v == nil {
		return nil
	}
	return v.scan.Files()
}

// Entry returns the captured regular-file identity for canonicalPath.
func (v *View) Entry(canonicalPath string) (vault.Entry, bool) {
	if v == nil {
		return vault.Entry{}, false
	}
	return v.scan.Entry(canonicalPath)
}

// Contains reports whether canonicalPath was a file or directory in this
// generation.
func (v *View) Contains(canonicalPath string) bool {
	return v != nil && v.scan.Contains(canonicalPath)
}

// Note returns an immutable Markdown reading projection by value.
func (v *View) Note(canonicalPath string) (Note, bool) {
	if v == nil {
		return Note{}, false
	}
	note, ok := v.notes[canonicalPath]
	return note, ok
}

// Render projects Markdown through the resolver and captured transclusion
// bodies owned by this same generation. Keeping both inputs behind View makes
// it impossible for a request to combine links from one scan with note bodies
// from another.
//
// relPath is where the body lives in the vault. It is the caller's answer to
// "relative to what", which every note body needs and which nothing in the
// body itself supplies.
func (v *View) Render(relPath, body string, lang wording.Lang) render.Result {
	return v.RenderIn("", relPath, body, lang)
}

// RenderIn is Render for a body that shares a page with another rendered body,
// naming the region this one occupies so the two cannot both call their first
// footnote the same thing. The region qualifies footnote ids only — heading
// ids stay unique within a body rather than across the page.
func (v *View) RenderIn(region, relPath, body string, lang wording.Lang) render.Result {
	if v == nil || v.markdown == nil {
		return render.Result{}
	}
	// The title the page will show, so the renderer can tell a heading that
	// repeats it from one that is the only thing naming the document.
	return v.markdown.HTMLIn(region, relPath, v.notes[relPath].Title, body, lang)
}

// Transclusion returns the immutable body captured for canonicalPath. It is
// render.Transclusions' consumer-owned capability and deliberately exposes no
// parsed note or mutable map.
func (v *View) Transclusion(canonicalPath string) (string, bool) {
	note, ok := v.Note(canonicalPath)
	return note.Body, ok
}

// Store publishes the current View and drives its single reconciliation loop.
// prev, retry, and the backoff fields are owned by that one loop; Current is
// safe for concurrent request use.
type Store struct {
	ptr atomic.Pointer[View]

	// fresh is the live account of the latest build attempt: the sources it
	// could not have and how many attempts in a row have come back incomplete.
	// It carries no build facts — when a build finished and whether it read
	// everything belong to a generation, and every published View carries its
	// own. The reconciliation loop is its only writer; requests read it
	// through View.Freshness.
	fresh atomic.Pointer[liveAttempt]

	source          Source
	log             *slog.Logger
	now             func() time.Time
	roles           schema.NavigationRoles
	scope           schema.KnowledgeScope
	artifactPolicy  schema.ArtifactPolicy
	articleLanguage schema.ArticleLanguage

	// contract is the folder's own vocabulary, read once when the program
	// started and unchanged after that — the same reading the four
	// capabilities above were derived from, so nothing here is fresher or
	// staler than they are. Each build asks it what a note's frontmatter
	// should have said; nothing writes to it, and no View holds it.
	contract *schema.Contract
	prev     vault.Scan
	retry    bool

	// consecutiveIncomplete, nextRetry, and incompleteScan bound the retry
	// loop. While rebuild attempts keep coming back incomplete over an
	// unchanged file domain, each failure doubles the wait before the next
	// expensive attempt, up to maxRetryDelay; any metadata-visible change
	// bypasses the wait and restarts the schedule, so recovery stays as
	// immediate as the scan interval.
	consecutiveIncomplete int
	nextRetry             time.Time
	incompleteScan        vault.Scan

	// incompleteSincePublish counts the build attempts that have come back
	// incomplete since the last generation that read everything it wanted, and
	// it is what decides when the folder stops holding that generation back.
	// It is a second count rather than a reuse of consecutiveIncomplete
	// because the two answer different questions: the backoff restarts
	// whenever the folder changes, since a changed folder deserves an
	// immediate attempt, while a folder that is being edited would then never
	// reach a degrade threshold at all — and a folder being edited is exactly
	// the case degrading exists for. It resets only on a whole read, so once
	// the threshold is crossed every later incomplete attempt publishes too.
	incompleteSincePublish int

	// lastComplete is when the last generation that read every source it
	// wanted was built. A degraded generation carries it so the pages can say
	// how old the whole picture is rather than implying there has never been
	// one.
	lastComplete time.Time

	// sinceRebuild counts scan ticks since the last completed build attempt,
	// driving the slow reconciliation cycle that catches metadata-invisible
	// edits.
	sinceRebuild int
}

// New captures and builds the initial generation synchronously. source and log
// are required wiring; contract may be nil, in which case instance-derived
// projections are built over the empty declared set while ordinary reading
// continues. governance is what the folder asserted about its own contract,
// which a nil contract cannot answer — a folder with no contract and a folder
// whose contract could not be read both arrive with none, and only the second
// is a fault. When contract is non-nil, governance must be
// contract.Governance().
func New(
	ctx context.Context,
	source Source,
	log *slog.Logger,
	contract *schema.Contract,
	governance schema.Governance,
) (*Store, error) {
	if source == nil {
		panic("snapshot: New requires a non-nil Source")
	}
	if log == nil {
		panic("snapshot: New requires a non-nil Logger")
	}

	roles, scope, policy, language := governance.Capabilities(contract)
	policy = policy.ValidateSource()
	scan, err := source.ScanAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("build initial vault snapshot: %w", err)
	}
	view, blocked, err := buildView(ctx, source, nil, scan, log, roles, scope, policy, language, contract)
	if err != nil {
		return nil, fmt.Errorf("build initial vault snapshot: %w", err)
	}
	store := &Store{
		source:          source,
		log:             log,
		now:             time.Now,
		roles:           roles,
		scope:           scope,
		artifactPolicy:  policy,
		articleLanguage: language,
		contract:        contract,
		prev:            scan,
		retry:           len(blocked) != 0,
	}
	builtAt := store.now()
	view.built = buildFacts{
		builtAt:  builtAt,
		complete: len(blocked) == 0,
		blocked:  blocked,
	}
	if view.built.complete {
		view.built.lastComplete = builtAt
		store.lastComplete = builtAt
	} else {
		// Startup publishes an incomplete generation so reading stays
		// available. It is the first attempt that did not read the folder
		// whole, and the threshold counts from here.
		store.incompleteSincePublish = 1
	}
	store.fresh.Store(&liveAttempt{blocked: blocked})
	view.freshness = &store.fresh
	store.ptr.Store(view)
	store.logBuild("vault snapshot built", view, scan)
	return store, nil
}

// Current returns the published generation. It is non-nil after New succeeds.
func (s *Store) Current() *View { return s.ptr.Load() }

// Run reconciles the vault until ctx is cancelled.
func (s *Store) Run(ctx context.Context) {
	runScanner(ctx, func() { s.rescan(ctx) })
}

func runScanner(ctx context.Context, scan func()) {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		}
	}
}

func (s *Store) rescan(ctx context.Context) {
	scan, err := s.source.ScanAvailable(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Warn("vault scan unavailable; retaining previous snapshot", "error", err)
		}
		return
	}
	// The metadata comparison cannot see an in-place edit that preserves
	// inode, mode, size, and mtime, so every reconcileEvery-th tick rebuilds
	// without the short-circuit. Reconciliation never fires while rebuilds
	// are failing: the backoff owns the rebuild cadence there, and every
	// retry attempt is already a full re-read.
	s.sinceRebuild++
	reconcile := !s.retry && s.sinceRebuild >= reconcileEvery
	if !s.retry && !reconcile && s.prev.SameFiles(scan) {
		return
	}
	// While rebuilds keep failing over an unchanged file domain, the expensive
	// attempt waits out its backoff delay. The cheap scan above already ran,
	// so any metadata-visible change — including the fix that makes the source
	// readable again — falls through here and retries at once.
	if s.retry && s.now().Before(s.nextRetry) && s.incompleteScan.SameFiles(scan) {
		return
	}
	if s.artifactPolicy.Available() {
		s.artifactPolicy = s.artifactPolicy.ValidateSource()
	}
	candidate, blocked, err := buildView(
		ctx,
		s.source,
		s.ptr.Load(),
		scan,
		s.log,
		s.roles,
		s.scope,
		s.artifactPolicy,
		s.articleLanguage,
		s.contract,
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			s.log.Warn("vault snapshot rebuild failed; retaining previous snapshot", "error", err)
		}
		return
	}
	// A completed build attempt re-read every file, whether or not it
	// publishes, so the reconciliation clock restarts here.
	s.sinceRebuild = 0
	if len(blocked) != 0 {
		// The attempt is recorded first, so the retry schedule that governs
		// how often the folder is re-read is the same whether or not this
		// attempt goes on to publish.
		s.noteIncomplete(scan, blocked)
		s.publishOnceDegraded(candidate, scan, blocked)
		return
	}
	// The attempt record is cleared before the pointer swap so a reader of the
	// new generation never sees the previous attempt's trouble beside it.
	s.fresh.Store(&liveAttempt{})
	builtAt := s.now()
	candidate.built = buildFacts{builtAt: builtAt, complete: true, lastComplete: builtAt}
	candidate.freshness = &s.fresh
	s.ptr.Store(candidate)
	s.prev = scan
	s.retry = false
	s.consecutiveIncomplete = 0
	s.incompleteSincePublish = 0
	s.lastComplete = builtAt
	s.nextRetry = time.Time{}
	s.incompleteScan = vault.Scan{}
	s.logBuild("vault snapshot rebuilt", candidate, scan)
}

// publishOnceDegraded publishes an attempt that could not read every source it
// wanted, once such attempts have outlasted degradeAfter in a row. Retaining
// the last whole generation is the right answer while a read failure might
// still be a passing one, so until then this does nothing. Past it, retention
// has become the more damaging fault of the two: every note written since is
// answered with a 404, in a folder whose reading surface is otherwise intact,
// with nothing on the page saying why. What publishes then holds every source
// that could be read plus the last copy of each one that could not, and its
// own record says it is not whole and names the sources behind that, so no
// page presents it as current.
//
// The retry state is untouched: the folder keeps trying for a whole read on
// the same schedule, and every later incomplete attempt publishes too, so work
// written during the failure appears as soon as the scan sees it.
func (s *Store) publishOnceDegraded(candidate *View, scan vault.Scan, blocked []BlockedSource) {
	if s.incompleteSincePublish < degradeAfter {
		return
	}
	candidate.built = buildFacts{
		builtAt:      s.now(),
		complete:     false,
		blocked:      blocked,
		lastComplete: s.lastComplete,
	}
	candidate.freshness = &s.fresh
	s.ptr.Store(candidate)
	s.prev = scan
	s.logBuild("vault snapshot published without its unreadable sources", candidate, scan)
}

// noteIncomplete records one incomplete rebuild attempt and schedules the next
// expensive retry. A failure over a file domain that changed since the last
// attempt restarts the schedule: the world moved, so this is a new failure
// rather than the same one again. The attempt's blocked sources and the
// running failure count go to the live record, which is how the pages serving
// the retained generation can say the folder has moved on without it; that
// generation's own build facts are already fixed in the view being served.
func (s *Store) noteIncomplete(scan vault.Scan, blocked []BlockedSource) {
	if !s.incompleteScan.SameFiles(scan) {
		s.consecutiveIncomplete = 0
	}
	s.consecutiveIncomplete++
	s.incompleteSincePublish++
	s.incompleteScan = scan
	s.nextRetry = s.now().Add(retryDelay(s.consecutiveIncomplete))
	s.retry = true
	s.fresh.Store(&liveAttempt{blocked: blocked, failedRetries: s.consecutiveIncomplete})
	s.log.Warn("vault snapshot incomplete; retaining previous generation",
		"scan_problems", len(scan.Problems()),
		"consecutive_incomplete", s.consecutiveIncomplete,
		"next_retry", s.nextRetry)
}

// retryDelay is the wait after the nth consecutive incomplete rebuild over an
// unchanged file domain: one scan interval, doubled per further failure,
// capped at maxRetryDelay.
func retryDelay(consecutive int) time.Duration {
	delay := scanInterval
	for i := 1; i < consecutive; i++ {
		delay *= 2
		if delay >= maxRetryDelay {
			return maxRetryDelay
		}
	}
	return delay
}

// buildView performs one generation build from one completed enumeration. A
// scan or file-read problem returns that source as blocked so the caller
// retries; the source itself is taken from previous when that generation read
// it, and is otherwise omitted. previous is nil at startup, when there is no
// earlier reading of the folder to fall back on. Cancellation aborts the
// generation so shutdown never publishes half a build.
func buildView(
	ctx context.Context,
	source Source,
	previous *View,
	scan vault.Scan,
	log *slog.Logger,
	roles schema.NavigationRoles,
	scope schema.KnowledgeScope,
	policy schema.ArtifactPolicy,
	languages schema.ArticleLanguage,
	contract *schema.Contract,
) (*View, []BlockedSource, error) {
	// Classification is generation data, so one point-in-time policy must build
	// every projection. View retains the source-bound handle separately and
	// Capture rebinds request-time metadata access to the current authority.
	projectionPolicy := policy.Capture()
	entries := scan.Files()
	parsedByPath := make(map[string]*vault.Note)
	parsedNotes := make([]*vault.Note, 0, len(entries))
	unreadableNotes := make([]*vault.Note, 0)
	publishedNotes := make(map[string]Note)
	schemaFindings := make(map[string][]judge.Finding)
	indexableNotes := make(map[string]bool)
	resources := make([]string, 0, len(entries))
	slotFiles := make(map[string][]byte)
	fileDocuments := make([]search.Document, 0, len(entries))
	blocked := blockedFromProblems(scan.Problems())
	carried := carriedFrom(previous)

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		relPath := entry.Path()
		note := vault.IsMarkdown(relPath)
		want := wantedBytes(entry, note)
		if !note {
			// A wikilink may point at any vault file, so the resolver is told
			// about every one of them whether or not its bytes are read.
			resources = append(resources, relPath)
			if !want.read {
				continue
			}
		}
		data, err := source.ReadFile(ctx, entry)
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return nil, nil, contextErr
			}
			log.Warn("vault source unavailable in snapshot generation", "path", relPath, "error", err)
			if want.holdsBackGeneration {
				blocked = append(blocked, BlockedSource{Path: relPath, Reason: err.Error()})
			}
			parsedNotes, unreadableNotes, fileDocuments = carried.lastCopyOf(
				relPath, note, want,
				parsedNotes, unreadableNotes, parsedByPath, publishedNotes, indexableNotes,
				slotFiles, fileDocuments)
			continue
		}
		if !note {
			fileDocuments = captureFile(relPath, data, want.indexable, slotFiles, fileDocuments)
			continue
		}
		parsed := vault.Parse(relPath, data)
		parsedByPath[relPath] = parsed
		parsedNotes = append(parsedNotes, parsed)
		indexableNotes[relPath] = want.indexable
		publishedNotes[relPath] = captureNote(parsed, data, languages, want.indexable)
		recordVerdict(schemaFindings, relPath, data, contract, log)
	}

	graphIndex := graph.New(slices.Concat(parsedNotes, unreadableNotes), resources)
	titles := titlesByName(parsedNotes)
	navigation := nav.New(entries, parsedByPath, graphIndex, roles, scope, projectionPolicy)
	searchIndex := search.NewIndex(indexDocuments(parsedNotes, indexableNotes, fileDocuments), projectionPolicy)

	slots, slotProblems := lesson.NewSlotIndex(slotFiles)
	for _, problem := range slotProblems {
		// One unusable sidecar is one lesson without its practice panel, not
		// the whole feature, so the generation keeps the ones that read.
		log.Warn("slot sidecar unusable in snapshot generation",
			"path", problem.Source, "problem", problem.Message)
	}
	concepts, err := lesson.NewConceptIndex(parsedNotes)
	if err != nil {
		log.Warn("concept sheets unavailable in snapshot generation", "error", err)
		concepts = lesson.ConceptIndex{}
	}

	planned := judge.NewPlanned(noteBodies(parsedNotes))
	backlinks := newBacklinks(parsedNotes, graphIndex)
	view := &View{
		graph:          graphIndex,
		navigation:     navigation,
		search:         searchIndex,
		slots:          slots,
		concepts:       concepts,
		planned:        planned,
		backlinks:      backlinks,
		health:         newHealth(parsedNotes, graphIndex, planned, backlinks, policy, titles),
		artifactPolicy: policy,
		privacyPolicy:  contract.PrivacyPolicy(),
		scan:           scan,
		notes:          publishedNotes,
		schemaFindings: schemaFindings,
		titles:         titles,
		parsed:         parsedByPath,
		sidecars:       slotFiles,
	}
	view.markdown = render.New(graphIndex, view, view)
	return view, blocked, nil
}

// recordVerdict reaches the schema's verdict for one note and keeps it when
// there is one, so the build loop stays a loop over notes rather than one that
// also lints them. A note the schema is content with is left out rather than
// stored empty: absent and clean read the same at the accessor, and one of
// them is cheaper.
//
// The only fault it can meet is a contract whose slug pattern is written as an
// expression nothing can compile, and that is one fault in one file rather
// than a reason to stop reading the folder: it is said once and the build goes
// on, leaving the note with no verdict rather than the folder with no
// generation.
func recordVerdict(into map[string][]judge.Finding, relPath string, data []byte, contract *schema.Contract, log *slog.Logger) {
	findings, err := judge.LintFrontmatter(relPath, data, contract)
	if err != nil {
		log.Warn("schema verdict unavailable for a note", "path", relPath, "error", err)
		return
	}
	if len(findings) > 0 {
		into[relPath] = findings
	}
}

// carriedGeneration is the reading of the folder a build falls back on, source
// by source, for the ones it cannot read itself. It holds nothing of its own:
// the maps belong to the published generation it was taken from, and are read
// and never written here.
type carriedGeneration struct {
	parsed   map[string]*vault.Note
	captured map[string]Note
	sidecars map[string][]byte
}

// carriedFrom takes the fallback reading from the generation currently
// published. A nil generation — startup, where the folder has never been read
// — yields empty maps, so a source that fails its first read is simply absent.
func carriedFrom(previous *View) carriedGeneration {
	if previous == nil {
		return carriedGeneration{}
	}
	return carriedGeneration{
		parsed:   previous.parsed,
		captured: previous.notes,
		sidecars: previous.sidecars,
	}
}

// lastCopyOf gives the generation being built whatever the fallback generation
// held for a source this reading could not open, choosing by whether that
// source is a note or a practice file. It returns the three collections the
// choice can extend, so the reading loop states the fallback once instead of
// branching on the kind of source in the middle of its read-failure handling.
func (c carriedGeneration) lastCopyOf(
	relPath string,
	note bool,
	want bytesWanted,
	parsedNotes, unreadableNotes []*vault.Note,
	parsedByPath map[string]*vault.Note,
	publishedNotes map[string]Note,
	indexableNotes map[string]bool,
	slotFiles map[string][]byte,
	fileDocuments []search.Document,
) (notes, unreadable []*vault.Note, files []search.Document) {
	if note {
		parsedNotes, unreadableNotes = c.carryNote(
			relPath, parsedNotes, unreadableNotes, parsedByPath, publishedNotes, indexableNotes)
		return parsedNotes, unreadableNotes, fileDocuments
	}
	return parsedNotes, unreadableNotes, c.carryFile(relPath, want, slotFiles, fileDocuments)
}

// carryNote gives the generation being built the copy of relPath the fallback
// generation read, marked as one that could not be re-read so the page showing
// it can say so. Both halves of that copy are required: the parsed note is what
// the resolver, navigation, and index are built from, and the captured
// projection is what the reading page renders, and a generation holding one
// without the other would answer the same question two ways.
//
// Without such a copy the note is left out, and the resolver gets a stub in its
// place: the file exists and a citation naming it lands on it, so the resolver
// still learns its path even though its bytes are missing. Without the stub,
// the health page would class every citation to it with the citations whose
// targets do not exist.
func (c carriedGeneration) carryNote(
	relPath string,
	parsedNotes, unreadableNotes []*vault.Note,
	parsedByPath map[string]*vault.Note,
	publishedNotes map[string]Note,
	indexableNotes map[string]bool,
) (parsed, unreadable []*vault.Note) {
	lastKnown, lastKnownOK := c.parsed[relPath]
	captured, capturedOK := c.captured[relPath]
	if !lastKnownOK || !capturedOK {
		return parsedNotes, append(unreadableNotes, vault.Parse(relPath, nil))
	}
	captured.Stale = true
	parsedByPath[relPath] = lastKnown
	publishedNotes[relPath] = captured
	// The carried copy answers for itself throughout: what the index holds and
	// what the note's own page says about being searchable describe the same
	// bytes, which are the last ones read.
	indexableNotes[relPath] = captured.Searchable
	return append(parsedNotes, lastKnown), unreadableNotes
}

// carryFile gives the generation being built the practice file the fallback
// generation read, when it read one. Any other file that could not be read is
// simply absent from this generation: it lends the search index its words and
// nothing else, and its own page reads it from disk at request time rather
// than from a generation.
func (c carriedGeneration) carryFile(
	relPath string,
	want bytesWanted,
	slotFiles map[string][]byte,
	fileDocuments []search.Document,
) []search.Document {
	lastKnown, ok := c.sidecars[relPath]
	if !ok {
		return fileDocuments
	}
	return captureFile(relPath, lastKnown, want.indexable, slotFiles, fileDocuments)
}

// blockedFromProblems carries the scan's unobservable paths into the build's
// blocked-source list, so a directory that cannot be opened is reported the
// same way as a file that cannot be read.
func blockedFromProblems(problems []vault.Problem) []BlockedSource {
	if len(problems) == 0 {
		return nil
	}
	blocked := make([]BlockedSource, 0, len(problems))
	for _, problem := range problems {
		reason := ""
		if err := problem.Err(); err != nil {
			reason = err.Error()
		}
		blocked = append(blocked, BlockedSource{Path: problem.Path(), Reason: reason})
	}
	return blocked
}

// noteBodies iterates the parsed bodies of one generation's notes. The planned
// index is built from every note the server read, not the narrower corpus the
// adjudicator harvests: nothing derived here leaves the machine, and the only
// reader of this page can already open each of those notes directly.
func noteBodies(notes []*vault.Note) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, n := range notes {
			if n == nil {
				continue
			}
			if !yield(n.Body) {
				return
			}
		}
	}
}

// captureFile files one vault entry that is not a note into the projections
// that can use it: its own parser, if it is a lesson sidecar, and the text
// corpus, if its bytes are characters.
//
// The bytes decide that last question, as they do on the file's own page: a
// name is not evidence about what is inside a file, and a term found in
// something the reader will only ever be shown as opaque bytes would point at a
// page that cannot show it.
func captureFile(
	relPath string,
	data []byte,
	indexable bool,
	slotFiles map[string][]byte,
	fileDocuments []search.Document,
) []search.Document {
	if lesson.IsSlotSidecar(relPath) {
		slotFiles[relPath] = data
	}
	// indexable carries the decision the entry itself was judged by. A sidecar
	// is read whatever its size, because a lesson is built from it, and deciding
	// again from the bytes alone would put a file in the index whose own page
	// says its contents are not searched.
	if !indexable || !render.IsText(data) {
		return fileDocuments
	}
	return append(fileDocuments, search.DocumentFromFile(relPath, data))
}

// bytesWanted is why a generation reads one vault file, and what follows if it
// cannot.
type bytesWanted struct {
	read      bool
	indexable bool
	// holdsBackGeneration means an unread file leaves the reading surface
	// incomplete, so the generation is retried rather than published. A note and
	// a sidecar a lesson is built from qualify. Any other file lends the index
	// its words and nothing else: losing it costs a search hit, while holding the
	// folder hostage to one permanently unreadable file would stop every later
	// change from ever being published, silently, at the generation before it.
	holdsBackGeneration bool
}

// wantedBytes decides what this generation needs from one scanned entry.
func wantedBytes(entry vault.Entry, note bool) bytesWanted {
	if note {
		// A note is always read: every file in the folder stays readable, and
		// its page renders whatever its size. Only the index has a bound, and
		// it is the one the file page already applies — a note is held three
		// times over there (its body, and two folded copies for matching), so
		// it was the one file kind with no ceiling at all.
		return bytesWanted{read: true, indexable: withinSourceCap(entry), holdsBackGeneration: true}
	}
	sidecar := lesson.IsSlotSidecar(entry.Path())
	indexable := readableAsText(entry)
	return bytesWanted{
		read:                sidecar || indexable,
		indexable:           indexable,
		holdsBackGeneration: sidecar,
	}
}

// indexDocuments gathers what this generation will answer searches from. A note
// the generation captured but could not hold in the index is skipped here
// rather than filtered later, so one place decides what is searchable and the
// note's own page can state the same fact.
func indexDocuments(
	notes []*vault.Note,
	indexable map[string]bool,
	files []search.Document,
) []search.Document {
	documents := make([]search.Document, 0, len(notes)+len(files))
	for _, note := range notes {
		if indexable[note.RelPath] {
			documents = append(documents, search.DocumentFromNote(note))
		}
	}
	return append(documents, files...)
}

// readableAsText reports whether a vault file that is not a note is a candidate
// for the text index: not shown as a picture, not handed to the PDF viewer, and
// small enough that its own page shows its characters rather than an
// information card. The predicates are the page's, so one rule holds across
// both faces — if yomihon shows it to you as text, you can find it.
func readableAsText(entry vault.Entry) bool {
	relPath := entry.Path()
	return !render.IsPicture(relPath) &&
		!render.IsPDF(relPath) &&
		withinSourceCap(entry)
}

// withinSourceCap reports whether a file is small enough for its characters to
// be held. It is the same ceiling the file page shows its own readers, so what
// yomihon will search and what it will display as text answer to one rule.
func withinSourceCap(entry vault.Entry) bool {
	return entry.Size() <= render.MaxSourceBytes
}

func (s *Store) logBuild(message string, view *View, scan vault.Scan) {
	s.log.Info(message,
		"files", len(scan.Files()),
		"scan_problems", len(scan.Problems()),
		"indexed", view.search.Len(),
		"paths", len(view.navigation.Paths()),
		"maps", len(view.navigation.Maps()),
		"journal", len(view.navigation.Journal()),
		"reports", len(view.navigation.Reports()),
		"slot_lessons", view.slots.Len(),
		"concepts", view.concepts.Len(),
	)
}
