package preference_test

import (
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/preference"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// newServer stands the preferences face up on its own. Nothing else is mounted
// because nothing else is reached: the page shows the request's own cookies and
// the words for them, so a vault, a snapshot and a status authority would all
// be scenery.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	preference.New(&preference.Dependencies{Log: slog.New(slog.DiscardHandler)}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// answer is what the endpoint said: its status, where it sent the reader, the
// cookies it set, and the raw header lines those cookies were written on. The
// raw lines are kept because a deletion is a wire fact — the standard library
// reads "Max-Age=0" back as a negative age, so asserting on the parsed field
// would be asserting on the reader rather than on what was sent.
type answer struct {
	status    int
	location  string
	cookies   []*http.Cookie
	setCookie []string
}

// cookie finds the one cookie of this name the answer carries, and reports
// whether it carried exactly one.
func (a answer) cookie(name string) (*http.Cookie, bool) {
	var found *http.Cookie
	count := 0
	for _, c := range a.cookies {
		if c.Name == name {
			found = c
			count++
		}
	}
	return found, count == 1
}

// names lists the cookies the answer set, in the order it set them.
func (a answer) names() []string {
	out := make([]string, 0, len(a.cookies))
	for _, c := range a.cookies {
		out = append(out, c.Name)
	}
	return out
}

// postForm submits one form the way a browser with no script would: a single
// POST with redirects unfollowed, so the answer's own status, Location and
// cookies stay observable.
func postForm(t *testing.T, target string, form url.Values) answer {
	t.Helper()
	return postBody(t, target, form.Encode())
}

func postBody(t *testing.T, target, body string) answer {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build POST %s: %v", target, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return send(t, req)
}

func send(t *testing.T, req *http.Request) answer {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	got := answer{
		status:    resp.StatusCode,
		location:  resp.Header.Get("Location"),
		cookies:   resp.Cookies(),
		setCookie: resp.Header.Values("Set-Cookie"),
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close response body: %v", closeErr)
	}
	return got
}

// page fetches the rendered page, optionally carrying stored choices with it.
// The choices go over as the header a browser would send — the bytes the
// endpoint actually reads — rather than as parsed cookie values, which would
// let the test and the endpoint agree through a shared parser.
func page(t *testing.T, target string, cookies ...string) (status int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatalf("build GET %s: %v", target, err)
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", strings.Join(cookies, "; "))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", target, err)
	}
	return resp.StatusCode, string(raw)
}

// tag is one element the page rendered, read back out of the markup. The page
// is examined as a browser would receive it rather than as the view that built
// it, because the view is the thing under test.
type tag struct {
	kind    string
	name    string
	value   string
	checked bool
}

// elements pulls every <input> or <button> off the page with the attributes a
// submission depends on.
func elements(t *testing.T, markup, element string) []tag {
	t.Helper()
	var out []tag
	open := "<" + element
	for rest := markup; ; {
		at := strings.Index(rest, open)
		if at < 0 {
			return out
		}
		end := strings.IndexByte(rest[at:], '>')
		if end < 0 {
			t.Fatalf("an %s tag is never closed: %q", element, rest[at:min(at+120, len(rest))])
		}
		found := rest[at : at+end]
		out = append(out, tag{
			kind:    attr(found, "type"),
			name:    attr(found, "name"),
			value:   attr(found, "value"),
			checked: strings.Contains(found, " checked ") || strings.HasSuffix(found, " checked"),
		})
		rest = rest[at+end:]
	}
}

func attr(source, name string) string {
	_, rest, found := strings.Cut(source, " "+name+`="`)
	if !found {
		return ""
	}
	value, _, closed := strings.Cut(rest, `"`)
	if !closed {
		return ""
	}
	return value
}

// radios groups the page's radio buttons by the field they are submitted under,
// keeping each group in the order the page rendered it.
func radios(t *testing.T, markup string) map[string][]tag {
	t.Helper()
	found := elements(t, markup, "input")
	if len(found) == 0 {
		t.Fatalf("the page carries no input at all; body = %q", markup[:min(len(markup), 400)])
	}
	out := map[string][]tag{}
	for _, in := range found {
		if in.kind == "radio" {
			out[in.name] = append(out[in.name], in)
		}
	}
	return out
}

