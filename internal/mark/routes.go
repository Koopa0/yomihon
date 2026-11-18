package mark

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxMarkBytes bounds the body a mark arrives in: the sum of what one may hold
// (a quote, the words around it, a reason, a thought) and the names around
// them. A body past it was not written by a reading page.
const maxMarkBytes = 64 << 10

// The placements a route tells a mark has in its note as the note is now, which
// are Locate's three answers: the words are there, the mark is of a section and
// the section is there, or neither is.
const (
	placedFound   = "found"
	placedSection = "section"
	placedChanged = "changed"
)

// clockSkew is how far ahead of this machine's clock a mark put back may say it
// was made. A page's clock and the server's are the same machine's; this is
// for a time that was not made by either.
const clockSkew = time.Minute

// Page is what a route needs of a note to place a mark in it: the words a
// reader sees, and the identity of the bytes they came from.
type Page struct {
	// Text is the note's words as a selection of the page holds them, which is
	// what Locate is given.
	Text string
	// Identity is the content identity of the note's bytes, as lowercase hex.
	Identity string
}

// placedMark is a mark as a route answers with it and is sent it back: the mark
// as it is kept, and where it stands in its note now. Placement is the
// server's word and is not read from what comes in.
type placedMark struct {
	Mark

	Placement string `json:"placement"`
}

// keepRequest is what a page sends to leave a mark. The id, the time and the
// identity are not in it: the server makes them.
type keepRequest struct {
	Kind   Kind   `json:"kind"`
	Path   string `json:"path"`
	Anchor string `json:"anchor"`
	Quote  string `json:"quote"`
	Before string `json:"before"`
	After  string `json:"after"`
	Reason string `json:"reason"`
	Body   string `json:"body"`
}

// reviseRequest is what a page sends to change a mark. A field it leaves out is
// left as it is.
type reviseRequest struct {
	Resolved *bool   `json:"resolved"`
	Reason   *string `json:"reason"`
	Body     *string `json:"body"`
}

// keepMark keeps a mark of any kind, at a place in a note this vault holds and
// on words the note has. A mark already kept in that place is the answer, and
// is not kept twice.
func (h *Handler) keepMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	var request keepRequest
	if !h.decode(w, r, &request) {
		return
	}
	page, readable := h.places.Reading(request.Path, lang)
	if !readable {
		h.refuse(w, r, ErrInvalid, "keep a mark")
		return
	}
	candidate := Mark{
		Kind:     request.Kind,
		RelPath:  request.Path,
		Anchor:   request.Anchor,
		Quote:    request.Quote,
		Before:   request.Before,
		After:    request.After,
		Reason:   request.Reason,
		Body:     request.Body,
		Identity: page.Identity,
		At:       time.Now(),
	}
	kept, added, err := h.store.Keep(&candidate, func(m *Mark) bool { return h.standsIn(m, &page) })
	if err != nil {
		h.refuse(w, r, err, "keep a mark")
		return
	}
	status := http.StatusOK
	if added {
		status = http.StatusCreated
	}
	h.send(w, r, status, &kept, lang)
}

// standsIn reports whether m can be kept in the note page is: the anchor is a
// place the note renders, and the words are in it exactly as chosen. Words the
// note never had are not a mark. The words around them are kept as they were
// sent: they only choose between several places the words are in, later.
func (h *Handler) standsIn(m *Mark, page *Page) bool {
	if !h.places.HasPlace(m.RelPath, m.Anchor) {
		return false
	}
	found := Locate(page.Text, []string{m.Anchor}, m)
	if m.Quote == "" {
		return found.Status == SectionOnly
	}
	return found.Status == Found && found.Exact
}

// putBack puts a mark that was removed back, as it was: the id, the time and
// the identity are the ones it had. The words are not asked to be found, because
// a note that changed while the mark was away is the changed state of a mark
// the reader still has.
func (h *Handler) putBack(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	id := r.PathValue("id")
	if !isID(id) {
		h.absent(w, r)
		return
	}
	var request placedMark
	if !h.decode(w, r, &request) {
		return
	}
	restored := request.Mark
	if restored.ID != id || restored.At.After(time.Now().Add(clockSkew)) || !h.places.HasPlace(restored.RelPath, restored.Anchor) {
		h.refuse(w, r, ErrInvalid, "put a mark back")
		return
	}
	kept, err := h.store.Add(&restored)
	if err != nil {
		h.refuse(w, r, err, "put a mark back")
		return
	}
	h.send(w, r, http.StatusOK, &kept, lang)
}

