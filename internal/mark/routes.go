package mark

import (
	"encoding/json"
	"errors"
	"fmt"
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
// are Locate's three answers: the words are there, at one place, the mark is of
// a section and the section is there, or neither is.
const (
	placedFound   = "found"
	placedSection = "section"
	placedChanged = "changed"
)

// maxBookmarkBytes bounds the body a bookmark arrives in: a course, a note, an
// anchor, a number, a digest and a time never come near it.
const maxBookmarkBytes = 4 << 10

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

// bookmarkRequest is what a page sends to set the bookmark of a course. At is
// sent only to put back a bookmark the page was handed; otherwise the time is
// the server's.
type bookmarkRequest struct {
	Course   string     `json:"course"`
	Path     string     `json:"path"`
	Anchor   string     `json:"anchor"`
	Offset   int        `json:"offset"`
	Identity string     `json:"identity"`
	At       *time.Time `json:"at"`
	Replaces replaces   `json:"replaces"`
}

// replaces is what a page that puts a bookmark back says it expects to
// replace: the bookmark set at a time, or null where it expects the course to
// have none, as it has after the page cleared one. A request that leaves it out
// expects nothing in particular. It is a type of its own because JSON null and
// an absent field must be told apart, and a pointer cannot.
type replaces struct {
	said bool
	at   *time.Time
}

func (r *replaces) UnmarshalJSON(data []byte) error {
	r.said = true
	if string(data) == "null" {
		return nil
	}
	var at time.Time
	if err := json.Unmarshal(data, &at); err != nil {
		return err
	}
	r.at = &at
	return nil
}

// movedBookmark is what setting a bookmark answers: the bookmark now kept and
// the one it replaced, which is what the page says it moved from and sets back.
// Previous is null where the course had none.
type movedBookmark struct {
	Bookmark Bookmark  `json:"bookmark"`
	Previous *Bookmark `json:"previous"`
}

// finishedNote is what finishing or reopening a note answers. At is the time
// the reader first said it was finished and is absent where it is not. Course
// is the course the request named, as it stands now, and is absent where it
// named none.
type finishedNote struct {
	Path     string        `json:"path"`
	Finished bool          `json:"finished"`
	At       *time.Time    `json:"at,omitempty"`
	Course   *courseCounts `json:"course,omitempty"`
}

// courseCounts is the rows on a course's main line, those not yet written
// included, and how many of the rows the reader has finished. A note a course
// lists twice is two rows. A side branch is in neither: it is not part of what
// the course plans.
type courseCounts struct {
	Path     string `json:"path"`
	Units    int    `json:"units"`
	Finished int    `json:"finished"`
}

// putBookmark sets the bookmark of a course, or of reading outside any, and
// answers it with the one it replaced. A bookmark set over another in the same
// course is the move, and putting the answered one back is the undo: it is sent
// with its time, and the file is as it was, unless the page that moved it
// away is not the last to have changed it: an undo names the bookmark it
// expects to replace, and is refused with 409 where another is kept.
func (h *Handler) putBookmark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request bookmarkRequest
	if !h.decode(w, r, &request, maxBookmarkBytes) {
		return
	}
	now := stamp()
	b := &Bookmark{
		RelPath: request.Path, Anchor: request.Anchor, Offset: request.Offset,
		Identity: request.Identity, At: now,
	}
	if request.At != nil {
		b.At = request.At.UTC().Truncate(time.Millisecond)
		if b.At.IsZero() || b.At.After(now.Add(clockSkew)) {
			h.refuseBookmark(w, r, fmt.Errorf("%w: the time is not one a bookmark was kept at", ErrInvalid), "put a bookmark")
			return
		}
	}
	var expect *Expectation
	if request.Replaces.said {
		expect = &Expectation{At: request.Replaces.at}
	}
	previous, had, err := h.setBookmark(request.Course, b, expect)
	if err != nil {
		h.refuseBookmark(w, r, err, "put a bookmark")
		return
	}
	moved := movedBookmark{Bookmark: *b}
	if had {
		moved.Previous = &previous
	}
	h.reply(w, r, &moved, "send the bookmark")
}

// removeBookmark clears the bookmark of a course, or of reading outside any,
// and answers it as it was, which is what putting it back is sent. It does not
// ask the vault: a bookmark of a course or a note that is gone is the reader's to
// clear.
func (h *Handler) removeBookmark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	course, ok := courseOf(r)
	if !ok {
		h.refuseBookmark(w, r, fmt.Errorf("%w: more than one course", ErrInvalid), "clear a bookmark")
		return
	}
	removed, had, err := h.store.ClearBookmark(course)
	switch {
	case err != nil:
		h.refuseBookmark(w, r, err, "clear a bookmark")
	case !had:
		http.Error(w, wording.BookmarkNotFound.In(origin.Language(r)), http.StatusNotFound)
	default:
		h.reply(w, r, &removed, "send the bookmark")
	}
}

