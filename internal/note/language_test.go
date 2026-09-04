package note_test

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// langAnswer is what the language endpoint said: its status, where it sent
// the reader, and the cookies it set. The response body is already closed —
// the endpoint's answers all live in the head.
type langAnswer struct {
	status   int
	location string
	cookies  []*http.Cookie
}

// postLang submits the language form the way a browser with no script would:
// one POST, redirects not followed, so the answer's own status, Location and
// cookies stay observable.
func postLang(t *testing.T, base *http.Client, target string, form url.Values) langAnswer {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build POST %s: %v", target, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := *base
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	answer := langAnswer{status: resp.StatusCode, location: resp.Header.Get("Location"), cookies: resp.Cookies()}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close response body: %v", closeErr)
	}
	return answer
}

// TestLanguagePostStoresTheChoiceAndReturnsTheReader pins the no-script
// language path end to end: the POST stores the cookie the chrome reads and
// answers 303 back to the page the form came from, and a GET carrying that
// cookie speaks the chosen language. The cookie's shape mirrors what the
// client-side preference writes use, so a choice made with script and one
// made without are the same stored fact.
func TestLanguagePostStoresTheChoiceAndReturnsTheReader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServer(t, root)

	// The journey starts from the page itself: the form the header offers is
	// the one submitted, so the action the markup names and the route the
	// server mounts cannot drift apart behind a hand-typed address.
	code, page := get(t, srv.Client(), srv.URL+"/notes/README.md")
	if code != http.StatusOK {
		t.Fatalf("GET /notes/README.md status = %d, want 200", code)
	}
	action, form := languageForm(t, page)
	answer := postLang(t, srv.Client(), srv.URL+action, form)
	if answer.status != http.StatusSeeOther {
		t.Fatalf("POST %s status = %d, want 303", action, answer.status)
	}
	if answer.location != "/notes/README.md" {
		t.Errorf("Location = %q, want the page the form came from", answer.location)
	}
	var langCookie *http.Cookie
	for _, c := range answer.cookies {
		if c.Name == wording.CookieName {
			langCookie = c
		}
	}
	if langCookie == nil {
		t.Fatalf("POST /lang set no %q cookie; answer = %+v", wording.CookieName, answer)
	}
	if langCookie.Value != "en" {
		t.Errorf("cookie value = %q, want %q", langCookie.Value, "en")
	}
	if langCookie.Path != "/" || langCookie.MaxAge != 31536000 || langCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie shape = path %q max-age %d samesite %v, want / 31536000 Lax",
			langCookie.Path, langCookie.MaxAge, langCookie.SameSite)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+answer.location, http.NoBody)
	if err != nil {
		t.Fatalf("build follow-up request: %v", err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+langCookie.Value)
	followUp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET after switch: %v", err)
	}
	defer func() {
		if closeErr := followUp.Body.Close(); closeErr != nil {
			t.Errorf("close follow-up body: %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(followUp.Body)
	if err != nil {
		t.Fatalf("read follow-up body: %v", err)
	}
	if body := string(raw); !strings.Contains(body, `<html lang="en"`) {
		t.Errorf("the page after the switch does not speak English; head = %q", body[:min(len(body), 200)])
	}

	back := postLang(t, srv.Client(), srv.URL+"/lang", url.Values{"lang": {"zh-Hant"}, "next": {"/"}})
	if back.status != http.StatusSeeOther {
		t.Fatalf("POST /lang back status = %d, want 303", back.status)
	}
	backCookie := ""
	for _, c := range back.cookies {
		if c.Name == wording.CookieName {
			backCookie = c.Value
		}
	}
	if backCookie != "zh-Hant" {
		t.Errorf("switching back stored %q, want %q", backCookie, "zh-Hant")
	}
}

// TestLanguagePostKeepsTheRedirectLocal holds the return address to this
// site's own pages. The field is client-controlled bytes; anything that could
// carry the reader off the machine's own interface falls back to Home.
func TestLanguagePostKeepsTheRedirectLocal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	srv := newServer(t, root)
	tests := []struct {
		name string
		next string
		want string
	}{
		{name: "a local path with a query survives", next: "/search?q=x", want: "/search?q=x"},
		{name: "empty falls to Home", next: "", want: "/"},
		{name: "an absolute URL falls to Home", next: "https://example.com/", want: "/"},
		{name: "a protocol-relative address falls to Home", next: "//example.com/", want: "/"},
		{name: "a backslash pair falls to Home", next: `/\example.com`, want: "/"},
		{name: "a relative path falls to Home", next: "notes/A.md", want: "/"},
		{name: "a decoded tab falls to Home", next: "/\t/evil.example", want: "/"},
		{name: "a decoded newline falls to Home", next: "/\n/evil.example", want: "/"},
		{name: "a delete byte falls to Home", next: "/\x7f/evil.example", want: "/"},
		{name: "a next line falls to Home", next: "/\u0085/evil.example", want: "/"},
		{name: "a line separator falls to Home", next: "/\u2028/evil.example", want: "/"},
		{name: "a paragraph separator falls to Home", next: "/\u2029/evil.example", want: "/"},
		// The refusal above is a set of separators, not of non-ASCII: most of
		// this vault's own paths are CJK, and rejecting them would send every
		// reader of them to Home on a language switch.
		{name: "a CJK path survives, escaped", next: "/notes/讀懂.md", want: "/notes/%e8%ae%80%e6%87%82.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer := postLang(t, srv.Client(), srv.URL+"/lang", url.Values{"lang": {"en"}, "next": {tt.next}})
			if answer.status != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303", answer.status)
			}
			if answer.location != tt.want {
				t.Errorf("Location for next=%q is %q, want %q", tt.next, answer.location, tt.want)
			}
		})
	}
}

