package note

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// sniffBytes is how much of a file's opening the raw endpoint reads to decide
// between plain text and opaque bytes when the name carries no extension to go
// on. 512 is the same window the standard library's own content sniffer uses.
const sniffBytes = 512

const (
	textContentType  = "text/plain; charset=utf-8"
	octetContentType = "application/octet-stream"
	vaultFileSandbox = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
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

// showFile serves a vault file that is not a captured note. The extension
// chooses a viewer for the kinds a browser renders natively; everything else
// is decided by the bytes. Text within the comfort cap becomes a highlighted
// source page, except a Markdown file the contract skips, which reads as a
// document. Anything left — opaque bytes, or text too large to render
// comfortably, including a markdown file over that bound — becomes an honest
// information page pointing at the raw endpoint.
//
// No status face, no ready accent, no diagnostics: a source file is not a
// note, and the write face has no opinion about it.
func (h *Handler) showFile(w http.ResponseWriter, r *http.Request, rel string, authority status.Authority, snap *snapshot.Generation) {
	lang := origin.Language(r)
	entry, ok := snap.Entry(rel)
	if !ok {
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}
	entry, err := h.sources.Source.Refresh(entry)
	if err != nil {
		// A refused path and a missing one answer alike: a file that vanished
		// between the scan and this request, a directory, and a symlink the
		// vault root turned away are all simply not here.
		h.sources.Log.Warn("refresh vault file", "path", rel, "error", err)
		h.showNotFound(w, r, r.URL.Path, authority, snap)
		return
	}

	name := path.Base(rel)
	pageShell := shell.Project(h.sources.VaultName, authority, snap)
	view := pages.FileView{
		Title:   name,
		RelPath: rel,
		Size:    entry.Size(),
		Sidebar: pages.NewSidebar(pageShell, rel),
	}

	switch {
	case render.IsPicture(rel):
		view.Kind = pages.FileImage
		view.ContentType = fileContentType(rel, nil)
	case render.IsPDF(rel):
		view.Kind = pages.FilePDF
		view.ContentType = fileContentType(rel, nil)
	case entry.Size() > render.MaxSourceBytes:
		view.Kind = pages.FileInfo
		head, readErr := h.sources.Source.ReadPrefix(r.Context(), entry, sniffBytes)
		if readErr != nil {
			h.respondFileReadError(w, rel, "read vault file prefix", readErr, lang)
			return
		}
		view.ContentType = fileContentType(rel, head)
	default:
		// Bounded by the size check above, so the whole file is in hand and the
		// text decision runs on all of it rather than a window.
		data, readErr := h.sources.Source.ReadFile(r.Context(), entry)
		if readErr != nil {
			h.respondFileReadError(w, rel, "read vault file", readErr, lang)
			return
		}
		view.ContentType = fileContentType(rel, data)
		if !render.IsText(data) {
			view.Kind = pages.FileInfo
			break
		}
		if vault.IsMarkdown(rel) && snap.SkipsNote(rel) {
			// Left out of the library by the contract, not unreadable: it reads
			// as a document, through the same renderer a note's body goes
			// through, so a link or a picture in it resolves as it would there.
			// The frontmatter is not part of the reading, as on a note.
			view.Kind = pages.FileDocument
			view.BodyHTML = snap.Render(rel, vault.Parse(rel, data).Body, lang).HTML
			break
		}
		view.Kind = pages.FileSource
		view.SourceHTML = render.SourceHTML(name, string(data))
	}

	if err := pages.File(view, layouts.ChromeFromRequest(r, name)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "render file page", "path", rel, "error", err)
	}
}

