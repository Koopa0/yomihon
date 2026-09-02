package origin

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// The refusal exactly as it must appear on the wire. These are spelled out
// rather than taken from the middleware's own constants: a check that compares a
// constant against itself proves only that Go finds equal values equal. It would
// stay green while the server shipped a header named something else, or shipped
// "cross-origin" — which is not a weaker refusal but an invitation.
const (
	wireCORPHeader = "Cross-Origin-Resource-Policy"
	wireCORPValue  = "same-origin"
	wireCSPHeader  = "Content-Security-Policy"
)

func TestFinalResponseStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{name: "continue", statusCode: http.StatusContinue, want: false},
		{name: "processing", statusCode: http.StatusProcessing, want: false},
		{name: "early hints", statusCode: http.StatusEarlyHints, want: false},
		{name: "switching protocols", statusCode: http.StatusSwitchingProtocols, want: true},
		{name: "success", statusCode: http.StatusOK, want: true},
		{name: "server error", statusCode: http.StatusInternalServerError, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := finalResponseStatus(tt.statusCode); got != tt.want {
				t.Errorf("finalResponseStatus(%d) = %t, want %t", tt.statusCode, got, tt.want)
			}
		})
	}
}

// TestProtect pins the server-wide embed refusal. The listener
// is loopback, but a browser will carry a request to it on behalf of any page
// the reader visits; the header denies that page the embed, so it cannot read a
// vault file's existence or size from a load event or run a servable script file
// in its own origin.
//
// The refusal has to survive whatever the handler beneath it does — answer
// without writing, write without a status, or set the header to something of its
// own. No single endpoint may weaken a policy the whole server rests on.
func TestProtect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		inner http.HandlerFunc
	}{
		{
			name:  "a handler that names a status",
			inner: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
		},
		{
			name:  "a handler that answers without writing anything",
			inner: func(http.ResponseWriter, *http.Request) {},
		},
		{
			name: "a handler that writes a body without a status",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("body")) //nolint:errcheck // httptest recorder writes cannot fail
			},
		},
		{
			name: "a handler that deletes the header",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(wireCORPHeader)
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "a handler that rewrites the header to something weaker",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(wireCORPHeader, "cross-origin")
				_, _ = w.Write([]byte("body")) //nolint:errcheck // httptest recorder writes cannot fail
			},
		},
		{
			name: "a handler that fails",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "no", http.StatusNotFound)
			},
		},
		{
			// The quietest way to lose it: no status, no body, nothing for a
			// wrapper to intercept — the server writes the 200 itself, from a
			// header map the handler has already emptied.
			name: "a handler that deletes the header and never writes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(wireCORPHeader)
			},
		},
		{
			name: "a handler that weakens the header and never writes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(wireCORPHeader, "cross-origin")
			},
		},
		{
			// A flush commits the response. Reached through a controller, it
			// would otherwise walk past the wrapper to the writer underneath.
			name: "a handler that deletes the header and flushes",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(wireCORPHeader)
				_ = http.NewResponseController(w).Flush() //nolint:errcheck // the assertion is the committed headers below
			},
		},
		{
			name: "a handler that deletes the header and flushes by assertion",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Del(wireCORPHeader)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/anything", http.NoBody)
			rec := httptest.NewRecorder()
			Protect(tt.inner).ServeHTTP(rec, req)

			// Result reports the headers as they were committed. The recorder's
			// live map would happily show a value that never left, which is how
			// a bypass hides from a test that trusts it.
			if got := rec.Result().Header.Get(wireCORPHeader); got != wireCORPValue {
				t.Errorf("committed %s = %q, want %q", wireCORPHeader, got, wireCORPValue)
			}
			csp := rec.Result().Header.Get("Content-Security-Policy")
			if !strings.Contains(csp, "default-src 'none'") ||
				!strings.Contains(csp, "script-src 'nonce-") ||
				!strings.Contains(csp, "'strict-dynamic'") {
				t.Errorf("committed CSP is not the nonce-bound reading policy: %q", csp)
			}
		})
	}
}

