package pages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The reading size is a persisted preference like the theme: the server stamps
// it from the cookie so the first paint is already the reader's size, and a
// cookie carrying anything unexpected falls back to the default — the value is
// user-controlled bytes, not an enum the server may trust.
func TestChromeFromRequestReadsTextSizePreference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cookieValue string
		want        string
	}{
		{name: "missing defaults to the base measure", want: "m"},
		{name: "large", cookieValue: "l", want: "l"},
		{name: "extra large", cookieValue: "xl", want: "xl"},
		{name: "an unknown value falls back", cookieValue: "xxl", want: "m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.cookieValue != "" {
				r.Header.Set("Cookie", "yomihon_textsize="+tt.cookieValue)
			}
			chrome := chromeFromRequest(r, "測試", 0, false)
			if chrome.TextSize != tt.want {
				t.Errorf("TextSize = %q, want %q", chrome.TextSize, tt.want)
			}
		})
	}
}
