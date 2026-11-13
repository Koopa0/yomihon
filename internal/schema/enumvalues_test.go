package schema_test

import (
	"maps"
	"os"
	"path/filepath"
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
		Enums map[string][]string `toml:"enums"`
	}
	meta, err := toml.Decode(string(data), &raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.Enums) == 0 {
		t.Fatal("fixture declares no enums; the completeness claim would be vacuous")
	}
	// Every enum the fixture writes, plus the type list its tables make.
	want := map[string][]string{}
	maps.Copy(want, raw.Enums)
	if _, written := raw.Enums["type"]; written {
		t.Fatal("fixture writes enums.type; this test reads the type list off the tables alone")
	}
	for _, key := range meta.Keys() {
		if len(key) == 2 && key[0] == "types" {
			want["type"] = append(want["type"], key[1])
		}
	}

	contract := loadFixture(t)
	for field, values := range want {
		got := contract.EnumValues(field)
		if diff := cmp.Diff(values, got); diff != "" {
			t.Errorf("EnumValues(%q) (-want +got):\n%s", field, diff)
		}
		if len(got) > 0 {
			got[0] = "changed"
		}
		if diff := cmp.Diff(values, contract.EnumValues(field)); diff != "" {
			t.Errorf("EnumValues(%q) changed after caller mutation (-want +got):\n%s", field, diff)
		}
	}
	for _, enum := range contract.Definition().Enums {
		if _, ok := want[enum.Field]; !ok {
			t.Errorf("Definition() carries enum %q the fixture does not write", enum.Field)
		}
	}
}

func TestEnumValuesAuthorityAndShape(t *testing.T) {
	t.Parallel()
	const declaration = `schema_version = "2"
[enums]
type = ["article", "cafe\u0301"]
domain = []
level = ["cafe\u0301", "Café"]
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
		name, field string
		want        []string
	}{
		{"declared types", "type", []string{"article", "café"}},
		{"empty enum", "domain", []string{}},
		{"absent enum", "lang", nil},
		{"unknown field", "unknown", nil},
		{"normalized values", "level", []string{"café", "Café"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, contract.EnumValues(tc.field)); diff != "" {
				t.Errorf("EnumValues(%q) (-want +got):\n%s", tc.field, diff)
			}
		})
	}
	for _, absent := range []*schema.Contract{nil, new(schema.Contract)} {
		for _, field := range []string{"type", "domain", "level", "unknown"} {
			if got := absent.EnumValues(field); got != nil {
				t.Errorf("ungoverned EnumValues(%q) = %q, want nil", field, got)
			}
		}
	}
}
