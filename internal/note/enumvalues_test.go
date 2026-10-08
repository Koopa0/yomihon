package note_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEnumGuidanceKeepsTheJudgedStatusGroup protects schema advice for scalar
// type spellings without relaxing the reading model's string-only metadata.
func TestEnumGuidanceKeepsTheJudgedStatusGroup(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pair := range [][2]string{
		{`"study-path", "topic-map"]`, `"study-path", "topic-map", "1", "true"]`},
		{`system = ["system", "template", "guide"]`, `system = ["system", "template", "guide", "1", "true"]`},
	} {
		if strings.Count(source, pair[0]) != 1 {
			t.Fatalf("scalar fixture needle %q is not unique", pair[0])
		}
		source = strings.Replace(source, pair[0], pair[1], 1)
	}
	var raw map[string]any
	if _, decodeErr := toml.Decode(source, &raw); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	enums := enumWholeTable(t, raw["enums"], "enums")
	types := enumWholeStrings(t, enums["type"], "enums.type")
	groups := enumWholeTable(t, enumWholeTable(t, raw["fields"], "fields")["status_group"], "fields.status_group")
	systemTypes := enumWholeStrings(t, groups["system"], "fields.status_group.system")
	for _, kind := range []string{"1", "true"} {
		for _, members := range [][]string{types, systemTypes} {
			count := 0
			for _, member := range members {
				if member == kind {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("scalar fixture type %q membership count = %d, want 1 in %q", kind, count, members)
			}
		}
	}
	wantValues := enumWholeStrings(t, enumWholeTable(t, enums["status"], "enums.status")["system"], "enums.status.system")
	if !slices.Equal(wantValues, []string{"active", "archived"}) || slices.Contains(wantValues, "draft") {
		t.Fatalf("scalar fixture system vocabulary = %q, want [active archived] excluding draft", wantValues)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(source), 0o600); writeErr != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatal(writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, kind, authored, readingType string }{
		{name: "numeric-bare", kind: "1", authored: "1"},
		{name: "boolean-bare", kind: "true", authored: "true"},
		{name: "numeric-quoted", kind: "1", authored: "'1'", readingType: "1"},
		{name: "boolean-quoted", kind: "true", authored: "'true'", readingType: "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := "bare"
			if tc.readingType != "" {
				form = "quoted"
			}
			body := func(status string) string {
				return "---\ntitle: Enum scalar\ntype: " + tc.authored + "\nstatus: '" + status + "'\n---\n\n# Body\n"
			}
			invalid := body("draft")
			findings, lintErr := judge.LintFrontmatter("Writing/Enum.md", []byte(invalid), contract)
			if lintErr != nil {
				t.Fatal(lintErr)
			}
			t.Logf("hit: scalar-enum type=%s form=%s judge=returned", tc.kind, form)
			want := []judge.Finding{{
				RuleID:          "schema.enum",
				Severity:        judge.SeverityError,
				Path:            "Writing/Enum.md",
				Field:           new("status"),
				Message:         `status "draft" is not a valid system status`,
				Evidence:        "frontmatter validated against vault-schema.toml",
				SuggestedAction: "fix the frontmatter to match the schema",
				SourceRule:      "vault-schema.toml",
				Target:          new("draft"),
				Fingerprint:     "v1:ff090e61331c4f15",
			}}
			if diff := cmp.Diff(want, findings); diff != "" {
				t.Fatalf("caught: scalar-enum type=%s form=%s judge=finding (-want +got):\n%s", tc.kind, form, diff)
			}
			root := writeOneNote(t, "Writing/Enum.md", invalid)
			store, _ := newSnapshotStore(t, root, slog.New(slog.DiscardHandler), contract, contract.Governance())
			reading, found := store.Current().Capture().Note("Writing/Enum.md")
			if !found {
				t.Fatal("scalar fixture absent from captured reading")
			}
			if reading.Type != tc.readingType {
				t.Fatalf("caught: scalar-enum type=%s form=%s Reading.Type=%q want=%q", tc.kind, form, reading.Type, tc.readingType)
			}
			enumScalarSurfaces(t, invalid, contract, tc.kind+" form="+form, wantValues, true)
			for _, value := range wantValues {
				t.Run("accepted-"+value, func(t *testing.T) {
					accepted := body(value)
					got, acceptedErr := judge.LintFrontmatter("Writing/Enum.md", []byte(accepted), contract)
					if acceptedErr != nil {
						t.Fatal(acceptedErr)
					}
					t.Logf("hit: scalar-enum type=%s form=%s value=%s judge=returned", tc.kind, form, value)
					for _, finding := range got {
						if finding.RuleID == "schema.enum" {
							t.Fatalf("caught: scalar-enum type=%s form=%s value=%s judge=rejected %+v", tc.kind, form, value, finding)
						}
					}
					enumScalarSurfaces(t, accepted, contract, tc.kind+" form="+form, nil, false)
				})
			}
		})
	}
}

