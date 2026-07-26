package pages

import (
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// Sidebar is the fully resolved left navigation for one request: the shared
// whole-vault model, plus the current note's context — its same-directory
// siblings, which map and folder branches to open, which entry is the one
// being read. A page with no current note builds one with an empty current path,
// so every branch renders in its default closed, unmarked state.
//
// The open sets are resolved once here from the model's precomputed indexes, so
// the templates only look up a boolean while rendering rather than re-walking the
// tree.
type Sidebar struct {
	Model *nav.Model

	// CurrentPath is the note being read, empty on a page with no note.
	CurrentPath string
	// HereDir labels the siblings block with the current note's directory; Here
	// lists that directory's files (the current one included, to be marked).
	HereDir string
	Here    []nav.NoteRef

	openMaps     map[string]bool
	openBranches map[string]bool
	openFolders  map[string]bool
}

// NewSidebar resolves the left navigation for one page. model is the whole-vault
// nav model; currentPath is the note being read ("" for a page with no note).
func NewSidebar(model *nav.Model, currentPath string) Sidebar {
	// The current path arrives from the request URL; the model's indexes are
	// keyed by NFC paths, so fold it once here to match on either form.
	currentPath = vault.NormalizeNFC(currentPath)
	sb := Sidebar{
		Model:        model,
		CurrentPath:  currentPath,
		openMaps:     map[string]bool{},
		openBranches: map[string]bool{},
		openFolders:  map[string]bool{},
	}
	if model == nil || currentPath == "" {
		return sb
	}

	sb.HereDir, sb.Here = model.Siblings(currentPath)

	// Open every map branch that lists the current note, down to the
	// branch it sits in (each heading prefix, so the ancestors open too).
	for _, p := range model.Placements(currentPath) {
		sb.openMaps[p.MapRelPath] = true
		headings := p.Headings
		for i := 1; i <= len(headings); i++ {
			sb.openBranches[branchKey(p.MapRelPath, headings[:i])] = true
		}
	}

	// Expand the folder branches on the path down to the current note.
	for _, dir := range ancestorDirs(currentPath) {
		sb.openFolders[dir] = true
	}
	return sb
}

// current reports whether relPath is the note being read. The receiver is a
// pointer only to avoid copying the resolved sets on each of the many lookups a
// render makes; the methods never mutate.
func (s *Sidebar) current(relPath string) bool {
	return relPath != "" && relPath == s.CurrentPath
}

// mapOpen reports whether a map branch contains the current note.
func (s *Sidebar) mapOpen(relPath string) bool { return s.openMaps[relPath] }

// branchOpen reports whether a map branch lies on the path to the
// current note. It rebuilds the same key NewSidebar stored, so the two agree.
func (s *Sidebar) branchOpen(mapRelPath string, headings []string) bool {
	return s.openBranches[branchKey(mapRelPath, headings)]
}

// folderOpen reports whether a folder is an ancestor of the current note.
func (s *Sidebar) folderOpen(relPath string) bool { return s.openFolders[relPath] }

// folderTreeOpen reports whether the folder tree should start open — true when
// the current note lives in a folder, so its branch is revealed by default.
func (s *Sidebar) folderTreeOpen() bool { return len(s.openFolders) > 0 }

// currentAttr marks the entry the reader is on with aria-current="page", and
// contributes nothing otherwise, so assistive technology announces the one
// current location without every other link carrying a redundant marker.
func currentAttr(isCurrent bool) templ.Attributes {
	if isCurrent {
		return templ.Attributes{"aria-current": "page"}
	}
	return nil
}

// pathsChainOpen reports whether a study path holds the current note, which
// makes the surrounding group part of the wayfinding chain.
func (s *Sidebar) pathsChainOpen() bool {
	if s.Model == nil {
		return false
	}
	return slices.ContainsFunc(s.Model.Paths(), func(path nav.Map) bool { return s.openMaps[path.RelPath] })
}

// disclosureAttrs marks one sidebar disclosure for the single state owner
// that coordinates <details open>: key names the section stably across pages
// (the reader's manual toggles persist against it for the session), and
// chain marks the branch as part of the current note's wayfinding chain,
// which is always forced open and never persisted — recording a manual
// close would either hide the reader's own location on the next page or
// make persistence look broken.
func disclosureAttrs(key string, chain bool) templ.Attributes {
	attrs := templ.Attributes{"data-key": key}
	if chain {
		attrs["data-chain"] = true
	}
	return attrs
}

// hereLabel names the siblings block after the current note's directory: its
// innermost folder, or the vault root for a file that lives at the top.
func hereLabel(dir string) string {
	if dir == "" {
		return "Vault root"
	}
	if i := strings.LastIndexByte(dir, '/'); i >= 0 {
		return dir[i+1:]
	}
	return dir
}

// childChain extends a branch-heading chain by one level, returning a fresh
// slice so a recursive render never mutates its parent's chain.
func childChain(chain []string, heading string) []string {
	return slices.Concat(chain, []string{heading})
}

// branchKey identifies a map branch by its path and the heading chain leading
// to it, so the resolved open set and the renderer name the same branch.
// The separator is a control character that never appears in a heading.
func branchKey(mapRelPath string, headings []string) string {
	return mapRelPath + "\x1f" + strings.Join(headings, "\x1f")
}

// ancestorDirs lists the directory chain from the vault root down to the file at
// relPath, so the folder tree expands exactly the branches that reveal it:
// "Concepts/golang/Foo.md" yields ["Concepts", "Concepts/golang"]. A vault-root
// file has no folder ancestors.
func ancestorDirs(relPath string) []string {
	i := strings.LastIndexByte(relPath, '/')
	if i < 0 {
		return nil
	}
	dir := relPath[:i]
	var dirs []string
	for start := 0; start <= len(dir); {
		next := strings.IndexByte(dir[start:], '/')
		if next < 0 {
			dirs = append(dirs, dir)
			break
		}
		dirs = append(dirs, dir[:start+next])
		start += next + 1
	}
	return dirs
}

// CapabilityFault is one closed navigation projection stated in the rail: the
// Traditional Chinese summary of what is unavailable, and the contract's own
// English detail.
type CapabilityFault struct {
	Summary string
	Detail  string
}

// CapabilityFaults lists what the rail has to say about closed navigation
// projections, one entry per distinct cause. One cause commonly closes both
// paths and instance projections — a contract that could not be read closes
// everything under it — and printing the same sentence twice under two headings
// reads as two faults rather than one.
func (s *Sidebar) CapabilityFaults() []CapabilityFault {
	if s.Model == nil {
		return nil
	}
	navigation := s.Model.NavigationDiagnostic()
	artifact := s.Model.ArtifactDiagnostic()
	switch {
	case navigation != "" && navigation == artifact:
		return []CapabilityFault{{Summary: "路徑、地圖與治理項目投影目前無法使用。", Detail: navigation}}
	case navigation != "" && artifact != "":
		return []CapabilityFault{
			{Summary: "路徑與地圖目前無法使用。", Detail: navigation},
			{Summary: "治理項目投影目前無法使用。", Detail: artifact},
		}
	case navigation != "":
		return []CapabilityFault{{Summary: "路徑與地圖目前無法使用。", Detail: navigation}}
	case artifact != "":
		return []CapabilityFault{{Summary: "治理項目投影目前無法使用。", Detail: artifact}}
	default:
		return nil
	}
}
