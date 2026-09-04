// Package snapshot owns the reading server's coherent vault generations. One
// scanner observes the rooted vault capability, reads each markdown file at most
// once per generation, builds every derived projection from those captured
// notes, and publishes one generation with an atomic pointer swap. A rebuild
// that cannot read everything retains the last published generation.
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
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

// scanInterval is the reconciliation cadence: about two seconds, with no
// watcher and no second file-identity model.
const scanInterval = 2 * time.Second

// maxRetryDelay caps the exponential backoff between full rebuild attempts while
// a wanted source stays unreadable. A metadata-visible change retries at once.
const maxRetryDelay = time.Minute

// degradeAfter is how many build attempts in a row may come back incomplete —
// counted from the last generation that read every source it wanted — before the
// folder publishes what it could read instead of holding the previous
// generation. The retry schedule puts those a few seconds apart, because waiting
// longer answers every note written since with a 404 in a folder otherwise fine.
const degradeAfter = 3

// reconcileEvery is the number of scan ticks between unconditional rebuilds. The
// fast path compares only file identity and metadata, so an in-place edit that
// preserves inode, mode, size and mtime is invisible to it and can stay stale for
// about five minutes; a quiescent folder pays one full re-read per period.
const reconcileEvery = 150

// Source is the rooted read capability required to construct a generation. It is
// defined by its only consumer so a test can count scans and reads.
type Source interface {
	ScanAvailable(context.Context) (vaultfs.Scan, error)
	ReadFile(context.Context, vaultfs.Entry) ([]byte, error)
}

// Freshness is the published account of how the reading generation relates to
// the folder on disk. Both degraded states are otherwise invisible: a startup
// view may omit a source so reading stays available, and a failing rebuild
// retains the previous generation. It is assembled at read time from a
// generation's fixed buildFacts and its owning Store's live liveAttempt.
type Freshness struct {
	// BuiltAt is when the generation being reported finished building.
	BuiltAt time.Time
	// Complete reports whether that generation read every source it wanted; a
	// retained stale view keeps it true, and Blocked says what holds the next.
	Complete bool
	// Blocked lists the sources that generation never read and the ones the
	// latest attempt could not, each with its error. Empty means whole.
	Blocked []BlockedSource
	// FailedRetries counts the incomplete rebuild attempts in a row behind it.
	FailedRetries int
	// LastComplete is when the most recent generation that read every source it
	// wanted was built, as known when the reported generation was published;
	// zero means no whole read since startup. It is separate from BuiltAt
	// because "this page is seconds old" and "the folder was last seen whole an
	// hour ago" are two facts a reader needs.
	LastComplete time.Time
}

// buildFacts is one generation's own fixed account of itself: when it finished
// building, whether it read every source it wanted, the sources it never got,
// and when a whole read last happened. It is set once and never changes.
type buildFacts struct {
	builtAt      time.Time
	complete     bool
	blocked      []BlockedSource
	lastComplete time.Time
}

// liveAttempt is a Store's continuously updated account of its latest rebuild
// attempt: the sources it currently cannot have, and how many attempts in a row
// came back incomplete. Every Generation published while one is current shares a
// pointer to it, so a page serving a retained generation can say so.
type liveAttempt struct {
	blocked       []BlockedSource
	failedRetries int
}

// BlockedSource is one vault path a build wanted, with why it could not have it.
type BlockedSource struct {
	Path   string
	Reason string
}

// Generation is one immutable reading generation, every projection built from
// the same scan and captured bytes. The artifact policy is a source-bound
// revocation capability: callers capture it once per request, so contract drift
// closes later requests without changing a response already being rendered.
type Generation struct {
	graph          *graph.Index
	navigation     *nav.Model
	search         *lexical.Index
	slots          lesson.SlotIndex
	concepts       lesson.ConceptIndex
	planned        judge.Planned
	backlinks      *Backlinks
	health         Health
	artifactPolicy schema.ArtifactPolicy
	privacyPolicy  schema.PrivacyPolicy

	scan     vaultfs.Scan
	notes    map[string]Reading
	markdown *render.Pipeline

	// schemaFindings is what the schema said about each note when this generation
	// read it, reached once so a page and the check command answer for one read.
	schemaFindings map[string][]judge.Finding

	// titles maps each declared title to every note declaring it, for the question
	// the resolver is built not to answer.
	titles map[string][]nav.NoteRef

	// parsed and sidecars are what a later build falls back on for a source it can
	// no longer read; the projections already hold both, so this costs two maps.
	parsed   map[string]*vault.Note
	sidecars map[string][]byte

	// built is this generation's own account of itself, fixed when it was
	// published, so it stays true beside the content a response captured.
	built buildFacts

	// freshness points at the owning Store's live account of the latest build
	// attempt. It is deliberately not frozen at build time: while rebuilds fail
	// the retained generation is the one being served, and it is what has to be
	// able to say the folder has moved on without it.
	freshness *atomic.Pointer[liveAttempt]
}

