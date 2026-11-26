package machinemarks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// jqOf is what the program in read-marks.jq reads of the machine files in dir: one
// entry for each mark, by its id. It skips the test when jq is not installed,
// unless YOMIHON_REQUIRE_JQ is set, which a CI job that is meant to have it sets.
type jqEntry struct {
	ID       string           `json:"id"`
	Mark     map[string]any   `json:"mark"`
	Conflict []map[string]any `json:"conflict"`
}

func jqOf(t *testing.T, dir string) map[string]jqEntry {
	t.Helper()
	path, err := exec.LookPath("jq")
	if err != nil {
		if os.Getenv("YOMIHON_REQUIRE_JQ") != "" {
			t.Fatalf("jq is required and is not installed: %v", err)
		}
		t.Skip("jq is not installed, so the machine files cannot be read without Yomihon here")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	args := append([]string{"-c", "-n", "-R", "-f", "read-marks.jq"}, files...)
	cmd := exec.CommandContext(t.Context(), path, args...) // #nosec G204 -- jq, with the test's own program and files
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if len(files) == 0 {
		return map[string]jqEntry{}
	}
	if err = cmd.Run(); err != nil {
		t.Fatalf("jq: %v\n%s", err, stderr.String())
	}
	got := map[string]jqEntry{}
	for _, line := range lines(out.Bytes()) {
		var e jqEntry
		if err = json.Unmarshal(line, &e); err != nil {
			t.Fatalf("jq said %q: %v", line, err)
		}
		got[e.ID] = e
	}
	return got
}

// asJSON is a mark as the generic value jq's output is read into.
func asJSON(t *testing.T, m *Mark) map[string]any {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// agree holds what Yomihon reads of the files in a vault no ledger helps with to
// what the program in read-marks.jq reads of them, mark by mark.
func agree(t *testing.T, reader *clone) map[string]jqEntry {
	t.Helper()
	want := jqOf(t, reader.marksDir())
	got := map[string]Entry{}
	for _, e := range reader.view().Entries(open) {
		got[e.Mark().ID] = e
	}
	if len(got) != len(want) {
		t.Errorf("Yomihon reads %d marks, jq reads %d", len(got), len(want))
	}
	for id, w := range want {
		e, ok := got[id]
		if !ok {
			t.Errorf("jq reads mark %s and Yomihon does not", id)
			continue
		}
		if !reflect.DeepEqual(asJSON(t, new(e.Mark())), w.Mark) {
			t.Errorf("mark %s: Yomihon reads %v, jq reads %v", id, asJSON(t, new(e.Mark())), w.Mark)
		}
		var rest []map[string]any
		for i := 1; i < len(e.Values); i++ {
			rest = append(rest, asJSON(t, &e.Values[i].Mark))
		}
		if len(rest) != len(w.Conflict) || (len(rest) > 0 && !reflect.DeepEqual(rest, w.Conflict)) {
			t.Errorf("mark %s: Yomihon's conflict is %v, jq's is %v", id, rest, w.Conflict)
		}
	}
	return want
}

// A vault of three machines with a conflict, deletions, a mark that changes
// state across machines, a file where an old line and a new one both are, git's
// conflict markers, a line that is not JSON and a line of a later format: what
// Yomihon reads and what the program in read-marks.jq reads are the same for every
// mark, and each is what is written here by hand.
func TestMachineFilesReadWithoutYomihon(t *testing.T) {
	a, b, c := newClone(t, "A"), newClone(t, "B"), newClone(t, "C")
	// 1: A writes, B changes it having read it, A changes it again.
	a.say(1, "m1 first")
	pull(a, b)
	b.say(1, "m1 by B")
	pull(b, a)
	a.say(1, "m1 by A last")
	// 2: a conflict, A's and B's, with C's later change on top of both.
	a.say(2, "m2 first")
	pull(a, b)
	a.say(2, "m2 A")
	b.say(2, "m2 B")
	// 3: A edits and B deletes without having read it: A's words stay shown.
	a.say(3, "m3 first")
	pull(a, b)
	a.say(3, "m3 A alive")
	b.delete(3)
	// 4: both delete.
	a.say(4, "m4 first")
	pull(a, b)
	a.delete(4)
	b.delete(4)
	// 5: A writes a highlight, C turns it into a question, B into a thought,
	// neither having read the other's.
	a.put(highlight(5, "a passage"), nil)
	pull(a, b)
	pull(a, c)
	q := a.entry(mid(5)).Mark()
	q.Kind = Question
	c.clock.tick()
	if _, err := c.m.Put(&q, c.entry(mid(5)).Seen, nil, open); err != nil {
		t.Fatal(err)
	}
	th := a.entry(mid(5)).Mark()
	th.Kind, th.Body = Thought, "now a thought"
	b.clock.tick()
	if _, err := b.m.Put(&th, b.entry(mid(5)).Seen, nil, open); err != nil {
		t.Fatal(err)
	}
	// 6: an old line and a newer line of one writer in one file, which is what
	// a file git merged from two ends of a branch holds.
	a.say(6, "m6 old")
	own := filepath.Join(a.marksDir(), a.id()+".jsonl")
	oldLines := lines(readFile(t, own))
	oldLine6 := oldLines[len(oldLines)-1]
	a.say(6, "m6 new")

	// A reader that has no ledger of its own sees only the files: pull all.
	d := newClone(t, "D")
	for _, from := range []*clone{a, b, c} {
		pull(from, d)
	}
	// Markers, a line that is not JSON and one of a later format, in a file.
	extra := filepath.Join(d.marksDir(), a.id()+".jsonl")
	data := readFile(t, extra)
	first := lines(data)[0]
	junk := slices.Concat([]byte("<<<<<<< HEAD\n"), data, []byte("=======\nthis is not json\n"), first, []byte("\n"), oldLine6, []byte("\n"),
		[]byte(`{"fmt":2,"m":"`+a.id()+`","rev":1,"mark":{"id":"`+mid(77)+`"}}`+"\n"), []byte(">>>>>>> agent\n"))
	if err := os.WriteFile(extra, junk, 0o600); err != nil {
		t.Fatal(err)
	}

	want := agree(t, d)

	words := func(id string) string {
		w := want[id]
		parts := []string{fmt.Sprint(w.Mark["body"])}
		for _, c := range w.Conflict {
			parts = append(parts, fmt.Sprint(c["body"]))
		}
		return strings.Join(parts, "|")
	}
	// The reading, by hand.
	for id, wantWords := range map[string]string{
		mid(1): "m1 by A last",
		mid(2): "m2 B|m2 A",
		mid(3): "m3 A alive|m3 first",
		mid(6): "m6 new",
	} {
		if got := words(id); got != wantWords {
			t.Errorf("by hand, mark %s reads %q, want %q", id, got, wantWords)
		}
	}
	if m := want[mid(4)].Mark; m["deleted"] != true {
		t.Errorf("mark 4 was deleted on both machines and reads %v", m)
	}
	if m := want[mid(5)]; len(m.Conflict) != 1 {
		t.Errorf("mark 5 changed state on two machines and reads %d values, want 2", len(m.Conflict)+1)
	}
	if _, ok := want[mid(77)]; ok {
		t.Error("a line of a later format was read")
	}
	if v := d.view(); v.Markers() != 3 || v.Newer() != 1 || v.Damaged() != 1 {
		t.Errorf("%d markers, %d newer, %d damaged; want 3, 1, 1", v.Markers(), v.Newer(), v.Damaged())
	}
}

// The line a hand changed is where the two part, which the program in read-marks.jq
// says: it has no way to check a checksum.
func TestTheProgramInTestdataDoesNotCheckAChecksum(t *testing.T) {
	t.Parallel()
	a, d := newClone(t, "A"), newClone(t, "D")
	a.say(1, "words")
	pull(a, d)
	file := filepath.Join(d.marksDir(), a.id()+".jsonl")
	if err := os.WriteFile(file, bytes.Replace(readFile(t, file), []byte("words"), []byte("WORDS"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := jqOf(t, d.marksDir()); len(got) != 1 {
		t.Fatalf("jq reads %d marks of a line a hand changed, want it read", len(got))
	}
	if got := len(d.view().Entries(open)); got != 0 {
		t.Fatalf("Yomihon reads %d marks of a line a hand changed, want none", got)
	}
}

// Random vaults: lines of three machines and two processes each, a few marks,
// revs, seen maps and deletions chosen at random, in files in a random order and
// with lines repeated and in any order. Yomihon and the program agree on each.
func TestRandomVaultsReadTheSameWithoutYomihon(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("jq"); err != nil && os.Getenv("YOMIHON_REQUIRE_JQ") == "" {
		t.Skip("jq is not installed")
	}
	rng := rand.New(rand.NewPCG(7, 11)) // #nosec G404 -- a test's own sequence
	machines := []string{"11111111", "22222222", "33333333"}
	processes := []string{"", "aaaaaaaa", "bbbbbbbb"}
	for round := range 60 {
		d := newClone(t, "D")
		if err := os.MkdirAll(d.marksDir(), 0o750); err != nil {
			t.Fatal(err)
		}
		files := map[string][]byte{}
		for range 4 + rng.IntN(20) {
			m := machines[rng.IntN(len(machines))]
			l := Line{Fmt: Format, M: m, Rev: 1 + rng.Uint64N(6), T: int64(rng.IntN(5)), S: processes[rng.IntN(len(processes))]}
			l.Mark = thought(1+rng.IntN(3), fmt.Sprintf("body %d", rng.IntN(100)))
			l.Mark.Deleted = rng.IntN(4) == 0
			if rng.IntN(3) == 0 {
				l.Mark.Kind = Question
			}
			l.Mark.At = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			for range rng.IntN(3) {
				if l.Seen == nil {
					l.Seen = Seen{}
				}
				w := machines[rng.IntN(len(machines))] + "/" + processes[rng.IntN(len(processes))]
				l.Seen[w] = rng.Uint64N(7)
			}
			raw, err := l.encode()
			if err != nil {
				t.Fatal(err)
			}
			file := machines[rng.IntN(len(machines))] + ".jsonl"
			files[file] = append(files[file], append(raw, '\n')...)
			if rng.IntN(5) == 0 {
				files[file] = append(files[file], append(raw, '\n')...)
			}
		}
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(d.marksDir(), name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("round %d: %d files", round, len(files))
		agree(t, d)
		if t.Failed() {
			t.Fatalf("round %d disagrees; the files are in %s", round, d.marksDir())
		}
	}
}

// The command a reader is shown to run is the program the tests hold to the
// screen: run as shown, in the vault's folder, it prints the marks.
func TestTheShownCommandReadsTheMarks(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("jq"); err != nil {
		if os.Getenv("YOMIHON_REQUIRE_JQ") != "" {
			t.Fatalf("jq is required and is not installed: %v", err)
		}
		t.Skip("jq is not installed")
	}
	command := ReadMarksCommand()
	if strings.Contains(command, "\n#") || strings.HasPrefix(strings.TrimPrefix(command, "jq -c -n -R '"), "#") {
		t.Fatal("the command carries a comment of the program")
	}
	a := newClone(t, "A")
	a.say(1, "first words")
	a.say(2, "second words")
	cmd := exec.CommandContext(t.Context(), "sh", "-c", command) // #nosec G204 -- the command the page shows
	cmd.Dir = a.root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the shown command failed: %v", err)
	}
	if n := len(lines(out)); n != 2 || !bytes.Contains(out, []byte("first words")) || !bytes.Contains(out, []byte("second words")) {
		t.Fatalf("the shown command printed %d marks: %s", n, out)
	}
}
