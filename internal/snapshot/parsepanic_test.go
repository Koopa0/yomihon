package snapshot

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// atLevel are the logged lines at level.
func atLevel(lines []loggedLine, level slog.Level) []loggedLine {
	var out []loggedLine
	for _, line := range lines {
		if line.Level == level {
			out = append(out, line)
		}
	}
	return out
}

// assertsReportedInFull holds one ERROR record to what a reader needs from it.
func assertsReportedInFull(t *testing.T, line loggedLine) {
	t.Helper()
	if line.Attrs["path"] != panicPoisoned || line.Attrs["panic"] != panicValue || line.Attrs["stack"] == "" {
		t.Errorf("ERROR record = %+v, want the path %q, the panic %q and a stack", line, panicPoisoned, panicValue)
	}
}

// poisonMark is the word every note a test means to break carries in its text,
// and it is how an injected parser knows which note it is looking at: the stages
// that parse a body are not told the path.
const poisonMark = "poisoned"

// stagePanics pairs each parser a note's bytes go through with the way a test
// makes that parser panic on a note carrying poisonMark.
var stagePanics = []struct {
	name string
	// install returns parsers whose stage panics with panicValue on a marked note and
	// is otherwise genuine.
	install func(genuine noteParsers) noteParsers
}{
	{"the note's own parse", func(p noteParsers) noteParsers {
		genuine := p.note
		p.note = func(rel string, data []byte) *vault.Note {
			if strings.Contains(string(data), poisonMark) {
				panic(panicValue)
			}
			return genuine(rel, data)
		}
		return p
	}},
	{"the search entry", func(p noteParsers) noteParsers {
		genuine := p.document
		p.document = func(n *vault.Note) lexical.Document {
			if strings.Contains(n.Body, poisonMark) {
				panic(panicValue)
			}
			return genuine(n)
		}
		return p
	}},
	{"the planned names", func(p noteParsers) noteParsers {
		genuine := p.planned
		p.planned = func(body string, contract *schema.Contract) judge.Planned {
			if strings.Contains(body, poisonMark) {
				panic(panicValue)
			}
			return genuine(body, contract)
		}
		return p
	}},
	{"the link targets", func(p noteParsers) noteParsers {
		genuine := p.links
		p.links = func(body string) []string {
			if strings.Contains(body, poisonMark) {
				panic(panicValue)
			}
			return genuine(body)
		}
		return p
	}},
}

// breakStage makes one of store's parser stages panic on every note carrying
// poisonMark, and returns the function that puts the real parsers back.
func breakStage(store *Store, stage int) (restore func()) {
	genuine := store.parsers
	store.parsers = stagePanics[stage].install(genuine)
	return func() { store.parsers = genuine }
}

// mustNotPanic runs step and turns a panic that escapes it into the test
// failure it is, naming the step, rather than a crashed test binary.
func mustNotPanic(tb testing.TB, step string, run func()) {
	tb.Helper()
	defer func() {
		if rec := recover(); rec != nil {
			tb.Fatalf("%s: a panic parsing one note escaped the snapshot build: %v", step, rec)
		}
	}()
	run()
}

const (
	panicPoisoned = "Concepts/Poison.md"
	panicValue    = "injected parser panic"
	panicFirst    = "---\ntype: concept\n---\npoisoned as first read\n"
)

// writePanicVault writes the notes the panic tests share. Alpha cites Beta, and
// cites a name the ledger note declares it still owes, so a build that lost
// either projection to the panic shows it; the poisoned note is the one a test
// then breaks.
func writePanicVault(t *testing.T, root string) {
	t.Helper()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nalpha, see [[Beta]] and [[Owed Concept]]\n")
	writeNote(t, root, "Concepts/Beta.md", "---\ntype: concept\n---\nbeta\n")
	writeNote(t, root, "Maps/ledger.md", "---\ntype: concept\n---\n\n## 缺口帳\n\n- Owed Concept\n")
	writeNote(t, root, panicPoisoned, panicFirst)
}

// assertTheRestIsServed holds a generation that met one panicking note to what
// the rest of the vault is owed: every other note, the projections built from
// every note's body, and the poisoned note named as one that could not be read.
func assertTheRestIsServed(t *testing.T, gen *Generation) {
	t.Helper()
	for _, served := range []string{"Concepts/Alpha.md", "Concepts/Beta.md", "Maps/ledger.md"} {
		if _, ok := gen.Note(served); !ok {
			t.Errorf("Note(%q) missing: one note's panic cost the notes beside it", served)
		}
	}
	if got := snapshotSearch(t, gen.Search(), "alpha"); len(got) != 1 || got[0].RelPath != "Concepts/Alpha.md" {
		t.Errorf("search for alpha = %+v, want the one note that says it: the index was lost with the poisoned note", got)
	}
	if refs := gen.CitedBy("Concepts/Beta.md"); len(refs) != 1 || refs[0].RelPath != "Concepts/Alpha.md" {
		t.Errorf("CitedBy(Beta) = %+v, want Alpha: the link projections were lost with the poisoned note", refs)
	}
	if !gen.TrackedForwardReference("Owed Concept") {
		t.Error("the planned names were lost with the poisoned note: a link to a name the vault owes reads as broken")
	}
	if got := gen.Health().Unwritten; len(got) != 0 {
		t.Errorf("Health().Unwritten = %+v, want none: the name Alpha cites is owed", got)
	}
}

