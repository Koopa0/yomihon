package machinemarks

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"
)

// rawLine is a line with the bytes it was read from, which are what is written
// back: a line that is kept is kept byte for byte, so its checksum still fits
// and a field this version does not know is not lost.
type rawLine struct {
	Line

	raw []byte
}

// Value is one value of a mark that no other line replaced.
type Value struct {
	Mark Mark
	// Writer is who wrote it, and Rev which of that writer's lines it is: with
	// the writer it names the value, as a choice between two is made.
	Writer string
	Rev    uint64
	// Time is when it was written, to the millisecond.
	Time time.Time
	// Own says this machine wrote it, in a process of its own.
	Own bool
}

// Entry is one mark as it is shown: the value that counts, and, when two writers
// each changed it without having read the other's change, the other values
// beside it. Nothing is merged: the values are the writers' own, whole.
type Entry struct {
	// Values are the lines no other line replaced, the one that counts first.
	// It is the newest of those that are not a deletion; only when every one is
	// a deletion is it a deletion, and a deletion by a writer that had not read
	// another's words does not hide them.
	Values []Value
	// Seen is the newest line of each writer of this mark, which a change made
	// from this entry says it read.
	Seen Seen
	// Aside are the values of this machine's own that a change replaced although
	// they were live and the change was not made on them: the value that two
	// computers each changed and the page was not shown, which the newer line of
	// this machine replaces by the rule that a writer's newer line always
	// replaces its older. They are kept whole in the ledger, in the order they
	// were set aside, so that nothing is lost; a page shows only the value that
	// counts.
	Aside []Value
}

// Mark is the value that counts.
func (e Entry) Mark() Mark { return e.Values[0].Mark }

// Deleted says the mark is deleted: every value of it is.
func (e Entry) Deleted() bool { return e.Values[0].Mark.Deleted }

// resolved is the lines of one mark worked down to what no line replaces.
type resolved struct {
	// heads are the lines no other line replaces, in the order they are shown.
	heads []*rawLine
	// newest is each writer's newest line of the mark.
	newest map[string]*rawLine
}

// resolve works the lines of one mark down to the ones none replaces. A line
// replaces another of the same mark if the same writer wrote it later, or if
// another writer's says it had read that line or one after it. What is left is
// one line, or several when no one replaced the others: a conflict.
//
// It reads a set: the order of ls and a line given twice make no difference,
// except that of two different lines with one writer and one rev, which only a
// hand writes, the first is the one read.
func resolve(ls []*rawLine) resolved {
	all, newest := distinct(ls)
	// read[w][from] is the highest rev of writer w that a line of another
	// writer, from, says it had read.
	read := map[string]map[string]uint64{}
	for _, l := range all {
		for w, n := range l.Seen {
			if w == l.Writer() {
				continue
			}
			if read[w] == nil {
				read[w] = map[string]uint64{}
			}
			read[w][l.Writer()] = max(read[w][l.Writer()], n)
		}
	}
	var heads []*rawLine
	for w, l := range newest {
		if !slices.ContainsFunc(slices.Collect(maps.Values(read[w])), func(n uint64) bool { return n >= l.Rev }) {
			heads = append(heads, l)
		}
	}
	if len(heads) == 0 {
		// Every line says another replaced it, which happens only if lines say
		// they had read what had not been written. No writer is dropped: each
		// writer's newest line stands, and not the older lines it replaced itself.
		heads = slices.Collect(maps.Values(newest))
	}
	slices.SortFunc(heads, func(a, b *rawLine) int {
		return cmp.Or(cmp.Compare(b.T, a.T), strings.Compare(b.Writer(), a.Writer()), cmp.Compare(b.Rev, a.Rev))
	})
	if i := slices.IndexFunc(heads, func(l *rawLine) bool { return !l.Mark.Deleted }); i > 0 {
		alive := heads[i]
		heads = slices.Insert(slices.Delete(heads, i, i+1), 0, alive)
	}
	return resolved{heads: heads, newest: newest}
}

// distinct is the lines of a set without the repeats of one writer's rev, and
// each writer's newest.
func distinct(ls []*rawLine) (all []*rawLine, newest map[string]*rawLine) {
	type lineKey struct {
		writer string
		rev    uint64
	}
	newest = map[string]*rawLine{}
	has := map[lineKey]bool{}
	for _, l := range ls {
		key := lineKey{l.Writer(), l.Rev}
		if has[key] {
			continue
		}
		has[key] = true
		all = append(all, l)
		if old := newest[l.Writer()]; old == nil || l.Rev > old.Rev {
			newest[l.Writer()] = l
		}
	}
	return all, newest
}

// entry is the resolved lines as an Entry. own says which lines this machine
// wrote.
func (r resolved) entry(own func(*rawLine) bool) *Entry {
	e := &Entry{Seen: Seen{}}
	for _, l := range r.heads {
		e.Values = append(e.Values, Value{
			Mark: l.Mark, Writer: l.Writer(), Rev: l.Rev, Time: time.UnixMilli(l.T), Own: own(l),
		})
	}
	for w, l := range r.newest {
		e.Seen[w] = l.Rev
	}
	return e
}

// merge groups lines by mark and resolves each group.
func merge(ls []*rawLine) map[string]resolved {
	groups := map[string][]*rawLine{}
	for _, l := range ls {
		groups[l.Mark.ID] = append(groups[l.Mark.ID], l)
	}
	out := make(map[string]resolved, len(groups))
	for id := range maps.Keys(groups) {
		out[id] = resolve(groups[id])
	}
	return out
}

// coveredHead is the value of the mark that a page which had read base showed
// it: the first value, in the order the reader is shown them, that base covers.
// A base that covers none, such as one that read nothing, is the page of a reader
// who has been shown no value in particular, and gets the one that counts.
func (r resolved) coveredHead(base Seen) *rawLine {
	for _, l := range r.heads {
		if l.Rev <= base[l.Writer()] {
			return l
		}
	}
	return r.heads[0]
}
