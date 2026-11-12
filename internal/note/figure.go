package note

import (
	"crypto/sha256"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/figure"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

const figureContentType = "text/html; charset=utf-8"

// figureDocument serves one figure as the document a reading page frames. It
// answers GET only, never redirects, and serves nothing but .html files.
//
// The file is read once. Everything the response says — whether it is served,
// whether its scripts may run, and which scripts those are — is decided from
// that one read's bytes, and those same bytes are what is written. There is no
// second look at the file between the decision and the bytes it was about.
func (h *Handler) figureDocument(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !figure.Is(rel) {
		http.Error(w, wording.FileNotFound.In(lang), http.StatusNotFound)
		return
	}
	file, entry, ok := h.openVaultFile(w, r, rel, lang)
	if !ok {
		return
	}
	// One byte past the ceiling is read so a file over it is known to be over
	// it, without reading the rest.
	data, err := io.ReadAll(io.LimitReader(file, figure.MaxBytes+1))
	if closeErr := file.Close(); closeErr != nil {
		h.sources.Log.Warn("close figure", "path", rel, "error", closeErr)
	}
	if err != nil {
		h.respondFileReadError(w, rel, "read figure", err, lang)
		return
	}
	h.serveFigure(w, r, rel, data, entry.Size())
}

// serveFigure answers with data and nothing else. It is handed bytes, not a
// file, so it has no way to read the file again; size is the file's size as
// selected, which a refusal for size names.
func (h *Handler) serveFigure(w http.ResponseWriter, r *http.Request, rel string, data []byte, size int64) {
	lang := origin.Language(r)
	inspection := figure.Inspect(data)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if inspection.Refusal != figure.Served {
		// The refusal is said in the frame too, under the same sandbox, for
		// the page that was drawn before the file changed into one refused.
		// None of the figure's bytes are in it.
		if !origin.SetContentSecurityPolicy(r.Context(), w, vaultFileSandbox) {
			h.sandboxUnavailable(w, rel, lang)
			return
		}
		finding := render.FigureFinding(&inspection, size)
		w.Header().Set("Content-Type", textContentType)
		w.WriteHeader(http.StatusUnprocessableEntity)
		// #nosec G705 -- the note is yomihon's own sentence, sent as text/plain
		// with nosniff under a sandbox that runs nothing.
		if _, err := io.WriteString(w, render.FigureFindingNote(finding.Kind, finding.Detail, lang)); err != nil {
			h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write figure refusal", "path", rel, "error", err)
		}
		return
	}
	policy := vaultFileSandbox
	if inspection.Scripted() && h.figureApproved(rel, inspection.Sum) {
		policy = scriptedSandbox(inspection.ScriptHashes())
	}
	if !origin.SetContentSecurityPolicy(r.Context(), w, policy) {
		h.sandboxUnavailable(w, rel, lang)
		return
	}
	w.Header().Set("Content-Type", figureContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	// #nosec G705 -- these are the figure's own bytes, sent as a document on
	// purpose: the policy set above is what confines them, and nothing was
	// written before it was known to reach the reader.
	if _, err := w.Write(data); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write figure", "path", rel, "error", err)
	}
}

// figureApproved reports whether the owner approved these exact bytes. With
// no approvals store, there is no approval. A store that cannot be read is no
// approval either, and the fault is logged: the figure is still shown, without
// its scripts.
func (h *Handler) figureApproved(rel string, sum [sha256.Size]byte) bool {
	if h.sources.FigureApproved == nil {
		return false
	}
	approval, err := h.sources.FigureApproved(rel, sum)
	if err != nil {
		h.sources.Log.Warn("figure approvals unreadable; figure served without script", "path", rel, "error", err)
		return false
	}
	return approval == render.Approved
}

func (h *Handler) sandboxUnavailable(w http.ResponseWriter, rel string, lang wording.Lang) {
	h.sources.Log.Error("serve figure", "path", rel, "error", errSandboxUnavailable)
	http.Error(w, wording.SandboxUnavailable.In(lang), http.StatusInternalServerError)
}

// scriptedSandbox is vaultFileSandbox with two directives changed and no
// other: the sandbox admits scripts, and script-src admits exactly the inline
// scripts whose hashes it lists. No host, no 'unsafe-inline', no
// 'unsafe-eval' and no 'strict-dynamic' is ever written here, so a script
// that is not one of the approved texts — injected, generated, evaluated, or
// loaded from anywhere — does not run.
func scriptedSandbox(hashes []string) string {
	if len(hashes) == 0 {
		return vaultFileSandbox
	}
	directives := strings.Split(vaultFileSandbox, "; ")
	changed := 0
	for i, directive := range directives {
		name, _, _ := strings.Cut(directive, " ")
		switch name {
		case "sandbox":
			directives[i] = "sandbox allow-scripts"
			changed++
		case "script-src":
			directives[i] = "script-src " + strings.Join(hashes, " ")
			changed++
		}
	}
	// A base that no longer names both directives once each cannot be
	// widened safely, so the figure is served without script. The route's
	// tests pin the scripted policy to a literal and turn red here.
	if changed != 2 {
		return vaultFileSandbox
	}
	return strings.Join(directives, "; ")
}
