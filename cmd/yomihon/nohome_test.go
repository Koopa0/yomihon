package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAReadingRoomOpensWithNoConfigurationDirectory holds the order of the two
// things this command does.
//
// Reading is the product; a kept reading place is a convenience on top of it.
// The directory that convenience is stored in comes from the environment, and
// an environment that does not name one — no HOME, and no XDG_CONFIG_HOME
// where the platform reads one — is a machine yomihon still has to serve. It
// says so once and goes on: no marks file, no route to post one to, and no
// control on the page offering something the process cannot keep.
//
// Before this was held, resolving that directory was the second thing the
// command did and it failed the whole start.
func TestAReadingRoomOpensWithNoConfigurationDirectory(t *testing.T) {
	root := t.TempDir()
	writeDeskFixture(t, root)

	// An environment naming no configuration directory, on either platform
	// this command supports.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	cfg, err := loadConfig(root)
	if err != nil {
		t.Fatalf("loadConfig with no configuration directory = %v, want a config the command can serve from", err)
	}
	if cfg.configDir != "" {
		t.Fatalf("loadConfig resolved %q as the configuration directory with nothing naming one", cfg.configDir)
	}
	if cfg.root != root || cfg.port != defaultPort {
		t.Errorf("loadConfig = %+v, want the vault root and the default port unaffected", cfg)
	}

	var logged bytes.Buffer
	site, err := newReadingSite(t.Context(), cfg.root, cfg.configDir, slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil {
		t.Fatalf("newReadingSite with no configuration directory: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})

	// The reading room is whole.
	for _, address := range []string{"/", "/notes/Concepts/alpha.md"} {
		recorder := httptest.NewRecorder()
		site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, address, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s with no configuration directory = %d, want 200", address, recorder.Code)
		}
		if address != "/" && strings.Contains(recorder.Body.String(), "data-mark-control") {
			t.Errorf("%s offers to keep a reading place the process has nowhere to keep", address)
		}
	}

	// And the route that would keep one is not mounted at all, rather than
	// mounted and refusing every request it is given. What answers instead is
	// the reading surface's own catch-all, which is registered for GET, so the
	// router reports the method rather than the address — the two ways an
	// unmounted route can read, and neither of them is a place being kept.
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, siteRequest(t, http.MethodPost, "/marks", strings.NewReader("path=a.md&offset=0")))
	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /marks with no configuration directory = %d, want the router to refuse an address it never mounted", recorder.Code)
	}

	if said := logged.String(); !strings.Contains(said, "reading place") {
		t.Errorf("the command said nothing about marks being unavailable; it logged:\n%s", said)
	}
}
