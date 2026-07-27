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
	"log/slog"
	pathpkg "path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/vault"
)

// scanInterval is the reconciliation cadence. It preserves the ruled
// approximately-two-second freshness bound without a second watcher or file
// identity model.
const scanInterval = 2 * time.Second

// Source is the rooted read capability required to construct a generation.
// The interface is defined by its only consumer so tests can count scans and
// reads without weakening vault.Reader's production identity checks.
type Source interface {
	ScanAvailable(context.Context) (vault.Scan, error)
	ReadFile(context.Context, vault.Entry) ([]byte, error)
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
	artifactPolicy schema.ArtifactPolicy

	scan     vault.Scan
	notes    map[string]Note
	markdown *render.Pipeline
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
func (v *View) Render(relPath, body string) render.Result {
	if v == nil || v.markdown == nil {
		return render.Result{}
	}
	return v.markdown.HTML(relPath, body)
}

// Transclusion returns the immutable body captured for canonicalPath. It is
// render.Transclusions' consumer-owned capability and deliberately exposes no
// parsed note or mutable map.
func (v *View) Transclusion(canonicalPath string) (string, bool) {
	note, ok := v.Note(canonicalPath)
	return note.Body, ok
}

// Store publishes the current View and drives its single reconciliation loop.
// prev and retry are owned by that one loop; Current is safe for concurrent
// request use.
type Store struct {
	ptr atomic.Pointer[View]

	source          Source
	log             *slog.Logger
	roles           schema.NavigationRoles
	artifactPolicy  schema.ArtifactPolicy
	articleLanguage schema.ArticleLanguage
	prev            vault.Scan
	retry           bool
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

	roles, policy, language := governance.Capabilities(contract)
	policy = policy.ValidateSource()
	scan, err := source.ScanAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("build initial vault snapshot: %w", err)
	}
	view, retry, err := buildView(ctx, source, scan, log, roles, policy, language)
	if err != nil {
		return nil, fmt.Errorf("build initial vault snapshot: %w", err)
	}
	store := &Store{
		source:          source,
		log:             log,
		roles:           roles,
		artifactPolicy:  policy,
		articleLanguage: language,
		prev:            scan,
		retry:           retry,
	}
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
	if !s.retry && s.prev.SameFiles(scan) {
		return
	}
	if s.artifactPolicy.Available() {
		s.artifactPolicy = s.artifactPolicy.ValidateSource()
	}
	candidate, retry, err := buildView(
		ctx,
		s.source,
		scan,
		s.log,
		s.roles,
		s.artifactPolicy,
		s.articleLanguage,
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			s.log.Warn("vault snapshot rebuild failed; retaining previous snapshot", "error", err)
		}
		return
	}
	if retry {
		s.retry = true
		s.log.Warn("vault snapshot incomplete; retaining previous generation",
			"scan_problems", len(scan.Problems()))
		return
	}
	s.ptr.Store(candidate)
	s.prev = scan
	s.retry = false
	s.logBuild("vault snapshot rebuilt", candidate, scan)
}

// buildView performs one generation build from one completed enumeration. A
// scan or file-read problem omits that source from the candidate and requests a
// retry. New may publish such a candidate to preserve startup availability;
// rescan retains its previous generation instead. Cancellation aborts the
// generation so shutdown never publishes half a build.
func buildView(
	ctx context.Context,
	source Source,
	scan vault.Scan,
	log *slog.Logger,
	roles schema.NavigationRoles,
	policy schema.ArtifactPolicy,
	languages schema.ArticleLanguage,
) (*View, bool, error) {
	// Classification is generation data, so one point-in-time policy must build
	// every projection. View retains the source-bound handle separately and
	// Capture rebinds request-time metadata access to the current authority.
	projectionPolicy := policy.Capture()
	entries := scan.Files()
	parsedByPath := make(map[string]*vault.Note)
	parsedNotes := make([]*vault.Note, 0, len(entries))
	publishedNotes := make(map[string]Note)
	resources := make([]string, 0, len(entries))
	slotFiles := make(map[string][]byte)
	fileDocuments := make([]search.Document, 0, len(entries))
	retry := len(scan.Problems()) != 0

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		relPath := entry.Path()
		note := strings.HasSuffix(relPath, ".md")
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
				return nil, false, contextErr
			}
			log.Warn("vault source unavailable in snapshot generation", "path", relPath, "error", err)
			retry = retry || want.holdsBackGeneration
			continue
		}
		if !note {
			fileDocuments = captureFile(relPath, data, want.indexable, slotFiles, fileDocuments)
			continue
		}
		parsed := vault.Parse(relPath, data)
		parsedByPath[relPath] = parsed
		parsedNotes = append(parsedNotes, parsed)
		publishedNotes[relPath] = captureNote(parsed, data, languages)
	}

	graphIndex := graph.New(parsedNotes, resources)
	navigation := nav.New(entries, parsedByPath, graphIndex, roles, projectionPolicy)
	documents := make([]search.Document, 0, len(parsedNotes)+len(fileDocuments))
	for _, note := range parsedNotes {
		documents = append(documents, search.DocumentFromNote(note))
	}
	documents = append(documents, fileDocuments...)
	searchIndex := search.NewIndex(documents, projectionPolicy)

	slots, err := lesson.NewSlotIndex(slotFiles)
	if err != nil {
		log.Warn("slot sidecars unavailable in snapshot generation", "error", err)
		slots = lesson.SlotIndex{}
	}
	concepts, err := lesson.NewConceptIndex(parsedNotes)
	if err != nil {
		log.Warn("concept sheets unavailable in snapshot generation", "error", err)
		concepts = lesson.ConceptIndex{}
	}

	view := &View{
		graph:          graphIndex,
		navigation:     navigation,
		search:         searchIndex,
		slots:          slots,
		concepts:       concepts,
		artifactPolicy: policy,
		scan:           scan,
		notes:          publishedNotes,
	}
	view.markdown = render.New(graphIndex, view)
	return view, retry, nil
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
		return bytesWanted{read: true, holdsBackGeneration: true}
	}
	sidecar := lesson.IsSlotSidecar(entry.Path())
	indexable := readableAsText(entry)
	return bytesWanted{
		read:                sidecar || indexable,
		indexable:           indexable,
		holdsBackGeneration: sidecar,
	}
}

// readableAsText reports whether a vault file that is not a note is a candidate
// for the text index: not shown as a picture, and small enough that its own page
// shows its characters rather than an information card. The predicate is the
// page's, so one rule holds across both faces — if yomihon shows it to you as
// text, you can find it.
func readableAsText(entry vault.Entry) bool {
	relPath := entry.Path()
	return !render.IsPicture(relPath) &&
		!strings.EqualFold(pathpkg.Ext(relPath), ".pdf") &&
		entry.Size() <= render.MaxSourceBytes
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