func TestProtectIssuesOneStrictNoncePolicyPerResponse(t *testing.T) {
	t.Parallel()

	handler := Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := Nonce(r.Context())
		_, _ = w.Write([]byte(`<script nonce="` + nonce + `" type="module" src="/static/yomihon.js"></script>`)) //nolint:errcheck // recorder writes cannot fail
	}))

	type response struct {
		nonce string
		csp   string
		body  string
	}
	responses := make([]response, 2)
	for i := range responses {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		result := rec.Result()
		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("read protected response %d: %v", i, err)
		}
		if err := result.Body.Close(); err != nil {
			t.Fatalf("close protected response %d: %v", i, err)
		}
		wire := string(body)
		const prefix = `<script nonce="`
		start := strings.Index(wire, prefix)
		if start < 0 {
			t.Fatalf("response %d has no nonce-bearing module: %q", i, wire)
		}
		start += len(prefix)
		end := strings.Index(wire[start:], `"`)
		if end < 0 {
			t.Fatalf("response %d has no closing nonce quote: %q", i, wire)
		}
		responses[i] = response{
			nonce: wire[start : start+end],
			csp:   result.Header.Get("Content-Security-Policy"),
			body:  wire,
		}
		if responses[i].nonce == "" {
			t.Fatalf("response %d nonce is empty", i)
		}
		wantCSP := "default-src 'none'; base-uri 'none'; connect-src 'self'; font-src 'self'; " +
			"form-action 'self'; frame-ancestors 'none'; frame-src 'self'; img-src 'self' data:; " +
			"manifest-src 'none'; media-src 'self'; object-src 'none'; " +
			"script-src 'nonce-" + responses[i].nonce + "' 'strict-dynamic'; script-src-attr 'none'; " +
			"style-src 'self' 'unsafe-inline'; worker-src 'none'"
		if responses[i].csp != wantCSP {
			t.Errorf("response %d CSP = %q, want byte-exact policy %q", i, responses[i].csp, wantCSP)
		}
		for name, want := range map[string]string{
			"Referrer-Policy":        "no-referrer",
			"X-Content-Type-Options": "nosniff",
			"X-DNS-Prefetch-Control": "off",
		} {
			if got := result.Header.Get(name); got != want {
				t.Errorf("response %d %s = %q, want %q", i, name, got, want)
			}
		}
	}
	if responses[0].nonce == responses[1].nonce {
		t.Errorf("two responses reused nonce %q", responses[0].nonce)
	}
}

func TestProtectOnlyAcceptsExplicitContentSecurityPolicyOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inner      http.HandlerFunc
		wantPolicy string
	}{
		{
			name: "a direct weak header is replaced",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Security-Policy", "default-src *")
				w.WriteHeader(http.StatusOK)
			},
			wantPolicy: "default-src 'none'",
		},
		{
			name: "an explicit raw policy survives",
			inner: func(w http.ResponseWriter, r *http.Request) {
				SetContentSecurityPolicy(r.Context(), w, "sandbox; default-src 'none'; frame-ancestors 'self'")
				w.WriteHeader(http.StatusOK)
			},
			wantPolicy: "sandbox; default-src 'none'; frame-ancestors 'self'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			rec := httptest.NewRecorder()
			Protect(tt.inner).ServeHTTP(rec, req)
			got := rec.Result().Header.Get("Content-Security-Policy")
			if !strings.Contains(got, tt.wantPolicy) {
				t.Errorf("committed CSP = %q, want it to contain %q", got, tt.wantPolicy)
			}
			if tt.wantPolicy != "default-src 'none'" && got != tt.wantPolicy {
				t.Errorf("committed explicit CSP = %q, want exact %q", got, tt.wantPolicy)
			}
		})
	}
}

func TestProtectRestoresFinalHeaderAfterEarlyHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inner      http.HandlerFunc
		wantStatus int
	}{
		{
			name: "explicit final status",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusEarlyHints)
				w.Header().Del(wireCORPHeader)
				w.WriteHeader(http.StatusNoContent)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "implicit final status after multiple informational statuses",
			inner: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusEarlyHints)
				w.Header().Del(wireCORPHeader)
				w.WriteHeader(http.StatusProcessing)
				w.Header().Del(wireCORPHeader)
			},
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(Protect(tt.inner))
			t.Cleanup(server.Close)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
			if err != nil {
				t.Fatalf("create protected request: %v", err)
			}
			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("GET protected server: %v", err)
			}
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("close protected response: %v", err)
				}
			})
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("response status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := resp.Header.Get(wireCORPHeader); got != wireCORPValue {
				t.Errorf("final %s = %q, want %q", wireCORPHeader, got, wireCORPValue)
			}
		})
	}
}

