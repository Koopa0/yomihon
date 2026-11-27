package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func exportDay(day int) time.Time { return time.Date(2026, 11, day, 1, 0, 0, 0, time.UTC) }

const (
	exPipeline = "Notes/go/Pipeline.md"
	exSpace    = "Notes/My notes (draft)/Space.md"
)

// exportNotes writes the notes the marks of these tests are on, and the rules
// file that withholds "Private".
func exportNotes(t *testing.T, root, contract string) {
	t.Helper()
	if contract != "" {
		putOpenThoughtFile(t, root, schema.ContractRelPath, contract)
	}
	putOpenThoughtFile(t, root, exPipeline, "# Pipeline\n\nIntro words.\n\n## Two signals\n\ncancel() does not wait for the work to end.\n\n## Other\n\nmore\n")
	putOpenThoughtFile(t, root, exSpace, "# A title [with] *marks*\n\nplain words\n")
	putOpenThoughtFile(t, root, "Private/Secret plan.md", "# Secret plan\n\n## Hidden heading\n\nwords\n")
	putOpenThoughtFile(t, root, "Notes/Titled.md", "# On [[Private/Secret plan]]\n\nbody\n")
	putOpenThoughtFile(t, root, "Notes/Head.md", "# Head\n\n## About [[Private/Secret plan]]\n\ntext\n")
	putOpenThoughtFile(t, root, "Notes/Plain.md", "# Plain\n\nSee [[Private/Secret plan]] for the details.\n")
}

// computer is one computer's copy of a vault and its own configuration
// directory: what two people sharing a vault through git each have.
type computer struct {
	t      *testing.T
	root   string
	config string
}

func newComputer(t *testing.T, contract string) computer {
	t.Helper()
	c := computer{t: t, root: t.TempDir(), config: t.TempDir()}
	exportNotes(t, c.root, contract)
	return c
}

// hand is this computer's hand on the marks of its vault.
func (c computer) hand() *machinemarks.Marks {
	c.t.Helper()
	reader, err := vault.Open(c.root)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = reader.Close() }) //nolint:errcheck // the test is over
	store, err := mark.Open(confdir.At(c.config), reader.Name())
	if err != nil {
		c.t.Fatal(err)
	}
	kept := mark.OpenVaultMarks(store, reader, slog.New(slog.DiscardHandler))
	c.t.Cleanup(func() { _ = kept.Close() }) //nolint:errcheck // the test is over
	hand, err := kept.Marks()
	if err != nil {
		c.t.Fatal(err)
	}
	return hand
}

