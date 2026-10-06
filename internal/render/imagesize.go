package render

import (
	"fmt"
	"html"
	"path"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

var imagePixelSize = regexp.MustCompile(`^\d+(?:x\d+)?$`)

func pictureEmbed(relPath string, link graph.Wikilink) string {
	alt := path.Base(relPath)
	size := ""
	if link.Aliased {
		size = imageSizeAttributes(link.Display)
		if size == "" {
			alt = link.Display
		}
	}
	//nolint:gocritic // sprintfQuotedString false positive: alt is HTML-escaped, the source is URL-escaped and attribute-escaped, and size admits only digits.
	return fmt.Sprintf(`<img src="%s" alt="%s"%s>`,
		attributeEscaper.Replace(rawHref(relPath)), html.EscapeString(alt), size)
}

// imageSizeAttributes accepts only whole pixel counts from the image dialect.
// The digits can enter quoted attributes directly; malformed sizes remain
// labels rather than becoming HTML. The reading column still caps the width.
func imageSizeAttributes(size string) string {
	if !imagePixelSize.MatchString(size) {
		return ""
	}
	width, height, both := strings.Cut(size, "x")
	attributes := ` width="` + width + `"`
	if both {
		attributes += ` height="` + height + `"`
	}
	return attributes
}
