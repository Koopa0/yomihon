package note_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
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
				{lang: "zh-Hant", words: "<code>based_on</code> 裡有一項被 YAML 讀成巢狀清單，沒有讀成引用。請替連結加上引號，例如 "},
				{lang: "en", words: "An item in <code>based_on</code> was read as a nested YAML list, not a reference. Quote the link, for example "},
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

func TestReferenceSequenceConfiguredSystemNoticeAndMixedSource(t *testing.T) {
	t.Parallel()
	for _, shape := range []struct {
		name     string
		noteType string
		status   string
		fields   string
		want     []string
	}{
		{name: "system draft mixed and replacement", noteType: "system", status: "draft", fields: "based_on: [\"[[Good]]\", [Bad]]\nlineage: [[New]]\n", want: []string{"based_on", "lineage"}},
		{name: "system archived mixed and replacement", noteType: "system", status: "archived", fields: "based_on: [\"[[Good]]\", [Bad]]\nlineage: [[New]]\n", want: []string{"based_on", "lineage"}},
		{name: "knowledge mixed", noteType: "memo", status: "draft", fields: "based_on: [\"[[Good]]\", [Bad]]\n", want: []string{"based_on"}},
		{name: "system quoted", noteType: "system", status: "draft", fields: "based_on: [\"[[Good]]\"]\nlineage: [\"[[Good]]\"]\n"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			const rel = "Concepts/Probe.md"
			root := writeNotes(t, map[string]string{
				rel:               "---\ntitle: Probe\ntype: " + shape.noteType + "\nstatus: " + shape.status + "\n" + shape.fields + "---\n\nReadable reference body.\n",
				"Sources/Good.md": "---\ntitle: Good\ntype: memo\nstatus: draft\n---\n\nThe real declared source.\n",
			})
			data, err := os.ReadFile(filepath.Join("..", "judge", "testdata", "vault-reference-sequence", "System", "schemas", "vault-schema.toml"))
			if err != nil {
				t.Fatal(err)
			}
			declaration := string(data)
			for _, replacement := range [][2]string{
				{`type = ["concept", "lesson", "memo"]`, `type = ["concept", "lesson", "memo", "system"]`},
				{"[enums.status]\n", "[enums.status]\nsystem = [\"draft\", \"archived\"]\n"},
				{"[fields.status_group]\n", "[fields.status_group]\nsystem = [\"system\"]\n"},
			} {
				if strings.Count(declaration, replacement[0]) != 1 {
					t.Fatalf("contract replacement %q has no unique declaration", replacement[0])
				}
				declaration = strings.Replace(declaration, replacement[0], replacement[1], 1)
			}
			path := filepath.Join(t.TempDir(), "contract.toml")
			if writeErr := os.WriteFile(path, []byte(declaration), 0o600); writeErr != nil { // #nosec G703 -- fixed basename below t.TempDir
				t.Fatal(writeErr)
			}
			contract, err := schema.LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			server := newServerWithContract(t, root, contract)
			for _, language := range []struct {
				lang   string
				prefix string
				words  string
			}{
				{lang: "zh-Hant", words: " 裡有一項被 YAML 讀成巢狀清單，沒有讀成引用。請替連結加上引號，例如 "},
				{lang: "en", prefix: "An item in ", words: " was read as a nested YAML list, not a reference. Quote the link, for example "},
			} {
				t.Run(language.lang, func(t *testing.T) {
					t.Parallel()
					request, requestErr := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/"+rel, http.NoBody)
					if requestErr != nil {
						t.Fatal(requestErr)
					}
					request.Header.Set("Cookie", "yomihon_lang="+language.lang)
					response, responseErr := server.Client().Do(request)
					if responseErr != nil {
						t.Fatal(responseErr)
					}
					body, readErr := io.ReadAll(response.Body)
					closeErr := response.Body.Close()
					if readErr != nil || closeErr != nil {
						t.Fatalf("read/close response: %v / %v", readErr, closeErr)
					}
					page := string(body)
					t.Log("hit: configured system reference note route reached")
					if response.StatusCode != http.StatusOK || !strings.Contains(page, "Readable reference body.") {
						t.Errorf("caught: configured reference notice lost readable body: status %d", response.StatusCode)
					}
					if !strings.Contains(page, `<html lang="`+language.lang+`"`) {
						t.Errorf("caught: configured reference notice root language is not %q", language.lang)
					}
					if count := strings.Count(page, language.words); count != len(shape.want) {
						t.Errorf("caught: configured reference notice count = %d, want %d", count, len(shape.want))
					}
					for _, field := range shape.want {
						if count := strings.Count(page, language.prefix+"<code>"+field+"</code>"+language.words); count != 1 {
							t.Errorf("caught: configured reference notice for %q count = %d, want 1", field, count)
						}
						for _, example := range []string{": [&#34;[[Note]]&#34;]</code>", ": [&#34;Note&#34;]</code>"} {
							if !strings.Contains(page, "<code>"+field+example) {
								t.Errorf("caught: configured reference repair code for %q is absent", field)
							}
						}
						if strings.Contains(page, "<code>"+field+"</code> 的值被 YAML") {
							t.Errorf("caught: configured reference notice blames the whole %q field", field)
						}
					}
					if !strings.Contains(basedOnBlock(t, page), `href="/notes/Sources/Good.md"`) {
						t.Error("caught: mixed reference notice lost the real declared source link")
					}
					if strings.Contains(page, "schema.reference_nested_sequence") {
						t.Error("caught: configured reference notice exposes internal rule identifier")
					}
				})
			}
		})
	}
}