// say writes mark m as this computer reads the mark now, or as new.
func (c computer) say(m machinemarks.Mark) machinemarks.Result { //nolint:gocritic // hugeParam: a test helper
	c.t.Helper()
	hand := c.hand()
	view, err := hand.View()
	if err != nil {
		c.t.Fatal(err)
	}
	var base machinemarks.Seen
	if e, ok := view.Entry(m.ID, mark.AllMarks); ok {
		base = e.Seen
	}
	time.Sleep(3 * time.Millisecond) // a line's time is to the millisecond, and decides which value is shown
	res, err := hand.Put(c.t.Context(), &m, base, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	return res
}

// pull brings the file of from into this computer's vault, as git does.
func (c computer) pull(from computer) {
	c.t.Helper()
	view, err := from.hand().View()
	if err != nil {
		c.t.Fatal(err)
	}
	name := view.MachineID() + ".jsonl"
	data, err := os.ReadFile(filepath.Join(from.root, filepath.FromSlash(marksFolder), name)) // #nosec G304 -- a test's own path
	if err != nil {
		c.t.Fatal(err)
	}
	dir := filepath.Join(c.root, filepath.FromSlash(marksFolder))
	if err = os.MkdirAll(dir, 0o750); err != nil {
		c.t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil { // #nosec G703 -- a test's own path
		c.t.Fatal(err)
	}
}

// tryPut writes mark m as new, and says whether the marks took it: a path a
// machine file refuses is not kept, and a test of spellings goes on.
func (c computer) tryPut(m *machinemarks.Mark) bool {
	c.t.Helper()
	time.Sleep(3 * time.Millisecond)
	_, err := c.hand().Put(c.t.Context(), m, machinemarks.Seen{}, nil)
	return err == nil
}

func (c computer) export(args ...string) leftOpenResult {
	c.t.Helper()
	return leftOpen(c.t, c.root, c.config, append([]string{"--export"}, args...)...)
}

// theTimes are the moment a value was written, which a test cannot set.
var theTimes = regexp.MustCompile(`\d{4}-\d\d-\d\d \d\d:\d\d UTC`)

func withoutTimes(s string) string { return theTimes.ReplaceAllString(s, "<time>") }

func mustExport(t *testing.T, c computer) string {
	t.Helper()
	got := c.export()
	if got.exit != 0 {
		t.Fatalf("marks --export exit %d, stderr %q; want 0", got.exit, got.stderr)
	}
	return got.stdout
}

// The shape: no header of its own, the four groups in the order the reader
// wants them, a mark on a whole section with its source line alone, a quote the
// page would draw differently in a fence, a title written so it stays words, a
// path with a space between angle brackets, and nothing for a deleted mark.
func TestExportWritesTheShape(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	keep := keptMarks{t: t, root: c.root, config: c.config}
	keep.keep(
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: exPipeline, Anchor: "two-signals", Body: "Why does cancel not wait?", At: exportDay(1)},
		machinemarks.Mark{ID: markID(2), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", At: exportDay(2)},
		machinemarks.Mark{ID: markID(3), Kind: machinemarks.Thought, Path: exSpace, Quote: "plain words", Body: "line one\r\nline two", At: exportDay(3)},
		machinemarks.Mark{ID: markID(4), Kind: machinemarks.Highlight, Path: exPipeline, Anchor: "two-signals", Quote: "cancel() does not wait for the work to end.", At: exportDay(4)},
		machinemarks.Mark{ID: markID(5), Kind: machinemarks.Highlight, Path: exPipeline, Quote: "1. a\n2. b", At: exportDay(5)},
		machinemarks.Mark{ID: markID(6), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Quote: "more", Body: "answered", Resolved: exportDay(6), At: exportDay(6)},
		machinemarks.Mark{ID: markID(7), Kind: machinemarks.Thought, Path: exPipeline, Body: "deleted words", At: exportDay(7)},
	)
	keep.forget(markID(7))

	const want = "# Marks\n" +
		"\n" +
		"Links are relative to the vault's root folder.\n" +
		"\n" +
		"## Open questions\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md), Two signals\n" +
		"\n" +
		"Why does cancel not wait?\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md), Other\n" +
		"\n" +
		"## Thoughts\n" +
		"\n" +
		"> plain words\n" +
		"\n" +
		"— [A title \\[with\\] marks](<Notes/My notes (draft)/Space.md>)\n" +
		"\n" +
		"line one\n" +
		"line two\n" +
		"\n" +
		"## Highlights\n" +
		"\n" +
		"> cancel() does not wait for the work to end.\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md), Two signals\n" +
		"\n" +
		"> ```\n" +
		"> 1. a\n" +
		"> 2. b\n" +
		"> ```\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md)\n" +
		"\n" +
		"## Resolved questions\n" +
		"\n" +
		"> more\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md), Other\n" +
		"\n" +
		"answered\n"
	got := c.export()
	if diff := cmp.Diff(leftOpenResult{stdout: want}, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("marks --export mismatch (-want +got):\n%s", diff)
	}
}

func TestExportOfAVaultWithNoMarksSaysSo(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	const want = "# Marks\n\nLinks are relative to the vault's root folder.\n\nNo marks.\n"
	got := c.export()
	if diff := cmp.Diff(leftOpenResult{stdout: want}, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("marks --export of an empty vault (-want +got):\n%s", diff)
	}
	// A vault whose marks were all deleted reads the same.
	keep := keptMarks{t: t, root: c.root, config: c.config}
	keep.keep(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "gone", At: exportDay(1)})
	keep.forget(markID(1))
	if diff := cmp.Diff(leftOpenResult{stdout: want}, c.export(), cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("marks --export of a vault whose marks are all deleted (-want +got):\n%s", diff)
	}
}

// The words of every mark below that sits where the rules withhold, or that
// spells such ground, carry one of these, so that a leak is found by its word
// and a test cannot pass by printing nothing: the control run below, with no
// rules file, prints every one.
var exportSecretWords = []string{
	"SECRET-PATH", "SECRET-QUOTE", "SECRET-WORDS", "SECRET-TITLE", "SECRET-HEADING",
	"SECRET-GONE", "SECRET-LINK",
}

// exportWithheldMarks leaves the public marks, and one mark for each way the
// gate withholds one, on the computer.
func exportWithheldMarks(t *testing.T, c computer) {
	t.Helper()
	keptMarks{t: t, root: c.root, config: c.config}.keep(
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Quote: "cancel() does not wait for the work to end.", Body: "PUBLIC-ONE", At: exportDay(1)},
		// The note is in withheld ground: its path, what was chosen in it and what
		// the reader wrote.
		machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: "Private/Secret plan.md", Anchor: "hidden-heading", Quote: "SECRET-PATH quote", Body: "SECRET-PATH words", At: exportDay(2)},
		// A note that is gone, whose path was in withheld ground.
		machinemarks.Mark{ID: markID(3), Kind: machinemarks.Question, Path: "Private/Gone.md", Anchor: "x", Body: "SECRET-GONE", At: exportDay(3)},
		// A note in the open, whose words name withheld ground.
		machinemarks.Mark{ID: markID(4), Kind: machinemarks.Thought, Path: exPipeline, Body: "see Private/Secret plan.md SECRET-WORDS", At: exportDay(4)},
		machinemarks.Mark{ID: markID(5), Kind: machinemarks.Highlight, Path: exPipeline, Quote: "SECRET-QUOTE in Private/Secret plan", At: exportDay(5)},
		machinemarks.Mark{ID: markID(6), Kind: machinemarks.Thought, Path: exPipeline, Body: "a link [[Private/Secret plan]] SECRET-LINK", At: exportDay(6)},
		// A note whose title cites withheld ground, and one whose section does.
		machinemarks.Mark{ID: markID(7), Kind: machinemarks.Thought, Path: "Notes/Titled.md", Body: "SECRET-TITLE", At: exportDay(7)},
		machinemarks.Mark{ID: markID(8), Kind: machinemarks.Thought, Path: "Notes/Head.md", Anchor: "about-private-secret-plan", Body: "SECRET-HEADING", At: exportDay(8)},
		// What the page shows of a link to withheld ground is its name.
		machinemarks.Mark{ID: markID(9), Kind: machinemarks.Thought, Path: "Notes/Plain.md", Quote: "Private/Secret plan", Body: "SECRET-LINK", At: exportDay(9)},
		machinemarks.Mark{ID: markID(10), Kind: machinemarks.Thought, Path: exSpace, Quote: "plain words", Body: "PUBLIC-TWO", At: exportDay(10)},
	)
}

