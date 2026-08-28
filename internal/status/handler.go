package status

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxFormBytes bounds the POST /status body: four short form fields never
// need more than this.
const maxFormBytes = 4096

// Handler serves the write face's single HTTP endpoint.
type Handler struct {
	writer *Writer
	shell  func() pages.Shell
	log    *slog.Logger
}

// NewHandler wires the write face's HTTP surface around an existing
// Writer. A fail-closed write face is still a non-nil Writer whose View
// is closed. shell is sampled once only after a failed write, so the recovery
// page uses one coherent reading snapshot.
func NewHandler(writer *Writer, shell func() pages.Shell, log *slog.Logger) *Handler {
	if writer == nil {
		panic("status: NewHandler requires a non-nil Writer")
	}
	if shell == nil {
		panic("status: NewHandler requires a non-nil shell provider")
	}
	if log == nil {
		panic("status: NewHandler requires a non-nil logger")
	}
	return &Handler{writer: writer, shell: shell, log: log}
}

// Register mounts the write face's route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /status", h.flip)
}

// flip applies one status transition. Success preserves the frozen 303
// contract. Every failure renders a same-shell recovery page that states
// whether the note bytes changed and never offers a second POST.
func (h *Handler) flip(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		h.respondRecovery(w, r, "", "", "", &recovery{
			code:       http.StatusBadRequest,
			summary:    wording.FormUnreadable,
			nextAction: wording.FormUnreadableNext,
		})
		return
	}

	path := strings.TrimSpace(r.PostFormValue("path"))
	from := strings.TrimSpace(r.PostFormValue("from"))
	to := strings.TrimSpace(r.PostFormValue("to"))
	if path == "" || from == "" || to == "" {
		h.respondRecovery(w, r, path, from, to, &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.FieldsRequired,
			nextAction: wording.RecoveryStartOver,
		})
		return
	}
	contentIdentity, ok := decodeContentIdentity(r.PostFormValue("content_identity"))
	if !ok {
		h.respondRecovery(w, r, path, from, to, &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.IdentityRequired,
			nextAction: wording.IdentityRequiredNext,
		})
		return
	}

	err := h.writer.Flip(path, from, to, contentIdentity)
	if err == nil {
		// The target names the status this note just left. The reading page
		// states the change once, in a live region, and states it only when
		// that value differs from the status the note now carries — so this
		// parameter can report a transition and cannot invent one. Without it
		// the whole confirmation was the re-rendered chip, which reads the
		// same whether the press worked or somebody else's did, and which a
		// reader who cannot see it never receives at all.
		// #nosec G710 -- Flip succeeded only after its vault-local path check;
		// the prefix is a fixed same-origin literal and the value is escaped.
		http.Redirect(w, r, notesHref(path)+"?from="+url.QueryEscape(from), http.StatusSeeOther)
		return
	}
	failure := recoveryFor(err)
	failure.boundIdentity = hex.EncodeToString(contentIdentity[:])
	h.respondRecovery(w, r, path, from, to, failure)
}

type recovery struct {
	code     int
	changed  bool
	noteGone bool
	// summary and nextAction are the two sentences a refusal is made of, kept
	// in both languages until the request that will read them is known.
	summary         wording.Phrase
	nextAction      wording.Phrase
	technicalDetail string
	logMessage      string
	cause           error
	// boundIdentity is the hex identity the refused write bound itself to, set
	// only where a write got far enough to have one. The page carries it so its
	// own invitation back to the note can be held until the reading generation
	// holds at least that version: sending the reader back into the same bytes
	// that were just refused would stage the same refusal again.
	boundIdentity string
}

