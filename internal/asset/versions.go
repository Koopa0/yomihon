package asset

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Versions projects addresses from the immutable asset registry. Its zero
// value uses the served bytes' hashes; a fixed token changes only the version
// value, retaining the registered paths and complete module set.
type Versions struct {
	Token string // empty uses the served-byte hash
}

// URL identifies the registered bytes in the address itself, using this
// version selection. A cache that overrides revalidation still has to fetch a
// new address when a build changes those bytes. Unknown names are a
// template's programmer error, not a request.
func (v Versions) URL(name string) string {
	e, known := registry[name]
	if !known {
		panic("asset: unknown URL name: " + urlName(name).String())
	}
	return v.versionedURL(name, e)
}

type urlName string

func (n urlName) String() string { return string(n) }

func (v Versions) versionedURL(name string, e entry) string {
	token := v.Token
	if token == "" {
		token = versionToken(e)
	}
	return "/static/" + name + "?v=" + url.QueryEscape(token)
}

var clientImportMap = buildClientImportMap(registry)

// versionToken is shared by URL projection, stylesheet rewriting and cache
// authority; each entry's validator was computed from its final served bytes.
func versionToken(e entry) string { return e.etag[1:13] }

// ImportMap projects every native client module and the vendored Mermaid
// facade with this version selection. Relative imports resolve to absolute path
// keys, so the modules reached through the entry receive the same byte
// identities too. Changing the token cannot change module membership.
// Mermaid's chunks retain their authored content-hash filenames.
func (v Versions) ImportMap() string {
	if v.Token == "" {
		return clientImportMap
	}
	return v.buildClientImportMap(registry)
}

func buildClientImportMap(reg map[string]entry) string {
	return (Versions{}).buildClientImportMap(reg)
}

func (v Versions) buildClientImportMap(reg map[string]entry) string {
	imports := make(map[string]string)
	for name, e := range reg {
		if e.contentType == jsContentType && !strings.Contains(name, "/") && (strings.HasSuffix(name, ".js") || name == "mermaid.esm.min.mjs") {
			imports["/static/"+name] = v.versionedURL(name, e)
		}
	}
	data, err := json.Marshal(struct {
		Imports map[string]string `json:"imports"`
	}{Imports: imports})
	if err != nil {
		panic(fmt.Sprintf("asset: encoding client import map: %v", err))
	}
	return string(data)
}
