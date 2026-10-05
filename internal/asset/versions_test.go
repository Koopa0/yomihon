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
			if got := versionedURL("app.css", fixed(cssContentType, []byte(tt.body))); got != tt.want {
				t.Errorf("caught: versioned URL for %q = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestClientImportMapCoversEveryRegisteredOwnModule(t *testing.T) {
	t.Parallel()
	got := decodedModuleImports(t, ImportMap())
	want := make(map[string]string)
	for name, e := range registry {
		if strings.Contains(name, "/") || !strings.HasSuffix(name, ".js") {
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
func TestClientImportMapFollowsByteChangesWithoutIncludingVendoredResources(t *testing.T) {
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
				"/static/yomihon.js": "/static/yomihon.js?v=a7937b64b8ca",
				"/static/child.js":   tt.want,
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
