package schema_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestEnumValuesCompleteDeclaration(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Enums  map[string]any `toml:"enums"`
		Fields struct {
			StatusGroup map[string][]string `toml:"status_group"`
		} `toml:"fields"`
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatal(err)
	}
	contract := loadFixture(t)
	declared := make(map[string]bool)
	for _, field := range reflect.VisibleFields(reflect.TypeFor[schema.Enums]()) {
		if !field.IsExported() {
			continue
		}
		name := field.Tag.Get("toml")
		declared[name] = true
		value, ok := raw.Enums[name]
		if !ok {
			t.Fatalf("fixture lacks enum declaration %q", name)
		}
		if name == "status" {
			continue
		}
		if field.Type != reflect.TypeFor[[]string]() {
			t.Fatalf("enum declaration %q is not a flat vocabulary", name)
		}
		want := enumDeclarationStrings(t, value)
		for _, kind := range []string{"", "lesson", "unknown"} {
			got := contract.EnumValues(name, kind)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("EnumValues(%q, %q) (-want +got):\n%s", name, kind, diff)
			}
			if len(got) > 0 {
				got[0] = "changed"
			}
			got = append(got, "appended")
			if len(got) != len(want)+1 {
				t.Fatal("caller append did not extend its vocabulary")
			}
			if diff := cmp.Diff(want, contract.EnumValues(name, kind)); diff != "" {
				t.Errorf("EnumValues(%q, %q) changed after caller mutation (-want +got):\n%s", name, kind, diff)
			}
		}
	}
	for name := range raw.Enums {
		if !declared[name] {
			t.Errorf("fixture enum %q has no declaration", name)
		}
	}
	groups, ok := raw.Enums["status"].(map[string]any)
	if !ok {
		t.Fatal("fixture status is not a grouped vocabulary")
	}
	exercised := make(map[string]bool)
	kinds := append(enumDeclarationStrings(t, raw.Enums["type"]), "", "unknown")
	for _, kind := range kinds {
		group := "note"
		for candidate, members := range raw.Fields.StatusGroup {
			for _, member := range members {
				if member == kind {
					group = candidate
				}
			}
		}
		value, ok := groups[group]
		if !ok {
			t.Fatalf("fixture type %q names missing group %q", kind, group)
		}
		exercised[group] = true
		want := enumDeclarationStrings(t, value)
		got := contract.EnumValues("status", kind)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("EnumValues(status, %q) (-want +got):\n%s", kind, diff)
		}
		if len(got) > 0 {
			got[0] = "changed"
		}
		got = append(got, "appended")
		if len(got) != len(want)+1 {
			t.Fatal("caller append did not extend its vocabulary")
		}
		if diff := cmp.Diff(want, contract.EnumValues("status", kind)); diff != "" {
			t.Errorf("EnumValues(status, %q) changed after caller mutation (-want +got):\n%s", kind, diff)
		}
	}
	for group := range groups {
		if !exercised[group] {
			t.Errorf("fixture status group %q was not exercised", group)
		}
	}
}

func enumDeclarationStrings(t *testing.T, value any) []string {
	t.Helper()
	values, ok := value.([]any)
	if !ok {
		t.Fatalf("fixture vocabulary has type %T, want []any", value)
	}
	out := make([]string, len(values))
	for i, value := range values {
		word, ok := value.(string)
		if !ok {
			t.Fatalf("fixture vocabulary member has type %T, want string", value)
		}
		out[i] = word
	}
	return out
}

func TestEnumValuesAuthorityAndShape(t *testing.T) {
	t.Parallel()
	const declaration = `schema_version = "1"
[enums]
type = ["article", "empty", "cafe\u0301"]
domain = []
source_kind = ["cafe\u0301", "Café"]
[enums.status]
note = ["cafe\u0301", "Café"]
empty = []
custom = ["custom-word"]
[fields.status_group]
empty = ["empty"]
custom = ["cafe\u0301"]
[rules]
slug_pattern = "^[a-z]+$"
[[lifecycle]]
status = "cafe\u0301"
applies_to = ["article"]
from = []
owner = []
`
	path := filepath.Join(t.TempDir(), "contract.toml")
	if err := os.WriteFile(path, []byte(declaration), 0o600); err != nil {
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, field, kind string
		want              []string
	}{
		{"empty flat", "domain", "article", []string{}},
		{"absent flat", "level", "article", nil},
		{"unknown field", "unknown", "article", nil},
		{"empty status", "status", "empty", []string{}},
		{"default status", "status", "article", []string{"café", "Café"}},
		{"unknown type", "status", "unknown", []string{"café", "Café"}},
		{"empty type", "status", "", []string{"café", "Café"}},
		{"normalized type", "status", "cafe\u0301", []string{"custom-word"}},
		{"distinct case type", "status", "CAFÉ", []string{"café", "Café"}},
		{"normalized flat", "source_kind", "article", []string{"café", "Café"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, contract.EnumValues(tc.field, tc.kind)); diff != "" {
				t.Errorf("EnumValues(%q, %q) (-want +got):\n%s", tc.field, tc.kind, diff)
			}
		})
	}
	for _, absent := range []*schema.Contract{nil, new(schema.Contract)} {
		for _, field := range []string{"type", "status", "domain", "unknown"} {
			if got := absent.EnumValues(field, "article"); got != nil {
				t.Errorf("ungoverned EnumValues(%q, article) = %q, want nil", field, got)
			}
		}
	}
	if contract.StatusGroup("unknown") != "" || contract.Statuses("unknown") != nil {
		t.Error("judging fallback granted unknown type lifecycle authority")
	}
}
