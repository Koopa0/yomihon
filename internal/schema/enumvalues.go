package schema

import (
	"reflect"
	"slices"
	"sync"
)

var enumFieldIndexes = sync.OnceValue(func() map[string][]int {
	indexes := make(map[string][]int)
	for _, field := range reflect.VisibleFields(reflect.TypeFor[Enums]()) {
		if field.IsExported() && field.Type == reflect.TypeFor[[]string]() {
			indexes[field.Tag.Get("toml")] = field.Index
		}
	}
	return indexes
})

// EnumValues returns a detached, ordered vocabulary for a frontmatter field.
// Status uses the judging group, including the general-note fallback for an
// undeclared type; this grants no lifecycle authority. Flat vocabularies do not
// depend on noteType. An unknown field or an ungoverned contract returns nil.
// Declared empty lists retain their empty shape.
func (c *Contract) EnumValues(field, noteType string) []string {
	if c == nil || c.version == "" {
		return nil
	}
	if field == "status" {
		return c.StatusesInGroup(c.JudgedStatusGroup(noteType))
	}
	index, ok := enumFieldIndexes()[field]
	if !ok {
		return nil
	}
	// Cache the declaration's tags once; read each vocabulary from this
	// contract so contracts with different values never share policy.
	enums := reflect.ValueOf(&c.definition.Enums).Elem()
	values, _ := reflect.TypeAssert[[]string](enums.FieldByIndex(index))
	return slices.Clone(values)
}