// finish says a note is finished, or says it is not, and answers how the course
// it is read in stands. Saying again what is so is not a refusal and changes
// nothing: a note finished twice keeps the time of the first.
func (h *Handler) finish(finished bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		lang := origin.Language(r)
		rel := r.PathValue("path")
		course, ok := courseOf(r)
		switch {
		case !ok || validatePath(rel) != nil:
			h.refuseFinished(w, r, ErrInvalid, finished, "name a note")
			return
		case !finished:
			// Saying a note is not finished asks only for the shape of what is
			// named: the note may have been deleted or renamed since, and the
			// entry is the reader's to take back. Saying it again changes nothing.
			if course != NoCourse && validatePath(course) != nil {
				h.refuseFinished(w, r, ErrInvalid, finished, "name a course")
				return
			}
		case !h.places.HasPlace(rel, ""):
			http.Error(w, wording.NoteNotFound.In(lang), http.StatusNotFound)
			return
		case course != NoCourse && !h.walks(course, rel):
			h.refuseFinished(w, r, ErrInvalid, finished, "name a course that walks the note")
			return
		}
		if err := h.store.SetFinished(rel, finished); err != nil {
			h.refuseFinished(w, r, err, finished, "finish a note")
			return
		}
		held, err := h.store.FinishedNotes()
		if err != nil {
			h.refuseFinished(w, r, err, finished, "read the finished notes")
			return
		}
		answer := h.standing(rel, course, held)
		h.reply(w, r, &answer, "send the finished note")
	}
}

// standing is how a note and the course it is read in stand, given the notes
// the reader has finished: whether the note is, and when, and for a course the
// rows on its main line and how many of those are.
func (h *Handler) standing(rel, course string, held []Finished) finishedNote {
	answer := finishedNote{Path: rel}
	for i := range held {
		if held[i].RelPath == rel {
			answer.Finished, answer.At = true, &held[i].At
		}
	}
	if course == NoCourse {
		return answer
	}
	planned, rows, _ := h.places.Rows(course)
	answer.Course = &courseCounts{Path: course, Units: planned}
	done := make(map[string]bool, len(held))
	for i := range held {
		done[held[i].RelPath] = true
	}
	for _, row := range rows {
		if done[row] {
			answer.Course.Finished++
		}
	}
	return answer
}

// refuseFinished answers for a note whose being finished could not be said or
// read. A file that cannot be read is the same answer as one that cannot be
// written: the reader is told it was not saved, and the file is left for them.
func (h *Handler) refuseFinished(w http.ResponseWriter, r *http.Request, err error, finished bool, doing string) {
	lang := origin.Language(r)
	if errors.Is(err, ErrInvalid) {
		h.log.Warn("a finished note was refused", "doing", doing, "error", err)
		refused := wording.FinishedRefused
		if !finished {
			refused = wording.FinishedUndoRefused
		}
		http.Error(w, refused.In(lang), http.StatusUnprocessableEntity)
		return
	}
	h.log.Error("write the finished notes", "doing", doing, "directory", h.store.Dir(), "error", err)
	http.Error(w, wording.FinishedNotStored.In(lang), http.StatusInternalServerError)
}

// reply sends what a route answers as JSON.
func (h *Handler) reply(w http.ResponseWriter, r *http.Request, answer any, doing string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(answer); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), doing, "error", err)
	}
}

// decode reads the one JSON object a write is, and answers for a request that
// is not one: a form or text, which is all a page of another origin can send
// without the preflight the cross-origin protection refuses; a body past the
// limit; a field nobody knows; a second value after the first.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, into any, limit int64) bool {
	lang := origin.Language(r)
	if kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || kind != "application/json" {
		http.Error(w, wording.RequestUnreadable.In(lang), http.StatusUnsupportedMediaType)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
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
		http.Error(w, wording.RequestUnreadable.In(lang), status)
		return false
	}
	return true
}

// absent answers for an id that names no mark, whether it is not an id at all
// or is one nothing holds.
func (h *Handler) absent(w http.ResponseWriter, r *http.Request) {
	http.Error(w, wording.MarkNotFound.In(origin.Language(r)), http.StatusNotFound)
}
