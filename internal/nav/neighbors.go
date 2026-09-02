package nav

// Neighbors is one study path's answer to "what comes before and after this
// note". Prev or Next is the zero NoteRef when the note opens or closes its
// sequence component.
type Neighbors struct {
	// PathTitle and PathRelPath name the syllabus the ordering came from, so a
	// note two paths both teach can label each answer with its course.
	PathTitle   string
	PathRelPath string
	Prev        NoteRef
	Next        NoteRef
}

// PathNeighbors returns, for each study path that walks the note at relPath,
// the readable lessons immediately before and after it in that path's own
// order. It is the course's answer; the folder's is FolderStep.
//
// The order is the declared one: each path's components were built from the
// sequence grammar — one main line joining the projectable primary groups, and
// one component per projectable local branch. Primary and local never link to
// each other: the lesson after the one a side branch hangs from is the next
// main-line lesson, and a side branch's last lesson has no next. An accepted
// entry that does not resolve is planned, not walkable — it drops out of the
// openable component, and the resolved lessons on either side of it join up
// within that component, never across a component boundary.
//
// A note listed twice in the same path takes its first occurrence, in the
// first component that lists it; within one syllabus that is a deterministic
// reading, not a guess. A note two paths both list gets one answer per path,
// because the server cannot know which course the reader is walking and this
// vault never guesses — it reports in full.
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
