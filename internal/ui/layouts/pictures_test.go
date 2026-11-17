package layouts

import (
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderPicture(t *testing.T, c templ.Component) (string, error) {
	t.Helper()
	var out strings.Builder
	err := c.Render(t.Context(), &out)
	return out.String(), err
}

func TestAPictureIsAMarkerThatCarriesItsNameAndIsHiddenFromReaders(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		c    templ.Component
		want string
	}{
		{"icon", Icon("bookmark"), `<span class="y-icon" data-icon="bookmark" aria-hidden="true"></span>`},
		{"icon with a hyphen", Icon("external-link"), `data-icon="external-link"`},
		{"still", Still("desk-first"), `<div class="y-still" data-still="desk-first" aria-hidden="true"></div>`},
		{"endpaper", Endpaper("endpaper-12"), `<div class="y-endpaper" data-endpaper="endpaper-12" aria-hidden="true"></div>`},
		{"ornament", Ornament(), `<div class="y-ornament" aria-hidden="true"></div>`},
	} {
		got, err := renderPicture(t, tc.c)
		if err != nil {
			t.Errorf("%s: render error = %v", tc.name, err)
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s renders %q, want it to contain %q", tc.name, got, tc.want)
		}
	}
}

// A name that is not in the set must stop the page rather than draw a blank,
// and a name of another kind of picture is not this kind's.
func TestAPictureNamedForNothingIsAnErrorNotABlank(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		c    templ.Component
	}{
		{"the design's name for finished", Icon("done")},
		{"an icon with its extension", Icon("bookmark.png")},
		{"an icon that is a still", Icon("still-desk-first")},
		{"a path", Icon("../art/still-desk-first")},
		{"nothing", Icon("")},
		{"a still that is an endpaper", Still("endpaper-01")},
		{"a still with its prefix", Still("still-desk-first")},
		{"an endpaper that is a still", Endpaper("still-desk-first")},
		{"an endpaper that is the ornament", Endpaper("ornament-end")},
		{"a thirteenth endpaper", Endpaper("endpaper-13")},
		{"an endpaper with no number", Endpaper("endpaper-")},
	} {
		got, err := renderPicture(t, tc.c)
		if err == nil {
			t.Errorf("%s: rendered %q, want an error", tc.name, got)
		}
		if got != "" {
			t.Errorf("%s: wrote %q before failing", tc.name, got)
		}
	}
}
