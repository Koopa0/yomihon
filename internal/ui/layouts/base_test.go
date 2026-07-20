package layouts

import (
	"bytes"
	"strings"
	"testing"
)

func TestBaseDeclaresTraditionalChineseDocumentLanguage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<html lang="zh-Hant"`) {
		t.Errorf("base does not declare the interface language; html = %q", html)
	}
}

func TestBaseStartsBodyWithSkipLink(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	want := `<body><a class="y-skiplink" href="#main-content">跳至主要內容</a>`
	if !strings.Contains(html, want) {
		t.Errorf("Base() does not start the body with %q; html = %q", want, html)
	}
}

func TestBaseLoadsOneModuleEntry(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	const entry = `<script nonce="response-nonce" type="module" src="/static/yomihon.js"></script>`
	if got := strings.Count(html, entry); got != 1 {
		t.Errorf("Base() module entries = %d, want 1 exact %q; html = %q", got, entry, html)
	}
	if got := strings.Count(html, `<script`); got != 1 {
		t.Errorf("Base() script elements = %d, want only the module entry; html = %q", got, html)
	}
}

func TestBaseRendersSingleKeyShortcutPreference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		enabled     bool
		wantState   string
		wantChecked bool
	}{
		{name: "enabled", enabled: true, wantState: "on", wantChecked: true},
		{name: "disabled", wantState: "off", wantChecked: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			chrome := Chrome{Title: "測試", SingleKeyShortcutsEnabled: tt.enabled}
			if err := Base(chrome).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render base: %v", err)
			}
			html := buf.String()
			for _, want := range []string{
				`data-single-key-shortcuts="` + tt.wantState + `"`,
				`data-single-key-shortcuts-toggle`,
				`aria-label="單字鍵快捷鍵"`,
				`>單字鍵<`,
				`>開<`,
				`>關<`,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("Base() is missing %q; html = %q", want, html)
				}
			}
			controlStart := strings.Index(html, `data-single-key-shortcuts-toggle`)
			if controlStart < 0 {
				t.Fatalf("Base() has no single-key shortcut control; html = %q", html)
			}
			controlEnd := strings.IndexByte(html[controlStart:], '>')
			if controlEnd < 0 {
				t.Fatalf("single-key shortcut control is not closed; html = %q", html)
			}
			control := html[controlStart : controlStart+controlEnd]
			if got := strings.Contains(control, "checked"); got != tt.wantChecked {
				t.Errorf("single-key shortcut control checked = %t, want %t; control = %q", got, tt.wantChecked, control)
			}
		})
	}
}

func TestBaseNeverEmitsAnUnnoncedExecutableScript(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	for rest := html; ; {
		start := strings.Index(rest, "<script")
		if start < 0 {
			break
		}
		end := strings.IndexByte(rest[start:], '>')
		if end < 0 {
			t.Fatalf("Base() has an unterminated script tag: %q", rest[start:])
		}
		tag := rest[start : start+end+1]
		if strings.Contains(tag, `type="application/json"`) {
			rest = rest[start+end+1:]
			continue
		}
		if !strings.Contains(tag, `nonce="response-nonce"`) {
			t.Errorf("Base() executable script has no response nonce: %q", tag)
		}
		rest = rest[start+end+1:]
	}
}

func TestHeaderAdvanceableChip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		chrome      Chrome
		wantChip    bool
		wantLabel   string
		wantVisible string
	}{
		{name: "known count", chrome: Chrome{Advanceable: 7, AdvanceableKnown: true}, wantChip: true, wantLabel: `aria-label="7 篇筆記可進入下一個合法狀態"`, wantVisible: ">7</a>"},
		{name: "closed policy", chrome: Chrome{}, wantChip: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := header(tt.chrome).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render header: %v", err)
			}
			html := buf.String()
			hasChip := strings.Contains(html, "data-advanceable-chip")
			if hasChip != tt.wantChip {
				t.Errorf("header advanceable chip present = %t, want %t; html = %q", hasChip, tt.wantChip, html)
			}
			if tt.wantLabel != "" && !strings.Contains(html, tt.wantLabel) {
				t.Errorf("header missing %q; html = %q", tt.wantLabel, html)
			}
			if tt.wantVisible != "" && !strings.Contains(html, tt.wantVisible) {
				t.Errorf("header missing visible count %q; html = %q", tt.wantVisible, html)
			}
			if hasChip && !strings.Contains(html, `href="/"`) {
				t.Errorf("header advanceable chip does not link Home; html = %q", html)
			}
		})
	}
}

func TestHeaderSearchKeepsAccessibleNameWhenLabelIsVisuallyHidden(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `class="y-searchbtn" href="/search" data-search-open aria-label="搜尋筆記"`) {
		t.Errorf("header search link has no stable accessible name; html = %q", html)
	}
}

func TestSearchDialogKeepsGETFallbackAroundLiveResults(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := searchDialog().Render(t.Context(), &buf); err != nil {
		t.Fatalf("render search dialog: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-live-search-endpoint="/search/results"`,
		`data-live-search-form`,
		`method="get" action="/search"`,
		`name="q"`,
		`data-live-search-input`,
		`autofocus`,
		`data-live-search-status`,
		`role="status"`,
		`aria-live="polite"`,
		`aria-atomic="true"`,
		`data-live-search-results`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("searchDialog() is missing %q; html = %q", want, html)
		}
	}
}
