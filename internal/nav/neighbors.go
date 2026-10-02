package nav

// Neighbors is one study path's answer to "what comes before and after this
// note". Prev or Next is the zero NoteRef when the note opens or closes its
// sequence component.
type Neighbors struct {
	// PathTitle and PathRelPath name the syllabus the ordering came from.
	PathTitle   string
	PathRelPath string
	// Unit is the noun that path counts and steps in, so a step onward is
	// called what the author arranged.
	Unit Unit
	Prev NoteRef
	Next NoteRef
}

// PathNeighbors returns, for each study path that walks the note at relPath,
// the readable lessons immediately before and after it in that path's own
// order. It is the course's answer; the folder's is FolderStep.
//
// Neighbors are read off a path's components, so the main line and a side
// branch never link, and an accepted entry that does not resolve drops out with
// its neighbors joining around it. A note listed twice in one path takes its
// first occurrence; one two paths both list gets an answer per path. What a
// side branch offers at its ends is PathPlace's, and none of it is here.
func (m *Model) PathNeighbors(relPath string) []Neighbors {
	if m == nil || relPath == "" {
		return nil
	}
	var out []Neighbors
	for i := range m.paths {
		path := &m.paths[i]
		component, at, found := path.locate(relPath)
		if !found {
			continue
		}
		stops := path.components[component]
		n := Neighbors{PathTitle: path.Title, PathRelPath: path.RelPath, Unit: path.Unit}
		if at > 0 {
			n.Prev = stops[at-1]
		}
		if at+1 < len(stops) {
			n.Next = stops[at+1]
		}
		out = append(out, n)
	}
	return out
}

// locate is where a note first appears among a path's components, in the order
// they are walked: which component, and where in it. Both answers about a note
// in a path — its neighbors and its place — read it from here.
func (p *Path) locate(relPath string) (component, at int, found bool) {
	for i, stops := range p.components {
		if at := indexOfStop(stops, relPath); at >= 0 {
			return i, at, true
		}
	}
	return 0, 0, false
}

// Place is where a note sits in one study path beyond the order it is read in:
// the part it belongs to, whether it is a lesson of a side branch, and what the
// path hands a reader at a branch's two ends. It is not a step. A branch's way
// back and its way on lead off the order the course is walked in, so none of
// them can be mistaken for a previous or a next lesson, and the main line's own
// steps and counts are what Neighbors and the path's totals say, whatever
// branches hang from it.
type Place struct {
	// Part is the name of the part the note sits in, empty where it sits in
	// none. A side branch's lessons sit in the part of the lesson it hangs from.
	Part string
	// OnBranch is whether the note is a lesson of a side branch.
	OnBranch bool
	// Back is the main-line lesson the branch hangs from, offered on the
	// branch's first lesson. It is zero everywhere else, and where that lesson
	// cannot be opened.
	Back NoteRef
	// Onward is the main-line lesson after the one the branch hangs from,
	// offered on the branch's last lesson: the next one that can be opened, so a
	// lesson planned and not yet written is passed over. It is zero everywhere
	// else, and where there is none.
	Onward NoteRef
	// ToContents is set on a branch's last lesson that has no main-line lesson
	// to go on to: the branch hangs from the course's last lesson, or from
	// nothing at all. The way on is then the course's contents.
	ToContents bool
	// Asides are the first lessons of the side branches hanging from this
	// lesson, in the order the author wrote them. They are offered beside the
	// lesson's own steps and are not among them.
	Asides []NoteRef
}

// PathPlace answers where the note at relPath sits in the study path at
// pathRel. found is false when that path does not walk the note. A note listed
// twice in one path takes its first occurrence for its part and branch, and
// carries the asides of every row that names it.
func (m *Model) PathPlace(relPath, pathRel string) (place Place, found bool) {
	if m == nil || relPath == "" {
		return Place{}, false
	}
	at := m.indexOfPath(pathRel)
	if at < 0 {
		return Place{}, false
	}
	path := &m.paths[at]
	component, index, ok := path.locate(relPath)
	if !ok || component >= len(path.info) {
		return Place{}, ok
	}
	stops, info := path.components[component], path.info[component]
	if index < len(info.parts) {
		place.Part = info.parts[index]
	}
	if info.branch != nil {
		place.OnBranch = true
		if index == 0 {
			place.Back = info.branch.anchor
		}
		if index == len(stops)-1 {
			place.Onward = info.branch.onward
			place.ToContents = place.Onward.RelPath == ""
		}
		return place, true
	}
	for i, stop := range stops {
		if stop.RelPath == relPath {
			place.Asides = append(place.Asides, info.asides[i]...)
		}
	}
	return place, true
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
