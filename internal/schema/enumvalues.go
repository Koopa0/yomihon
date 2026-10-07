package schema

import (
	"reflect"
	"slices"
)

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
	enums := reflect.ValueOf(c.definition.Enums)
	// The declaration's tags identify flat vocabularies, so a new declaration
	// needs no second field list in the judge or its reading advice.
	for _, declaration := range reflect.VisibleFields(enums.Type()) {
		if !declaration.IsExported() || declaration.Type != reflect.TypeFor[[]string]() || declaration.Tag.Get("toml") != field {
			continue
		}
		values, _ := reflect.TypeAssert[[]string](enums.FieldByIndex(declaration.Index))
		return slices.Clone(values)
	}
	return nil
}