// Capture returns a request-local Generation bound to one point-in-time artifact
// authority, so a contract change mid-response cannot reach the response.
func (g *Generation) Capture() *Generation {
	if g == nil {
		return nil
	}
	captured := *g
	captured.artifactPolicy = g.artifactPolicy.Capture()
	captured.privacyPolicy = g.privacyPolicy
	captured.search = g.search.WithArtifactPolicy(captured.artifactPolicy)
	return &captured
}

// Graph returns the immutable wikilink resolver for this generation.
func (g *Generation) Graph() *graph.Index {
	if g == nil {
		return nil
	}
	return g.graph
}

// CitedBy returns the notes citing relPath in this generation, sorted by the
// name each shows. Nothing citing a note is an answer rather than a gap.
func (g *Generation) CitedBy(relPath string) []nav.NoteRef {
	if g == nil {
		return nil
	}
	return g.backlinks.To(relPath)
}

// Freshness reports how the generation this view holds relates to the folder on
// disk right now. The build facts are this generation's own, so a response is
// never told a newer generation's build time beside content it is not showing.
// The blocked list unions what this generation never read with what the running
// attempt cannot read, naming more trouble than the reader suffers but never
// less. The returned value is the caller's own copy.
func (g *Generation) Freshness() Freshness {
	if g == nil {
		return Freshness{}
	}
	out := Freshness{
		BuiltAt:      g.built.builtAt,
		Complete:     g.built.complete,
		Blocked:      slices.Clone(g.built.blocked),
		LastComplete: g.built.lastComplete,
	}
	if g.freshness == nil {
		return out
	}
	attempt := g.freshness.Load()
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

// Health returns this generation's whole-folder view of what needs attention.
func (g *Generation) Health() Health {
	if g == nil {
		return Health{}
	}
	return g.health
}

// AnyCitations reports whether any note in this generation cites another.
func (g *Generation) AnyCitations() bool {
	return g != nil && g.backlinks.Any()
}

// TrackedForwardReference reports whether target is a name the vault is
// deliberately writing toward: it resolves to no file, and some note declared it
// as a concept still owed, so the link records intent rather than a fault.
// Resolution is asked first, as the adjudicator asks it, because a target that
// does resolve can still fail to render for an unrelated reason.
func (g *Generation) TrackedForwardReference(target string) bool {
	if g == nil || g.graph == nil {
		return false
	}
	return g.graph.Resolve(target).Kind == graph.KindUnresolved && g.planned.Has(target)
}

// Navigation returns the immutable navigation model for this generation.
func (g *Generation) Navigation() *nav.Model {
	if g == nil {
		return nil
	}
	return g.navigation
}

// Search returns the immutable lexical index for this generation.
func (g *Generation) Search() *lexical.Index {
	if g == nil {
		return nil
	}
	return g.search
}

// Slots returns the immutable lesson-slot index for this generation.
func (g *Generation) Slots() lesson.SlotIndex {
	if g == nil {
		return lesson.SlotIndex{}
	}
	return g.slots
}

// Concepts returns the immutable concept-sheet index for this generation.
func (g *Generation) Concepts() lesson.ConceptIndex {
	if g == nil {
		return lesson.ConceptIndex{}
	}
	return g.concepts
}

// ArtifactPolicy returns this Generation's point-in-time artifact authority. A handler
// calls Capture at entry; that Generation then holds one authority all response long.
func (g *Generation) ArtifactPolicy() schema.ArtifactPolicy {
	if g == nil {
		return schema.ArtifactPolicy{}
	}
	return g.artifactPolicy.Capture()
}

// PrivacyPolicy returns this Generation's egress authority, as the generation that
// read the contract found it. Nothing this Generation serves consults it: egress
// authority governs the adjudication commands, and a page is not one. It travels
// here so a reader can be told why those commands are unusable, which their own
// output cannot say without quoting the vault out under the missing policy.
func (g *Generation) PrivacyPolicy() schema.PrivacyPolicy {
	if g == nil {
		return schema.PrivacyPolicy{}
	}
	return g.privacyPolicy
}

// Files returns this generation's captured regular files in canonical path
// order. The returned slice and entries are independent of the Generation.
func (g *Generation) Files() []vaultfs.Entry {
	if g == nil {
		return nil
	}
	return g.scan.Files()
}

// Skipped returns the paths this generation's scan saw and did not index, such
// as a symbolic link standing where a note is expected. They are carried
// beside the files because a folder that organises by link would otherwise
// lose notes with nothing anywhere saying so.
func (g *Generation) Skipped() []vaultfs.Skipped {
	if g == nil {
		return nil
	}
	return g.scan.Skipped()
}

// Entry returns the captured regular-file identity for canonicalPath.
func (g *Generation) Entry(canonicalPath string) (vaultfs.Entry, bool) {
	if g == nil {
		return vaultfs.Entry{}, false
	}
	return g.scan.Entry(canonicalPath)
}

// Contains reports whether canonicalPath was a file or directory in this
// generation.
func (g *Generation) Contains(canonicalPath string) bool {
	return g != nil && g.scan.Contains(canonicalPath)
}

// Note returns an immutable Markdown reading projection by value.
func (g *Generation) Note(canonicalPath string) (Reading, bool) {
	if g == nil {
		return Reading{}, false
	}
	note, ok := g.notes[canonicalPath]
	return note, ok
}

// Render projects markdown through the resolver and captured transclusion bodies
// owned by this same generation, so a request cannot combine links from one scan
// with note bodies from another. relPath is where the body lives in the vault —
// the answer to "relative to what", which nothing in the body supplies.
func (g *Generation) Render(relPath, body string, lang wording.Lang) render.Result {
	return g.RenderIn("", relPath, body, lang)
}

// RenderIn is Render for a body sharing a page with another, naming the region
// this one occupies. It qualifies footnote ids only, not heading ids.
func (g *Generation) RenderIn(region, relPath, body string, lang wording.Lang) render.Result {
	if g == nil || g.markdown == nil {
		return render.Result{}
	}
	// The title the page will show, so the renderer can spot a heading repeating it.
	return g.markdown.HTMLIn(region, relPath, g.notes[relPath].Title, body, lang)
}

// Transclusion returns the immutable body captured for canonicalPath, exposing
// no parsed note and no mutable map.
func (g *Generation) Transclusion(canonicalPath string) (string, bool) {
	note, ok := g.Note(canonicalPath)
	return note.Body, ok
}

// Store publishes the current Generation and drives its single reconciliation loop,
// which owns prev, retry and the backoff fields. Current is safe concurrently.
type Store struct {
	ptr atomic.Pointer[Generation]

	// fresh is the live account of the latest build attempt: the sources it
	// could not have and how many attempts in a row came back incomplete. It
	// carries no build facts — those belong to a generation. The reconciliation
	// loop is its only writer.
	fresh atomic.Pointer[liveAttempt]

	source       Source
	log          *slog.Logger
	now          func() time.Time
	capabilities schema.Capabilities

	// contract is the folder's own vocabulary, read once at startup — the same
	// reading the capabilities above came from. No Generation holds it.
	contract *schema.Contract
	prev     vaultfs.Scan
	retry    bool

	// consecutiveIncomplete, nextRetry, and incompleteScan bound the retry loop.
	// While attempts keep coming back incomplete over an unchanged file domain,
	// each failure doubles the wait up to maxRetryDelay; any metadata-visible
	// change bypasses the wait and restarts the schedule.
	consecutiveIncomplete int
	nextRetry             time.Time
	incompleteScan        vaultfs.Scan

	// incompleteSincePublish counts the build attempts that have come back
	// incomplete since the last generation that read everything, and decides
	// when the folder stops holding that generation back. It is separate from
	// consecutiveIncomplete because that count restarts whenever the folder
	// changes, and a folder being edited would then never reach the threshold.
	incompleteSincePublish int

	// lastComplete is when the last generation that read every source it wanted
	// was built; a degraded one carries it so a page can say how old that is.
	lastComplete time.Time

	// sinceRebuild counts scan ticks since the last completed build attempt,
	// driving the slow cycle that catches metadata-invisible edits.
	sinceRebuild int

	// running records that the reconciliation loop has been claimed. The
	// fields above are that loop's alone and carry no synchronization, so a
	// second Run is refused rather than left to advance them beside the first.
	running atomic.Bool
}

// New captures and builds the initial generation synchronously. source and log
// are required; a nil contract builds instance-derived projections over the
// empty declared set while ordinary reading continues. governance is what the
// folder asserted about its own contract, which a nil contract cannot answer —
// a folder with no contract and one whose contract could not be read both
// arrive with none, and only the second is a fault.
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

	capabilities := contract.Capabilities(governance)
	capabilities.Artifacts = capabilities.Artifacts.ValidateSource()
	validatePrivacySource(contract)
	scan, err := source.ScanAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("build initial vault snapshot: %w", err)
	}
	gen, blocked, err := buildGeneration(ctx, source, nil, scan, log, capabilities, contract)
	if err != nil {
		return nil, fmt.Errorf("build initial vault snapshot: %w", err)
	}
	store := &Store{
		source:       source,
		log:          log,
		now:          time.Now,
		capabilities: capabilities,
		contract:     contract,
		prev:         scan,
		retry:        len(blocked) != 0,
	}
	builtAt := store.now()
	gen.built = buildFacts{
		builtAt:  builtAt,
		complete: len(blocked) == 0,
		blocked:  blocked,
	}
	if gen.built.complete {
		gen.built.lastComplete = builtAt
		store.lastComplete = builtAt
	} else {
		// Startup publishes an incomplete generation so reading stays available,
		// and it is the first attempt the degrade threshold counts.
		store.incompleteSincePublish = 1
	}
	store.fresh.Store(&liveAttempt{blocked: blocked})
	gen.freshness = &store.fresh
	store.ptr.Store(gen)
	store.logBuild("vault snapshot built", gen, scan)
	return store, nil
}