// TestLanguagePostRefusesALanguageItDoesNotSpeak pins the write's hygiene: the
// endpoint stores only the two values the interface can honour, and an unknown
// one is refused outright rather than silently normalised — the reader asked
// for something, and pretending to grant it would leave them where they were
// with a receipt saying otherwise.
func TestLanguagePostRefusesALanguageItDoesNotSpeak(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	srv := newServer(t, root)
	for _, value := range []string{"", "ja", "EN", "zh"} {
		t.Run("value "+value, func(t *testing.T) {
			t.Parallel()
			answer := postLang(t, srv.Client(), srv.URL+"/lang", url.Values{"lang": {value}, "next": {"/"}})
			if answer.status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", answer.status)
			}
			for _, c := range answer.cookies {
				if c.Name == wording.CookieName {
					t.Errorf("a refused choice still set the cookie to %q", c.Value)
				}
			}
		})
	}
}

// languageForm extracts the header's language form from a rendered page: the
// action it posts to and the two hidden fields it carries. Submitting these,
// rather than a hand-typed address, is what couples the markup to the route.
func languageForm(t *testing.T, page string) (action string, form url.Values) {
	t.Helper()
	formAt := strings.Index(page, `class="y-langform"`)
	if formAt < 0 {
		t.Fatalf("page carries no language form; body = %q", page[:min(len(page), 400)])
	}
	end := strings.Index(page[formAt:], "</form>")
	if end < 0 {
		t.Fatalf("language form is not closed")
	}
	markup := page[formAt : formAt+end]
	attr := func(name, source string) string {
		key := name + `="`
		at := strings.Index(source, key)
		if at < 0 {
			t.Fatalf("language form is missing %s; form = %q", name, source)
		}
		rest := source[at+len(key):]
		return rest[:strings.IndexByte(rest, '"')]
	}
	form = url.Values{}
	for rest := markup; ; {
		at := strings.Index(rest, "<input")
		if at < 0 {
			break
		}
		tag := rest[at:]
		tag = tag[:strings.IndexByte(tag, '>')]
		form.Set(attr("name", tag), attr("value", tag))
		rest = rest[at+len(tag):]
	}
	return attr("action", markup), form
}
