package schema

import (
	"fmt"
	"slices"
	"time"
)

// The two frontmatter fields a contract can date a note by, in the order they
// are looked for. A contract declaring both is read for the day the note was
// made, because that is what dates a report; one declaring only the second is
// read for that instead. One field answers for the whole vault, so a listing's
// date column carries one meaning rather than two true figures under one name.
const (
	createdField = "created"
	updatedField = "updated"
)

// AuthoredDate resolves the day a note declares for itself, through whichever
// frontmatter field the vault contract gave authority to. Its zero value
// answers with no day at all, never with one taken from the filesystem, the
// note's path, or its prose.
type AuthoredDate struct {
	field string
}

// AuthoredDate returns the contract-derived date resolver. A field is
// authoritative only when the universal fields.known vocabulary declares it; a
// lesson-only declaration cannot describe every note.
func (c *Contract) AuthoredDate() AuthoredDate {
	if c == nil {
		return AuthoredDate{}
	}
	for _, field := range []string{createdField, updatedField} {
		if slices.Contains(c.definition.Fields.Known, field) {
			return AuthoredDate{field: field}
		}
	}
	return AuthoredDate{}
}

// Field names the frontmatter field this contract dates a note by, and is empty
// where the contract declared neither. A surface that has to say how it knows a
// day asks here rather than re-reading fields.known for the same two words.
func (d AuthoredDate) Field() string {
	return d.field
}

// Declared reports whether the contract gave a date field authority.
func (d AuthoredDate) Declared() bool {
	return d.field != ""
}

// Resolve returns one note's declared day as an ISO 8601 calendar date. It
// answers with nothing where the contract gave no field authority and where the
// note left that field out; a value of any other shape is the author's to
// repair, and comes back empty with the reason.
//
// YAML hands an unquoted date over as a time and a quoted one as text, so both
// spellings are read. What comes back is a calendar day with no time of day and
// no zone: what a note declares is the day its author wrote, and a shelf
// ordered on anything finer would be sorting by a precision nobody typed.
func (d AuthoredDate) Resolve(frontmatter map[string]any) (string, error) {
	if !d.Declared() {
		return "", nil
	}
	value, ok := frontmatter[d.field]
	if !ok || value == nil {
		return "", nil
	}
	switch v := value.(type) {
	case time.Time:
		return v.Format(time.DateOnly), nil
	case string:
		for _, layout := range []string{time.DateOnly, time.RFC3339} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.Format(time.DateOnly), nil
			}
		}
	}
	return "", fmt.Errorf("frontmatter %q must be a calendar date such as 2026-09-18", d.field)
}