// Current returns the published generation. It is non-nil after New succeeds.
func (s *Store) Current() *Generation { return s.ptr.Load() }

// Run reconciles the vault until ctx is cancelled. Call it once per Store: it
// advances the fields the reconciliation loop owns, which are unsynchronized
// because there is one loop, and a second call is a programming error rather
// than a runtime condition.
func (s *Store) Run(ctx context.Context) {
	if !s.running.CompareAndSwap(false, true) {
		panic("snapshot: Store.Run called twice")
	}
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

// validatePrivacySource re-reads the contract file behind the egress
// declaration and latches the declaration unusable if the bytes have moved,
// on the same beat the artifact policy is checked. Nothing is assigned back:
// every copy of the policy points at one latch inside the contract, so closing
// it once closes it for the judging commands and for the page that has to say
// why they stopped answering. Calling it is the whole effect.
func validatePrivacySource(contract *schema.Contract) {
	contract.PrivacyPolicy().ValidateSource()
}

func (s *Store) rescan(ctx context.Context) {
	scan, err := s.source.ScanAvailable(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.log.Warn("vault scan unavailable; retaining previous snapshot", "error", err)
		}
		return
	}
	// The metadata comparison cannot see an in-place edit that preserves inode,
	// mode, size and mtime, so every reconcileEvery-th tick rebuilds without the
	// short-circuit. It never fires while rebuilds are failing: the backoff owns
	// the cadence there, and every retry is already a full re-read.
	s.sinceRebuild++
	reconcile := !s.retry && s.sinceRebuild >= reconcileEvery
	if !s.retry && !reconcile && s.prev.SameFiles(scan) {
		return
	}
	// While rebuilds keep failing over an unchanged file domain, the expensive
	// attempt waits out its backoff; any visible change falls through and retries.
	if s.retry && s.now().Before(s.nextRetry) && s.incompleteScan.SameFiles(scan) {
		return
	}
	if s.capabilities.Artifacts.Available() {
		s.capabilities.Artifacts = s.capabilities.Artifacts.ValidateSource()
	}
	validatePrivacySource(s.contract)
	candidate, blocked, err := buildGeneration(
		ctx,
		s.source,
		s.ptr.Load(),
		scan,
		s.log,
		s.capabilities,
		s.contract,
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			s.log.Warn("vault snapshot rebuild failed; retaining previous snapshot", "error", err)
		}
		return
	}
	// A completed attempt re-read every file, so the reconciliation clock restarts.
	s.sinceRebuild = 0
	if len(blocked) != 0 {
		// The attempt is recorded first, so the retry schedule is the same
		// whether or not this attempt goes on to publish.
		s.noteIncomplete(scan, blocked)
		s.publishOnceDegraded(candidate, scan, blocked)
		return
	}
	// Cleared before the swap, so a reader of the new generation sees no stale trouble.
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
	s.incompleteScan = vaultfs.Scan{}
	s.logBuild("vault snapshot rebuilt", candidate, scan)
}

