package layouts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/asset"
)

// A cache may replace the origin's revalidation policy. Its lookup key must
// therefore carry the identity of the exact bytes the page asks it to serve.
func TestBaseVersions(t *testing.T) {
	t.Parallel()
	checkBaseVersions(t, asset.Versions{}, "")
}

func TestBaseUsesOnlyTheFixedVersionValues(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, token, query string
	}{
		{"recording", "recorded0000", "v=recorded0000"},
		{"HTML delimiters", `</script>&"`, "v=%3C%2Fscript%3E%26%22"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkBaseVersions(t, asset.Versions{Token: tt.token}, tt.query)
		})
	}
}

func checkBaseVersions(t *testing.T, versions asset.Versions, query string) {
	t.Helper()
	var buf bytes.Buffer
	const nonce = "response-nonce"
	if err := Base(Chrome{Title: "asset versions", Nonce: nonce, Assets: versions}).Render(t.Context(), &buf); err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	asset.Register(mux)
	var styles, entries, maps int
	var imports map[string]string
	var mapAt, entryAt int
	position := 0
	visit := func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		attrs := make(map[string]string)
		for _, attr := range node.Attr {
			attrs[attr.Key] = attr.Val
		}
		switch {
		case node.Data == "link" && attrs["rel"] == "stylesheet":
			styles++
			assertServedAssetVersion(t, mux, attrs["href"], query)
		case node.Data == "script" && attrs["type"] == "module":
			entries++
			entryAt = position
			assertServedAssetVersion(t, mux, attrs["src"], query)
		case node.Data == "script" && attrs["type"] == "importmap":
			maps++
			mapAt = position
			if attrs["nonce"] != nonce {
				t.Errorf("caught: import map nonce = %q, want %q", attrs["nonce"], nonce)
			}
			var data struct {
				Imports map[string]string `json:"imports"`
			}
			if node.FirstChild == nil {
				t.Fatal("caught: import map has no JSON body")
			}
			if decodeErr := json.Unmarshal([]byte(node.FirstChild.Data), &data); decodeErr != nil {
				t.Fatalf("caught: import map is not JSON: %v", decodeErr)
			}
			var keys map[string]json.RawMessage
			if membersErr := json.Unmarshal([]byte(node.FirstChild.Data), &keys); membersErr != nil {
				t.Fatal(membersErr)
			}
			if len(keys) != 1 || keys["imports"] == nil {
				t.Errorf("caught: import map has unexpected top-level members: %v", keys)
			}
			imports = data.Imports
		}
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		position++
		visit(node)
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if styles != 2 || entries != 1 || maps != 1 || mapAt >= entryAt {
		t.Fatalf("caught: asset declarations = styles %d, entries %d, maps %d (map at %d, entry at %d); want two styles and one map before one entry", styles, entries, maps, mapAt, entryAt)
	}
	files, err := assets.Files.ReadDir("js")
	if err != nil {
		t.Fatal(err)
	}
	examined := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".js") {
			continue
		}
		examined++
		key := "/static/" + file.Name()
		value, present := imports[key]
		if !present {
			t.Errorf("caught: import map omits client module %q", key)
			continue
		}
		assertServedAssetVersion(t, mux, value, query)
		parsed, err := url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Path != key {
			t.Errorf("caught: import map maps %q to a different module %q", key, value)
		}
	}
	if examined == 0 || len(imports) != examined {
		t.Errorf("caught: import map has %d members, want every one of %d embedded client modules and nothing else", len(imports), examined)
	}
	t.Log("invoked: complete Base registry asset projection")
}

func assertServedAssetVersion(t *testing.T, mux *http.ServeMux, address, query string) {
	t.Helper()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(parsed.Path, "/static/") {
		t.Fatalf("caught: asset URL %q does not name a registered static path", address)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody))
	if response.Code != http.StatusOK {
		t.Fatalf("caught: GET %q status = %d, want 200", address, response.Code)
	}
	sum := sha256.Sum256(response.Body.Bytes())
	want := "v=" + hex.EncodeToString(sum[:])[:12]
	if query != "" {
		want = query
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.Fragment != "" || parsed.RawQuery != want {
		t.Errorf("caught: asset URL %q does not carry the expected version query %q", address, want)
	}
}

func TestImportMapScriptKeepsTheNonceInsideOneAttribute(t *testing.T) {
	t.Parallel()
	const nonce = `response"><script>alert(1)</script>`
	var buf bytes.Buffer
	if err := importMapScript(nonce, asset.Versions{}).Render(t.Context(), &buf); err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var nonces []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "script" {
			value := ""
			for _, attr := range node.Attr {
				if attr.Key == "nonce" {
					value = attr.Val
				}
			}
			nonces = append(nonces, value)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(nonces) != 1 || nonces[0] != nonce {
		t.Errorf("caught: nonce escaped its import-map attribute: script nonces = %q, want one %q", nonces, nonce)
	}
}