// TestAPanicInOneNotesParseAtStartupCostsThatNoteNotTheVault is the lock on the
// startup scan's containment, for every parser a note's bytes go through. One
// note's parse panics at one stage; the build still publishes a generation, the
// other notes and everything built from their bodies are in it, the file is
// named where a reader looks for what could not be read, the resolver still
// holds a stub for it so its page is the unreadable page and not the one for an
// address that names nothing, and one ERROR record says which file and what the
// parser panicked with. A vault with no contract is held to the same, because
// the schema verdict is not what parses a body there.
func TestAPanicInOneNotesParseAtStartupCostsThatNoteNotTheVault(t *testing.T) {
	t.Parallel()
	for stage := range stagePanics {
		for _, withContract := range []bool{true, false} {
			name := stagePanics[stage].name
			if !withContract {
				name += ", no contract"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				writePanicVault(t, root)
				var contract *schema.Contract
				governance := schema.Ungoverned()
				if withContract {
					contract = testContract(t, root)
					governance = contract.Governance()
				}
				log, logged := newRecordingLogger()
				store := newStore(openReader(t, root), log, contract, governance)
				breakStage(store, stage)
				mustNotPanic(t, "startup scan", func() {
					if err := store.buildFirst(t.Context(), nil); err != nil {
						t.Fatalf("buildFirst() error = %v, want a generation of the notes that parse", err)
					}
				})
				gen := store.Current()

				assertTheRestIsServed(t, gen)
				if _, ok := gen.Note(panicPoisoned); ok {
					t.Errorf("Note(%q) present: a note whose parse panicked was served as though it had been read", panicPoisoned)
				}
				if _, ok := gen.Entry(panicPoisoned); !ok {
					t.Errorf("Entry(%q) missing: the file is on disk, so its page must be the unreadable page and not the one for an address that names nothing", panicPoisoned)
				}
				if got := gen.NoteCount(); got != 4 {
					t.Errorf("NoteCount() = %d, want 4: a note that could not be parsed is still a note somebody wrote", got)
				}
				// A citation of the note still lands on its file, as it does for a
				// note the read could not open: the resolver holds a stub for it.
				if got := gen.Graph().Resolve("Poison"); got.Kind != graph.KindUnique || got.RelPath != panicPoisoned {
					t.Errorf("Resolve(Poison) = %+v, want the file %q", got, panicPoisoned)
				}

				fresh := gen.Freshness()
				if fresh.Complete {
					t.Error("Freshness().Complete = true for a generation that could not read one of its notes")
				}
				if len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != panicPoisoned {
					t.Fatalf("Freshness().Blocked = %+v, want exactly %q named", fresh.Blocked, panicPoisoned)
				}
				if !strings.Contains(fresh.Blocked[0].Reason, panicValue) {
					t.Errorf("Blocked reason = %q, want the panic value %q in it", fresh.Blocked[0].Reason, panicValue)
				}

				errs := atLevel(logged(), slog.LevelError)
				if len(errs) != 1 {
					t.Fatalf("logged %d ERROR records, want exactly 1", len(errs))
				}
				assertsReportedInFull(t, errs[0])
			})
		}
	}
}

