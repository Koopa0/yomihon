package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNoRailCarriesAControlOfItsOwn holds every rail to the shape the header's
// button relies on: the column is the one element the button names, and
// nothing inside it opens or folds it. A control inside the column would go
// with the column when it folds, and the reader would be left with nothing on
// screen to bring it back with; the header's button stays where it is.
func TestNoRailCarriesAControlOfItsOwn(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	rails := []struct {
		name string
		rail func(layouts.Chrome) templ.Component
	}{
		{"reading rail", func(c layouts.Chrome) templ.Component {
			return sidebar(NewNoteSidebar(nav.Shell{Nav: model}, "Writing/lessons/go/L01.md", "golang"), c)
		}},
		{"shared sidebar", func(c layouts.Chrome) templ.Component {
			return sidebar(NewLibrarySidebar(nav.Shell{Nav: model}, LibraryNone, ""), c)
		}},
		{"long folder", func(c layouts.Chrome) templ.Component {
			return sidebar(longFolderSidebar(t), c)
		}},
	}
	for _, r := range rails {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(r.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				if err := r.rail(layouts.Chrome{Nonce: "n", Lang: lang}).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render: %v", err)
				}
				html := buf.String()
				if got := strings.Count(html, `id="_y-nav-rail"`); got != 1 {
					t.Fatalf("the column is drawn with the id the header's button controls %d times, want exactly once", got)
				}
				_, aside, _ := strings.Cut(html, `id="_y-nav-rail"`)
				aside, _, _ = strings.Cut(aside, "</aside>")
				for _, control := range []string{"data-rail-toggle", "data-nav-toggle", "<button"} {
					if strings.Contains(aside, control) {
						t.Errorf("the column carries %q, a control of its own that would leave with it", control)
					}
				}
			})
		}
	}
}

// TestRailHeadIsTheFilterAlone keeps the head row to the one thing in it: the
// filter, drawn hidden for the script to reveal before the first paint, on the
// rails long enough to have one, and no row at all on the rails that are not.
// An empty sticky row would be a band of nothing standing over the tree.
func TestRailHeadIsTheFilterAlone(t *testing.T) {
	t.Parallel()
	c := layouts.Chrome{Nonce: "n", Lang: wording.En}
	short := NewFolderSidebar(nav.Shell{Nav: buildModel(t)}, "Notes/alpha.md")
	for _, tt := range []struct {
		name string
		s    Sidebar
		head bool
	}{
		{"short", short, false},
		{"long", longFolderSidebar(t), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var b bytes.Buffer
			if err := sidebar(tt.s, c).Render(t.Context(), &b); err != nil {
				t.Fatal(err)
			}
			html := b.String()
			if got := strings.Contains(html, `<div class="y-railhead" hidden>`); got != tt.head {
				t.Errorf("head row drawn hidden = %v, want %v", got, tt.head)
			}
			if got := strings.Contains(html, `<input class="y-filter"`); got != tt.head {
				t.Errorf("filter drawn = %v, want %v", got, tt.head)
			}
			if tt.head && !strings.Contains(html, "head.hidden = false") {
				t.Error("the script that settles the rail does not reveal the head row before the first paint")
			}
		})
	}
}