// values reads a radio group down to the values it offers, in page order.
func values(group []tag) []string {
	out := make([]string, 0, len(group))
	for _, r := range group {
		out = append(out, r.value)
	}
	return out
}

// storedChoices names, for each cookie the page can write, the values it may
// carry — spelled out here rather than read from the code under test. A test
// that asked the page's own source what the page should offer would agree with
// it however either of them drifts.
var storedChoices = map[string][]string{
	"lang":      {"zh-Hant", "en"},
	"theme":     {"system", "light", "dark"},
	"textsize":  {"m", "l", "xl"},
	"font":      {"serif", "sans", "kai"},
	"ruby":      {"on", "off"},
	"shortcuts": {"on", "off"},
}

// cookieFor is the cookie each field lands in, again spelled out rather than
// derived: the field name and the cookie name share a shape today, and a test
// that reconstructed one from the other would keep passing on the day they
// stop.
var cookieFor = map[string]string{
	"lang":      "yomihon_lang",
	"theme":     "yomihon_theme",
	"textsize":  "yomihon_textsize",
	"font":      "yomihon_font",
	"ruby":      "yomihon_ruby",
	"shortcuts": "yomihon_shortcuts",
}

// TestApplyingAChoiceStoresItAndReturnsTheReader pins the whole no-script path
// for every value the page offers that a cookie may hold: one POST stores one
// cookie and answers 303 back to the page the form came from. The cookie's
// shape is asserted whole — a year, this site's root, no Secure and no
// HttpOnly — because each of those is a decision, and the two absences are the
// ones a well-meaning change would reverse.
func TestApplyingAChoiceStoresItAndReturnsTheReader(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	stored := 0
	for _, field := range slices.Sorted(maps.Keys(storedChoices)) {
		for _, value := range storedChoices[field] {
			if field == "theme" && value == "system" {
				// Following the system stores nothing; its own test below says
				// what it does instead.
				continue
			}
			stored++
			t.Run(field+"="+value, func(t *testing.T) {
				t.Parallel()
				got := postForm(t, srv.URL+"/preferences", url.Values{
					field:  {value},
					"next": {"/notes/A.md"},
				})
				if got.status != http.StatusSeeOther {
					t.Fatalf("status = %d, want 303", got.status)
				}
				if got.location != "/notes/A.md" {
					t.Errorf("Location = %q, want the page the form came from", got.location)
				}
				if len(got.cookies) != 1 {
					t.Fatalf("the answer set %d cookies (%v), want exactly one", len(got.cookies), got.names())
				}
				c, only := got.cookie(cookieFor[field])
				if !only {
					t.Fatalf("no single %q cookie; the answer set %v", cookieFor[field], got.names())
				}
				if c.Value != value {
					t.Errorf("stored value = %q, want %q", c.Value, value)
				}
				if c.Path != "/" || c.MaxAge != 31536000 || c.SameSite != http.SameSiteLaxMode {
					t.Errorf("cookie shape = path %q max-age %d samesite %v, want / 31536000 Lax",
						c.Path, c.MaxAge, c.SameSite)
				}
				if c.HttpOnly {
					t.Error("the cookie is HttpOnly; the restore check after a back/forward has to read it")
				}
				if c.Secure {
					t.Error("the cookie is Secure; this site is loopback HTTP and the value would never be sent")
				}
			})
		}
	}
	if stored != 14 {
		t.Errorf("the table covers %d stored values, want the 14 the page offers", stored)
	}
}

