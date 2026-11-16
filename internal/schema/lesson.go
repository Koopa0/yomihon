package schema

// IsLessonType reports whether noteType is the type this vault files course
// members as. A study path asks it to tell a path made of lessons from a path
// made of other notes, so the comparison against that spelling is made here
// and nowhere else. A contract that gives no type the lesson role has none, and
// so does the zero value.
func (r NavigationRoles) IsLessonType(noteType string) bool {
	if r.sets == nil || r.sets.lesson == "" {
		return false
	}
	return NormalizeWord(noteType) == r.sets.lesson
}
