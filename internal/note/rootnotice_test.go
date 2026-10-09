package note_test

import (
	"bytes"
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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
	var logged bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	store, err := snapshot.New(t.Context(), reader, log, nil, schema.Governance{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source: reader, VaultName: shell.VaultName(reader.Name()), Status: writer.Authority,
		Snapshot: func() *snapshot.Generation { return store.Current() }, RequestReconcile: func() {}, ObservedStatus: writer.ObservedStatus,
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
		logged.Reset()
		store, err = snapshot.New(t.Context(), reader, log, nil, schema.Governance{})
		if err != nil {
			t.Fatal(err)
		}

		rawError := ""
		if state == "missing" {
			_, observeErr := reader.ObserveRoot()
			if observeErr == nil {
				t.Fatal("missing lookup did not return the original cause")
			}
			rawError = observeErr.Error()
			warnings := rootWarningRecords(t, logged.String())
			t.Log("invoked: original root error WARN boundary")
			want := []map[string]string{{"level": "WARN", "selected_path": root, "opened_root_name": reader.Name(), "error": rawError}}
			if diff := cmp.Diff(want, warnings); diff != "" {
				t.Fatalf("caught: root error WARN boundary (-want +got):\n%s", diff)
			}
		}
		for _, tt := range []struct {
			lang           string
			title          string
			summary        string
			unknownTitle   string
			unknownSummary string
		}{
			{lang: "en", title: "Reading the startup folder", summary: "The selected path no longer points to the folder opened at startup. yomihon is still reading the original folder. Restart to open the folder now at the selected path.", unknownTitle: "Folder identity cannot be confirmed", unknownSummary: "The selected path cannot currently be confirmed to name the folder opened at startup. yomihon still uses the originally opened folder. Restore access to the path or restart."},
			{lang: "zh-Hant", title: "讀取的是啟動時的資料夾", summary: "選取的路徑已不再指向啟動時開啟的資料夾。yomihon 仍在讀取原本的資料夾；請重新啟動，開啟現在選取的資料夾。", unknownTitle: "資料夾身分無法確認", unknownSummary: "目前無法確認選取的路徑是否仍指向啟動時開啟的資料夾。yomihon 仍使用原本開啟的資料夾；請恢復路徑的存取權，或重新啟動。"},
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
					t.Log("invoked: root notice GET", state, tt.lang, path)
					detail := "Selected path: " + strconv.Quote(root) + "; Startup opened name: " + strconv.Quote(reader.Name())
					if tt.lang == "zh-Hant" {
						detail = "選取路徑：" + strconv.Quote(root) + "；啟動時開啟的名稱：" + strconv.Quote(reader.Name())
					}
					detailStart := strings.Index(section, `class="y-diagdetail"`)
					if detailStart < 0 {
						t.Fatal("root detail node absent")
					}
					valueStart := strings.IndexByte(section[detailStart:], '>')
					if valueStart < 0 {
						t.Fatal("root detail node never opens")
					}
					valueStart += detailStart + 1
					valueEnd := strings.Index(section[valueStart:], "</code>")
					if valueEnd < 0 {
						t.Fatal("root detail node never closes")
					}
					actualDetail := html.UnescapeString(section[valueStart : valueStart+valueEnd])
					t.Logf("observed: whole root detail=%q", actualDetail)
					if diff := cmp.Diff(detail, actualDetail); diff != "" {
						t.Errorf("caught: whole root detail (-want +got):\n%s", diff)
					}
					if state == "missing" && (strings.Contains(section, html.EscapeString(rawError)) || strings.Contains(section, "observe vault root:")) {
						t.Errorf("caught: raw root error reached page: %s", section)
					}
					for _, want := range []string{tt.title, tt.summary} {
						if !strings.Contains(section, html.EscapeString(want)) {
							t.Errorf("localized root notice missing %q", want)
						}
					}
					if strings.Contains(section, html.EscapeString(detail)+"; ") {
						t.Errorf("caught: appended root detail cause: %s", section)
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

func rootWarningRecords(t *testing.T, text string) []map[string]string {
	t.Helper()
	var warnings []map[string]string
	decoder := json.NewDecoder(strings.NewReader(text))
	for decoder.More() {
		var record struct {
			Message  string `json:"msg"`
			Level    string `json:"level"`
			Selected string `json:"selected_path"`
			Opened   string `json:"opened_root_name"`
			Error    string `json:"error"`
		}
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		if record.Message == "vault root identity unconfirmed; restore access or restart" {
			warnings = append(warnings, map[string]string{"level": record.Level, "selected_path": record.Selected, "opened_root_name": record.Opened, "error": record.Error})
		}
	}
	return warnings
}
