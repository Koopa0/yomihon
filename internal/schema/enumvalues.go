package schema

import "slices"

// EnumValues returns a detached, ordered vocabulary for a frontmatter field:
// the declared types for "type", the values a contract wrote under [enums] for
// any other field. A field with no vocabulary, or an ungoverned contract,
// returns nil; a declared empty list keeps its empty shape.
func (c *Contract) EnumValues(field string) []string {
	if c == nil || c.version == "" {
		return nil
	}
	if field == FieldType {
		names := make([]string, len(c.types))
		for i, def := range c.types {
			names[i] = def.name
		}
		return names
	}
	for _, enum := range c.enums {
		if enum.field == field {
			return slices.Clone(enum.values)
		}
	}
	return nil
}
