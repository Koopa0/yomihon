package mark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// The reasons a 409 gives, which the page tells apart: a page that is old, a
// send that came after a newer send of the same edit, and a mark that is not
// there.
const (
	reasonStale      = "stale"
	reasonSuperseded = "superseded"
	reasonGone       = "gone"
)

// placedMark is a mark as a route answers with it: the mark as it is kept, where
// it stands in its note now, and what the page now has read of it, which is what
// a change made from it says it was made on top of. Placement is the server's
// word and is not read from what comes in.
type placedMark struct {
	machinemarks.Mark

	Placement string            `json:"placement"`
	Seen      machinemarks.Seen `json:"seen"`
	// Note is the mark as the reading page draws it in the margin, in the words
	// of the page's language, for a page that has just kept or changed it. It is
	// left out of a mark that is deleted, and where nothing draws one.
	Note string `json:"note,omitempty"`
}

// NoteDrawer draws a mark as the reading page draws it beside its text. The page
// that answers for a note is the one that knows how, so the routes ask it: a
// page that drew the same mark twice, once on arrival and once in a script,
// would be free to draw two different things.
type NoteDrawer interface {
	// Note is the margin note of m as a fragment of markup, or empty where the
	// mark has none to draw: a highlight on a passage is drawn in the text alone.
	Note(m *machinemarks.Mark, placement string, lang wording.Lang) string
}

// keepRequest is what a page sends to leave a mark. The id, the time and the
// identity are not in it: the server makes them.
type keepRequest struct {
	// ID is an id the page made for this mark, 32 lowercase hex digits, and is
	// optional. A page that sends one can send the same request again when an
	// answer never reached it: the mark with that id is the answer, and nothing
	// is kept twice.
	ID     string            `json:"id"`
	Kind   machinemarks.Kind `json:"kind"`
	Path   string            `json:"path"`
	Anchor string            `json:"anchor"`
	Quote  string            `json:"quote"`
	Before string            `json:"before"`
	After  string            `json:"after"`
	Body   string            `json:"body"`
}

// reviseRequest is what a page sends to change a mark: what it had read of the
// mark, which editing session this send is of, and the words or the resolution
// that are new. A field it leaves out is left as it is, except seen, which is
// what the change says it was made on top of and is not optional.
type reviseRequest struct {
	Seen *machinemarks.Seen `json:"seen"`
	Edit *machinemarks.Edit `json:"edit"`
	// Kind is the state the mark is turned into. A mark keeps its id and its
	// words through a change of state; what the new state does not hold is
	// refused.
	Kind     *machinemarks.Kind `json:"kind"`
	Body     *string            `json:"body"`
	Resolved *bool              `json:"resolved"`
}

// baseRequest is what a page sends to delete a mark, or to undo that.
type baseRequest struct {
	Seen *machinemarks.Seen `json:"seen"`
	Edit *machinemarks.Edit `json:"edit"`
}

// conflictBody is what a 409 says.
type conflictBody struct {
	Reason string            `json:"reason"`
	Seen   machinemarks.Seen `json:"seen,omitempty"`
}

// refuseWrite answers a write the reader folder cannot take, which a page does
// not offer and a request that was made by hand is told: 422, in the words every
// refused mark is told, and why in the log, not in the answer.
func (h *Handler) refuseWrite(w http.ResponseWriter, r *http.Request) bool {
	why := h.kept.Writable()
	if why == WritableOK {
		return false
	}
	h.log.Warn("a mark was refused: the reader folder cannot be written to", "why", why, "directory", h.store.Dir())
	http.Error(w, wording.MarkNotKept.In(origin.Language(r)), http.StatusUnprocessableEntity)
	return true
}

