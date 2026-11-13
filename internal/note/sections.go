package note

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// maxOpeningRunes bounds the first sentence of a thought shown beside the text it
// answers, so one long run of words does not become a column of its own.
const maxOpeningRunes = 120

// sectionsIn lays the reader's own record over a note's rendered text: beside
// each section the controls, what the reader marked there and the day, and the
// first sentence and the day of every thought of theirs whose source is that
// section; beside the title the thoughts about the note as a whole. It returns
// the body with the margins in it and the title's thoughts, which the page draws
// itself.
//
// Everything it shows is read from this generation and the reader's own marks,
// so the page says on arrival what a second reading would find, without a script.
func (h *Handler) sectionsIn(ctx context.Context, snap *snapshot.Generation, n *snapshot.Reading, result *render.Result, idPrefix string, lang wording.Lang, door bool, place *mark.Continuation) (body string, title []pages.ThoughtLine) {
	plan := &pages.SectionPlan{
		Lang:     lang,
		RelPath:  n.RelPath,
		IDPrefix: idPrefix,
		Marks:    h.sources.UncertaintyAddress != "",
		Door:     door,
		Sections: make(map[string]pages.SectionMargin),
	}
	marked := h.marksIn(n.RelPath)
	for _, entry := range result.Outline() {
		at, isMarked := marked[strings.TrimPrefix(entry.ID, idPrefix)]
		plan.Sections[entry.ID] = pages.SectionMargin{Text: entry.Text, Marked: isMarked, MarkedOn: at}
	}
	if place != nil && place.Anchor != "" {
		plan.PlaceAnchor = idPrefix + place.Anchor
	}
	title = h.thoughtsAbout(snap, n, plan, idPrefix)
	body, err := pages.InjectSections(ctx, result.HTML, plan)
	if err != nil {
		// The page reads the same without the margins, so it is served without them.
		h.sources.Log.Error("draw the margins beside the sections", "path", n.RelPath, "error", err)
		return result.HTML, title
	}
	return body, title
}

// marksIn is when the reader marked each section of this note "Not sure yet",
// by the anchor each mark names. A mark file that cannot be read gives none: the
// page reads the same without them, and the script that keeps the marks asks the
// file again and says so when it cannot answer either.
func (h *Handler) marksIn(rel string) map[string]time.Time {
	if h.sources.Uncertainties == nil {
		return nil
	}
	kept, err := h.sources.Uncertainties()
	if err != nil {
		return nil
	}
	marked := make(map[string]time.Time)
	for i := range kept {
		if kept[i].RelPath == rel {
			marked[kept[i].Anchor] = kept[i].At
		}
	}
	return marked
}

// thoughtsAbout files the reader's own thoughts under what they are about. A
// thought is a note of the contract's thought type that names this note in
// based_on: with no section it is about the note as a whole, which is the
// returned list, and with a section it stands in that section's margin in the
// plan. A thought about a block, or about a section the note no longer has, is
// left where it is: the first has no heading to stand beside, and the second
// names a place nobody can see.
func (h *Handler) thoughtsAbout(snap *snapshot.Generation, n *snapshot.Reading, plan *pages.SectionPlan, idPrefix string) (whole []pages.ThoughtLine) {
	role := snap.NavigationRoles().AnswerType()
	if role == "" {
		return nil
	}
	for _, declaration := range snap.DeclaredBy(n.RelPath) {
		thought, ok := snap.Note(declaration.Note.RelPath)
		if !ok || schema.NormalizeWord(thought.Type) != role {
			continue
		}
		line := pages.ThoughtLine{
			Href:     pages.ResumeHref(thought.RelPath, "", 0),
			Opening:  cmp.Or(thoughtOpening(thought.Body), thought.Title),
			Language: thought.Language,
			On:       thought.Created,
		}
		for _, place := range declaration.Locations {
			switch {
			case place.Block != "":
			case place.Heading == "":
				whole = append(whole, line)
			default:
				id := idPrefix + graph.SectionID(place.Heading)
				if section, named := plan.Sections[id]; named {
					section.Thoughts = append(section.Thoughts, line)
					plan.Sections[id] = section
				}
			}
		}
	}
	for id, section := range plan.Sections {
		newestFirst(section.Thoughts)
		plan.Sections[id] = section
	}
	newestFirst(whole)
	return whole
}

// newestFirst orders thoughts by the day each was begun, latest first; those with
// no day, and those begun on the same one, keep to the order their notes sort in.
func newestFirst(lines []pages.ThoughtLine) {
	slices.SortStableFunc(lines, func(a, b pages.ThoughtLine) int {
		return cmp.Or(b.On.Compare(a.On), vault.ComparePaths(a.Href, b.Href))
	})
}

// thoughtOpening is the first sentence of what the reader wrote in a thought,
// read from its current text: the line a draft opens with to say a guess was
// written before the answer is skipped, being the page's words and not theirs.
// A thought with nothing written yet has no opening.
func thoughtOpening(body string) string {
	for line := range strings.SplitSeq(render.PlainText(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || wording.ThoughtBeforeAnswer.Says(line) {
			continue
		}
		return firstSentence(line)
	}
	return ""
}

// firstSentence cuts a line at its first sentence end: a Chinese or Japanese
// stop, or a Latin one followed by a space or the end of the line, so a version
// number is not read as two sentences. A sentence longer than the bound is cut at
// it and marked.
func firstSentence(line string) string {
	runes := []rune(line)
	end := len(runes)
	for i, r := range runes {
		if strings.ContainsRune("。！？", r) || (strings.ContainsRune(".!?", r) && (i+1 == len(runes) || unicode.IsSpace(runes[i+1]))) {
			end = i + 1
			break
		}
	}
	sentence := strings.TrimSpace(string(runes[:end]))
	if utf8.RuneCountInString(sentence) <= maxOpeningRunes {
		return sentence
	}
	return strings.TrimSpace(string([]rune(sentence)[:maxOpeningRunes])) + "…"
}
