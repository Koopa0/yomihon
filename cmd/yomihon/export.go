package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/wording"
)

// marks --export is the command that hands the reader's own words to a
// program: every mark, as a quotation of the place it is from and what the
// reader wrote there, in Markdown, to the standard output. The reader typing
// the flag is the consent, so there is no second switch. Without it the
// command never prints a word the reader wrote.
//
// It is built on the same gate as the list and does not share the list's
// refusal for a note it cannot read. That refusal is there because an unread
// note could be a thought still open; the export lists only what the machine
// files hold, and the gate asks only a path and words, so a note that cannot
// be read does not make its marks disappear.

const (
	// exportTitle is the one line of a heading the export has of its own. The
	// command line speaks English only, so these are not in the wording package;
	// the words of the groups, which a page copying a group says too, are.
	exportTitle = "# Marks"
	// exportNone is all an export of a vault with no marks says after its title.
	exportNone = "No marks."

	// exportIncomplete is the line for a marks file that could not be read,
	// said in place of a count, since a count would say how much sits where it
	// cannot be seen. The rest of the marks are still printed.
	exportIncomplete = "Some marks could not be read; this export may be incomplete."

	// The words of the other values of a mark. They say what a value is and when
	// it was written, and nothing of which computer wrote it.
	exportOtherOne   = "Other version:"
	exportOtherMany  = "Other versions:"
	exportDeleted    = "deleted."
	exportTimeLayout = "2006-01-02 15:04 UTC"
)

// exportGroup is one of the four groups an export is made of, in the order it
// prints them: what the reader still has to answer comes first.
type exportGroup int

const (
	groupOpenQuestions exportGroup = iota
	groupThoughts
	groupHighlights
	groupResolvedQuestions
	groupCount
)

func (g exportGroup) heading(lang wording.Lang) string {
	switch g {
	case groupOpenQuestions:
		return wording.ExportOpenQuestions.In(lang)
	case groupThoughts:
		return wording.ExportThoughts.In(lang)
	case groupHighlights:
		return wording.ExportHighlights.In(lang)
	case groupResolvedQuestions:
		return wording.ExportResolvedQuestions.In(lang)
	default:
		panic(fmt.Sprintf("main: unknown exportGroup: %d", int(g)))
	}
}

func groupOf(m *machinemarks.Mark) exportGroup {
	switch m.Kind {
	case machinemarks.Question:
		if m.Resolved.IsZero() {
			return groupOpenQuestions
		}
		return groupResolvedQuestions
	case machinemarks.Thought:
		return groupThoughts
	case machinemarks.Highlight:
		return groupHighlights
	default:
		panic("main: unknown mark Kind: " + string(m.Kind))
	}
}

// stateWord is what a value of a mark is, in a line that lists the values.
func stateWord(m *machinemarks.Mark) string {
	if groupOf(m) == groupResolvedQuestions {
		return "resolved question"
	}
	return string(m.Kind)
}

// exportedMark is one mark and the values of it that do not count: the other
// side of a conflict, and the values this computer set aside.
type exportedMark struct {
	mark   machinemarks.Mark
	others []machinemarks.Value
}

// exportMarks builds the export under the privacy gate and seals it. notices are
// the lines that go to stderr beside it.
func (r *openRequest) exportMarks(ctx context.Context) (payload egress.Payload, notices []string, err error) {
	opened, err := r.openVault(ctx)
	if err != nil {
		return egress.Payload{}, nil, err
	}
	source, snap, gate := opened.source, opened.snap, opened.gate
	defer func() {
		if closeErr := source.Close(); closeErr != nil && err == nil {
			payload, notices, err = egress.Payload{}, nil, errOpenScan
		}
	}()
	adopted, err := r.adoptEarlier(source, snap, opened.allowed)
	if err != nil {
		return egress.Payload{}, nil, err
	}
	if r.configDir.IsZero() {
		reason := cmp.Or(r.noConfigDir, "the environment names none")
		return egress.Payload{}, nil, fmt.Errorf(`no configuration directory, so the marks for this folder cannot be found: %s; nothing was listed`, reason)
	}
	store, err := mark.Open(r.configDir, source.Name())
	if err != nil {
		return egress.Payload{}, nil, errOpenMarksUnreadable
	}
	x := &exporter{snap: snap, gate: gate, allowed: opened.allowed, lang: wording.En, locate: note.MarkLocator(snap, wording.En)}
	kept := mark.OpenKept(store, source, slog.New(slog.DiscardHandler), x.admit)
	defer func() {
		if closeErr := kept.Close(); closeErr != nil && err == nil {
			payload, notices, err = egress.Payload{}, nil, errOpenMarksUnreadable
		}
	}()
	marks, incomplete, err := x.gather(kept)
	if err != nil {
		return egress.Payload{}, nil, errOpenMarksUnreadable
	}
	if r.afterAssemble != nil {
		r.afterAssemble()
	}
	// The export was assembled under the rules file read at the start; it is
	// handed over only while that same source still stands.
	payload, err = gate.Seal([]byte(x.write(marks)))
	if err != nil {
		return egress.Payload{}, nil, openRefusal(err)
	}
	if gate.WithoutRules() {
		notices = append(notices, egress.NoRulesFile)
	}
	if adopted {
		notices = append(notices, openAdoptedNotice)
	}
	if incomplete {
		notices = append(notices, exportIncomplete)
	}
	return payload, notices, nil
}