// keepMark keeps a mark of any kind, at a place in a note this vault holds and on
// words the note has. A highlight or a question already kept in that place is the
// answer, and is not kept twice; a thought is always new.
func (h *Handler) keepMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	var request keepRequest
	if !h.decode(w, r, &request, maxMarkBytes) {
		return
	}
	if h.refuseWrite(w, r) {
		return
	}
	page, readable := h.places.Reading(request.Path, lang)
	if !readable {
		h.refuse(w, r, fmt.Errorf("%w: not a note this vault holds", machinemarks.ErrInvalidMark), "keep a mark")
		return
	}
	id := request.ID
	if id == "" {
		var err error
		if id, err = newID(); err != nil {
			h.refuse(w, r, err, "keep a mark")
			return
		}
	} else if !isID(id) {
		h.refuse(w, r, fmt.Errorf("%w: the id is not 32 lowercase hex digits", machinemarks.ErrInvalidMark), "keep a mark")
		return
	}
	if request.ID != "" && h.answerKept(w, r, id) {
		return
	}
	candidate := machinemarks.Mark{
		ID: id, Kind: request.Kind, Path: request.Path, Anchor: request.Anchor,
		Quote: request.Quote, Before: request.Before, After: request.After,
		Identity: page.Identity, Body: request.Body, At: time.Now(),
	}
	if err := askedForWhatKindHolds(&candidate); err != nil {
		h.refuse(w, r, err, "keep a mark")
		return
	}
	marks, err := h.kept.Marks()
	if err != nil {
		h.refuse(w, r, err, "keep a mark")
		return
	}
	admit := func(m *machinemarks.Mark) bool { return h.standsIn(m, &page) }
	var res machinemarks.Result
	var stored machinemarks.Mark
	status := http.StatusCreated
	if candidate.Kind == machinemarks.Thought {
		stored = candidate
		res, err = h.keepThought(writeContext(r), marks, &stored, admit)
	} else {
		var out machinemarks.Outcome
		same := func(other *machinemarks.Mark) bool { return sameAct(other, &candidate) }
		out, err = marks.Keep(writeContext(r), &candidate, same, admit, h.kept.gate)
		res, stored = out.Result, out.Mark
		if err == nil && !out.Written {
			status = http.StatusOK
		}
	}
	if err = h.landed(err, "keep a mark"); err != nil {
		// A send of the same id that was still on its way when this one came
		// has written the mark: that mark is the answer.
		if request.ID != "" && errors.Is(err, machinemarks.ErrNoBase) && h.answerKept(w, r, id) {
			return
		}
		h.refuse(w, r, err, "keep a mark")
		return
	}
	h.send(w, r, status, &stored, res.Seen, lang)
}

// answerKept answers a request to keep a mark with an id the page made, when a
// mark of that id is kept already: the same send again, which keeps nothing
// twice. It reports whether it answered.
func (h *Handler) answerKept(w http.ResponseWriter, r *http.Request, id string) bool {
	view, err := h.kept.View()
	if err != nil {
		return false
	}
	entry, ok := view.Entry(id, h.kept.gate)
	if !ok {
		return false
	}
	shown := entry.Mark()
	h.send(w, r, http.StatusOK, &shown, entry.Seen, origin.Language(r))
	return true
}

// keepThought writes a thought. A thought is always new, with an id nobody has,
// so nothing a lock would protect is read first: the place is judged before it
// is written.
func (h *Handler) keepThought(ctx context.Context, marks *machinemarks.Marks, thought *machinemarks.Mark, admit func(*machinemarks.Mark) bool) (machinemarks.Result, error) {
	if !admit(thought) {
		return machinemarks.Result{}, fmt.Errorf("%w: the place is not one this vault renders", machinemarks.ErrInvalidMark)
	}
	return marks.Put(ctx, thought, nil, nil, h.kept.gate)
}

