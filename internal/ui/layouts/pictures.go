package layouts

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/asset"
)

// Icon draws the pictogram called name — the file name under assets/icons
// without its .png, such as "bookmark" or "external-link". It is the only way a
// page gets a pictogram.
//
// An icon is decorative. It takes the ink of the place it sits in and the size
// that place names (--icon-size), and a control made only of an icon holds its
// own aria-label and title, so the icon cannot be dropped in as an unlabelled
// image.
//
// A name that is not one of the set is an error from the component, not a blank
// where the page meant a picture; the page tests render every page.
func Icon(name string) templ.Component {
	if !asset.HasPicture("icons", name) {
		return unknownPicture("icon", name)
	}
	return icon(name)
}

// Still draws one of the small stills an empty page or a threshold carries:
// "desk-first", "stand-empty", "catalogue-empty", "not-found", "unreadable" or
// "reading". It is ink only and says nothing to a screen reader; the words
// beside it say the state.
func Still(name string) templ.Component {
	if !asset.HasPicture("art", "still-"+name) {
		return unknownPicture("still", name)
	}
	return still(name)
}

// Endpaper draws the band at the top of a course page. number is the endpaper
// the rules file gives a domain, 1 to 12, which is the only way a page names
// one: the picture's own name is built here.
func Endpaper(number int) templ.Component {
	name := fmt.Sprintf("endpaper-%02d", number)
	if !asset.HasPicture("art", name) {
		return unknownPicture("endpaper", name)
	}
	return endpaper(name)
}

// Ornament draws the mark that closes a note.
func Ornament() templ.Component {
	return ornament()
}

func unknownPicture(kind, name string) templ.Component {
	return templ.ComponentFunc(func(context.Context, io.Writer) error {
		return fmt.Errorf("layouts: no %s is named %q", kind, name)
	})
}
