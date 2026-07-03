package status

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/koopa0/kurodo/internal/schema"
)

// maxFormBytes bounds the POST /status body: three short form fields never
// need more than this (security.md — request size limits).
const maxFormBytes = 4096

// Handler serves the write face's single HTTP endpoint.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the write face's HTTP surface around an existing
// Service. svc must not be nil — a fail-closed write face is still a
// *Service (Closed() reports true), not a missing one.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	if svc == nil {
		panic("status: NewHandler requires a non-nil Service")
	}
	return &Handler{svc: svc, log: log}
}

// Register mounts the write face's route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /status", h.flip)
}

// flip handles POST /status (path, from, to) — see docs/spec.md §4 for the
// full write-path algorithm and error vocabulary this implements.
func (h *Handler) flip(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "cannot parse form", http.StatusBadRequest)
		return
	}

	path := strings.TrimSpace(r.PostFormValue("path"))
	from := strings.TrimSpace(r.PostFormValue("from"))
	to := strings.TrimSpace(r.PostFormValue("to"))
	if path == "" || from == "" || to == "" {
		http.Error(w, "path, from, and to are all required", http.StatusUnprocessableEntity)
		return
	}

	err := h.svc.Flip(r.Context(), path, from, to)
	switch {
	case err == nil:
		// On the seal (→ ready) carry a one-shot ?sealed=1 so the reading page
		// plays the settle animation once and then strips it; every other
		// transition redirects plainly. The literal "ready" is the one status
		// that wears the seal (the same one the reading UI styles as primary),
		// not a copy of the toml state machine — legality still came from schema.
		target := "/notes/" + path
		if to == "ready" {
			target += "?sealed=1"
		}
		// #nosec G710 -- Flip already succeeded, meaning path passed
		// Service.Flip's filepath.IsLocal vault-escape check; "/notes/" and the
		// query are fixed same-origin literals, not attacker-controlled.
		http.Redirect(w, r, target, http.StatusSeeOther)
	case errors.Is(err, ErrClosed):
		http.Error(w, "the vault contract is unavailable; the write face is closed (fail-closed)", http.StatusServiceUnavailable)
	case errors.Is(err, ErrStale):
		http.Error(w, "this page is stale; reload and try again", http.StatusConflict)
	case errors.Is(err, ErrConcurrentWrite):
		// Distinct from ErrStale above: this is not an old browser tab —
		// something touched the file in the narrow window between kurodo
		// reading it and writing it back (docs/spec.md §4's "mtime changed" row).
		http.Error(w, "the file was modified between read and write; try again", http.StatusConflict)
	case errors.Is(err, ErrDirty):
		http.Error(w, "this file has uncommitted changes that a flip would pollute the audit trail with; resolve them first", http.StatusConflict)
	case errors.Is(err, ErrStatusLine):
		http.Error(w, "the frontmatter status field is a schema violation; hand this file to kura or a human", http.StatusUnprocessableEntity)
	case errors.Is(err, schema.ErrUnknownStatus), errors.Is(err, schema.ErrIllegalTransition), errors.Is(err, schema.ErrOwnerForbidden):
		// docs/spec.md §4's error table mandates the schema's own rejection
		// reason verbatim, not a generic message — err already carries it
		// (schema.Transition's wrapped sentinel text). Logged like every
		// other rejection branch so a 422 is diagnosable without asking
		// Koopa to reproduce it.
		h.log.Error("status transition rejected", "path", path, "from", from, "to", to, "error", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, ErrCommitFailed):
		// Deliberately not the generic 500 branch below: docs/spec.md §4's
		// error table gives this its own presentation ("file already changed
		// + raw git output + manual remediation command") because the fix
		// requires seeing what git said.
		// This is a loopback-only, single-operator tool (wall 2) — there is
		// no other party who could read this response.
		h.log.Error("status commit failed", "path", path, "error", err)
		http.Error(w, fmt.Sprintf("the note was rewritten but the git commit failed; fix manually in the vault: %v", err), http.StatusInternalServerError)
	default:
		h.log.Error("flip failed", "path", path, "from", from, "to", to, "error", err)
		http.Error(w, "cannot flip status", http.StatusInternalServerError)
	}
}
