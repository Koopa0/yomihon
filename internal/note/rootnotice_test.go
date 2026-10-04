package note_test

import (
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
)

func TestRootReplacementReachesBothPagesInBothLanguages(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault<&>")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Existing.md"), []byte("old folder\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := openReadingVault(t, root)
	writer := openStatusWriter(t, reader, nil, schema.Governance{})
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	store, err := snapshot.New(t.Context(), reader, log, nil, schema.Governance{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source: reader, VaultName: shell.VaultName(reader.Name()), Status: writer.Authority,
		Snapshot: func() *snapshot.Generation { return store.Current() }, ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Continuation:   func() (mark.Continuation, bool) { return mark.Continuation{}, false }, Log: log,
	}).Register(mux)
	for _, state := range []string{"changed", "missing", "restored"} {
		if state == "missing" {
			if removeErr := os.Remove(root); removeErr != nil {
				t.Fatal(removeErr)
			}
		}
		if state == "restored" {
			if renameErr := os.Rename(root+".old", root); renameErr != nil {
				t.Fatal(renameErr)
			}
		}
		store, err = snapshot.New(t.Context(), reader, log, nil, schema.Governance{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tt := range []struct {
			lang           string
			title          string
			summary        string
			selectedLabel  string
			openedLabel    string
			unknownTitle   string
			unknownSummary string
		}{
			{lang: "en", title: "Reading the startup folder", summary: "The selected path no longer points to the folder opened at startup. yomihon is still reading the original folder. Restart to open the folder now at the selected path.", selectedLabel: "Selected path", openedLabel: "Startup opened name", unknownTitle: "Folder identity cannot be confirmed", unknownSummary: "The selected path cannot currently be confirmed to name the folder opened at startup. yomihon still uses the originally opened folder. Restore access to the path or restart."},
			{lang: "zh-Hant", title: "讀取的是啟動時的資料夾", summary: "選取的路徑已不再指向啟動時開啟的資料夾。yomihon 仍在讀取原本的資料夾；請重新啟動，開啟現在選取的資料夾。", selectedLabel: "選取路徑", openedLabel: "啟動時開啟的名稱", unknownTitle: "資料夾身分無法確認", unknownSummary: "目前無法確認選取的路徑是否仍指向啟動時開啟的資料夾。yomihon 仍使用原本開啟的資料夾；請恢復路徑的存取權，或重新啟動。"},
		} {
			for _, path := range []string{"/", "/health"} {
				t.Run(state+"/"+tt.lang+path, func(t *testing.T) {
					request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
					request.Header.Set("Cookie", "yomihon_lang="+tt.lang)
					response := httptest.NewRecorder()
					mux.ServeHTTP(response, request)
					if response.Code != http.StatusOK {
						t.Fatalf("page status = %d", response.Code)
					}
					body := response.Body.String()
					needle := `data-home-block="notice"`
					if path == "/health" {
						needle = "data-health-notice"
					}
					start := strings.Index(body, needle)
					if state == "restored" {
						if start >= 0 {
							t.Fatal("confirmed recovery still drew a root notice")
						}
						return
					}
					if start < 0 {
						t.Fatalf("root notice missing from %s", path)
					}
					end := strings.Index(body[start:], "</section>")
					if end < 0 {
						t.Fatal("notice section never closed")
					}
					section := body[start : start+end]
					if state == "missing" {
						tt.title, tt.summary = tt.unknownTitle, tt.unknownSummary
					}
					for _, want := range []string{tt.title, tt.summary, tt.selectedLabel + ": " + strconv.Quote(root), tt.openedLabel + ": " + strconv.Quote(reader.Name())} {
						if !strings.Contains(section, html.EscapeString(want)) {
							t.Errorf("localized root notice missing %q", want)
						}
					}
					if strings.Contains(section, "vault<&>") {
						t.Error("root detail was not escaped")
					}
				})
			}
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/freshness/Existing.md?identity="+identityOf("old folder\n"), http.NoBody)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != "unchanged" {
			t.Fatalf("root notice changed per-note freshness in %s: %d %q", state, response.Code, response.Body.String())
		}
	}
}
