package note

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

// maxSourceBytes is the comfort cap on a file rendered as highlighted source.
// Past it the page would cost more to build and scroll than it is worth, so the
// file gets an information page pointing at its bytes instead. A note is not
// subject to this: markdown keeps the reading page at any size.
const maxSourceBytes = 1 << 20 // 1 MiB

// sniffBytes is how much of a file's opening the raw endpoint reads to decide
// between plain text and opaque bytes when the name carries no extension to go
// on. 512 is the same window the standard library's own content sniffer uses.
const sniffBytes = 512

const (
	textContentType  = "text/plain; charset=utf-8"
	octetContentType = "application/octet-stream"
)

// mediaTypes pins the content type of every kind this feature renders in a
// viewer, rather than asking the operating system's mime table, whose contents
// vary by machine. Everything else falls back to that table and then to a
// content sniff, so a stable set of viewers never depends on an /etc file.
var mediaTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".pdf":  "application/pdf",
}

// imageExts are the kinds an <img> element can display. A file's extension —
// not its bytes — chooses the viewer, because that is the only thing that says
// how the reader wants to see it; the bytes only decide text from binary.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
}

// servable restates the vault scanner's own rule at the route boundary: the
// browse tree lists exactly the regular files whose path carries no dot-leading
// segment, and those are exactly the files a reader may open.
//
// The markdown-suffix check this replaces was not merely a type filter, it was
// one of three traversal defenses. filepath.IsLocal admits ".git/config" and
// ".obsidian/plugins/x.js" — it rejects escapes above the root, not names that
// begin with a dot — and the vault's .git holds the whole history of every note
// in it. Widening the route without restating the rule here would quietly widen
// the served set to those trees.
// The segments are split on the running system's own separator, taken from the
// path after it leaves the URL's slash form. A rule about path elements has to
// agree with the system about where an element ends, or a name the system reads
// as two segments would be inspected here as one.
func servable(rel string) bool {
	if rel == "" {
		return false
	}
	name := filepath.FromSlash(rel)
	if !filepath.IsLocal(name) {
		return false
	}
	for seg := range strings.SplitSeq(name, string(filepath.Separator)) {
		if strings.HasPrefix(seg, ".") {
			return false
		}
	}
	return true
}

// openInVault opens a vault-relative path for reading and reports the entry's
// own metadata.
//
// The handle comes from an os.Root, which is what confines the path: it refuses
// every symbolic link, whether the link's target lies outside the vault or
// inside it, and whether the link is the final element or a directory buried in
// the middle of the path. A check on the last element alone would miss the
// latter. Lstat, rather than Stat, then describes the named entry itself, and
// the regular-file test is what turns away the things a link is not — a
// directory, a device, a socket.
//
// Every failure here is a 404 to the caller. Which paths exist, and which are
// merely refused, is not something a caller who guessed a path has earned.
func openInVault(root, rel string) (*os.File, fs.FileInfo, error) {
	vaultRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	// Closing the root does not close a file already opened through it.
	defer vaultRoot.Close() //nolint:errcheck // a read-only handle; a close error cannot affect the response

	name := filepath.FromSlash(rel)
	info, err := vaultRoot.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fs.ErrNotExist
	}
	f, err := vaultRoot.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, info, nil
}

// looksText reports whether b is plausibly text: no NUL byte, and valid UTF-8.
// The extension is deliberately not consulted — a .txt holding a compiled
// object must never be poured into a source page, and a build file with no
// extension at all must still read as what it is.
//
// b must not end mid-character; callers that pass a truncated window trim it
// with trimPartialRune first.
func looksText(b []byte) bool {
	return bytes.IndexByte(b, 0) < 0 && utf8.Valid(b)
}

// trimPartialRune drops a multi-byte character that a fixed-size sniff window
// cut in half, so a text file does not read as binary merely because its
// opening bytes ended in the middle of a character.
func trimPartialRune(b []byte) []byte {
	for i := len(b) - 1; i >= 0 && i >= len(b)-3; i-- {
		if b[i]&0xC0 == 0x80 {
			continue // a continuation byte: the lead is further back
		}
		if b[i]&0x80 == 0 {
			return b // plain ASCII: nothing was cut
		}
		var size int
		switch {
		case b[i]&0xE0 == 0xC0:
			size = 2
		case b[i]&0xF0 == 0xE0:
			size = 3
		case b[i]&0xF8 == 0xF0:
			size = 4
		default:
			return b // not a lead byte at all; let the UTF-8 check reject it
		}
		if i+size > len(b) {
			return b[:i]
		}
		return b
	}
	return b
}

// errUnreadable marks a failure that struck after the vault root had already
// agreed a file exists and may be served: a disk that faulted mid-read, not a
// path that was turned away. The two must answer differently. A refusal has to
// look exactly like absence, or it becomes an answer about the vault's shape;
// a fault is the server's own problem and should say so.
var errUnreadable = errors.New("unreadable file")

// readNote reads a markdown note through the containment the other kinds get.
// vault.ReadNote inspects the path string with filepath.IsLocal and then hands
// the name to os.ReadFile, which follows symbolic links: a link named like a
// note, sitting inside the vault and pointing anywhere at all, would be read and
// rendered. The vault root refuses links outright, so the note takes the same
// door as every other file, and only what the browse tree lists can be read.
func (h *Handler) readNote(rel string) (*vault.Note, error) {
	f, _, err := openInVault(h.deps.Root, rel)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // a read-only handle; a close error cannot affect the response

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", errUnreadable, rel, err)
	}
	return vault.Parse(rel, data), nil
}

