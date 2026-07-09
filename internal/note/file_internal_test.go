package note

import (
	"strings"
	"testing"
)

// TestServable pins the route boundary's own rule. filepath.IsLocal is not
// enough on its own: it admits every dot-leading name, and those name the trees
// the scanner deliberately never walks.
func TestServable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rel  string
		want bool
	}{
		{name: "a listed note", rel: "Notes/real.md", want: true},
		{name: "a listed file with no extension", rel: "Makefile", want: true},
		{name: "a name with a dot inside it", rel: "Notes/v1.2.md", want: true},
		{name: "spaces and CJK", rel: "Maps/日本語 課綱.md", want: true},

		{name: "empty", rel: "", want: false},
		{name: "the git directory", rel: ".git/config", want: false},
		{name: "the obsidian directory", rel: ".obsidian/plugins/x.js", want: false},
		{name: "a dot file at the root", rel: ".gitignore", want: false},
		{name: "a dot directory in the middle", rel: "Notes/.hidden/x.md", want: false},
		{name: "escaping the root", rel: "../secret.md", want: false},
		{name: "escaping through a segment", rel: "Notes/../../secret.md", want: false},
		{name: "an absolute path", rel: "/etc/passwd", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := servable(tt.rel); got != tt.want {
				t.Errorf("servable(%q) = %v, want %v", tt.rel, got, tt.want)
			}
		})
	}
}

// TestLooksText fixes the one decision the extension is never trusted with.
func TestLooksText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []byte
		want bool
	}{
		{name: "ascii", in: []byte("build:\n\tgo build\n"), want: true},
		{name: "empty", in: []byte{}, want: true},
		{name: "utf-8 CJK", in: []byte("日本語の本文\n"), want: true},
		{name: "utf-8 emoji", in: []byte("印 🖋\n"), want: true},
		{name: "a NUL anywhere makes it binary", in: []byte("text\x00more text"), want: false},
		{name: "an ELF header", in: []byte{0x7f, 'E', 'L', 'F', 0x00}, want: false},
		{name: "invalid utf-8", in: []byte{0xff, 0xfe, 0x41}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := looksText(tt.in); got != tt.want {
				t.Errorf("looksText(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestTrimPartialRune covers the sniff window's edge: a character the window
// cut in half must not make a text file read as binary.
func TestTrimPartialRune(t *testing.T) {
	t.Parallel()
	// "日" is three bytes; a window ending after one or two of them is truncated.
	full := []byte("あ日")
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{name: "nothing cut", in: full, want: "あ日"},
		{name: "one byte of the last rune", in: full[:len(full)-2], want: "あ"},
		{name: "two bytes of the last rune", in: full[:len(full)-1], want: "あ"},
		{name: "ascii only", in: []byte("abc"), want: "abc"},
		{name: "empty", in: []byte{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(trimPartialRune(tt.in))
			if got != tt.want {
				t.Errorf("trimPartialRune(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if !looksText(trimPartialRune(tt.in)) {
				t.Errorf("trimPartialRune(%q) still does not read as text", tt.in)
			}
		})
	}

	// The whole point: a truncated window without the trim reads as binary.
	cut := full[:len(full)-1]
	if looksText(cut) {
		t.Error("a window cut mid-character already reads as text; the trim guards nothing")
	}
	if !strings.HasPrefix(string(full), string(trimPartialRune(cut))) {
		t.Error("the trim dropped more than the partial character")
	}
}