// leaks are the words of leaked that a result carries anywhere.
func leaks(got leftOpenResult, leaked []string) []string {
	var found []string
	for _, word := range leaked {
		if strings.Contains(got.stdout, word) || strings.Contains(got.stderr, word) {
			found = append(found, word)
		}
	}
	return found
}

// An export never prints a withheld note's path, its title, the words chosen in
// it or the reader's words about it, nor a mark that spells such ground, and it
// does not say how many it left out. The gate is the real one, read from a
// rules file that withholds a folder.
func TestExportSaysNothingOfWithheldGround(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	exportWithheldMarks(t, c)
	got := c.export()
	if got.exit != 0 {
		t.Fatalf("exit %d, stderr %q; want 0", got.exit, got.stderr)
	}
	leaked := append(slices.Clone(exportSecretWords), "Private", "Secret", "Hidden", "Gone", "private", "hidden")
	if found := leaks(got, leaked); len(found) > 0 {
		t.Errorf("the export says %q of withheld ground:\n%s\nstderr: %q", found, got.stdout, got.stderr)
	}
	for _, public := range []string{"PUBLIC-ONE", "PUBLIC-TWO"} {
		if !strings.Contains(got.stdout, public) {
			t.Errorf("the export left out %s, which no rule withholds:\n%s", public, got.stdout)
		}
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want nothing: a count of what was left out says how much there is", got.stderr)
	}

	// The control: with no rules file nothing is withheld, and every word the
	// test looks for is there to be found.
	open := newComputer(t, "")
	exportWithheldMarks(t, open)
	all := open.export()
	if found := leaks(all, exportSecretWords); len(found) != len(exportSecretWords) {
		t.Errorf("with nothing withheld the export holds %q of %q: the test cannot tell a leak from silence", found, exportSecretWords)
	}
}

// aSideOfWords leaves a mark on A that B changed to words the rules may
// withhold, and A changed too, later, without having read B's: the public value
// is the one shown and B's is the other side.
func aSideOfWords(t *testing.T, contract, bWords string) computer {
	t.Helper()
	a, b := newComputer(t, contract), newComputer(t, contract)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "PUBLIC-FIRST", At: exportDay(1)})
	b.pull(a)
	b.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: bWords, At: exportDay(1)})
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "PUBLIC-SECOND", At: exportDay(1)})
	a.pull(b)
	return a
}

// aSetAsideOfWords leaves A with a value of its own, with these words, that its
// own later change replaced while B's newer value was the one shown.
func aSetAsideOfWords(t *testing.T, contract, aWords string) computer {
	t.Helper()
	a, b := newComputer(t, contract), newComputer(t, contract)
	a.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "first", At: exportDay(1)})
	b.pull(a)
	a.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: aWords, At: exportDay(1)})
	b.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "PUBLIC-B", At: exportDay(1)})
	a.pull(b)
	a.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "PUBLIC-A", At: exportDay(1)})
	return a
}

// A mark with a side the gate withholds is withheld whole, whichever side it is
// and wherever the side came from: the other computer's value, or a value this
// computer set aside. Each is run twice: with a rules file, where the mark is
// gone, and without one, where every side is printed, which shows the scenario
// is the one that reaches the gate.
func TestExportWithholdsAWholeMarkWhenOneSideIsWithheld(t *testing.T) {
	t.Parallel()
	const secret = "SECRET-SIDE words of Private/Secret plan.md"
	for _, tt := range []struct {
		name  string
		build func(t *testing.T, contract string) computer
		// main is the words the mark shows.
		main string
	}{
		{"the other computer's value", func(t *testing.T, contract string) computer { t.Helper(); return aSideOfWords(t, contract, secret) }, "PUBLIC-SECOND"},
		{"a value this computer set aside", func(t *testing.T, contract string) computer { t.Helper(); return aSetAsideOfWords(t, contract, secret) }, "PUBLIC-A"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			open := mustExport(t, tt.build(t, ""))
			for _, word := range []string{"SECRET-SIDE", tt.main} {
				if !strings.Contains(open, word) {
					t.Fatalf("with nothing withheld the export is missing %q, so the test does not reach the case:\n%s", word, open)
				}
			}
			got := tt.build(t, leftOpenContract).export()
			if got.exit != 0 {
				t.Fatalf("exit %d, stderr %q", got.exit, got.stderr)
			}
			if found := leaks(got, []string{"SECRET-SIDE", "Private", tt.main}); len(found) > 0 {
				t.Errorf("a mark with a withheld side is printed in part (%q):\n%s", found, got.stdout)
			}
		})
	}
}

