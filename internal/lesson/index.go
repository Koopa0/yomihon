package lesson

import (
	"maps"
	"slices"
	"strings"
)

const slotSubdir = "System/slots"

// SlotIndex is the immutable lookup from a lesson slug to its parsed slot
// sidecar. The key is the slug declared inside each file, never the filename:
// the two are deliberately unrelated. Its zero value is an empty index.
type SlotIndex struct {
	bySlug map[string]*Sidecar
}

// NewSlotIndex parses the captured *.yaml children of System/slots and keys
// them by the slug each declares, retaining neither the map nor its bytes. A
// malformed sidecar, or two files claiming one slug, is a reported problem.
func NewSlotIndex(files map[string][]byte) (SlotIndex, []Problem) {
	sidecars := make(map[string]*Sidecar, len(files))
	var problems []Problem
	for _, relPath := range slices.Sorted(maps.Keys(files)) {
		if !IsSlotSidecar(relPath) {
			continue
		}
		s, err := parseSidecar(relPath, files[relPath])
		if err != nil {
			// One file yomihon cannot read costs that one lesson its practice
			// panel, not every lesson in the vault.
			problems = append(problems, Problem{Source: relPath, Message: err.Error()})
			continue
		}
		sidecars[relPath] = s
	}
	idx, indexed := indexSidecars(sidecars)
	return idx, append(problems, indexed...)
}

// Problem is one sidecar yomihon could not use, and why. yomihon reports and a
// human edits the file; nothing here repairs anything.
type Problem struct {
	Source  string
	Message string
}

func indexSidecars(sidecars map[string]*Sidecar) (SlotIndex, []Problem) {
	idx := SlotIndex{bySlug: make(map[string]*Sidecar, len(sidecars))}
	sources := make(map[string]string, len(sidecars))
	var problems []Problem
	for _, relPath := range slices.Sorted(maps.Keys(sidecars)) {
		name, _ := slotSidecarName(relPath)
		s := sidecars[relPath]
		if s.Slug == "" {
			problems = append(problems, Problem{Source: relPath, Message: "slot sidecar " + name + " declares no slug"})
			continue
		}
		if found := s.Validate(); len(found) != 0 {
			for _, message := range found {
				problems = append(problems, Problem{Source: relPath, Message: message})
			}
			continue
		}
		if _, dup := idx.bySlug[s.Slug]; dup {
			problems = append(problems, Problem{
				Source:  relPath,
				Message: "slot slug " + s.Slug + " is already declared by " + sources[s.Slug],
			})
			continue
		}
		idx.bySlug[s.Slug] = s
		sources[s.Slug] = relPath
	}
	return idx, problems
}

// IsSlotSidecar reports whether relPath names a direct YAML child of
// System/slots. Generation scanners use it to capture exactly those bytes.
func IsSlotSidecar(relPath string) bool {
	_, ok := slotSidecarName(relPath)
	return ok
}

func slotSidecarName(relPath string) (string, bool) {
	name, ok := strings.CutPrefix(relPath, slotSubdir+"/")
	return name, ok && !strings.Contains(name, "/") && strings.HasSuffix(name, ".yaml")
}

// Lookup returns an independent copy of the slot sidecar for a lesson slug,
// and whether one exists. A lesson with no sidecar is normal. The copy is deep,
// so a caller cannot mutate the index or race a reader through the value.
func (x SlotIndex) Lookup(slug string) (*Sidecar, bool) {
	s, ok := x.bySlug[slug]
	if !ok {
		return nil, false
	}
	return cloneSidecar(s), true
}

// Len reports the number of indexed sidecars.
func (x SlotIndex) Len() int {
	return len(x.bySlug)
}

func cloneSidecar(source *Sidecar) *Sidecar {
	clone := *source
	clone.Patterns = slices.Clone(source.Patterns)
	for i := range clone.Patterns {
		if source.Patterns[i].Slots == nil {
			continue
		}
		clone.Patterns[i].Slots = make(map[string]Position, len(source.Patterns[i].Slots))
		for key, position := range source.Patterns[i].Slots {
			position.Fills = slices.Clone(position.Fills)
			clone.Patterns[i].Slots[key] = position
		}
	}
	return &clone
}
