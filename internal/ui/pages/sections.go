package pages

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/a-h/templ"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/wording"
)

// SectionPlan is what the reading page draws beside each section of a note's
// text: the reader's own controls and the record of what they left there before.
// The rendered body is the author's and carries none of it, so the plan is laid
// over the finished markup rather than written into the renderer.
type SectionPlan struct {
	// Lang is the language the interface speaks. The article around the margin
	// speaks the note's own, so everything drawn in it says which it is.
	Lang wording.Lang
	// RelPath is the note whose text this is, and IDPrefix the id space its
	// headings occupy on the page: empty beside one note, a column's name where
	// two share a document.
	RelPath  string
	IDPrefix string
	// Marks is whether this process keeps the reader's "Not sure yet" marks, and
	// so whether a section offers the control at all.
	Marks bool
	// Door is whether a thought can be started from this note. It gates the
	// link beside each section and the line above a question fold.
	Door bool
	// Sections holds the sections to draw, by the id each heading carries.
	Sections map[string]SectionMargin
	// PlaceAnchor is the id of the element the reader left off at, empty where
	// their place is not in this note. The section holding it is found in the
	// body itself, since a place can be any anchor and not only a heading.
	PlaceAnchor string

	// place is the id of the section that holds the place, found by
	// InjectSections.
	place string
}

// SectionMargin is what the reader already left at one section.
type SectionMargin struct {
	// Text is the heading's own words, which name the link that starts a thought
	// about it.
	Text string
	// Marked is whether the reader marked the section "Not sure yet", and
	// MarkedOn the day they did.
	Marked   bool
	MarkedOn time.Time
	// Thoughts are the reader's own notes whose source is this section.
	Thoughts []ThoughtLine
}

// ThoughtLine is one thought of the reader's own, listed beside what it answers:
// where to open it, its first sentence in the words it was written in, and the
// day it was begun.
type ThoughtLine struct {
	Href     string
	Opening  string
	Language string
	On       time.Time
}

// drawn reports whether a section gets a margin of its own. Where the page
// offers neither the mark nor the link, only a section that holds something the
// reader left, or the place they left off at, has anything to show.
func (p *SectionPlan) drawn(id string, m *SectionMargin) bool {
	return p.Marks || p.Door || len(m.Thoughts) > 0 || (id != "" && id == p.place)
}

// bare reports whether the plan draws nothing at all, so the body can be handed
// back untouched.
func (p *SectionPlan) bare() bool {
	if p == nil {
		return true
	}
	for id := range p.Sections {
		m := p.Sections[id]
		if p.drawn(id, &m) {
			return false
		}
	}
	return !p.Door
}

// InjectSections lays the plan over a rendered body. Each heading the plan names
// is wrapped with its margin, which follows the heading directly so the
// controls are the next things a keyboard reaches after the section's name, and
// a closed question fold gets one line above it saying a guess is welcome
// before the answer opens. Every other byte of the body is copied as it came.
// A margin that cannot be drawn fails the whole: the body is returned as it
// came, with the error, so the page reads the same without any of it.
func InjectSections(ctx context.Context, body string, plan *SectionPlan) (string, error) {
	if plan == nil {
		return body, nil
	}
	plan.place = holdingSection(body, plan)
	if plan.bare() {
		return body, nil
	}
	w := &weave{plan: plan}
	w.out.Grow(len(body) + 256*len(plan.Sections))
	tokens := html.NewTokenizer(strings.NewReader(body))
	for {
		kind := tokens.Next()
		raw := bytes.Clone(tokens.Raw())
		if kind == html.ErrorToken {
			w.out.Write(raw)
			break
		}
		if kind == html.StartTagToken && w.start(ctx, tokens.Token(), raw) {
			continue
		}
		if kind == html.EndTagToken && w.end(ctx, tokens.Token(), raw) {
			continue
		}
		w.out.Write(raw)
	}
	if w.err != nil {
		return body, w.err
	}
	return w.out.String(), nil
}

// weave is the body being assembled and where the cursor stands in it.
type weave struct {
	plan *SectionPlan
	out  strings.Builder
	err  error
	// section is the section the cursor is under, empty above the first.
	section string
	// open is the heading tag being copied and openID its id, both empty outside
	// one.
	open, openID string
}

// start handles an opening tag, and reports whether it wrote the tag itself.
func (w *weave) start(ctx context.Context, token html.Token, raw []byte) bool {
	if id, ok := planned(w.plan, &token, w.open); ok {
		w.section = id
		margin := w.plan.Sections[id]
		if !w.plan.drawn(id, &margin) {
			return false
		}
		w.out.WriteString(`<div class="y-sect">`)
		w.out.Write(raw)
		w.open, w.openID = token.Data, id
		return true
	}
	if w.plan.Door && closedQuestion(&token) {
		w.draw(ctx, predictPrompt(thoughtHref(w.plan.RelPath, trimID(w.section, w.plan.IDPrefix)), w.plan.Lang))
	}
	return false
}

