package note_test

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEnumGuidanceWholeInventory keeps expected words independent of the
// Contract's detached Definition and the vocabulary lookup used by the UI.
func TestEnumGuidanceWholeInventory(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", custom), func(t *testing.T) {
			contract, inventory := enumWholeContract(t, custom)
			for _, kind := range append(slices.Clone(inventory.flat["type"]), "invalid-type") {
				for _, field := range inventory.fields {
					if field == "type" && kind != "invalid-type" {
						continue
					}
					if field != "type" && kind == "invalid-type" {
						continue
					}
					want := inventory.flat[field]
					invalid := "invalid-" + field
					if slices.Contains(want, invalid) {
						t.Fatalf("fixture declares invalid stimulus %q", invalid)
					}
					t.Run(field+"/"+kind+"/invalid", func(t *testing.T) {
						rel, body := enumWholeNote(kind, field, invalid)
						findings := enumWholeJudge(t, rel, body, contract, field, kind)
						targets := 0
						for _, finding := range findings {
							if finding.RuleID == "schema.enum" && finding.Field != nil && *finding.Field == field && finding.Target != nil && *finding.Target == invalid {
								targets++
							}
						}
						if targets != 1 {
							t.Fatalf("caught: enum-guidance field=%s type=%s judge=missing-target got=%d want=1", field, kind, targets)
						}
						enumWholeSurfaces(t, rel, body, contract, field, kind, want, true)
					})
					for index, value := range want {
						acceptedType := kind
						if field == "type" {
							acceptedType = value
						}
						t.Run(fmt.Sprintf("%s/%s/accepted-%d", field, acceptedType, index), func(t *testing.T) {
							rel, body := enumWholeNote(acceptedType, field, value)
							findings := enumWholeJudge(t, rel, body, contract, field, acceptedType)
							t.Logf("hit: enum-membership field=%s type=%s value=%s judge=returned", field, acceptedType, value)
							enumWholeAccepted(t, findings, field, acceptedType, value, acceptedType == "invalid-type")
							enumWholeSurfaces(t, rel, body, contract, field, acceptedType, nil, false)
						})
					}
				}
			}
		})
	}
}

// enumWholeInventory is every vocabulary the raw contract states: the type
// list, which is enums.type followed by each [types.*] table in the order it
// is written, and each [enums] list.
type enumWholeInventory struct {
	fields []string
	flat   map[string][]string
}

func enumWholeContract(t *testing.T, custom bool) (*schema.Contract, enumWholeInventory) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if custom {
		data = enumWholeCustomBytes(t, data)
	}
	var raw map[string]any
	meta, decodeErr := toml.Decode(string(data), &raw)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	enums := enumWholeTable(t, raw["enums"], "enums")
	inventory := enumWholeInventory{fields: []string{"type"}, flat: make(map[string][]string)}
	if declared, ok := enums["type"]; ok {
		inventory.flat["type"] = enumWholeStrings(t, declared, "enums.type")
	}
	for _, key := range meta.Keys() {
		if len(key) == 2 && key[0] == "types" && !slices.Contains(inventory.flat["type"], key[1]) {
			inventory.flat["type"] = append(inventory.flat["type"], key[1])
		}
	}
	if len(inventory.flat["type"]) == 0 {
		t.Fatal("the fixture declares no type, so the type vocabulary goes unchecked")
	}
	for _, key := range meta.Keys() {
		if len(key) != 2 || key[0] != "enums" || key[1] == "type" {
			continue
		}
		inventory.fields = append(inventory.fields, key[1])
		inventory.flat[key[1]] = enumWholeStrings(t, enums[key[1]], "enums."+key[1])
	}
	listed := len(enums)
	if _, ok := enums["type"]; ok {
		listed--
	}
	if len(inventory.fields)-1 != listed {
		t.Fatalf("raw enum keys = %v, inventory = %v", enums, inventory.fields)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatal(writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contract, inventory
}

// enumWholeCustomBytes extends every declared list, and the type list by one
// more table, without editing its fixture.
func enumWholeCustomBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	lines := strings.Split(string(data), "\n")
	section := ""
	replaced := 0
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = trimmed
			continue
		}
		key, value, found := strings.Cut(trimmed, "=")
		if !found || section != "[enums]" {
			continue
		}
		key = strings.TrimSpace(key)
		var values []string
		if _, err := toml.Decode("v = "+value, &struct {
			V *[]string `toml:"v"`
		}{V: &values}); err != nil {
			t.Fatalf("decode enums.%s: %v", key, err)
		}
		values = append(values, "<em>literal-"+key)
		quoted := make([]string, len(values))
		for i, word := range values {
			quoted[i] = strconv.Quote(word)
		}
		lines[index] = key + " = [" + strings.Join(quoted, ", ") + "]"
		replaced++
	}
	if replaced == 0 {
		t.Fatal("custom fixture extended no enum list")
	}
	return []byte(strings.Join(lines, "\n") + "\n[types.extra-kind]\n")
}

func enumWholeTable(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	table, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s raw shape = %T, want table", name, value)
	}
	return table
}

