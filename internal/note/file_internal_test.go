package note

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
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
	if !bytes.HasPrefix(full, trimPartialRune(cut)) {
		t.Error("the trim dropped more than the partial character")
	}
}

func TestServeRawReadsOnlyTheRequestedRange(t *testing.T) {
	t.Parallel()
	const size = int64(64 << 20)
	content := &countingSeeker{size: size}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/raw/large", http.NoBody)
	req.Header.Set("Range", "bytes=33554432-33554435")
	recorder := httptest.NewRecorder()

	if err := serveRaw(recorder, req, "large", time.Time{}, content); err != nil {
		t.Fatalf("serveRaw() error = %v", err)
	}
	result := recorder.Result()
	defer result.Body.Close() //nolint:errcheck // in-memory response body
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if result.StatusCode != http.StatusPartialContent || string(body) != "xxxx" {
		t.Errorf("range response = %d %q, want 206 %q", result.StatusCode, body, "xxxx")
	}
	if got := result.Header.Get("Content-Range"); got != "bytes 33554432-33554435/67108864" {
		t.Errorf("Content-Range = %q, want %q", got, "bytes 33554432-33554435/67108864")
	}
	// An extensionless file needs one fixed content sniff. The body read stays
	// at the requested four bytes instead of scaling with the 64 MiB object.
	if got, want := content.read, int64(sniffBytes+4); got != want {
		t.Errorf("source bytes read = %d, want %d-byte sniff plus requested range", got, want)
	}
}

func TestServeRawHEADAndConditionalRequestsReadNoBody(t *testing.T) {
	t.Parallel()
	modified := time.Date(2026, time.July, 18, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name       string
		method     string
		rel        string
		ifModified string
		wantStatus int
		wantRead   int64
	}{
		{name: "known type HEAD", method: http.MethodHead, rel: "large.pdf", wantStatus: http.StatusOK},
		{
			name:       "extensionless HEAD",
			method:     http.MethodHead,
			rel:        "large",
			wantStatus: http.StatusOK,
			wantRead:   sniffBytes,
		},
		{
			name:       "not modified",
			method:     http.MethodGet,
			rel:        "large.pdf",
			ifModified: modified.Format(http.TimeFormat),
			wantStatus: http.StatusNotModified,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := &countingSeeker{size: 64 << 20}
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/raw/"+tt.rel, http.NoBody)
			if tt.ifModified != "" {
				req.Header.Set("If-Modified-Since", tt.ifModified)
			}
			recorder := httptest.NewRecorder()
			if err := serveRaw(recorder, req, tt.rel, modified, content); err != nil {
				t.Fatalf("serveRaw() error = %v", err)
			}
			result := recorder.Result()
			defer result.Body.Close() //nolint:errcheck // in-memory response body
			body, err := io.ReadAll(result.Body)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if result.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", result.StatusCode, tt.wantStatus)
			}
			if len(body) != 0 {
				t.Errorf("response body length = %d, want 0", len(body))
			}
			if content.read != tt.wantRead {
				t.Errorf("source bytes read = %d, want %d", content.read, tt.wantRead)
			}
		})
	}
}

func TestServeRawKeepsOpenedObjectAcrossAtomicReplacement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "plain")
	if err := os.WriteFile(path, []byte("selected"), 0o600); err != nil {
		t.Fatalf("write selected file: %v", err)
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	entry, err := reader.Lookup("plain")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	file, err := reader.OpenFile(t.Context(), entry)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("File.Close() error = %v", closeErr)
		}
	})
	if renameErr := os.Rename(path, filepath.Join(root, "selected")); renameErr != nil {
		t.Fatalf("rename selected file: %v", renameErr)
	}
	if writeErr := os.WriteFile(path, []byte("replacement"), 0o600); writeErr != nil {
		t.Fatalf("write replacement: %v", writeErr)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/raw/plain", http.NoBody)
	if serveErr := serveRaw(recorder, req, "plain", entry.ModTime(), file); serveErr != nil {
		t.Fatalf("serveRaw() error = %v", serveErr)
	}
	if got := recorder.Body.String(); got != "selected" {
		t.Errorf("serveRaw() body = %q, want bytes from opened object", got)
	}
}

type countingSeeker struct {
	size   int64
	offset int64
	read   int64
}

func (s *countingSeeker) Read(p []byte) (int, error) {
	if s.offset >= s.size {
		return 0, io.EOF
	}
	n := min(int64(len(p)), s.size-s.offset)
	for i := range int(n) {
		p[i] = 'x'
	}
	s.offset += n
	s.read += n
	return int(n), nil
}

func (s *countingSeeker) Seek(offset int64, whence int) (int64, error) {
	next := offset
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		next += s.offset
	case io.SeekEnd:
		next += s.size
	default:
		return 0, fmt.Errorf("unknown seek origin %d", whence)
	}
	if next < 0 {
		return 0, fmt.Errorf("negative seek offset %d", next)
	}
	s.offset = next
	return next, nil
}