// raw serves a vault file's bytes unchanged, under the containment the report
// briefings established. Every response states its content type outright and
// forbids browser sniffing. Document types that could execute in yomihon's
// origin also receive a Content-Security-Policy sandbox; PDF keeps the narrower
// confinement described by sandboxFor. A markup document is also marked as a
// download, so opening its address saves the file instead of rendering it.
//
// The sandbox here is tighter than the report route's in one respect: both
// policies refuse scripts, but the report policy admits data: fonts, images,
// and media so a self-contained briefing can render its embedded resources,
// while a vault file gets no sources at all. A vault file has no reason ever to
// execute against yomihon's origin, so a bare sandbox is what a same-origin SVG
// or HTML document meets. Without it, opening one top-level would give it read
// of the whole reading surface.
func (h *Handler) raw(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	rel := vault.NormalizeNFC(r.PathValue("path"))
	if !servable(rel) {
		http.Error(w, wording.FileNotFound.In(lang), http.StatusNotFound)
		return
	}
	snap := h.sources.Snapshot().Capture()
	entry, ok := snap.Entry(rel)
	if !ok {
		http.Error(w, wording.FileNotFound.In(lang), http.StatusNotFound)
		return
	}
	entry, err := h.sources.Source.Refresh(entry)
	if err != nil {
		h.sources.Log.Warn("refresh vault file", "path", rel, "error", err)
		http.Error(w, wording.FileNotFound.In(lang), http.StatusNotFound)
		return
	}
	file, err := h.sources.Source.OpenFile(r.Context(), entry)
	if err != nil {
		h.respondFileReadError(w, rel, "open vault file", err, lang)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			h.sources.Log.Warn("close raw vault file", "path", rel, "error", closeErr)
		}
	}()
	if err := serveRaw(w, r, rel, entry.ModTime(), file); err != nil {
		if errors.Is(err, errSandboxUnavailable) {
			h.sources.Log.Error("serve raw vault file", "path", rel, "error", err)
			http.Error(w, wording.SandboxUnavailable.In(lang), http.StatusInternalServerError)
			return
		}
		h.respondFileReadError(w, rel, "prepare raw vault file", err, lang)
	}
}

// errSandboxUnavailable means the response's own sandbox could not be
// established, so this route wrote no vault bytes. It is not a read failure:
// the file was in hand and readable, and what was missing was the confinement
// a same-origin document has to be served under.
var errSandboxUnavailable = errors.New("the response sandbox could not be established")

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
	// A markup document is handed over as a download rather than opened as a
	// page. Opened, its elements are the browser's to act on, and a link
	// element can ask for a page to be fetched ahead of a visit in a way the
	// policy below does not govern. None of yomihon's own pages opens one of
	// these as a document: a picture is drawn from its bytes by an img, which
	// grants markup no such reach, and that is unchanged by the disposition.
	if markupDocument(contentType) {
		w.Header().Set("Content-Disposition", attachment(path.Base(rel)))
	}
	// A same-origin document this route hands the browser gets its confinement
	// from the policy, so bytes are written only once that policy is known to
	// reach the reader. Serving them under the reading shell's own policy
	// instead would give an authored SVG or HTML file script access to the
	// whole surface, which is the one outcome the sandbox exists to prevent.
	if !origin.SetContentSecurityPolicy(r.Context(), w, sandboxFor(contentType)) {
		return fmt.Errorf("%w: %s", errSandboxUnavailable, rel)
	}
	// Cross-origin embedding is refused one layer up, in the server's own
	// header seam, so every response — this one, the report bytes, and any
	// future endpoint — carries the same refusal without each having to
	// remember it.
	// ServeContent answers range requests, which a PDF viewer relies on, and
	// leaves the content type alone because it is already set.
	http.ServeContent(w, r, "", modTime, content)
	return nil
}

func (h *Handler) respondFileReadError(w http.ResponseWriter, rel, operation string, err error, lang wording.Lang) {
	if errors.Is(err, vault.ErrSourceChanged) {
		h.sources.Log.Warn(operation, "path", rel, "error", err)
		http.Error(w, wording.FileNotFound.In(lang), http.StatusNotFound)
		return
	}
	h.sources.Log.Error(operation, "path", rel, "error", err)
	http.Error(w, wording.FileUnreadable.In(lang), http.StatusInternalServerError)
}

// sandboxFor chooses how strongly a raw response is sandboxed.
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
// served under vaultFileSandbox.
func sandboxFor(contentType string) string {
	if strings.HasPrefix(contentType, "application/pdf") {
		return "frame-ancestors 'self'"
	}
	return vaultFileSandbox
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
	if render.IsTextPrefix(data) {
		return textContentType
	}
	return octetContentType
}

// markupDocument reports whether a browser opening contentType would build a
// document of elements from it: HTML, and any XML, SVG included.
func markupDocument(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		// A type the parser cannot read is not one a browser renders either,
		// but an unreadable answer is treated as the one that needs care.
		return true
	}
	return mediaType == "text/html" ||
		strings.HasSuffix(mediaType, "/xml") ||
		strings.HasSuffix(mediaType, "+xml") ||
		mediaType == "text/xsl"
}

// attachment is the Content-Disposition that saves a file under its own name.
// The formatter quotes or percent-encodes the name as it needs, so every name
// has a value: it answers nothing only for a type or parameter name that is
// not a token, and both of these are.
func attachment(name string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": name})
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
