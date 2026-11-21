package mark

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxFormBytes bounds the body a mark arrives in: a path, an id, a number and a
// digest never come near it.
const maxFormBytes = 4096

// Paths are where the Handler answers. They are exported because the route and
// the address a reading page posts to are decided on two sides of a package
// boundary: the command reads them from here and hands them to the page, which
// renders them rather than spelling its own. An address written twice is one
// that drifts.
var Paths = struct{ Bookmark, Question, Mark string }{Bookmark: "/bookmark", Question: "/questions", Mark: "/mark"}

// retiredBookmarkPath and retiredQuestionPath are where a bookmark and a
// question were posted before the routes took their present names. A page drawn
// by the previous version may still post there.
const (
	retiredBookmarkPath = "/marks"
	retiredQuestionPath = "/uncertainties"
)

// Places says whether a note is readable now and renders an anchor, and what
// words it holds. An empty anchor names the note as a whole. The routes keep a
// mark only in a note this reports readable, so none stores a path the vault
// never showed. The question and mark routes also ask about the anchor; the
// bookmark route holds its anchor to a shape only. Reading is what a quote is
// found in, in the language the page is drawn in, because a page's own words
// (a callout's title, a footnote's label) are part of what a reader selects.
type Places interface {
	HasPlace(rel, anchor string) bool
	Reading(rel string, lang wording.Lang) (Page, bool)
}

// Handler serves the routes a mark reaches the process through, and the ones a
// page reads the kept marks back from.
//
// A page cannot write a file, so a mark travels over a route of its own on the
// same loopback listener, with a body cap and a replace-by-rename discipline.
// They are not a face an agent has any business calling: these endpoints are the
// reader's.
//
// It keeps a mark only in a note the vault holds now. The path is the one field
// of a mark that can reach the desk as text, so a path nobody's note carries is
// refused rather than stored for a later page to print. It reaches the marks
// only through a Store.
type Handler struct {
	store  *Store
	places Places
	log    *slog.Logger
}

// NewHandler wires the routes around a Store and the vault whose notes a mark
// may name.
func NewHandler(store *Store, places Places, log *slog.Logger) *Handler {
	if store == nil {
		panic("mark: NewHandler requires a non-nil Store")
	}
	if places == nil {
		panic("mark: NewHandler requires a non-nil Places")
	}
	if log == nil {
		panic("mark: NewHandler requires a non-nil logger")
	}
	return &Handler{store: store, places: places, log: log}
}

// Register mounts the routes, and the retired addresses that lead to them. The
// serving command supplies the loopback and cross-origin protections.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+Paths.Bookmark, h.getBookmark)
	mux.HandleFunc("POST "+Paths.Bookmark, h.postBookmark)
	mux.HandleFunc("POST "+retiredBookmarkPath, movedTo(Paths.Bookmark))
	mux.HandleFunc("GET "+Paths.Question, h.getQuestions)
	mux.HandleFunc("POST "+Paths.Question, h.postQuestion)
	mux.HandleFunc("GET "+retiredQuestionPath, movedTo(Paths.Question))
	mux.HandleFunc("POST "+retiredQuestionPath, movedTo(Paths.Question))
	mux.HandleFunc("POST "+Paths.Mark, h.keepMark)
	mux.HandleFunc("PUT "+Paths.Mark+"/{id}", h.putBack)
	mux.HandleFunc("PATCH "+Paths.Mark+"/{id}", h.reviseMark)
	mux.HandleFunc("DELETE "+Paths.Mark+"/{id}", h.removeMark)
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
// the note and the anchor, which is all a page needs to tell whether the place
// is in it and where, and answers no content where none is kept. The browser is
// told not to keep the answer: one that was kept would be the staleness this
// exists to remove.
//
// A file that cannot be read answers no content too, and is logged: the page
// asked on the reader's behalf, and there is nowhere to continue from.
func (h *Handler) getBookmark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	kept, found, err := h.store.Bookmark(NoCourse)
	if err != nil {
		h.log.Warn("read the bookmark", "directory", h.store.Dir(), "error", err)
	}
	if err != nil || !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(struct {
		Path   string `json:"path"`
		Anchor string `json:"anchor"`
	}{Path: kept.RelPath, Anchor: kept.Anchor}); err != nil {
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
		http.Error(w, wording.MarkRefused.In(lang), http.StatusUnprocessableEntity)
		return
	}
	kept := &Bookmark{
		RelPath:  r.PostForm.Get("path"),
		Anchor:   r.PostForm.Get("anchor"),
		Offset:   offset,
		Identity: r.PostForm.Get("identity"),
		At:       time.Now(),
	}
	switch err = h.keepBookmark(kept); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrInvalid):
		// The reader wrote none of these fields, so the refusal is a fault in
		// the page rather than something they can correct. It is logged where
		// it can be acted on and answered with the one sentence they can use.
		h.log.Warn("a bookmark was refused", "error", err)
		http.Error(w, wording.MarkRefused.In(lang), http.StatusUnprocessableEntity)
	default:
		h.log.Error("write the bookmark", "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.MarkNotStored.In(lang), http.StatusInternalServerError)
	}
}

// keepBookmark stores kept when its shape is one these files hold and its note
// is one the vault holds. The shape is judged first, so a path no note could
// carry never reaches the vault to be looked up.
//
// Only the note is asked after: the empty anchor names it as a whole. An anchor
// is any id the rendered body carries — a block address or a footnote as readily
// as a heading — and the vault's answer for an anchor covers headings alone, so
// asking it about the anchor would refuse a place a reading page honestly
// stamps. The anchor stays bounded by its shape, and it only ever reaches the
// desk inside an address.
func (h *Handler) keepBookmark(kept *Bookmark) error {
	if err := validateBookmark(NoCourse, kept); err != nil {
		return err
	}
	if !h.places.HasPlace(kept.RelPath, "") {
		return fmt.Errorf("%w: %q is not a note this vault holds", ErrInvalid, kept.RelPath)
	}
	return h.store.SetBookmark(NoCourse, kept)
}

// getQuestions answers the questions kept now, as the page draws them: the note,
// the anchor and the day.
func (h *Handler) getQuestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	held, err := h.store.List(Question)
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
	entries := make([]entry, len(held))
	for i := range held {
		entries[i] = entry{Path: held[i].RelPath, Anchor: held[i].Anchor, Time: held[i].At}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(entries); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the questions", "error", err)
	}
}

// postQuestion marks the section this submission names as a question, or
// removes the mark when it already is one.
func (h *Handler) postQuestion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.MarkFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	kept := &Mark{
		Kind:    Question,
		RelPath: r.PostForm.Get("path"),
		Anchor:  r.PostForm.Get("anchor"),
		At:      time.Now(),
	}
	marked, err := h.store.Toggle(kept, func(m *Mark) bool { return h.places.HasPlace(m.RelPath, m.Anchor) })
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err = json.NewEncoder(w).Encode(struct {
			Marked bool `json:"marked"`
		}{Marked: marked}); err != nil {
			h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the question result", "error", err)
		}
	case errors.Is(err, ErrInvalid):
		h.log.Warn("a question was refused", "error", err)
		http.Error(w, wording.QuestionRefused.In(lang), http.StatusUnprocessableEntity)
	case errors.Is(err, ErrUnreadable):
		h.log.Error("read the questions", "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(lang), http.StatusInternalServerError)
	default:
		h.log.Error("write the questions", "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionNotStored.In(lang), http.StatusInternalServerError)
	}
}
