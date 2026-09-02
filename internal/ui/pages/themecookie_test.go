package pages

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// The theme cookie has three honest answers, not two. A reader who chose gets
// their choice stamped on the root — either value, since an explicit "light"
// must keep beating a dark system preference after the stylesheet learns to
// read that preference. A reader who never chose gets no stamp at all: the
// stylesheet then follows the system, and a hardcoded "light" here is exactly
// the wrong first paint on every dark desk. Garbage falls with the unchosen,
// as every preference cookie's garbage does.
func TestChromeFromRequestReadsThemeChoice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cookieValue string
		want        string
	}{
		{name: "no choice leaves the root unstamped", want: ""},
		{name: "dark", cookieValue: "dark", want: "dark"},
		{name: "light is a choice too", cookieValue: "light", want: "light"},
		{name: "an unknown value is no choice", cookieValue: "sepia", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.cookieValue != "" {
				r.Header.Set("Cookie", "yomihon_theme="+tt.cookieValue)
			}
			if got := layouts.ChromeFromRequest(r, "測試").Theme; got != tt.want {
				t.Errorf("ChromeFromRequest theme = %q, want %q", got, tt.want)
			}
		})
	}
}