// TestWriterKeepsTheWriterUnderneathReachable pins that the wrapper hides none
// of the abilities the real writer has. A response controller reaches the
// wrapper's FlushError directly and follows Unwrap for Hijack; the copy that
// serves a file's bytes must still find the writer's own io.ReaderFrom rather
// than fall back to a buffer.
func TestWriterKeepsTheWriterUnderneathReachable(t *testing.T) {
	t.Parallel()

	t.Run("a flush reaches the writer underneath", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		w := &writer{ResponseWriter: rec}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through the wrapper = %v, want it to reach the writer", err)
		}
		if !rec.Flushed {
			t.Error("the writer underneath was never flushed")
		}
	})

	t.Run("the copy fast path is offered, and commits the refusal", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		w := &writer{ResponseWriter: rec}

		// io.Copy reaches for the destination's io.ReaderFrom; hiding it would
		// send every served file through a buffer instead.
		rf, ok := any(w).(io.ReaderFrom)
		if !ok {
			t.Fatal("the wrapper hides the writer's io.ReaderFrom")
		}
		// Called directly, not through io.Copy: io.Copy prefers a source's
		// io.WriterTo, so a copy from a strings.Reader would never reach here
		// and would prove nothing about this path.
		if _, err := rf.ReadFrom(strings.NewReader("bytes")); err != nil {
			t.Fatalf("ReadFrom = %v", err)
		}
		if got := rec.Result().Header.Get(wireCORPHeader); got != wireCORPValue {
			t.Errorf("after a copy, %s = %q, want %q", wireCORPHeader, got, wireCORPValue)
		}
		if rec.Body.String() != "bytes" {
			t.Errorf("body = %q, want %q", rec.Body.String(), "bytes")
		}
	})

	t.Run("a hijack reaches the writer underneath", func(t *testing.T) {
		t.Parallel()
		result := make(chan error, 1)
		server := httptest.NewServer(Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			conn, rw, err := http.NewResponseController(w).Hijack()
			if err != nil {
				result <- err
				return
			}
			_, writeErr := rw.WriteString("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n")
			flushErr := rw.Flush()
			closeErr := conn.Close()
			result <- errors.Join(writeErr, flushErr, closeErr)
		})))
		t.Cleanup(server.Close)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
		if err != nil {
			t.Fatalf("create hijack request: %v", err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("GET hijacked server: %v", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close hijacked response: %v", err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("hijacked response status = %d, want %d", resp.StatusCode, http.StatusNoContent)
		}
		if err := <-result; err != nil {
			t.Errorf("Hijack through wrapper: %v", err)
		}
	})
}

// A loopback listener keeps other machines out; it does not keep other names
// out. A page can publish a domain, let the browser load it, then re-answer
// that name with 127.0.0.1 — after which the browser calls yomihon
// same-origin, the cross-origin check passes, and a script on that page reads
// the vault and reaches the endpoint that writes. The name the reader typed is
// what tells the two apart, so a request claiming any other name is refused
// before a handler runs. The allowed forms are every way a reader can reach a
// listener on this machine, spelled the several ways the domain name system
// treats as one name.
func TestLoopbackOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		want int
	}{
		{name: "the address the listener binds", host: "127.0.0.1:9610", want: http.StatusOK},
		{name: "the reserved name for it", host: "localhost:9610", want: http.StatusOK},
		{name: "a default port leaves no colon", host: "127.0.0.1", want: http.StatusOK},
		{name: "the rest of the loopback range", host: "127.0.0.2:9610", want: http.StatusOK},
		{name: "loopback over the sixth version", host: "[::1]:9610", want: http.StatusOK},
		{name: "names carry no case", host: "LocalHost:9610", want: http.StatusOK},
		{name: "the root label is the same name", host: "localhost.:9610", want: http.StatusOK},

		{name: "the rebound name itself", host: "evil.example:9610", want: http.StatusForbidden},
		{name: "and without a port", host: "evil.example", want: http.StatusForbidden},
		{name: "a name that merely opens with it", host: "localhost.evil.example:9610", want: http.StatusForbidden},
		{name: "a name that merely ends with it", host: "notlocalhost:9610", want: http.StatusForbidden},
		{name: "an address off this machine", host: "192.168.8.225:9610", want: http.StatusForbidden},
		{name: "the wildcard is not an address to reach", host: "0.0.0.0:9610", want: http.StatusForbidden},
		{name: "no name at all", host: "", want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reached := false
			handler := LoopbackOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/a.md", http.NoBody)
			req.Host = tt.host
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("Host %q status = %d, want %d", tt.host, rec.Code, tt.want)
			}
			if want := tt.want == http.StatusOK; reached != want {
				t.Errorf("Host %q reached the handler = %v, want %v", tt.host, reached, want)
			}
		})
	}
}

