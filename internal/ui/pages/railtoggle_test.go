package pages

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEveryRailCarriesTheFoldControlBesideItsPanel holds the three rails to one
// shape: the button that folds the column is the first thing in the rail and a
// sibling of the panel it controls, never inside what it hides, and the panel
// is the one element the button's aria-controls names. A rail that kept its
// contents outside the panel would leave the fold with nothing to hide; a
// button inside it would go with the column and leave nothing to bring it back.
func TestEveryRailCarriesTheFoldControlBesideItsPanel(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	rails := []struct {
		name string
		rail func(layouts.Chrome) templ.Component
	}{
		{"reading rail", func(c layouts.Chrome) templ.Component {
			return readingRail(NewReadingRail(nav.Shell{Nav: model}, "Writing/lessons/go/L01.md", "golang"), c)
		}},
		{"shared sidebar", func(c layouts.Chrome) templ.Component {
			return sidebar(NewSidebar(nav.Shell{Nav: model}, ""), c)
		}},
		{"study-path rail", func(c layouts.Chrome) templ.Component {
			return syllabusRail(PathView{}, c)
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
				if got := strings.Count(html, "data-rail-toggle"); got != 1 {
					t.Fatalf("%d fold controls, want exactly one", got)
				}
				toggle := strings.Index(html, "data-rail-toggle")
				body := strings.Index(html, `id="_y-nav-rail-body"`)
				if body < 0 {
					t.Fatal("the rail has no panel with id _y-nav-rail-body")
				}
				if strings.Count(html, `id="_y-nav-rail-body"`) != 1 {
					t.Error("the panel id is not unique")
				}
				if toggle > body {
					t.Error("the fold control comes after the panel it controls; it has to lead the rail")
				}
				panel := html[body:]
				panel, _, _ = strings.Cut(panel, "</aside>")
				if strings.Contains(panel, "data-rail-toggle") {
					t.Error("the fold control is inside the panel it hides")
				}
				if !strings.Contains(html, `aria-controls="_y-nav-rail-body"`) {
					t.Error("the fold control does not name the panel")
				}
				if !strings.Contains(html, `<div class="y-railhead" hidden>`) {
					t.Error("the rail's head row is not drawn hidden; without script a press would do nothing")
				}
				if !strings.Contains(html, "head.hidden = false") {
					t.Error("the script that settles the rail does not reveal the control before the first paint")
				}
				// The panel closes before the rail does, so nothing but the panel
				// and the control is a child of the aside.
				if !strings.Contains(html, "</div></aside>") {
					t.Error("the panel does not close directly before the rail")
				}
			})
		}
	}
}

// TestFoldControlStatesTheColumnFromTheCookie holds what the button says on the
// first byte to what the request carried: aria-expanded is the column's state,
// the name does not change with it, and the tooltip names the action from that
// state. The fallback is a column that is shown. The key is advertised, in the
// tooltip and in aria-keyshortcuts, only while single-key shortcuts are on.
func TestFoldControlStatesTheColumnFromTheCookie(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rail     string
		keys     bool
		expanded string
		action   func(wording.Lang) string
	}{
		{"", true, "true", wording.HideRail.In},
		{"open", true, "true", wording.HideRail.In},
		{"collapsed", true, "false", wording.ShowRail.In},
		{"open", false, "true", wording.HideRail.In},
		{"collapsed", false, "false", wording.ShowRail.In},
	}
	for _, tt := range tests {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(fmt.Sprintf("%s/%v/%s", tt.rail, tt.keys, lang), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				if err := railHead(layouts.Chrome{Lang: lang, Rail: tt.rail, SingleKeyShortcutsEnabled: tt.keys}, true).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render: %v", err)
				}
				html := buf.String()
				title := tt.action(lang)
				if tt.keys {
					title += wording.RailKeyHint.In(lang)
				}
				for _, want := range []string{
					`aria-expanded="` + tt.expanded + `"`,
					`aria-label="` + wording.ToggleRail.In(lang) + `"`,
					`title="` + title + `"`,
					`data-title-hide="` + wording.HideRail.In(lang) + `"`,
					`data-title-show="` + wording.ShowRail.In(lang) + `"`,
					`data-title-key="` + wording.RailKeyHint.In(lang) + `"`,
				} {
					if !strings.Contains(html, want) {
						t.Errorf("fold control missing %q; html = %q", want, html)
					}
				}
				if got := strings.Contains(html, `aria-keyshortcuts="["`); got != tt.keys {
					t.Errorf("aria-keyshortcuts advertised = %v with single keys %v", got, tt.keys)
				}
			})
		}
	}
}

// TestRailHeadHoldsTheFilterBesideTheButton keeps the filter in the head row,
// after the button and outside the panel, on the rails that have one, and off
// the study-path rail that never had one.
func TestRailHeadHoldsTheFilterBesideTheButton(t *testing.T) {
	t.Parallel()
	var with, without bytes.Buffer
	if err := railHead(layouts.Chrome{Lang: wording.En}, true).Render(t.Context(), &with); err != nil {
		t.Fatal(err)
	}
	if err := railHead(layouts.Chrome{Lang: wording.En}, false).Render(t.Context(), &without); err != nil {
		t.Fatal(err)
	}
	button, filter := strings.Index(with.String(), "data-rail-toggle"), strings.Index(with.String(), "data-nav-filter")
	if button < 0 || filter < button {
		t.Errorf("the filter does not follow the button in the head row: button at %d, filter at %d", button, filter)
	}
	if strings.Contains(without.String(), "data-nav-filter") {
		t.Error("a rail without a filter drew one")
	}
}