// TestAPanicInOneNotesParseDuringARebuildCostsThatNoteNotTheServer is the same
// lock on the path a running server takes, for every parser: a note that read
// fine is saved into a state a parser panics on, and the next rebuild must
// neither end the process nor take the notes beside it. Until the folder
// degrades the last whole generation is retained; after, the published one
// carries the last good copy of the note — its words and what its body yielded
// to search — marked stale, and names it blocked. Fixing the note recovers it.
func TestAPanicInOneNotesParseDuringARebuildCostsThatNoteNotTheServer(t *testing.T) {
	t.Parallel()
	for stage := range stagePanics {
		t.Run(stagePanics[stage].name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writePanicVault(t, root)
			contract := testContract(t, root)
			store := buildStore(t, openReader(t, root), discardLogger(), contract)
			whole := store.Current()
			if !whole.Freshness().Complete {
				t.Fatal("the first generation, built with the real parsers, is not whole; the rebuild below proves nothing")
			}
			base := time.Now()
			clock := base
			store.now = func() time.Time { return clock }

			// The note is saved into the state its parse panics on, and a new note
			// is written beside it, so what the containment costs the reader is
			// visible. The size change is what makes the next rescan rebuild.
			restore := breakStage(store, stage)
			writeNote(t, root, panicPoisoned, "---\ntype: concept\n---\npoisoned, saved into a state that panics\n")
			const added = "Concepts/Gamma.md"
			writeNote(t, root, added, "---\ntype: concept\n---\ngamma\n")

			// One rescan per scan interval, as the ticker drives it. The first three
			// attempts land on ticks 0, 1 and 3, and the third is where retention ends.
			mustNotPanic(t, "rebuild", func() {
				for tick := range 4 {
					clock = base.Add(time.Duration(tick) * scanInterval)
					store.rescan(t.Context())
				}
			})
			degraded := store.Current()
			if degraded == whole {
				t.Fatal("the rebuild never published: one note's panic held the whole folder at its old generation")
			}
			assertTheRestIsServed(t, degraded)
			if _, ok := degraded.Note(added); !ok {
				t.Error("a note written beside the one whose parse panics never reached a published generation")
			}
			carried, ok := degraded.Note(panicPoisoned)
			if !ok {
				t.Fatalf("Note(%q) missing: the last good copy of the note was dropped instead of carried", panicPoisoned)
			}
			if !carried.Stale || !strings.Contains(carried.Body, "poisoned as first read") {
				t.Errorf("carried note = {Stale:%t Body:%q}, want the last good copy marked as one that could not be re-read", carried.Stale, carried.Body)
			}
			if got := snapshotSearch(t, degraded.Search(), "first read"); len(got) != 1 || got[0].RelPath != panicPoisoned {
				t.Errorf("search for the carried words = %+v, want the carried note: its search entry was not carried with it", got)
			}
			fresh := degraded.Freshness()
			if fresh.Complete || len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != panicPoisoned {
				t.Errorf("Freshness() = %+v, want an incomplete generation naming only %q", fresh, panicPoisoned)
			}

			// Fixing the note is the ordinary path: it parses again, the next attempt
			// publishes a whole generation, and every degraded fact clears.
			restore()
			clock = base.Add(20 * scanInterval)
			store.rescan(t.Context())
			recovered := store.Current()
			if recovered == degraded {
				t.Fatal("a note that parses again did not publish a new generation")
			}
			if fresh := recovered.Freshness(); !fresh.Complete || len(fresh.Blocked) != 0 {
				t.Errorf("recovered Freshness() = %+v, want whole with nothing blocked", fresh)
			}
			note, ok := recovered.Note(panicPoisoned)
			if !ok || note.Stale || !strings.Contains(note.Body, "saved into a state that panics") {
				t.Errorf("recovered note = %+v, present %t; want the edited body read afresh", note, ok)
			}
		})
	}
}

// TestAPanicIsLoggedInFullOnceForTheBytesThatCauseIt holds the log to what
// somebody reading it can use. The same bytes panicking again is one WARN line;
// different bytes that still panic are a new ERROR with a stack; and a note that
// stopped panicking is forgotten, so the same bytes panicking after it was
// fixed are reported in full again rather than as a repeat of something old.
func TestAPanicIsLoggedInFullOnceForTheBytesThatCauseIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writePanicVault(t, root)
	contract := testContract(t, root)
	log, logged := newRecordingLogger()
	store := buildStore(t, openReader(t, root), log, contract)
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }
	tick := 0
	rescan := func() {
		clock = base.Add(time.Duration(tick) * scanInterval)
		tick++
		mustNotPanic(t, "rebuild", func() { store.rescan(t.Context()) })
	}
	counts := func() (errs, repeats int) {
		lines := logged()
		return len(atLevel(lines, slog.LevelError)), len(linesWith(lines, logPanicRepeated))
	}

	restore := breakStage(store, 1)
	writeNote(t, root, panicPoisoned, "---\ntype: concept\n---\npoisoned, edited once\n")
	rescan()
	if errs, warns := counts(); errs != 1 || warns != 0 {
		t.Fatalf("first panic: %d ERROR and %d WARN records, want 1 and 0", errs, warns)
	}
	rescan()
	if errs, warns := counts(); errs != 1 || warns != 1 {
		t.Errorf("same bytes again: %d ERROR and %d WARN records, want 1 and 1", errs, warns)
	}

	writeNote(t, root, panicPoisoned, "---\ntype: concept\n---\npoisoned, edited twice\n")
	rescan()
	if errs, _ := counts(); errs != 2 {
		t.Errorf("different bytes that still panic: %d ERROR records, want 2: a new set of bytes is a new report", errs)
	}

	// The note stops panicking, and then these same bytes panic again later.
	restore()
	rescan()
	if fresh := store.Current().Freshness(); !fresh.Complete {
		t.Fatalf("Freshness() = %+v after the parsers recovered, want whole", fresh)
	}
	breakStage(store, 1)
	clock = base.Add(2 * time.Hour)
	mustNotPanic(t, "reconciling rebuild", func() { store.rescan(t.Context()) })
	if errs, _ := counts(); errs != 3 {
		t.Errorf("the same bytes after the note had been fixed: %d ERROR records, want 3: a fixed note must not leave its old report standing", errs)
	}
}
