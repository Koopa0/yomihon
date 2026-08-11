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

// TestStatusRecoveryHeadlineNeverHidesAChangedFile holds the one thing this
// page cannot get wrong. A refusal may name the operator's next move as the
// title, which reads better than restating the outcome — but a failure that
// already replaced the note's bytes has to keep saying so, and a headline
// supplied for the refusal it was written for must not travel onto that page
// and bury it.
func TestStatusRecoveryHeadlineNeverHidesAChangedFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		changed bool
		want    string
	}{
		{name: "refused before the write", want: "先處理這篇筆記的其他修改"},
		{name: "failed after the write", changed: true, want: "狀態已寫入，需要手動收尾"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := StatusRecoveryView{
				Changed:    tt.changed,
				Headline:   "先處理這篇筆記的其他修改",
				Summary:    "摘要",
				NextAction: "下一步內容",
			}
			if got := view.Title(); got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
			html := renderRecovery(t, &view)
			if !strings.Contains(html, `<h1 id="recovery-title" class="y-title">`+tt.want+`</h1>`) {
				t.Errorf("recovery heading is not %q; html = %q", tt.want, html)
			}
		})
	}
}

// TestStatusRecoveryFallsBackToTheOutcomeTitle keeps the headline optional: a
// refusal that supplies none still gets a title, and it is the one the page had
// before any refusal carried its own.
func TestStatusRecoveryFallsBackToTheOutcomeTitle(t *testing.T) {
	t.Parallel()
	view := StatusRecoveryView{Summary: "摘要", NextAction: "下一步內容"}
	if got := view.Title(); got != "狀態尚未變更" {
		t.Errorf("Title() with no headline = %q, want %q", got, "狀態尚未變更")
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
