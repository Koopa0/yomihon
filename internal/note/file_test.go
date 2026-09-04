package note_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sentinel marks a file the routes must never serve. Its bytes appearing in any
// response body is the failure, not merely a 200: a leak that arrives with a
// 404 status is still a leak.
const sentinel = "yomihon-must-never-serve-this-sentinel"

// bigTextBytes is one byte past the source-rendering cap, so the file it fills
// takes the information page rather than the highlighter.
const bigTextBytes = (1 << 20) + 1

// fileVault writes one vault holding every kind the route must distinguish,
// plus dot-leading names and links at both leaf and directory positions. Link
// targets live inside and outside the root and carry one sentinel, so the test
// observes leaked bytes rather than trusting only the response status.
func fileVault(t *testing.T) (root string) {
	t.Helper()
	parent := t.TempDir()
	root = filepath.Join(parent, "vault")
	mkdir(t, filepath.Join(root, "Notes"))
	mkdir(t, filepath.Join(root, "Private"))
	mkdir(t, filepath.Join(root, ".git"))
	mkdir(t, filepath.Join(root, ".obsidian"))

	write(t, filepath.Join(root, "Notes", "real.md"), []byte("a real note body\n"))
	write(t, filepath.Join(root, "Makefile"), []byte("build:\n\tgo build ./...\n"))
	write(t, filepath.Join(root, "notes.txt"), []byte("plain text, no extension trickery\n"))
	write(t, filepath.Join(root, "page.html"), []byte("<script>alert(1)</script>\n"))
	// A NUL byte is what makes this binary; the name gives nothing away.
	write(t, filepath.Join(root, "blob"), []byte{0x7f, 'E', 'L', 'F', 0x00, 0x01, 0x02})
	write(t, filepath.Join(root, "pic.png"), []byte("\x89PNG\r\n\x1a\n fake pixels"))
	write(t, filepath.Join(root, "icon.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	write(t, filepath.Join(root, "doc.pdf"), []byte("%PDF-1.4 fake document"))
	write(t, filepath.Join(root, "board.canvas"), []byte(`{"nodes":[{"id":"a","type":"text"}]}`))
	write(t, filepath.Join(root, "view.base"), []byte("filters:\n  and:\n    - file.ext == \"md\"\n"))
	write(t, filepath.Join(root, "big.txt"), bytes.Repeat([]byte("x"), bigTextBytes))

	// Refused shapes and the files their links would otherwise expose.
	write(t, filepath.Join(root, ".git", "config"), []byte(sentinel+"\n"))
	write(t, filepath.Join(root, ".obsidian", "app.json"), []byte(sentinel+"\n"))
	write(t, filepath.Join(parent, "secret.txt"), []byte(sentinel+"\n"))
	write(t, filepath.Join(parent, "secret.md"), []byte(sentinel+"\n"))
	write(t, filepath.Join(root, "Private", "secret.txt"), []byte(sentinel+"\n"))
	write(t, filepath.Join(root, "Private", "secret.md"), []byte(sentinel+"\n"))
	symlink(t, filepath.Join(parent, "secret.txt"), filepath.Join(root, "escape.txt"))
	symlink(t, parent, filepath.Join(root, "up"))
	// Relative targets exercise os.Root's documented in-root link following.
	// An absolute target is rejected before it reaches the policy under test.
	symlink(t, "notes.txt", filepath.Join(root, "inside.txt"))
	symlink(t, "Private", filepath.Join(root, "linked"))
	// A link that wears a note's name. The markdown branch reads through a
	// different door than the others, and it must be the same door.
	symlink(t, filepath.Join(parent, "secret.md"), filepath.Join(root, "link.md"))
	return root
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func write(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

// fetch returns a response's status, headers, and body together, because the
// raw endpoint's contract is as much in its headers as in its bytes.
func fetch(t *testing.T, client *http.Client, url string) (code int, header http.Header, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body %s: %v", url, err)
	}
	return resp.StatusCode, resp.Header, string(b)
}

// TestFileRouteRendersEachKind pins the dispatch: the extension picks a viewer
// for the kinds a browser draws, and the bytes decide everything else. The
// markdown route is unchanged, and no file page grows a write face.
func TestFileRouteRendersEachKind(t *testing.T) {
	t.Parallel()
	srv := newServer(t, fileVault(t))

	tests := []struct {
		name    string
		path    string
		want    []string
		notWant []string
	}{
		{
			name: "no extension, text bytes, highlighted as source",
			path: "Makefile",
			want: []string{`<pre class="chroma"`, `ui-type">原始碼<`, `go build`},
		},
		{
			name: "text file is source, never executed",
			path: "page.html",
			// The file's own text is shown, tokenized and escaped. A live tag
			// here would run against the reading surface's own origin.
			want:    []string{`ui-type">原始碼<`, `<pre class="chroma"`, `&lt;`, `>alert<`},
			notWant: []string{`<script>alert(1)</script>`},
		},
		{
			name:    "binary bytes take the information page",
			path:    "blob",
			want:    []string{`ui-type">檔案資訊<`, "沒有可呈現此檔案的閱讀器", "application/octet-stream"},
			notWant: []string{`<pre class="chroma"`},
		},
		{
			name:    "text past the cap takes the information page",
			path:    "big.txt",
			want:    []string{`ui-type">檔案資訊<`, "沒有可呈現此檔案的閱讀器", "text/plain"},
			notWant: []string{`<pre class="chroma"`},
		},
		{
			name: "an image is displayed over its raw bytes",
			path: "pic.png",
			want: []string{`ui-type">圖片<`, `<img src="/raw/pic.png"`},
		},
		{
			name: "an svg is an image, not source",
			path: "icon.svg",
			want: []string{`ui-type">圖片<`, `<img src="/raw/icon.svg"`},
		},
		{
			name: "a pdf is handed to the browser's viewer",
			path: "doc.pdf",
			want: []string{`ui-type">PDF<`, `src="/raw/doc.pdf"`},
		},
		{
			name: "a canvas is highlighted source, tokenized as JSON",
			path: "board.canvas",
			want: []string{`ui-type">原始碼<`, `<pre class="chroma"`, `<span class="`},
		},
		{
			name: "a base is highlighted source, tokenized as YAML",
			path: "view.base",
			want: []string{`ui-type">原始碼<`, `<pre class="chroma"`, `<span class="`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _, body := fetch(t, srv.Client(), srv.URL+"/notes/"+tt.path)
			if code != http.StatusOK {
				t.Fatalf("GET /notes/%s = %d, want 200", tt.path, code)
			}
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("GET /notes/%s body is missing %q", tt.path, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(body, notWant) {
					t.Errorf("GET /notes/%s body contains %q, which it must not", tt.path, notWant)
				}
			}
			// A file is not a note: the write face reaches only a note's status.
			for _, face := range []string{"data-seal", "y-statuspanel", "y-sealbar"} {
				if strings.Contains(body, face) {
					t.Errorf("GET /notes/%s carries %q, but a file has no write face", tt.path, face)
				}
			}
		})
	}
}

// TestRawServesBytesUnderASandbox is the containment lock. The bytes go out
// unchanged, and every header that keeps a same-origin SVG or HTML document
// from executing against yomihon's origin is present and exact.
func TestRawServesBytesUnderASandbox(t *testing.T) {
	t.Parallel()
	root := fileVault(t)
	srv := newServer(t, root)

	code, header, body := fetch(t, srv.Client(), srv.URL+"/raw/icon.svg")
	if code != http.StatusOK {
		t.Fatalf("GET /raw/icon.svg = %d, want 200", code)
	}
	want, err := os.ReadFile(filepath.Join(root, "icon.svg")) // #nosec G304 -- the path is this test's own t.TempDir fixture
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if body != string(want) {
		t.Errorf("GET /raw/icon.svg body = %q, want the file's bytes unchanged %q", body, want)
	}

	for _, h := range []struct{ name, want string }{
		{"Content-Security-Policy", "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
			"font-src 'none'; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
			"img-src 'none'; media-src 'none'; object-src 'none'; script-src 'none'; " +
			"script-src-attr 'none'; style-src 'unsafe-inline'; worker-src 'none'"},
		{"X-Content-Type-Options", "nosniff"},
		{"Cache-Control", "no-store"},
		{"Content-Type", "image/svg+xml"},
	} {
		if got := header.Get(h.name); got != h.want {
			t.Errorf("GET /raw/icon.svg %s = %q, want %q", h.name, got, h.want)
		}
	}
	// The report route allows a briefing its own scripts; a vault file has no
	// such claim on this origin.
	if strings.Contains(header.Get("Content-Security-Policy"), "allow-scripts") {
		t.Error("the raw sandbox allows scripts; vault bytes must never execute")
	}
}

// TestRawPDFDropsTheSandbox pins the one exception to the raw sandbox. A PDF
// cannot script the app's origin — the browser renders it in its own isolated
// viewer, and the pinned type with nosniff keeps it from being read as anything
// that could — so it carries no sandbox directive, which some browsers' viewers
// refuse to load under. The framing confinement stays, so only yomihon's own
// shell may still embed it.
func TestRawPDFDropsTheSandbox(t *testing.T) {
	t.Parallel()
	srv := newServer(t, fileVault(t))

	_, pdfHeader, _ := fetch(t, srv.Client(), srv.URL+"/raw/doc.pdf")
	if got := pdfHeader.Get("Content-Security-Policy"); got != "frame-ancestors 'self'" {
		t.Errorf("GET /raw/doc.pdf CSP = %q, want the framing confinement without the sandbox", got)
	}
	// Every other kind keeps the full sandbox — the PDF exemption is exactly
	// that, not a general loosening.
	_, svgHeader, _ := fetch(t, srv.Client(), srv.URL+"/raw/icon.svg")
	const rawPolicy = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
		"font-src 'none'; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
		"img-src 'none'; media-src 'none'; object-src 'none'; script-src 'none'; " +
		"script-src-attr 'none'; style-src 'unsafe-inline'; worker-src 'none'"
	if got := svgHeader.Get("Content-Security-Policy"); got != rawPolicy {
		t.Errorf("GET /raw/icon.svg CSP = %q, want the full sandbox", got)
	}
}

// TestRawNamesTheContentType pins the type of every kind the viewers rely on,
// and shows that a name with no extension is answered from its bytes rather
// than guessed.
func TestRawNamesTheContentType(t *testing.T) {
	t.Parallel()
	srv := newServer(t, fileVault(t))

	tests := []struct{ path, want string }{
		{path: "pic.png", want: "image/png"},
		{path: "icon.svg", want: "image/svg+xml"},
		{path: "doc.pdf", want: "application/pdf"},
		{path: "notes.txt", want: "text/plain; charset=utf-8"},
		{path: "Makefile", want: "text/plain; charset=utf-8"},
		{path: "blob", want: "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			code, header, _ := fetch(t, srv.Client(), srv.URL+"/raw/"+tt.path)
			if code != http.StatusOK {
				t.Fatalf("GET /raw/%s = %d, want 200", tt.path, code)
			}
			if got := header.Get("Content-Type"); got != tt.want {
				t.Errorf("GET /raw/%s Content-Type = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRawKeepsThePinnedVaultWhenTheConfiguredNameIsReplaced(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	mkdir(t, root)
	write(t, filepath.Join(root, "README.md"), []byte("home\n"))
	write(t, filepath.Join(root, "plain.txt"), []byte("original rooted bytes\n"))
	srv := newServer(t, root)

	if err := os.Rename(root, filepath.Join(parent, "selected-vault")); err != nil {
		t.Fatalf("rename selected vault: %v", err)
	}
	mkdir(t, root)
	write(t, filepath.Join(root, "README.md"), []byte("replacement home\n"))
	write(t, filepath.Join(root, "plain.txt"), []byte("replacement path bytes\n"))

	code, _, body := fetch(t, srv.Client(), srv.URL+"/raw/plain.txt")
	if code != http.StatusOK {
		t.Fatalf("GET /raw/plain.txt = %d, want 200", code)
	}
	if body != "original rooted bytes\n" {
		t.Errorf("GET /raw/plain.txt = %q, want bytes from the pinned vault", body)
	}
}

func TestRawPreservesHTTPRangeSemantics(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), []byte("home\n"))
	write(t, filepath.Join(root, "plain.txt"), []byte("0123456789"))
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/raw/plain.txt", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=2-5")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test response; read error below is authoritative
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusPartialContent || string(body) != "2345" {
		t.Errorf("range response = %d %q, want 206 %q", resp.StatusCode, body, "2345")
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes 2-5/10" {
		t.Errorf("Content-Range = %q, want %q", got, "bytes 2-5/10")
	}
}

// TestFileRoutesRefuseWhatTheTreeDoesNotList is the defense layer the markdown
// suffix check used to provide. A dot-leading segment names a directory the
// scanner never walks — .git carries the whole vault's history — and a symlink
// at any path component is not a tree entry the scanner selected. Neither route
// may answer for those names, on either the page or the bytes.
func TestFileRoutesRefuseWhatTheTreeDoesNotList(t *testing.T) {
	t.Parallel()
	srv := newServer(t, fileVault(t))

	// Positive controls cover both a root leaf and a leaf below a real
	// directory, so refusing links cannot collapse ordinary nested reads.
	for _, rel := range []string{"Makefile", "Notes/real.md"} {
		if code, _, _ := fetch(t, srv.Client(), srv.URL+"/notes/"+rel); code != http.StatusOK {
			t.Fatalf("GET /notes/%s = %d, want 200", rel, code)
		}
		if code, _, _ := fetch(t, srv.Client(), srv.URL+"/raw/"+rel); code != http.StatusOK {
			t.Fatalf("GET /raw/%s = %d, want 200", rel, code)
		}
	}

	refused := []struct{ name, path string }{
		{name: "dot directory", path: ".git/config"},
		{name: "dot directory, second", path: ".obsidian/app.json"},
		{name: "dot file", path: ".gitignore"},
		{name: "symlink out of the vault", path: "escape.txt"},
		{name: "symlinked directory out of the vault", path: "up/secret.txt"},
		{name: "symlink inside the vault is still not a regular file", path: "inside.txt"},
		{name: "symlinked directory inside the vault", path: "linked/secret.txt"},
		{name: "symlinked directory inside the vault wearing a note path", path: "linked/secret.md"},
		{name: "a symlink wearing a note's name", path: "link.md"},
		{name: "a directory is not a file", path: "Notes"},
		{name: "a regular file is not a directory", path: "notes.txt/child"},
		{name: "encoded dot-dot", path: "%2e%2e%2fsecret.txt"},
		{name: "encoded dot-dot twice", path: "%2e%2e%2f%2e%2e%2fsecret.txt"},
		{name: "mixed dot-dot", path: "..%2fsecret.txt"},
	}
	for _, route := range []string{"/notes/", "/raw/"} {
		for _, tt := range refused {
			t.Run(route+tt.name, func(t *testing.T) {
				t.Parallel()
				code, _, body := fetch(t, srv.Client(), srv.URL+route+tt.path)
				// Exactly 404: a refusal that answers 500 tells a caller that
				// the path it guessed was special, which is itself an answer.
				if code != http.StatusNotFound {
					t.Errorf("GET %s%s = %d, want 404", route, tt.path, code)
				}
				if strings.Contains(body, sentinel) {
					t.Errorf("GET %s%s leaked the sentinel of a file it must never serve", route, tt.path)
				}
			})
		}
	}
}

// TestMarkdownStillTakesTheNotePage guards the widening against overreach: a
// note keeps the reading page, with the write face the file pages lack.
func TestMarkdownStillTakesTheNotePage(t *testing.T) {
	t.Parallel()
	srv := newServer(t, fileVault(t))

	code, _, body := fetch(t, srv.Client(), srv.URL+"/notes/Notes/real.md")
	if code != http.StatusOK {
		t.Fatalf("GET a note = %d, want 200", code)
	}
	if !strings.Contains(body, "a real note") {
		t.Error("the note page lost the note's body")
	}
	if strings.Contains(body, `ui-type">原始碼<`) {
		t.Error("a note rendered as a source file")
	}
}
