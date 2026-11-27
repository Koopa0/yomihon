package machinemarks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/mark/sealedline"
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
	if _, err := a.m.Put(t.Context(), &m, nil, nil); !errors.Is(err, ErrOwnFileUnreadable) {
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
	if markersIn(readFile(t, file)) == 0 {
		t.Fatal("the merged file has no markers")
	}
	// Both sides were written by this machine's one process, in order: the
	// later replaces the earlier, and the reader's words are what is read.
	if got := a.words(1); got != "the reader's side" {
		t.Fatalf("reads %q", got)
	}
	a.say(2, "written after the merge")
	if markersIn(readFile(t, file)) != 0 {
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
	if !bytes.Contains(readFile(t, own), sealedline.Lines(data)[0]) {
		t.Fatal("a line was not kept byte for byte")
	}
}

func TestTheLimits(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	// Within the bound on words, past the one on a line once its quotes are
	// escaped.
	big := thought(1, strings.Repeat(`"`, maxBodyBytes))
	if _, err := a.m.Put(t.Context(), &big, nil, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("Put of a line past the limit = %v, want ErrLimit", err)
	}
	if got := a.view().Entries(open); len(got) != 0 || bytes.Contains(a.ledgerBytes(), []byte(`"kind":"mark"`)) {
		t.Fatal("a refused line was written")
	}
	if _, err := os.Stat(a.marksDir()); err == nil {
		t.Fatal("a refused line made the folders")
	}
	if _, err := a.m.Put(t.Context(), nil, nil, nil); !errors.Is(err, ErrInvalidMark) {
		t.Errorf("Put(nil) = %v", err)
	}
	if _, err := a.m.Put(t.Context(), new(thought(1, "x")), nil, &Edit{}); !errors.Is(err, ErrInvalidMark) {
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
		if _, err := a.m.Put(t.Context(), &fourth, nil, nil); !errors.Is(err, ErrLimit) {
			t.Fatalf("%s: the fourth mark = %v, want ErrLimit", name, err)
		}
		if got := len(a.view().Entries(open)); got != 3 {
			t.Fatalf("%s: reads %d marks, want 3", name, got)
		}
		// An edit of a mark that is there is not growth.
		a.say(1, "edited")
		a.delete(3)
		_, err := a.m.Put(t.Context(), &fourth, nil, nil)
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

// Validate is the one rule of a mark's shape, which a page's request, a line of
// any machine and a mark of the earlier file are held to.
func TestValidate(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		shape func(*Mark)
		ok    bool
	}{
		{"a thought", func(*Mark) {}, true},
		{"a question that says nothing", func(m *Mark) { m.Kind, m.Body = Question, "" }, true},
		{"a highlight", func(m *Mark) { m.Kind, m.Body, m.Quote = Highlight, "", "q" }, true},
		{"a deleted highlight of nothing", func(m *Mark) { m.Kind, m.Body, m.Deleted = Highlight, "", true }, true},
		{"a deleted thought that says nothing", func(m *Mark) { m.Body, m.Deleted = "", true }, true},
		{"a resolution kept through a change of state", func(m *Mark) { m.Resolved = m.At }, true},
		{"a short id", func(m *Mark) { m.ID = "short" }, false},
		{"an id in capitals", func(m *Mark) { m.ID = strings.Repeat("A", 32) }, false},
		{"a kind that is none", func(m *Mark) { m.Kind = "idea" }, false},
		{"no time", func(m *Mark) { m.At = time.Time{} }, false},
		{"a highlight of nothing", func(m *Mark) { m.Kind, m.Body = Highlight, "" }, false},
		{"a highlight with words of its own", func(m *Mark) { m.Kind, m.Quote = Highlight, "q" }, false},
		{"a thought that says nothing", func(m *Mark) { m.Body = "" }, false},
		{"words around nothing", func(m *Mark) { m.Before = "before" }, false},
		{"words that are not text", func(m *Mark) { m.Body = "a\xffb" }, false},
		{"words with a NUL", func(m *Mark) { m.Body = "a\x00b" }, false},
		{"a quote too long", func(m *Mark) { m.Quote = strings.Repeat("q", 8<<10+1) }, false},
		{"words before too long", func(m *Mark) { m.Quote, m.Before = "q", strings.Repeat("b", 513) }, false},
		{"words after too long", func(m *Mark) { m.Quote, m.After = "q", strings.Repeat("b", 513) }, false},
		{"a body too long", func(m *Mark) { m.Body = strings.Repeat("b", 32<<10+1) }, false},
		{"an identity", func(m *Mark) { m.Identity = strings.Repeat("ab", 32) }, true},
		{"an identity that is not one", func(m *Mark) { m.Identity = "abc" }, false},
		{"an identity in capitals", func(m *Mark) { m.Identity = strings.Repeat("AB", 32) }, false},
		{"no note", func(m *Mark) { m.Path = "" }, false},
		{"a path too long", func(m *Mark) { m.Path = strings.Repeat("a", 1025) }, false},
		{"a path that leaves the vault", func(m *Mark) { m.Path = "../outside.md" }, false},
		{"a rooted path", func(m *Mark) { m.Path = "/etc/passwd" }, false},
		{"a path that is a dot", func(m *Mark) { m.Path = "." }, false},
		{"a path with a dot dot inside", func(m *Mark) { m.Path = "a/../b.md" }, false},
		{"a path with a line break", func(m *Mark) { m.Path = "a\nb.md" }, false},
		{"a path with an escape", func(m *Mark) { m.Path = "a\x1b[2J.md" }, false},
		{"a path with a line separator", func(m *Mark) { m.Path = "a\u2028b.md" }, false},
		{"a path that is not NFC", func(m *Mark) { m.Path = "Cafe\u0301/notes.md" }, false},
		{"a path with a space", func(m *Mark) { m.Path = "Notes/A published note.md" }, true},
		{"a path in Chinese", func(m *Mark) { m.Path = "Notes/go/提早返回的管線.md" }, true},
		{"a path with an ideographic space", func(m *Mark) { m.Path = "Notes/日本\u3000語.md" }, true},
		{"no anchor", func(m *Mark) { m.Anchor = "" }, true},
		{"a block anchor", func(m *Mark) { m.Anchor = "^safe" }, true},
		{"an anchor with an ampersand", func(m *Mark) { m.Anchor = "a&b" }, true},
		{"an anchor with a C1 control", func(m *Mark) { m.Anchor = "a\u0085b" }, true},
		{"an anchor of 256 bytes", func(m *Mark) { m.Anchor = strings.Repeat("a", 256) }, true},
		{"an anchor too long", func(m *Mark) { m.Anchor = strings.Repeat("a", 257) }, false},
		{"an anchor of 256 bytes in kanji", func(m *Mark) { m.Anchor = strings.Repeat("漢", 85) + "a" }, true},
		{"an anchor too long in kanji", func(m *Mark) { m.Anchor = strings.Repeat("漢", 86) }, false},
		{"an anchor that is not text", func(m *Mark) { m.Anchor = "\xff" }, false},
		{"an anchor with DEL", func(m *Mark) { m.Anchor = "a\x7fb" }, false},
	} {
		m := thought(1, "x")
		tt.shape(&m)
		if err := m.Validate(); (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrInvalidMark)) {
			t.Errorf("%s: Validate = %v, want ok %v", tt.name, err, tt.ok)
		}
	}
	for _, r := range "#?/\\\"'<> \t\n\x00\x1f" {
		if err := CheckAnchor("a" + string(r) + "b"); err == nil {
			t.Errorf("CheckAnchor accepted %U", r)
		}
	}
}

// A write is refused with ErrInvalidMark and writes nothing, and the same shape
// in a line another machine wrote, whole and with its checksum right, is
// counted as damaged and not read.
func TestAMarkOutOfShapeIsRefusedOnTheWayInAndOut(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	m := thought(1, "x")
	m.Path = "../outside.md"
	if _, err := a.m.Put(t.Context(), &m, nil, nil); !errors.Is(err, ErrInvalidMark) {
		t.Errorf("Put = %v, want ErrInvalidMark", err)
	}
	if _, err := os.Stat(a.marksDir()); err == nil {
		t.Fatal("a refused write made the folders")
	}
	raw, err := (&markLine{Fmt: lineFormat, M: "22222222", Rev: 1, T: 1, Mark: m}).encode()
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
		t.Fatalf("another machine's line out of shape reads as %d marks, %d damaged; want none read and 1 counted", len(v.Entries(open)), v.Damaged())
	}
}

