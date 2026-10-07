package report

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

const oversizedBriefingZH = "這份簡報的大小是 16.0 MB（16,777,217 位元組），超過 16 MiB 的閱讀上限；原始檔仍可下載。"
const oversizedBriefingEN = "This briefing is 16.0 MB (16,777,217 bytes), over the 16 MiB reading limit; the original file can still be downloaded."

// The source bound is independent of the production declaration so changing
// that declaration cannot silently move both the reader and its oracle.
func TestRawRefusesOversizedBriefingBeforeReading(t *testing.T) {
	root := vaultWithBriefing(t)
	writeSizedBriefing(t, root, 16*1024*1024+1)
	h := newHandler(t, root)
	for _, tt := range []struct {
		name string
		lang wording.Lang
		want string
	}{
		{name: "zh-Hant", lang: wording.ZhHant, want: oversizedBriefingZH},
		{name: "en", lang: wording.En, want: oversizedBriefingEN},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, canceled := range []bool{false, true} {
				name := "ordinary"
				if canceled {
					name = "already-canceled"
				}
				t.Run(name, func(t *testing.T) {
					ctx := t.Context()
					if canceled {
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						cancel()
					}
					rr := requestBriefing(ctx, h, "/reports/"+briefingName+"/raw", tt.lang)
					t.Log("producer-hit: registered oversized briefing raw request")
					if canceled && rr.Code != http.StatusForbidden {
						t.Errorf("caught: report size refusal read an already-canceled oversized briefing: status = %d, want 403", rr.Code)
					}
					assertBriefingSizeRefusal(t, rr, tt.want)
				})
			}
		})
	}
}

