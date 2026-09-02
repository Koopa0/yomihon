// Package origin enforces the browser's same-origin resource boundary for
// every response served by yomihon's loopback reading site.
package origin

import (
	"context"
	"crypto/rand"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// corpHeader names the refusal, in one place, so the two spots that must agree
// about it cannot drift apart.
const (
	corpHeader          = "Cross-Origin-Resource-Policy"
	corpValue           = "same-origin"
	cspHeader           = "Content-Security-Policy"
	referrerPolicy      = "Referrer-Policy"
	contentTypeOptions  = "X-Content-Type-Options"
	dnsPrefetchControl  = "X-DNS-Prefetch-Control"
	referrerPolicyValue = "no-referrer"
	contentTypeNoSniff  = "nosniff"
	dnsPrefetchOff      = "off"
)

type nonceContextKey struct{}

// Nonce returns the application-script nonce issued for this response. It is
// empty only when the caller renders outside Protect, as isolated component
// tests do.
func Nonce(ctx context.Context) string {
	nonce, ok := ctx.Value(nonceContextKey{}).(string)
	if !ok {
		return ""
	}
	return nonce
}

// SetContentSecurityPolicy replaces the reading shell's default policy for a
// response whose content owns a stricter, distinct sandbox, and reports
// whether that policy will reach the reader.
//
// Calling Header.Set alone is intentionally insufficient: Protect reasserts
// the shell's own policy at the commit boundary, over any header a handler
// wrote. So the stricter policy has to reach the response writer Protect
// installed, which this locates by walking the Unwrap chain the way
// net/http.ResponseController does. A single type assertion would have missed
// it behind any middleware added later, and missed it silently.
//
// A false answer means this response is inside Protect and that writer could
// not be reached: the commit boundary will replace the stricter policy with
// the reading shell's own, which by comparison grants scripts, frames and
// same-origin fetches. Bytes that needed the stricter one must then not be
// written at all — an error page under a permissive policy is safe, the
// content is not. Outside Protect nothing rewrites the header afterwards, so
// the plain Header.Set above stands and the answer is true.
func SetContentSecurityPolicy(ctx context.Context, w http.ResponseWriter, policy string) bool {
	w.Header().Set(cspHeader, policy)
	for {
		switch writer := w.(type) {
		case interface{ setContentSecurityPolicy(string) }:
			writer.setContentSecurityPolicy(policy)
			return true
		case interface{ Unwrap() http.ResponseWriter }:
			w = writer.Unwrap()
		default:
			// The nonce is issued by Protect and by nothing else, so its
			// absence is what distinguishes a response no commit boundary
			// will touch from one whose boundary this walk failed to find.
			return Nonce(ctx) == ""
		}
	}
}

func readingPolicy(nonce string) string {
	return "default-src 'none'; base-uri 'none'; connect-src 'self'; font-src 'self'; " +
		"form-action 'self'; frame-ancestors 'none'; frame-src 'self'; img-src 'self' data:; " +
		"manifest-src 'none'; media-src 'self'; object-src 'none'; " +
		"script-src 'nonce-" + nonce + "' 'strict-dynamic'; script-src-attr 'none'; " +
		"style-src 'self' 'unsafe-inline'; worker-src 'none'"
}

// finalResponseStatus follows net/http's response state machine: informational
// statuses remain open for a later response, except 101, which transfers the
// connection to the selected protocol and is terminal.
func finalResponseStatus(statusCode int) bool {
	return statusCode == http.StatusSwitchingProtocols || statusCode >= 200
}

// Protect stamps every final response with a refusal to be embedded
// by any origin but yomihon's own. The listener is loopback, but a browser is a
// confused deputy: a page the reader visits elsewhere can still reach
// 127.0.0.1 with its own credentials and pull a response in as an image, a
// frame, or a script — learning from the load whether a named file exists and
// how large it is, and running any servable script file in its own origin.
// That is exactly the crossing the loopback boundary is meant to forbid.
// Same-origin is the whole app: the shell, its assets, and the sandboxed
// frames all load from this one origin, so nothing legitimate is refused.
//
// A final response can be committed by a named status, a body written without
// one, a copy through the writer's reader-from, a flush, or the server's
// implicit 200 after a handler returns without writing. Informational statuses
// other than 101 do not commit the final response; a later final status can
// therefore restore the header after an Early Hints handler changes it.
// Hijacking is not among these paths: it takes the connection and yomihon
// writes no HTTP response at all.
//
// What this does not defend against is a handler that reaches past the wrapper
// on purpose, unwrapping to the real writer and writing there. That is not a
// policy weakened by accident, which is what this guards, and such a handler
// could take the connection outright anyway.
func Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := rand.Text()
		cw := &writer{ResponseWriter: w, defaultCSP: readingPolicy(nonce)}
		cw.applyHeaders()
		ctx := context.WithValue(r.Context(), nonceContextKey{}, nonce)
		next.ServeHTTP(cw, r.WithContext(ctx))
		if !cw.wroteFinalHeader {
			cw.applyHeaders()
		}
	})
}

