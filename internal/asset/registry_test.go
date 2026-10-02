package asset

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	projectassets "github.com/koopa0/yomihon/assets"
)

func TestEveryRegisteredAssetServesItsExactEntry(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Register(mux)
	for name, want := range registry {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/"+name, http.NoBody)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET /static/%s status = %d, want %d", name, response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != want.contentType {
				t.Errorf("GET /static/%s Content-Type = %q, want %q", name, got, want.contentType)
			}
			if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("GET /static/%s X-Content-Type-Options = %q, want nosniff", name, got)
			}
			if got := response.Body.Bytes(); len(got) == 0 {
				t.Errorf("GET /static/%s body is empty", name)
			}
		})
	}
}

func TestProductScriptRegistryIsExact(t *testing.T) {
	t.Parallel()
	want := []string{
		"contents.js",
		"diagrams.js",
		"drawer.js",
		"freshness.js",
		"langform.js",
		"lesson.js",
		"mark.js",
		"preferences.js",
		"preview.js",
		"rail.js",
		"search.js",
		"shortcuts.js",
		"sidebar.js",
		"thought.js",
		"uncertainty.js",
		"yomihon.js",
	}
	var got []string
	for name := range registry {
		if strings.HasSuffix(name, ".js") {
			got = append(got, name)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("registered product scripts = %q, want exact closed set %q", got, want)
	}
}

func TestBrandSVGRegistryIsExact(t *testing.T) {
	t.Parallel()

	var got []string
	for name := range registry {
		if strings.HasSuffix(name, ".svg") {
			got = append(got, name)
		}
	}
	slices.Sort(got)
	want := []string{"yomihon-mark.svg"}
	if !slices.Equal(got, want) {
		t.Errorf("registered SVG assets = %q, want exact closed set %q", got, want)
	}
	registered, ok := registry["yomihon-mark.svg"]
	if !ok {
		t.Fatal("canonical brand mark is absent from the registry")
	}
	embedded, err := projectassets.Files.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read embedded brand mark: %v", err)
	}
	if !bytes.Equal(registered.body, embedded) {
		t.Error("registered brand mark body differs from the canonical embedded bytes")
	}
}

// unjoinableAtRule matches the at-rules a stylesheet may only carry at its own
// head: an @import or @charset in the middle of the served bundle is dropped by
// the browser, along with whatever it was meant to bring in.
var unjoinableAtRule = regexp.MustCompile(`@(?:import|charset|namespace)\b`)

// cssComment matches one comment, which may mention those at-rules without
// carrying one.
var cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// TestStylesheetIsItsPartsJoinedInOrder holds the one stylesheet the pages link
// to against the four files it is made of: each present once, in the order the
// sheet is read, base first and components last. The order is spelled out here
// rather than read from stylesheetParts, so that changing it in one place is a
// failure and not two files agreeing with each other.
func TestStylesheetIsItsPartsJoinedInOrder(t *testing.T) {
	t.Parallel()
	order := []string{"css/reset.css", "css/fonts.css", "css/tokens.css", "css/components.css"}

	mux := http.NewServeMux()
	Register(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", http.NoBody))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /static/app.css status = %d, want %d", response.Code, http.StatusOK)
	}
	served := response.Body.Bytes()

	parts := make([][]byte, len(order))
	total := len(order) - 1 // one newline between each pair of parts, and nothing else
	for i, name := range order {
		b, err := projectassets.Files.ReadFile(name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		if len(b) == 0 {
			t.Fatalf("embedded %s is empty, so its place in the bundle proves nothing", name)
		}
		parts[i] = b
		total += len(b)
	}

	if !bytes.HasPrefix(served, parts[0]) {
		t.Errorf("the served stylesheet does not begin with %s, so it no longer opens with its base", order[0])
	}
	rest := served
	for i, part := range parts {
		at := bytes.Index(rest, part)
		if at < 0 {
			t.Errorf("the served stylesheet carries no %s after %s, so the parts are missing or out of order", order[i], strings.Join(order[:i], ", "))
			break
		}
		rest = rest[at+len(part):]
	}
	if len(served) != total {
		t.Errorf("the served stylesheet is %d bytes, want %d: the four parts and one newline between each, with nothing added or repeated", len(served), total)
	}

	for _, at := range unjoinableAtRule.FindAllString(cssComment.ReplaceAllString(string(served), ""), -1) {
		t.Errorf("the served stylesheet carries %s outside a comment; joined mid-sheet it is ignored by the browser, so the part that wrote it has to be self-contained", at)
	}
}
