package machinemarks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/machinefile"
)

// otherFile is the name of a machine file of a machine that is not this one's.
const otherFile = "deadbeef.jsonl"

// Another machine's file that cannot be read hides none of the other marks, and
// is counted: past the size every machine may write, a folder where a file
// should be, and a link.
func TestAnUnreadableMachineFileHidesNoOtherMark(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"too long", "a folder", "a link"} {
		a, b := newClone(t, "A"), newClone(t, "B")
		a.say(1, "from A")
		b.say(2, "from B")
		pull(b, a)
		bad := filepath.Join(a.marksDir(), otherFile)
		switch kind {
		case "too long":
			if err := os.WriteFile(bad, bytes.Repeat([]byte("x"), machinefile.MaxFileBytes+1), 0o600); err != nil {
				t.Fatal(err)
			}
		case "a folder":
			if err := os.Mkdir(bad, 0o750); err != nil {
				t.Fatal(err)
			}
		case "a link":
			if err := os.Symlink(filepath.Join(a.root, "courses", "pg", "vacuum.md"), bad); err != nil {
				t.Skipf("no links here: %v", err)
			}
		}
		v := a.view()
		if v.Unreadable() != 1 || a.words(1) != "from A" || a.words(2) != "from B" {
			t.Fatalf("%s: %d unreadable, reads %q and %q", kind, v.Unreadable(), a.words(1), a.words(2))
		}
		// A write goes on, since this machine's own file was read.
		a.say(3, "written beside it")
		if a.words(3) != "written beside it" {
			t.Fatalf("%s: a write beside it did not take", kind)
		}
	}
}

// This machine's own file that cannot be read is not written over: the write
// would be of a file that was not read.
func TestThisMachinesOwnFileThatCannotBeReadIsNotWrittenOver(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "kept")
	own := filepath.Join(a.marksDir(), a.id()+".jsonl")
	if err := os.Remove(own); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(own, 0o750); err != nil {
		t.Fatal(err)
	}
	before := tree(t, a.root)
	ledgerBefore := string(a.ledgerBytes())
	// A read goes on: the marks are shown from the ledger and the other machines'
	// files, and the file that cannot be read is counted. Only a write refuses.
	v, err := a.m.View()
	if err != nil {
		t.Fatalf("View = %v, want a read that goes on", err)
	}
	if !v.OwnUnreadable() || v.Unreadable() != 0 {
		t.Fatalf("own unreadable = %v, others unreadable = %d, want the own file counted by itself", v.OwnUnreadable(), v.Unreadable())
	}
	if got := a.words(1); got != "kept" {
		t.Fatalf("mark 1 reads %q beside a file that cannot be read, want it from the ledger", got)
	}
	m := thought(2, "refused")
	if _, err := a.m.Put(t.Context(), &m, nil, nil, open); !errors.Is(err, ErrOwnFileUnreadable) {
		t.Fatalf("Put = %v, want ErrOwnFileUnreadable", err)
	}
	if diff := changedPaths(before, tree(t, a.root)); diff != "" || string(a.ledgerBytes()) != ledgerBefore {
		t.Fatalf("a refused write changed %q or the ledger", diff)
	}
}

