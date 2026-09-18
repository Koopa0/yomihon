package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestReadingPageInterfaceBlocksDeclareTheInterfaceLanguage holds the language
// boundary inside the article of a note that declared one of its own. The
// article speaks the author's language, and each block the page adds inside it
// out of yomihon's own words — the schema findings, the file row, the sentence
// about a file that could not be re-read, the foot of the article and the
// status bar — declares the interface language again. Without that a Japanese
// note has its Chinese or English chrome announced to assistive technology as
// Japanese.
//
// The neighbour titles at the foot are the other authors' words, so they carry
// what those notes declared and nothing where they declared nothing: the reset
// above them must not hand them a language nobody chose.
//
// The run walks both languages the chrome speaks, which is what separates a
// block that follows the reader from one that merely agrees with the default.
// Each expectation is a whole opening tag, the only way to say which element is
// being asked about.
func TestReadingPageInterfaceBlocksDeclareTheInterfaceLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang wording.Lang
		tag  string
	}{
		{name: "Traditional Chinese chrome", lang: wording.ZhHant, tag: "zh-Hant"},
		{name: "English chrome", lang: wording.En, tag: "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := NoteView{
				Title:        "L01 わたしは学生です",
				RelPath:      "Writing/lessons/japanese/L01.md",
				Language:     "ja",
				Type:         "lesson",
				Status:       "draft",
				ObsidianHref: "obsidian://open?path=/vault/Writing/lessons/japanese/L01.md",
				Stale:        true,
				Updated:      "2026-07-10",
				UpdatedAt:    "2026-07-10",
				Governed:     true,
				Transitions:  []Transition{{To: "ready"}},
				SchemaNotices: [][]wording.SchemaPart{{
					{Text: "mystery_key", Code: true},
					{Text: " is not a field the schema knows."},
				}},
				Prev:        nav.NoteRef{Name: "L00 はじめに", RelPath: "Writing/lessons/japanese/L00.md", Language: "ja"},
				Next:        nav.NoteRef{Name: "L02", RelPath: "Writing/lessons/japanese/L02.md"},
				StepsLabel:  "Japanese course",
				StepsCourse: true,
			}
			var buf bytes.Buffer
			if err := Note(view, layouts.Chrome{Lang: tt.lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			for _, want := range []string{
				`<article class="y-article" lang="ja">`,
				`<div id="schema-notices" lang="` + tt.tag + `">`,
				// The head's dt is yomihon's own word for the fact and declares
				// the interface language; the dd right after it is the note's own
				// path and carries none — the literal string between them is the
				// proof, since a lang attribute sneaking onto the <a> would break
				// this exact match.
				`<dt lang="` + tt.tag + `">` + wording.RawFile.In(tt.lang) + `</dt><dd><a href="/raw/Writing/lessons/japanese/L01.md">`,
				`<a class="y-metarow__raw" lang="` + tt.tag + `" href="obsidian://open?path=/vault/Writing/lessons/japanese/L01.md">`,
				`<p class="y-fileinfo__note" lang="` + tt.tag + `" data-note-stale>`,
				`<nav class="y-steps y-steps--course" lang="` + tt.tag + `" aria-label="Japanese course">`,
				`<section class="y-sealbar" lang="` + tt.tag + `" aria-label="`,
				`<span class="y-steps__name" lang="ja">L00 はじめに</span>`,
				`<span class="y-steps__name">L02</span>`,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("the reading page is missing %q; html = %q", want, html)
				}
			}
			// The date is the one interface word in the file row with no class
			// naming it, so the row is read out and asked directly. Nothing in
			// it may still be announced in the note's language.
			at := strings.Index(html, `<details class="y-metarow"`)
			if at < 0 {
				t.Fatalf("the reading page carries no file row; html = %q", html)
			}
			row, _, closed := strings.Cut(html[at:], "</details>")
			if !closed {
				t.Fatalf("the file row never closes; html = %q", html)
			}
			if !strings.Contains(row, `<span lang="`+tt.tag+`">`) {
				t.Errorf("the file row's date does not declare the interface language %q; row = %q", tt.tag, row)
			}
			if strings.Contains(row, `lang="ja"`) {
				t.Errorf("the file row announces yomihon's own words in the note's language; row = %q", row)
			}
		})
	}
}

