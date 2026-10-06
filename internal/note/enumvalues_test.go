package note_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEnumGuidanceReadsTheContractOnEveryReadingSurface binds the displayed
// values to the whole declaration rather than to a convenient example list.
func TestEnumGuidanceReadsTheContractOnEveryReadingSurface(t *testing.T) {
	for _, variant := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", variant), func(t *testing.T) {
			contract := enumGuidanceContract(t, variant)
			for _, tc := range []struct{ field, kind, value string }{
				{"status", "lesson", "invalid-status"},
				{"status", "concept", "invalid-status"},
				{"status", "guide", "invalid-status"},
				{"status", "invalid-type", "invalid-status"},
				{"type", "invalid-type", "draft"},
				{"domain", "lesson", "draft"},
			} {
				for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
					t.Run(tc.field+"/"+tc.kind+"/"+string(lang), func(t *testing.T) {
						domain := "golang"
						if tc.field == "domain" {
							domain = "invalid-domain"
						}
						body := fmt.Sprintf("---\ntitle: Enum guidance\ntype: %s\ndomain: %s\nstatus: %s\nslug: enum-guidance\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: '[[Enum]]'\n---\n\n# Body\n", tc.kind, domain, tc.value)
						const rel = "Writing/Enum.md"
						srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
						var want []string
						switch tc.field {
						case "status":
							want = []string{"captured", "cleaned", "seedling", "growing", "evergreen", "imported", "draft", "ready", "published", "archived"}
							if tc.kind == "lesson" {
								want = []string{"imported", "draft", "ready", "published", "archived"}
								if variant {
									want = append(want, "extra-status")
								}
							}
							if tc.kind == "guide" {
								want = []string{"active", "archived"}
							}
						case "type":
							want = []string{"inbox", "transcript", "source-note", "concept", "writing", "lesson", "system", "guide", "template", "moc", "source-map", "study-path", "topic-map"}
							if variant {
								want = append(want, "extra-kind")
							}
						case "domain":
							want = []string{"golang", "japanese", "meta"}
							if variant {
								want = append(want, "<b>literal</b>")
							}
						}
						t.Log("hit: real GET vocabulary consumer reached")
						page := enumGuidancePage(t, srv.Client(), srv.URL+"/notes/"+rel, lang)
						notices := enumElements(page, func(n *html.Node) bool { return enumAttribute(n, "id") == "schema-notices" })
						if len(notices) != 1 {
							t.Fatalf("schema notice blocks = %d, want 1", len(notices))
						}
						assertEnumList(t, notices[0], tc.field, want, lang)
						if tc.field == "status" && contract.DeclaresType(tc.kind) {
							panels := enumElements(page, func(n *html.Node) bool {
								return enumHasClass(n, "y-statusflag") && strings.Contains(enumText(n), tc.value) && !enumHasAncestor(n, "id", "schema-notices")
							})
							if len(panels) != 2 {
								t.Fatalf("wide/narrow status flags = %d, want 2", len(panels))
							}
							for _, panel := range panels {
								assertEnumList(t, panel, tc.field, want, lang)
							}
						}
						health := enumGuidancePage(t, srv.Client(), srv.URL+"/health", lang)
						if !contract.DeclaresType(tc.kind) {
							statusRows := enumElements(health, func(n *html.Node) bool {
								return n.Data == "tr" && strings.Contains(enumText(n), tc.value) && strings.Contains(enumText(n), tc.kind)
							})
							if len(statusRows) != 1 {
								t.Fatalf("unknown-type status rows = %d, want 1", len(statusRows))
							}
							if strings.Contains(enumText(statusRows[0]), enumGuidancePrefix(lang)) {
								t.Errorf("caught: unknown type received lifecycle status-list advice")
							}
						}

						rows := enumElements(health, func(n *html.Node) bool {
							return n.Type == html.ElementNode && n.Data == "tr" && strings.Contains(enumText(n), "Enum guidance")
						})
						if len(rows) == 0 {
							t.Fatal("health never names the faulty note")
						}
						matched := false
						for _, row := range rows {
							if strings.Contains(enumText(row), enumGuidancePrefix(lang)) {
								assertEnumList(t, row, tc.field, want, lang)
								matched = true
							}
						}
						if !matched {
							t.Errorf("caught: Health has no allowed %s values", tc.field)
						}
						if tc.field == "status" && contract.DeclaresType(tc.kind) {
							statusRows := enumElements(health, func(n *html.Node) bool {
								return n.Data == "tr" && strings.Contains(enumText(n), tc.value) && strings.Contains(enumText(n), tc.kind)
							})
							if len(statusRows) != 1 {
								t.Fatalf("status Health rows = %d, want 1", len(statusRows))
							}
							assertEnumList(t, statusRows[0], tc.field, want, lang)
						}
					})
				}
			}
		})
	}
}

