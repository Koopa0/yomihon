package search

import (
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// panelFolders is how many folders a panel offers after its notes.
const panelFolders = 2

// titleLead is how far into a title a hit may sit before the panel cuts the
// title to start near it, so the hit is never the part a one-line row loses.
const titleLead = 14

// panelView is the answer as a field shows it: up to six notes, the folders
// whose names match, and the way to every note the query has.
func panelView(snap *RequestSnapshot, a *answer, q string) pages.SearchPanelView {
	view := pages.SearchPanelView{Query: strings.TrimSpace(q), Total: a.found.Total, Diagnostic: a.diagnostic != ""}
	if view.Query == "" || view.Diagnostic {
		return view
	}
	view.AllHref = pages.SearchState{Query: q}.Address()
	for i := range a.found.Results {
		r := &a.found.Results[i]
		view.Notes = append(view.Notes, pages.SearchPanelNote{
			TitleRuns: titleRuns(r.Title, a.tokens),
			Folder:    lastLevels(r.RelPath, 1),
			Href:      pages.VaultHref("/notes/", r.RelPath),
			Language:  r.Language,
		})
	}
	if snap.Shell.Nav != nil {
		view.Folders = matchingFolders(snap.Shell.Nav.Folders(), "", a.tokens)
	}
	if view.Total == 0 {
		if word := snap.Index.NearWord(q); word != "" {
			view.Near = pages.SearchNear{Word: word, Href: pages.SearchState{Query: word}.Address()}
		}
	}
	return view
}

// matchingFolders walks the folder tree for names the tokens all appear in,
// shallowest first, and stops at what a panel shows.
func matchingFolders(folders []nav.Folder, parent string, tokens []string) []pages.SearchPanelFolder {
	var out []pages.SearchPanelFolder
	var walk func(level []nav.Folder, parent string)
	walk = func(level []nav.Folder, parent string) {
		for i := range level {
			f := &level[i]
			if len(out) < panelFolders && f.NoteCount() > 0 && namesAll(f.Name, tokens) {
				out = append(out, pages.SearchPanelFolder{
					NameRuns: plainRuns(f.Name, tokens),
					Parent:   parent,
					Count:    f.NoteCount(),
					Href:     pages.VaultHref("/folders/", f.RelPath),
				})
			}
		}
		for i := range level {
			if len(out) < panelFolders {
				walk(level[i].Subfolders, level[i].Name)
			}
		}
	}
	if len(tokens) > 0 {
		walk(folders, parent)
	}
	return out
}

func namesAll(name string, tokens []string) bool {
	folded := strings.ToLower(name)
	for _, t := range tokens {
		if t == "" || !strings.Contains(folded, strings.ToLower(t)) {
			return false
		}
	}
	return true
}

// titleRuns cuts a title into hit and plain runs, opening it near the first
// hit with an ellipsis when the hit sits too far in for one line to show it.
func titleRuns(title string, tokens []string) []pages.SnippetRun {
	runs := plainRuns(title, tokens)
	before := 0
	for i, run := range runs {
		if run.Hit {
			if before <= titleLead {
				return runs
			}
			keep := []rune(runs[i-1].Text)
			cut := max(0, len(keep)-6)
			for cut > 0 && keep[cut-1] != ' ' && utf8.RuneLen(keep[cut-1]) == 1 {
				cut--
			}
			runs[i-1].Text = "…" + string(keep[cut:])
			return runs[i-1:]
		}
		before += utf8.RuneCountInString(run.Text)
	}
	return runs
}