func enumWholeStrings(t *testing.T, value any, name string) []string {
	t.Helper()
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("%s raw shape = %T, want list", name, value)
	}
	words := make([]string, len(list))
	for index, item := range list {
		word, ok := item.(string)
		if !ok {
			t.Fatalf("%s[%d] raw shape = %T, want string", name, index, item)
		}
		words[index] = word
	}
	return words
}

// enumWholeNote is a note that satisfies the contract except for the one
// value under test. Its domain is read off its folder, so a domain stimulus
// moves the note rather than writing a key.
func enumWholeNote(kind, field, value string) (rel, body string) {
	values := map[string]string{"type": kind, "slug": "enum-guidance", "created": "2026-06-01", "based_on": "[[Enum]]"}
	domain := "golang"
	switch field {
	case "domain":
		domain = value
	case "":
	default:
		values[field] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range keys {
		b.WriteString(key + ": '" + strings.ReplaceAll(values[key], "'", "''") + "'\n")
	}
	b.WriteString("---\n\n# Enum guidance\n")
	return "Concepts/" + domain + "/Enum.md", b.String()
}

func enumWholeJudge(t *testing.T, rel, body string, contract *schema.Contract, field, kind string) []judge.Finding {
	t.Helper()
	findings := judge.LintFrontmatter(rel, []byte(body), contract)
	t.Logf("hit: enum-guidance field=%s type=%s judge=returned", field, kind)
	return findings
}

func enumWholeAccepted(t *testing.T, findings []judge.Finding, field, kind, value string, unknown bool) {
	t.Helper()
	typeFaults := 0
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID != "schema.enum" {
			continue
		}
		if unknown && finding.Field != nil && *finding.Field == "type" && finding.Target != nil && *finding.Target == kind {
			typeFaults++
			continue
		}
		t.Errorf("caught: enum-membership field=%s type=%s value=%s schema.enum rejected=%+v", field, kind, value, *finding)
	}
	if unknown && typeFaults != 1 {
		t.Errorf("caught: enum-guidance field=%s type=%s judge=unknown-type-stimulus got=%d want=1", field, kind, typeFaults)
	}
}

func enumWholeSurfaces(t *testing.T, rel, body string, contract *schema.Contract, field, kind string, want []string, invalid bool) {
	t.Helper()
	enumReadingSurfaces(t, rel, body, contract, field, kind, want, invalid, "enum-guidance")
}

func enumReadingSurfaces(t *testing.T, rel, body string, contract *schema.Contract, field, kind string, want []string, invalid bool, marker string) {
	t.Helper()
	srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
	href := (&url.URL{Path: "/notes/" + rel}).String()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, sink := range []string{"note", "health"} {
			path := href
			if sink == "health" {
				path = "/health"
			}
			page := enumGuidancePage(t, srv.Client(), srv.URL+path, lang)
			t.Logf("hit: "+marker+" field=%s type=%s sink=%s lang=%s", field, kind, sink, lang)
			var nodes []*html.Node
			if sink == "note" {
				nodes = enumSchemaNotices(page)
			} else {
				// The row stands in the group named for notes that do not meet
				// the schema: the kind is the group's heading, not a word in the row.
				for _, group := range enumElements(page, func(n *html.Node) bool {
					return n.Data == "section" && enumHasClass(n, "y-healthgroup") && len(enumElements(n, func(title *html.Node) bool {
						return enumHasClass(title, "y-healthgroup__title") && enumText(title) == wording.HealthSchemaTitle.In(lang)
					})) == 1
				}) {
					nodes = append(nodes, enumElements(group, func(n *html.Node) bool {
						return n.Data == "tr" && len(enumElements(n, func(link *html.Node) bool {
							return link.Data == "a" && enumAttribute(link, "href") == href
						})) == 1
					})...)
				}
			}
			if invalid && len(nodes) != 1 {
				t.Errorf("caught: "+marker+" field=%s type=%s sink=%s lang=%s target-container got=%d want=1", field, kind, sink, lang, len(nodes))
			}
			matches := 0
			for _, node := range nodes {
				advice := enumElements(node, func(n *html.Node) bool {
					return enumWholeHasAdvice(n, field, lang) && len(enumElements(n, func(child *html.Node) bool {
						return child != n && enumWholeHasAdvice(child, field, lang)
					})) == 0
				})
				for _, item := range advice {
					matches++
					if !invalid {
						continue
					}
					codes := enumElements(item, func(n *html.Node) bool { return n.Data == "code" })
					listStart := len(codes) - len(want) - 1
					if listStart < 0 || enumText(codes[listStart]) != field {
						t.Errorf("caught: "+marker+" field=%s type=%s sink=%s lang=%s allowed-values missing field code", field, kind, sink, lang)
						continue
					}
					got := make([]string, 0, len(want))
					for _, code := range codes[listStart+1:] {
						got = append(got, enumText(code))
					}
					separator, middle, end := "、", " 值：", "。"
					if lang == wording.En {
						separator, middle, end = ", ", " values: ", "."
					}
					sentence := enumGuidancePrefix(lang) + field + middle + strings.Join(want, separator) + end
					text := enumText(item)
					position := strings.Index(text, enumGuidancePrefix(lang))
					if !slices.Equal(got, want) || position < 0 || text[position:] != sentence || strings.Count(text, enumGuidancePrefix(lang)) != 1 {
						t.Errorf("caught: "+marker+" field=%s type=%s sink=%s lang=%s allowed-values got=%q text=%q want=%q sentence=%q", field, kind, sink, lang, got, text, want, sentence)
					}
				}
			}
			if (invalid && matches != 1) || (!invalid && matches != 0) {
				t.Errorf("caught: "+marker+" field=%s type=%s sink=%s lang=%s advice-count got=%d invalid=%t", field, kind, sink, lang, matches, invalid)
			}
		}
	}
}

