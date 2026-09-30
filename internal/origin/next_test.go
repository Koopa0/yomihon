package origin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// browserHost is the host a browser on this site's page lands on when a
// response carries location. Go's own URL parser is not the reader here: a
// browser strips leading and trailing C0 controls and spaces, drops every tab
// and newline, and reads a backslash as a slash before it resolves a
// reference against the page, and each of those is a way an address that
// looks local to Go leaves the site.
func browserHost(t *testing.T, location string) string {
	t.Helper()
	read := strings.TrimFunc(location, func(r rune) bool { return r <= 0x20 })
	read = strings.NewReplacer("\t", "", "\n", "", "\r", "", `\`, "/").Replace(read)
	ref, err := url.Parse(read)
	if err != nil {
		// An address no parser can read takes the reader nowhere at all.
		return siteHost
	}
	base := &url.URL{Scheme: "http", Host: siteHost, Path: "/notes/A.md"}
	return base.ResolveReference(ref).Host
}

const siteHost = "127.0.0.1:9610"

// redirectedTo is what a browser would be sent to if a form's return field
// carried next: the Location http.Redirect writes for LocalNext's answer.
func redirectedTo(t *testing.T, next string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+siteHost+"/lang", http.NoBody)
	http.Redirect(w, r, LocalNext(next), http.StatusSeeOther)
	return w.Header().Get("Location")
}

// TestAReturnAddressNeverLeavesThisSite holds the property the guard exists
// for, read the way the browser reads it: whatever the form carried, the
// address the response sends stays on this host. The vectors are the shapes
// that have each looked local to a check of the raw bytes — separators a
// parser drops, a backslash a browser turns into a slash, and dot segments the
// redirect cleans away before the header is written.
func TestAReturnAddressNeverLeavesThisSite(t *testing.T) {
	t.Parallel()
	for _, next := range []string{
		"https://evil.example/",
		"//evil.example/",
		`/\evil.example`,
		`\/evil.example`,
		`/./\evil.example`,
		`/.\/evil.example`,
		`/a/../\evil.example`,
		`/a/b/../../\evil.example`,
		`/./\evil.example/?q=1`,
		"/./\\evil.example/",
		"/\t/evil.example",
		"/\n/evil.example",
		"/\x7f/evil.example",
		"/\u0085/evil.example",
		"/\u2028/evil.example",
		"/\u2029/evil.example",
		" //evil.example",
	} {
		location := redirectedTo(t, next)
		if host := browserHost(t, location); host != siteHost {
			t.Errorf("next=%q is answered with Location %q, which a browser reads as host %q", next, location, host)
		}
	}
}

// TestALocalAddressComesBackAsTheRedirectWouldSendIt pins the other half: an
// honest address survives, and the answer is already the cleaned address the
// redirect writes, so what was judged is what is sent.
func TestALocalAddressComesBackAsTheRedirectWouldSendIt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		next string
		want string
	}{
		{next: "/", want: "/"},
		{next: "/search?q=x", want: "/search?q=x"},
		{next: "/folders/Notes/", want: "/folders/Notes/"},
		{next: "/notes/%E8%AE%80.md#top", want: "/notes/%E8%AE%80.md#top"},
		{next: "/notes/a/../b.md", want: "/notes/b.md"},
		{next: "/search?q=a/../b", want: "/search?q=a/../b"},
		{next: "", want: "/"},
		{next: "notes/A.md", want: "/"},
		{next: `/./\evil.example`, want: "/"},
	}
	for _, tt := range tests {
		if got := LocalNext(tt.next); got != tt.want {
			t.Errorf("LocalNext(%q) = %q, want %q", tt.next, got, tt.want)
		}
		if got := LocalNext(tt.want); got != tt.want {
			t.Errorf("LocalNext(%q) = %q; an address it already answered must come back unchanged", tt.want, got)
		}
	}
}
