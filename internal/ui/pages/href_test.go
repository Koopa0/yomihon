package pages

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			chrome := chromeFromRequest(r, "測試", 0, false)
			if chrome.SingleKeyShortcutsEnabled != tt.wantEnabled {
				t.Errorf("SingleKeyShortcutsEnabled = %t, want %t", chrome.SingleKeyShortcutsEnabled, tt.wantEnabled)
			}
		})
	}
}
