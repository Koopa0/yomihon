package status

import (
	"bytes"
	"context"
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
)

// maxFormBytes bounds the POST /status body: three short form fields never
// need more than this.
const maxFormBytes = 4096

// Handler serves the write face's single HTTP endpoint.
type Handler struct {
	lifecycle *Lifecycle
	shell     func() pages.Shell
	log       *slog.Logger
}

// NewHandler wires the write face's HTTP surface around an existing
// Lifecycle. A fail-closed write face is still a non-nil Lifecycle whose View
// is closed. shell is sampled once only after a failed write, so the recovery
// page uses one coherent reading snapshot.
func NewHandler(lifecycle *Lifecycle, shell func() pages.Shell, log *slog.Logger) *Handler {
	if lifecycle == nil {
		panic("status: NewHandler requires a non-nil Lifecycle")
	}
	if shell == nil {
		panic("status: NewHandler requires a non-nil shell provider")
	}
	if log == nil {
		panic("status: NewHandler requires a non-nil logger")
	}
	return &Handler{lifecycle: lifecycle, shell: shell, log: log}
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
			summary:    "表單內容無法解析。",
			nextAction: "返回原本的筆記頁面，重新載入後再選擇一次狀態轉換。",
		})
		return
	}

	path := strings.TrimSpace(r.PostFormValue("path"))
	from := strings.TrimSpace(r.PostFormValue("from"))
	to := strings.TrimSpace(r.PostFormValue("to"))
	if path == "" || from == "" || to == "" {
		h.respondRecovery(w, r, path, from, to, &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    "path、from 與 to 都是必填欄位。",
			nextAction: "返回筆記或首頁，從目前頁面重新開始這次操作。",
		})
		return
	}

	err := h.lifecycle.Flip(path, from, to)
	if err == nil {
		target := notesHref(path)
		if to == schema.SealStatus {
			target += "?sealed=1"
		}
		// #nosec G710 -- Flip succeeded only after its vault-local path check;
		// the prefix and query are fixed same-origin literals.
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	h.respondRecovery(w, r, path, from, to, recoveryFor(err))
}