// TestAValueNoChoiceOffersIsRefusedWithNothingStored pins the hygiene of the
// write. A form field is client-controlled bytes; a value no rendered form
// offers is refused outright rather than normalised into the nearest one,
// because answering with a redirect would hand back a receipt for a change
// nothing made.
func TestAValueNoChoiceOffersIsRefusedWithNothingStored(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	refused := map[string][]string{
		"lang":      {"", "EN", "ja", "zh"},
		"theme":     {"", "Light", "auto"},
		"textsize":  {"", "M", "s"},
		"font":      {"", "Serif", "mincho"},
		"ruby":      {"", "ON", "yes"},
		"shortcuts": {"", "OFF", "true"},
	}
	for field, offered := range storedChoices {
		if len(refused[field]) < 3 {
			t.Fatalf("%s has %d refused values, want at least three beside its %d offered ones",
				field, len(refused[field]), len(offered))
		}
	}
	for _, field := range slices.Sorted(maps.Keys(refused)) {
		for _, value := range refused[field] {
			t.Run(field+"="+value, func(t *testing.T) {
				t.Parallel()
				got := postForm(t, srv.URL+"/preferences", url.Values{field: {value}, "next": {"/"}})
				if got.status != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422", got.status)
				}
				if len(got.cookies) != 0 {
					t.Errorf("a refused value still set %v", got.names())
				}
			})
		}
	}
}

// TestFollowingTheSystemDeletesTheStoredTheme pins the one option that is not a
// stored value. Light and dark are the two a cookie may carry; "whatever the
// operating system says" is the state its absence holds, so choosing it has to
// remove the cookie rather than write a third word into it.
func TestFollowingTheSystemDeletesTheStoredTheme(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	got := postForm(t, srv.URL+"/preferences", url.Values{"theme": {"system"}, "next": {"/notes/A.md"}})
	if got.status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", got.status)
	}
	if got.location != "/notes/A.md" {
		t.Errorf("Location = %q, want the page the form came from", got.location)
	}
	if len(got.setCookie) != 1 {
		t.Fatalf("the answer wrote %d Set-Cookie lines (%v), want exactly one", len(got.setCookie), got.setCookie)
	}
	line := got.setCookie[0]
	if !strings.HasPrefix(line, "yomihon_theme=;") {
		t.Errorf("Set-Cookie = %q, want the theme cookie emptied", line)
	}
	if !strings.Contains(line, "Max-Age=0") {
		t.Errorf("Set-Cookie = %q, want it to expire the cookie with Max-Age=0", line)
	}
}

// TestResetClearsEveryStoredChoice pins the one control that touches all of
// them. The set of cookies it clears is compared against a written list in both
// directions: one it forgets leaves a reader with a setting no page still
// offers to change, and one it clears that is not listed here is something this
// page took upon itself to remove.
func TestResetClearsEveryStoredChoice(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	got := postForm(t, srv.URL+"/preferences", url.Values{"reset": {"1"}, "next": {"/notes/A.md"}})
	if got.status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", got.status)
	}
	if got.location != "/notes/A.md" {
		t.Errorf("Location = %q, want the page the form came from", got.location)
	}
	want := []string{
		"yomihon_font",
		"yomihon_lang",
		"yomihon_ruby",
		"yomihon_shortcuts",
		"yomihon_textsize",
		"yomihon_theme",
	}
	cleared := slices.Clone(got.names())
	slices.Sort(cleared)
	if !slices.Equal(cleared, want) {
		t.Errorf("reset cleared %v, want exactly %v", cleared, want)
	}
	for _, line := range got.setCookie {
		if !strings.Contains(line, "Max-Age=0") {
			t.Errorf("Set-Cookie = %q, want it to expire the cookie with Max-Age=0", line)
		}
	}
}

// TestResetRefusesAnyOtherValue keeps the clearing control from firing on
// anything but the value its own button sends. It is the one control here with
// no undo.
func TestResetRefusesAnyOtherValue(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	for _, value := range []string{"", "0", "yes", "true"} {
		t.Run("reset="+value, func(t *testing.T) {
			t.Parallel()
			got := postForm(t, srv.URL+"/preferences", url.Values{"reset": {value}, "next": {"/"}})
			if got.status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", got.status)
			}
			if len(got.cookies) != 0 {
				t.Errorf("a refused reset still cleared %v", got.names())
			}
		})
	}
}

