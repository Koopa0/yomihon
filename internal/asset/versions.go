package asset

import (
	"encoding/json"
	"fmt"
	"strings"
)

// URL identifies the registered bytes in the address itself. A cache that
// overrides revalidation still has to fetch a new address when a build changes
// those bytes. Unknown names are a template's programmer error, not a request.
func URL(name string) string {
	e, known := registry[name]
	if !known {
		panic("asset: unregistered URL name: " + name)
	}
	return versionedURL(name, e)
}

func versionedURL(name string, e entry) string {
	return "/static/" + name + "?v=" + e.etag[1:13]
}

var clientImportMap = buildClientImportMap(registry)

// ImportMap is the immutable JSON projection of the registered native client
// modules. Relative imports resolve to these absolute path keys, so the
// modules reached through the entry receive the same byte identities too.
// Vendored mermaid modules retain their vendored-version addresses.
func ImportMap() string {
	return clientImportMap
}

func buildClientImportMap(reg map[string]entry) string {
	imports := make(map[string]string)
	for name, e := range reg {
		if e.contentType == jsContentType && strings.HasSuffix(name, ".js") && !strings.Contains(name, "/") {
			imports["/static/"+name] = versionedURL(name, e)
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
