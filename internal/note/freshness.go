package note

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"sync"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// freshness is what a reading page can be told about the note it is showing.
// Five answers rather than a yes and a no, because the page acts differently
// on each: only one of them means a reload would deliver something new, and
// two of them mean the question could not be settled this time. Collapsing the
// unsettled pair into "changed" would turn a passing read failure into a claim
// the reader would act on, which is the same refusal the write face makes when
// it cannot confirm what it is replacing.
type freshness string

const (
	// freshUnchanged: the bytes on disk are the bytes this page rendered — the
	// status it printed included, when it carried one — and the published
	// generation agrees.
	freshUnchanged freshness = "unchanged"
	// freshStale: the note changed and the published generation already holds
	// its content, so reloading now shows the new version. A flip of the
	// status value alone also lands here: that value sits outside the content
	// the identity covers, and a fresh render prints it anew while an open
	// page keeps what it printed.
	freshStale freshness = "stale"
	// freshPreparing: the note changed on disk and the published generation has
	// not caught up. Reloading now would render the same bytes again, so the
	// page says a new version was detected and does not offer the reload yet.
	freshPreparing freshness = "preparing"
	// freshGone: nothing is at this path any more. A rename reads the same way
	// as a delete from here, and the page says so rather than guessing which.
	freshGone freshness = "gone"
	// freshUnreadable: the file is there and this attempt could not read it.
	// Not knowing is its own answer and never becomes one of the others.
	freshUnreadable freshness = "unreadable"
)

// identityHexLen is the width of the identity a page carries: one SHA-256 sum
// in lower-case hex.
const identityHexLen = 2 * sha256.Size

// freshnessLog remembers the last unexpected read failure it reported, so a
// page asking every few seconds cannot turn one broken file into an unbounded
// stream of identical lines. One slot rather than a table per path: this is a
// local tool with one reader, a second failing path simply replaces the first,
// and what carries information is the change of cause, not its repetition.
// An absent file is not recorded here at all — it is a legitimate answer, and
// a file that will not come back would otherwise report itself forever.
type freshnessLog struct {
	mu    sync.Mutex
	path  string
	cause string
}

// changed reports whether this failure differs from the last one recorded, and
// records it either way.
func (l *freshnessLog) changed(path, cause string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.path == path && l.cause == cause {
		return false
	}
	l.path, l.cause = path, cause
	return true
}

// freshness answers whether the note a page rendered is still the note on
// disk. It reads the file itself rather than the published generation: the
// generation is rebuilt from a metadata comparison that an edit preserving
// inode, mode, size and mtime slips past until the next unconditional
// rebuild, and a reader who just saved in Obsidian should not wait that out to
// be told anything. Nothing here writes: the vault is read through the same
// rooted capability the page used, and the published generation is only
// observed.
//
// The identity a page carries covers every byte except the frontmatter status
// value: the write face binds a ruling to exactly the bytes it does not
// rewrite, and this check keeps that boundary. A page left open keeps showing
// whatever status it printed, however live the value was at render time, so
// the reading page carries that printed status beside the identity and the
// answer covers the pair — a flip of the one value the identity leaves out is
// stale like any other change a reload would deliver. A caller that carries no
// status, as the recovery page does not, is compared on its identity alone.
//
// A page whose render pulled words in from other notes carries a third value:
// the identity of the excerpts it transcluded, which the host's own identity
// cannot cover because those bytes live in other files. It is compared the
// same way the status is — only when carried, and only once the host's own
// bytes are level — so a page that transcluded nothing keeps exactly the ask
// and the answer it always had.
func (h *Handler) freshness(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !servable(rel) || !vault.IsMarkdown(rel) {
		http.Error(w, wording.FreshnessNotWatchable.In(lang), http.StatusNotFound)
		return
	}
	query := r.URL.Query()
	rendered, ok := parseIdentity(query.Get("identity"))
	if !ok {
		http.Error(w, malformedIdentity("identity", lang), http.StatusBadRequest)
		return
	}
	ask := freshnessAsk{
		rendered:      rendered,
		printedStatus: query.Get("status"),
		statusCarried: query.Has("status"),
	}
	if query.Has("embeds") {
		transcluded, ok := parseIdentity(query.Get("embeds"))
		if !ok {
			http.Error(w, malformedIdentity("embeds", lang), http.StatusBadRequest)
			return
		}
		ask.transcluded, ask.transcludedCarried = transcluded, true
	}
	// A polled state has no cache: an answer held from a previous tick is the
	// one thing this endpoint must never give.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	answer := h.compareNote(r, rel, &ask)
	if _, err := w.Write([]byte(answer)); err != nil {
		return
	}
}

