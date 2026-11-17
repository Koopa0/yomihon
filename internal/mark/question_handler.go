package mark

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// QuestionAddress is shared by the page controls and their route.
const QuestionAddress = "/questions"

// Places says whether a note is readable now and renders an anchor. An empty
// anchor names the note as a whole. Both mark routes keep a place only in a note
// this reports readable, so neither stores a path the vault never showed. The
// question route also asks about the anchor; the bookmark route holds its
// anchor to a shape only.
type Places interface {
	HasPlace(rel, anchor string) bool
}

// QuestionHandler reads and toggles the reader's accumulated location marks.
type QuestionHandler struct {
	file   *File
	places Places
	log    *slog.Logger
}

// NewQuestionHandler connects the route to the existing vault-local store.
func NewQuestionHandler(file *File, places Places, log *slog.Logger) *QuestionHandler {
	if file == nil {
		panic("mark: NewQuestionHandler requires a non-nil File")
	}
	if places == nil {
		panic("mark: NewQuestionHandler requires a non-nil Places")
	}
	if log == nil {
		panic("mark: NewQuestionHandler requires a non-nil logger")
	}
	return &QuestionHandler{file: file, places: places, log: log}
}

// Register mounts read and toggle endpoints. The serving command supplies the
// same loopback and cross-origin protections as the bookmark route.
func (h *QuestionHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+QuestionAddress, h.list)
	mux.HandleFunc("POST "+QuestionAddress, h.toggle)
	mux.HandleFunc("GET "+retiredQuestionAddress, movedTo(QuestionAddress))
	mux.HandleFunc("POST "+retiredQuestionAddress, movedTo(QuestionAddress))
}

// retiredQuestionAddress is where a question was kept before the route took
// its present name.
const retiredQuestionAddress = "/uncertainties"

func (h *QuestionHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	held, err := h.file.Questions()
	if err != nil {
		h.log.Error("read the questions", "path", h.file.QuestionPath(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(origin.Language(r)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(held); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the questions", "error", err)
	}
}

func (h *QuestionHandler) toggle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.MarkFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	kept := &Question{
		RelPath: r.PostFormValue("path"),
		Anchor:  r.PostFormValue("anchor"),
		At:      time.Now(),
	}
	marked, err := h.file.ToggleQuestion(kept, func(u *Question) bool { return h.places.HasPlace(u.RelPath, u.Anchor) })
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
	case errors.Is(err, ErrUnreadableQuestions):
		h.log.Error("read the questions", "path", h.file.QuestionPath(), "error", err)
		http.Error(w, wording.QuestionUnavailable.In(lang), http.StatusInternalServerError)
	default:
		h.log.Error("write the questions", "path", h.file.QuestionPath(), "error", err)
		http.Error(w, wording.QuestionNotStored.In(lang), http.StatusInternalServerError)
	}
}
