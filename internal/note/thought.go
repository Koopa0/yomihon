package note

import (
	"fmt"
	"net/http"
	"slices"
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
	authority := h.sources.Status()
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
	markdown := thoughtMarkdown(h.sources.Contract, role, &n, sectionText)
	// The name carries the minute, so two thoughts on one source differ unless
	// they start within the same minute. Obsidian owns the folder and what
	// happens on a clash; this suggestion neither probes nor writes disk.
	name := thoughtNoteName(n.Title, lang, time.Now())
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

// thoughtMarkdown omits values that the source and contract cannot determine.
// sectionText is the section heading as the reader sees it: Obsidian matches a
// heading link by its text, not by the id this site gives it.
func thoughtMarkdown(contract *schema.Contract, role string, source *snapshot.Reading, sectionText string) string {
	definition := contract.Definition()
	var b strings.Builder
	b.WriteString("---\n")
	write := func(field, value string) {
		if value != "" && slices.Contains(definition.Fields.Known, field) {
			b.WriteString(field + ": " + strconv.Quote(value) + "\n")
		}
	}
	write("type", role)
	var initial []string
	for _, status := range contract.Statuses(role) {
		if contract.DeclaresInitial(role, status) {
			initial = append(initial, status)
		}
	}
	if len(initial) == 1 {
		write("status", initial[0])
	}
	if slices.Contains(definition.Enums.Domain, source.Domain) && !slices.Contains(definition.Fields.DomainExempt, role) {
		write("domain", source.Domain)
	}
	target := strings.TrimSuffix(source.RelPath, ".md")
	if heading := headingLinkText(sectionText); heading != "" {
		target += "#" + heading
	}
	b.WriteString("based_on: " + strconv.Quote("[["+target+"]]") + "\n---\n")
	return b.String()
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
