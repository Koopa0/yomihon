package layouts

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/wording"
)

// storedChoice describes one reading choice the way an outside reader of the
// interface would have to describe it: by the cookie name on the wire, the
// values that cookie may carry, the answer when it carries none of them, and
// what the chrome then says. Everything on the left of this table is written
// out as a literal, because a cookie name and a cookie value are what a
// browser sends — comparing the code's own row against itself would agree with
// any rename, including one that silently retires a reader's stored choice.
type storedChoice struct {
	cookie   string
	values   []string
	fallback string
	unknown  []string
	answer   func(Chrome) string
}

func storedChoices() []storedChoice {
	return []storedChoice{
		{
			cookie:   "yomihon_theme",
			values:   []string{"light", "dark"},
			fallback: "",
			unknown:  []string{"", "sepia", "Dark", "DARK", "light2"},
			answer:   func(c Chrome) string { return c.Theme },
		},
		{
			cookie:   "yomihon_ruby",
			values:   []string{"on", "off"},
			fallback: "on",
			unknown:  []string{"", "OFF", "Off", "0", "hidden"},
			answer:   func(c Chrome) string { return c.Ruby },
		},
		{
			cookie:   "yomihon_textsize",
			values:   []string{"m", "l", "xl"},
			fallback: "m",
			unknown:  []string{"", "s", "XL", "xxl", "large"},
			answer:   func(c Chrome) string { return c.TextSize },
		},
		{
			cookie:   "yomihon_shortcuts",
			values:   []string{"on", "off"},
			fallback: "on",
			unknown:  []string{"", "OFF", "Off", "disabled", "no"},
			answer: func(c Chrome) string {
				if c.SingleKeyShortcutsEnabled {
					return "on"
				}
				return "off"
			},
		},
		{
			cookie:   "yomihon_font",
			values:   []string{"serif", "sans", "kai"},
			fallback: "",
			unknown:  []string{"", "Serif", "KAI", "kaiti", "mono"},
			answer:   func(c Chrome) string { return c.Font },
		},
	}
}

// chromeWithCookie builds the chrome a request carrying exactly one preference
// cookie produces. An empty name sends no cookie at all, which is the case a
// first visit presents.
func chromeWithCookie(t *testing.T, name, value string) Chrome {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	if name != "" {
		r.Header.Set("Cookie", name+"="+value)
	}
	return ChromeFromRequest(r, "測試")
}

// TestChromeReadsEveryStoredChoiceThroughOneTable holds each reading choice to
// the values it publishes and to its answer when the browser sends something
// else. A cookie is bytes a reader can write, so the unknown rows are the ones
// that matter: a value nobody offered — a neighbouring spelling, another
// capitalisation, an empty one — has to read as a choice never made rather
// than reach the page.
func TestChromeReadsEveryStoredChoiceThroughOneTable(t *testing.T) {
	t.Parallel()
	for _, choice := range storedChoices() {
		t.Run(choice.cookie, func(t *testing.T) {
			t.Parallel()
			for _, value := range choice.values {
				if got := choice.answer(chromeWithCookie(t, choice.cookie, value)); got != value {
					t.Errorf("%s=%s answers %q, want the stored value %q", choice.cookie, value, got, value)
				}
			}
			if got := choice.answer(chromeWithCookie(t, "", "")); got != choice.fallback {
				t.Errorf("no %s cookie answers %q, want the fallback %q", choice.cookie, got, choice.fallback)
			}
			for _, value := range choice.unknown {
				if got := choice.answer(chromeWithCookie(t, choice.cookie, value)); got != choice.fallback {
					t.Errorf("%s=%q answers %q, want the fallback %q", choice.cookie, value, got, choice.fallback)
				}
			}
		})
	}
}

// TestPreferencesPublishesEveryStoredChoice holds the list a caller walks to
// the cookies the first paint actually reads. The expected side is written out
// in full: this list is what an outside surface offers a reader and validates
// their answer against, so it has to be pinned to the names and values on the
// wire rather than to whatever the table happens to hold today.
func TestPreferencesPublishesEveryStoredChoice(t *testing.T) {
	t.Parallel()
	want := []Preference{
		{Cookie: "yomihon_theme", Values: []string{"light", "dark"}, Fallback: ""},
		{Cookie: "yomihon_ruby", Values: []string{"on", "off"}, Fallback: "on"},
		{Cookie: "yomihon_textsize", Values: []string{"m", "l", "xl"}, Fallback: "m"},
		{Cookie: "yomihon_shortcuts", Values: []string{"on", "off"}, Fallback: "on"},
		{Cookie: "yomihon_font", Values: []string{"serif", "sans", "kai"}, Fallback: ""},
	}
	if diff := cmp.Diff(want, Preferences()); diff != "" {
		t.Errorf("Preferences() mismatch (-want +got):\n%s", diff)
	}
	// Every choice the chrome reads for the first paint has to be on that same
	// list. A row left off it is a preference a reader carries and no surface
	// can name, which is the drift the one table exists to prevent.
	published := Preferences()
	for _, row := range []preference{themeChoice, rubyChoice, textSizeChoice, shortcutsChoice, fontChoice} {
		if !slices.ContainsFunc(published, func(p Preference) bool { return p.Cookie == row.cookie }) {
			t.Errorf("the chrome reads %q, which Preferences() does not list", row.cookie)
		}
	}
}