// Every side of a conflict is printed, grouped with its mark: the newest live
// one as the mark, the others under it. A deletion by a computer that had not
// read the words is a side too, and does not hide them.
func TestExportPrintsEverySideOfAConflict(t *testing.T) {
	t.Parallel()
	a, b := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Quote: "cancel() does not wait for the work to end.", Body: "first", At: exportDay(1)})
	a.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Body: "kept question", At: exportDay(2)})
	b.pull(a)
	// B has read the first value and writes its own, and deletes the question; A
	// has read neither change and writes later.
	b.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Quote: "cancel() does not wait for the work to end.", Body: "words of B\nover two lines", At: exportDay(1)})
	deleteOn(t, b, markID(2))
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Quote: "cancel() does not wait for the work to end.", Body: "words of A, the newest", At: exportDay(1)})
	a.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Body: "question changed by A", At: exportDay(2)})
	a.pull(b)

	const want = "# Marks\n" +
		"\n" +
		"Links are relative to the vault's root folder.\n" +
		"\n" +
		"## Open questions\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md), Other\n" +
		"\n" +
		"question changed by A\n" +
		"\n" +
		"Other version:\n" +
		"- <time>: deleted.\n" +
		"\n" +
		"## Thoughts\n" +
		"\n" +
		"> cancel() does not wait for the work to end.\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md)\n" +
		"\n" +
		"words of A, the newest\n" +
		"\n" +
		"Other version:\n" +
		"- <time>: thought. words of B\n" +
		"  over two lines\n"
	if diff := cmp.Diff(want, withoutTimes(mustExport(t, a))); diff != "" {
		t.Errorf("marks --export of a conflict (-want +got):\n%s", diff)
	}
}

// deleteOn deletes the mark as the computer's page, which has read it, would.
func deleteOn(t *testing.T, c computer, id string) {
	t.Helper()
	hand := c.hand()
	view, err := hand.View()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := view.Entry(id, mark.AllMarks)
	if !ok {
		t.Fatalf("no mark %s to delete", id)
	}
	time.Sleep(3 * time.Millisecond)
	if _, err = hand.Delete(t.Context(), id, entry.Seen, nil); err != nil {
		t.Fatal(err)
	}
}

// A value this computer's own change replaced while the page was shown another
// is kept in the ledger, and the export prints it with its mark.
func TestExportPrintsWhatThisComputerSetAside(t *testing.T) {
	t.Parallel()
	a, b := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "first", At: exportDay(1)})
	b.pull(a)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "A's words", At: exportDay(1)})
	b.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "B's words", At: exportDay(1)})
	a.pull(b)
	// A reads B's newer value and changes it, and so replaces its own.
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "B's words, changed by A", At: exportDay(1)})

	if strings.Contains(string(readFileOf(t, filepath.Join(a.root, filepath.FromSlash(marksFolder)))), "A's words") {
		t.Fatal("the test did not reach the case: a file of the vault still holds the value")
	}
	const want = "# Marks\n" +
		"\n" +
		"Links are relative to the vault's root folder.\n" +
		"\n" +
		"## Thoughts\n" +
		"\n" +
		"— [Pipeline](Notes/go/Pipeline.md)\n" +
		"\n" +
		"B's words, changed by A\n" +
		"\n" +
		"Other version:\n" +
		"- <time>: thought. A's words\n"
	if diff := cmp.Diff(want, withoutTimes(mustExport(t, a))); diff != "" {
		t.Errorf("marks --export with a value set aside (-want +got):\n%s", diff)
	}
}

// readFileOf is every file directly in dir, one after another.
func readFileOf(t *testing.T, dir string) []byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	var all []byte
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- a test's own path
		if readErr != nil {
			t.Fatal(readErr)
		}
		all = append(all, data...)
	}
	return all
}

