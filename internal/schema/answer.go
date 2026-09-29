package schema

import "slices"

// AnswerType returns the existing note type explicitly assigned to a reader's
// own thoughts by navigation.answer_type. Missing or invalid declarations
// return no type; neither a familiar type name nor a lifecycle stage grants
// this role.
func (r NavigationRoles) AnswerType() string {
	return r.answerType
}

func deriveAnswerType(section *navigationSection, unknownKeys, enumTypes []string) string {
	if section == nil || len(unknownKeys) > 0 || !slices.Contains(enumTypes, section.AnswerType) {
		return ""
	}
	return section.AnswerType
}