func enumGuidanceContract(t *testing.T, custom bool) *schema.Contract {
	t.Helper()
	if !custom {
		return loadHomeContract(t)
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pair := range [][2]string{
		{`"study-path", "topic-map"]`, `"study-path", "topic-map", "extra-kind"]`},
		{`domain = ["golang", "japanese", "meta"]`, `domain = ["golang", "japanese", "meta", "<b>literal</b>"]`},
		{`lesson = ["imported", "draft", "ready", "published", "archived"]`, `lesson = ["imported", "draft", "ready", "published", "archived", "extra-status"]`},
	} {
		if strings.Count(source, pair[0]) != 1 {
			t.Fatalf("fixture needle %q is not unique", pair[0])
		}
		source = strings.Replace(source, pair[0], pair[1], 1)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(source), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatal(writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func enumGuidancePage(t *testing.T, client *http.Client, address string, lang wording.Lang) *html.Node {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", address, resp.StatusCode)
	}
	tree, err := html.Parse(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(enumElements(tree, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "b" && enumText(n) == "literal"
	})) > 0 {
		t.Error("caught: a contract value became HTML")
	}
	return tree
}

func assertEnumList(t *testing.T, node *html.Node, field string, want []string, lang wording.Lang) {
	t.Helper()
	if !strings.Contains(enumText(node), enumGuidancePrefix(lang)) {
		t.Errorf("caught: no allowed %s values in %q", field, enumText(node))
		return
	}
	candidates := enumElements(node, func(n *html.Node) bool {
		return n.Type == html.ElementNode && strings.Contains(enumText(n), enumGuidancePrefix(lang)) && len(enumElements(n, func(c *html.Node) bool { return c.Data == "code" && enumText(c) == field })) > 0
	})
	if len(candidates) == 0 {
		t.Errorf("caught: no allowed %s code list", field)
		return
	}
	node = candidates[len(candidates)-1]
	codes := enumElements(node, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "code" })
	var got []string
	for _, code := range codes {
		value := enumText(code)
		if value == field {
			got = nil
			continue
		}
		got = append(got, value)
	}
	if !slices.Equal(got, want) {
		t.Errorf("caught: allowed %s values = %q, want contract %q", field, got, want)
	}
}

func enumGuidancePrefix(lang wording.Lang) string {
	if lang == wording.En {
		return "Allowed "
	}
	return "允許的 "
}

func enumElements(root *html.Node, matches func(*html.Node) bool) []*html.Node {
	var found []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if matches(n) {
			found = append(found, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(root)
	return found
}

func enumText(root *html.Node) string {
	var text strings.Builder
	for _, node := range enumElements(root, func(n *html.Node) bool { return n.Type == html.TextNode }) {
		text.WriteString(node.Data)
	}
	return text.String()
}

func enumAttribute(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func enumHasClass(node *html.Node, class string) bool {
	return slices.Contains(strings.Fields(enumAttribute(node, "class")), class)
}
func enumHasAncestor(node *html.Node, key, value string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if enumAttribute(parent, key) == value {
			return true
		}
	}
	return false
}

// TestEnumGuidanceClosesAfterContractDrift prevents a retained generation from
// presenting the startup vocabulary as advice about a different contract.
func TestEnumGuidanceClosesAfterContractDrift(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatal(writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const rel = "Writing/Enum.md"
	const body = "---\ntitle: Enum guidance\ntype: lesson\ndomain: invalid-domain\nstatus: invalid-status\nslug: enum-guidance\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n# Body\n"
	srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
	for _, route := range []string{"/notes/" + rel, "/health"} {
		page := enumGuidancePage(t, srv.Client(), srv.URL+route, wording.En)
		if !strings.Contains(enumText(page), enumGuidancePrefix(wording.En)) {
			t.Fatal("open authority never supplied vocabulary")
		}
	}
	if writeErr := os.WriteFile(path, append(data, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatal(writeErr)
	}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, route := range []string{"/notes/" + rel, "/health"} {
			page := enumGuidancePage(t, srv.Client(), srv.URL+route, lang)
			if strings.Contains(enumText(page), enumGuidancePrefix(lang)) {
				t.Errorf("caught: closed authority leaked allowed values on %s", route)
			}
		}
	}
}

// TestLegalAndUngovernedNotesDoNotInventEnumFaults keeps advice specific to a
// rejected value rather than offering every vocabulary beside every note.
func TestLegalAndUngovernedNotesDoNotInventEnumFaults(t *testing.T) {
	const rel = "Writing/Enum.md"
	const body = "---\ntitle: Enum guidance\ntype: lesson\ndomain: golang\nstatus: draft\nslug: enum-guidance\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n# Body\n"
	for _, contract := range []*schema.Contract{loadHomeContract(t), nil} {
		srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			for _, route := range []string{"/notes/" + rel, "/health"} {
				page := enumGuidancePage(t, srv.Client(), srv.URL+route, lang)
				if strings.Contains(enumText(page), enumGuidancePrefix(lang)) {
					t.Errorf("caught: non-faulty note invented an enum list on %s", route)
				}
			}
		}
	}
}

// TestAnExplicitlyEmptyStatusVocabularyIsNamed distinguishes a declaration
// allowing no value from an authority that could not answer at all.
func TestAnExplicitlyEmptyStatusVocabularyIsNamed(t *testing.T) {
	const source = `schema_version = "1"
[enums]
type = ["empty-type", "doc"]
[enums.status]
note = []
doc = ["draft"]
[fields.status_group]
doc = ["doc"]
[scan]
knowledge_dirs = ["Writing"]
no_frontmatter_is_legal = true
[artifacts]
non_instance_dirs = []
[[lifecycle]]
status = "draft"
applies_to = ["doc"]
from = []
owner = ["koopa"]
`
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(source), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatal(writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const rel = "Writing/Enum.md"
	const body = "---\ntitle: Enum guidance\ntype: empty-type\nstatus: invalid-status\n---\n\n# Body\n"
	srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, route := range []string{"/notes/" + rel, "/health"} {
			page := enumGuidancePage(t, srv.Client(), srv.URL+route, lang)
			assertEnumList(t, page, "status", []string{}, lang)
			none := "無。"
			if lang == wording.En {
				none = "none."
			}
			if !strings.Contains(enumText(page), none) {
				t.Errorf("caught: explicitly empty vocabulary omitted %q on %s", none, route)
			}
		}
	}
}