// A mark is printed once: where two computers' files both hold lines of it, and
// where the values of a conflict say the same words.
func TestExportPrintsAMarkOnce(t *testing.T) {
	t.Parallel()
	a, b := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "ONCE-WORDS", At: exportDay(1)})
	b.pull(a)
	// B reads it and changes it: both files hold lines of the mark, and one
	// replaces the other.
	b.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "ONCE-WORDS-CHANGED", At: exportDay(1)})
	a.pull(b)
	out := mustExport(t, a)
	if n := strings.Count(out, "ONCE-WORDS"); n != 1 {
		t.Errorf("the mark is printed %d times, want once:\n%s", n, out)
	}
	if strings.Contains(out, "Other version") {
		t.Errorf("a value that was replaced is printed as another version:\n%s", out)
	}

	// Two computers wrote the same words without reading each other: a conflict
	// whose sides say one thing, which is not said twice.
	c, d := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	c.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "first", At: exportDay(1)})
	d.pull(c)
	c.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "SAME-WORDS", At: exportDay(1)})
	d.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "SAME-WORDS", At: exportDay(1)})
	c.pull(d)
	out = mustExport(t, c)
	if n := strings.Count(out, "SAME-WORDS"); n != 1 {
		t.Errorf("two sides with the same words are printed %d times, want once:\n%s", n, out)
	}
}

// A side of a conflict that says what the mark says, but of another note or of
// another place in the note, is not the same side: the reader moved the mark on
// one computer, and the other computer's value is where it stood. It is printed,
// with the place it was at.
func TestExportPrintsASideThatMovedAsAnotherVersion(t *testing.T) {
	t.Parallel()
	c, d := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	c.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "first", At: exportDay(1)})
	d.pull(c)
	c.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Body: "SAME-WORDS", At: exportDay(1)})
	d.say(machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exSpace, Body: "SAME-WORDS", At: exportDay(1)})
	c.pull(d)
	const want = "# Marks\n" +
		"\n" +
		"Links are relative to the vault's root folder.\n" +
		"\n" +
		"## Thoughts\n" +
		"\n" +
		"— [A title \\[with\\] marks](<Notes/My notes (draft)/Space.md>)\n" +
		"\n" +
		"SAME-WORDS\n" +
		"\n" +
		"Other version:\n" +
		"- <time>: thought. — [Pipeline](Notes/go/Pipeline.md)\n" +
		"  SAME-WORDS\n"
	if diff := cmp.Diff(want, withoutTimes(mustExport(t, c))); diff != "" {
		t.Errorf("marks --export of a side that moved (-want +got):\n%s", diff)
	}

	// Moved within the note: the same words under another heading.
	e, f := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	e.say(machinemarks.Mark{ID: markID(3), Kind: machinemarks.Thought, Path: exPipeline, Body: "first", At: exportDay(1)})
	f.pull(e)
	e.say(machinemarks.Mark{ID: markID(3), Kind: machinemarks.Thought, Path: exPipeline, Anchor: "other", Body: "SAME-WORDS", At: exportDay(1)})
	f.say(machinemarks.Mark{ID: markID(3), Kind: machinemarks.Thought, Path: exPipeline, Anchor: "two-signals", Body: "SAME-WORDS", At: exportDay(1)})
	e.pull(f)
	out := withoutTimes(mustExport(t, e))
	if !strings.Contains(out, "Other version:\n- <time>: thought. — [Pipeline](Notes/go/Pipeline.md), Other\n  SAME-WORDS") {
		t.Errorf("a side that moved to another heading of the note is not printed with its place:\n%s", out)
	}
}

// A mark's path is the spelling its file holds, and a file can hold any. The
// gate is asked of the path as it is written, and a spelling that names
// withheld ground in another way (another case, another normalisation of the
// same letters, a detour through a folder) is withheld as that ground is.
func TestExportWithholdsEverySpellingOfWithheldGround(t *testing.T) {
	t.Parallel()
	contract := strings.Replace(openThoughtContract, "never_egress_dirs = []", `never_egress_dirs = ["Privé"]`, 1)
	spellings := []string{
		"Privé/Secret.md",             // as the rule spells it
		"PRIVÉ/Secret.md",             // upper case
		"privé/secret.md",             // lower case
		"Prive\u0301/Secret.md",       // the same letters, decomposed
		"./Privé/Secret.md",           // a leading dot
		"Notes/../Privé/Secret.md",    // a detour
		"Privé//Secret.md",            // a doubled slash
		"Privé/./Secret.md",           // a dot in the middle
		"PRIVE\u0301/Secret.md",       // both
		"Notes\\..\\Privé\\Secret.md", // backslashes
	}
	build := func(contract string) computer {
		c := newComputer(t, contract)
		putOpenThoughtFile(t, c.root, "Privé/Secret.md", "# Secret\n\nwords\n")
		return c
	}
	routes := []struct {
		name string
		put  func(c computer, spelling string)
	}{
		{"machine files", func(c computer, spelling string) {
			c.tryPut(&machinemarks.Mark{ID: markID(10), Kind: machinemarks.Thought, Path: spelling, Quote: "SECRET-QUOTE", Body: "SECRET-MACHINE", At: exportDay(1)})
		}},
		{"a hand-edited earlier file", func(c computer, spelling string) {
			writeEarlierMarks(t, c.config, vaultNameOf(t, c.root),
				machinemarks.Mark{ID: markID(30), Kind: machinemarks.Question, Path: spelling, Anchor: "x", Body: "SECRET-LEGACY", At: exportDay(1)})
		}},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()
			reached := 0
			for _, spelling := range spellings {
				// The control first: with nothing withheld, what the test plants is
				// printed, so a silence below is the gate's. A spelling the store
				// itself refuses never reaches the export, and is not counted.
				open := build("")
				route.put(open, spelling)
				if got := open.export(); got.exit != 0 || !strings.Contains(got.stdout, "SECRET-") {
					continue
				}
				reached++
				c := build(contract)
				route.put(c, spelling)
				got := c.export()
				if found := leaks(got, []string{"SECRET-", "Secret", "Priv", "PRIV", "priv"}); len(found) > 0 {
					t.Errorf("the export prints %q of the path %q, a spelling of withheld ground:\n%s", found, spelling, got.stdout)
				}
			}
			if reached < 2 {
				t.Errorf("only %d of %d spellings reached the export: the test cannot tell a leak from silence", reached, len(spellings))
			}
		})
	}
}

