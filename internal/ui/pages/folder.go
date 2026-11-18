package pages

import (
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// FolderView is one level of the vault's own directories: where it sits, how
// much is under it, and the list of what is in it.
type FolderView struct {
	// Sidebar is the column beside the page, or the drawer it is where the page
	// has no column.
	Sidebar Sidebar
	// Crumbs lead from the vault down to this folder's parent, the vault
	// first, so a reader can climb out at any step.
	Crumbs []nav.NoteRef
	Title  string
	// Count is what this level holds, already written out: the folders it has
	// and the notes under it at every depth. Empty when the list's own heading
	// already says it.
	Count string
	List  List
	// Cover is the folder's README stood in front of its listing, when it has
	// one with an opening paragraph.
	Cover Cover
}

// Cover is a folder's opening paragraph, already rendered and sanitised, and
// the way to the whole README when there is more of it.
type Cover struct {
	HTML string
	// ReadHref is empty when the paragraph is all the README says.
	ReadHref string
}

// ListQuery is what a request says about how a list is shown.
type ListQuery struct {
	// Sort is the order in effect. Default is the order the page opens in, which
	// a link leaves out; a reader's own choice is always written.
	Sort, Default ListSort
	Filter        string
	Expand        bool
}

func (q ListQuery) defaultSort() ListSort {
	if q.Default == "" {
		return SortByName
	}
	return q.Default
}

// Resolved settles the order against the page's default once the page knows
// which that is: a request that chose nothing gets it.
func (q ListQuery) Resolved(def ListSort) ListQuery {
	q.Default = def
	if q.Sort == "" {
		q.Sort = def
	}
	return q
}

// ParseListQuery reads the sort, the filter word and the unfolded flag from a
// request's query. An order it does not know is left unchosen.
func ParseListQuery(v url.Values) ListQuery {
	filter := []rune(strings.TrimSpace(v.Get("filter")))
	if len(filter) > maxFilterRunes {
		filter = filter[:maxFilterRunes]
	}
	return ListQuery{
		Sort:   ParseListSort(v.Get("sort")),
		Filter: string(filter),
		Expand: v.Get("expand") == "1",
	}
}

// maxFilterRunes is as long a word as a filter reads: a title is not longer
// than the words anyone types to find it.
const maxFilterRunes = 100

// FolderInput is everything one level of the tree is built from.
type FolderInput struct {
	Dir, Name, VaultName string
	// Notes are the files this folder holds directly, without the README the
	// cover has taken.
	Notes      []nav.NoteRef
	Subfolders []nav.Folder
	Model      *nav.Model
	Query      ListQuery
	Now        time.Time
	Lang       wording.Lang
	// ArticleLang names the language a note is written in, for a note that does
	// not declare it.
	ArticleLang ArticleLanguageFor
}

// folderCount says what a level holds, leaving out a term that is zero. It is
// empty when the level holds neither, and the page then draws no line.
func folderCount(notes, folders int, lang wording.Lang) string {
	var parts []string
	if folders > 0 {
		parts = append(parts, counted(folders, wording.SubfolderCountOne, wording.SubfolderCountMany, lang))
	}
	if notes > 0 {
		parts = append(parts, noteCountFact(notes, lang))
	}
	return strings.Join(parts, wording.CountJoin.In(lang))
}

// NewFolderLevel builds one level of the tree from the list rule.
func NewFolderLevel(in *FolderInput) FolderView {
	notes, others := splitNotesAndFiles(in.Notes)
	total := countNotes(in.Notes, in.Subfolders)
	folders := len(listFolders(in.Subfolders, in))
	ordered := hasAuthorOrder(listNotes(notes, in))
	def := SortByName
	if ordered {
		def = SortByOrder
	}
	q := in.Query.Resolved(def)
	in.Query = q

	v := FolderView{Crumbs: folderCrumbs(in.Dir, in.VaultName), Title: in.Name}
	list := folderBar(in, len(notes), folders, total, ordered)
	list.Count = folderCount(total, folders, in.Lang)

	base := folderHref(in.Dir)
	switch {
	case q.Filter != "":
		list.Groups = filterGroups(in, q)
		list.Cleared = ListCleared{Text: strings.Replace(wording.NoMatch.In(in.Lang), "%s", q.Filter, 1), Href: base + q.Address(q.Sort, "", q.Expand)}
	case q.Expand && folders > 0:
		list.Label = strings.Replace(wording.AllNotes.In(in.Lang), "%d", Thousands(total), 1)
		list.Groups = expandedGroups(in, notes, q)
	default:
		list.Groups = levelGroups(in, notes, q)
	}
	if q.Filter == "" {
		list.Groups = append(list.Groups, otherFilesGroup(others, in.Lang)...)
	}
	list.Wide = q.Sort != SortByRecent && (len(notes)+folders > manyRows || q.Expand && folders > 0)
	if folders == 0 {
		foldLoneHeading(&list)
	}
	v.List = list
	return v
}

// folderToggle is the link that opens a folder's folders one level, or closes
// them again.
func folderToggle(in *FolderInput) ListAction {
	q, word := in.Query, wording.ExpandFolders
	if q.Expand {
		word = wording.FoldFolders
	}
	return ListAction{Text: word.In(in.Lang), Href: folderHref(in.Dir) + q.Address(q.Sort, q.Filter, !q.Expand)}
}

// folderBar is the controls over a folder's list: the filter once the folder
// holds many notes, and the order. Opening the folders sits on the head of the
// folders' own group; the bar carries it only when that group is not drawn.
func folderBar(in *FolderInput, notes, folders, total int, ordered bool) List {
	q, base := in.Query, folderHref(in.Dir)
	list := List{Empty: wording.FolderEmpty.In(in.Lang)}
	if notes > manyRows || (q.Expand && total > manyRows) {
		list.Filter = &ListFilter{
			Hint:   strings.Replace(wording.FilterNotes.In(in.Lang), "%s", Thousands(total), 1),
			Value:  q.Filter,
			Action: base,
		}
		if q.Sort != q.defaultSort() {
			list.Keep = append(list.Keep, ListParam{"sort", string(q.Sort)})
		}
		if q.Expand {
			list.Keep = append(list.Keep, ListParam{"expand", "1"})
		}
	}
	if folders > 0 && (q.Expand || q.Filter != "") {
		list.Toggle = folderToggle(in)
	}
	if ordered || notes+folders > distinguishingRows {
		name := wording.SortByTitle
		if notes == 0 {
			name = wording.SortByName
		}
		list.Switch = q.ListSwitchFor(base, q.Expand, ordered, name, in.Lang)
	}
	return list
}

// foldLoneHeading lets a lone group's heading stand at the head of the bar, so
// the list is not headed twice and its rule runs across the whole list, and
// drops the count line that would say the same number again.
func foldLoneHeading(list *List) {
	if len(list.Groups) != 1 || list.Label != "" {
		return
	}
	g := &list.Groups[0]
	if g.Heading == "" {
		return
	}
	if list.Filter == nil && len(list.Switch.Choices) == 0 {
		if g.Count != "" {
			list.Count = ""
		}
		return
	}
	list.Label, list.LabelCount, list.LabelAuthored = g.Heading, g.Count, g.Authored
	list.Ruled = true
	g.Heading, g.Count = "", ""
	list.Count = ""
}

// folderCrumbs is the trail above a folder: the vault, then each parent.
func folderCrumbs(dir, vaultName string) []nav.NoteRef {
	var crumbs []nav.NoteRef
	if vaultName != "" {
		crumbs = append(crumbs, nav.NoteRef{Name: vaultName, RelPath: ""})
	}
	return append(crumbs, Breadcrumb(dir)...)
}

// listNote reads a note as the list rule sees it.
func listNote(ref nav.NoteRef, in *FolderInput) ListNote {
	listed := in.Model.Listed(ref.RelPath)
	language := ref.Language
	if language == "" && in.ArticleLang != nil {
		language = in.ArticleLang(ref.RelPath)
	}
	return ListNote{Title: ref.Name, RelPath: ref.RelPath, Language: language, Draft: listed.Draft, Modified: listed.Modified}
}

func listNotes(refs []nav.NoteRef, in *FolderInput) []ListNote {
	notes := make([]ListNote, len(refs))
	for i := range refs {
		notes[i] = listNote(refs[i], in)
	}
	return notes
}

func listFolders(folders []nav.Folder, in *FolderInput) []ListFolder {
	out := make([]ListFolder, 0, len(folders))
	for i := range folders {
		if n := folders[i].NoteCount(); n > 0 {
			out = append(out, ListFolder{
				Name: folders[i].Name, RelPath: folders[i].RelPath,
				Notes: n, Latest: in.Model.Latest(&folders[i]),
			})
		}
	}
	return out
}

// levelGroups is the folder opened as it stands: the folders it holds, then
// its notes.
func levelGroups(in *FolderInput, notes []nav.NoteRef, q ListQuery) []ListGroup {
	var groups []ListGroup
	folders := listFolders(in.Subfolders, in)
	if len(folders) > 0 {
		groups = append(groups, ListGroup{
			Heading: wording.Folders.In(in.Lang),
			Count:   Thousands(len(folders)),
			Rows:    FolderRows(folders, q.Sort, in.Now, in.Lang),
			Action:  folderToggle(in),
		})
	}
	noteGroups := NoteRows(listNotes(notes, in), q.Sort, in.Now, in.Lang)
	if len(groups) > 0 && len(noteGroups) > 0 && noteGroups[0].Heading == "" {
		noteGroups[0].Heading = wording.NotesGroup.In(in.Lang)
		noteGroups[0].Count = Thousands(len(noteGroups[0].Rows))
	}
	return append(groups, noteGroups...)
}

// expandedGroups opens each folder one level: its own notes under the folder's
// name, and a row for each folder below it.
func expandedGroups(in *FolderInput, notes []nav.NoteRef, q ListQuery) []ListGroup {
	own := listNotes(notes, in)
	subs := listFolders(in.Subfolders, in)
	all := slices.Clone(own)
	for i := range in.Subfolders {
		all = append(all, listNotes(in.Subfolders[i].Notes, in)...)
	}
	showDate := q.Sort == SortByRecent || datesDistinguish(all)
	read := func(refs []nav.NoteRef) []ListRow {
		noteRefs, _ := splitNotesAndFiles(refs)
		held := listNotes(noteRefs, in)
		orderNotes(held, q.Sort)
		return noteListRows(held, showDate, in.Now, in.Lang)
	}
	var groups []ListGroup
	if len(own) > 0 {
		groups = append(groups, ListGroup{Heading: wording.TopLevel.In(in.Lang), Count: noteCountFact(len(own), in.Lang), Rows: read(notes)})
	}
	byPath := make(map[string]*nav.Folder, len(in.Subfolders))
	for i := range in.Subfolders {
		byPath[in.Subfolders[i].RelPath] = &in.Subfolders[i]
	}
	for _, sub := range SortFolders(subs, q.Sort) {
		f := byPath[sub.RelPath]
		g := ListGroup{Heading: f.Name, Href: folderHref(f.RelPath), Authored: true, Count: noteCountFact(sub.Notes, in.Lang), Rows: read(f.Notes)}
		deeper := FolderRows(listFolders(f.Subfolders, in), q.Sort, in.Now, in.Lang)
		for i := range deeper {
			deeper[i].Folder = true
		}
		g.Rows = append(g.Rows, deeper...)
		groups = append(groups, g)
	}
	return groups
}

// foundNotes are the notes of one directory whose title holds the filter.
type foundNotes struct {
	dir   string
	notes []ListNote
}

// findNotes walks the folder at every depth and keeps, per directory, the
// notes whose title holds the word, the directories in the order first met.
func findNotes(in *FolderInput, word string) (dirs []string, byDir map[string]*foundNotes) {
	byDir = map[string]*foundNotes{}
	add := func(dir string, refs []nav.NoteRef) {
		for _, ref := range refs {
			if !vault.IsMarkdown(ref.RelPath) {
				continue
			}
			n := listNote(ref, in)
			if !(&ListRow{Title: n.Title}).mark(word) {
				continue
			}
			f := byDir[dir]
			if f == nil {
				f = &foundNotes{dir: dir}
				byDir[dir] = f
				dirs = append(dirs, dir)
			}
			f.notes = append(f.notes, n)
		}
	}
	add("", in.Notes)
	var walk func(folders []nav.Folder)
	walk = func(folders []nav.Folder) {
		for i := range folders {
			add(strings.TrimPrefix(folders[i].RelPath, in.Dir+"/"), folders[i].Notes)
			walk(folders[i].Subfolders)
		}
	}
	walk(in.Subfolders)
	return dirs, byDir
}

// filterGroups finds the notes under this folder, at every depth, whose title
// holds the word, grouped by the folder each sits in.
func filterGroups(in *FolderInput, q ListQuery) []ListGroup {
	dirs, byDir := findNotes(in, q.Filter)
	var every []ListNote
	for _, f := range byDir {
		every = append(every, f.notes...)
	}
	showDate := q.Sort == SortByRecent || datesDistinguish(every)
	c := nameCollator()
	slices.SortFunc(dirs, func(a, b string) int {
		if a == "" || b == "" {
			return strings.Compare(a, b)
		}
		return c.CompareString(a, b)
	})
	var groups []ListGroup
	for _, dir := range dirs {
		f := byDir[dir]
		orderNotes(f.notes, q.Sort)
		rows := noteListRows(f.notes, showDate, in.Now, in.Lang)
		for i := range rows {
			rows[i].mark(q.Filter)
		}
		g := ListGroup{Rows: rows, Count: Thousands(len(rows))}
		if len(dirs) > 1 || dir != "" {
			g.Heading = lastTwo(dir)
			if dir == "" {
				g.Heading = wording.TopLevel.In(in.Lang)
			} else {
				g.Authored = true
				g.Href = folderHref(path2(in.Dir, dir))
			}
		}
		groups = append(groups, g)
	}
	return groups
}

func path2(dir, sub string) string {
	if dir == "" {
		return sub
	}
	return dir + "/" + sub
}

// lastTwo keeps the last two levels of a path, joined by an arrow: a deeper
// path reads as where the note sits rather than as a full address.
func lastTwo(dir string) string {
	segs := strings.Split(dir, "/")
	if len(segs) > 2 {
		segs = segs[len(segs)-2:]
	}
	return strings.Join(segs, " › ")
}

// otherFilesGroup lists the files that are not notes, each by its own name.
func otherFilesGroup(files []nav.NoteRef, lang wording.Lang) []ListGroup {
	if len(files) == 0 {
		return nil
	}
	rows := make([]ListRow, len(files))
	for i, f := range files {
		rows[i] = ListRow{Title: f.Name, Href: notesHref(f.RelPath)}
	}
	return []ListGroup{{Heading: wording.OtherFiles.In(lang), Count: Thousands(len(rows)), Rows: rows}}
}

// crumbHref is where a crumb leads: the vault's own crumb has no folder and
// goes to the index of folders.
func crumbHref(relPath string) string {
	if relPath == "" {
		return indexHref(folderMode)
	}
	return folderHref(relPath)
}
