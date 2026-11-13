package note

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// thought offers text to an external editor; it never creates a vault file.
func (h *Handler) thought(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	snap := h.sources.Snapshot().Capture()
	authority := h.sources.Authority()
	rel := vault.NormalizeNFC(r.PathValue("path"))
	role := snap.NavigationRoles().AnswerType()
	n, ok := readableNote(snap, rel)
	if !ok || role == "" || !thoughtSourceAvailable(snap, rel) {
		h.showNotFound(w, r, rel, authority, snap)
		return
	}
	section := r.URL.Query().Get("section")
	label := n.Title
	sectionText := ""
	if section != "" {
		result := snap.Render(rel, n.Body, lang)
		found := section == result.TitleAnchor
		if found {
			sectionText = n.Title
		}
		for _, heading := range result.TOC {
			if heading.ID == section {
				label = heading.Text
				sectionText = heading.Text
				found = true
				break
			}
		}
		if !found {
			h.showNotFound(w, r, rel, authority, snap)
			return
		}
	}
	// The name carries the minute, so two thoughts on one source differ unless
	// they start within the same minute. Obsidian owns the folder and what
	// happens on a clash; this suggestion neither probes nor writes disk. The
	// note is titled by its own name, so the title a contract may require and
	// the file the reader saves read the same.
	name := thoughtNoteName(n.Title, lang, time.Now())
	markdown := thoughtMarkdown(role, &n, sectionText)
	view := pages.ThoughtView{
		SourceTitle: label,
		SourceHref:  pages.ResumeHref(rel, section, 0),
		Markdown:    markdown,
		EditorHref:  obsidianNewHref(h.sources.Source, name, markdown),
	}
	if err := pages.Thought(view, layouts.ChromeFromRequest(r, wording.ThoughtDoor.In(lang))).Render(r.Context(), w); err != nil {
		h.sources.Log.Error("render thought stub", "error", err)
	}
}

// obsidianOpenHref is the link that opens a note in Obsidian, or "" when the
// vault is not one Obsidian has opened.
func obsidianOpenHref(source *vault.Reader, rel string) string {
	if !source.HasObsidianFolder() {
		return ""
	}
	return pages.ObsidianHref(source.Name(), rel)
}

// obsidianNewHref is the link that starts a note in Obsidian, or "" when the
// vault is not one Obsidian has opened.
func obsidianNewHref(source *vault.Reader, name, content string) string {
	if !source.HasObsidianFolder() {
		return ""
	}
	return pages.ObsidianNewHref(source.Name(), name, content)
}

// thoughtSourceAvailable requires the generated citation to name this file.
func thoughtSourceAvailable(snap *snapshot.Generation, rel string) bool {
	target := strings.TrimSuffix(rel, ".md")
	if strings.ContainsAny(target, "#^|[]") {
		return false
	}
	parsed, valid := graph.ParseWikilink(target)
	if !valid {
		return false
	}
	resolved := snap.Graph().Resolve(parsed.Target)
	return resolved.Kind == graph.KindUnique && resolved.RelPath == rel
}

// thoughtMarkdown is the frontmatter a reader's thought starts with: its role
// type, the draft flag, and the citation of the source. The note's title is not
// written: it is the name the file is saved under, and the heading the reader
// writes under it. sectionText is the section heading as the reader sees it:
// Obsidian matches a heading link by its text, not by the id this site gives it.
func thoughtMarkdown(role string, source *snapshot.Reading, sectionText string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: " + strconv.Quote(role) + "\n")
	// A thought starts unfinished: the reader removes the flag in the editor
	// once it is written, and until then the shelf of what they left open
	// lists it.
	b.WriteString(schema.DraftField + ": true\n")
	b.WriteString("based_on: " + strconv.Quote("[["+thoughtCitation(source.RelPath, sectionText)+"]]") + "\n---\n")
	return b.String()
}

// thoughtCitation is the link target, without its brackets, that names rel and
// the section whose heading reads sectionText: the vault's own path without the
// note extension, then the heading as Obsidian matches it. An empty section
// names the whole note.
func thoughtCitation(rel, sectionText string) string {
	target := strings.TrimSuffix(rel, ".md")
	if heading := headingLinkText(sectionText); heading != "" {
		target += "#" + heading
	}
	return target
}

// headingLinkText makes heading text safe inside a wikilink. Characters that
// end or split a link target are written as spaces, as Obsidian does when it
// composes a heading link itself.
func headingLinkText(text string) string {
	cleaned := strings.Map(func(r rune) rune {
		if strings.ContainsRune("#|[]^:", r) {
			return ' '
		}
		return r
	}, text)
	return strings.Join(strings.Fields(cleaned), " ")
}

// maxThoughtTitleRunes bounds the source title inside a thought's file name so
// the whole name stays well under the 255-byte limit file systems share, even
// when every character is three bytes wide.
const maxThoughtTitleRunes = 60

// thoughtNoteName is the readable file name a reader's thought starts with:
// the source title, a word for what the note is, and the minute it was begun.
// The name has no folder and no extension; Obsidian places the file by the
// reader's own setting for new notes and adds the extension itself.
func thoughtNoteName(sourceTitle string, lang wording.Lang, at time.Time) string {
	title := fileNameText(sourceTitle)
	if runes := []rune(title); len(runes) > maxThoughtTitleRunes {
		title = strings.TrimSpace(string(runes[:maxThoughtTitleRunes]))
	}
	name := fmt.Sprintf(wording.ThoughtNoteNameFmt.In(lang), title, at.Format("2006-01-02 1504"))
	return strings.Join(strings.Fields(name), " ")
}

// fileNameText makes a title safe as part of a file name. The characters
// Obsidian refuses in a name on some platform (\ / : * " < > ? |), and those it
// refuses everywhere because they break links ([ ] # ^ |), become spaces, as do
// control characters; a leading dot, which would hide the file, is dropped.
// Obsidian publishes no such list, so this one is the union its own file-name
// check shows for the three desktop platforms.
func fileNameText(text string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`\/:*"<>?|[]#^`, r) {
			return ' '
		}
		return r
	}, text)
	return strings.TrimLeft(strings.Join(strings.Fields(cleaned), " "), ".")
}