func recoveryFor(err error) *recovery {
	if r := recoveryForStatusField(err); r != nil {
		return r
	}
	if r := recoveryForInstall(err); r != nil {
		return r
	}
	switch {
	case errors.Is(err, ErrInvalidPath):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.PathNotRelative,
			nextAction: wording.PathNotRelativeNext,
		}
	case errors.Is(err, ErrClosed):
		return &recovery{
			code:       http.StatusServiceUnavailable,
			summary:    wording.ContractUnavailable,
			nextAction: wording.ContractUnavailableNext,
			logMessage: "status write face is closed",
			cause:      err,
		}
	case errors.Is(err, ErrArtifactPolicyUnavailable):
		return &recovery{
			code:            http.StatusServiceUnavailable,
			summary:         wording.ArtifactPolicyUnavailable,
			nextAction:      wording.ArtifactPolicyUnavailableNext,
			technicalDetail: err.Error(),
			logMessage:      "status artifact policy is unavailable",
			cause:           err,
		}
	case errors.Is(err, ErrNonInstance):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.NonInstanceReason,
			nextAction: wording.NotGoverned,
		}
	case errors.Is(err, ErrStale):
		return &recovery{
			code:       http.StatusConflict,
			summary:    wording.PageStale,
			nextAction: wording.PageStaleNext,
		}
	case errors.Is(err, ErrContentChanged):
		return &recovery{
			code:       http.StatusConflict,
			summary:    wording.ContentMoved,
			nextAction: wording.ContentMovedNext,
		}
	case errors.Is(err, ErrConcurrentWrite):
		return &recovery{
			code:       http.StatusConflict,
			summary:    wording.ContentRaced,
			nextAction: wording.ContentRacedNext,
			// Which install step the refusal came from, and on volumes
			// that cannot swap two entries atomically what that costs, rides
			// in the error text. It is the only record of the guarantee this
			// vault's filesystem was able to give, so it reaches the log.
			logMessage: "status flip refused a raced write",
			cause:      err,
		}
	case errors.Is(err, ErrDurabilityUnsupported):
		return &recovery{
			code:       http.StatusServiceUnavailable,
			summary:    wording.DurabilityUnsupported,
			nextAction: wording.PlatformUnsupportedNext,
		}
	case errors.Is(err, ErrPublishedReserved):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.PublishedRefused,
			nextAction: wording.PublishedRefusedNext,
		}
	case errors.Is(err, schema.ErrUnknownStatus),
		errors.Is(err, schema.ErrIllegalTransition):
		return recoveryForSchemaError(err)
	case errors.Is(err, fs.ErrNotExist):
		return &recovery{
			code:       http.StatusNotFound,
			noteGone:   true,
			summary:    wording.NoteGone,
			nextAction: wording.NoteGoneNext,
		}
	default:
		return &recovery{
			code:       http.StatusInternalServerError,
			summary:    wording.WriteFailed,
			nextAction: wording.WriteFailedNext,
			logMessage: "status flip failed",
			cause:      err,
		}
	}
}

// recoveryForInstall maps the outcomes that happen after the note's bytes
// already changed on disk, or nil when err is neither. They share one shape
// the page has to keep: the file is not what it was, no second POST is
// offered, and the operator finishes by hand.
func recoveryForInstall(err error) *recovery {
	switch {
	case errors.Is(err, ErrInstallStranded):
		return &recovery{
			code:            http.StatusInternalServerError,
			changed:         true,
			summary:         wording.ConcurrentWriteLeftBoth,
			nextAction:      wording.ConcurrentWriteLeftBothNext,
			technicalDetail: err.Error(),
			logMessage:      "status install left both versions on disk",
			cause:           err,
		}
	case errors.Is(err, ErrInstallUncertain):
		return &recovery{
			code:       http.StatusInternalServerError,
			changed:    true,
			summary:    wording.DurabilityUnconfirmed,
			nextAction: wording.DurabilityUnconfirmedNext,
			logMessage: "status install durability is uncertain",
			cause:      err,
		}
	}
	return nil
}

// recoveryForStatusField maps the refusals rooted in how the note's own
// frontmatter writes its status field, or nil when err is neither. The two
// stay distinct on the page: a missing or duplicated status line is a fault
// in the note, while an unsupported syntax is a note the reader understands
// perfectly well and only the surgical rewriter declines.
func recoveryForStatusField(err error) *recovery {
	switch {
	case errors.Is(err, ErrStatusLine):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.StatusFieldInvalid,
			nextAction: wording.StatusFieldInvalidNext,
		}
	case errors.Is(err, ErrStatusSyntaxUnsupported):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    wording.StatusFieldUnsupportedYAML,
			nextAction: wording.StatusFieldUnsupportedYAMLNext,
		}
	}
	return nil
}

