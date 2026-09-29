package note

import (
	"crypto/rand"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

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
	role := h.sources.Contract.AnswerType()
	n, ok := readableNote(snap, rel)
	if !ok || role == "" || !thoughtSourceAvailable(snap, rel) {
		h.showNotFound(w, r, rel, authority, snap)
		return
	}
	section := r.URL.Query().Get("section")
	label := n.Title
	if section != "" {
		result := snap.Render(rel, n.Body, lang)
		found := section == result.TitleAnchor
		for _, heading := range result.TOC {
			if heading.ID == section {
				label = heading.Text
				found = true
				break
			}
		}
		if !found {
			h.showNotFound(w, r, rel, authority, snap)
			return
		}
	}
	markdown := thoughtMarkdown(h.sources.Contract, role, &n, section)
	// A second thought must not target the first one's file. The editor owns
	// the final name; this bounded suggestion neither probes nor writes disk.
	destination := path.Join(path.Dir(rel), "thought-"+rand.Text()+".md")
	view := pages.ThoughtView{
		SourceTitle: label,
		SourceHref:  pages.ResumeHref(rel, section, 0),
		Markdown:    markdown,
		EditorHref:  pages.ObsidianNewHref(h.sources.Source.Name(), destination, markdown),
	}
	if err := pages.Thought(view, layouts.ChromeFromRequest(r, wording.ThoughtDoor.In(lang))).Render(r.Context(), w); err != nil {
		h.sources.Log.Error("render thought stub", "error", err)
	}
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
func thoughtMarkdown(contract *schema.Contract, role string, source *snapshot.Reading, section string) string {
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
		if contract.StartsAt(role, status) {
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
	if section != "" {
		target += "#" + section
	}
	b.WriteString("based_on: " + strconv.Quote("[["+target+"]]") + "\n---\n")
	return b.String()
}
