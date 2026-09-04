// Package origin enforces the browser's same-origin resource boundary for
// every response served by yomihon's loopback reading site.
//
// It answers two more questions asked at the same edge, for the same reason —
// both are read off the request and neither belongs to any one face. Which
// language the interface speaks for this request, which the reader chooses and
// every page and every sentence then follows; and whether a write that failed
// partway through a response is a fault worth an operator's attention or a
// reader who closed the tab, which decides how loudly it is logged.
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

// Language reads which language this request asked the interface to speak,
// falling back to the default when the reader has chosen none or has sent a
// value the dictionary does not know.
func Language(r *http.Request) wording.Lang {
	c, err := r.Cookie(wording.CookieName)
	if err != nil {
		return wording.ZhHant
	}
	return wording.FromCookieValue(c.Value)
}

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
// response whose content owns a stricter sandbox, and reports whether that
// policy will reach the reader. A false answer means Protect will overwrite it
// at the commit boundary, so the bytes that needed it must not be written.
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
			// Protect issues the nonce and nothing else does, so its absence
			// distinguishes a response no commit boundary will rewrite.
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
// statuses stay open for a later response, except 101, which is terminal.
func finalResponseStatus(statusCode int) bool {
	return statusCode == http.StatusSwitchingProtocols || statusCode >= 200
}

// Protect stamps every final response with the refusal to be embedded by any
// origin but yomihon's own, the reading shell's content policy, and the
// referrer and sniffing headers. Every path that commits a response reasserts
// them first — a named status, a body written without one, a ReadFrom copy, a
// flush, and the implicit 200 after a handler writes nothing — and a new
// commit path has to do the same.
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

// LoopbackOnly refuses any request whose Host names something other than this
// machine's loopback. Binding the listener to 127.0.0.1 keeps other machines
// out but not other names: a page whose own domain re-answers as 127.0.0.1
// would otherwise reach yomihon with the browser treating the two as one
// origin.
func LoopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, wording.ServerIsForThisMachine.In(Language(r)), http.StatusForbidden)
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

// writer reasserts the response headers at the last moment they can still be
// written: when the status line is committed.
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
// name — hijacking, deadlines — stay reachable through an http.ResponseController.
func (w *writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush commits the response, so like every other path that does it reasserts
// the headers first.
func (w *writer) Flush() {
	_ = w.FlushError() //nolint:errcheck // http.Flusher has no error return
}

// FlushError is the form an http.ResponseController looks for before Flusher
// and before it unwraps, which is what keeps a flush from reaching the writer
// underneath with headers this wrapper never saw.
func (w *writer) FlushError() error {
	if !w.wroteFinalHeader {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// ReadFrom keeps a file's bytes on the underlying io.ReaderFrom's fast path,
// which the wrapper would otherwise hide.
func (w *writer) ReadFrom(r io.Reader) (int64, error) {
	if !w.wroteFinalHeader {
		w.WriteHeader(http.StatusOK)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}
