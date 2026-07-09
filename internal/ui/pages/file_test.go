package pages

import "testing"

// TestHumanSize pins what the information page tells a reader about a file it
// cannot show. The exact byte count always travels with the rounded figure,
// because the page exists to be exact, and a lone file's byte is one byte.
func TestHumanSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{name: "empty", n: 0, want: "0 bytes"},
		{name: "one byte is singular", n: 1, want: "1 byte"},
		{name: "two bytes", n: 2, want: "2 bytes"},
		{name: "just under a kilobyte", n: 1023, want: "1,023 bytes"},
		{name: "a kilobyte", n: 1024, want: "1.0 KB (1,024 bytes)"},
		{name: "a megabyte", n: 1 << 20, want: "1.0 MB (1,048,576 bytes)"},
		{name: "a gigabyte", n: 1 << 30, want: "1.0 GB (1,073,741,824 bytes)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := humanSize(tt.n); got != tt.want {
				t.Errorf("humanSize(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