// askedForWhatKindHolds refuses what a kind does not carry: a highlight is a
// passage and says nothing of its own, and a thought says something.
func askedForWhatKindHolds(m *machinemarks.Mark) error {
	switch {
	case m.Kind == machinemarks.Highlight && (m.Quote == "" || m.Body != ""):
		return fmt.Errorf("%w: a highlight is a passage and carries no words of its own", machinemarks.ErrInvalidMark)
	case m.Kind == machinemarks.Thought && m.Body == "":
		return fmt.Errorf("%w: a thought says something", machinemarks.ErrInvalidMark)
	}
	return nil
}

// sameAct reports whether a mark that is there already is the same act of the
// reader's as b: one kind in one place, which for a passage is the words and the
// words around them, with the same reason of their own for a question, and not one that was resolved, which asking again
// opens anew. Words that differ are a new mark: nothing the reader wrote is
// answered with an earlier mark that says something else.
func sameAct(a, b *machinemarks.Mark) bool {
	if a.Kind != b.Kind || a.Path != b.Path || a.Anchor != b.Anchor || a.Quote != b.Quote ||
		a.Body != b.Body || !a.Resolved.IsZero() {
		return false
	}
	// The same words in another place are another mark: the words around them are
	// what tell the places apart.
	return a.Quote == "" || (a.Before == b.Before && a.After == b.After)
}

// standsIn reports whether m can be kept in the note page is: the anchor is a
// place the note renders, and the words are in it exactly as chosen, at one
// place. Words the note never had are not a mark, and neither are words at
// several places that what was sent around them does not tell apart. The words
// around them are kept as they were sent, to place the words again later.
func (h *Handler) standsIn(m *machinemarks.Mark, page *Page) bool {
	if !h.places.HasPlace(m.Path, m.Anchor) {
		return false
	}
	old := fromMachine(m)
	found := Locate(page.Text, []string{m.Anchor}, &old)
	if m.Quote == "" {
		return found.Status == SectionOnly
	}
	return found.Status == Found && found.Exact
}

// readable finds the mark a request names, and answers for one that is not
// there. A request that is to change it is write. A mark the gate withholds is answered exactly as one that is not there.
func (h *Handler) readable(w http.ResponseWriter, r *http.Request, write bool) (machinemarks.Entry, bool) {
	id := r.PathValue("id")
	if !isID(id) {
		h.absent(w, r)
		return machinemarks.Entry{}, false
	}
	view, err := h.kept.View()
	if err != nil {
		h.refuse(w, r, fmt.Errorf("%w: %w", ErrUnreadable, err), "read a mark")
		return machinemarks.Entry{}, false
	}
	if write && h.kept.Holds(view, id) {
		// A change addressed to a mark that only the earlier file has takes the
		// earlier marks in first, as the first write of any kind does, and is
		// then made on what was taken in.
		if err = h.kept.TakeIn(writeContext(r)); err != nil {
			h.refuse(w, r, err, "take in the earlier marks")
			return machinemarks.Entry{}, false
		}
		if view, err = h.kept.View(); err != nil {
			h.refuse(w, r, fmt.Errorf("%w: %w", ErrUnreadable, err), "read a mark")
			return machinemarks.Entry{}, false
		}
	}
	entry, ok := view.Entry(id, h.kept.gate)
	if !ok {
		h.absent(w, r)
		return machinemarks.Entry{}, false
	}
	return entry, true
}

// getMark answers a mark as it is read now: the value that counts, which is the
// newest live one where computers changed it differently (the others stay in the
// files and are not told here), and what a page that shows it has read, which a change
// is sent on top of. A deleted mark is answered too, with deleted set, so that
// what was deleted can be put back.
func (h *Handler) getMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	entry, ok := h.readable(w, r, false)
	if !ok {
		return
	}
	lang := origin.Language(r)
	current := entry.Mark()
	answer := h.placed(&current, entry.Seen, lang)
	h.reply(w, r, &answer, "send a mark")
}