// TestPreferencesHandsOutNothingACallerCanWriteThrough keeps the table the one
// authority on what a cookie may carry. The returned rows carry slices, and a
// slice handed out unchanged is a way into this package's own memory: one
// caller trimming the values it was given would quietly narrow what every
// later caller is told a reader may choose.
func TestPreferencesHandsOutNothingACallerCanWriteThrough(t *testing.T) {
	t.Parallel()
	handed := Preferences()
	if len(handed) == 0 {
		t.Fatal("Preferences() lists nothing, so writing through it proves nothing")
	}
	for i := range handed {
		handed[i].Cookie = "clobbered"
		handed[i].Fallback = "clobbered"
		for j := range handed[i].Values {
			handed[i].Values[j] = "clobbered"
		}
		handed[i].Values = append(handed[i].Values, "extra")
	}
	want := []Preference{
		{Cookie: "yomihon_theme", Values: []string{"light", "dark"}, Fallback: ""},
		{Cookie: "yomihon_ruby", Values: []string{"on", "off"}, Fallback: "on"},
		{Cookie: "yomihon_textsize", Values: []string{"m", "l", "xl"}, Fallback: "m"},
		{Cookie: "yomihon_shortcuts", Values: []string{"on", "off"}, Fallback: "on"},
		{Cookie: "yomihon_font", Values: []string{"serif", "sans", "kai"}, Fallback: ""},
	}
	if diff := cmp.Diff(want, Preferences()); diff != "" {
		t.Errorf("Preferences() after a caller wrote through it (-want +got):\n%s", diff)
	}
	// The chrome reads the rows, not the copies, so a write that reached them
	// would answer a request with a value nobody may store.
	if got := chromeWithCookie(t, "yomihon_theme", "dark").Theme; got != "dark" {
		t.Errorf("theme after a caller wrote through Preferences() = %q, want \"dark\"", got)
	}
}

// TestBaseStampsTheTypefaceOnlyWhenChosen pins the root element's four
// typeface states, in the shape the theme already holds. A choice is stamped
// so the first paint is already set in the reader's face; no choice leaves the
// attribute off entirely — not empty — because the stylesheet's base value
// answers a root that carries no choice, and an empty attribute is a
// choice-shaped thing that chose nothing.
func TestBaseStampsTheTypefaceOnlyWhenChosen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		font string
		want string
	}{
		{name: "kai", font: "kai", want: `data-font="kai"`},
		{name: "sans", font: "sans", want: `data-font="sans"`},
		{name: "serif", font: "serif", want: `data-font="serif"`},
		{name: "no choice", font: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Base(Chrome{Title: "測試", Font: tt.font}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render base: %v", err)
			}
			html := buf.String()
			if tt.want != "" {
				if !strings.Contains(html, tt.want) {
					t.Errorf("Base() root is missing %q", tt.want)
				}
				return
			}
			if at := strings.Index(html, "data-font"); at >= 0 {
				t.Errorf("Base() stamps a typeface nobody chose; near %q", html[at:min(at+40, len(html))])
			}
		})
	}
}

// TestPreferencesHrefCarriesWhereTheReaderIs holds the address the chrome
// points at. The expected values are written out rather than built from the
// same escaping call the code makes, because what has to hold is that a
// reader's current address survives the round trip as a query value — a
// comparison against the code's own escaping would agree with any escaping at
// all, including none.
func TestPreferencesHrefCarriesWhereTheReaderIs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		returnTo string
		want     string
	}{
		{name: "nowhere to return to", returnTo: "", want: "/preferences"},
		{name: "a note whose name has a space", returnTo: "/notes/a b.md", want: "/preferences?from=%2Fnotes%2Fa+b.md"},
		{name: "a search", returnTo: "/search?q=x&n=2", want: "/preferences?from=%2Fsearch%3Fq%3Dx%26n%3D2"},
		{name: "the page itself needs no return address", returnTo: "/preferences", want: "/preferences"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := preferencesHref(tt.returnTo); got != tt.want {
				t.Errorf("preferencesHref(%q) = %q, want %q", tt.returnTo, got, tt.want)
			}
		})
	}
}

// TestKeyboardKeysNamesTheFourKeysTheInterfaceAnswers holds the list to the
// keys that actually do something, written as the literals a reader sees. It
// then holds the help panel to that same rendering: the panel showing its own
// copy of the list is how a key comes to be promised in one place after it has
// moved in the other.
func TestKeyboardKeysNamesTheFourKeysTheInterfaceAnswers(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var keys bytes.Buffer
			if err := KeyboardKeys(lang).Render(t.Context(), &keys); err != nil {
				t.Fatalf("render keyboard keys: %v", err)
			}
			list := keys.String()
			if !strings.HasPrefix(list, `<dl class="y-kbdhelp__list">`) {
				t.Errorf("the key list is not a description list; list = %q", list)
			}
			if got := strings.Count(list, "<dt>"); got != 4 {
				t.Errorf("the key list names %d keys, want the four the interface answers; list = %q", got, list)
			}
			for _, key := range []string{
				`<span class="ui-kbd">⌘K</span>`,
				`<span class="ui-kbd">Ctrl K</span>`,
				`<span class="ui-kbd">Esc</span>`,
				`<span class="ui-kbd">/</span>`,
				`<span class="ui-kbd">[</span>`,
			} {
				if !strings.Contains(list, key) {
					t.Errorf("the key list is missing %q; list = %q", key, list)
				}
			}

			var head bytes.Buffer
			if err := header(Chrome{Lang: lang}).Render(t.Context(), &head); err != nil {
				t.Fatalf("render header: %v", err)
			}
			panel := elementSubtree(t, head.String(), `id="kbd-help"`)
			if !strings.Contains(panel, list) {
				t.Errorf("the help panel does not show this key list; panel = %q", panel)
			}
		})
	}
}
