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

// UncertaintyAddress is shared by the page controls and their route.
const UncertaintyAddress = "/uncertainties"

// Places says whether a note is readable now and renders an anchor. An empty
// anchor names the note as a whole. Both mark routes accept a place only for a
// note it names, so a request cannot store text the vault never showed; the
// uncertainty route asks about the anchor as well, the continuation route about
// the note alone.
type Places interface {
	HasPlace(rel, anchor string) bool
}

// UncertaintyHandler reads and toggles the reader's accumulated location marks.
type UncertaintyHandler struct {
	file   *File
	places Places
	log    *slog.Logger
}

// NewUncertaintyHandler connects the route to the existing vault-local store.
func NewUncertaintyHandler(file *File, places Places, log *slog.Logger) *UncertaintyHandler {
	if file == nil {
		panic("mark: NewUncertaintyHandler requires a non-nil File")
	}
	if places == nil {
		panic("mark: NewUncertaintyHandler requires a non-nil Places")
	}
	if log == nil {
		panic("mark: NewUncertaintyHandler requires a non-nil logger")
	}
	return &UncertaintyHandler{file: file, places: places, log: log}
}

// Register mounts read and toggle endpoints. The serving command supplies the
// same loopback and cross-origin protections as the continuation route.
func (h *UncertaintyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+UncertaintyAddress, h.list)
	mux.HandleFunc("POST "+UncertaintyAddress, h.toggle)
}

func (h *UncertaintyHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	held, err := h.file.Uncertainties()
	if err != nil {
		h.log.Error("read the uncertainty marks", "path", h.file.UncertaintyPath(), "error", err)
		http.Error(w, wording.UncertaintyUnavailable.In(origin.Language(r)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(held); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the uncertainty marks", "error", err)
	}
}

func (h *UncertaintyHandler) toggle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	lang := origin.Language(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.MarkFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	kept := &Uncertainty{
		RelPath: r.PostFormValue("path"),
		Anchor:  r.PostFormValue("anchor"),
		At:      time.Now(),
	}
	marked, err := h.file.ToggleUncertainty(kept, func(u *Uncertainty) bool { return h.places.HasPlace(u.RelPath, u.Anchor) })
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err = json.NewEncoder(w).Encode(struct {
			Marked bool `json:"marked"`
		}{Marked: marked}); err != nil {
			h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "send the uncertainty result", "error", err)
		}
	case errors.Is(err, ErrInvalid):
		h.log.Warn("an uncertainty mark was refused", "error", err)
		http.Error(w, wording.UncertaintyRefused.In(lang), http.StatusUnprocessableEntity)
	case errors.Is(err, ErrUnreadableUncertainties):
		h.log.Error("read the uncertainty marks", "path", h.file.UncertaintyPath(), "error", err)
		http.Error(w, wording.UncertaintyUnavailable.In(lang), http.StatusInternalServerError)
	default:
		h.log.Error("write the uncertainty marks", "path", h.file.UncertaintyPath(), "error", err)
		http.Error(w, wording.UncertaintyNotStored.In(lang), http.StatusInternalServerError)
	}
}