// reviseMark changes the words of a thought or of a question, or resolves or
// reopens a question. It is made on top of what the page had read, which it
// says.
func (h *Handler) reviseMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	var request reviseRequest
	if !isID(r.PathValue("id")) {
		h.absent(w, r)
		return
	}
	if !h.decode(w, r, &request, maxMarkBytes) {
		return
	}
	if request.Seen == nil {
		h.refuse(w, r, machinemarks.ErrNoBase, "change a mark")
		return
	}
	if h.refuseWrite(w, r) {
		return
	}
	entry, ok := h.readable(w, r, true)
	if !ok {
		return
	}
	// The change is made on the value the page read, which its seen says, and not
	// on the newest there is: a value that another computer wrote and the page
	// never showed is not the page's to copy over what it showed.
	page := pageValue(entry, *request.Seen)
	if entry.Deleted() || page.Mark.Deleted {
		h.refuse(w, r, machinemarks.ErrGone, "change a mark")
		return
	}
	was := page.Mark
	next, err := revised(&was, &request, time.Now())
	if err != nil {
		h.refuse(w, r, err, "change a mark")
		return
	}
	marks, err := h.kept.Marks()
	if err != nil {
		h.refuse(w, r, err, "change a mark")
		return
	}
	res, err := marks.Put(writeContext(r), &next, *request.Seen, request.Edit, h.kept.gate)
	if err = h.landed(err, "change a mark"); err != nil {
		h.refuse(w, r, err, "change a mark")
		return
	}
	h.sendNow(w, r, http.StatusOK, &next, res.Seen, lang)
}

// revised is the mark m with what a request changes of it, which only a mark of
// the state that has it may have: words are a thought's and a question's, a
// resolution a question's. A state is turned into another with its words as they
// are sent, so a thought turned into a highlight is sent with none. Resolving a
// question that is resolved already leaves the time it was resolved at;
// reopening it clears the time.
func revised(was *machinemarks.Mark, r *reviseRequest, now time.Time) (machinemarks.Mark, error) {
	m := *was
	if r.Body == nil && r.Resolved == nil && r.Kind == nil {
		return machinemarks.Mark{}, fmt.Errorf("%w: nothing to change", machinemarks.ErrInvalidMark)
	}
	if r.Kind != nil {
		switch *r.Kind {
		case machinemarks.Highlight, machinemarks.Thought, machinemarks.Question:
			m.Kind = *r.Kind
		default:
			return machinemarks.Mark{}, fmt.Errorf("%w: %q is not a state a mark is in", machinemarks.ErrInvalidMark, string(*r.Kind))
		}
	}
	if r.Resolved != nil && m.Kind != machinemarks.Question {
		return machinemarks.Mark{}, fmt.Errorf("%w: only a question is resolved", machinemarks.ErrInvalidMark)
	}
	if r.Body != nil {
		m.Body = *r.Body
	}
	if r.Resolved != nil {
		switch {
		case !*r.Resolved:
			m.Resolved = time.Time{}
		case m.Resolved.IsZero():
			m.Resolved = now
		}
	}
	if err := askedForWhatKindHolds(&m); err != nil {
		return machinemarks.Mark{}, err
	}
	return m, nil
}

// removeMark deletes a mark and answers with it as it was, deleted. The words
// stay in the mark's line, so that it can be put back.
func (h *Handler) removeMark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	if !isID(r.PathValue("id")) {
		h.absent(w, r)
		return
	}
	var request baseRequest
	if !h.decode(w, r, &request, maxMarkBytes) {
		return
	}
	if request.Seen == nil {
		h.refuse(w, r, machinemarks.ErrNoBase, "delete a mark")
		return
	}
	if h.refuseWrite(w, r) {
		return
	}
	entry, ok := h.readable(w, r, true)
	if !ok {
		return
	}
	marks, err := h.kept.Marks()
	if err != nil {
		h.refuse(w, r, err, "delete a mark")
		return
	}
	res, err := marks.Delete(writeContext(r), entry.Mark().ID, *request.Seen, request.Edit, h.kept.gate)
	if err = h.landed(err, "delete a mark"); err != nil {
		h.refuse(w, r, err, "delete a mark")
		return
	}
	gone := pageValue(entry, *request.Seen).Mark
	gone.Deleted = true
	h.sendNow(w, r, http.StatusOK, &gone, res.Seen, lang)
}