// Git's conflict markers inside a machine file: the lines between them are
// read, the markers are counted, and the next write of this machine's file
// leaves none. Made by a real merge of two branches.
func TestThisMachinesOwnFileConflictsOnABranch(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "base")
	git(t, a.root, "init", "-q", "-b", "main")
	git(t, a.root, "add", "-A")
	git(t, a.root, "commit", "-q", "-m", "base")
	// An agent opens a branch in the same working tree, and the reader changes
	// the mark on both sides of the merge.
	git(t, a.root, "checkout", "-q", "-b", "agent")
	a.say(1, "the agent's side")
	git(t, a.root, "add", "-A")
	git(t, a.root, "commit", "-q", "-m", "agent")
	git(t, a.root, "checkout", "-q", "main")
	a.say(1, "the reader's side")
	git(t, a.root, "add", "-A")
	git(t, a.root, "commit", "-q", "-m", "reader")
	cmd := gitMerge(t, a.root)
	if !strings.Contains(cmd, "CONFLICT") {
		t.Fatalf("the merge did not conflict: %s", cmd)
	}
	file := filepath.Join(a.marksDir(), a.id()+".jsonl")
	if machinefile.ConflictMarkers(readFile(t, file)) == 0 {
		t.Fatal("the merged file has no markers")
	}
	v := a.view()
	if v.Markers() != 3 && v.Markers() == 0 {
		t.Fatalf("Markers = %d", v.Markers())
	}
	// Both sides were written by this machine's one process, in order: the
	// later replaces the earlier, and the reader's words are what is read.
	if got := a.words(1); got != "the reader's side" {
		t.Fatalf("reads %q", got)
	}
	a.say(2, "written after the merge")
	if machinefile.ConflictMarkers(readFile(t, file)) != 0 || a.view().Markers() != 0 {
		t.Fatal("a write left the markers in the file")
	}
	// The markers' bytes, the damaged line among them if any, are in the ledger.
	if !bytes.Contains(a.ledgerBytes(), []byte("<<<<<<<")) {
		t.Fatal("the file that was replaced was not kept in the ledger")
	}
}

// gitMerge merges the agent's branch and answers what git said: a conflict is
// an exit status, which is the point.
func gitMerge(t *testing.T, dir string) string {
	t.Helper()
	out, _ := gitTry(t, dir, "merge", "agent") //nolint:errcheck // a conflict is the point
	return out
}