// exporter writes the marks of one vault under its privacy gate.
type exporter struct {
	snap    *snapshot.Generation
	gate    *egress.Gate
	allowed func(string) bool
	// lang is the language the export is written in: the command line speaks
	// English only, and a page that copies a group is handed the reader's.
	lang   wording.Lang
	locate func(rel, anchor string) note.MarkedLocation
}

// citation is the mark as it is written out, and false when the gate withholds
// it: its note is in withheld ground, or its title, its section, the words the
// reader chose or the words they wrote spell withheld ground. The gate is asked
// of the path before anything is resolved from it, and of every text that would
// be printed.
func (x *exporter) citation(m *machinemarks.Mark) (note.Citation, bool) {
	if !x.allowed(m.Path) {
		return note.Citation{}, false
	}
	located := x.locate(m.Path, m.Anchor)
	var title, section string
	if located.Lost != note.LostNote {
		title = x.snap.DeclaredTitle(m.Path)
	}
	if located.Lost == note.LostNothing && m.Anchor != "" && located.AnchorText != title {
		section = located.AnchorText
	}
	reading, _ := x.snap.Note(m.Path)
	for _, text := range []string{title, section, m.Quote, m.Body} {
		if text != "" && x.gate.Spells(text, reading.Body, x.snap) {
			return note.Citation{}, false
		}
	}
	return note.Citation{
		Title: escapeTerminal(title), Path: escapeTerminal(m.Path), Section: escapeTerminal(section),
		Quote: escapeText(m.Quote), Words: escapeText(m.Body),
	}, true
}

// admit is the gate the marks are read under.
func (x *exporter) admit(m *machinemarks.Mark) bool {
	_, ok := x.citation(m)
	return ok
}

// gather reads every mark the gate lets out, with the values of it that do
// not count, in the order the groups print them. incomplete says a file of
// marks could not be read in full.
func (x *exporter) gather(kept *mark.Kept) (marks []exportedMark, incomplete bool, err error) {
	view, err := kept.View()
	if err != nil {
		return nil, false, err
	}
	pending, err := kept.Pending(view)
	if err != nil {
		return nil, false, err
	}
	var aside []machinemarks.Value
	hand, handErr := kept.Marks()
	switch {
	case handErr == nil:
		if aside, err = hand.SetAside(); err != nil {
			return nil, false, err
		}
	case errors.Is(handErr, machinefile.ErrLink) || errors.Is(handErr, machinefile.ErrNotAFolder):
		// The reader folder cannot be entered, so the view holds only what the
		// earlier file does, and nothing was set aside in it.
	default:
		return nil, false, handErr
	}
	asideOf := map[string][]machinemarks.Value{}
	for i := range aside {
		asideOf[aside[i].Mark.ID] = append(asideOf[aside[i].Mark.ID], aside[i])
	}
	for _, e := range view.Entries(x.admit) {
		if e.Deleted() {
			continue
		}
		extra := asideOf[e.Mark().ID]
		// A value set aside is a side of the mark like any other: one the gate
		// withholds takes the whole mark with it.
		if slices.ContainsFunc(extra, func(v machinemarks.Value) bool { return !x.admit(&v.Mark) }) {
			continue
		}
		marks = append(marks, exportedMark{mark: e.Mark(), others: otherValues(e.Values, extra)})
	}
	for i := range pending {
		// An earlier mark the vault already holds is shown as the vault holds
		// it, or not at all when the gate withholds it there.
		if _, held := view.Entry(pending[i].ID, mark.AllMarks); !held {
			marks = append(marks, exportedMark{mark: pending[i]})
		}
	}
	slices.SortStableFunc(marks, func(a, b exportedMark) int {
		return cmp.Or(cmp.Compare(groupOf(&a.mark), groupOf(&b.mark)), a.mark.At.Compare(b.mark.At), strings.Compare(a.mark.ID, b.mark.ID))
	})
	incomplete = view.Unreadable()+view.Damaged()+view.Newer()+view.LegacySkipped()+view.LedgerChanged() > 0 || view.OwnUnreadable()
	return marks, incomplete, nil
}

