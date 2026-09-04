package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
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

func TestBaseProjectsCanonicalBrandAssetOnce(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	const asset = "/static/yomihon-mark.svg"
	const favicon = `<link rel="icon" type="image/svg+xml" href="` + asset + `">`
	const headerMark = `<img class="y-brand__mark" src="` + asset + `" width="24" height="24" alt="" aria-hidden="true">`
	if got := strings.Count(html, favicon); got != 1 {
		t.Errorf("Base() favicon projections = %d, want one exact %q; html = %q", got, favicon, html)
	}
	if got := strings.Count(html, headerMark); got != 1 {
		t.Errorf("Base() header mark projections = %d, want one exact %q; html = %q", got, headerMark, html)
	}
	if got := strings.Count(html, asset); got != 2 {
		t.Errorf("Base() canonical brand references = %d, want favicon plus header only; html = %q", got, html)
	}
	for _, forbidden := range []string{`rel="manifest"`, `apple-touch-icon`, `.ico`, `y-brand__dot`} {
		if strings.Contains(html, forbidden) {
			t.Errorf("Base() contains forbidden brand projection %q; html = %q", forbidden, html)
		}
	}
}

func TestHeaderKeepsWordmarkAsExactAccessibleName(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	const brandLink = `<a class="y-brand__name" href="/"><img class="y-brand__mark" src="/static/yomihon-mark.svg" width="24" height="24" alt="" aria-hidden="true"><span>yomihon</span></a>`
	if got := strings.Count(html, brandLink); got != 1 {
		t.Fatalf("header brand link projections = %d, want one exact accessible wordmark; html = %q", got, html)
	}
	if strings.Contains(html, `aria-label="yomihon"`) {
		t.Errorf("header duplicates the visible accessible name with aria-label; html = %q", html)
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
				`aria-describedby="kbd-pref-note kbd-pref-takeover"`,
				`>單鍵快捷鍵<`,
				`>目前開啟<`,
				`>目前關閉<`,
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

// TestHeaderCarriesNoBareCount holds the header to what a reader can read.
// A count with no word beside it told nobody what had been counted — the
// vault's owner could not name it either — and the folder index already
// carries the same figure broken down by status, where each part is named and
// clickable.
func TestHeaderCarriesNoBareCount(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	for _, gone := range []string{"data-advanceable-chip", "y-advancechip", "還有下一步"} {
		if strings.Contains(html, gone) {
			t.Errorf("header still carries the bare advanceable count (%q); html = %q", gone, html)
		}
	}
}

// elementSubtree returns the rendered div carrying the given attribute,
// including everything nested inside it and nothing that follows it. Stopping
// at the first "</div>" would instead stop at the first nested child's close,
// so a control moved out of the element could still fall inside the slice.
func elementSubtree(t *testing.T, html, attr string) string {
	t.Helper()
	start := strings.Index(html, attr)
	if start < 0 {
		t.Fatalf("no element carrying %s; html = %q", attr, html)
	}
	start = strings.LastIndex(html[:start], "<div")
	if start < 0 {
		t.Fatalf("%s is not on a div; html = %q", attr, html)
	}
	depth := 0
	for i := start; i < len(html); {
		switch {
		case strings.HasPrefix(html[i:], "<div"):
			depth++
			i += len("<div")
		case strings.HasPrefix(html[i:], "</div>"):
			depth--
			i += len("</div>")
			if depth == 0 {
				return html[start:i]
			}
		default:
			i++
		}
	}
	t.Fatalf("element carrying %s is not closed; html = %q", attr, html)
	return ""
}

// TestShortcutPreferenceLivesWithTheKeysItGoverns checks the preference is
// reachable where a reader asks what the keys do, and that the panel answers
// the three questions a switch has to answer: which keys, which way it is
// set, and where to change it. The write face has no keyboard shortcut, so
// the panel names no status key on any folder.
func TestShortcutPreferenceLivesWithTheKeysItGoverns(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	panel := elementSubtree(t, html, `id="kbd-help"`)
	if !strings.Contains(panel, "data-single-key-shortcuts-toggle") {
		t.Errorf("the preference is not inside the keyboard help panel; panel = %q", panel)
	}
	if want := wording.SingleKeyShortcutsNote.In(wording.ZhHant); !strings.Contains(panel, want) {
		t.Errorf("panel does not say what turning it off costs; want %q; panel = %q", want, panel)
	}
	for _, want := range []string{">" + wording.CurrentlyOn.In(wording.ZhHant) + "<", ">" + wording.CurrentlyOff.In(wording.ZhHant) + "<"} {
		if !strings.Contains(panel, want) {
			t.Errorf("panel does not state which way the preference is set (%q); panel = %q", want, panel)
		}
	}
	for _, gone := range []string{"按住", "認證"} {
		if strings.Contains(panel, gone) {
			t.Errorf("panel still documents a retired status key (%q); panel = %q", gone, panel)
		}
	}
}

// TestKeyboardHelpSaysThePageTakesTheKeys checks the panel admits the price of
// an armed single key: the page holds it, so whatever else the reader's
// browser would have done with a lone slash does not happen. The switch is
// what gives that key back, so the sentence is also what the switch is
// described by, and both are asked for in both languages — a panel honest in
// one of the two is a reader in the other left with the old silence.
func TestKeyboardHelpSaysThePageTakesTheKeys(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := header(Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render header: %v", err)
			}
			panel := elementSubtree(t, buf.String(), `id="kbd-help"`)
			if want := wording.SingleKeyShortcutsTakeover.In(lang); !strings.Contains(panel, want) {
				t.Errorf("panel does not say the page takes the keys; want %q; panel = %q", want, panel)
			}
			// The sentence has to reach a reader who arrives at the checkbox
			// by ear as well as by eye, and only a named description does
			// that. Rendering it beside the control is not the same thing.
			if !strings.Contains(panel, `aria-describedby="kbd-pref-note kbd-pref-takeover"`) {
				t.Errorf("the switch is not described by both of its sentences; panel = %q", panel)
			}
			if !strings.Contains(panel, `id="kbd-pref-takeover"`) {
				t.Errorf("the sentence the switch names has no element to point at; panel = %q", panel)
			}
		})
	}
}