func recoveryForSchemaError(err error) *recovery {
	summary := wording.TransitionRefused
	switch {
	case errors.Is(err, schema.ErrUnknownStatus):
		summary = wording.StatusOutsideEnum
	case errors.Is(err, schema.ErrIllegalTransition):
		summary = wording.TransitionNotAllowed
	}
	return schemaRecovery(summary, err)
}

func schemaRecovery(summary wording.Phrase, err error) *recovery {
	return &recovery{
		code:            http.StatusUnprocessableEntity,
		summary:         summary,
		nextAction:      wording.SchemaRefusalNext,
		technicalDetail: err.Error(),
	}
}

func (h *Handler) respondRecovery(
	w http.ResponseWriter,
	r *http.Request,
	path, from, to string,
	failure *recovery,
) {
	if failure.logMessage != "" {
		h.log.Error(failure.logMessage, "path", path, "from", from, "to", to, "error", failure.cause)
	}
	notePath := recoveryNotePath(path)
	if failure.noteGone {
		notePath = ""
	}
	shell := h.shell()
	lang := pages.LanguageFromRequest(r)
	view := pages.StatusRecoveryView{
		Changed:         failure.changed,
		Summary:         failure.summary.In(lang),
		NextAction:      failure.nextAction.In(lang),
		TechnicalDetail: failure.technicalDetail,
		NotePath:        notePath,
		NoteIdentity:    failure.boundIdentity,
		ObsidianHref:    pages.ObsidianHref(h.writer.VaultRoot(), notePath),
		Sidebar:         pages.NewSidebar(shell.Nav, notePath, lang),
	}
	component := pages.StatusRecovery(view, pages.ChromeFromRequest(r, view.Title()))
	if err := writeRecovery(w, r.Context(), failure.code, failure.changed, component, lang); err != nil {
		h.log.Error("render status recovery", "path", path, "changed", failure.changed, "error", err)
	}
}

func recoveryNotePath(path string) string {
	normalized, _, err := normalizeRelPath(path)
	if err != nil {
		return ""
	}
	return normalized
}

func writeRecovery(
	w http.ResponseWriter,
	ctx context.Context,
	code int,
	changed bool,
	component templ.Component,
	lang wording.Lang,
) error {
	var body bytes.Buffer
	if err := component.Render(ctx, &body); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusInternalServerError)
		message := wording.RecoveryRenderFailedUnchanged
		if changed {
			message = wording.RecoveryRenderFailedChanged
		}
		// #nosec G705 -- the body is one of two sentences written in this repository
		// and chosen by language; nothing from the request reaches it, and the
		// response is text/plain with nosniff besides.
		if _, writeErr := io.WriteString(w, message.In(lang)); writeErr != nil {
			return errors.Join(
				fmt.Errorf("render recovery page: %w", err),
				fmt.Errorf("write recovery fallback: %w", writeErr),
			)
		}
		return fmt.Errorf("render recovery page: %w", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if _, err := w.Write(body.Bytes()); err != nil {
		return fmt.Errorf("write recovery page: %w", err)
	}
	return nil
}

// decodeContentIdentity reads the form's hex-encoded content identity. The
// field is required — a caller must state which version of the note its
// ruling was read against — so an absent, blank, or malformed value reports
// false rather than standing in for any real identity.
func decodeContentIdentity(field string) ([sha256.Size]byte, bool) {
	var identity [sha256.Size]byte
	decoded, err := hex.DecodeString(strings.TrimSpace(field))
	if err != nil || len(decoded) != sha256.Size {
		return identity, false
	}
	copy(identity[:], decoded)
	return identity, true
}

// notesHref percent-escapes each path segment while preserving slash
// separators. The successful redirect remains local and byte-identical to the
// reading face's note links without introducing a presentation dependency into
// Writer.
func notesHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/notes/" + strings.Join(segments, "/")
}
