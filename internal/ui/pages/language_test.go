package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
)

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
				return fileInfo(FileView{Title: "blob", Size: 2, ContentType: "application/octet-stream"}).Render(t.Context(), buf)
			},
			want:      []string{"沒有可呈現此檔案的閱讀器", "2 位元組", "開啟原始位元組"},
			forbidden: []string{"no reader here", "2 bytes", "Open raw bytes"},
		},
		{
			name: "search",
			render: func(buf *bytes.Buffer) error {
				return SearchResults("needle", nil, "technical diagnostic", true, nil).Render(t.Context(), buf)
			},
			want:      []string{"中介資料搜尋目前無法使用", "vault schema 的治理資料無法使用", `lang="en"`},
			forbidden: []string{"Metadata search unavailable", "Search is temporarily unavailable"},
		},
		{
			name: "study path warning",
			render: func(buf *bytes.Buffer) error {
				return entryRow(PathEntryView{Text: "Missing", Kind: nav.EntryAmbiguous}).Render(t.Context(), buf)
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
				if err := statusPanel(view).Render(t.Context(), buf); err != nil {
					return err
				}
				return diagnostics(view).Render(t.Context(), buf)
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
	if err := SlotMachine(&lesson.Sidecar{}, "response-nonce").Render(t.Context(), &buf); err != nil {
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
	if err := SlotMachine(view, "response-nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render slot machine: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<script nonce="response-nonce" type="application/json" class="y-slotdata">`) {
		t.Errorf("slot data has no response nonce: %q", html)
	}
}