// TestTheReturnAddressStaysOnThisSite holds the redirect to this site's own
// pages. The field is client-controlled bytes; anything that could carry the
// reader off the machine's own interface falls back to Home.
func TestTheReturnAddressStaysOnThisSite(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := postForm(t, srv.URL+"/preferences", url.Values{"lang": {"en"}, "next": {tt.next}})
			if got.status != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303", got.status)
			}
			if got.location != tt.want {
				t.Errorf("Location for next=%q is %q, want %q", tt.next, got.location, tt.want)
			}
		})
	}
}

// TestASubmissionCarriesExactlyOneChoice makes one-field-per-form a contract
// rather than a habit of the markup. A body naming none of the choices asked
// for nothing, and one naming two would have to be answered by deciding which
// the reader meant; both are refused with nothing written.
func TestASubmissionCarriesExactlyOneChoice(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	tests := []struct {
		name string
		form url.Values
	}{
		{name: "no choice at all", form: url.Values{"next": {"/"}}},
		{name: "two choices", form: url.Values{"theme": {"dark"}, "font": {"kai"}, "next": {"/"}}},
		{name: "a choice and a reset", form: url.Values{"theme": {"dark"}, "reset": {"1"}, "next": {"/"}}},
		{name: "a field this page does not offer", form: url.Values{"paper": {"cream"}, "next": {"/"}}},
		{name: "one choice carrying two answers", form: url.Values{"theme": {"dark", "light"}, "next": {"/"}}},
		{name: "a reset carrying two answers", form: url.Values{"reset": {"1", "1"}, "next": {"/"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := postForm(t, srv.URL+"/preferences", tt.form)
			if got.status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", got.status)
			}
			if len(got.cookies) != 0 {
				t.Errorf("a refused submission still set %v", got.names())
			}
		})
	}
}

// TestAMethodThisAddressDoesNotServeIsRefused pins what the router answers a
// method neither route names. The status alone would say little — a path that
// matched nothing at all would be a 404 and a path handled without naming a
// method would be a 200 — so the assertion is on the Allow header naming both
// methods that are mounted, which only a router that knows this address serves
// exactly those two can write.
func TestAMethodThisAddressDoesNotServeIsRefused(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+"/preferences", http.NoBody)
			if err != nil {
				t.Fatalf("build %s: %v", method, err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s /preferences: %v", method, err)
			}
			defer func() {
				if closeErr := resp.Body.Close(); closeErr != nil {
					t.Errorf("close response body: %v", closeErr)
				}
			}()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", resp.StatusCode)
			}
			allow := resp.Header.Get("Allow")
			for _, served := range []string{http.MethodGet, http.MethodPost} {
				if !strings.Contains(allow, served) {
					t.Errorf("Allow = %q, want it to name %s", allow, served)
				}
			}
		})
	}
}

// TestASubmissionLargerThanAChoiceIsRefused keeps the body bound where two
// short fields put it. Nothing a rendered form sends comes close, so a body
// past the bound was built by hand.
func TestASubmissionLargerThanAChoiceIsRefused(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	body := "theme=dark&next=%2F&padding=" + strings.Repeat("x", 5000)
	got := postBody(t, srv.URL+"/preferences", body)
	if got.status != http.StatusBadRequest {
		t.Fatalf("status = %d for a %d-byte body, want 400", got.status, len(body))
	}
	if len(got.cookies) != 0 {
		t.Errorf("an oversized submission still set %v", got.names())
	}
}

// TestThePageSpeaksTheStoredLanguage confirms the page is written in the
// language the browser is carrying rather than in whichever one the source
// happens to name first.
func TestThePageSpeaksTheStoredLanguage(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	code, body := page(t, srv.URL+"/preferences", "yomihon_lang=en")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, `<html lang="en"`) {
		t.Errorf("the page does not declare English; head = %q", body[:min(len(body), 200)])
	}
	if !strings.Contains(body, "Interface language") {
		t.Errorf("the page is not written in English; body = %q", body[:min(len(body), 400)])
	}
}

// TestThePageCarriesTheAddressItWasReachedFrom pins the return trip. The
// address is checked while the page is built, so every form on it already
// carries a path this site can go back to.
func TestThePageCarriesTheAddressItWasReachedFrom(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	code, body := page(t, srv.URL+"/preferences?from=%2Fsearch%3Fq%3Dx")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	assertEveryReturnAddress(t, body, "/search?q=x")
}

