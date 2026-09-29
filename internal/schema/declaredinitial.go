package schema

// DeclaresInitial reports an explicit initial = true lifecycle declaration.
// StartsAt also permits legacy contracts to infer a legal first status from
// predecessor lists; this narrower question does not assign that inferred
// state the reader's unfinished-thought meaning.
func (c *Contract) DeclaresInitial(noteType, status string) bool {
	return c != nil && c.initialDeclared && c.StartsAt(noteType, status)
}
