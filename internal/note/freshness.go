package note

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"sync"

	"github.com/koopa0/yomihon/internal/vault"
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
func (h *Handler) freshness(w http.ResponseWriter, r *http.Request) {
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !servable(rel) || !vault.IsMarkdown(rel) {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	rendered, ok := parseIdentity(query.Get("identity"))
	if !ok {
		http.Error(w, "identity must be "+strconv.Itoa(identityHexLen)+" hex digits", http.StatusBadRequest)
		return
	}
	// A polled state has no cache: an answer held from a previous tick is the
	// one thing this endpoint must never give.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	answer := h.compareNote(r, rel, rendered, query.Get("status"), query.Has("status"))
	if _, err := w.Write([]byte(answer)); err != nil {
		return
	}
}

// compareNote settles the five answers for one note, against everything the
// page carries: the identity of the bytes it rendered, and — when statusCarried
// — the status it printed beside the title.
func (h *Handler) compareNote(
	r *http.Request,
	rel string,
	rendered [sha256.Size]byte,
	printedStatus string,
	statusCarried bool,
) freshness {
	entry, err := h.deps.Source.Lookup(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return freshGone
		}
		h.noteFreshnessFailure(rel, "lookup", err)
		return freshUnreadable
	}
	data, err := h.deps.Source.ReadFile(r.Context(), entry)
	if err != nil {
		h.noteFreshnessFailure(rel, "read", err)
		return freshUnreadable
	}
	disk := vault.ContentIdentity(data)

	// The published generation is asked second and only about the disk, never
	// about the page: what decides whether a reload is worth offering is
	// whether the generation a reload would render already holds what this
	// read just saw.
	published, ok := h.deps.Snapshot().Capture().Note(rel)
	if !ok || published.ContentIdentity != disk {
		return freshPreparing
	}
	if disk != rendered {
		return freshStale
	}
	// The printed status settles the answer only when a caller carried one and
	// the bytes are level. The disk's side of the pair comes from the same
	// parse that decides what a page prints, so the two sides cannot disagree
	// about what a status line means.
	if statusCarried && vault.Parse(rel, data).Status() != printedStatus {
		return freshStale
	}
	return freshUnchanged
}

// noteFreshnessFailure records a read failure once per change of cause. A
// reader with the page open asks again every few seconds, and a fault that
// persists has already been said.
func (h *Handler) noteFreshnessFailure(rel, op string, err error) {
	cause := op + ": " + err.Error()
	if h.freshnessFailures.changed(rel, cause) {
		h.deps.Log.Warn("freshness check could not read the note", "path", rel, "operation", op, "error", err)
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