// showFile serves a vault file that is not a note. The extension chooses a
// viewer for the kinds a browser renders natively; everything else is decided
// by the bytes. Text within the comfort cap becomes a highlighted source page,
// and anything left — opaque bytes, or text too large to render comfortably —
// becomes an honest information page pointing at the raw endpoint.
//
// No status face, no seal, no diagnostics: a source file is not a note, and the
// write face has no opinion about it.
func (h *Handler) showFile(w http.ResponseWriter, r *http.Request, rel string) {
	f, info, err := openInVault(h.deps.Root, rel)
	if err != nil {
		// A refused path and a missing one answer alike: a file that vanished
		// between the scan and this request, a directory, and a symlink the
		// vault root turned away are all simply not here.
		h.deps.Log.Warn("open vault file", "path", rel, "error", err)
		http.NotFound(w, r)
		return
	}
	defer f.Close() //nolint:errcheck // a read-only handle; a close error cannot affect the response

	name := path.Base(rel)
	pending, pendingKnown := h.pending()
	view := pages.FileView{
		Title:       name,
		RelPath:     rel,
		Size:        info.Size(),
		ContentType: fileContentType(rel, f),
		Sidebar:     pages.NewSidebar(h.deps.Nav(), rel, nil, pending, pendingKnown),
	}

	ext := strings.ToLower(path.Ext(rel))
	switch {
	case imageExts[ext]:
		view.Kind = pages.FileImage
	case ext == ".pdf":
		view.Kind = pages.FilePDF
	case info.Size() > maxSourceBytes:
		view.Kind = pages.FileInfo
	default:
		// Bounded by the size check above, so the whole file is in hand and the
		// text decision runs on all of it rather than a window.
		data, readErr := io.ReadAll(f)
		if readErr != nil {
			h.deps.Log.Error("read vault file", "path", rel, "error", readErr)
			http.Error(w, "cannot read file", http.StatusInternalServerError)
			return
		}
		if !looksText(data) {
			view.Kind = pages.FileInfo
			break
		}
		view.Kind = pages.FileSource
		view.SourceHTML = render.SourceHTML(name, string(data))
	}

	if err := pages.File(view, pages.ChromeFromRequest(r, name)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("render file page", "path", rel, "error", err)
	}
}

// raw serves a vault file's bytes unchanged, under the containment the report
// briefings established. Two things make it safe to hand a first-party origin
// the contents of an arbitrary file: the content type is stated outright and
// never sniffed by the browser, and a Content-Security-Policy sandbox lands the
// response in a unique opaque origin however it is loaded — inside a page, in a
// frame, or opened top-level.
//
// The sandbox here is stricter than the report route's. A briefing runs its own
// charts and so is allowed scripts; a vault file has no reason ever to execute
// against yomihon's origin, so a bare sandbox is what a same-origin SVG or HTML
// document meets. Without it, opening one top-level would give it read of the
// whole reading surface.
func (h *Handler) raw(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if !servable(rel) {
		http.NotFound(w, r)
		return
	}
	f, info, err := openInVault(h.deps.Root, rel)
	if err != nil {
		h.deps.Log.Warn("open vault file", "path", rel, "error", err)
		http.NotFound(w, r)
		return
	}
	defer f.Close() //nolint:errcheck // a read-only handle; a close error cannot affect the response

	contentType := fileContentType(rel, f)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", rawContentSecurityPolicy(contentType))
	// Cross-origin embedding is refused one layer up, in the server's own
	// header seam, so every response — this one, the report bytes, and any
	// future endpoint — carries the same refusal without each having to
	// remember it.
	// ServeContent answers range requests, which a PDF viewer relies on, and
	// leaves the content type alone because it is already set.
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// rawContentSecurityPolicy chooses how strongly a raw response is sandboxed.
//
// The sandbox exists to neutralize a same-origin document that could run
// scripts against the app's origin — an SVG or an HTML file served from this
// same host. A PDF cannot do that: the browser hands it to its own isolated
// document viewer, never renders it as a page in this origin, and the pinned
// application/pdf type with nosniff keeps it from being read as anything that
// could. So the sandbox buys a PDF no safety it does not already have, while it
// does stop some browsers' viewers from loading the document at all. A PDF
// therefore keeps only the framing confinement — yomihon's own shell is still
// the sole page that may embed it, enforced here and again by the same-origin
// resource policy the server stamps on every response — and everything else is
// fully sandboxed.
func rawContentSecurityPolicy(contentType string) string {
	if strings.HasPrefix(contentType, "application/pdf") {
		return "frame-ancestors 'self'"
	}
	return "sandbox; frame-ancestors 'self'"
}

// fileContentType names a file's bytes: the pinned type for a kind this feature
// renders, then the machine's mime table, and finally the bytes themselves for
// a name with no extension to go on. The sniff can only ever answer plain text
// or opaque bytes, so it can never talk a browser into executing anything.
//
// It leaves the reader positioned at the start of the file.
func fileContentType(rel string, f io.ReadSeeker) string {
	ext := strings.ToLower(path.Ext(rel))
	if ct, ok := mediaTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	var head [sniffBytes]byte
	n, readErr := io.ReadFull(f, head[:])
	// The rewind comes before any verdict: the caller reads this same handle
	// from the beginning, and a peek that gave up early must not leave it
	// standing in the middle of the file.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return octetContentType
	}
	// A file shorter than the window is the ordinary case, not a failure.
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return octetContentType
	}
	if looksText(trimPartialRune(head[:n])) {
		return textContentType
	}
	return octetContentType
}