func enumWholeHasAdvice(node *html.Node, field string, lang wording.Lang) bool {
	return node.Type == html.ElementNode && strings.Contains(enumText(node), enumGuidancePrefix(lang)) && len(enumElements(node, func(code *html.Node) bool {
		return code.Data == "code" && enumText(code) == field
	})) > 0
}

// TestEnumGuidanceReadsTheContractOnEveryReadingSurface binds the displayed
// values to the whole declaration rather than to a convenient example list.
func TestEnumGuidanceReadsTheContractOnEveryReadingSurface(t *testing.T) {
	for _, variant := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", variant), func(t *testing.T) {
			contract := enumGuidanceContract(t, variant)
			for _, tc := range []struct{ field, kind, value string }{
				{"type", "invalid-type", "invalid-type"},
				{"domain", "lesson", "invalid-domain"},
				{"level", "lesson", "invalid-level"},
			} {
				for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
					t.Run(tc.field+"/"+tc.kind+"/"+string(lang), func(t *testing.T) {
						rel, body := enumWholeNote(tc.kind, tc.field, tc.value)
						srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
						var want []string
						switch tc.field {
						case "type":
							want = []string{"inbox", "concept", "note", "system", "guide", "template", "writing", "lesson", "study-path", "moc", "source-map", "topic-map", "thought"}
							if variant {
								want = []string{"inbox", "concept", "note", "system", "guide", "template", "writing", "lesson", "study-path", "moc", "source-map", "topic-map", "extra-kind", "thought"}
							}
						case "domain":
							want = []string{"golang", "japanese", "meta"}
							if variant {
								want = append(want, "<b>literal</b>")
							}
						case "level":
							want = []string{"fundamental"}
							if variant {
								want = append(want, "<b>literal</b>")
							}
						}
						page := enumGuidancePage(t, srv.Client(), srv.URL+"/notes/"+rel, lang)
						t.Logf("hit: enum-guidance field=%s type=%s sink=note lang=%s", tc.field, tc.kind, lang)
						notices := enumSchemaNotices(page)
						if len(notices) != 1 {
							t.Fatalf("schema notice blocks = %d, want 1", len(notices))
						}
						assertEnumList(t, notices[0], tc.field, want, lang)
						health := enumGuidancePage(t, srv.Client(), srv.URL+"/health", lang)
						t.Logf("hit: enum-guidance field=%s type=%s sink=health lang=%s", tc.field, tc.kind, lang)
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
		{`domain = ["golang", "japanese", "meta"]`, `domain = ["golang", "japanese", "meta", "<b>literal</b>"]`},
		{`level = ["fundamental"]`, `level = ["fundamental", "<b>literal</b>"]`},
		{"[types.thought]\n", "[types.extra-kind]\n\n[types.thought]\n"},
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

// enumSchemaNotices finds the block of schema notices a reading page draws
// above the body: the element whose children are all notice paragraphs, other
// than the one that says a frontmatter fence never closes.
func enumSchemaNotices(page *html.Node) []*html.Node {
	return enumElements(page, func(n *html.Node) bool {
		if n.Data != "div" || n.FirstChild == nil {
			return false
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode || child.Data != "p" || !enumHasClass(child, "y-statusflag") {
				return false
			}
		}
		text := enumText(n)
		return text != wording.FrontmatterNeverCloses.In(wording.ZhHant) && text != wording.FrontmatterNeverCloses.In(wording.En)
	})
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
	rel, body := enumWholeNote("lesson", "level", "invalid-level")
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
	rel, body := enumWholeNote("lesson", "level", "fundamental")
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

// TestAnExplicitlyEmptyTypeVocabularyIsNamed distinguishes a declaration
// allowing no value from an authority that could not answer at all.
func TestAnExplicitlyEmptyTypeVocabularyIsNamed(t *testing.T) {
	const source = `schema_version = "2"
[folders]
checked = ["Writing"]
frontmatter_optional = true
[enums]
type = []
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
	const body = "---\ntype: empty-type\n---\n\n# Enum guidance\n"
	srv := newServerWithContract(t, writeOneNote(t, rel, body), contract)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, route := range []string{"/notes/" + rel, "/health"} {
			page := enumGuidancePage(t, srv.Client(), srv.URL+route, lang)
			assertEnumList(t, page, "type", []string{}, lang)
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