type recovery struct {
	code            int
	changed         bool
	noteGone        bool
	summary         string
	nextAction      string
	technicalDetail string
	logMessage      string
	cause           error
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
			summary:    "path 必須是 vault 內的相對 slash 路徑。",
			nextAction: "返回首頁，從導覽重新開啟筆記後再操作。",
		}
	case errors.Is(err, ErrClosed):
		return &recovery{
			code:       http.StatusServiceUnavailable,
			summary:    "vault contract 無法使用；生命週期寫入已關閉（fail-closed）。",
			nextAction: "修正 vault contract 後重新啟動 yomihon，再從筆記頁面操作。",
			logMessage: "status write face is closed",
			cause:      err,
		}
	case errors.Is(err, ErrArtifactPolicyUnavailable):
		return &recovery{
			code:            http.StatusServiceUnavailable,
			summary:         "vault artifact policy 無法使用；生命週期寫入已關閉。",
			nextAction:      "修正或還原 vault-schema.toml 的 artifact policy，重新啟動 yomihon 後再操作。",
			technicalDetail: err.Error(),
			logMessage:      "status artifact policy is unavailable",
			cause:           err,
		}
	case errors.Is(err, ErrNonInstance):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    nonInstanceReason,
			nextAction: "這個資源仍可閱讀，但 status 只能在生命週期治理的筆記上變更。",
		}
	case errors.Is(err, ErrStale):
		return &recovery{
			code:       http.StatusConflict,
			summary:    "這個頁面已過期；磁碟上的狀態已經不同。",
			nextAction: "返回筆記並重新載入目前狀態，確認後再選擇一次轉換。",
		}
	case errors.Is(err, ErrConcurrentWrite):
		return &recovery{
			code:       http.StatusConflict,
			summary:    "檔案在讀取與寫入之間遭到修改。",
			nextAction: "先檢查 Obsidian 或其他工具的最新變更，再重新載入筆記；不要直接重送。",
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
			summary:    DurableInstallUnavailableDiagnostic,
			nextAction: "改用目前支援 status 寫入的 macOS 或 Linux；閱讀與搜尋仍可在此平台使用。",
		}
	case errors.Is(err, ErrPublishedReserved):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    "published 記錄的是一次已完成的發布；沒有任何發布器能為這次寫入作證，yomihon 不會設定這個值。",
			nextAction: "筆記沒有任何變更。若發布確實已完成，請直接編輯筆記的 frontmatter 記下這個事實。",
		}
	case errors.Is(err, schema.ErrUnknownStatus),
		errors.Is(err, schema.ErrIllegalTransition):
		return recoveryForSchemaError(err)
	case errors.Is(err, fs.ErrNotExist):
		return &recovery{
			code:       http.StatusNotFound,
			noteGone:   true,
			summary:    "找不到這篇筆記；它可能已被刪除或移動。",
			nextAction: "返回首頁，從目前的導覽重新找到筆記；這次操作沒有寫入任何內容。",
		}
	default:
		return &recovery{
			code:       http.StatusInternalServerError,
			summary:    "yomihon 無法完成狀態變更。",
			nextAction: "查看 yomihon 日誌並確認 vault 狀態；在原因不明時不要反覆重送。",
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
			summary:         "有其他程式在寫入途中改了這個檔案，yomihon 無法確定筆記現在是哪一份內容。兩份都留在磁碟上，沒有刪除任何內容。",
			nextAction:      "不要重送。請依下方訊息比對筆記與旁邊保留的那一份，手動留下正確的內容後再操作。",
			technicalDetail: err.Error(),
			logMessage:      "status install left both versions on disk",
			cause:           err,
		}
	case errors.Is(err, ErrInstallUncertain):
		return &recovery{
			code:       http.StatusInternalServerError,
			changed:    true,
			summary:    "筆記已改寫，但無法確認資料已耐久保存。",
			nextAction: "不要重送。請直接在 vault 中檢查筆記內容與檔案狀態，確認後再手動收尾。",
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
			summary:    "frontmatter 的 status 欄位違反 schema。",
			nextAction: "直接編輯筆記，讓 frontmatter 恰好包含一個合法的 status 欄位，再重新載入。",
		}
	case errors.Is(err, ErrStatusSyntaxUnsupported):
		return &recovery{
			code:       http.StatusUnprocessableEntity,
			summary:    "frontmatter 的 status 欄位使用了 yomihon 狀態改寫不支援的 YAML 寫法。",
			nextAction: "直接編輯筆記，把 status 欄位改成單行的「status: 值」形式（不使用引號鍵、flow mapping 或 YAML 錨點），再重新載入。",
		}
	}
	return nil
}

func recoveryForSchemaError(err error) *recovery {
	summary := "vault schema 拒絕這個狀態轉換。"
	switch {
	case errors.Is(err, schema.ErrUnknownStatus):
		summary = "狀態值不在 vault schema 的允許清單中。"
	case errors.Is(err, schema.ErrIllegalTransition):
		summary = "vault schema 不允許這個狀態轉換。"
	}
	return schemaRecovery(summary, err)
}

func schemaRecovery(summary string, err error) *recovery {
	return &recovery{
		code:            http.StatusUnprocessableEntity,
		summary:         summary,
		nextAction:      "依照 vault schema 修正狀態值或轉換，再重新載入筆記。",
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
	view := pages.StatusRecoveryView{
		Changed:         failure.changed,
		Summary:         failure.summary,
		NextAction:      failure.nextAction,
		TechnicalDetail: failure.technicalDetail,
		NotePath:        notePath,
		Sidebar:         pages.NewSidebar(shell.Nav, notePath),
	}
	component := pages.StatusRecovery(view, shell.Chrome(r, view.Title()))
	if err := writeRecovery(w, r.Context(), failure.code, failure.changed, component); err != nil {
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
) error {
	var body bytes.Buffer
	if err := component.Render(ctx, &body); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusInternalServerError)
		message := "狀態尚未變更；復原頁面顯示失敗。\n"
		if changed {
			message = "狀態已寫入，需要手動收尾；復原頁面顯示失敗，請勿重送，請直接檢查 vault。\n"
		}
		if _, writeErr := io.WriteString(w, message); writeErr != nil {
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

const nonInstanceReason = "不屬於生命週期治理範圍"

// notesHref percent-escapes each path segment while preserving slash
// separators. The successful redirect remains local and byte-identical to the
// reading face's note links without introducing a presentation dependency into
// Lifecycle.
func notesHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/notes/" + strings.Join(segments, "/")
}