// TestEnumGuidanceWholeInventory keeps expected words independent of the
// Contract's detached Definition and the vocabulary lookup used by the UI.
func TestEnumGuidanceWholeInventory(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", custom), func(t *testing.T) {
			contract, inventory := enumWholeContract(t, custom)
			for _, kind := range append(slices.Clone(inventory.flat["type"]), "invalid-type") {
				group := inventory.groupFor(kind)
				for _, field := range inventory.fields {
					if field == "type" && kind != "invalid-type" {
						continue
					}
					if field != "type" && field != "status" && (group == schema.SystemDocumentGroup || kind == "invalid-type") {
						continue
					}
					want := inventory.flat[field]
					if field == "status" {
						want = inventory.groups[group]
					}
					invalid := "invalid-" + field
					if slices.Contains(want, invalid) {
						t.Fatalf("fixture declares invalid stimulus %q", invalid)
					}
					t.Run(field+"/"+kind+"/invalid", func(t *testing.T) {
						body := enumWholeBody(kind, field, invalid)
						findings := enumWholeJudge(t, body, contract, field, kind)
						targets := 0
						for _, finding := range findings {
							if finding.RuleID == "schema.enum" && finding.Field != nil && *finding.Field == field && finding.Target != nil && *finding.Target == invalid {
								targets++
							}
						}
						if targets != 1 {
							t.Fatalf("caught: enum-guidance field=%s type=%s judge=missing-target got=%d want=1", field, kind, targets)
						}
						enumWholeSurfaces(t, body, contract, field, kind, want, true)
					})
					for index, value := range want {
						acceptedType := kind
						if field == "type" {
							acceptedType = value
						}
						t.Run(fmt.Sprintf("%s/%s/accepted-%d", field, acceptedType, index), func(t *testing.T) {
							body := enumWholeBody(acceptedType, field, value)
							findings := enumWholeJudge(t, body, contract, field, acceptedType)
							t.Logf("hit: enum-membership field=%s type=%s value=%s judge=returned", field, acceptedType, value)
							enumWholeAccepted(t, findings, field, acceptedType, value, acceptedType == "invalid-type")
							enumWholeSurfaces(t, body, contract, field, acceptedType, nil, false)
						})
					}
				}
				if group == schema.SystemDocumentGroup {
					t.Run("system-early-return/"+kind, func(t *testing.T) {
						body := enumWholeBody(kind, "domain", "invalid-optional")
						for _, field := range inventory.fields {
							if field != "type" && field != "status" && field != "domain" {
								body = strings.Replace(body, "\n---\n", "\n"+field+": 'invalid-optional'\n---\n", 1)
							}
						}
						findings := enumWholeJudge(t, body, contract, "optional-fields", kind)
						for _, finding := range findings {
							if finding.RuleID == "schema.frontmatter" {
								t.Fatalf("system optional stimulus failed to parse: %+v", finding)
							}
						}
						enumWholeAccepted(t, findings, "optional-fields", kind, "invalid-optional", false)
					})
				}
			}
		})
	}
}

type enumWholeInventory struct {
	fields []string
	flat   map[string][]string
	groups map[string][]string
	types  map[string]string
}

func (iv enumWholeInventory) groupFor(kind string) string {
	if group, ok := iv.types[kind]; ok {
		return group
	}
	return "note"
}

