package render

import (
	"bytes"
	"testing"
)

// TestIsText fixes the one decision the extension is never trusted with.
func TestIsText(t *testing.T) {
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
			if got := IsText(tt.in); got != tt.want {
				t.Errorf("IsText(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestIsPDF fixes the viewer decision to the final extension alone, any case.
func TestIsPDF(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{path: "paper.pdf", want: true},
		{path: "Sources/Paper.PDF", want: true},
		{path: "weird.Pdf", want: true},
		{path: "note.md.pdf", want: true},
		{path: "archive.pdf.md", want: false},
		{path: "paper.pdfx", want: false},
		{path: "pdf", want: false},
		{path: "Notes/paper", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			if got := IsPDF(tt.path); got != tt.want {
				t.Errorf("IsPDF(%q) = %v, want %v", tt.path, got, tt.want)
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
			if !IsText(trimPartialRune(tt.in)) {
				t.Errorf("trimPartialRune(%q) still does not read as text", tt.in)
			}
		})
	}

	// The whole point: a truncated window without the trim reads as binary.
	cut := full[:len(full)-1]
	if IsText(cut) {
		t.Error("a window cut mid-character already reads as text; the trim guards nothing")
	}
	if !bytes.HasPrefix(full, trimPartialRune(cut)) {
		t.Error("the trim dropped more than the partial character")
	}
}