// Without --export no output carries a word the reader wrote: the words they
// chose, what they wrote about it, and the words of a conflict's other side.
func TestMarksWithoutExportPrintsNoWords(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	words := []string{"QUOTE-WORDS", "THOUGHT-WORDS", "QUESTION-WORDS", "OTHER-SIDE-WORDS"}
	keep := keptMarks{t: t, root: c.root, config: c.config}
	keep.keep(
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Quote: "QUOTE-WORDS", Body: "QUESTION-WORDS", At: exportDay(1)},
		machinemarks.Mark{ID: markID(2), Kind: machinemarks.Thought, Path: exPipeline, Quote: "QUOTE-WORDS", Body: "THOUGHT-WORDS", At: exportDay(2)},
		machinemarks.Mark{ID: markID(3), Kind: machinemarks.Highlight, Path: exPipeline, Quote: "QUOTE-WORDS", At: exportDay(3)},
	)
	other := newComputer(t, leftOpenContract)
	other.pull(c)
	other.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Body: "OTHER-SIDE-WORDS", At: exportDay(1)})
	c.pull(other)
	if out := c.export().stdout; !strings.Contains(out, "OTHER-SIDE-WORDS") {
		t.Fatalf("the test does not reach the case: the export lacks the other side's words:\n%s", out)
	}
	for _, args := range [][]string{nil, {"--format", "json"}, {"--format", "human"}, {"--format", "md"}} {
		got := leftOpen(t, c.root, c.config, args...)
		if got.exit != 0 {
			t.Fatalf("marks %v: exit %d, stderr %q", args, got.exit, got.stderr)
		}
		if found := leaks(got, words); len(found) > 0 {
			t.Errorf("marks %v prints the reader's words %q:\n%s", args, found, got.stdout)
		}
	}
}

// A file of another computer that cannot be read does not hide the marks of the
// others, and does not make the export a refusal: the rest is printed and one
// line on stderr says it may be incomplete, without a count.
func TestExportSaysWhenAFileOfMarksCannotBeRead(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	c.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "STILL-HERE", At: exportDay(1)})
	whole := c.export()
	if whole.stderr != "" {
		t.Fatalf("stderr = %q before anything is wrong, want none", whole.stderr)
	}
	// A folder where another computer's file should be.
	if err := os.MkdirAll(filepath.Join(c.root, filepath.FromSlash(marksFolder), "deadbeef.jsonl"), 0o750); err != nil {
		t.Fatal(err)
	}
	got := c.export()
	want := leftOpenResult{stdout: whole.stdout, stderr: "yomihon: Some marks could not be read; this export may be incomplete.\n"}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("marks --export with a file that cannot be read (-want +got):\n%s", diff)
	}
}

// A marks store that cannot be used at all is a refusal that prints nothing,
// and names no path.
func TestExportRefusesWhenTheMarksCannotBeFound(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	var stdout, stderr bytes.Buffer
	exit := runOpenThoughts(t.Context(), []string{"--root", c.root, "--export"}, config{noConfigDir: "neither $XDG_CONFIG_HOME nor $HOME are defined"}, &stdout, &stderr, false)
	want := "yomihon: no configuration directory, so the marks for this folder cannot be found: neither $XDG_CONFIG_HOME nor $HOME are defined; nothing was listed\n"
	if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2, nothing, %q", exit, stdout.String(), stderr.String(), want)
	}
}

