package lesson

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SlotIndex maps a lesson slug (e.g. "jp-minna-l01") to its parsed slot
// sidecar. The key is the slug declared INSIDE each file, never the filename —
// the two are deliberately unrelated (D29), so a lesson note is joined to its
// slot data by the one stable identifier both share.
type SlotIndex map[string]*Sidecar

// BuildSlotIndex loads every *.yaml sidecar under dir (System/slots/) and keys
// it by the slug each file declares. A file that fails to parse is a hard error:
// the slot data is machine-owned and byte-controlled (D29, byte-identical from
// yomihon), so a malformed sidecar is a build fault to surface, not reader-facing
// content to tolerate. Two files claiming the same slug is likewise an error —
// the join would be ambiguous, and an ambiguous join is never guessed (the
// vault dialect's rule, mirrored here).
func BuildSlotIndex(dir string) (SlotIndex, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read slots dir %s: %w", dir, err)
	}
	idx := make(SlotIndex, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		s, err := Load(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if s.Slug == "" {
			return nil, fmt.Errorf("slot sidecar %s declares no slug", name)
		}
		if prev, dup := idx[s.Slug]; dup {
			return nil, fmt.Errorf("slot slug %q declared by two sidecars (%s and %s)", s.Slug, prev.Lesson, s.Lesson)
		}
		idx[s.Slug] = s
	}
	return idx, nil
}

// Lookup returns the slot sidecar for a lesson slug, and whether one exists. A
// lesson with no sidecar is normal — most notes are not slot lessons — and the
// caller renders that lesson without a slot card.
func (x SlotIndex) Lookup(slug string) (*Sidecar, bool) {
	s, ok := x[slug]
	return s, ok
}