// TestAnAddressOffThisSiteNeverReachesTheForms holds the same guard on the way
// in as on the way out. The address arrives in a query a reader can type, so
// what the page renders into its forms is already the checked answer rather
// than the bytes it was handed.
func TestAnAddressOffThisSiteNeverReachesTheForms(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	for _, from := range []string{"//evil.example", `/\evil.example`, "https://evil.example/"} {
		t.Run(from, func(t *testing.T) {
			t.Parallel()
			code, body := page(t, srv.URL+"/preferences?from="+url.QueryEscape(from))
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			assertEveryReturnAddress(t, body, "/")
		})
	}
}

// TestThePageOpenedDirectlyReturnsToItself pins the case with no page behind
// it: applying a choice leaves the reader here, and the entry in the chrome
// does not fold this page's own address into itself.
func TestThePageOpenedDirectlyReturnsToItself(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	code, body := page(t, srv.URL+"/preferences")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	assertEveryReturnAddress(t, body, "/preferences")
	if strings.Contains(body, "from=%2Fpreferences") {
		t.Errorf("the chrome's own entry carries this page as its return address; a second visit would nest another copy")
	}
}

// ownForms is the markup of each form this page posts back to itself. The
// chrome around it carries a form of its own; what that one does with the
// return address is the chrome's business, and counting it here would move this
// assertion every time the chrome does.
func ownForms(t *testing.T, markup string) []string {
	t.Helper()
	var out []string
	for rest := markup; ; {
		at := strings.Index(rest, "<form ")
		if at < 0 {
			return out
		}
		rest = rest[at:]
		end := strings.Index(rest, "</form>")
		if end < 0 {
			t.Fatalf("a form on the page is never closed: %q", rest[:min(len(rest), 200)])
		}
		if form := rest[:end]; attr(form, "action") == "/preferences" {
			out = append(out, form)
		}
		rest = rest[end:]
	}
}

// assertEveryReturnAddress checks the address each of this page's own forms
// carries. Every one of them, because a single form left behind is the one a
// reader eventually submits.
func assertEveryReturnAddress(t *testing.T, markup, want string) {
	t.Helper()
	forms := ownForms(t, markup)
	if len(forms) != len(storedChoices)+1 {
		t.Fatalf("the page carries %d forms of its own, want one for each of the %d choices and one for the reset",
			len(forms), len(storedChoices))
	}
	for _, form := range forms {
		carried := 0
		for _, in := range elements(t, form, "input") {
			if in.kind != "hidden" || in.name != "next" {
				continue
			}
			carried++
			if in.value != want {
				t.Errorf("a form returns to %q, want %q", in.value, want)
			}
		}
		if carried != 1 {
			t.Errorf("a form carries %d return addresses, want exactly one", carried)
		}
	}
}

// TestThePageOffersExactlyTheseChoices is the drift lock. It cannot ask the
// page what the page should show — the page is built from the same table it
// would be compared against, and that comparison passes however both of them
// move. So the answer is written out here, and the page, the endpoint and this
// list all have to agree.
func TestThePageOffersExactlyTheseChoices(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	code, body := page(t, srv.URL+"/preferences")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	found := radios(t, body)

	for _, field := range slices.Sorted(maps.Keys(storedChoices)) {
		got := values(found[field])
		if !slices.Equal(got, storedChoices[field]) {
			t.Errorf("%s offers %v, want %v", field, got, storedChoices[field])
		}
	}
	for _, field := range slices.Sorted(maps.Keys(found)) {
		if _, listed := storedChoices[field]; !listed {
			t.Errorf("the page offers a choice named %q that nothing here describes", field)
		}
	}

	clears := 0
	for _, button := range elements(t, body, "button") {
		if button.kind == "submit" && button.name == "reset" && button.value == "1" {
			clears++
		}
	}
	if clears != 1 {
		t.Errorf("the page carries %d controls that clear every choice, want exactly one", clears)
	}

	// The same written list, put to the endpoint: a value the page offers must
	// be one the endpoint honours, and a value outside the list must not be.
	for _, field := range slices.Sorted(maps.Keys(storedChoices)) {
		for _, value := range storedChoices[field] {
			got := postForm(t, srv.URL+"/preferences", url.Values{field: {value}, "next": {"/"}})
			if got.status != http.StatusSeeOther {
				t.Errorf("%s=%s answered %d, want the 303 an offered value gets", field, value, got.status)
			}
		}
		for _, value := range []string{"paper", "listed-nowhere"} {
			outside := postForm(t, srv.URL+"/preferences", url.Values{field: {value}, "next": {"/"}})
			if outside.status != http.StatusUnprocessableEntity {
				t.Errorf("%s=%s answered %d, want the 422 a value the page does not offer gets",
					field, value, outside.status)
			}
		}
	}
}

