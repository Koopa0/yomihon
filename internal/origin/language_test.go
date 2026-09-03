package origin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestLanguageHonoursOneCookieValue holds the cookie's hygiene to the same
// rule the other reading preferences follow: one known value is honoured and
// everything else falls to the default, because a cookie is user-controllable
// and a language yomihon does not speak has no rendering to fall back on.
func TestLanguageHonoursOneCookieValue(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		value string
		want  wording.Lang
	}{
		{name: "no cookie at all", value: "", want: wording.ZhHant},
		{name: "the one other language", value: "en", want: wording.En},
		{name: "the default named outright", value: "zh-Hant", want: wording.ZhHant},
		{name: "a language yomihon does not speak", value: "ja", want: wording.ZhHant},
		{name: "a truncated value", value: "e", want: wording.ZhHant},
		{name: "a case the cookie never issued", value: "EN", want: wording.ZhHant},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.value != "" {
				r.Header.Set("Cookie", wording.CookieName+"="+tt.value)
			}
			if got := Language(r); got != tt.want {
				t.Errorf("Language(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