// reviseMark resolves or reopens a question, or changes its reason or a
// thought's words.
func (h *Handler) reviseMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	id := r.PathValue("id")
	if !isID(id) {
		h.absent(w, r)
		return
	}
	var request reviseRequest
	if !h.decode(w, r, &request) {
		return
	}
	revised, found, err := h.store.Revise(id, Revision{Reason: request.Reason, Body: request.Body, Resolved: request.Resolved}, time.Now())
	switch {
	case err != nil:
		h.refuse(w, r, err, "change a mark")
	case !found:
		h.absent(w, r)
	default:
		h.send(w, r, http.StatusOK, &revised, lang)
	}
}

// removeMark removes a mark and answers with it as it was, which is what putting
// it back is sent.
func (h *Handler) removeMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	id := r.PathValue("id")
	if !isID(id) {
		h.absent(w, r)
		return
	}
	removed, found, err := h.store.Remove(id)
	switch {
	case err != nil:
		h.refuse(w, r, err, "remove a mark")
	case !found:
		h.absent(w, r)
	default:
		h.send(w, r, http.StatusOK, &removed, lang)
	}
}

// decode reads the one JSON object a write is, and answers for a request that
// is not one: a form or text, which is all a page of another origin can send
// without the preflight the cross-origin protection refuses; a body past the
// cap; a field nobody knows; a second value after the first.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, into any) bool {
	lang := origin.Language(r)
	if kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || kind != "application/json" {
		http.Error(w, wording.MarkRequestUnreadable.In(lang), http.StatusUnsupportedMediaType)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxMarkBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(into)
	if err == nil {
		if _, more := decoder.Token(); !errors.Is(more, io.EOF) {
			err = errors.New("a second value follows the first")
		}
	}
	if err != nil {
		status := http.StatusBadRequest
		if _, big := errors.AsType[*http.MaxBytesError](err); big {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, wording.MarkRequestUnreadable.In(lang), status)
		return false
	}
	return true
}

// absent answers for an id that names no mark, whether it is not an id at all
// or is one nothing holds.
func (h *Handler) absent(w http.ResponseWriter, r *http.Request) {
	http.Error(w, wording.MarkNotFound.In(origin.Language(r)), http.StatusNotFound)
}

// refuse answers for a write that did not happen, saying no more than the
// reader can act on: the mark was refused, or the file could not be read, or
// could not be written.
func (h *Handler) refuse(w http.ResponseWriter, r *http.Request, err error, doing string) {
	lang := origin.Language(r)
	switch {
	case errors.Is(err, ErrExists):
		http.Error(w, wording.MarkNotKept.In(lang), http.StatusConflict)
	case errors.Is(err, ErrInvalid):
		h.log.Warn("a mark was refused", "doing", doing, "error", err)
		http.Error(w, wording.MarkNotKept.In(lang), http.StatusUnprocessableEntity)
	case errors.Is(err, ErrUnreadable):
		h.log.Error("read the marks", "doing", doing, "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(lang), http.StatusInternalServerError)
	default:
		h.log.Error("write the marks", "doing", doing, "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionNotStored.In(lang), http.StatusInternalServerError)
	}
}

// send answers with m and where it stands in its note now.
func (h *Handler) send(w http.ResponseWriter, r *http.Request, status int, m *Mark, lang wording.Lang) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(placedMark{Mark: *m, Placement: h.placement(m, lang)}); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send a mark", "error", err)
	}
}

// placement says where m stands in its note as the note is now.
func (h *Handler) placement(m *Mark, lang wording.Lang) string {
	page, readable := h.places.Reading(m.RelPath, lang)
	if !readable {
		return placedChanged
	}
	var sections []string
	if h.places.HasPlace(m.RelPath, m.Anchor) {
		sections = []string{m.Anchor}
	}
	switch Locate(page.Text, sections, m).Status {
	case Found:
		return placedFound
	case SectionOnly:
		return placedSection
	default:
		return placedChanged
	}
}
