package preference_test

import (
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// A stored choice must be named where the page says what the browser keeps.
// Deriving the whole cookie set makes a newly stored choice demand its words
// here too; looking only for today's missing choice would permit the next one
// to disappear from the same account.
func TestStorageAccountNamesEveryStoredChoice(t *testing.T) {
	t.Parallel()
	legends := map[string][2]string{
		"yomihon_lang":      {"介面語言", "language"},
		"yomihon_theme":     {"外觀", "appearance"},
		"yomihon_textsize":  {"字級", "text size"},
		"yomihon_font":      {"字體", "typeface"},
		"yomihon_ruby":      {"顯示讀音", "readings"},
		"yomihon_shortcuts": {"單鍵快捷鍵", "single-key shortcuts"},
		"yomihon_rail":      {"側欄", "sidebar"},
	}
	kept := []string{wording.CookieName}
	for _, p := range layouts.Preferences() {
		kept = append(kept, p.Cookie)
	}
	slices.Sort(kept)
	t.Logf("invoked: complete stored cookie catalog %q", kept)
	if diff := cmp.Diff(slices.Sorted(maps.Keys(legends)), kept); diff != "" {
		t.Fatalf("caught: storage cookie catalog differs (-account +stored):\n%s", diff)
	}

	cases := []struct {
		name      string
		lang      wording.Lang
		column    int
		prefix    string
		separator string
		session   string
		note      string
	}{
		{
			name: "chinese", lang: wording.ZhHant, column: 0,
			prefix: "cookie 記著：", separator: "、",
			session: "分頁內的值：側欄各區的展開狀態、側欄篩選字",
			note:    "設定存成這個位址上的 cookie，側欄各區的展開狀態與篩選字只活到分頁關閉；沒有任何一項離開這台機器。",
		},
		{
			name: "english", lang: wording.En, column: 1,
			prefix: "Cookies: ", separator: ", ",
			session: "Per-tab values: sidebar section disclosure, sidebar filter text",
			note:    "Settings are cookies on this address; sidebar section disclosure and filter text die with the tab; none of it leaves this machine.",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t)
			status, body := page(t, srv, "/preferences", "yomihon_lang="+string(tt.lang), "yomihon_rail=collapsed")
			if status != http.StatusOK {
				t.Fatalf("GET /preferences status = %d, want %d", status, http.StatusOK)
			}
			lists, notes := storageAccount(t, body)
			t.Logf("invoked: committed storage account %s", tt.lang)
			if len(lists) != 2 || len(notes) != 1 || !strings.HasPrefix(lists[0], tt.prefix) {
				t.Fatalf("caught: storage account shape = lists %q, notes %q, want two lists and one note with prefix %q", lists, notes, tt.prefix)
			}
			got := strings.Split(strings.TrimPrefix(lists[0], tt.prefix), tt.separator)
			slices.Sort(got)
			want := make([]string, 0, len(kept))
			for _, cookie := range kept {
				want = append(want, legends[cookie][tt.column])
			}
			slices.Sort(want)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: storage account omits or invents cookie legends (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]string{tt.session, tt.note}, []string{lists[1], notes[0]}); diff != "" {
				t.Errorf("caught: per-tab account conflates section disclosure with the stored sidebar choice (-want +got):\n%s", diff)
			}
		})
	}
}

func storageAccount(t *testing.T, body string) (lists, notes []string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse preferences response: %v", err)
	}
	var footers []*html.Node
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || n.Data != "footer" {
			continue
		}
		for _, attr := range n.Attr {
			if attr.Key == "class" && slices.Contains(strings.Fields(attr.Val), "y-prefs__storage") {
				footers = append(footers, n)
			}
		}
	}
	if len(footers) != 1 {
		t.Fatalf("storage footers = %d, want exactly one", len(footers))
	}
	for n := range footers[0].Descendants() {
		if n.Type != html.ElementNode || (n.Data != "li" && n.Data != "p") {
			continue
		}
		var text strings.Builder
		for child := range n.Descendants() {
			if child.Type == html.TextNode {
				text.WriteString(child.Data)
			}
		}
		if n.Data == "li" {
			lists = append(lists, text.String())
		} else {
			notes = append(notes, text.String())
		}
	}
	return lists, notes
}