// TestExactlyOneOptionIsMarkedForEachChoice pins what a reader arriving with a
// clean browser sees. A group with nothing marked states no current answer and
// submits none: pressing its apply would send an empty field, which the
// endpoint correctly refuses — leaving a reader refused by a form they filled
// in the only way it offered.
func TestExactlyOneOptionIsMarkedForEachChoice(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	tests := []struct {
		name    string
		cookies []string
		want    map[string]string
	}{
		{
			name: "a browser carrying nothing",
			want: map[string]string{
				"lang": "zh-Hant", "theme": "system", "textsize": "m",
				"font": "serif", "ruby": "on", "shortcuts": "on",
			},
		},
		{
			name: "a browser carrying every choice",
			cookies: []string{
				"yomihon_lang=en",
				"yomihon_theme=dark",
				"yomihon_textsize=xl",
				"yomihon_font=kai",
				"yomihon_ruby=off",
				"yomihon_shortcuts=off",
			},
			want: map[string]string{
				"lang": "en", "theme": "dark", "textsize": "xl",
				"font": "kai", "ruby": "off", "shortcuts": "off",
			},
		},
		{
			name: "a browser carrying values nobody offered",
			cookies: []string{
				"yomihon_theme=chartreuse",
				"yomihon_font=chartreuse",
				"yomihon_textsize=chartreuse",
			},
			want: map[string]string{
				"lang": "zh-Hant", "theme": "system", "textsize": "m",
				"font": "serif", "ruby": "on", "shortcuts": "on",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := page(t, srv.URL+"/preferences", tt.cookies...)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			found := radios(t, body)
			if len(found) != len(tt.want) {
				t.Fatalf("the page offers %d choices, want %d", len(found), len(tt.want))
			}
			for _, field := range slices.Sorted(maps.Keys(tt.want)) {
				marked := make([]string, 0, 1)
				for _, option := range found[field] {
					if option.checked {
						marked = append(marked, option.value)
					}
				}
				if !slices.Equal(marked, []string{tt.want[field]}) {
					t.Errorf("%s has %v marked, want exactly [%s]", field, marked, tt.want[field])
				}
			}
		})
	}
}

// TestEveryStoredReadingChoiceReachesThePage closes the drift from the other
// side. The list above pins what the page shows; this asks whether the chrome
// still keeps a choice the page has stopped showing — a preference the first
// paint honours and no surface can change is a setting a reader can only reach
// by editing a cookie by hand.
func TestEveryStoredReadingChoiceReachesThePage(t *testing.T) {
	t.Parallel()
	srv := newServer(t)

	_, body := page(t, srv.URL+"/preferences")
	found := radios(t, body)

	kept := layouts.Preferences()
	if len(kept) == 0 {
		t.Fatal("the chrome keeps no reading choices at all, so every row below passes for the wrong reason")
	}
	for _, p := range kept {
		field := ""
		for name, cookie := range cookieFor {
			if cookie == p.Cookie {
				field = name
			}
		}
		if field == "" {
			t.Errorf("the chrome reads a cookie %q that no field on this page writes", p.Cookie)
			continue
		}
		offered := values(found[field])
		for _, value := range p.Values {
			if !slices.Contains(offered, value) {
				t.Errorf("the chrome honours %s=%q and the page does not offer it; offered = %v", p.Cookie, value, offered)
			}
		}
	}
}
