package schema

// IsLessonType reports whether noteType is the type this vault files course
// members as. A study path asks it to tell a path made of lessons from a path
// made of other notes, so the comparison against that spelling is made here
// and nowhere else. A contract that does not list the type among its note
// types has none, and so does the zero value.
func (r NavigationRoles) IsLessonType(noteType string) bool {
	if r.sets == nil || r.sets.lesson == "" {
		return false
	}
	return NormalizeWord(noteType) == r.sets.lesson
}

// withLessonType returns r carrying the lesson type the contract declares. The
// declaration is independent of the path and map sets, so it is kept even where
// those could not be honoured, as the answer type is.
func (r NavigationRoles) withLessonType(name string, declared bool) NavigationRoles {
	if !declared {
		return r
	}
	sets := roleSets{}
	if r.sets != nil {
		sets = *r.sets
	}
	sets.lesson = NormalizeWord(name)
	r.sets = &sets
	return r
}
