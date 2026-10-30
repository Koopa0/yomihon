package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/asset"
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
	want := `<body><a class="y-skiplink" href="#_y-main">跳至主要內容</a>`
	if !strings.Contains(html, want) {
		t.Errorf("Base() does not start the body with %q; html = %q", want, html)
	}
}

// TestBaseCarriesOnlyTheScriptedMarkRulesImportMapAndModuleEntry pins
// the four script elements a page carries, and the order they come in. The
// mark sets data-js before the first style so a scripted page is laid out as it
// will stay, and says whether this document is the answer to a press; the
// speculation rules follow it, ahead of the stylesheets, so the browser learns
// what to fetch as early as the head allows; the import map names versioned
// modules before the module entry closes the body.
// Another element, or any of these out of place, is a change to what every
// page runs.
func TestBaseCarriesOnlyTheScriptedMarkRulesImportMapAndModuleEntry(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	// The version value itself is the asset registry's to pin; here only the
	// entry's place and its versioned address matter.
	const entry = `<script nonce="response-nonce" type="module" src="/static/yomihon.js?v=`
	const importMap = `<script type="importmap" nonce="response-nonce">`
	// The statement stands on its own line because that is how templ fmt lays
	// out a script element, and the format check keeps it so.
	const mark = "<script nonce=\"response-nonce\">\n\t\t\t\t{\n\t\t\t\t\tconst d = document.documentElement;\n\t\t\t\t\td.dataset.js = \"on\";\n\t\t\t\t\tconst entry = performance.getEntriesByType(\"navigation\")[0];\n\t\t\t\t\tconst kept = new URLSearchParams(location.search).has(\"at\");\n\t\t\t\t\tif (entry?.type === \"back_forward\") d.dataset.arrival = \"traverse\";\n\t\t\t\t\telse if (entry?.type === \"reload\") d.dataset.arrival = \"reload\";\n\t\t\t\t\telse if (kept || entry?.name.includes(\"#\"))\n\t\t\t\t\t\td.dataset.arrival = \"place\";\n\t\t\t\t\tif (document.prerendering) {\n\t\t\t\t\t\td.dataset.prerender = \"\";\n\t\t\t\t\t\tdocument.onprerenderingchange = () => delete d.dataset.prerender;\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t</script>"
	const rules = `<script type="speculationrules" nonce="response-nonce">` + `{"prefetch":`
	if got := strings.Count(html, entry); got != 1 {
		t.Errorf("Base() module entries = %d, want 1 opening %q; html = %q", got, entry, html)
	}
	if got := strings.Count(html, mark); got != 1 {
		t.Errorf("Base() scripted marks = %d, want 1 exact %q; html = %q", got, mark, html)
	}
	if got := strings.Count(html, rules); got != 1 {
		t.Errorf("Base() speculation rules = %d, want 1 element opening %q; html = %q", got, rules, html)
	}
	if got := strings.Count(html, importMap); got != 1 {
		t.Errorf("Base() import maps = %d, want one element opening %q; html = %q", got, importMap, html)
	}
	if got := strings.Count(html, `<script`); got != 4 {
		t.Errorf("Base() script elements = %d, want the scripted mark, the speculation rules, the import map and the module entry; html = %q", got, html)
	}
	markAt := strings.Index(html, mark)
	rulesAt := strings.Index(html, rules)
	mapAt := strings.Index(html, importMap)
	firstStyle := strings.Index(html, `<link rel="stylesheet"`)
	entryAt := strings.Index(html, entry)
	if firstStyle < 0 {
		t.Fatalf("Base() has no stylesheet link, so the order of the mark cannot be checked; html = %q", html)
	}
	if markAt < 0 || markAt > firstStyle {
		t.Errorf("Base() scripted mark at %d, first stylesheet at %d; the mark must come first so the first style already sees data-js; html = %q", markAt, firstStyle, html)
	}
	if rulesAt < markAt || rulesAt > firstStyle {
		t.Errorf("Base() speculation rules at %d, mark at %d, first stylesheet at %d; the rules come after the mark and before the stylesheets; html = %q", rulesAt, markAt, firstStyle, html)
	}
	if entryAt < firstStyle {
		t.Errorf("Base() module entry at %d, first stylesheet at %d; the entry closes the body; html = %q", entryAt, firstStyle, html)
	}
	if markAt >= rulesAt || rulesAt >= mapAt || mapAt >= entryAt {
		t.Errorf("Base() script order is mark at %d, rules at %d, import map at %d, entry at %d; want mark, rules, map, entry; html = %q", markAt, rulesAt, mapAt, entryAt, html)
	}
}