// putBack puts a mark that was deleted back, as it was deleted: its words are
// those of the line that deleted it. A mark that is not deleted is answered as
// it is and nothing is written, so that putting back twice is putting back once.
func (h *Handler) putBack(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	if !isID(r.PathValue("id")) {
		h.absent(w, r)
		return
	}
	var request baseRequest
	if !h.decode(w, r, &request, maxMarkBytes) {
		return
	}
	if request.Seen == nil {
		h.refuse(w, r, machinemarks.ErrNoBase, "put a mark back")
		return
	}
	if h.refuseWrite(w, r) {
		return
	}
	entry, ok := h.readable(w, r, true)
	if !ok {
		return
	}
	// What is put back is the value the page read deleted, which its seen says.
	// A page that read a value that is not deleted has nothing to put back, and is
	// answered with the mark as it is now.
	current := pageValue(entry, *request.Seen).Mark
	if !current.Deleted {
		shown := entry.Mark()
		h.send(w, r, http.StatusOK, &shown, entry.Seen, lang)
		return
	}
	marks, err := h.kept.Marks()
	if err != nil {
		h.refuse(w, r, err, "put a mark back")
		return
	}
	current.Deleted = false
	res, err := marks.Put(writeContext(r), &current, *request.Seen, request.Edit, h.kept.gate)
	if err = h.landed(err, "put a mark back"); err != nil {
		h.refuse(w, r, err, "put a mark back")
		return
	}
	h.sendNow(w, r, http.StatusOK, &current, res.Seen, lang)
}

// landed says a change whose line is in the ledger is kept, whether or not the
// vault took the file just now: the ledger is the commit point, the mark is read
// from it at once, and the next page that reads the marks puts it in the vault.
// The failure is the log's and not the reader's, and a retry is not asked for,
// which would keep a thought twice. Any other error is returned as it is.
func (h *Handler) landed(err error, doing string) error {
	if errors.Is(err, machinemarks.ErrNotInVault) {
		h.log.Warn("a mark is kept in the local backup and not yet in the vault", "doing", doing, "error", err)
		return nil
	}
	return err
}

// pageValue is the value of a mark that a page showed, which is the first value
// its seen covers; a seen that covers none is a page that read nothing in
// particular, and gets the value that counts.
func pageValue(entry machinemarks.Entry, seen machinemarks.Seen) machinemarks.Value {
	for i := range entry.Values {
		if entry.Values[i].Rev <= seen[entry.Values[i].Writer] {
			return entry.Values[i]
		}
	}
	return entry.Values[0]
}

// sendNow answers a change with the mark as the reader will see it now, which is
// the newest live value of it: where another computer's value, which the page had
// not read, is newer or live, that is what the reader sees, not the value that
// was just written. fallback is what is answered when the mark cannot be read
// back.
func (h *Handler) sendNow(w http.ResponseWriter, r *http.Request, status int, fallback *machinemarks.Mark, seen machinemarks.Seen, lang wording.Lang) {
	if view, err := h.kept.View(); err == nil {
		if entry, ok := view.Entry(fallback.ID, h.kept.gate); ok {
			shown := entry.Mark()
			// What the page has read is what it is shown: the value that counts is
			// covered, so that the next change is built on it, and no value that is
			// not shown is.
			if top := &entry.Values[0]; seen[top.Writer] < top.Rev {
				seen = maps.Clone(seen)
				seen[top.Writer] = top.Rev
			}
			h.send(w, r, status, &shown, seen, lang)
			return
		}
	}
	h.send(w, r, status, fallback, seen, lang)
}

