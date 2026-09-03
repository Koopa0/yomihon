package nav

// Neighbors is one study path's answer to "what comes before and after this
// note". Prev or Next is the zero NoteRef when the note opens or closes its
// sequence component.
type Neighbors struct {
	// PathTitle and PathRelPath name the syllabus the ordering came from.
	PathTitle   string
	PathRelPath string
	Prev        NoteRef
	Next        NoteRef
}

// PathNeighbors returns, for each study path that walks the note at relPath,
// the readable lessons immediately before and after it in that path's own
// order. It is the course's answer; the folder's is FolderStep.
//
// Neighbors are read off a path's components, so the main line and a side
// branch never link, and an accepted entry that does not resolve drops out with
// its neighbors joining around it. A note listed twice in one path takes its
// first occurrence; one two paths both list gets an answer per path.
func (m *Model) PathNeighbors(relPath string) []Neighbors {
	if m == nil || relPath == "" {
		return nil
	}
	var out []Neighbors
	for i := range m.paths {
		path := &m.paths[i]
		for _, stops := range path.components {
			at := indexOfStop(stops, relPath)
			if at < 0 {
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

// indexOfStop is where a note first appears in one component's walk, or -1.
func indexOfStop(stops []NoteRef, relPath string) int {
	for i := range stops {
		if stops[i].RelPath == relPath {
			return i
		}
	}
	return -1
}