// git writes these markers into a file it could not merge. A line of marks is a
// JSON object, so it is never one, and a line that only begins like one is not.
func TestConflictMarkersAreGitsLinesAndNoOthers(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		data string
		want int
	}{
		{"a merge that stopped", "{\"a\":1}\n<<<<<<< HEAD\n{\"a\":2}\n=======\n{\"a\":3}\n>>>>>>> branch\n", 3},
		{"with the base, as diff3 writes it", "<<<<<<< ours\n{}\n||||||| base\n{}\n=======\n{}\n>>>>>>> theirs\n", 4},
		{"written on Windows", "<<<<<<< HEAD\r\n{}\r\n=======\r\n{}\r\n>>>>>>> b\r\n", 3},
		{"bare markers and a longer marker size", "<<<<<<<\n========\n>>>>>>>>>>\n", 3},
		{"lines of marks", "{\"a\":1}\n{\"b\":\"<<<<<<< x\"}\n", 0},
		{"only like a marker", "<<<<<< six\n<<<<<<<x\n======= text\n=======x\n<<<<<<=\n", 0},
		{"nothing", "", 0},
	} {
		if got := markersIn([]byte(tt.data)); got != tt.want {
			t.Errorf("%s: markersIn(%q) = %d, want %d", tt.name, tt.data, got, tt.want)
		}
	}
}

// markersIn is how many lines of data are git's conflict markers.
func markersIn(data []byte) int {
	n := 0
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if conflictMarker(bytes.TrimSuffix(line, []byte("\r"))) {
			n++
		}
	}
	return n
}