// publishOnceDegraded publishes an attempt that could not read every source it
// wanted, once such attempts have outlasted degradeAfter in a row. Until then
// retaining the last whole generation is right; past it, retention is the more
// damaging fault, since every note written since is answered with a 404 in a
// folder otherwise intact. What publishes holds the last copy of each source it
// could not read and records that it is not whole. The retry state is untouched.
func (s *Store) publishOnceDegraded(candidate *Generation, scan vaultfs.Scan, blocked []BlockedSource) {
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
// attempt restarts the schedule: the world moved, so this is a new failure. The
// blocked sources and the running count go to the live record, which is how a
// page serving the retained generation can say the folder has moved on.
func (s *Store) noteIncomplete(scan vaultfs.Scan, blocked []BlockedSource) {
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
		"scan_skipped", len(scan.Skipped()),
		"consecutive_incomplete", s.consecutiveIncomplete,
		"next_retry", s.nextRetry)
}

// retryDelay is the wait after the nth consecutive incomplete rebuild over an
// unchanged file domain: one scan interval, doubled each time, capped.
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

// buildGeneration performs one generation build from one completed enumeration. A scan
// or file-read problem returns that source as blocked so the caller retries; the
// source itself is taken from previous when that generation read it, and is
// otherwise omitted. previous is nil at startup. Cancellation aborts the build.
func buildGeneration(
	ctx context.Context,
	source Source,
	previous *Generation,
	scan vaultfs.Scan,
	log *slog.Logger,
	//nolint:gocritic // hugeParam: the copy is the point and it replaces four heavier
	// parameters. A pointer would let the reconciliation loop's next
	// revalidation reach a build already in progress.
	capabilities schema.Capabilities,
	contract *schema.Contract,
) (*Generation, []BlockedSource, error) {
	// Classification is generation data, so one point-in-time policy builds every
	// projection; Capture rebinds request-time access to the current authority.
	projectionPolicy := capabilities.Artifacts.Capture()
	entries := scan.Files()
	g := newGeneration(len(entries))
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
			// A wikilink may point at any vault file, read or not.
			g.resources = append(g.resources, relPath)
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
			g.carry(carried, relPath, note, want)
			continue
		}
		if !note {
			g.captureFile(relPath, data, want.indexable)
			continue
		}
		g.captureNote(vault.Parse(relPath, data), data, capabilities.Language, want.indexable)
		g.recordVerdict(relPath, data, contract, log)
	}

	graphIndex := graph.New(slices.Concat(g.ordered, g.unreadable), g.resources)
	titles := titlesByName(g.ordered)
	navigation := nav.New(entries, g.parsed, graphIndex, capabilities.Navigation, capabilities.Knowledge, projectionPolicy)
	searchIndex := lexical.NewIndex(indexDocuments(g.ordered, g.indexable, g.files), projectionPolicy)

	slots, slotProblems := lesson.NewSlotIndex(g.sidecars)
	for _, problem := range slotProblems {
		// One unusable sidecar costs one lesson its practice panel, not the rest.
		log.Warn("slot sidecar unusable in snapshot generation",
			"path", problem.Source, "problem", problem.Message)
	}
	concepts, err := lesson.NewConceptIndex(g.ordered)
	if err != nil {
		log.Warn("concept sheets unavailable in snapshot generation", "error", err)
		concepts = lesson.ConceptIndex{}
	}

	planned := judge.NewPlanned(noteBodies(g.ordered))
	backlinks := newBacklinks(g.ordered, graphIndex)
	gen := &Generation{
		graph:          graphIndex,
		navigation:     navigation,
		search:         searchIndex,
		slots:          slots,
		concepts:       concepts,
		planned:        planned,
		backlinks:      backlinks,
		health:         newHealth(g.ordered, graphIndex, planned, backlinks, capabilities.Artifacts, titles),
		artifactPolicy: capabilities.Artifacts,
		privacyPolicy:  contract.PrivacyPolicy(),
		scan:           scan,
		notes:          g.readings,
		schemaFindings: g.findings,
		titles:         titles,
		parsed:         g.parsed,
		sidecars:       g.sidecars,
	}
	gen.markdown = render.New(graphIndex, gen, gen)
	return gen, blocked, nil
}

