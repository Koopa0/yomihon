package nav

import (
	"github.com/koopa0/yomihon/internal/vault"
)

// TeachingPath returns the one study path the reading rail should show for
// relPath. nil means the folder rail stands in: the note is taught by no path,
// or by more than one without a single primary.
//
// A note one path teaches answers with that path. Several paths teaching the
// same note answer only when exactly one of them shares the note's declared
// domain — the primary course for that note — and a tie or an undeclared domain
// falls back to the folder.
func (m *Model) TeachingPath(relPath, noteDomain string) *Path {
	if m == nil || relPath == "" {
		return nil
	}
	relPath = vault.NormalizeNFC(relPath)
	if m.IsPath(relPath) {
		return m.Path(relPath)
	}
	steps := m.PathNeighbors(relPath)
	switch len(steps) {
	case 0:
		return nil
	case 1:
		return m.Path(steps[0].PathRelPath)
	}
	if noteDomain == "" {
		return nil
	}
	var primary string
	for i := range steps {
		step := steps[i]
		path := m.Path(step.PathRelPath)
		if path == nil || path.Domain != noteDomain {
			continue
		}
		if primary != "" {
			return nil
		}
		primary = step.PathRelPath
	}
	if primary == "" {
		return nil
	}
	return m.Path(primary)
}
