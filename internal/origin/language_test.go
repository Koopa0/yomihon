package origin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestLanguageFollowsTheBrowserOnlyWhenNothingWasChosen holds the order the
// interface decides its language in: a stored choice, even the default, beats
// what the browser asks for, and a stored value the dictionary does not know
// is no choice at all.
func TestLanguageFollowsTheBrowserOnlyWhenNothingWasChosen(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		cookie string
		header string
		want   wording.Lang
	}{
		{name: "nothing chosen, an English browser", header: "en-US,en;q=0.9", want: wording.En},
		{name: "nothing chosen, a Chinese browser", header: "zh-TW,zh;q=0.9,en;q=0.8", want: wording.ZhHant},
		{name: "nothing chosen, no header", want: wording.ZhHant},
		{name: "Traditional Chinese chosen, an English browser", cookie: "zh-Hant", header: "en", want: wording.ZhHant},
		{name: "English chosen, a Chinese browser", cookie: "en", header: "zh-TW", want: wording.En},
		{name: "a stored value that is not a choice", cookie: "ja", header: "en", want: wording.En},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			if tt.cookie != "" {
				r.Header.Set("Cookie", wording.CookieName+"="+tt.cookie)
			}
			if tt.header != "" {
				r.Header.Set("Accept-Language", tt.header)
			}
			if got := Language(r); got != tt.want {
				t.Errorf("Language(cookie %q, Accept-Language %q) = %q, want %q", tt.cookie, tt.header, got, tt.want)
			}
		})
	}
}

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