// TestKeyboardHelpSaysWhereTheSidebarKeyActs checks the sidebar key's row
// carries the condition that decides whether pressing it does anything. The
// row named the preference and stopped there, so at a width where the sidebar
// never folds the panel described a key that cannot work, and the reader who
// tried it had the preference to blame and no way to learn better.
func TestKeyboardHelpSaysWhereTheSidebarKeyActs(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := header(Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render header: %v", err)
			}
			panel := elementSubtree(t, buf.String(), `id="kbd-help"`)
			row := shortcutRow(t, panel, "[")
			if want := wording.ShortcutSidebarNarrowOnly.In(lang); !strings.Contains(row, want) {
				t.Errorf("the sidebar key's row does not say where it acts; want %q; row = %q", want, row)
			}
		})
	}
}

// shortcutRow returns the description the keyboard help panel gives one key.
// Asking the panel as a whole would let a sentence about the sidebar key pass
// from anywhere on the panel, including another key's row.
func shortcutRow(t *testing.T, panel, key string) string {
	t.Helper()
	marker := `<dt><span class="ui-kbd">` + key + `</span></dt><dd>`
	_, rest, found := strings.Cut(panel, marker)
	if !found {
		t.Fatalf("the panel has no row for %q; panel = %q", key, panel)
	}
	row, _, closed := strings.Cut(rest, "</dd>")
	if !closed {
		t.Fatalf("the row for %q is not closed; panel = %q", key, panel)
	}
	return row
}

func TestHeaderSearchKeepsAccessibleNameWhenLabelIsVisuallyHidden(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `class="y-searchbtn" href="/search" data-search-open aria-label="`+wording.SearchNotes.In(wording.ZhHant)+`"`) {
		t.Errorf("header search link has no stable accessible name; html = %q", html)
	}
}

func TestSearchDialogKeepsGETFallbackAroundLiveResults(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := searchDialog(wording.ZhHant).Render(t.Context(), &buf); err != nil {
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
		`data-live-search-countone="`,
		`data-live-search-countmany="`,
		`data-live-search-offline="`,
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

// The size preference must be on the root before the first paint — a page that
// paints at the default and then jumps to the reader's size would make the
// preference feel broken every time.
func TestBaseStampsTextSizeOnTheRoot(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{TextSize: "xl"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("Base().Render() error = %v", err)
	}
	if !strings.Contains(buf.String(), `data-textsize="xl"`) {
		t.Error(`Base() root is missing data-textsize="xl"`)
	}
}

// TestTextSizeControlNamesTheSizeItIsAt holds the state the cycling control
// never exposed. Its two neighbours in the same header — the theme and
// furigana toggles — carry aria-pressed and keep it current on every press, so
// a reader who cannot see the page can tell what they are at and what a press
// did. The text-size button is a three-state cycle, which no boolean can
// carry, and it was left with a fixed name: the type on screen changed and
// nothing said so, at any of the three sizes or after a reload.
func TestTextSizeControlNamesTheSizeItIsAt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		size string
		want string
	}{
		{size: "m", want: wording.TextSizeMedium.In(wording.ZhHant)},
		{size: "l", want: wording.TextSizeLarge.In(wording.ZhHant)},
		{size: "xl", want: wording.TextSizeExtraLarge.In(wording.ZhHant)},
		{size: "", want: wording.TextSizeMedium.In(wording.ZhHant)},
	}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Base(Chrome{TextSize: tt.size}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("Base().Render() error = %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, `aria-label="`+tt.want+`"`) {
				t.Errorf("the text-size control does not name its size %q:\n%s", tt.want, textSizeButton(html))
			}
			for _, other := range []string{wording.TextSizeMedium.In(wording.ZhHant), wording.TextSizeLarge.In(wording.ZhHant), wording.TextSizeExtraLarge.In(wording.ZhHant)} {
				if other != tt.want && strings.Contains(html, `aria-label="`+other+`"`) {
					t.Errorf("the control also claims to be at %q", other)
				}
			}
		})
	}
}

// textSizeButton extracts the control's own markup so a failure shows the tag
// under test rather than the whole document.
func textSizeButton(html string) string {
	at := strings.Index(html, "data-textsize-toggle")
	if at < 0 {
		return "the page carries no text-size control at all"
	}
	start := strings.LastIndex(html[:at], "<")
	end := strings.Index(html[at:], ">")
	return html[start : at+end+1]
}