// end handles a closing tag, and reports whether it wrote the tag itself: the
// one that closes a heading being copied, behind which its margin follows.
func (w *weave) end(ctx context.Context, token html.Token, raw []byte) bool {
	if w.open == "" || token.Data != w.open {
		return false
	}
	w.out.Write(raw)
	margin := w.plan.Sections[w.openID]
	w.draw(ctx, sectionMargin(w.openID, &margin, w.plan))
	w.out.WriteString(`</div>`)
	w.open = ""
	return true
}

// draw renders one component into the body being assembled, keeping the first
// failure.
func (w *weave) draw(ctx context.Context, c templ.Component) {
	if err := c.Render(ctx, &w.out); err != nil && w.err == nil {
		w.err = fmt.Errorf("drawing the margin beside a section: %w", err)
	}
}

// holdingSection is the id of the section the element carrying the plan's place
// anchor stands in: the heading itself, or the last heading above it. It is
// empty where the anchor is not in the body or stands above the first section.
func holdingSection(body string, plan *SectionPlan) string {
	if plan.PlaceAnchor == "" {
		return ""
	}
	tokens := html.NewTokenizer(strings.NewReader(body))
	section := ""
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			return ""
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := tokens.Token()
		if id, ok := planned(plan, &token, ""); ok {
			section = id
		}
		if idOf(&token) == plan.PlaceAnchor {
			return section
		}
	}
}

// HoldsClosedQuestion reports whether a section of a rendered note holds a
// question fold that starts closed: the section named by the heading id, with
// every section under it, or the whole note where the id is empty. It reads the
// same markup the page lays its prompt over, so the line a thought draft opens
// with and the line the page shows above the fold are never decided apart.
func HoldsClosedQuestion(body, section string) bool {
	tokens := html.NewTokenizer(strings.NewReader(body))
	scope := questionScope{section: section, inside: section == ""}
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			return false
		}
		if kind != html.StartTagToken {
			continue
		}
		if verdict, decided := scope.see(tokens.Token()); decided {
			return verdict
		}
	}
}

// questionScope follows which section the cursor is in while a body is read for
// a closed question: inside from the heading it names until the next heading of
// the same or a higher level.
type questionScope struct {
	section string
	inside  bool
	level   int
}

// see takes one opening tag and says whether the answer is decided and what it is.
func (q *questionScope) see(token html.Token) (verdict, decided bool) {
	if _, authored := headingFacts(&token); isHeadingTag(token.Data) && authored != 0 {
		id := idOf(&token)
		switch {
		case q.section != "" && id == q.section:
			q.inside, q.level = true, authored
		case q.section != "" && q.inside && authored <= q.level:
			return false, true
		}
		return false, false
	}
	found := q.inside && closedQuestion(&token)
	return found, found
}

// isHeadingTag reports whether a tag name is one of the six heading elements.
func isHeadingTag(name string) bool {
	return len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6'
}

// idOf is the id a tag carries, by the browser's first-id rule.
func idOf(token *html.Token) string {
	for _, attr := range token.Attr {
		if attr.Key == "id" {
			return attr.Val
		}
	}
	return ""
}

// headingFacts are the id a renderer-stamped heading carries and the level its
// author wrote, which is zero for a tag that is not one of the renderer's.
func headingFacts(token *html.Token) (id string, authored int) {
	id = idOf(token)
	for _, attr := range token.Attr {
		if attr.Key == "data-level" && len(attr.Val) == 1 && attr.Val[0] >= '1' && attr.Val[0] <= '6' {
			authored = int(attr.Val[0] - '0')
		}
	}
	return id, authored
}

// planned reports the id of a heading the plan has a section for. Only the
// renderer's own headings qualify: they carry the authored level, which no
// markup an author could write does.
func planned(plan *SectionPlan, token *html.Token, open string) (string, bool) {
	if open != "" || !isHeadingTag(token.Data) {
		return "", false
	}
	id, authored := headingFacts(token)
	if _, named := plan.Sections[id]; authored == 0 || !named {
		return "", false
	}
	return id, true
}

// closedQuestion reports whether a tag opens a foldable callout of the question
// type that begins shut, which is the one place a reader can be asked, gently,
// to guess before the answer is there to be read.
func closedQuestion(token *html.Token) bool {
	if token.Data != "details" {
		return false
	}
	question, shut := false, true
	for _, attr := range token.Attr {
		switch attr.Key {
		case "data-callout":
			question = attr.Val == "question"
		case "open":
			shut = false
		}
	}
	return question && shut
}

// trimID is a heading's id without the id space of the column it stands in,
// which is the spelling the address of the note itself uses for it.
func trimID(id, prefix string) string { return strings.TrimPrefix(id, prefix) }

// dateOf is the day a mark or a thought was made, as the shelf prints it.
func dateOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}
