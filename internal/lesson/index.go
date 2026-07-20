package lesson

import (
	"fmt"
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
func NewSlotIndex(files map[string][]byte) (SlotIndex, error) {
	sidecars := make(map[string]*Sidecar, len(files))
	for _, relPath := range slices.Sorted(maps.Keys(files)) {
		if !IsSlotSidecar(relPath) {
			continue
		}
		s, err := parseSidecar(relPath, files[relPath])
		if err != nil {
			return SlotIndex{}, err
		}
		sidecars[relPath] = s
	}
	return indexSidecars(sidecars)
}

func indexSidecars(sidecars map[string]*Sidecar) (SlotIndex, error) {
	idx := SlotIndex{bySlug: make(map[string]*Sidecar, len(sidecars))}
	sources := make(map[string]string, len(sidecars))
	for _, relPath := range slices.Sorted(maps.Keys(sidecars)) {
		name, _ := slotSidecarName(relPath)
		s := sidecars[relPath]
		if s.Slug == "" {
			return SlotIndex{}, fmt.Errorf("slot sidecar %s declares no slug", name)
		}
		if problems := s.Validate(); len(problems) != 0 {
			return SlotIndex{}, fmt.Errorf("validate slot sidecar %s: %s", name, strings.Join(problems, "; "))
		}
		if _, dup := idx.bySlug[s.Slug]; dup {
			return SlotIndex{}, fmt.Errorf("slot slug %q declared by two sidecars (%s and %s)", s.Slug, sources[s.Slug], relPath)
		}
		idx.bySlug[s.Slug] = s
		sources[s.Slug] = relPath
	}
	return idx, nil
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
