package schema

import "slices"

// AnswerType returns the existing note type explicitly assigned to a reader's
// own thoughts by navigation.answer_type. Missing or invalid declarations
// return no type; neither a familiar type name nor a lifecycle stage grants
// this role.
func (r NavigationRoles) AnswerType() string {
	if r.sets == nil {
		return ""
	}
	return r.sets.answer
}

// withAnswerType returns r carrying the declared answer type. The declaration
// is independent of the path and map sets, so it is kept even where those
// could not be honoured.
func (r NavigationRoles) withAnswerType(section *navigationSection, unknownKeys, enumTypes []string) NavigationRoles {
	if section == nil || len(unknownKeys) > 0 || !slices.Contains(enumTypes, section.AnswerType) {
		return r
	}
	sets := roleSets{}
	if r.sets != nil {
		sets = *r.sets
	}
	sets.answer = section.AnswerType
	r.sets = &sets
	return r
}
