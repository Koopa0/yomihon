package layouts

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/asset"
)

// TestChromeCarriesTheVaultTheRouterSupplies holds the one path the folder's
// name takes to a page: set once around the whole router, read when a page's
// chrome is built, and empty where nothing set it.
func TestChromeCarriesTheVaultTheRouterSupplies(t *testing.T) {
	t.Parallel()
	read := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, ChromeFromRequest(r, "t").Vault); err != nil { // #nosec G705 -- test response
			t.Errorf("write the chrome's vault: %v", err)
		}
	})

	named := httptest.NewRecorder()
	VaultNamed(func() string { return "Field Notes" }, read).ServeHTTP(named, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	if got := named.Body.String(); got != "Field Notes" {
		t.Errorf("chrome Vault behind VaultNamed = %q, want %q", got, "Field Notes")
	}

	bare := httptest.NewRecorder()
	read.ServeHTTP(bare, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	if got := bare.Body.String(); got != "" {
		t.Errorf("chrome Vault with nothing set = %q, want none", got)
	}
}

// TestHeaderNamesTheFolderBesideTheWordmark holds what the header draws for the
// name: a link to the desk after a rule, escaped like any text, and neither of
// them when the folder has no name to show.
func TestHeaderNamesTheFolderBesideTheWordmark(t *testing.T) {
	t.Parallel()
	render := func(name string) string {
		var buf bytes.Buffer
		if err := header(Chrome{Vault: name, Assets: asset.Versions{Token: "recorded0000"}}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render header: %v", err)
		}
		return buf.String()
	}

	named := render("Notes <&>")
	const rule, link = `<span class="y-brand__rule" aria-hidden="true"></span>`, `<a class="y-brand__vault" href="/">Notes &lt;&amp;&gt;</a>`
	if at, from := strings.Index(named, rule), strings.Index(named, link); at < 0 || from < at {
		t.Errorf("header of a named folder lacks %q, then %q; html = %q", rule, link, named)
	}
	unnamed := render("")
	if strings.Contains(unnamed, "y-brand__vault") || strings.Contains(unnamed, "y-brand__rule") {
		t.Errorf("header of a folder with no name draws a rule or a name; html = %q", unnamed)
	}
}
