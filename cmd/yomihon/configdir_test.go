package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
)

// TestAConfigurationDirectoryThatIsNotAbsoluteKeepsNothingInTheFolderBeingRead
// holds promise 1 where the platform's answer is what would break it. yomihon
// with no folder named serves the one it stands in, and a configuration
// directory the environment spells relative — HOME=. on macOS, %AppData% on
// Windows, HOME on Linux with no XDG_CONFIG_HOME — is read against that same
// folder, so a bookmark would be kept inside the vault. The command starts
// without a configuration directory instead, says why, and writes nothing.
//
// Not parallel: it sets the environment and the working directory.
func TestAConfigurationDirectoryThatIsNotAbsoluteKeepsNothingInTheFolderBeingRead(t *testing.T) {
	root := t.TempDir()
	writeDeskFixture(t, root)
	t.Chdir(root)
	t.Setenv("HOME", ".")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", ".")
	before := vaultState(t, root)

	cfg, err := loadConfig(root)
	if err != nil {
		t.Fatalf("loadConfig with a relative configuration directory = %v, want a config the command can serve from", err)
	}
	if !cfg.configDir.IsZero() {
		t.Errorf("loadConfig resolved %v as the configuration directory, which is relative to the folder being read", cfg.configDir)
	}
	if !strings.Contains(cfg.noConfigDir, "not an absolute path") {
		t.Errorf("loadConfig says %q of the configuration directory, want that it is not an absolute path", cfg.noConfigDir)
	}
	if listed := userConfig(); !listed.configDir.IsZero() {
		t.Errorf("userConfig resolved %v as the configuration directory, which is relative to the folder being read", listed.configDir)
	}

	site, err := newReadingSite(t.Context(), cfg.root, cfg.configDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	form := url.Values{"path": {"Concepts/alpha.md"}, "offset": {"0"}, "identity": {strings.Repeat("a", 64)}}
	req := siteRequest(t, http.MethodPost, mark.BookmarkPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, req)
	if err := site.close(); err != nil {
		t.Fatalf("readingSite.close() error = %v", err)
	}
	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /bookmark with a relative configuration directory = %d, want the route not mounted", recorder.Code)
	}

	after := vaultState(t, root)
	for rel, was := range before {
		if now, ok := after[rel]; !ok {
			t.Errorf("%s was removed from the vault", rel)
		} else if now != was {
			t.Errorf("%s changed in the vault:\n before %s\n after  %s", rel, was, now)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			t.Errorf("%s was created in the vault", rel)
		}
	}
}
