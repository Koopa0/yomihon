package note

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/vaultfs"
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
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open() error = %v", err)
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

// opaqueWriter forwards a response and names no way back to what it wrapped,
// which is how a raw file's sandbox would be lost: the commit boundary that
// carries the policy sits underneath it, out of reach.
type opaqueWriter struct{ http.ResponseWriter }

// TestServeRawWithholdsBytesItCannotConfine is the promise the raw route's
// sandbox is made of. An authored SVG or HTML file served from this origin
// runs in it, and only the policy stops that — so a response whose policy will
// be replaced at the commit boundary has to carry no vault bytes at all.
func TestServeRawWithholdsBytesItCannotConfine(t *testing.T) {
	t.Parallel()

	const document = `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`

	tests := []struct {
		name        string
		wrap        func(http.ResponseWriter) http.ResponseWriter
		wantRefusal bool
	}{
		{
			// The control: the same call through the commit boundary alone
			// serves the document, so the refusal below is the wrapper's doing.
			name: "the commit boundary itself",
			wrap: func(w http.ResponseWriter) http.ResponseWriter { return w },
		},
		{
			name:        "a wrapper that names no way back",
			wrap:        func(w http.ResponseWriter) http.ResponseWriter { return opaqueWriter{w} },
			wantRefusal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var served error
			handler := origin.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				served = serveRaw(tt.wrap(w), r, "icon.svg", time.Unix(1, 0), strings.NewReader(document))
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/raw/icon.svg", http.NoBody))

			if got := errors.Is(served, errSandboxUnavailable); got != tt.wantRefusal {
				t.Errorf("serveRaw() refused the sandbox = %v (err = %v), want %v", got, served, tt.wantRefusal)
			}
			if leaked := strings.Contains(rec.Body.String(), "<script"); leaked == tt.wantRefusal {
				t.Errorf("the file's own bytes in the response = %v, want %v; body = %q",
					leaked, !tt.wantRefusal, rec.Body.String())
			}
		})
	}
}
