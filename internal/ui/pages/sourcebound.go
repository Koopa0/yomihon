package pages

import (
	"encoding/base64"

	"github.com/a-h/templ"
)

// sourceBoundHealthID addresses one canonical captured path. Encoding every
// byte keeps URL-sensitive names distinct without introducing HTML whitespace.
func sourceBoundHealthID(path string) string {
	return "health-source-bound-" + base64.RawURLEncoding.EncodeToString([]byte(path))
}

// sourceBoundHealthHref opens the existing whole report so its target exists
// even when the finding falls beyond the first page.
func sourceBoundHealthHref(path string) string {
	return "/health?page=all#" + sourceBoundHealthID(path)
}

func sourceBoundHealthAttrs(id string) templ.Attributes {
	if id == "" {
		return nil
	}
	return templ.Attributes{"id": id, "class": "y-findings__target"}
}