// generation is the set of collections one reading of the folder fills, owned
// together because every one is filled from two places: the source this reading
// opened, and the copy the last generation held for a source it could not. A
// projection added here is added to a receiver both paths already have.
type generation struct {
	// parsed is every note this generation holds, by path.
	parsed map[string]*vault.Note
	// ordered is the same set as a list, in the order the folder was read.
	ordered []*vault.Note
	// unreadable are stubs, so a citation to one still lands on a file.
	unreadable []*vault.Note
	// readings is the reading projection each page renders.
	readings map[string]Reading
	// indexable records the decision each note's own entry was judged by.
	indexable map[string]bool
	// sidecars are the practice files the lesson parser reads.
	sidecars map[string][]byte
	// files are the index documents for vault files that are not notes.
	files []lexical.Document
	// resources are every non-note path, read or not: a wikilink may name any.
	resources []string
	// findings are the schema's verdicts, kept only for notes that drew one.
	findings map[string][]judge.Finding
}

// newGeneration opens an empty generation sized for a folder of entries files.
func newGeneration(entries int) *generation {
	return &generation{
		parsed:     make(map[string]*vault.Note),
		ordered:    make([]*vault.Note, 0, entries),
		unreadable: make([]*vault.Note, 0),
		readings:   make(map[string]Reading),
		indexable:  make(map[string]bool),
		sidecars:   make(map[string][]byte),
		files:      make([]lexical.Document, 0, entries),
		resources:  make([]string, 0, entries),
		findings:   make(map[string][]judge.Finding),
	}
}

