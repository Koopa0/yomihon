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

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxFormBytes bounds the POST /status body: four short form fields never
// need more than this.
const maxFormBytes = 4096

// Handler serves the write face's single HTTP endpoint.
type Handler struct {
	writer *Writer
	shell  func() nav.Shell
	log    *slog.Logger
}

// NewHandler wires the write face's HTTP surface around an existing Writer. A
// fail-closed write face is still a non-nil Writer. shell is sampled once only
// after a failed write, so the recovery page uses one reading snapshot.
func NewHandler(writer *Writer, shell func() nav.Shell, log *slog.Logger) *Handler {
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

// flip applies one status transition. Success answers 303; every failure
// renders a same-shell recovery page that states whether the note bytes
// changed and never offers a second POST.
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

	// Read exactly as the form sent them: tidying the ends of a path lets a
	// note named with a space resolve to its neighbour, and a neighbour that
	// holds the same bytes would satisfy the identity the form bound itself to.
	path := r.PostFormValue("path")
	from := r.PostFormValue("from")
	to := r.PostFormValue("to")
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

	err := h.writer.Flip(r.Context(), path, from, to, contentIdentity)
	if err == nil {
		// The parameter only addresses the sentence; whether the reading page
		// prints it is decided by the receipt this flip just minted, which a
		// reloaded or hand-typed address finds nothing left to spend.
		// #nosec G710 -- Flip succeeded only after its vault-local path check;
		// the prefix is a fixed same-origin literal and the value is escaped.
		http.Redirect(w, r, notesHref(path)+"?from="+url.QueryEscape(from), http.StatusSeeOther)
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Nothing was attempted and nobody is left to read a page about it, so
		// falling through would log a failure for a write that never started.
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
	summary    wording.Phrase
	nextAction wording.Phrase
	// nextActionNamesDoor marks a nextAction pointing at the page's "Open in
	// Obsidian" action, which renders only where the editor address can be
	// built; elsewhere the sentence is swapped for one without the pointer.
	nextActionNamesDoor bool
	technicalDetail     string
	logMessage          string
	cause               error
	// boundIdentity is the hex identity the refused write bound itself to, set
	// only where a write got far enough to have one. The page holds its
	// invitation back to the note until the reading generation has that
	// version, so the reader is not sent into the bytes just refused.
	boundIdentity string
}

// recoveryForUngoverned answers a note the lifecycle does not reach: a
// template artifact, or a note in a folder the contract's knowledge layer
// never named. Both take the same page and code — readable, refused
// unchanged, repaired by nothing here — and differ only in the opening
// sentence, because the reason does.
func recoveryForUngoverned(err error) *recovery {
	var summary wording.Phrase
	switch {
	case errors.Is(err, ErrNonInstance):
		summary = wording.NonInstanceReason
	case errors.Is(err, ErrOutsideKnowledgeScope):
		summary = wording.OutsideKnowledgeScope
	default:
		return nil
	}
	return &recovery{
		code:       http.StatusUnprocessableEntity,
		summary:    summary,
		nextAction: wording.NotGoverned,
	}
}

func recoveryFor(err error) *recovery {
	if r := recoveryForStatusField(err); r != nil {
		return r
	}
	if r := recoveryForInstall(err); r != nil {
		return r
	}
	if r := recoveryForIrregularEntry(err); r != nil {
		return r
	}
	if r := recoveryForUngoverned(err); r != nil {
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
			// The error text names which install step refused and what the
			// volume could guarantee, which nothing else records.
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

// recoveryForIrregularEntry maps the two refusals for a path whose shape the
// write face declines to follow, or nil when err is neither. Both are produced
// before any byte is written, so the unchanged page is truthful; they part
// over which entry broke the shape, which is the operator's first question.
func recoveryForIrregularEntry(err error) *recovery {
	var summary wording.Phrase
	switch {
	case errors.Is(err, errNotRegular):
		summary = wording.TargetNotRegular
	case errors.Is(err, errPathNotRegular):
		summary = wording.PathNotRegular
	default:
		return nil
	}
	return &recovery{
		code:            http.StatusUnprocessableEntity,
		summary:         summary,
		nextAction:      wording.TargetNotRegularNext,
		technicalDetail: err.Error(),
	}
}

// recoveryForInstall maps the outcomes that happen after the note's bytes
// already changed on disk, or nil when err is neither. The page keeps one
// shape for them: no second POST, and the operator finishes by hand.
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
// stay distinct: a missing or duplicated status line is a fault in the note,
// an unsupported syntax is one only the surgical rewriter declines.
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
		code:                http.StatusUnprocessableEntity,
		summary:             summary,
		nextAction:          wording.SchemaRefusalNext,
		nextActionNamesDoor: true,
		technicalDetail:     err.Error(),
	}
}

// recoveryNextAction picks the repair sentence a recovery page can honour: one
// naming the "Open in Obsidian" action is kept only while the page has it.
func recoveryNextAction(failure *recovery, obsidianHref string) wording.Phrase {
	if failure.nextActionNamesDoor && obsidianHref == "" {
		return wording.SchemaRefusalNextNoDoor
	}
	return failure.nextAction
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
	lang := origin.Language(r)
	door := pages.ObsidianHref(h.writer.VaultRoot(), notePath)
	view := pages.StatusRecoveryView{
		Changed:         failure.changed,
		Summary:         failure.summary.In(lang),
		NextAction:      recoveryNextAction(failure, door).In(lang),
		TechnicalDetail: failure.technicalDetail,
		NotePath:        notePath,
		NoteIdentity:    failure.boundIdentity,
		ObsidianHref:    door,
		Sidebar:         pages.NewSidebar(shell.Nav, notePath),
	}
	chrome := layouts.ChromeFromRequest(r, view.Title(lang))
	// A better place to return to than the generic POST fallback: the note
	// this refusal was about, whenever the refusal still has one.
	if notePath != "" {
		chrome.ReturnTo = notesHref(notePath)
	}
	component := pages.StatusRecovery(view, chrome)
	if err := writeRecovery(r.Context(), w, failure.code, failure.changed, component, lang); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "render status recovery", "path", path, "changed", failure.changed, "error", err)
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
	ctx context.Context,
	w http.ResponseWriter,
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
// field is required, so an absent, blank or malformed value reports false
// rather than standing in for a real identity. The page writes the field
// itself, so the value is read exactly as submitted.
func decodeContentIdentity(field string) ([sha256.Size]byte, bool) {
	var identity [sha256.Size]byte
	decoded, err := hex.DecodeString(field)
	if err != nil || len(decoded) != sha256.Size {
		return identity, false
	}
	copy(identity[:], decoded)
	return identity, true
}

// notesHref percent-escapes each path segment while preserving slash
// separators, so the redirect matches the reading face's own note links.
func notesHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/notes/" + strings.Join(segments, "/")
}