// malformedIdentity says which of the poll's two identity fields was not the
// digest it has to be, and how long one is. The field name is the one written
// in the address, so it is quoted rather than translated.
func malformedIdentity(field string, lang wording.Lang) string {
	return fmt.Sprintf(wording.FreshnessIdentityFmt.In(lang), field, identityHexLen)
}

// freshnessAsk is everything one polling page states about itself: the
// identity of the bytes it rendered, the status it printed beside the title,
// and the identity of what it transcluded. The two optional halves each
// travel with their own carried flag because absence and emptiness are
// different claims — a page that printed no status stamps an empty one, while
// the recovery page carries none at all, and a page that transcluded nothing
// carries no stamp rather than a digest of nothing.
type freshnessAsk struct {
	rendered           [sha256.Size]byte
	printedStatus      string
	statusCarried      bool
	transcluded        [sha256.Size]byte
	transcludedCarried bool
}

// compareNote settles the five answers for one note, against everything the
// page carries in its ask.
func (h *Handler) compareNote(r *http.Request, rel string, ask *freshnessAsk) freshness {
	entry, err := h.sources.Source.Lookup(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return freshGone
		}
		h.noteFreshnessFailure(rel, "lookup", err)
		return freshUnreadable
	}
	data, err := h.sources.Source.ReadFile(r.Context(), entry)
	if err != nil {
		h.noteFreshnessFailure(rel, "read", err)
		return freshUnreadable
	}
	disk := vault.ContentIdentity(data)

	// The published generation is asked second and only about the disk, never
	// about the page: what decides whether a reload is worth offering is
	// whether the generation a reload would render already holds what this
	// read just saw.
	snap := h.sources.Snapshot().Capture()
	published, ok := snap.Note(rel)
	if !ok || published.ContentIdentity != disk {
		return freshPreparing
	}
	if disk != ask.rendered {
		return freshStale
	}
	// The printed status settles the answer only when a caller carried one and
	// the bytes are level. The disk's side of the pair comes from the same
	// parse that decides what a page prints, so the two sides cannot disagree
	// about what a status line means.
	if ask.statusCarried && vault.Parse(rel, data).Status() != ask.printedStatus {
		return freshStale
	}
	// The transcluded stamp is settled last and from the generation alone: a
	// change to an excerpt the page already expanded reaches it only once a
	// reload would actually render that change, and the render below reads
	// the same captured bodies a reload would. Until the generation catches
	// such an edit up, the honest answer is that nothing a reload could
	// deliver has changed. What the stamp covers is exactly what was
	// expanded: an embed that resolved to nothing on the open page is not in
	// it, so a source that has since become embeddable is news only a reader
	// who reloads on their own receives. The recomputation runs only for a
	// page that carried a stamp, so a page that transcluded nothing costs
	// this endpoint nothing new.
	if ask.transcludedCarried &&
		hex.EncodeToString(ask.transcluded[:]) != h.transcludedNow(snap, rel, published.Body) {
		return freshStale
	}
	return freshUnchanged
}

// transcludedNow is the transcluded identity a reload would stamp right now:
// the same render the reading page performs, over the generation's own copy
// of the body, keeping the digest and discarding the page. The language is
// pinned because the digest covers source bytes pulled from other notes,
// which no language of the interface's own sentences can reach.
func (h *Handler) transcludedNow(snap *snapshot.Generation, rel, body string) string {
	return snap.Render(rel, body, wording.ZhHant).TranscludedIdentity
}

// noteFreshnessFailure records a read failure once per change of cause. A
// reader with the page open asks again every few seconds, and a fault that
// persists has already been said.
func (h *Handler) noteFreshnessFailure(rel, op string, err error) {
	cause := op + ": " + err.Error()
	if h.freshnessFailures.changed(rel, cause) {
		h.sources.Log.Warn("freshness check could not read the note", "path", rel, "operation", op, "error", err)
	}
}

// parseIdentity accepts exactly the hex a page carries and nothing wider: a
// short or malformed value is a caller's mistake, and decoding it leniently
// would answer a question nobody asked.
func parseIdentity(hexed string) (id [sha256.Size]byte, ok bool) {
	if len(hexed) != identityHexLen {
		return id, false
	}
	if _, err := hex.Decode(id[:], []byte(hexed)); err != nil {
		return id, false
	}
	return id, true
}
