package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// guidePath is how a message names a file of the repository's docs folder.
var guidePath = regexp.MustCompile(`docs/[A-Za-z0-9._-]+\.md`)

// TestTheUpgradeGuideAnOlderRulesFileIsSentToIsInTheRepository holds the
// pointer to what it points at. A reader whose rules file is in the older
// format is told where the upgrade guide is by the command line and by the
// desk, in both languages; each of them has to name a file that is there, in
// both of its languages, and a URL that ends in the same path, so a file moved
// without its message, or a message changed without its file, turns this red.
// Every surface is asked through its production entry, and the path is read
// out of what it says rather than from a constant the test shares with it.
func TestTheUpgradeGuideAnOlderRulesFileIsSentToIsInTheRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for path, content := range map[string]string{
		"Writing/note.md":      "# A note\n\nBody.\n",
		schema.ContractRelPath: "# rules\nschema_version = \"1\"\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
			t.Fatalf("write %s: %v", path, err)
		}
	}

	surfaces := map[string]string{}
	var stdout, stderr bytes.Buffer
	if exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=json"}, &stdout, &stderr, false); exit != 2 {
		t.Fatalf("check exit = %d, want 2", exit)
	}
	surfaces["check"] = stderr.String()

	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})
	for name, acceptLanguage := range map[string]string{"desk in English": "en", "desk in Chinese": "zh-TW"} {
		desk := httptest.NewRecorder()
		request := siteRequest(t, http.MethodGet, "/", http.NoBody)
		request.Header.Set("Accept-Language", acceptLanguage)
		site.ServeHTTP(desk, request)
		if desk.Code != http.StatusOK {
			t.Fatalf("GET / (%s) status = %d, want %d", name, desk.Code, http.StatusOK)
		}
		surfaces[name] = desk.Body.String()
	}

	for name, text := range surfaces {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := guidePath.FindString(text)
			if path == "" {
				t.Fatalf("the %s names no file under docs/, so a reader is not sent to the upgrade guide", name)
			}
			for _, file := range []string{path, strings.TrimSuffix(path, ".md") + ".zh-TW.md"} {
				info, statErr := os.Stat(filepath.Join("..", "..", filepath.FromSlash(file)))
				if statErr != nil || !info.Mode().IsRegular() {
					t.Errorf("the message sends a reader to %s, which is not a file of the repository: %v", file, statErr)
				}
			}
			if want := "https://github.com/koopa0/yomihon/blob/main/" + path; !strings.Contains(text, want) {
				t.Errorf("the message names %s but not the address %s a reader without the repository can open", path, want)
			}
		})
	}
}
