package schema

import (
	"fmt"
	"slices"
	"time"
)

// createdField is the frontmatter field a contract dates a note by: the day
// the note was made, which is also what dates a report.
const createdField = "created"

// AuthoredDate resolves the day a note declares for itself, through whichever
// frontmatter field the vault contract gave authority to. Its zero value
// answers with no day at all, never with one taken from the filesystem, the
// note's path, or its prose.
type AuthoredDate struct {
	field string
}

// AuthoredDate returns the contract-derived date resolver. The field is
// authoritative only when fields.common declares it; a declaration on one type
// cannot describe every note.
func (c *Contract) AuthoredDate() AuthoredDate {
	if c == nil || !slices.Contains(c.common, createdField) {
		return AuthoredDate{}
	}
	return AuthoredDate{field: createdField}
}

// Field names the frontmatter field this contract dates a note by, and is empty
// where the contract declared none. A surface that has to say how it knows a
// day asks here rather than re-reading fields.common for the same word.
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
