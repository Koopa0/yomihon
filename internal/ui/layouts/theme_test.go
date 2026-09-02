package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestBaseStampsThemeOnlyWhenChosen pins the root element's three theme
// states. A chosen theme is stamped so the first paint is already right and an
// explicit "light" outranks a dark system preference in the stylesheet. No
// choice leaves the attribute off entirely — not empty — because the
// stylesheet's system-preference block applies to a root that carries no
// choice, and an empty attribute is a choice-shaped thing that chose nothing.
func TestBaseStampsThemeOnlyWhenChosen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		theme string
		want  string
	}{
		{name: "dark choice", theme: "dark", want: `data-theme="dark"`},
		{name: "light choice", theme: "light", want: `data-theme="light"`},
		{name: "no choice", theme: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Base(Chrome{Title: "測試", Theme: tt.theme}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render base: %v", err)
			}
			html := buf.String()
			if tt.want != "" {
				if !strings.Contains(html, tt.want) {
					t.Errorf("Base() root is missing %q", tt.want)
				}
				return
			}
			// The needle keeps its equals sign: the theme toggle button
			// legitimately carries data-theme-toggle on every page, and the
			// stamped choice is the only data-theme with a value.
			if strings.Contains(html, `data-theme="`) {
				at := strings.Index(html, `data-theme="`)
				t.Errorf("Base() stamps a theme nobody chose; near %q", html[at:at+40])
			}
		})
	}
}

// TestTextSizeControlCarriesItsThreeNamesForTheScript pins the label source
// the client runtime reads. The server names the control's current size in
// aria-label; the script that cycles sizes — and the one that re-syncs after
// a back/forward-cache restore — renames it from these three attributes, in
// the page's own language. Without them the script would need its own copy of
// the words, and a restored English page would be relabeled from somebody
// else's dictionary.
func TestTextSizeControlCarriesItsThreeNamesForTheScript(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		chrome Chrome
		want   []string
	}{
		{
			name:   "in the default language",
			chrome: Chrome{},
			want:   []string{`data-label-m="字級：中"`, `data-label-l="字級：大"`, `data-label-xl="字級：特大"`},
		},
		{
			name:   "in English",
			chrome: Chrome{Lang: wording.En},
			want:   []string{`data-label-m="Text size: medium"`, `data-label-l="Text size: large"`, `data-label-xl="Text size: extra large"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := header(tt.chrome).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render header: %v", err)
			}
			html := buf.String()
			control := textSizeButton(html)
			for _, want := range tt.want {
				if !strings.Contains(control, want) {
					t.Errorf("text-size control is missing %q; control = %q", want, control)
				}
			}
		})
	}
}