// otherValues are the values of a mark that do not count, newest first: the
// others of a conflict and the values set aside. A value that says what an
// earlier one said, or what the mark says, is not said twice.
func otherValues(values, aside []machinemarks.Value) []machinemarks.Value {
	all := slices.Concat(values[1:], aside)
	slices.SortStableFunc(all, func(a, b machinemarks.Value) int {
		return cmp.Or(b.Time.Compare(a.Time), strings.Compare(b.Writer, a.Writer), cmp.Compare(b.Rev, a.Rev))
	})
	seen := map[string]bool{valueKey(&values[0].Mark): true}
	return slices.DeleteFunc(all, func(v machinemarks.Value) bool {
		key := valueKey(&v.Mark)
		defer func() { seen[key] = true }()
		return seen[key]
	})
}

// valueKey is what a value says, for telling one that repeats another.
func valueKey(m *machinemarks.Mark) string {
	if m.Deleted {
		return "deleted"
	}
	return fmt.Sprintf("%s\x00%t\x00%s", m.Kind, m.Resolved.IsZero(), m.Body)
}

// write is the export as the Markdown it is handed over as.
func (x *exporter) write(marks []exportedMark) string {
	var b strings.Builder
	b.WriteString(exportTitle + "\n\n" + wording.ExportLede.In(x.lang) + "\n")
	if len(marks) == 0 {
		b.WriteString("\n" + exportNone + "\n")
		return b.String()
	}
	for g := range groupCount {
		first := true
		for i := range marks {
			if groupOf(&marks[i].mark) != g {
				continue
			}
			if first {
				b.WriteString("\n## " + g.heading(x.lang) + "\n")
				first = false
			}
			b.WriteString("\n" + x.entry(&marks[i]) + "\n")
		}
	}
	return b.String()
}

// entry is one mark: its citation, and the other values of it after.
func (x *exporter) entry(e *exportedMark) string {
	cite, ok := x.citation(&e.mark)
	if !ok {
		// The gate was asked of this mark when it was read, and nothing it
		// answers changes while one export is made.
		panic("main: a mark the gate let in is withheld")
	}
	text := note.Cite(&cite, x.lang, func(markdown string) string {
		return x.snap.Render(e.mark.Path, markdown, x.lang).HTML
	})
	if len(e.others) == 0 {
		return text
	}
	var b strings.Builder
	b.WriteString(text + "\n\n")
	heading := exportOtherMany
	if len(e.others) == 1 {
		heading = exportOtherOne
	}
	b.WriteString(heading + "\n")
	for i := range e.others {
		b.WriteString(otherLine(&e.others[i]) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// otherLine is one value of a mark that does not count, as an item of a list:
// when it was written, what it is, and the reader's words in it.
func otherLine(v *machinemarks.Value) string {
	line := "- " + v.Time.UTC().Format(exportTimeLayout) + ": "
	if v.Mark.Deleted {
		return line + exportDeleted
	}
	line += stateWord(&v.Mark) + "."
	words := splitWords(escapeText(v.Mark.Body))
	if len(words) == 0 {
		return line
	}
	// Continuing lines are indented under the item, so that words that look like
	// a heading or a list are the item's own.
	return line + " " + strings.Join(words, "\n  ")
}

func splitWords(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// escapeText is text the reader or the vault wrote, for a terminal: the line
// ends are the ones Markdown knows, and what a terminal would act on is written
// as a visible escape, as escapeTerminal writes it. A line feed and a tab stay,
// since they are the text's own layout.
func escapeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.ContainsFunc(text, actsOnTerminal) {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case actsOnTerminal(r):
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