// Lines of another machine's file that were changed after they were written are
// not shown and are counted; lines of a later format are neither read nor lost;
// both stay in this machine's file across a write of its own only if they are
// its own.
func TestALineChangedAfterItWasWrittenIsCountedAndNotRead(t *testing.T) {
	t.Parallel()
	a, b := newClone(t, "A"), newClone(t, "B")
	b.say(2, "B's words")
	pull(b, a)
	otherPath := filepath.Join(a.marksDir(), b.id()+".jsonl")
	data := readFile(t, otherPath)
	if err := os.WriteFile(otherPath, bytes.Replace(data, []byte("B's words"), []byte("B's edited"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	v := a.view()
	if v.Damaged() != 1 || a.words(2) != "(none)" {
		t.Fatalf("%d damaged, reads %q", v.Damaged(), a.words(2))
	}
}

// The bytes of this machine's file that a write is about to replace, when they
// are not what it last wrote, are kept whole in the ledger; its own marks come
// from the ledger meanwhile; and a line of a later format in it is kept.
func TestWhatAnAgentChangedInThisMachinesFileIsKeptBeforeItIsReplaced(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "first words")
	a.say(2, "second words")
	own := filepath.Join(a.marksDir(), a.id()+".jsonl")
	data := readFile(t, own)
	newerLine := []byte(`{"fmt":2,"m":"` + a.id() + `","something":"a later version wrote this"}` + "\n")
	changed := append(bytes.Replace(data, []byte("second words"), []byte("second WORDS"), 1), newerLine...)
	if err := os.WriteFile(own, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	v := a.view()
	if v.DamagedOwn() != 1 || v.Missing() != 1 || a.words(2) != "second words" || v.Newer() != 1 {
		t.Fatalf("%d damaged, %d missing, %d newer, reads %q", v.DamagedOwn(), v.Missing(), v.Newer(), a.words(2))
	}
	a.say(3, "a write over it")
	if !bytes.Contains(a.ledgerBytes(), []byte("second WORDS")) {
		t.Fatal("the changed bytes were not kept in the ledger")
	}
	now := readFile(t, own)
	if !bytes.Contains(now, newerLine) {
		t.Fatalf("a line of a later format was not kept: %s", now)
	}
	if bytes.Contains(now, []byte("second WORDS")) || !bytes.Contains(now, []byte("second words")) {
		t.Fatalf("the file still holds the changed line, or lost the mark: %s", now)
	}
	if a.view().DamagedOwn() != 0 || a.view().Missing() != 0 {
		t.Fatal("what was mended is still said")
	}
	// Bytes the writer made itself are not kept again, whatever happens to them
	// in other ways: an unchanged write adds no copy of the file.
	size := len(a.ledgerBytes())
	a.say(4, "a write over a file that was just written")
	if grew := len(a.ledgerBytes()) - size; grew > 2*len(data) {
		t.Fatalf("the ledger grew by %d bytes for one mark: the file was kept again", grew)
	}
}

// A line is read from bytes that are kept as they are: carriage returns that a
// checkout on Windows put there are read past, and a field this version does
// not know in a line it wrote is not lost on rewrite.
func TestALineIsKeptByteForByte(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "one")
	own := filepath.Join(a.marksDir(), a.id()+".jsonl")
	data := readFile(t, own)
	crlf := bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	if err := os.WriteFile(own, crlf, 0o600); err != nil {
		t.Fatal(err)
	}
	if v := a.view(); v.DamagedOwn() != 0 || v.Missing() != 0 || a.words(1) != "one" {
		t.Fatalf("a file with carriage returns reads as %d damaged, %d missing, %q", v.DamagedOwn(), v.Missing(), a.words(1))
	}
	a.say(2, "two")
	if bytes.Contains(readFile(t, own), []byte("\r")) {
		t.Fatal("the rewritten file still has carriage returns")
	}
	if !bytes.Contains(readFile(t, own), lines(data)[0]) {
		t.Fatal("a line was not kept byte for byte")
	}
}

func TestTheLimits(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	// Within the bound on words, past the one on a line once its quotes are
	// escaped.
	big := thought(1, strings.Repeat(`"`, maxBodyBytes))
	if _, err := a.m.Put(t.Context(), &big, nil, nil, open); !errors.Is(err, ErrLimit) {
		t.Fatalf("Put of a line past the limit = %v, want ErrLimit", err)
	}
	if got := a.view().Entries(open); len(got) != 0 || bytes.Contains(a.ledgerBytes(), []byte(`"kind":"mark"`)) {
		t.Fatal("a refused line was written")
	}
	if _, err := os.Stat(a.marksDir()); err == nil {
		t.Fatal("a refused line made the folders")
	}
	for _, mark := range []Mark{
		{ID: "short", Kind: Thought, Path: "a.md", At: thought(1, "").At},
		{ID: mid(1), Kind: "idea", Path: "a.md", At: thought(1, "").At},
		{ID: mid(1), Kind: Thought, Path: "", At: thought(1, "").At},
		{ID: mid(1), Kind: Thought, Path: "a.md"},
		{ID: mid(1), Kind: Thought, Path: "a.md", Body: "a\xffb", At: thought(1, "").At},
		{ID: mid(1), Kind: Thought, Path: "a.md", Body: "a\x00b", At: thought(1, "").At},
		{ID: mid(1), Kind: Thought, Path: "a.md", Before: "words before nothing", At: thought(1, "").At},
	} {
		if _, err := a.m.Put(t.Context(), &mark, nil, nil, open); !errors.Is(err, ErrInvalidMark) {
			t.Errorf("Put(%+v) = %v, want ErrInvalidMark", mark, err)
		}
	}
	if _, err := a.m.Put(t.Context(), nil, nil, nil, open); !errors.Is(err, ErrInvalidMark) {
		t.Errorf("Put(nil) = %v", err)
	}
	if _, err := a.m.Put(t.Context(), new(thought(1, "x")), nil, &Edit{}, open); !errors.Is(err, ErrInvalidMark) {
		t.Errorf("Put with an edit with no id = %v", err)
	}
}

// The count of the marks that are not deleted, and of the lines of the file,
// are bounds of a write and not of a read.
func TestTheCountsAreBoundsOfAWriteAndNotOfARead(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"marks", "lines"} {
		a := newClone(t, "A")
		for n := 1; n <= 3; n++ {
			a.say(n, "mark")
		}
		if name == "marks" {
			a.m.maxLive = 3
		} else {
			a.m.maxLines = 3
		}
		fourth := thought(4, "one too many")
		if _, err := a.m.Put(t.Context(), &fourth, nil, nil, open); !errors.Is(err, ErrLimit) {
			t.Fatalf("%s: the fourth mark = %v, want ErrLimit", name, err)
		}
		if got := len(a.view().Entries(open)); got != 3 {
			t.Fatalf("%s: reads %d marks, want 3", name, got)
		}
		// An edit of a mark that is there is not growth.
		a.say(1, "edited")
		a.delete(3)
		_, err := a.m.Put(t.Context(), &fourth, nil, nil, open)
		switch name {
		case "marks":
			if err != nil {
				t.Fatalf("a deleted mark did not make room: %v", err)
			}
		default:
			if !errors.Is(err, ErrLimit) {
				t.Fatalf("a deleted mark's line stopped counting: %v", err)
			}
		}
	}
}

// A mark is held to the shape the store of the earlier file holds it to, on the
// way in and on the way out: the lines of other machines are untrusted, and a
// line that carries what no page of this reader produces is not read.
func TestAMarkIsHeldToTheShapeTheEarlierFileHolds(t *testing.T) {
	t.Parallel()
	nfd := "Café/notes.md" // "e" and a combining accent: not NFC
	for _, tc := range []struct {
		name  string
		shape func(*Mark)
	}{
		{"a path that leaves the vault", func(m *Mark) { m.Path = "../outside.md" }},
		{"a rooted path", func(m *Mark) { m.Path = "/etc/passwd" }},
		{"a path that is a dot", func(m *Mark) { m.Path = "." }},
		{"a path with a dot dot inside", func(m *Mark) { m.Path = "a/../b.md" }},
		{"a path with a line break", func(m *Mark) { m.Path = "a\nb.md" }},
		{"a path with a line separator", func(m *Mark) { m.Path = "a\u2028b.md" }},
		{"a path that is not NFC", func(m *Mark) { m.Path = nfd }},
		{"an anchor with a fragment mark", func(m *Mark) { m.Anchor = "a#b" }},
		{"an anchor with a space", func(m *Mark) { m.Anchor = "a b" }},
		{"an anchor too long", func(m *Mark) { m.Anchor = strings.Repeat("a", 257) }},
		{"a quote too long", func(m *Mark) { m.Quote = strings.Repeat("q", 8<<10+1) }},
		{"words before too long", func(m *Mark) { m.Quote, m.Before = "q", strings.Repeat("b", 513) }},
		{"words after too long", func(m *Mark) { m.Quote, m.After = "q", strings.Repeat("b", 513) }},
		{"a body too long", func(m *Mark) { m.Body = strings.Repeat("b", 32<<10+1) }},
		{"an identity that is not a content identity", func(m *Mark) { m.Identity = "abc" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newClone(t, "A")
			m := thought(1, "x")
			tc.shape(&m)
			if _, err := a.m.Put(t.Context(), &m, nil, nil, open); !errors.Is(err, ErrInvalidMark) {
				t.Errorf("Put = %v, want ErrInvalidMark", err)
			}
			// The same shape in a line another machine wrote, whole and with its
			// checksum right: it is not read.
			line := Line{Fmt: Format, M: "22222222", Rev: 1, T: 1, Mark: m}
			raw, err := line.encode()
			if err != nil {
				t.Fatal(err)
			}
			if err = os.MkdirAll(a.marksDir(), 0o750); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(a.marksDir(), "22222222.jsonl"), append(raw, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			if v := a.view(); len(v.Entries(open)) != 0 || v.Damaged() != 1 {
				t.Errorf("%d marks read and %d damaged from a line that is not the shape of a mark, want none read and 1 damaged", len(v.Entries(open)), v.Damaged())
			}
		})
	}
}
