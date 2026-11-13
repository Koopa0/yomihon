package schema

// DraftField is the frontmatter key an author writes to say a note is not
// finished yet. The product defines it, so it is spelled here once and read
// the same way by every face whether or not a contract governs the folder.
const DraftField = "draft"

// Draft reports whether a note's frontmatter declares it unfinished: the key
// holds the boolean true. Absent, false or any other shape is a finished note.
func Draft(frontmatter map[string]any) bool {
	draft, isBool := frontmatter[DraftField].(bool)
	return isBool && draft
}
