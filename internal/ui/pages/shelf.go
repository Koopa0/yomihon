package pages

import "slices"

// Shelf is one organisation's listing of documents, in the order that
// organisation puts them: a name, a measure, one sentence, and rows. The vault
// declares several — the courses, the maps, the reports, the folders — and each
// is the same shelf seen at a different width. The home page shows a shelf's
// opening rows, an index page shows it unfolded, and the left rail shows the
// one shelf the reader is inside. Nothing here knows which of those it is in.
//
// A string left empty is not drawn, and that is how a shelf says it does not
// know rather than saying nothing is there. When the contract describing an
// organisation cannot be read, "2 courses" and "no courses" are both claims the
// page cannot make; the reason is stated once elsewhere on the page, so the
// shelf that would have repeated it stays quiet instead.
//
// A shelf carries no status vocabulary, no rule names and no course grammar.
// Whoever owns the organisation reads those, decides what a row says, and hands
// over words: the schema contract is read in one place and this is not it.
//
// Its rows are read and never written. A width narrows by taking a run of them,
// so the same rows are shared by every view of one shelf built for one request,
// and a width that wrote to a row would change what another had already shown.
// Anything a width needs to say about a row it says in its own markup.
type Shelf struct {
	Title string
	// Lede is the one sentence. Any figure beyond Count belongs inside it,
	// already written out, for the same reason Count is a string.
	Lede string
	// Count is the one measure, formatted by the owner. It is a string because
	// how a number reads is a matter of language, and language is the owner's:
	// a shelf that formatted its own count would need the wording table and the
	// reader's language, which is two things it has no other use for.
	Count string
	// Href is the page this shelf unfolds to, and is empty on that page itself.
	Href string
	// Empty is what the shelf says when the organisation holds nothing. It is
	// the shelf's own sentence rather than the page's, because what an empty
	// listing means differs by organisation and only the owner knows it.
	Empty string
	Rows  []Row
}

// Row is one document on the shelf.
type Row struct {
	Text string
	// Href is where the row leads, and is empty for a row that is listed
	// without being a stop: a lesson nobody has written yet is still part of
	// the course, and saying so is not the same as offering to open it.
	Href string
	// Mark is the one thing this organisation measures a row by, already in
	// words — a course's extent, a report's date. An organisation that needs
	// two writes them as one.
	Mark string
	// Fault says this row is itself a hole in the shelf — a missing lesson, a
	// source that could not be read. It does not describe the document the row
	// points at: a row leading to a note with a fault in it is an ordinary row.
	Fault bool
	// Current marks the row the reader is on. It says which row that is and
	// nothing about how it should look: a width that shows the reader where
	// they are settles that for itself, and the widths that do not show it
	// ignore the mark.
	Current bool
}

// railRows takes the rows a rail can show around the one the reader is on, and
// reports how many it left out. A shelf that fits is returned whole, so the
// ordinary case carries no elision at all.
//
// The window is centred on the current row rather than taken from the top,
// which is the whole difference between this width and the others: a folder of
// seven hundred entries would otherwise show its first two dozen and never the
// one being read. A shelf the reader is not inside has no centre, so it opens
// at the beginning.
func railRows(s *Shelf, limit int) (window []Row, trimmed int) {
	limit = max(limit, 0)
	if len(s.Rows) <= limit {
		return s.Rows, 0
	}
	at := slices.IndexFunc(s.Rows, func(r Row) bool { return r.Current })
	if at < 0 {
		return s.Rows[:limit], len(s.Rows) - limit
	}
	start := max(0, at-limit/2)
	end := min(len(s.Rows), start+limit)
	start = max(0, end-limit)
	return s.Rows[start:end], len(s.Rows) - (end - start)
}

// shelfRows takes the rows a narrow width can show, in the order a reader meets
// them, and stops once it has limit of them. Only rows that lead somewhere are
// taken: a narrow shelf is a way in, and a row that is not a stop cannot be
// one. A limit below zero is read as none rather than trusted into a make.
func shelfRows(s *Shelf, limit int) []Row {
	limit = max(limit, 0)
	rows := make([]Row, 0, limit)
	for _, row := range s.Rows {
		if len(rows) == limit {
			return rows
		}
		if row.Href == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}
