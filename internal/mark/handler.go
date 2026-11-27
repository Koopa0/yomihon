package mark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxFormBytes bounds the body a mark arrives in: a path, an id, a number and a
// digest never come near it.
const maxFormBytes = 4096

// writeContext is the context a change of the reader's marks is made under: the
// request's, without its cancellation. A change the server has received is made
// even when the page that sent it has gone, a page left as its words were sent
// or a fetch the browser gave up on, since what the reader wrote is theirs to
// keep. Each of its waits for a lock is bounded by confdir.LockWait on its own:
// the vault's, and the ledger's for each line it records. Its wait for a change
// of this process already under way, which machinemarks makes one at a time, is
// not bounded.
func writeContext(r *http.Request) context.Context {
	return context.WithoutCancel(r.Context())
}

// The paths the Handler answers at. They are exported because the route and the
// address a reading page posts to are decided on two sides of a package
// boundary: the command reads them from here and hands them to the page, which
// renders them rather than spelling its own. An address written twice is one
// that drifts.
const (
	BookmarkPath = "/bookmark"
	QuestionPath = "/questions"
	MarkPath     = "/mark"
	FinishedPath = "/finished"
)

// retiredBookmarkPath and retiredQuestionPath are where a bookmark and a
// question were posted before the routes took their present names. A page drawn
// by the previous version may still post there.
const (
	retiredBookmarkPath = "/marks"
	retiredQuestionPath = "/uncertainties"
)

// Places says whether a note is readable now and renders an anchor, what words
// it holds, and what the courses walk. An empty anchor names the note as a
// whole. The routes keep a mark only in a note this reports readable, so none
// stores a path the vault never showed. The question and mark routes also ask
// about the anchor; the bookmark route holds its anchor to a shape only.
// Reading is what a quote is found in, in the language the page is drawn in,
// because a page's own words (a callout's title, a footnote's label) are part
// of what a reader selects. Units is what a course walks: the notes of its main
// line and, apart from them, those of its side branches; ok is false for a
// course there is none of. Rows is what it counts: the rows its main line plans,
// those not yet written included, and the notes of those a reader can open, one
// for each row.
type Places interface {
	HasPlace(rel, anchor string) bool
	Reading(rel string, lang wording.Lang) (Page, bool)
	Units(course string) (mainLine, branches []string, ok bool)
	Rows(course string) (planned int, rows []string, ok bool)
}

// Handler serves the routes a mark reaches the process through, and the ones a
// page reads the marks back from.
//
// A page cannot write a file, so a mark travels over a route of its own on the
// same loopback listener, with a body cap. They are not a face an agent has any
// business calling: these endpoints are the reader's. The routes that write
// refuse a request another origin made (the command wraps them in
// http.NewCrossOriginProtection and origin.LoopbackOnly) and a vault whose
// reader folder cannot be written to.
//
// It keeps a mark only in a note the vault holds now. The path is the one field
// of a mark that can reach the desk as text, so a path nobody's note carries is
// refused rather than stored for a later page to print. It reaches the
// highlights, thoughts and questions only through VaultMarks, which keeps them
// in the vault's machine files, and the bookmarks and finished notes only
// through a Store, which keeps them in the configuration directory.
type Handler struct {
	store      *Store
	vaultMarks *VaultMarks
	places     Places
	log        *slog.Logger
}

// NewHandler wires the routes around a Store, the marks of the vault and
// the vault whose notes a mark may name.
func NewHandler(store *Store, vaultMarks *VaultMarks, places Places, log *slog.Logger) *Handler {
	if store == nil {
		panic("mark: NewHandler requires a non-nil Store")
	}
	if vaultMarks == nil {
		panic("mark: NewHandler requires a non-nil VaultMarks")
	}
	if places == nil {
		panic("mark: NewHandler requires a non-nil Places")
	}
	if log == nil {
		panic("mark: NewHandler requires a non-nil logger")
	}
	return &Handler{store: store, vaultMarks: vaultMarks, places: places, log: log}
}

// Register mounts the routes, and the retired addresses that lead to them. The
// serving command supplies the loopback and cross-origin protections.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+BookmarkPath, h.getBookmark)
	mux.HandleFunc("POST "+BookmarkPath, h.postBookmark)
	mux.HandleFunc("PUT "+BookmarkPath, h.putBookmark)
	mux.HandleFunc("DELETE "+BookmarkPath, h.removeBookmark)
	mux.HandleFunc("POST "+retiredBookmarkPath, movedTo(BookmarkPath))
	mux.HandleFunc("GET "+QuestionPath, h.getQuestions)
	mux.HandleFunc("POST "+QuestionPath, h.postQuestion)
	mux.HandleFunc("GET "+retiredQuestionPath, movedTo(QuestionPath))
	mux.HandleFunc("POST "+retiredQuestionPath, movedTo(QuestionPath))
	mux.HandleFunc("POST "+MarkPath, h.addMark)
	mux.HandleFunc("GET "+MarkPath+"/{id}", h.getMark)
	mux.HandleFunc("PUT "+MarkPath+"/{id}", h.putBack)
	mux.HandleFunc("PATCH "+MarkPath+"/{id}", h.reviseMark)
	mux.HandleFunc("DELETE "+MarkPath+"/{id}", h.removeMark)
	mux.HandleFunc("PUT "+FinishedPath+"/{path...}", h.finish(true))
	mux.HandleFunc("DELETE "+FinishedPath+"/{path...}", h.finish(false))
}

