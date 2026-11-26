package machinemarks

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/vault"
)

// open is every mark, whatever the privacy rules say: what a test wants to see.
func open(*Mark) bool { return true }

// clock is a time the test sets, in milliseconds since the epoch.
type clock struct{ ms int64 }

func (c *clock) now() time.Time { return time.UnixMilli(c.ms) }
func (c *clock) tick()          { c.ms += 1000 }

var clocks sync.Map

// clockOf is the clock of the machines of one test, which share it: a time is
// later on any machine than an earlier one on another.
func clockOf(t *testing.T) *clock {
	t.Helper()
	c, _ := clocks.LoadOrStore(t.Name(), &clock{ms: 1_700_000_000_000})
	ck, ok := c.(*clock)
	if !ok {
		t.Fatal("the clocks hold something that is not a clock")
	}
	return ck
}

// clone is one machine's working tree of a vault and its own configuration
// directory: what a computer has when the vault is shared through git.
type clone struct {
	t      *testing.T
	name   string
	root   string
	conf   string
	dir    confdir.Dir
	clock  *clock
	m      *Marks
	legacy func() ([]Mark, error)
}

func newClone(t *testing.T, name string) *clone {
	t.Helper()
	c := &clone{t: t, name: name, root: t.TempDir(), conf: t.TempDir(), clock: clockOf(t)}
	if err := os.MkdirAll(filepath.Join(c.root, "courses", "pg"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.root, "courses", "pg", "vacuum.md"), []byte("# Vacuum\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := confdir.VaultDir(confdir.At(c.conf), c.root)
	if err != nil {
		t.Fatal(err)
	}
	c.dir = dir
	c.restart()
	return c
}

// restart is a new process over the same vault and configuration directory.
func (c *clone) restart() {
	c.t.Helper()
	r, err := vault.Open(c.root)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = r.Close() }) //nolint:errcheck // the test is over
	f, err := machinefile.Open(r)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = f.Close() }) //nolint:errcheck // the test is over
	c.m = Open(f, c.dir, slog.New(slog.DiscardHandler), Options{Now: c.clock.now, Legacy: c.legacy})
}

func (c *clone) marksDir() string { return filepath.Join(c.root, "reader-notes", ".yomihon", "marks") }

// id is this machine's id, which its first write makes.
func (c *clone) id() string {
	c.t.Helper()
	return c.view().MachineID()
}

func (c *clone) view() *View {
	c.t.Helper()
	v, err := c.m.View()
	if err != nil {
		c.t.Fatal(err)
	}
	return v
}

// entry is the mark as this machine reads it, and fails if there is none.
func (c *clone) entry(id string) Entry {
	c.t.Helper()
	e, ok := c.view().Entry(id, open)
	if !ok {
		c.t.Fatalf("%s reads no mark %s", c.name, id)
	}
	return e
}

func (c *clone) put(m Mark, base Seen) Result { //nolint:gocritic // hugeParam: a test helper; the call sites read better with a value
	c.t.Helper()
	c.clock.tick()
	res, err := c.m.Put(&m, base, nil)
	if err != nil {
		c.t.Fatalf("%s: Put: %v", c.name, err)
	}
	return res
}

// say writes a thought with these words as the whole value of mark n, on top of
// what this machine reads of it now.
func (c *clone) say(n int, words string) Result {
	c.t.Helper()
	var base Seen
	if e, ok := c.view().Entry(mid(n), open); ok {
		base = e.Seen
	}
	return c.put(thought(n, words), base)
}

func (c *clone) delete(n int) Result {
	c.t.Helper()
	c.clock.tick()
	res, err := c.m.Delete(mid(n), c.entry(mid(n)).Seen, nil)
	if err != nil {
		c.t.Fatalf("%s: Delete: %v", c.name, err)
	}
	return res
}

// words is what the entry of mark n says now, with each other value after it.
func (c *clone) words(n int) string {
	c.t.Helper()
	e, ok := c.view().Entry(mid(n), open)
	if !ok {
		return "(none)"
	}
	var parts []string
	for i := range e.Values {
		v := &e.Values[i]
		w := v.Mark.Body
		if v.Mark.Deleted {
			w = "deleted:" + w
		}
		parts = append(parts, w)
	}
	return strings.Join(parts, " | ")
}

// pull stands for git: the machine files are what is shared, each machine
// changes only its own, and a pull brings the other's own file over the copy
// this working tree has.
func pull(from, to *clone) {
	to.t.Helper()
	name := from.id() + ".jsonl"
	if name == to.view().MachineID()+".jsonl" {
		return
	}
	data, err := os.ReadFile(filepath.Join(from.marksDir(), name)) // #nosec G304 -- a test's own path
	if err != nil {
		to.t.Fatal(err)
	}
	if err = os.MkdirAll(to.marksDir(), 0o750); err != nil {
		to.t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(to.marksDir(), name), data, 0o600); err != nil { // #nosec G703 -- a test's own path
		to.t.Fatal(err)
	}
}

// mid is the id of mark n.
func mid(n int) string { return fmt.Sprintf("%032x", n) }

func thought(n int, body string) Mark {
	return Mark{
		ID: mid(n), Kind: Thought, Path: "courses/pg/vacuum.md", Anchor: "dead-tuples", Body: body,
		At: time.Date(2026, 11, 21, 9, 0, n, 0, time.UTC),
	}
}

func highlight(n int, quote string) Mark {
	m := thought(n, "")
	m.Kind, m.Quote = Highlight, quote
	return m
}

// readFile is a file of the vault.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path) // #nosec G304 -- a test's own path
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// tree is every file under dir with what it holds, to see that nothing changed.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p) //nolint:errcheck // p is under dir
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		b, err := os.ReadFile(p) // #nosec G304 G122 -- a test's own path, not a link
		out[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// git runs git in dir, with an environment that is the test's own.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...) // #nosec G204 -- git, with the test's own arguments
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out.String())
	}
}

// gitTry runs git and answers what it said and whether it failed.
func gitTry(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...) // #nosec G204 -- git, with the test's own arguments
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err
}

// ledgerBytes is the ledger file as it is.
func (c *clone) ledgerBytes() []byte {
	b, _ := os.ReadFile(c.dir.Join("ledger.jsonl")) //nolint:errcheck // none is the empty ledger
	return b
}

// sorted is the keys of a set of words.
func sorted(s ...string) []string { slices.Sort(s); return s }