// unwrappingWriter is the shape every well-behaved response wrapper in the
// standard library has: it adds behaviour and hands the writer beneath it back
// on request, so an optional capability behind it stays reachable.
type unwrappingWriter struct{ http.ResponseWriter }

func (w unwrappingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// opaqueWriter is the shape that loses the capability: it forwards writes and
// names no way back to what it wrapped.
type opaqueWriter struct{ http.ResponseWriter }

// TestAStricterPolicySurvivesEveryWrapperThatUnwraps is the contract two
// routes serving vault bytes rest on. Each hands the browser a document that
// would otherwise run in yomihon's own origin, and the policy is the whole of
// its confinement — so a middleware added between the commit boundary and the
// handler must not be able to cost them that policy without either of them
// noticing.
func TestAStricterPolicySurvivesEveryWrapperThatUnwraps(t *testing.T) {
	t.Parallel()

	const strict = "sandbox; default-src 'none'; frame-ancestors 'self'"

	tests := []struct {
		name       string
		wrap       func(http.ResponseWriter) http.ResponseWriter
		wantPinned bool
		wantPolicy string
	}{
		{
			name:       "the commit boundary itself",
			wrap:       func(w http.ResponseWriter) http.ResponseWriter { return w },
			wantPinned: true, wantPolicy: strict,
		},
		{
			name:       "one wrapper that unwraps",
			wrap:       func(w http.ResponseWriter) http.ResponseWriter { return unwrappingWriter{w} },
			wantPinned: true, wantPolicy: strict,
		},
		{
			name: "two wrappers that unwrap",
			wrap: func(w http.ResponseWriter) http.ResponseWriter {
				return unwrappingWriter{unwrappingWriter{w}}
			},
			wantPinned: true, wantPolicy: strict,
		},
		{
			// The case the answer exists for: nothing leads back to the
			// commit boundary, so it reasserts the reading shell's own
			// policy over the strict one and the caller is told.
			name:       "a wrapper that names no way back",
			wrap:       func(w http.ResponseWriter) http.ResponseWriter { return opaqueWriter{w} },
			wantPinned: false, wantPolicy: "script-src 'nonce-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var pinned bool
			handler := Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pinned = SetContentSecurityPolicy(r.Context(), tt.wrap(w), strict)
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/raw/icon.svg", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if pinned != tt.wantPinned {
				t.Errorf("SetContentSecurityPolicy() = %v, want %v", pinned, tt.wantPinned)
			}
			if got := rec.Result().Header.Get(wireCSPHeader); !strings.Contains(got, tt.wantPolicy) {
				t.Errorf("committed policy = %q, want it to contain %q", got, tt.wantPolicy)
			}
		})
	}
}

// TestAStricterPolicyStandsWhereNothingWouldRewriteIt keeps the answer from
// meaning "a commit boundary was found" rather than "the policy reaches the
// reader". A response served outside Protect has nothing that will rewrite the
// header afterwards, so the plain header set is the whole mechanism and the
// caller has no reason to withhold its bytes.
func TestAStricterPolicyStandsWhereNothingWouldRewriteIt(t *testing.T) {
	t.Parallel()

	const strict = "sandbox; default-src 'none'"
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/raw/icon.svg", http.NoBody)
	rec := httptest.NewRecorder()

	if !SetContentSecurityPolicy(req.Context(), rec, strict) {
		t.Error("SetContentSecurityPolicy() = false outside Protect, where the header it just set is final")
	}
	if got := rec.Header().Get(wireCSPHeader); got != strict {
		t.Errorf("header = %q, want %q", got, strict)
	}
}

// TestALoopbackRefusalSpeaksTheReadersLanguage covers the one sentence this
// package writes for a person. It used to be an English literal, which a
// reader working in the other language met as the only untranslated line the
// interface ever showed them — and on a refusal, where being understood
// matters most.
func TestALoopbackRefusalSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cookie string
		want   wording.Lang
	}{
		{name: "no choice means the default", want: wording.ZhHant},
		{name: "a reader who chose English", cookie: string(wording.En), want: wording.En},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := LoopbackOnly(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("a refused request reached the handler")
			}))
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			request.Host = "evil.example"
			if tt.cookie != "" {
				request.Header.Set("Cookie", wording.CookieName+"="+tt.cookie)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
			want := wording.ServerIsForThisMachine.In(tt.want)
			if got := strings.TrimSpace(response.Body.String()); got != want {
				t.Errorf("refusal = %q, want %q", got, want)
			}
		})
	}
}
