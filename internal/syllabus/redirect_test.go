package syllabus_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/pages"
)

// A course was a study path with a syllabus page, and a reader may hold a link
// or a bookmark written then. The two retired addresses answer with a permanent
// redirect to the same place under the course's own address, carrying the rest
// of the path and the query over still escaped, and serve nothing themselves.
func TestRetiredAddressesRedirectToTheCourseAddresses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVault(t, root)
	srv := newServer(t, root)

	from := url.Values{pages.CourseFromParam: {"Writing/lessons/golang/Slices.md"}}.Encode()
	tests := []struct {
		name, asked, want string
	}{
		{"the index", "/paths", "/courses"},
		{"the index with a query", "/paths?x=1", "/courses?x=1"},
		{"a course", "/syllabus/Maps/Go%20path.md", "/courses/Maps/Go%20path.md"},
		{"a course the reader came to from a note", "/syllabus/Maps/Go%20path.md?" + from, "/courses/Maps/Go%20path.md?" + from},
		{"a course whose name is not ASCII", "/syllabus/Maps/%E8%AA%B2%E7%A8%8B.md", "/courses/Maps/%E8%AA%B2%E7%A8%8B.md"},
		{"an address with nothing after the prefix", "/syllabus/", "/courses/"},
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+tc.asked, http.NoBody)
			if err != nil {
				t.Fatalf("new request %s: %v", tc.asked, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.asked, err)
			}
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Errorf("close response body: %v", closeErr)
			}
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Fatalf("GET %s status = %d, want %d", tc.asked, resp.StatusCode, http.StatusMovedPermanently)
			}
			if got := resp.Header.Get("Location"); got != tc.want {
				t.Errorf("GET %s redirects to %q, want %q", tc.asked, got, tc.want)
			}
		})
	}
}

// The redirect is the whole answer at a retired address and the course's page
// is served only at its own: following the redirect reaches the page, and the
// old address never returns a page of its own.
func TestACourseIsServedOnlyAtItsOwnAddress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVault(t, root)
	srv := newServer(t, root)

	code, page := get(t, srv.Client(), srv.URL+"/syllabus/Maps/Go%20path.md")
	if code != http.StatusOK || !strings.Contains(page, "Go path") {
		t.Fatalf("following the retired address: status = %d, want 200 with the course page", code)
	}
	direct, directPage := get(t, srv.Client(), srv.URL+"/courses/Maps/Go%20path.md")
	if direct != http.StatusOK || directPage != page {
		t.Errorf("the course at its own address differs from the page the redirect leads to (status %d)", direct)
	}

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/syllabus/Maps/Go%20path.md", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("the retired address served a page of its own, want a redirect")
	}
}
