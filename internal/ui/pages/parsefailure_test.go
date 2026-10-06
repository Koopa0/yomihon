package pages

import (
	"encoding/json"
	"html"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

const parserMessage = "panic while parsing this note: <script>alert(1)</script> & bad token"

// parseFailureView decodes an independent diagnostic fixture. The fixture also
// compiles against the older views, whose missing cause silently loses it.
func parseFailureView(t *testing.T, text string, view any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text), view); err != nil {
		t.Fatal(err)
	}
}

func TestParseFailureNamesTheNoteContent(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var view NotFoundView
			parseFailureView(t, `{"Asked":"/notes/Poison.md","Unreadable":true,"ParsePanic":true,"Reason":"`+parserMessage+`"}`, &view)
			var out strings.Builder
			if err := NotFound(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &out); err != nil {
				t.Fatal(err)
			}
			page := out.String()
			want := "This note's text could not be parsed"
			repair := "Check this note's content in your editor."
			if lang == wording.ZhHant {
				want = "這篇筆記的文字無法解析"
				repair = "在原本的編輯器檢查這篇筆記的內容。"
			}
			for _, text := range []string{want, repair, html.EscapeString(parserMessage), "/notes/Poison.md"} {
				if !strings.Contains(page, html.EscapeString(text)) && !strings.Contains(page, text) {
					t.Errorf("caught: parse page lacks %q", text)
				}
			}
			for _, wrong := range []string{"a permission", "權限設定", "<script>alert(1)</script>"} {
				if strings.Contains(page, wrong) {
					t.Errorf("caught: parse page contains %q", wrong)
				}
			}
			if !strings.Contains(page, `<code lang="en">`+html.EscapeString(parserMessage)+`</code>`) {
				t.Error("caught: parser message is not escaped English code")
			}
		})
	}
}

func TestHealthNamesEveryBlockedCause(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, mixed := range []bool{false, true} {
			t.Run(string(lang)+map[bool]string{false: "/parse", true: "/mixed"}[mixed], func(t *testing.T) {
				t.Parallel()
				var view HealthView
				rows := `{"Path":"Poison.md","Reason":"` + parserMessage + `","ParsePanic":true}`
				if mixed {
					rows = `{"Path":"Closed.md","Reason":"permission denied"},` + rows
				}
				parseFailureView(t, `{"Blocked":[`+rows+`]}`, &view)
				var out strings.Builder
				if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &out); err != nil {
					t.Fatal(err)
				}
				page := out.String()
				want := "These files opened, but their note content could not be parsed."
				if lang == wording.ZhHant {
					want = "這些檔案已開啟，但筆記內容無法解析。"
				}
				if mixed {
					want = "Some files could not be opened; others opened but their note content could not be parsed."
					if lang == wording.ZhHant {
						want = "有些檔案打不開；另一些已開啟，但筆記內容無法解析。"
					}
				}
				if !strings.Contains(page, want) {
					t.Errorf("caught: health lacks cause-aware explanation %q", want)
				}
				if !strings.Contains(page, html.EscapeString(parserMessage)) {
					t.Error("caught: health lost escaped parser message")
				}
				if strings.Contains(page, "<script>alert(1)</script>") {
					t.Error("caught: health trusts parser HTML")
				}
				if strings.Contains(page, "These files could not be opened on this read") || strings.Contains(page, "這一次讀取時打不開這些檔案") {
					t.Error("caught: health falsely calls the whole set unopenable")
				}
				if !strings.Contains(html.UnescapeString(page), `"panic while parsing this note:`) {
					t.Error("caught: health parser message is not quoted")
				}
				if mixed && (!strings.Contains(page, "Closed.md") || !strings.Contains(page, "permission denied")) {
					t.Error("caught: mixed health lost the read failure")
				}
			})
		}
	}
}

func TestUnreadableReasonDoesNotChooseParseWording(t *testing.T) {
	t.Parallel()
	var view HealthView
	parseFailureView(t, `{"Blocked":[{"Path":"Closed.md","Reason":"panic while parsing this note: read error"}]}`, &view)
	if got := view.blockedLede(wording.En); !strings.HasPrefix(got, "These files could not be opened on this read") {
		t.Errorf("caught: reason text chose the cause: %q", got)
	}
	var absent NotFoundView
	parseFailureView(t, `{"Asked":"/notes/Absent.md","ParsePanic":true,"Reason":"irrelevant"}`, &absent)
	var out strings.Builder
	if err := NotFound(absent, layouts.Chrome{Lang: wording.En}).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Nothing is here") {
		t.Error("caught: absent file turned into a parse failure")
	}
}

// TestRetainedParseFailureNamesTheCurrentContent covers the reading article
// shared by an ordinary note and both columns of a comparison. A failed fresh
// attempt may already be recorded while its retained reading is not yet stale.
func TestRetainedParseFailureNamesTheCurrentContent(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, stale := range []bool{false, true} {
			t.Run(string(lang)+map[bool]string{false: "/retained", true: "/stale"}[stale], func(t *testing.T) {
				t.Parallel()
				var view NoteView
				flag := "false"
				if stale {
					flag = "true"
				}
				parseFailureView(t, `{"Title":"Poison","RelPath":"Poison.md","BodyHTML":"<p>last good content</p>","Stale":`+flag+`,"ParsePanic":true,"ParseReason":"`+parserMessage+`"}`, &view)
				for name, component := range map[string]templ.Component{"note": Note(view, layouts.Chrome{Lang: lang}), "compare": Compare(CompareView{A: view, B: NoteView{Title: "Good"}}, layouts.Chrome{Lang: lang})} {
					var out strings.Builder
					if err := component.Render(t.Context(), &out); err != nil {
						t.Fatal(err)
					}
					page := out.String()
					want := "The file opened, but its current note content could not be parsed."
					if lang == wording.ZhHant {
						want = "檔案已開啟，但目前的筆記內容無法解析。"
					}
					if !strings.Contains(page, want) {
						t.Errorf("caught: %s retained page lost parse diagnosis", name)
					}
					if !strings.Contains(page, `<code lang="en">`+html.EscapeString(parserMessage)+`</code>`) {
						t.Errorf("caught: %s retained page lost escaped English code parser message", name)
					}
					if strings.Contains(page, "This file could not be read this time") || strings.Contains(page, "這個檔案這一次讀不進來") {
						t.Errorf("caught: %s retained page has generic read diagnosis", name)
					}
					if !strings.Contains(page, "last good content") {
						t.Errorf("caught: %s lost retained readable body", name)
					}
				}
			})
		}
	}
}