// A folder with no rules file withholds nothing and says so on stderr, so that
// what is printed is Markdown alone.
func TestExportOfAFolderWithNoRulesFileSaysSoOnStderr(t *testing.T) {
	t.Parallel()
	c := newComputer(t, "")
	c.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "Private/Secret plan.md", Body: "NOT-WITHHELD", At: exportDay(1)})
	got := c.export()
	if got.exit != 0 || got.stderr != "yomihon: "+egress.NoRulesFile+"\n" {
		t.Errorf("exit %d, stderr %q; want 0 and the sentence", got.exit, got.stderr)
	}
	if strings.Contains(got.stdout, egress.NoRulesFile) || !strings.HasPrefix(got.stdout, "# Marks\n") || !strings.Contains(got.stdout, "NOT-WITHHELD") {
		t.Errorf("stdout must be the export alone, with the mark:\n%s", got.stdout)
	}
}

// The list refuses when a note it cannot read could be a thought still open.
// The export lists what the files of marks hold and does not read notes for
// that, so a note that cannot be read does not make its marks disappear or the
// export a refusal.
func TestExportDoesNotRefuseForANoteItCannotRead(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	locked := filepath.Join(c.root, "Inside", "Locked.md")
	putOpenThoughtFile(t, c.root, "Inside/Locked.md", openThoughtNote("Locked", "reflection", draftFlag, "2026-09-09"))
	c.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "Inside/Locked.md", Quote: "LOCKED-QUOTE", Body: "LOCKED-WORDS", At: exportDay(1)})
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) }) //nolint:errcheck // TempDir removal reports what is left
	if data, err := os.ReadFile(locked); err == nil { // #nosec G304 -- a path under this test's TempDir
		t.Skipf("this user can read a mode-0 file (%d bytes), so no read fails", len(data))
	}
	if refused := leftOpen(t, c.root, c.config, "--format", "json"); refused.exit != 2 {
		t.Fatalf("the list exits %d, want 2: the test does not reach the case where the list refuses", refused.exit)
	}
	got := c.export()
	if got.exit != 0 || !strings.Contains(got.stdout, "LOCKED-QUOTE") || !strings.Contains(got.stdout, "LOCKED-WORDS") || !strings.Contains(got.stdout, "Inside/Locked.md") {
		t.Errorf("export exit %d, stderr %q, want the mark of the note it cannot read:\n%s", got.exit, got.stderr, got.stdout)
	}
}

// The marks of the earlier file are printed until the first write takes them in.
func TestExportPrintsTheEarlierFilesMarksBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	writeEarlierMarks(t, c.config, vaultNameOf(t, c.root),
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Body: "EARLIER-WORDS", At: exportDay(1)},
		machinemarks.Mark{ID: markID(2), Kind: machinemarks.Question, Path: "Private/Secret plan.md", Anchor: "hidden-heading", Body: "EARLIER-SECRET", At: exportDay(2)},
	)
	got := c.export()
	if got.exit != 0 || !strings.Contains(got.stdout, "EARLIER-WORDS") {
		t.Errorf("exit %d, stderr %q:\n%s\nwant the earlier file's question", got.exit, got.stderr, got.stdout)
	}
	if found := leaks(got, []string{"EARLIER-SECRET", "Private"}); len(found) > 0 {
		t.Errorf("the export prints %q from the earlier file, which sits in withheld ground", found)
	}
}

// A mark the vault holds, whose value the gate withholds, is not printed from
// the earlier file either, as the value it was.
func TestExportDoesNotPrintFromTheEarlierFileAMarkTheVaultWithholds(t *testing.T) {
	t.Parallel()
	a, b := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	b.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "Private/Secret plan.md", Body: "SECRET-HELD", At: exportDay(1)})
	a.pull(b)
	writeEarlierMarks(t, a.config, vaultNameOf(t, a.root),
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Body: "EARLIER-PUBLIC", At: exportDay(1)},
	)
	got := a.export()
	if found := leaks(got, []string{"SECRET-HELD", "EARLIER-PUBLIC", "Private"}); len(found) > 0 {
		t.Errorf("the export prints %q of a mark whose value the vault holds back:\n%s", found, got.stdout)
	}
	// The control: with nothing withheld the vault's value is the one printed.
	open, other := newComputer(t, ""), newComputer(t, "")
	other.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "Private/Secret plan.md", Body: "SECRET-HELD", At: exportDay(1)})
	open.pull(other)
	writeEarlierMarks(t, open.config, vaultNameOf(t, open.root),
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: exPipeline, Anchor: "other", Body: "EARLIER-PUBLIC", At: exportDay(1)},
	)
	if out := open.export().stdout; !strings.Contains(out, "SECRET-HELD") || strings.Contains(out, "EARLIER-PUBLIC") {
		t.Errorf("with nothing withheld the vault's value is the mark, and the earlier file's is not printed:\n%s", out)
	}
}

// What a terminal would act on is written out visibly, in the words, the quote,
// the title and the section, since a vault's text is not the reader's to trust.
func TestExportEscapesWhatATerminalWouldActOn(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	putOpenThoughtFile(t, c.root, "Notes/Escapes.md", "# Esc\u202etitle\n\n## Sec\u202etion\n\ntext\n")
	c.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "Notes/Escapes.md", Anchor: "sec-tion", Quote: "q\x1b[31mred\u0085x\u2028y", Body: "w\x1b]0;title\x07 w\u202ex\rline", At: exportDay(1)})
	out := mustExport(t, c)
	for _, r := range out {
		if r != '\n' && r != '\t' && actsOnTerminal(r) {
			t.Fatalf("the export holds %U, which acts on a terminal:\n%q", r, out)
		}
	}
	if !strings.Contains(out, `\u001b[31mred`) || !strings.Contains(out, `\u202e`) {
		t.Errorf("the escapes are not written out visibly:\n%s", out)
	}
}