func enumWholeContract(t *testing.T, custom bool) (*schema.Contract, enumWholeInventory) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if _, decodeErr := toml.Decode(string(data), &raw); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	enums := enumWholeTable(t, raw["enums"], "enums")
	inventory := enumWholeInventory{flat: make(map[string][]string), groups: make(map[string][]string), types: make(map[string]string)}
	seen := make(map[string]bool)
	for _, field := range reflect.VisibleFields(reflect.TypeFor[schema.Enums]()) {
		if !field.IsExported() {
			t.Fatalf("fixture inventory cannot cover unexported enum field %s", field.Name)
		}
		tag := field.Tag.Get("toml")
		value, ok := enums[tag]
		if tag == "" || seen[tag] || !ok {
			t.Fatalf("enum tag %q is blank, duplicate, or absent from raw fixture", tag)
		}
		seen[tag] = true
		inventory.fields = append(inventory.fields, tag)
		if field.Type == reflect.TypeFor[[]string]() {
			inventory.flat[tag] = enumWholeStrings(t, value, "enums."+tag)
			continue
		}
		if field.Type != reflect.TypeFor[map[string][]string]() || tag != "status" {
			t.Fatalf("enum tag %q has unsupported declaration shape %s", tag, field.Type)
		}
		for group, values := range enumWholeTable(t, value, "enums.status") {
			inventory.groups[group] = enumWholeStrings(t, values, "enums.status."+group)
		}
	}
	if len(seen) != len(enums) {
		t.Fatalf("raw enum keys = %v, exported tag inventory = %v", enums, seen)
	}
	if custom {
		data = enumWholeCustomBytes(t, data, inventory)
		raw = nil
		if _, customDecodeErr := toml.Decode(string(data), &raw); customDecodeErr != nil {
			t.Fatal(customDecodeErr)
		}
		enums = enumWholeTable(t, raw["enums"], "enums")
		for key := range inventory.flat {
			inventory.flat[key] = enumWholeStrings(t, enums[key], "enums."+key)
		}
		for key, values := range enumWholeTable(t, enums["status"], "enums.status") {
			inventory.groups[key] = enumWholeStrings(t, values, "enums.status."+key)
		}
	}
	fields := enumWholeTable(t, raw["fields"], "fields")
	for group, members := range enumWholeTable(t, fields["status_group"], "fields.status_group") {
		if _, ok := inventory.groups[group]; !ok {
			t.Fatalf("type mapping references undeclared status group %q", group)
		}
		for _, kind := range enumWholeStrings(t, members, "fields.status_group."+group) {
			if !slices.Contains(inventory.flat["type"], kind) || inventory.types[kind] != "" {
				t.Fatalf("mapped type %q is undeclared or assigned more than once", kind)
			}
			inventory.types[kind] = group
		}
	}
	covered := make(map[string]bool)
	for _, kind := range inventory.flat["type"] {
		covered[inventory.groupFor(kind)] = true
	}
	for group := range inventory.groups {
		if !covered[group] {
			t.Fatalf("declared status group %q has no exercised declared type", group)
		}
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

// enumWholeCustomBytes extends every declared list without editing its fixture.
func enumWholeCustomBytes(t *testing.T, data []byte, inventory enumWholeInventory) []byte {
	t.Helper()
	lines := strings.Split(string(data), "\n")
	section := ""
	replaced := make(map[string]int)
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = trimmed
			continue
		}
		key, _, found := strings.Cut(trimmed, "=")
		key = strings.TrimSpace(key)
		if !found || (section != "[enums]" && section != "[enums.status]") {
			continue
		}
		values := inventory.flat[key]
		if section == "[enums.status]" {
			values = inventory.groups[key]
		}
		if values == nil {
			t.Fatalf("unknown custom enum declaration %s/%s", section, key)
		}
		extra := "<b>literal</b>-" + key
		if key == "type" {
			extra = "extra-kind"
		} else if section == "[enums.status]" {
			extra = "extra-status-" + key
		}
		values = append(slices.Clone(values), extra)
		quoted := make([]string, len(values))
		for i, word := range values {
			quoted[i] = strconv.Quote(word)
		}
		lines[index] = key + " = [" + strings.Join(quoted, ", ") + "]"
		replaced[section+key]++
	}
	for key := range inventory.flat {
		if replaced["[enums]"+key] != 1 {
			t.Fatalf("custom flat enum %s replacement count = %d, want 1", key, replaced["[enums]"+key])
		}
	}
	for key := range inventory.groups {
		if replaced["[enums.status]"+key] != 1 {
			t.Fatalf("custom status enum %s replacement count = %d, want 1", key, replaced["[enums.status]"+key])
		}
	}
	return []byte(strings.Join(lines, "\n"))
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

