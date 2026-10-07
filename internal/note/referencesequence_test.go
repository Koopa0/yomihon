package note_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReferenceSequenceNoticeKeepsTheNoteReadable(t *testing.T) {
	t.Parallel()
	for _, shape := range []struct {
		name       string
		value      string
		wantNotice bool
	}{
		{name: "unquoted", value: "[[Note]]", wantNotice: true},
		{name: "several nested items", value: "[[One], [Two]]", wantNotice: true},
		{name: "quoted reference", value: "[\"[[Note]]\"]"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			const rel = "Concepts/golang/Probe.md"
			root := writeOneNote(t, rel, "---\ntitle: Probe\ntype: concept\ndomain: golang\nstatus: seedling\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: "+shape.value+"\n---\n\nReadable reference body.\n")
			server := newServerWithContract(t, root, loadHomeContract(t))
			for _, language := range []struct {
				lang  string
				words string
			}{
				{lang: "zh-Hant", words: " 的值被 YAML 讀成巢狀清單，沒有讀成引用。請替連結加上引號，例如 "},
				{lang: "en", words: " was read as a nested YAML list, not a reference. Quote the link, for example "},
			} {
				t.Run(language.lang, func(t *testing.T) {
					t.Parallel()
					request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/"+rel, http.NoBody)
					if err != nil {
						t.Fatal(err)
					}
					request.Header.Set("Cookie", "yomihon_lang="+language.lang)
					response, err := server.Client().Do(request)
					if err != nil {
						t.Fatal(err)
					}
					data, readErr := io.ReadAll(response.Body)
					closeErr := response.Body.Close()
					if readErr != nil || closeErr != nil {
						t.Fatalf("read/close response: %v / %v", readErr, closeErr)
					}
					page := string(data)
					t.Log("hit: reference sequence note route reached")
					if response.StatusCode != http.StatusOK || !strings.Contains(page, "Readable reference body.") {
						t.Errorf("caught: reference notice lost readable body: status %d", response.StatusCode)
					}
					wantCount := 0
					if shape.wantNotice {
						wantCount = 1
					}
					if count := strings.Count(page, language.words); count != wantCount {
						t.Errorf("caught: reference notice count = %d, want %d", count, wantCount)
					}
					if shape.wantNotice && !strings.Contains(page, "<code>based_on: [&#34;[[Note]]&#34;]</code>") {
						t.Error("caught: reference quotation repair is not escaped readable code")
					}
					if strings.Contains(page, "schema.reference_nested_sequence") {
						t.Error("caught: internal rule identifier replaced localized reference notice")
					}
				})
			}
		})
	}
}
