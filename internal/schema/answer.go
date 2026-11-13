package schema

// AnswerType returns the note type that holds the thought role: the notes a
// reader writes about what they read. A contract that gives no type the role,
// or one whose roles could not be honoured, returns no type.
func (r NavigationRoles) AnswerType() string {
	if r.sets == nil {
		return ""
	}
	return r.sets.answer
}