func TestOversizedBriefingKeepsShellAndOriginalDownload(t *testing.T) {
	root := vaultWithBriefing(t)
	writeSizedBriefing(t, root, 16*1024*1024+1)
	h := newHandler(t, root)
	for _, tt := range []struct {
		name string
		lang wording.Lang
		want string
	}{
		{name: "zh-Hant", lang: wording.ZhHant, want: oversizedBriefingZH},
		{name: "en", lang: wording.En, want: oversizedBriefingEN},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rr := requestBriefing(t.Context(), h, "/reports/"+briefingName, tt.lang)
			if rr.Code != http.StatusOK {
				t.Fatalf("oversized shell status = %d, want 200", rr.Code)
			}
			body := rr.Body.String()
			for _, want := range []string{
				tt.want,
				`class="y-shell2"`,
				`data-reading-rail="reports"`,
				`<h1 class="y-reporttitle">Bounded briefing</h1>`,
				`<title>Bounded briefing — yomihon</title>`,
				`href="/raw/System/reports/daily-briefing/` + briefingName + `"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("oversized shell is missing %q", want)
				}
			}
			if strings.Contains(body, "<iframe") {
				t.Error("caught: oversized report shell still embeds a refused briefing")
			}
			if strings.Contains(body, "REPORT-BODY-SENTINEL") {
				t.Error("oversized shell exposed briefing body bytes")
			}
		})
	}
}

func TestWellOversizedBriefingReportsExactSize(t *testing.T) {
	root := vaultWithBriefing(t)
	writeSizedBriefing(t, root, 32*1024*1024)
	h := newHandler(t, root)
	for _, tt := range []struct {
		name string
		lang wording.Lang
		want string
	}{
		{name: "zh-Hant", lang: wording.ZhHant, want: "這份簡報的大小是 32.0 MB（33,554,432 位元組），超過 16 MiB 的閱讀上限；原始檔仍可下載。"},
		{name: "en", lang: wording.En, want: "This briefing is 32.0 MB (33,554,432 bytes), over the 16 MiB reading limit; the original file can still be downloaded."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := requestBriefing(t.Context(), h, "/reports/"+briefingName+"/raw", tt.lang)
			t.Log("producer-hit: registered 32 MiB briefing raw request")
			assertBriefingSizeRefusal(t, raw, tt.want)
			shell := requestBriefing(t.Context(), h, "/reports/"+briefingName, tt.lang)
			if shell.Code != http.StatusOK {
				t.Fatalf("32 MiB shell status = %d, want 200", shell.Code)
			}
			if !strings.Contains(shell.Body.String(), tt.want) {
				t.Errorf("caught: 32 MiB shell does not report its exact size: want %q", tt.want)
			}
		})
	}
}

func TestBriefingAtReadingBoundServesCompleteBytesAndFrame(t *testing.T) {
	root := vaultWithBriefing(t)
	want := writeSizedBriefing(t, root, 16*1024*1024)
	h := newHandler(t, root)
	rr := requestBriefing(t.Context(), h, "/reports/"+briefingName+"/raw", wording.En)
	if rr.Code != http.StatusOK {
		t.Fatalf("exactly-bound raw status = %d, want 200", rr.Code)
	}
	if !bytes.Equal(rr.Body.Bytes(), want) {
		t.Errorf("exactly-bound raw body differs from original: got %d bytes, want %d", rr.Body.Len(), len(want))
	}
	shell := requestBriefing(t.Context(), h, "/reports/"+briefingName, wording.En)
	if shell.Code != http.StatusOK || !strings.Contains(shell.Body.String(), `<iframe`) ||
		!strings.Contains(shell.Body.String(), `src="/reports/`+briefingName+`/raw"`) {
		t.Errorf("exactly-bound shell status = %d; want 200 with briefing frame", shell.Code)
	}
}

func TestBriefingReadingBoundUsesRefreshedSize(t *testing.T) {
	for _, tt := range []struct {
		name   string
		before int
		after  int
	}{
		{name: "grows after capture", before: 512, after: 16*1024*1024 + 1},
		{name: "shrinks after capture", before: 16*1024*1024 + 1, after: 512},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := vaultWithBriefing(t)
			writeSizedBriefing(t, root, tt.before)
			h := newHandler(t, root)
			want := writeSizedBriefing(t, root, tt.after)
			rr := requestBriefing(t.Context(), h, "/reports/"+briefingName+"/raw", wording.En)
			if tt.after > 16*1024*1024 {
				assertBriefingSizeRefusal(t, rr, oversizedBriefingEN)
				return
			}
			if rr.Code != http.StatusOK || !bytes.Equal(rr.Body.Bytes(), want) {
				t.Errorf("refreshed small briefing status = %d, bytes = %d; want 200 and all %d original bytes", rr.Code, rr.Body.Len(), len(want))
			}
		})
	}
}

func writeSizedBriefing(t *testing.T, root string, size int) []byte {
	t.Helper()
	prefix := []byte("<!doctype html><title>Bounded briefing</title><body>REPORT-BODY-SENTINEL")
	path := filepath.Join(root, "System", "reports", "daily-briefing", briefingName)
	// Oversized fixtures are refused from metadata, so retain the sentinel
	// prefix and extend sparsely without allocating their body bytes.
	if size > 16*1024*1024 {
		if err := os.WriteFile(path, prefix, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(path, int64(size)); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	body := bytes.Repeat([]byte("x"), size)
	copy(body, prefix)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return body
}

func requestBriefing(ctx context.Context, h http.Handler, target string, lang wording.Lang) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	r.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	rr := httptest.NewRecorder()
	origin.Protect(h).ServeHTTP(rr, r)
	return rr
}

func assertBriefingSizeRefusal(t *testing.T, rr *httptest.ResponseRecorder, sentence string) {
	t.Helper()
	if rr.Code != http.StatusForbidden {
		t.Errorf("caught: report reading bound did not refuse oversized briefing: status = %d, want 403", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("refusal Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("refusal Cache-Control = %q, want no-store", got)
	}
	if got := rr.Body.String(); got != sentence+"\n" {
		t.Errorf("refusal body does not equal the localized sentence: got %d bytes, want %d", len(got), len(sentence)+1)
	}
	if strings.Contains(rr.Body.String(), "REPORT-BODY-SENTINEL") {
		t.Error("refusal exposed briefing body bytes")
	}
}
