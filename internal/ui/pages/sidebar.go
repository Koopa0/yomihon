package pages

import (
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
)

// Sidebar is the fully resolved left navigation for one request: the shared
// whole-vault model, plus the current note's context — its same-directory
// siblings, which study-path and folder branches to open, which entry is the one
// being read — and the lifecycle doorway carrying the count of notes still
// awaiting a decision. A page with no current note (a report) builds one with an
// empty current path, so every branch renders in its default closed, unmarked
// state.
//
// The open sets are resolved once here from the model's precomputed indexes, so
// the templates only look up a boolean while rendering rather than re-walking the
// tree.
type Sidebar struct {
	Model     *nav.Model
	Lifecycle []LifecycleItem

	// Pending is the count of notes still awaiting a decision; PendingKnown is
	// false when the write face is closed, so the doorway shows no number rather
	// than a misleading zero.
	Pending      int
	PendingKnown bool

	// CurrentPath is the note being read, empty on a page with no note.
	CurrentPath string
	// HereDir labels the siblings block with the current note's directory; Here
	// lists that directory's files (the current one included, to be marked).
	HereDir string
	Here    []nav.NoteRef

	openSyllabi  map[string]bool
	openSections map[string]bool
	openFolders  map[string]bool
}

// NewSidebar resolves the left navigation for one page. model is the whole-vault
// nav model; currentPath is the note being read ("" for a page with no note);
// lifecycle and the pending figure come from the note handler. pendingKnown is
// false when the write face is closed, so the doorway shows no number.
func NewSidebar(model *nav.Model, currentPath string, lifecycle []LifecycleItem, pending int, pendingKnown bool) Sidebar {
	// The current path arrives from the request URL; the model's indexes are
	// keyed by NFC paths, so fold it once here to match on either form.
	currentPath = graph.NormalizeNFC(currentPath)
	sb := Sidebar{
		Model:        model,
		Lifecycle:    lifecycle,
		Pending:      pending,
		PendingKnown: pendingKnown,
		CurrentPath:  currentPath,
		openSyllabi:  map[string]bool{},
		openSections: map[string]bool{},
		openFolders:  map[string]bool{},
	}
	if model == nil || currentPath == "" {
		return sb
	}

	sb.HereDir, sb.Here = model.Siblings(currentPath)

	// Open every study-path branch that lists the current note, down to the
	// section it sits in (each heading prefix, so the ancestors open too).
	for _, p := range model.Placements(currentPath) {
		sb.openSyllabi[p.SyllabusRelPath] = true
		for i := 1; i <= len(p.Headings); i++ {
			sb.openSections[sectionKey(p.SyllabusRelPath, p.Headings[:i])] = true
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

// syllabusOpen reports whether a study-path branch contains the current note.
func (s *Sidebar) syllabusOpen(relPath string) bool { return s.openSyllabi[relPath] }

// sectionOpen reports whether a study-path section lies on the path to the
// current note. It rebuilds the same key NewSidebar stored, so the two agree.
func (s *Sidebar) sectionOpen(syllabusRelPath string, headings []string) bool {
	return s.openSections[sectionKey(syllabusRelPath, headings)]
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

// pathsChainOpen reports whether any study-path branch holds the current
// note, which makes the surrounding group part of the wayfinding chain: the
// reader's location must stay visible even if the group was closed by hand
// on an earlier page.
func (s *Sidebar) pathsChainOpen() bool { return len(s.openSyllabi) > 0 }

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

// navRestoreScript is the sidebar's pre-paint disclosure restore, inlined so
// it runs synchronously after the sidebar's markup exists and before the
// first paint that could flash a wrong state. It reapplies the session's
// persisted manual toggles to every keyed disclosure — except the
// wayfinding chain, which is always forced open. The enhancement script
// owns every later change and writes the same storage key.
const navRestoreScript = `<script>
(() => {
	'use strict';
	let stored = {};
	try { stored = JSON.parse(sessionStorage.getItem('kurodo.nav') || '{}') || {}; } catch { stored = {}; }
	document.querySelectorAll('.k-rail-left details[data-key]').forEach((d) => {
		if (d.hasAttribute('data-chain')) { d.open = true; return; }
		const want = stored[d.dataset.key];
		if (typeof want === 'boolean') { d.open = want; }
	});
})();
</script>`

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

// childChain extends a section-heading chain by one level, returning a fresh
// slice so a recursive render never mutates its parent's chain.
func childChain(chain []string, heading string) []string {
	return slices.Concat(chain, []string{heading})
}

// sectionKey identifies a study-path section by its path and the heading chain
// leading to it, so the resolved open set and the renderer name the same section.
// The separator is a control character that never appears in a heading.
func sectionKey(syllabusRelPath string, headings []string) string {
	return syllabusRelPath + "\x1f" + strings.Join(headings, "\x1f")
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
