package layouts

import (
	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/asset"
)

// An import map is JSON data signed by the same page nonce as its module
// entry. The JSON encoder escapes HTML delimiters; the nonce is an attribute
// value and must be escaped separately.
func importMapScript(nonce string) templ.Component {
	return templ.Raw(`<script type="importmap" nonce="` + templ.EscapeString(nonce) + `">` + asset.ImportMap() + `</script>`)
}
