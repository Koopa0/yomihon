package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestStatusRecoveryDistinguishesMutationState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		changed   bool
		want      []string
		forbidden []string
	}{
		{
			name:      "unchanged",
			want:      []string{"狀態尚未變更", "這次操作沒有變更筆記檔案。", `data-status-changed="false"`},
			forbidden: []string{"請勿重送"},
		},
		{
			name:      "changed",
			changed:   true,
			want:      []string{"狀態已寫入，需要手動收尾", "這次操作已變更筆記檔案；請勿重送這次操作。", `data-status-changed="true"`},
			forbidden: []string{"這次操作沒有變更"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			html := renderRecovery(t, &StatusRecoveryView{
				Changed:    tt.changed,
				Summary:    "摘要",
				NextAction: "下一步內容",
			})
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("recovery page is missing %q; html = %q", want, html)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(html, forbidden) {
					t.Errorf("recovery page unexpectedly contains %q; html = %q", forbidden, html)
				}
			}
		})
	}
}

func TestStatusRecoveryEscapesDetailAndOffersOnlySafeGETLinks(t *testing.T) {
	t.Parallel()
	html := renderRecovery(t, &StatusRecoveryView{
		Summary:         "摘要",
		NextAction:      "下一步內容",
		TechnicalDetail: `<script>alert("x")</script>`,
		NotePath:        "Writing/日本 語?#.md",
	})
	for _, want := range []string{
		`<code lang="en">&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;</code>`,
		`href="/notes/Writing/%E6%97%A5%E6%9C%AC%20%E8%AA%9E%3F%23.md"`,
		`href="/"`,
		`aria-label="復原操作"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("recovery page is missing %q; html = %q", want, html)
		}
	}
	for _, forbidden := range []string{`method="post"`, `action="/status"`, `<script>alert`} {
		if strings.Contains(html, forbidden) {
			t.Errorf("recovery page unexpectedly contains %q; html = %q", forbidden, html)
		}
	}
}

func TestStatusRecoveryWithoutPathOffersHomeOnly(t *testing.T) {
	t.Parallel()
	html := renderRecovery(t, &StatusRecoveryView{Summary: "摘要", NextAction: "下一步內容"})
	if strings.Contains(html, ">返回筆記</a>") {
		t.Errorf("recovery page offers a note link without a validated path; html = %q", html)
	}
	if !strings.Contains(html, `href="/">返回首頁</a>`) {
		t.Errorf("recovery page has no Home recovery link; html = %q", html)
	}
}

func renderRecovery(t *testing.T, view *StatusRecoveryView) string {
	t.Helper()
	var buf bytes.Buffer
	if err := StatusRecovery(*view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render recovery: %v", err)
	}
	return buf.String()
}

// TestStatusRecoveryObsidianAction locks the recovery page's editor link to
// the views that know which note they are about: a view carrying the href
// renders it among the GET actions, and a view without one — a refusal that
// never resolved a note — offers no such door.
func TestStatusRecoveryObsidianAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		href string
		want bool
	}{
		{name: "resolved note offers the editor", href: "obsidian://open?path=/vault/Notes/n.md", want: true},
		{name: "unresolved refusal offers none", href: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := StatusRecoveryView{Summary: "s", NextAction: "n", ObsidianHref: tt.href}
			var buf bytes.Buffer
			if err := StatusRecovery(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if got := strings.Contains(html, "在 Obsidian 開啟"); got != tt.want {
				t.Errorf("Obsidian action rendered = %v, want %v", got, tt.want)
			}
			if tt.want && !strings.Contains(html, `href="obsidian://open?path=/vault/Notes/n.md"`) {
				t.Errorf("recovery page does not carry the built href; html = %q", html)
			}
		})
	}
}