// TestBrowserCopyUsesTraditionalChinese keeps machine tokens independent from
// the human-facing labels, guidance, diagnostics, and accessible explanations
// rendered around them.
func TestBrowserCopyUsesTraditionalChinese(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		render    func(*bytes.Buffer) error
		want      []string
		forbidden []string
	}{
		{
			name: "file information",
			render: func(buf *bytes.Buffer) error {
				return fileInfo(FileView{Title: "blob", Size: 2, ContentType: "application/octet-stream"}, wording.ZhHant).Render(t.Context(), buf)
			},
			want:      []string{"沒有這種檔案的閱讀器", "2 位元組", "開啟原始位元組"},
			forbidden: []string{"no reader here", "2 bytes", "Open raw bytes"},
		},
		{
			name: "search",
			render: func(buf *bytes.Buffer) error {
				return SearchResults(SearchView{Query: "needle", Diagnostic: "technical diagnostic", Governed: true}, wording.ZhHant).Render(t.Context(), buf)
			},
			want:      []string{"依欄位篩選目前無法使用", "狀態與類型資料目前無法使用", `lang="en"`},
			forbidden: []string{"Metadata search unavailable", "Search is temporarily unavailable"},
		},
		{
			name: "study path warning",
			render: func(buf *bytes.Buffer) error {
				return entryRow(PathEntryView{Text: "Missing", Kind: nav.EntryAmbiguous}, wording.ZhHant).Render(t.Context(), buf)
			},
			want:      []string{`data-resolution="ambiguous"`, `title="目標有歧義"`, ">有歧義</span>"},
			forbidden: []string{`title="Target is ambiguous"`, ">ambiguous</span>"},
		},
		{
			name: "write and render diagnostics",
			render: func(buf *bytes.Buffer) error {
				view := NoteView{
					WriteDiagnostic: "technical write diagnostic",
					Diagnostic:      "frontmatter is not valid YAML: detail",
					RenderDiagnostics: []render.Diagnostic{{
						Kind:    render.DiagWikilinkBroken,
						Target:  "Missing",
						Message: "wikilink does not resolve",
					}},
				}
				// The panel is asked in the state the page draws it in: a
				// folder with a lifecycle, and a frontmatter that was read.
				// A note carrying an unread frontmatter has no status to act
				// on, so the page shows the diagnostics and no panel at all,
				// and asking for both from one note asks for a page that does
				// not exist.
				governed := view
				governed.Governed = true
				governed.Diagnostic = ""
				if err := statusPanel(governed, wording.ZhHant).Render(t.Context(), buf); err != nil {
					return err
				}
				return diagnostics(view, wording.ZhHant).Render(t.Context(), buf)
			},
			want:      []string{"生命週期寫入目前無法使用", "frontmatter 不是有效的 YAML", "這個 wikilink 或嵌入的目標尚未建立", `lang="en"`},
			forbidden: []string{"Diagnostics", "Contract unavailable", "not a governable artifact"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := tt.render(&buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered browser copy is missing %q; html = %q", want, html)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(html, forbidden) {
					t.Errorf("rendered browser copy retains legacy English %q; html = %q", forbidden, html)
				}
			}
		})
	}
}

func TestSlotMachineRestoresChromeLanguage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := SlotMachine(&lesson.Sidecar{}, "response-nonce", wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render slot machine: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<section class="y-slotmachine" lang="zh-Hant" aria-label="句型練習">`) {
		t.Errorf("slot-machine chrome does not declare Traditional Chinese: %q", html)
	}
}

func TestSlotMachineDataCarriesTheResponseNonce(t *testing.T) {
	t.Parallel()
	view := &lesson.Sidecar{Patterns: []lesson.Pattern{{
		ID:       "plain",
		Template: "plain sentence",
		GlossZH:  "普通句子",
	}}}
	var buf bytes.Buffer
	if err := SlotMachine(view, "response-nonce", wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render slot machine: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<script nonce="response-nonce" type="application/json" class="y-slotdata">`) {
		t.Errorf("slot data has no response nonce: %q", html)
	}
}