func TestBaseProjectsCanonicalBrandAssetOnce(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Assets: asset.Versions{Token: "recorded0000"}}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	const brandAsset = "/static/yomihon-mark.svg?v=recorded0000"
	const favicon = `<link rel="icon" type="image/svg+xml" href="` + brandAsset + `">`
	const headerMark = `<img class="y-brand__mark" src="` + brandAsset + `" width="24" height="24" alt="" aria-hidden="true">`
	if got := strings.Count(html, favicon); got != 1 {
		t.Errorf("Base() favicon projections = %d, want one exact %q; html = %q", got, favicon, html)
	}
	if got := strings.Count(html, headerMark); got != 1 {
		t.Errorf("Base() header mark projections = %d, want one exact %q; html = %q", got, headerMark, html)
	}
	if got := strings.Count(html, brandAsset); got != 2 {
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
	if err := header(Chrome{Assets: asset.Versions{Token: "recorded0000"}}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	const brandLink = `<a class="y-brand__name" href="/"><img class="y-brand__mark" src="/static/yomihon-mark.svg?v=recorded0000" width="24" height="24" alt="" aria-hidden="true"><span>yomihon</span></a>`
	if got := strings.Count(html, brandLink); got != 1 {
		t.Fatalf("header brand link projections = %d, want one exact accessible wordmark; html = %q", got, html)
	}
	if strings.Contains(html, `aria-label="yomihon"`) {
		t.Errorf("header duplicates the visible accessible name with aria-label; html = %q", html)
	}
}

// TestBaseStampsSingleKeyShortcutPreference holds the root to the stored
// choice the keyboard module reads, and the chrome to drawing no switch for
// it: the switch is on the settings page, beside the sentence naming the keys.
func TestBaseStampsSingleKeyShortcutPreference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		enabled   bool
		wantState string
	}{
		{name: "enabled", enabled: true, wantState: "on"},
		{name: "disabled", wantState: "off"},
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
			if want := `data-single-key-shortcuts="` + tt.wantState + `"`; !strings.Contains(html, want) {
				t.Errorf("Base() is missing %q; html = %q", want, html)
			}
			if strings.Contains(html, `type="checkbox"`) {
				t.Errorf("Base() draws a checkbox; the single-key switch belongs to the settings page; html = %q", html)
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

// TestKeyboardHelpNamesItsOwnKeyAndTheWayToTheSwitch checks the panel the ?
// key opens answers the questions a reader opening it has: that ? is the key
// that brought it, and where the switch for the single keys is. The write face
// has no keyboard shortcut, so the panel names no status key on any folder.
func TestKeyboardHelpNamesItsOwnKeyAndTheWayToTheSwitch(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := keyboardHelp(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render keyboard help: %v", err)
	}
	panel := elementSubtree(t, buf.String(), `id="_y-kbd-help"`)
	if row := strings.TrimSpace(shortcutRow(t, panel, "?")); row != wording.ShortcutOpenHelp.In(wording.ZhHant) {
		t.Errorf("the help key's row = %q, want %q", row, wording.ShortcutOpenHelp.In(wording.ZhHant))
	}
	if !strings.Contains(panel, `href="/preferences"`) {
		t.Errorf("panel does not lead to the page holding the single-key switch; panel = %q", panel)
	}
	if strings.Contains(panel, `type="checkbox"`) {
		t.Errorf("panel still carries a switch of its own; panel = %q", panel)
	}
	for _, gone := range []string{"按住", "認證"} {
		if strings.Contains(panel, gone) {
			t.Errorf("panel still documents a retired status key (%q); panel = %q", gone, panel)
		}
	}
}

// TestKeyboardHelpDoesNotDisownTheSidebarKey holds the sidebar key's row to the
// one sentence about what it does. The key once carried a second sentence
// saying it did nothing where the sidebar stays in view; it now folds the
// column at those widths too, so that sentence would promise silence from a
// key that answers.
func TestKeyboardHelpDoesNotDisownTheSidebarKey(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := keyboardHelp(Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render keyboard help: %v", err)
			}
			panel := elementSubtree(t, buf.String(), `id="_y-kbd-help"`)
			row := strings.TrimSpace(shortcutRow(t, panel, "["))
			if want := wording.ShortcutToggleSidebar.In(lang); row != want {
				t.Errorf("the sidebar key's row = %q, want exactly %q", row, want)
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
	if !strings.Contains(html, `commandfor="_y-search-dialog" data-search-open aria-label="`+wording.SearchNotes.In(wording.ZhHant)+`"`) {
		t.Errorf("header search control has no stable accessible name; html = %q", html)
	}
}

// TestTheHeaderPressNamesTheDialogItOpens is the markup half of opening the
// palette: the press says what it opens and what to do with it, so the browser
// does it and a page running no script still answers the press. The dialog has
// to carry the id the press names, or the press names nothing.
func TestTheHeaderPressNamesTheDialogItOpens(t *testing.T) {
	t.Parallel()

	var head bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &head); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := head.String()
	if !strings.Contains(html, `<button class="y-searchbtn" type="button" command="show-modal" commandfor="_y-search-dialog"`) {
		t.Errorf("the header press does not declare what it opens; html = %q", html)
	}

	var dialog bytes.Buffer
	if err := searchDialog(wording.ZhHant).Render(t.Context(), &dialog); err != nil {
		t.Fatalf("render search dialog: %v", err)
	}
	if !strings.Contains(dialog.String(), `<dialog id="_y-search-dialog"`) {
		t.Errorf("the palette does not carry the id the header press names; html = %q", dialog.String())
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

// The folded column has to be on the root before the first paint, and in the
// markup rather than in anything a script sets: the script arrives after the
// page has painted, and a column that painted and then vanished on every load
// would be worse than one that never folded.
func TestBaseStampsTheColumnStateOnTheRoot(t *testing.T) {
	t.Parallel()
	for _, rail := range []string{"open", "collapsed"} {
		var buf bytes.Buffer
		if err := Base(Chrome{Rail: rail}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("Base().Render() error = %v", err)
		}
		_, afterDoctype, _ := strings.Cut(buf.String(), "<html")
		root, _, _ := strings.Cut(afterDoctype, ">")
		if !strings.Contains(root, `data-rail="`+rail+`"`) {
			t.Errorf("Base() root is missing data-rail=%q; root = %q", rail, root)
		}
	}
}

// TestHeaderCarriesOnlyWhatReadingNeeds holds the row to the controls a reader
// uses while reading, in the order that keeps the fixed ones still. The theme,
// the text size, the language, the whole-folder view and the keyboard help are
// reached elsewhere, so none of their hooks may come back to the row; the
// furigana switch and the bookmark are drawn only when the page says so, and
// stand before the search entry and the way to the choices, so a page that
// draws neither moves nothing else.
func TestHeaderCarriesOnlyWhatReadingNeeds(t *testing.T) {
	t.Parallel()
	mark := &MarkOffer{Path: "a.md", Identity: "abc", Endpoint: "/marks"}
	tests := []struct {
		name   string
		chrome Chrome
		want   []string
	}{
		{name: "a page with neither", chrome: Chrome{}, want: []string{"y-searchbtn", "y-prefslink"}},
		{name: "a page with readings", chrome: Chrome{Readings: true}, want: []string{"y-rubybtn", "y-searchbtn", "y-prefslink"}},
		{name: "a note offering a mark", chrome: Chrome{Mark: mark}, want: []string{"y-headermark", "y-searchbtn", "y-prefslink"}},
		{name: "a note with both", chrome: Chrome{Readings: true, Mark: mark}, want: []string{"y-rubybtn", "y-headermark", "y-searchbtn", "y-prefslink"}},
	}
	controls := []string{"y-rubybtn", "y-headermark", "y-searchbtn", "y-prefslink"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := header(tt.chrome).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render header: %v", err)
			}
			html := buf.String()
			var got []string
			last := -1
			for _, control := range controls {
				at := strings.Index(html, `class="`+control)
				if at < 0 {
					at = strings.Index(html, `class="y-iconbtn `+control)
				}
				if at < 0 {
					continue
				}
				if at < last {
					t.Errorf("%s stands before a control the row draws ahead of it; html = %q", control, html)
				}
				last = at
				got = append(got, control)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("header controls (-want +got):\n%s", diff)
			}
			for _, gone := range []string{"data-theme-toggle", "data-textsize-toggle", `action="/lang"`, `href="/health"`, "popovertarget", "_y-kbd-help", "type=\"checkbox\""} {
				if strings.Contains(html, gone) {
					t.Errorf("the header row still carries %q, which is reached elsewhere now; html = %q", gone, html)
				}
			}
		})
	}
}

// TestHeaderMarksTheDestinationTheReaderIsOn holds the header's link to the
// reading choices. On that page it says so, in the one way a reader who listens
// is told it as well as one who looks; on any other page it does not claim it,
// since its address carries the page that led there rather than its own.
func TestHeaderMarksTheDestinationTheReaderIsOn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		chrome      Chrome
		wantSetting bool
	}{
		{name: "the reading choices", chrome: Chrome{Path: "/preferences", ReturnTo: "/notes/a.md"}, wantSetting: true},
		{name: "the health page", chrome: Chrome{Path: "/health", ReturnTo: "/health"}},
		{name: "a note", chrome: Chrome{Path: "/notes/a.md", ReturnTo: "/notes/a.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Base(tt.chrome).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render base: %v", err)
			}
			html := buf.String()
			settingAt := strings.Index(html, `<a class="y-prefslink"`)
			if settingAt < 0 {
				t.Fatal("the header lost the way to the reading choices")
			}
			tag := html[settingAt : settingAt+strings.Index(html[settingAt:], ">")]
			if got := strings.Contains(tag, `aria-current="page"`); got != tt.wantSetting {
				t.Errorf("settings link marked current = %v, want %v: %s", got, tt.wantSetting, tag)
			}
		})
	}
}