// statusOf is the one place a refusal of the marks becomes a status and, for a
// 409, the reason a page tells apart.
func statusOf(err error) (status int, reason string, seen machinemarks.Seen) {
	switch {
	case errors.Is(err, machinemarks.ErrNoBase):
		return http.StatusBadRequest, "", nil
	case errors.Is(err, machinemarks.ErrStale):
		if stale, ok := errors.AsType[*machinemarks.StaleError](err); ok {
			return http.StatusConflict, reasonStale, stale.Seen
		}
		return http.StatusConflict, reasonStale, machinemarks.Seen{}
	case errors.Is(err, machinemarks.ErrSuperseded):
		return http.StatusConflict, reasonSuperseded, nil
	case errors.Is(err, machinemarks.ErrGone):
		return http.StatusConflict, reasonGone, nil
	case errors.Is(err, machinemarks.ErrInvalidMark), errors.Is(err, machinemarks.ErrLimit):
		return http.StatusUnprocessableEntity, "", nil
	default:
		return http.StatusInternalServerError, "", nil
	}
}

// refuse answers for a write that did not happen, or a read that could not be
// made, saying no more than the reader can act on.
func (h *Handler) refuse(w http.ResponseWriter, r *http.Request, err error, doing string) {
	lang := origin.Language(r)
	status, reason, seen := statusOf(err)
	switch {
	case reason != "":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		body := conflictBody{Reason: reason, Seen: seen}
		if encodeErr := json.NewEncoder(w).Encode(&body); encodeErr != nil {
			h.log.Log(r.Context(), origin.WriteFailureLevel(r, encodeErr), "send a conflict", "error", encodeErr)
		}
	case status == http.StatusBadRequest:
		http.Error(w, wording.RequestUnreadable.In(lang), status)
	case status == http.StatusUnprocessableEntity:
		h.log.Warn("a mark was refused", "doing", doing, "error", err)
		http.Error(w, wording.MarkNotKept.In(lang), status)
	case errors.Is(err, ErrUnreadable), errors.Is(err, machinemarks.ErrOwnFileUnreadable), errors.Is(err, machinemarks.ErrLegacy):
		h.log.Error("read the marks", "doing", doing, "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(lang), status)
	default:
		h.log.Error("write the marks", "doing", doing, "directory", h.store.Dir(), "error", err)
		http.Error(w, wording.QuestionNotStored.In(lang), status)
	}
}

// send answers with m, where it stands in its note now, and what the page has
// read of it.
func (h *Handler) send(w http.ResponseWriter, r *http.Request, status int, m *machinemarks.Mark, seen machinemarks.Seen, lang wording.Lang) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	placed := h.placed(m, seen, lang)
	if err := json.NewEncoder(w).Encode(&placed); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send a mark", "error", err)
	}
}

// placed is m as a route answers with it.
func (h *Handler) placed(m *machinemarks.Mark, seen machinemarks.Seen, lang wording.Lang) placedMark {
	answer := placedMark{Mark: *m, Placement: h.placement(m, lang), Seen: seen}
	if drawer, ok := h.places.(NoteDrawer); ok && !m.Deleted {
		answer.Note = drawer.Note(m, answer.Placement, lang)
	}
	return answer
}

// placement says where m stands in its note as the note is now.
func (h *Handler) placement(m *machinemarks.Mark, lang wording.Lang) string {
	page, readable := h.places.Reading(m.Path, lang)
	if !readable {
		return placedChanged
	}
	var sections []string
	if h.places.HasPlace(m.Path, m.Anchor) {
		sections = []string{m.Anchor}
	}
	old := fromMachine(m)
	switch Locate(page.Text, sections, &old).Status {
	case Found:
		return placedFound
	case SectionOnly:
		return placedSection
	default:
		return placedChanged
	}
}
