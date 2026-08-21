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
			chrome := chromeFromRequest(r, "測試", Shell{})
			if chrome.SingleKeyShortcutsEnabled != tt.wantEnabled {
				t.Errorf("SingleKeyShortcutsEnabled = %t, want %t", chrome.SingleKeyShortcutsEnabled, tt.wantEnabled)
			}
		})
	}
}

// TestObsidianHref pins the editor hand-off URI byte-for-byte. Each expected
// string is hand-derived from the escaping rules, not read back from the
// escaper: spaces are %20 (a "+" would reach Obsidian as a literal plus),
// CJK is UTF-8 percent-escaped, and "?", "#", "%", and "&" are escaped so a
// name cannot cut the single query parameter short or start a second one.
func TestObsidianHref(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root string
		rel  string
		want string
	}{
		{
			name: "plain ascii path",
			root: "/Users/reader/vault",
			rel:  "Writing/lessons/japanese/L05.md",
			want: "obsidian://open?path=/Users/reader/vault/Writing/lessons/japanese/L05.md",
		},
		{
			name: "cjk and spaces are escaped",
			root: "/Users/reader/vault",
			rel:  "Concepts/golang/Go 位元運算.md",
			want: "obsidian://open?path=/Users/reader/vault/Concepts/golang/Go%20%E4%BD%8D%E5%85%83%E9%81%8B%E7%AE%97.md",
		},
		{
			name: "question mark cannot end the parameter",
			root: "/Users/reader/vault",
			rel:  "Notes/what is this?.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/what%20is%20this%3F.md",
		},
		{
			name: "hash cannot start a fragment",
			root: "/Users/reader/vault",
			rel:  "Notes/C#basics.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/C%23basics.md",
		},
		{
			name: "percent cannot start a stray escape",
			root: "/Users/reader/vault",
			rel:  "Notes/50% done.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/50%25%20done.md",
		},
		{
			name: "ampersand cannot start a second parameter",
			root: "/Users/reader/vault",
			rel:  "Notes/a&b.md",
			want: "obsidian://open?path=/Users/reader/vault/Notes/a%26b.md",
		},
		{
			name: "root segments are escaped too",
			root: "/Users/my reader/obsidian vault",
			rel:  "Notes/n.md",
			want: "obsidian://open?path=/Users/my%20reader/obsidian%20vault/Notes/n.md",
		},
		{name: "empty root yields no link", rel: "Notes/n.md", want: ""},
		{name: "empty rel yields no link", root: "/Users/reader/vault", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ObsidianHref(tt.root, tt.rel); got != tt.want {
				t.Errorf("ObsidianHref(%q, %q) = %q, want %q", tt.root, tt.rel, got, tt.want)
			}
		})
	}
}
