package mark

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxFormBytes bounds the body a mark arrives in: a path, an id, a number and
// a digest never come near it.
const maxFormBytes = 4096

// Address is where a mark is kept. It is exported because the route that
// answers here and the address the reading page posts to are decided on two
// sides of a package boundary: the command reads it from here and hands it to
// the page, which renders it rather than spelling one of its own. An address
// written twice is one that drifts.
const Address = "/marks"

// Handler serves the one route a mark reaches the process through.
//
// A page cannot write a file, so the mark travels over a route of its own on
// the same loopback listener, with the body cap and the replace-by-rename
// discipline the status write already has. It is not a face an agent has any
// business calling: like the status write, this endpoint is the reader's.
//
// It keeps a place only in a note the vault holds now, asked of the same Places
// the uncertainty route asks. The path is the one field of a mark that reaches
// the desk as text, so a path nobody's note carries is refused rather than
// stored for a later page to print.
type Handler struct {
	file   *File
	places Places
	log    *slog.Logger
}

// NewHandler wires the mark route around an existing file and the vault whose
// notes a place may name.
func NewHandler(file *File, places Places, log *slog.Logger) *Handler {
	if file == nil {
		panic("mark: NewHandler requires a non-nil File")
	}
	if places == nil {
		panic("mark: NewHandler requires a non-nil Places")
	}
	if log == nil {
		panic("mark: NewHandler requires a non-nil logger")
	}
	return &Handler{file: file, places: places, log: log}
}

// Register mounts the route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST "+Address, h.set)
}

// set keeps the place this submission names, replacing whatever was kept
// before. It answers no content on success: the page that posted knows what it
// sent, and there is nothing to render — the row that offers the mark back is
// built by the server on the desk, from the file rather than from a reply.
func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.MarkFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(r.PostFormValue("offset"))
	if err != nil {
		http.Error(w, wording.MarkRefused.In(lang), http.StatusUnprocessableEntity)
		return
	}
	kept := &Continuation{
		RelPath:  r.PostFormValue("path"),
		Anchor:   r.PostFormValue("anchor"),
		Offset:   offset,
		Identity: r.PostFormValue("identity"),
		At:       time.Now(),
	}
	switch err = h.keep(kept); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrInvalid):
		// The reader wrote none of these fields, so the refusal is a fault in
		// the page rather than something they can correct. It is logged where
		// it can be acted on and answered with the one sentence they can use.
		h.log.Warn("a mark was refused", "error", err)
		http.Error(w, wording.MarkRefused.In(lang), http.StatusUnprocessableEntity)
	default:
		h.log.Error("write the marks file", "path", h.file.Path(), "error", err)
		http.Error(w, wording.MarkNotStored.In(lang), http.StatusInternalServerError)
	}
}

// keep stores kept when its shape is one this file holds and its note is one
// the vault holds. The shape is judged first, so a path no note could carry
// never reaches the vault to be looked up.
//
// Only the note is asked after: the empty anchor names it as a whole. An anchor
// is any id the rendered body carries — a block address or a footnote as readily
// as a heading — and the vault's answer for an anchor covers headings alone, so
// asking it about the anchor would refuse a place a reading page honestly
// stamps. The anchor stays bounded by its shape, and it only ever reaches the
// desk inside an address.
func (h *Handler) keep(kept *Continuation) error {
	if err := validate(kept); err != nil {
		return err
	}
	if !h.places.HasPlace(kept.RelPath, "") {
		return fmt.Errorf("%w: %q is not a note this vault holds", ErrInvalid, kept.RelPath)
	}
	return h.file.SetContinuation(kept)
}
