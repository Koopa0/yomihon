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

// URL identifies the registered bytes in the address itself. A cache that
// overrides revalidation still has to fetch a new address when a build changes
// those bytes. Unknown names are a template's programmer error, not a request.
func URL(name string) string {
	return (Versions{}).URL(name)
}

// URL projects a registered asset's address using this version selection.
// Unknown names remain a template's programmer error, never a request lookup.
func (v Versions) URL(name string) string {
	e, known := registry[name]
	if !known {
		panic("asset: unknown URL name: " + urlName(name).String())
	}
	return v.versionedURL(name, e)
}

type urlName string

func (n urlName) String() string { return string(n) }

func versionedURL(name string, e entry) string {
	return (Versions{}).versionedURL(name, e)
}

func (v Versions) versionedURL(name string, e entry) string {
	token := v.Token
	if token == "" {
		token = e.etag[1:13]
	}
	return "/static/" + name + "?v=" + url.QueryEscape(token)
}

var clientImportMap = buildClientImportMap(registry)

// ImportMap is the immutable JSON projection of the registered native client
// modules. Relative imports resolve to these absolute path keys, so the
// modules reached through the entry receive the same byte identities too.
// The vendored Mermaid facade remains unversioned; its chunks keep their
// authored content-hash filenames.
func ImportMap() string {
	return (Versions{}).ImportMap()
}

// ImportMap projects every registered native client module with this version
// selection. Changing the token cannot change module membership.
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
		if e.contentType == jsContentType && strings.HasSuffix(name, ".js") && !strings.Contains(name, "/") {
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
