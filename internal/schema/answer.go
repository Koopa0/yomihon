package schema

import (
	"slices"
	"sync/atomic"
)

type answerTypeState struct {
	noteType string
	source   policySource
	stale    atomic.Bool
}

// AnswerType returns the existing note type explicitly assigned to a reader's
// own thoughts by navigation.answer_type. Missing or invalid declarations
// return no type; neither a familiar type name nor a lifecycle stage grants
// this role. The contract source is revalidated before returning the type.
// Once changed bytes are observed, this loaded contract stays closed even if
// the old bytes return. An unreadable source closes only the current call.
func (c *Contract) AnswerType() string {
	if c == nil || c.answerType == nil {
		return ""
	}
	state := c.answerType
	if state.stale.Load() {
		return ""
	}
	same, err := state.source.reread()
	if err != nil {
		return ""
	}
	if !same {
		state.stale.Store(true)
		return ""
	}
	if state.stale.Load() {
		return ""
	}
	return state.noteType
}

func deriveAnswerType(section *navigationSection, unknownKeys, enumTypes []string, source policySource) *answerTypeState {
	if section == nil || len(unknownKeys) > 0 || section.AnswerType == "" || !slices.Contains(enumTypes, section.AnswerType) {
		return nil
	}
	return &answerTypeState{noteType: section.AnswerType, source: source}
}
