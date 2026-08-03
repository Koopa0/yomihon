package nav

// Neighbors is one study path's answer to "what comes before and after this
// note". Prev or Next is the zero NoteRef when the note opens or closes the
// path.
type Neighbors struct {
	// PathTitle and PathRelPath name the syllabus the ordering came from, so a
	// note two paths both teach can label each answer with its course.
	PathTitle   string
	PathRelPath string
	Prev        NoteRef
	Next        NoteRef
}

// Neighbors returns, for each study path that lists the note at relPath, the
// readable lessons immediately before and after it in that path's own order.
//
// Two decisions define the answer, both taken from how a syllabus is written
// and read rather than from the data shapes. A warning row — a lesson planned
// but unwritten, an ambiguous name, a template — is not a stop: the syllabus
// is written forward, and adding a planned lesson must not break the reader's
// step to the next lesson they can actually open. And a note two paths both
// list gets one answer per path, because the server cannot know which course
// the reader is walking and this vault never guesses — it reports in full.
//
// A note listed twice in the same path takes its first occurrence; within one
// syllabus that is a deterministic reading, not a guess.
func (m *Model) Neighbors(relPath string) []Neighbors {
	if m == nil || relPath == "" {
		return nil
	}
	var out []Neighbors
	for i := range m.paths {
		path := &m.paths[i]
		stops := resolvedStops(path.Branches, nil)
		for at, stop := range stops {
			if stop.RelPath != relPath {
				continue
			}
			n := Neighbors{PathTitle: path.Title, PathRelPath: path.RelPath}
			if at > 0 {
				n.Prev = stops[at-1]
			}
			if at+1 < len(stops) {
				n.Next = stops[at+1]
			}
			out = append(out, n)
			break
		}
	}
	return out
}

// resolvedStops flattens a branch tree into its readable lessons in document
// order: a branch's own entries, then its subbranches, exactly as the syllabus
// lists them. Only resolved instance entries are stops; the warning kinds keep
// their place in the syllabus display and contribute nothing here.
func resolvedStops(branches []Branch, stops []NoteRef) []NoteRef {
	for i := range branches {
		b := &branches[i]
		for j := range b.Entries {
			if e := &b.Entries[j]; e.Kind == EntryResolved {
				stops = append(stops, NoteRef{Name: e.Text, RelPath: e.RelPath})
			}
		}
		stops = resolvedStops(b.Subbranches, stops)
	}
	return stops
}
