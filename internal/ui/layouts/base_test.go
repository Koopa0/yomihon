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

func TestHeaderAdvanceableChip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		chrome      Chrome
		wantChip    bool
		wantLabel   string
		wantVisible string
	}{
		{name: "known count", chrome: Chrome{Advanceable: 7, AdvanceableKnown: true}, wantChip: true, wantLabel: `aria-label="7 notes have a legal next status"`, wantVisible: ">7</a>"},
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
	if !strings.Contains(html, `class="y-searchbtn" href="/search" data-search-open aria-label="Search notes"`) {
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