// movedTo answers a retired address with a permanent redirect that keeps the
// method and the body, so a form posted by an older page still arrives. The
// target is a constant path on this server and the query is carried over.
func movedTo(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		where := target
		if r.URL.RawQuery != "" {
			where += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, where, http.StatusPermanentRedirect) // #nosec G710 -- where starts with a constant path on this server
	}
}

// getBookmark answers where the bookmark kept now is, for a page that was drawn
// before it was last set: one shown again from the browser's cache is the page
// of the visit before, and what it draws of the place is that visit's. It names
// the note, the anchor, how far below it and when, which is all a page needs to
// tell whether the place is in it, where, and how old it is; the identity is
// the page's own business and is not sent. It answers no content where none is
// kept. The course is the one the bookmark is kept for, and absent for reading
// outside any. The browser is told not to keep the answer: one that was kept
// would be the staleness this exists to remove.
//
// A file that cannot be read answers no content too, and is logged: the page
// asked on the reader's behalf, and there is nowhere to continue from.
func (h *Handler) getBookmark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	course, ok := courseOf(r)
	if !ok || (course != NoCourse && validatePath(course) != nil) {
		http.Error(w, wording.BookmarkRefused.In(origin.Language(r)), http.StatusUnprocessableEntity)
		return
	}
	b, found, err := h.store.Bookmark(course)
	if err != nil {
		h.log.Warn("read the bookmark", "directory", h.store.Dir(), "error", err)
	}
	if err != nil || !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(struct {
		Path   string    `json:"path"`
		Anchor string    `json:"anchor"`
		Offset int       `json:"offset"`
		At     time.Time `json:"at"`
	}{Path: b.RelPath, Anchor: b.Anchor, Offset: b.Offset, At: b.At}); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the bookmark", "error", err)
	}
}

