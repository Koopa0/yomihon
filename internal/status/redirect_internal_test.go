package status

import "testing"

// TestNotesHref pins the reading-page location a successful flip redirects to.
// Each path segment is percent-escaped on its own: "?" and "#" cannot bleed
// into the URL's query or fragment, CJK and spaces are encoded, and "/" stays
// the separator between segments. The expected strings are hand-derived from
// the byte encoding, not read back from the escaper.
func TestNotesHref(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "cjk and spaces are escaped, slashes stay separators",
			path: "Concepts/golang/Go 位元運算.md",
			want: "/notes/Concepts/golang/Go%20%E4%BD%8D%E5%85%83%E9%81%8B%E7%AE%97.md",
		},
		{
			name: "question mark cannot start a query",
			path: "Notes/what is this?.md",
			want: "/notes/Notes/what%20is%20this%3F.md",
		},
		{
			name: "hash cannot start a fragment",
			path: "Notes/C#basics.md",
			want: "/notes/Notes/C%23basics.md",
		},
		{
			name: "plain ascii path is unchanged",
			path: "Writing/lessons/japanese/L05.md",
			want: "/notes/Writing/lessons/japanese/L05.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := notesHref(tt.path); got != tt.want {
				t.Errorf("notesHref(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