// The export is handed over only while the rules file it was assembled under
// still stands.
func TestExportRefusesWhenTheRulesChangeAfterAssembly(t *testing.T) {
	t.Parallel()
	c := newComputer(t, "")
	c.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: "Private/Secret plan.md", Body: "LATE-RULES", At: exportDay(1)})
	var stdout, stderr bytes.Buffer
	request := openRequest{root: c.root, configDir: confdir.At(c.config), export: true, afterAssemble: func() {
		putOpenThoughtFile(t, c.root, schema.ContractRelPath, leftOpenContract)
	}}
	exit := request.run(t.Context(), &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), egress.ErrUnavailable.Error()) {
		t.Errorf("exit %d, stdout %q, stderr %q; want the privacy refusal and nothing printed", exit, stdout.String(), stderr.String())
	}
}

// A rules file that declares no privacy is refused as the list refuses it,
// before anything is read.
func TestExportRefusesARulesFileWithoutPrivacy(t *testing.T) {
	t.Parallel()
	c := newComputer(t, strings.Replace(leftOpenContract, "[privacy]\nnever_egress_dirs = [\"Private\"]\n", "", 1))
	c.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "UNDECLARED", At: exportDay(1)})
	got := c.export()
	if got.exit != 2 || got.stdout != "" || !strings.Contains(got.stderr, errOpenUndeclaredPrivacy.Error()) {
		t.Errorf("exit %d, stdout %q, stderr %q; want the refusal and nothing printed", got.exit, got.stdout, got.stderr)
	}
}

// The export writes nothing, in the vault or in the configuration directory.
func TestExportChangesNothing(t *testing.T) {
	t.Parallel()
	a, b := newComputer(t, leftOpenContract), newComputer(t, leftOpenContract)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "first", At: exportDay(1)})
	b.pull(a)
	a.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "A", At: exportDay(1)})
	b.say(machinemarks.Mark{ID: markID(1), Kind: machinemarks.Thought, Path: exPipeline, Body: "B", At: exportDay(1)})
	a.pull(b)
	before := treeBytes(t, a.root, a.config)
	if got := a.export(); got.exit != 0 {
		t.Fatalf("exit %d: %s", got.exit, got.stderr)
	}
	if diff := cmp.Diff(before, treeBytes(t, a.root, a.config)); diff != "" {
		t.Errorf("marks --export changed files (-before +after):\n%s", diff)
	}
}

// The help says what the command refuses beside --export, as the command does.
func TestExportHelpSaysItTakesNeitherFormatNorAdopt(t *testing.T) {
	t.Parallel()
	text, handled, err := helpRequest([]string{"marks", "--help"})
	if !handled || err != nil {
		t.Fatalf("helpRequest(marks --help) = %q, %v, %v", text, handled, err)
	}
	flat := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{"It takes no --format and no --adopt", "writes nothing"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the help of marks does not say %q:\n%s", want, text)
		}
	}
}

// --export belongs to marks alone, and writes Markdown, which no format names.
func TestExportIsOnlyForMarksAndTakesNoFormat(t *testing.T) {
	t.Parallel()
	c := newComputer(t, leftOpenContract)
	if got := c.export("--format", "json"); got.exit != 2 || got.stdout != "" || got.stderr != "yomihon: --export writes Markdown and does not accept --format\n" {
		t.Errorf("marks --export --format json: exit %d, stdout %q, stderr %q", got.exit, got.stdout, got.stderr)
	}
	// The export writes nothing, and --adopt writes: the two are not one run.
	if got := c.export("--adopt", filepath.Join(c.root, "earlier")); got.exit != 2 || got.stdout != "" || got.stderr != "yomihon: --export writes nothing and does not accept --adopt\n" {
		t.Errorf("marks --export --adopt: exit %d, stdout %q, stderr %q", got.exit, got.stdout, got.stderr)
	}
	for _, command := range []string{"check", "coverage", "exists"} {
		args := []string{"--root", c.root, "--export"}
		if command == "exists" {
			args = append(args, "candidate")
		}
		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), command, args, &stdout, &stderr, false)
		if want := "yomihon: " + command + " does not accept --export\n"; exit != 2 || stdout.Len() != 0 || stderr.String() != want {
			t.Errorf("%s --export: exit %d, stdout %q, stderr %q; want 2, nothing, %q", command, exit, stdout.String(), stderr.String(), want)
		}
	}
	if _, err := serveRoot([]string{"--export"}); err == nil {
		t.Error("serve --export is accepted")
	}
}
