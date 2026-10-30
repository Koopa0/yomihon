package layouts

import (
	"bytes"
	"strings"
	"testing"
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
			// The needle keeps its equals sign: the stamped choice is the
			// only data-theme with a value.
			if strings.Contains(html, `data-theme="`) {
				at := strings.Index(html, `data-theme="`)
				t.Errorf("Base() stamps a theme nobody chose; near %q", html[at:at+40])
			}
		})
	}
}
