package layouts

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
			chrome := ChromeFromRequest(r, "測試")
			if chrome.TextSize != tt.want {
				t.Errorf("TextSize = %q, want %q", chrome.TextSize, tt.want)
			}
		})
	}
}

func TestChromeFromRequestReadsSingleKeyShortcutPreference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cookieValue string
		wantEnabled bool
	}{
		{name: "missing defaults on", wantEnabled: true},
		{name: "enabled", cookieValue: "on", wantEnabled: true},
		{name: "disabled", cookieValue: "off"},
		{name: "invalid defaults on", cookieValue: "disabled", wantEnabled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.cookieValue != "" {
				r.Header.Set("Cookie", "yomihon_shortcuts="+tt.cookieValue)
			}
			chrome := ChromeFromRequest(r, "測試")
			if chrome.SingleKeyShortcutsEnabled != tt.wantEnabled {
				t.Errorf("SingleKeyShortcutsEnabled = %t, want %t", chrome.SingleKeyShortcutsEnabled, tt.wantEnabled)
			}
		})
	}
}

// The furigana preference is the one reading preference with no test of how it
// is read. Its cookie is the only one whose default is on, so the row that
// matters most is the missing one: a reader who has never touched the control
// gets the aids, and only the one stored word turns them off.
func TestChromeFromRequestReadsFuriganaPreference(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		cookieValue string
		want        bool
	}{
		{name: "missing leaves the aids on", want: true},
		{name: "the one word that turns them off", cookieValue: "off", want: false},
		{name: "on", cookieValue: "on", want: true},
		{name: "an unknown value leaves them on", cookieValue: "maybe", want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.cookieValue != "" {
				r.Header.Set("Cookie", "yomihon_ruby="+tt.cookieValue)
			}
			if got := ChromeFromRequest(r, "測試").RubyEnabled; got != tt.want {
				t.Errorf("RubyEnabled = %v, want %v", got, tt.want)
			}
		})
	}
}