// captureNote files one note this reading opened into every projection built
// from a note, down to the index membership its own entry was judged by.
func (g *generation) captureNote(parsed *vault.Note, data []byte, languages schema.ArticleLanguage, indexable bool) {
	g.parsed[parsed.RelPath] = parsed
	g.ordered = append(g.ordered, parsed)
	g.indexable[parsed.RelPath] = indexable
	g.readings[parsed.RelPath] = newReading(parsed, data, languages, indexable)
}

// recordVerdict reaches the schema's verdict for one note and keeps it when
// there is one. A note the schema is content with is left out rather than stored
// empty; absent and clean read the same at the accessor. The only fault it can
// meet is a slug pattern nothing can compile, which is said once and leaves that
// note without a verdict rather than the folder without a generation.
func (g *generation) recordVerdict(relPath string, data []byte, contract *schema.Contract, log *slog.Logger) {
	findings, err := judge.LintFrontmatter(relPath, data, contract)
	if err != nil {
		log.Warn("schema verdict unavailable for a note", "path", relPath, "error", err)
		return
	}
	if len(findings) > 0 {
		g.findings[relPath] = findings
	}
}

// carriedGeneration is the reading of the folder a build falls back on, source
// by source. Its maps belong to the published generation and are never written.
type carriedGeneration struct {
	parsed   map[string]*vault.Note
	captured map[string]Reading
	sidecars map[string][]byte
}

// carriedFrom takes the fallback reading from the generation currently
// published. A nil one yields empty maps, so a first failed read leaves nothing.
func carriedFrom(previous *Generation) carriedGeneration {
	if previous == nil {
		return carriedGeneration{}
	}
	return carriedGeneration{
		parsed:   previous.parsed,
		captured: previous.notes,
		sidecars: previous.sidecars,
	}
}

// carry gives this generation whatever the fallback held for a source this
// reading could not open, by whether that source is a note or a practice file.
func (g *generation) carry(from carriedGeneration, relPath string, note bool, want bytesWanted) {
	if note {
		g.carryNote(from, relPath)
		return
	}
	g.carryFile(from, relPath, want)
}

