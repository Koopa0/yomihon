package note

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
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
	rawSandboxPolicy = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
		"font-src 'none'; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
		"img-src 'none'; media-src 'none'; object-src 'none'; script-src 'none'; " +
		"script-src-attr 'none'; style-src 'unsafe-inline'; worker-src 'none'"
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

// showFile serves a vault file that is not a note. The extension chooses a
// viewer for the kinds a browser renders natively; everything else is decided
// by the bytes. Text within the comfort cap becomes a highlighted source page,
// and anything left — opaque bytes, or text too large to render comfortably —
// becomes an honest information page pointing at the raw endpoint.
//
// No status face, no seal, no diagnostics: a source file is not a note, and the
// write face has no opinion about it.
func (h *Handler) showFile(w http.ResponseWriter, r *http.Request, rel string, statusView status.View, snap *snapshot.View) {
	entry, ok := snap.Entry(rel)
	if !ok {
		http.Error(w, "找不到指定的檔案", http.StatusNotFound)
		return
	}
	entry, err := h.deps.Source.Refresh(entry)
	if err != nil {
		// A refused path and a missing one answer alike: a file that vanished
		// between the scan and this request, a directory, and a symlink the
		// vault root turned away are all simply not here.
		h.deps.Log.Warn("refresh vault file", "path", rel, "error", err)
		http.Error(w, "找不到指定的檔案", http.StatusNotFound)
		return
	}

	name := path.Base(rel)
	pageShell := shell.Project(statusView, snap.ArtifactPolicy(), snap)
	view := pages.FileView{
		Title:   name,
		RelPath: rel,
		Size:    entry.Size(),
		Sidebar: pages.NewSidebar(pageShell.Nav, rel),
	}

	ext := strings.ToLower(path.Ext(rel))
	switch {
	case imageExts[ext]:
		view.Kind = pages.FileImage
		view.ContentType = fileContentType(rel, nil)
	case ext == ".pdf":
		view.Kind = pages.FilePDF
		view.ContentType = fileContentType(rel, nil)
	case entry.Size() > maxSourceBytes:
		view.Kind = pages.FileInfo
		head, readErr := h.deps.Source.ReadPrefix(r.Context(), entry, sniffBytes)
		if readErr != nil {
			h.respondFileReadError(w, rel, "read vault file prefix", readErr)
			return
		}
		view.ContentType = fileContentType(rel, head)
	default:
		// Bounded by the size check above, so the whole file is in hand and the
		// text decision runs on all of it rather than a window.
		data, readErr := h.deps.Source.ReadFile(r.Context(), entry)
		if readErr != nil {
			h.respondFileReadError(w, rel, "read vault file", readErr)
			return
		}
		view.ContentType = fileContentType(rel, data)
		if !looksText(data) {
			view.Kind = pages.FileInfo
			break
		}
		view.Kind = pages.FileSource
		view.SourceHTML = render.SourceHTML(name, string(data))
	}

	if err := pages.File(view, pageShell.Chrome(r, name)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("render file page", "path", rel, "error", err)
	}
}

// raw serves a vault file's bytes unchanged, under the containment the report
// briefings established. Every response states its content type outright and
// forbids browser sniffing. Document types that could execute in yomihon's
// origin also receive a Content-Security-Policy sandbox; PDF keeps the narrower
// confinement described by rawContentSecurityPolicy.
//
// The sandbox here is stricter than the report route's. A briefing runs its own
// charts and so is allowed scripts; a vault file has no reason ever to execute
// against yomihon's origin, so a bare sandbox is what a same-origin SVG or HTML
// document meets. Without it, opening one top-level would give it read of the
// whole reading surface.
func (h *Handler) raw(w http.ResponseWriter, r *http.Request) {
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !servable(rel) {
		http.Error(w, "找不到指定的檔案", http.StatusNotFound)
		return
	}
	snap := h.deps.Snapshot().Capture()
	entry, ok := snap.Entry(rel)
	if !ok {
		http.Error(w, "找不到指定的檔案", http.StatusNotFound)
		return
	}
	entry, err := h.deps.Source.Refresh(entry)
	if err != nil {
		h.deps.Log.Warn("refresh vault file", "path", rel, "error", err)
		http.Error(w, "找不到指定的檔案", http.StatusNotFound)
		return
	}
	file, err := h.deps.Source.OpenFile(r.Context(), entry)
	if err != nil {
		h.respondFileReadError(w, rel, "open vault file", err)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			h.deps.Log.Warn("close raw vault file", "path", rel, "error", closeErr)
		}
	}()
	if err := serveRaw(w, r, rel, entry.ModTime(), file); err != nil {
		h.respondFileReadError(w, rel, "prepare raw vault file", err)
	}
}

// serveRaw writes one already-opened vault object. The caller establishes the
// rooted path identity before entering this function; ServeContent then owns
// HTTP preconditions, byte ranges, HEAD, and content length over that stable
// handle without reopening its path.
func serveRaw(
	w http.ResponseWriter,
	r *http.Request,
	rel string,
	modTime time.Time,
	content io.ReadSeeker,
) error {
	contentType, err := rawContentType(rel, content)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	origin.SetContentSecurityPolicy(w, rawContentSecurityPolicy(contentType))
	// Cross-origin embedding is refused one layer up, in the server's own
	// header seam, so every response — this one, the report bytes, and any
	// future endpoint — carries the same refusal without each having to
	// remember it.
	// ServeContent answers range requests, which a PDF viewer relies on, and
	// leaves the content type alone because it is already set.
	http.ServeContent(w, r, "", modTime, content)
	return nil
}

func (h *Handler) respondFileReadError(w http.ResponseWriter, rel, operation string, err error) {
	if errors.Is(err, vault.ErrSourceChanged) {
		h.deps.Log.Warn(operation, "path", rel, "error", err)
		http.Error(w, "找不到指定的檔案", http.StatusNotFound)
		return
	}
	h.deps.Log.Error(operation, "path", rel, "error", err)
	http.Error(w, "無法讀取檔案", http.StatusInternalServerError)
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
// served under the raw sandbox policy.
func rawContentSecurityPolicy(contentType string) string {
	if strings.HasPrefix(contentType, "application/pdf") {
		return "frame-ancestors 'self'"
	}
	return rawSandboxPolicy
}

// fileContentType names a file's bytes: the pinned type for a kind this feature
// renders, then the machine's mime table, and finally the bytes themselves for
// a name with no extension to go on. The sniff can only ever answer plain text
// or opaque bytes, so it can never talk a browser into executing anything.
func fileContentType(rel string, data []byte) string {
	if contentType, ok := namedContentType(rel); ok {
		return contentType
	}
	if len(data) > sniffBytes {
		data = data[:sniffBytes]
	}
	if looksText(trimPartialRune(data)) {
		return textContentType
	}
	return octetContentType
}

func namedContentType(rel string) (string, bool) {
	ext := strings.ToLower(path.Ext(rel))
	if ct, ok := mediaTypes[ext]; ok {
		return ct, true
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct, true
	}
	return "", false
}

func rawContentType(rel string, content io.ReadSeeker) (string, error) {
	if contentType, ok := namedContentType(rel); ok {
		return contentType, nil
	}
	var head [sniffBytes]byte
	n, readErr := io.ReadFull(content, head[:])
	_, seekErr := content.Seek(0, io.SeekStart)
	if seekErr != nil {
		return "", fmt.Errorf("rewind vault file after content sniff: %w", seekErr)
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("read vault file content sniff: %w", readErr)
	}
	return fileContentType(rel, head[:n]), nil
}