// LoopbackOnly refuses any request addressed to a name that is not this
// machine's own loopback.
//
// Binding the listener to 127.0.0.1 keeps other machines out, but it does not
// keep other names out. A page the reader visits can publish a domain whose
// address it controls, let the browser load the page, then re-answer the same
// name with 127.0.0.1. Every later request goes to yomihon while the browser
// still believes the page and the response share one origin — so it sends
// same-origin fetch metadata, the cross-origin check passes, and the script
// reads the response. The whole vault, and the one endpoint that writes, are
// then reachable from a web page. The packet never left the machine; the
// boundary did.
//
// What separates that request from a real one is the name the reader typed.
// A browser copies it into Host verbatim, so a request that arrives claiming
// any name but loopback was addressed somewhere else and is refused before a
// handler sees it. Loopback covers the whole 127.0.0.0/8 range, the IPv6
// loopback, and the reserved name localhost, which is every way a reader can
// legitimately reach a listener on this machine. The name is compared without
// case and with the root label removed, both of which the domain name system
// treats as the same name.
func LoopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, wording.ServerIsForThisMachine.In(wording.LanguageFromRequest(r)), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(host string) bool {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if name == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(strings.Trim(name, "[]"))
	return err == nil && addr.IsLoopback()
}

// writer reasserts the embed refusal at the last moment it can still be
// written — when the status line is committed, after every handler has had its
// say about the headers and before any of them reach the reader.
type writer struct {
	http.ResponseWriter

	wroteFinalHeader bool
	defaultCSP       string
	explicitCSP      string
}

func (w *writer) setContentSecurityPolicy(policy string) {
	w.explicitCSP = policy
	w.Header().Set(cspHeader, policy)
}

func (w *writer) applyHeaders() {
	w.Header().Set(corpHeader, corpValue)
	policy := w.defaultCSP
	if w.explicitCSP != "" {
		policy = w.explicitCSP
	}
	w.Header().Set(cspHeader, policy)
	w.Header().Set(referrerPolicy, referrerPolicyValue)
	w.Header().Set(contentTypeOptions, contentTypeNoSniff)
	w.Header().Set(dnsPrefetchControl, dnsPrefetchOff)
}

func (w *writer) WriteHeader(statusCode int) {
	if w.wroteFinalHeader {
		w.ResponseWriter.WriteHeader(statusCode)
		return
	}
	w.applyHeaders()
	w.ResponseWriter.WriteHeader(statusCode)
	if finalResponseStatus(statusCode) {
		w.wroteFinalHeader = true
	}
}

// Write commits the headers the way net/http itself would, so a handler that
// writes a body without naming a status still passes through WriteHeader above.
func (w *writer) Write(b []byte) (int, error) {
	if !w.wroteFinalHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap hands the real writer back, so the abilities this wrapper does not
// name for itself — hijacking, setting a deadline — stay reachable through an
// http.ResponseController rather than disappearing behind it. Flushing is named
// below precisely because it commits a response, and a response controller
// consults the outermost writer before it unwraps.
func (w *writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush commits the response, so like every other path that does, it restores
// the refusal first. A handler reaching http.Flusher by assertion lands here.
func (w *writer) Flush() {
	// http.Flusher cannot report an error. Callers that need it use FlushError.
	_ = w.FlushError() //nolint:errcheck // http.Flusher has no error return
}

// FlushError is the form an http.ResponseController looks for before it looks
// for a Flusher and before it unwraps. Naming it here is what keeps a flush
// from reaching the writer underneath and committing headers this wrapper never
// saw.
func (w *writer) FlushError() error {
	if !w.wroteFinalHeader {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// ReadFrom keeps the copy that serves a file's bytes on the writer's own fast
// path. Without it the wrapper would hide the underlying io.ReaderFrom, and
// every image and document would be copied through a buffer for no reason.
func (w *writer) ReadFrom(r io.Reader) (int64, error) {
	if !w.wroteFinalHeader {
		w.WriteHeader(http.StatusOK)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}
