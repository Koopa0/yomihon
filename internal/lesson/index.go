package lesson

import (
	"maps"
	"slices"
	"strings"
)

const slotSubdir = "System/slots"

// SlotIndex is the immutable lookup index from a lesson slug (for example,
// "jp-minna-l01") to its parsed slot sidecar. The key is the slug declared
// inside each file, never the filename: the two are deliberately unrelated,
// so a lesson note is joined to its slot data by the one stable identifier
// both share. Its zero value is an empty index.
type SlotIndex struct {
	bySlug map[string]*Sidecar
}

// NewSlotIndex parses the captured direct *.yaml children of System/slots and
// keys them by the slug each file declares. The map key is the canonical
// vault-relative path selected by the generation scan; the value is that
// generation's captured file content. Construction retains neither the map nor
// its byte slices. A malformed sidecar is a build fault to surface, not
// reader-facing content to tolerate. Two files claiming the same slug are
// likewise an error because the join would be ambiguous.
func NewSlotIndex(files map[string][]byte) (SlotIndex, Problems) {
	sidecars := make(map[string]*Sidecar, len(files))
	var problems Problems
	for _, relPath := range slices.Sorted(maps.Keys(files)) {
		if !IsSlotSidecar(relPath) {
			continue
		}
		s, err := parseSidecar(relPath, files[relPath])
		if err != nil {
			// One file yomihon cannot read is one lesson without a practice
			// panel. It used to be all of them: the first failure returned an
			// empty index, so a single unknown key in one sidecar withheld the
			// feature from every lesson in the vault, and the only trace was a
			// line in the startup log. What a file's own fault costs is that
			// file.
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

// Problems is every sidecar that could not be used in one generation.
type Problems []Problem

func indexSidecars(sidecars map[string]*Sidecar) (SlotIndex, Problems) {
	idx := SlotIndex{bySlug: make(map[string]*Sidecar, len(sidecars))}
	sources := make(map[string]string, len(sidecars))
	var problems Problems
	for _, relPath := range slices.Sorted(maps.Keys(sidecars)) {
		name, _ := slotSidecarName(relPath)
		s := sidecars[relPath]
		// Validate already reports one message per problem and repairs
		// nothing, which is what this package is for. Joining that list into a
		// single error and returning it was what turned a report into a kill
		// switch.
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

// IsSlotSidecar reports whether relPath names a direct YAML child of the
// vault's System/slots directory. Generation scanners use this predicate to
// capture exactly the non-Markdown bytes SlotIndex needs without duplicating
// the lesson path contract.
func IsSlotSidecar(relPath string) bool {
	_, ok := slotSidecarName(relPath)
	return ok
}

func slotSidecarName(relPath string) (string, bool) {
	name, ok := strings.CutPrefix(relPath, slotSubdir+"/")
	return name, ok && !strings.Contains(name, "/") && strings.HasSuffix(name, ".yaml")
}

// Lookup returns an independent copy of the slot sidecar for a lesson slug, and
// whether one exists. A lesson with no sidecar is normal — most notes are not
// slot lessons — and the caller renders that lesson without a slot card. The
// copy includes nested pattern, slot, and fill data, so a caller cannot mutate
// the index or race a concurrent reader through the returned value.
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
