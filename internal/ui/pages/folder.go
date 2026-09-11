package pages

import (
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// FolderView is one level of the vault's own directories: where it sits, how
// much is under it, and the shelf of what is in it. It is the same shelf the
// folder mode's own page unfolds, one level down, so a reader descending the
// tree meets one listing rather than a new kind of page at every step.
type FolderView struct {
	// Crumbs are the folders above this one, so a reader can climb out.
	Crumbs []nav.NoteRef
	// Count is what this level holds, already written out: the folders it has
	// and the files in it.
	Count string
	Shelf Shelf
}

// NewFolderLevel builds one level of the tree. The folders come first with
// how many notes sit under each of them, then the notes in this folder, then
// the other files the desk can open, labelled and uncounted.
func NewFolderLevel(dir, name string, notes []nav.NoteRef, subfolders []nav.Folder, lang wording.Lang, articleLang ArticleLanguageFor) FolderView {
	count := plural(countNotes(notes, nil), wording.FolderNoteCountOne, wording.FolderNoteCountMany, lang)
	if len(subfolders) > 0 {
		count = plural(len(subfolders), wording.SubfolderCountOne, wording.SubfolderCountMany, lang) + count
	}
	return FolderView{
		Crumbs: Breadcrumb(dir),
		Count:  count,
		Shelf: Shelf{
			Title: name,
			Empty: wording.FolderEmpty.In(lang),
			Rows:  folderRows(notes, subfolders, lang, false, articleLang),
		},
	}
}