// postBookmark keeps the place this submission names, replacing whatever was
// kept before. It answers no content on success: the page that posted knows what
// it sent, and there is nothing to render — the row that offers the bookmark
// back is built by the server on the desk, from the file rather than from a
// reply.
func (h *Handler) postBookmark(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.MarkFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(r.PostForm.Get("offset"))
	if err != nil {
		http.Error(w, wording.BookmarkRefused.In(lang), http.StatusUnprocessableEntity)
		return
	}
	b := &Bookmark{
		RelPath:  r.PostForm.Get("path"),
		Anchor:   r.PostForm.Get("anchor"),
		Offset:   offset,
		Identity: r.PostForm.Get("identity"),
		At:       stamp(),
	}
	if _, _, err = h.setBookmark(NoCourse, b, nil); err != nil {
		h.refuseBookmark(w, r, err, "keep the bookmark")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// refuseBookmark answers for a bookmark that was not kept or cleared.
//
// The reader wrote none of the fields of a refused one, so the refusal is a
// fault in the page rather than something they can correct. It is logged where
// it can be acted on and answered with the one sentence they can use.
func (h *Handler) refuseBookmark(w http.ResponseWriter, r *http.Request, err error, doing string) {
	lang := origin.Language(r)
	if errors.Is(err, ErrChanged) {
		http.Error(w, wording.BookmarkMovedElsewhere.In(lang), http.StatusConflict)
		return
	}
	if errors.Is(err, ErrInvalid) {
		h.log.Warn("a bookmark was refused", "doing", doing, "error", err)
		http.Error(w, wording.BookmarkRefused.In(lang), http.StatusUnprocessableEntity)
		return
	}
	h.log.Error("write the bookmark", "doing", doing, "directory", h.store.Dir(), "error", err)
	http.Error(w, wording.BookmarkNotStored.In(lang), http.StatusInternalServerError)
}

// setBookmark stores b as the bookmark of course and answers the one it
// replaced, if that is the one expect names, when its shape is one these files
// hold, its note is one the vault holds and, for a course, the course walks it.
// The shape is judged first, so a path no note could carry never reaches the
// vault to be looked up.
//
// Only the note is asked after: the empty anchor names it as a whole. An anchor
// is any id the rendered body carries — a block address or a footnote as
// readily as a heading — and the vault's answer for an anchor covers headings
// alone, so asking it about the anchor would refuse a place a reading page
// honestly stamps. The anchor stays bounded by its shape, and it only ever
// reaches the desk inside an address.
func (h *Handler) setBookmark(course string, b *Bookmark, expect *Expectation) (Bookmark, bool, error) {
	if err := validateBookmark(course, b); err != nil {
		return Bookmark{}, false, err
	}
	if !h.places.HasPlace(b.RelPath, "") {
		return Bookmark{}, false, fmt.Errorf("%w: %q is not a note this vault holds", ErrInvalid, b.RelPath)
	}
	if course != NoCourse && !h.walks(course, b.RelPath) {
		return Bookmark{}, false, fmt.Errorf("%w: %q does not walk %q", ErrInvalid, course, b.RelPath)
	}
	return h.store.MoveBookmark(course, b, expect)
}

// walks says whether the course walks rel, on its main line or on a side
// branch.
func (h *Handler) walks(course, rel string) bool {
	mainLine, branches, ok := h.places.Units(course)
	return ok && (slices.Contains(mainLine, rel) || slices.Contains(branches, rel))
}

// courseOf reads the course a request names in its query: none, for reading
// outside any course, or one. It is false for a request that names two, which
// no page makes.
func courseOf(r *http.Request) (string, bool) {
	named := r.URL.Query()["course"]
	switch len(named) {
	case 0:
		return NoCourse, true
	case 1:
		return named[0], true
	default:
		return NoCourse, false
	}
}

// questionOf is the whole-section question a path and an anchor name, the mark
// the question button makes and takes away.
func questionOf(path, anchor string) machinemarks.Mark {
	return machinemarks.Mark{Kind: machinemarks.Question, Path: path, Anchor: anchor}
}

// isSectionQuestion reports whether m is a question of a whole section.
func isSectionQuestion(m *machinemarks.Mark) bool {
	return m.Kind == machinemarks.Question && m.Quote == ""
}

// getQuestions answers the questions of whole sections kept now, as the page
// draws them: the note, the anchor and the day.
func (h *Handler) getQuestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	view, err := h.vaultMarks.View()
	if err != nil {
		h.log.Error("read the questions", "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(origin.Language(r)), http.StatusInternalServerError)
		return
	}
	type entry struct {
		Path   string    `json:"path"`
		Anchor string    `json:"anchor"`
		Time   time.Time `json:"time"`
	}
	pending, err := h.vaultMarks.Pending(view)
	if err != nil {
		h.log.Error("read the questions", "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(origin.Language(r)), http.StatusInternalServerError)
		return
	}
	entries := []entry{}
	for _, e := range view.Entries(AllMarks) {
		if m := e.Mark(); !e.Deleted() && isSectionQuestion(&m) {
			entries = append(entries, entry{Path: m.Path, Anchor: m.Anchor, Time: m.At})
		}
	}
	for i := range pending {
		if isSectionQuestion(&pending[i]) {
			entries = append(entries, entry{Path: pending[i].Path, Anchor: pending[i].Anchor, Time: pending[i].At})
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(entries); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the questions", "error", err)
	}
}

// postQuestion marks the section this submission names as a question, or
// removes the mark when it already is one. Both are one step under the vault's
// lock. A question that two computers changed differently is not taken away
// blind: it is answered 409 with what is read now, and the reader sees it before
// the next press.
func (h *Handler) postQuestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.MarkFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	if h.refuseWrite(w, r) {
		return
	}
	candidate := questionOf(r.PostForm.Get("path"), r.PostForm.Get("anchor"))
	candidate.ID, candidate.At = newID(), stamp()
	marks, err := h.vaultMarks.Marks()
	if err != nil {
		h.refuse(w, r, err, "mark a question")
		return
	}
	same := func(m *machinemarks.Mark) bool {
		return isSectionQuestion(m) && m.Path == candidate.Path && m.Anchor == candidate.Anchor
	}
	admit := func(m *machinemarks.Mark) bool { return h.places.HasPlace(m.Path, m.Anchor) }
	out, err := marks.Toggle(writeContext(r), &candidate, same, admit)
	err = h.landed(err, "mark a question")
	switch {
	case err == nil:
		// Marked is whether the section is marked now: a question made by
		// another computer that this one had not read stays beside the one just
		// taken away, and is taken away by pressing again.
		h.reply(w, r, struct {
			Marked bool `json:"marked"`
		}{Marked: !out.Mark.Deleted || out.Remaining > 0}, "send the question result")
	case errors.Is(err, machinemarks.ErrInvalidMark):
		h.log.Warn("a question was refused", "error", err)
		http.Error(w, wording.QuestionRefused.In(lang), http.StatusUnprocessableEntity)
	default:
		h.refuse(w, r, err, "mark a question")
	}
}
