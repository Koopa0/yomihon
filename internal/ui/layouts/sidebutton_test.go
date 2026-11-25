package layouts

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/asset"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheSidebarButtonLeadsTheHeaderRow holds where the one control for the
// left column stands: first in the header's row, ahead of the wordmark, over
// the column it acts on, naming the column by id.
func TestTheSidebarButtonLeadsTheHeaderRow(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{Lang: wording.En, Rail: "collapsed", Assets: asset.Versions{Token: "recorded0000"}}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	if got := strings.Count(html, "data-nav-toggle"); got != 1 {
		t.Fatalf("the header draws %d sidebar buttons, want exactly 1", got)
	}
	button, brand := strings.Index(html, "data-nav-toggle"), strings.Index(html, `class="y-brand"`)
	if brand < 0 || button > brand {
		t.Errorf("the sidebar button stands at %d, after the wordmark at %d; it leads the row", button, brand)
	}
	if !strings.Contains(html, `aria-controls="_y-nav-rail"`) {
		t.Error("the sidebar button does not name the column it acts on")
	}
}

// TestTheSidebarButtonStatesTheColumnFromTheCookie holds what the button says
// on the first byte to what the request carried: aria-expanded is the wide
// window's column, the name does not change with it, and the tooltip names the
// action from that state. A column the reader never opened is folded. The key
// is advertised, in the tooltip and in aria-keyshortcuts, only while single-key
// shortcuts are on.
func TestTheSidebarButtonStatesTheColumnFromTheCookie(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rail     string
		keys     bool
		expanded string
		action   func(wording.Lang) string
	}{
		{"collapsed", true, "false", wording.ShowRail.In},
		{"open", true, "true", wording.HideRail.In},
		{"open", false, "true", wording.HideRail.In},
		{"collapsed", false, "false", wording.ShowRail.In},
	}
	for _, tt := range tests {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(fmt.Sprintf("%s/%v/%s", tt.rail, tt.keys, lang), func(t *testing.T) {
				t.Parallel()
				var buf bytes.Buffer
				c := Chrome{Lang: lang, Rail: tt.rail, SingleKeyShortcutsEnabled: tt.keys, Assets: asset.Versions{Token: "recorded0000"}}
				if err := header(c).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render: %v", err)
				}
				html, _, _ := strings.Cut(buf.String(), "</button>")
				title := tt.action(lang)
				if tt.keys {
					title += wording.RailKeyHint.In(lang)
				}
				for _, want := range []string{
					`aria-expanded="` + tt.expanded + `"`,
					`aria-labelledby="_y-sidebtn-word"`,
					`title="` + title + `"`,
					`data-title-hide="` + wording.HideRail.In(lang) + `"`,
					`data-title-show="` + wording.ShowRail.In(lang) + `"`,
					`data-title-key="` + wording.RailKeyHint.In(lang) + `"`,
					`>` + wording.SidebarButton.In(lang) + `</span>`,
				} {
					if !strings.Contains(html, want) {
						t.Errorf("sidebar button missing %q; html = %q", want, html)
					}
				}
				if got := strings.Contains(html, `aria-keyshortcuts="["`); got != tt.keys {
					t.Errorf("aria-keyshortcuts advertised = %v with single keys %v", got, tt.keys)
				}
			})
		}
	}
}