func enumWholeBody(kind, field, value string) string {
	values := map[string]string{"title": "Enum guidance", "type": kind, "domain": "golang", "status": "archived", "slug": "enum-guidance", "created": "2026-06-01", "updated": "2026-06-01", "based_on": "[[Enum]]"}
	if field != "" {
		values[field] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var body strings.Builder
	body.WriteString("---\n")
	for _, key := range keys {
		body.WriteString(key + ": '" + strings.ReplaceAll(values[key], "'", "''") + "'\n")
	}
	body.WriteString("---\n\n# Body\n")
	return body.String()
}

func enumWholeJudge(t *testing.T, body string, contract *schema.Contract, field, kind string) []judge.Finding {
	t.Helper()
	findings, err := judge.LintFrontmatter("Writing/Enum.md", []byte(body), contract)
	if err != nil {
		t.Fatal(err)
	}
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

func enumWholeSurfaces(t *testing.T, body string, contract *schema.Contract, field, kind string, want []string, invalid bool) {
	t.Helper()
	enumReadingSurfaces(t, body, contract, field, kind, want, invalid, "enum-guidance")
}

func enumScalarSurfaces(t *testing.T, body string, contract *schema.Contract, kind string, want []string, invalid bool) {
	t.Helper()
	enumReadingSurfaces(t, body, contract, "status", kind, want, invalid, "scalar-enum")
}

func enumReadingSurfaces(t *testing.T, body string, contract *schema.Contract, field, kind string, want []string, invalid bool, marker string) {
	t.Helper()
	srv := newServerWithContract(t, writeOneNote(t, "Writing/Enum.md", body), contract)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, sink := range []string{"note", "health"} {
			path := "/notes/Writing/Enum.md"
			if sink == "health" {
				path = "/health"
			}
			page := enumGuidancePage(t, srv.Client(), srv.URL+path, lang)
			t.Logf("hit: "+marker+" field=%s type=%s sink=%s lang=%s", field, kind, sink, lang)
			var nodes []*html.Node
			if sink == "note" {
				nodes = enumElements(page, func(n *html.Node) bool { return enumAttribute(n, "id") == "_y-schema-notices" })
			} else {
				nodes = enumElements(page, func(n *html.Node) bool {
					return n.Data == "tr" && len(enumElements(n, func(link *html.Node) bool {
						return link.Data == "a" && enumAttribute(link, "href") == "/notes/Writing/Enum.md"
					})) == 1 && len(enumElements(n, func(label *html.Node) bool {
						return enumHasClass(label, "y-findings__kind") && enumText(label) == wording.HealthSchemaTitle.In(lang)
					})) == 1
				})
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
						page := enumGuidancePage(t, srv.Client(), srv.URL+"/notes/"+rel, lang)
						t.Logf("hit: enum-guidance field=%s type=%s sink=note lang=%s", tc.field, tc.kind, lang)
						notices := enumElements(page, func(n *html.Node) bool { return enumAttribute(n, "id") == "_y-schema-notices" })
						if len(notices) != 1 {
							t.Fatalf("schema notice blocks = %d, want 1", len(notices))
						}
						assertEnumList(t, notices[0], tc.field, want, lang)
						if tc.field == "status" && contract.DeclaresType(tc.kind) {
							panels := enumElements(page, func(n *html.Node) bool {
								return enumHasClass(n, "y-statusflag") && strings.Contains(enumText(n), tc.value) && !enumHasAncestor(n, "id", "_y-schema-notices")
							})
							if len(panels) != 2 {
								t.Fatalf("wide/narrow status flags = %d, want 2", len(panels))
							}
							for _, panel := range panels {
								assertEnumList(t, panel, tc.field, want, lang)
							}
						}
						health := enumGuidancePage(t, srv.Client(), srv.URL+"/health", lang)
						t.Logf("hit: enum-guidance field=%s type=%s sink=health lang=%s", tc.field, tc.kind, lang)
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
