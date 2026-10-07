package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestVersionedAssetURLChangesWithTheServedBytes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"first", "first", "/static/app.css?v=a7937b64b8ca"},
		{"second", "second", "/static/app.css?v=16367aacb67a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := (Versions{}).versionedURL("app.css", fixed(cssContentType, []byte(tt.body))); got != tt.want {
				t.Errorf("caught: versioned URL for %q = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestClientImportMapCoversEveryRegisteredModuleAndFacade(t *testing.T) {
	t.Parallel()
	got := decodedModuleImports(t, (Versions{}).ImportMap())
	want := make(map[string]string)
	for name, e := range registry {
		if strings.Contains(name, "/") || !(strings.HasSuffix(name, ".js") || name == "mermaid.esm.min.mjs") {
			continue
		}
		if e.contentType != jsContentType {
			t.Fatalf("registered client module %q has type %q, want %q", name, e.contentType, jsContentType)
		}
		sum := sha256.Sum256(e.body)
		want["/static/"+name] = "/static/" + name + "?v=" + hex.EncodeToString(sum[:])[:12]
	}
	if len(want) == 0 {
		t.Fatal("no registered client module was examined")
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: complete registry import map (-want +got):\n%s", diff)
	}
}

// Copies model two builds without altering the process's immutable registry.
// A change to an indirectly imported module must change only its own address.
func TestClientImportMapFollowsByteChangesWithoutIncludingChunks(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"first build", "first", "/static/child.js?v=a7937b64b8ca"},
		{"second build", "second", "/static/child.js?v=16367aacb67a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reg := map[string]entry{
				"yomihon.js":          fixed(jsContentType, []byte("first")),
				"child.js":            fixed(jsContentType, []byte(tt.body)),
				"app.css":             fixed(cssContentType, []byte("first")),
				"fonts/example.woff2": fixed(woff2ContentType, []byte("first")),
				"mermaid.esm.min.mjs": fixed(jsContentType, []byte("first")),
				"chunks/example.js":   fixed(jsContentType, []byte("first")),
				"not-a-module.js":     fixed("text/plain", []byte("first")),
			}
			want := map[string]string{
				"/static/yomihon.js":         "/static/yomihon.js?v=a7937b64b8ca",
				"/static/child.js":           tt.want,
				"/static/mermaid.esm.min.mjs": "/static/mermaid.esm.min.mjs?v=a7937b64b8ca",
			}
			if diff := cmp.Diff(want, decodedModuleImports(t, buildClientImportMap(reg))); diff != "" {
				t.Errorf("caught: changed module byte identity (-want +got):\n%s", diff)
			}
		})
	}
}

func decodedModuleImports(t *testing.T, data string) map[string]string {
	t.Helper()
	var value struct {
		Imports map[string]string `json:"imports"`
	}
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		t.Fatal(err)
	}
	return value.Imports
}

func TestFixedVersionsPreservePathsAndRegistryMembershipAcrossByteChanges(t *testing.T) {
	t.Parallel()
	versions := Versions{Token: "recorded0000"}
	for _, body := range []string{"first", "second"} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			reg := map[string]entry{
				"yomihon.js":          fixed(jsContentType, []byte("first")),
				"child.js":            fixed(jsContentType, []byte(body)),
				"app.css":             fixed(cssContentType, []byte(body)),
				"chunks/child.js":     fixed(jsContentType, []byte(body)),
				"mermaid.esm.min.mjs": fixed(jsContentType, []byte(body)),
				"wrong.js":            fixed("text/plain", []byte(body)),
			}
			want := map[string]string{
				"/static/yomihon.js":         "/static/yomihon.js?v=recorded0000",
				"/static/child.js":           "/static/child.js?v=recorded0000",
				"/static/mermaid.esm.min.mjs": "/static/mermaid.esm.min.mjs?v=recorded0000",
			}
			if diff := cmp.Diff(want, decodedModuleImports(t, versions.buildClientImportMap(reg))); diff != "" {
				t.Errorf("caught: fixed complete import map (-want +got):\n%s", diff)
			}
			if got := versions.versionedURL("app.css", reg["app.css"]); got != "/static/app.css?v=recorded0000" {
				t.Errorf("caught: fixed CSS address = %q, want /static/app.css?v=recorded0000", got)
			}
		})
	}
}

func TestVersionsNamesTheUnknownRegisteredAsset(t *testing.T) {
	t.Parallel()
	defer func() {
		if got := recover(); got != "asset: unknown URL name: missing.css" {
			t.Errorf("caught: unknown registered name panic = %v, want asset: unknown URL name: missing.css", got)
		}
	}()
	(Versions{Token: "recorded0000"}).URL("missing.css")
}