// carryNote gives the generation being built the copy of relPath the fallback
// generation read, marked as one that could not be re-read. Both halves are
// required: the resolver, navigation and index are built from the parsed note,
// and the page renders the captured projection. Without such a copy the note is
// left out and the resolver gets a stub, so a citation naming it still lands on
// it rather than joining the citations to files that do not exist.
func (g *generation) carryNote(from carriedGeneration, relPath string) {
	lastKnown, lastKnownOK := from.parsed[relPath]
	captured, capturedOK := from.captured[relPath]
	if !lastKnownOK || !capturedOK {
		g.unreadable = append(g.unreadable, vault.Parse(relPath, nil))
		return
	}
	captured.Stale = true
	g.parsed[relPath] = lastKnown
	g.ordered = append(g.ordered, lastKnown)
	g.readings[relPath] = captured
	// The carried copy answers for itself, so the index and the note's own page
	// describe the same bytes — the last ones read.
	g.indexable[relPath] = captured.Searchable
}

// carryFile gives the generation being built the practice file the fallback read,
// when it read one. Any other unreadable file is simply absent.
func (g *generation) carryFile(from carriedGeneration, relPath string, want bytesWanted) {
	lastKnown, ok := from.sidecars[relPath]
	if !ok {
		return
	}
	g.captureFile(relPath, lastKnown, want.indexable)
}

// blockedFromProblems carries the scan's unobservable paths into the build's
// blocked-source list, so an unopenable directory reports like an unread file.
func blockedFromProblems(problems []vaultfs.Problem) []BlockedSource {
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

// noteBodies iterates the parsed bodies of one generation's notes: every note
// the server read, not the narrower corpus the adjudicator harvests.
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

// captureFile files one vault entry that is not a note into the projections that
// can use it: its own parser, if it is a lesson sidecar, and the text corpus, if
// its bytes are characters. The bytes decide that last question, as they do on
// the file's own page, because a name is not evidence about what is inside.
func (g *generation) captureFile(relPath string, data []byte, indexable bool) {
	if lesson.IsSlotSidecar(relPath) {
		g.sidecars[relPath] = data
	}
	// indexable carries the decision the entry itself was judged by; deciding again
	// here would index a file whose own page says it is not searched.
	if !indexable || !render.IsText(data) {
		return
	}
	g.files = append(g.files, lexical.DocumentFromFile(relPath, data))
}

// bytesWanted is why a generation reads one vault file, and what follows if not.
type bytesWanted struct {
	read      bool
	indexable bool
	// holdsBackGeneration means an unread file leaves the reading surface
	// incomplete, so the generation is retried rather than published. A note and
	// a lesson's sidecar qualify; any other file only lends the index its words,
	// and holding the folder to one permanently unreadable file would stop every
	// later change from ever being published.
	holdsBackGeneration bool
}

// wantedBytes decides what this generation needs from one scanned entry.
func wantedBytes(entry vaultfs.Entry, note bool) bytesWanted {
	if note {
		// A note is always read; only the index has a bound, the one the file page
		// applies, because a note is held there three times over.
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
// too large for the index is skipped here, so one place decides searchability.
func indexDocuments(
	notes []*vault.Note,
	indexable map[string]bool,
	files []lexical.Document,
) []lexical.Document {
	documents := make([]lexical.Document, 0, len(notes)+len(files))
	for _, note := range notes {
		if indexable[note.RelPath] {
			documents = append(documents, lexical.DocumentFromNote(note))
		}
	}
	return append(documents, files...)
}

// readableAsText reports whether a vault file that is not a note is a candidate
// for the text index: not a picture, not a PDF, and small enough that its own
// page shows its characters. The predicates are the page's, so if yomihon shows
// a file to you as text you can find it.
func readableAsText(entry vaultfs.Entry) bool {
	relPath := entry.Path()
	return !render.IsPicture(relPath) &&
		!render.IsPDF(relPath) &&
		withinSourceCap(entry)
}

// withinSourceCap reports whether a file is small enough for its characters to be
// held. It is the ceiling the file page applies, so search and display agree.
func withinSourceCap(entry vaultfs.Entry) bool {
	return entry.Size() <= render.MaxSourceBytes
}

func (s *Store) logBuild(message string, gen *Generation, scan vaultfs.Scan) {
	s.log.Info(message,
		"files", len(scan.Files()),
		"scan_problems", len(scan.Problems()),
		"scan_skipped", len(scan.Skipped()),
		"indexed", gen.search.Len(),
		"paths", gen.navigation.PathCount(),
		"maps", gen.navigation.MapCount(),
		"journal", len(gen.navigation.Journal()),
		"reports", len(gen.navigation.Reports()),
		"slot_lessons", gen.slots.Len(),
		"concepts", gen.concepts.Len(),
	)
}
