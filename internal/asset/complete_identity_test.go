package asset

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	projectassets "github.com/koopa0/yomihon/assets"
)

func TestImportMapVersionsTheCompleteModuleSet(t *testing.T) {
	t.Parallel()
	want := make(map[string]string)
	err := fs.WalkDir(projectassets.Files, "js", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.Count(name, "/") == 1 && strings.HasSuffix(name, ".js") || name == "js/mermaid/mermaid.esm.min.mjs") {
			return nil
		}
		key := strings.TrimPrefix(name, "js/")
		if name == "js/mermaid/mermaid.esm.min.mjs" {
			key = "mermaid.esm.min.mjs"
		}
		response := identityResponse(t, "/static/"+key)
		sum := sha256.Sum256(response.Body.Bytes())
		want["/static/"+key] = "/static/" + key + "?v=" + hex.EncodeToString(sum[:])[:12]
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded modules: %v", err)
	}
	if len(want) != 19 {
		t.Fatalf("embedded native modules and Mermaid facade = %d, want 19", len(want))
	}
	if diff := cmp.Diff(want, decodedModuleImports(t, (Versions{}).ImportMap())); diff != "" {
		t.Errorf("caught: complete module addresses (-want +got):\n%s", diff)
	}
}

func TestCacheAuthorityUsesCurrentBytesAndExcludesEveryChunk(t *testing.T) {
	t.Parallel()
	chunks := 0
	for name := range registry {
		if strings.HasPrefix(name, "chunks/") {
			chunks++
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bare := identityResponse(t, "/static/"+name)
			sum := sha256.Sum256(bare.Body.Bytes())
			current := hex.EncodeToString(sum[:])[:12]
			for _, tt := range []struct {
				name  string
				token string
				want  string
			}{
				{name: "bare", want: "no-cache"},
				{name: "stale", token: "000000000000", want: "no-cache"},
				{name: "recording", token: "recorded0000", want: "no-cache"},
				{name: "current", token: current, want: "max-age=31536000, immutable"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					address := "/static/" + name
					if tt.token != "" {
						address += "?v=" + tt.token
					}
					want := tt.want
					if strings.HasPrefix(name, "chunks/") {
						want = "no-cache"
					}
					response := identityResponse(t, address)
					if got := response.Header().Get("Cache-Control"); got != want {
						t.Errorf("caught: GET %s Cache-Control = %q, want %q", address, got, want)
					}
					if diff := cmp.Diff(bare.Body.Bytes(), response.Body.Bytes()); diff != "" {
						t.Errorf("GET %s changed bytes (-want +got):\n%s", address, diff)
					}
				})
			}
		})
	}
	if chunks == 0 {
		t.Fatal("no registered Mermaid chunk examined")
	}
}

func identityResponse(t *testing.T, address string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody))
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("GET %s status = %d, body bytes = %d, want 200 and nonempty body", address, response.Code, response.Body.Len())
	}
	return response
}
