package note_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownTargetsReachTheRegisteredReadingRoutes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"Inbox/sub/Source.md":  "# Source\n\n[relative](../../Areas/Vacuum%20note.md) [asset](../../Areas/manual.txt) [colon](../../Docs/a%3Ab.md)\n\n[**choose**](Other.md) [missing](Absent.md)\n\n## 待補\n\n- [[Absent.md]]\n",
		"Areas/Vacuum note.md": "# Target\n\nTARGET-720\n",
		"Areas/manual.txt":     "RESOURCE-720\n",
		"Inbox/sub/Other.md":   "local\n",
		"Else/Other.md":        "another\n",
		"Docs/a:b.md":          "COLON-TARGET-720\n",
	}
	for path, body := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	server := newServer(t, root)
	for _, lang := range []string{"zh-Hant", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/Inbox/sub/Source.md", http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Cookie", "yomihon_lang="+lang)
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil {
				t.Fatalf("read/close: %v / %v", readErr, closeErr)
			}
			text := string(body)
			if !strings.Contains(text, `<html lang="`+lang+`"`) {
				t.Fatalf("interface language %s was not rendered", lang)
			}
			if response.StatusCode != http.StatusOK || strings.Count(text, `href="/notes/Areas/Vacuum%20note.md"`) != 1 || !strings.Contains(text, `href="/raw/Areas/manual.txt"`) || !strings.Contains(text, `href="/notes/Docs/a:b.md"`) || !strings.Contains(text, `href="/notes/Inbox/sub/Other.md"`) || !strings.Contains(text, "<strong>choose</strong>") {
				t.Fatalf("assembled Markdown routes/status mismatch (%d):\n%s", response.StatusCode, text)
			}
			if strings.Contains(text, "markdown-broken") || strings.Contains(text, "markdown-ambiguous") {
				t.Errorf("internal diagnostic name reached the page:\n%s", text)
			}
			missingSummary := "這個 Markdown 連結沒有唯一可用的書庫目標；已留下文字並標示原因。"
			if lang == "en" {
				missingSummary = "This Markdown link has no usable vault target; its text remains with an explanation."
			}
			if !strings.Contains(text, missingSummary) {
				t.Errorf("planned wikilink suppressed the independent Markdown path diagnostic: missing %q", missingSummary)
			}
			for _, target := range []struct{ path, content string }{{"/notes/Areas/Vacuum%20note.md", "TARGET-720"}, {"/raw/Areas/manual.txt", "RESOURCE-720"}, {"/notes/Docs/a:b.md", "COLON-TARGET-720"}} {
				if err := markdownTargetResponse(t, server.Client(), server.URL+target.path, target.content); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func markdownTargetResponse(t *testing.T, client *http.Client, address, content string) error {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), content) {
		t.Errorf("Markdown target GET status %d/body %q lacks %q", response.StatusCode, body, content)
	}
	return nil
}
