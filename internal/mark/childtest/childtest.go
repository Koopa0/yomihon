// Package childtest drives a second process that a test started from its own
// test binary: it reads what the process says, line by line, kills it at a
// moment the test leaves to chance, and reads what it said before it died.
// Only tests import it. It starts nothing itself: the test makes the command,
// since a name that starts a program is one no file the module ships may hold.
//
// A child says "begin <words>" as a step begins and "done <words>" once the
// step is answered for. What the words are is the test's.
package childtest

import (
	"bufio"
	"io"
	"strings"
	"testing"
	"time"
)

// Child is a running second process and what it has said.
type Child struct {
	lines chan string
	kill  func() error
}

// Watch reads out, the standard output of a process the test started, to its
// end, so that the process never waits on a full pipe. kill ends the process
// and waits for it.
func Watch(out io.Reader, kill func() error) *Child {
	// 64 lines is more than a child says between two kills.
	c := &Child{lines: make(chan string, 64), kill: kill}
	go func() {
		defer close(c.lines)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			c.lines <- scanner.Text()
		}
		_, _ = io.Copy(io.Discard, out) //nolint:errcheck // what is left after a kill says nothing
	}()
	return c
}

// KillAfter kills the child once it has begun begins steps, then, when until
// is not nil, once until holds, and then wait later; and answers the words of
// every step it said was done, in the order it said them. A child that stops
// before it is killed, or an until that does not hold within a minute, fails
// the test.
func (c *Child) KillAfter(t testing.TB, begins int, until func() bool, wait time.Duration) []string {
	t.Helper()
	var done []string
	read := func(line string) {
		if words, ok := strings.CutPrefix(line, "done "); ok {
			done = append(done, words)
		}
	}
	for begun := 0; begun < begins; {
		line, ok := <-c.lines
		if !ok {
			t.Fatal("the child stopped before it was killed")
		}
		if strings.HasPrefix(line, "begin ") {
			begun++
		}
		read(line)
	}
	for deadline := time.Now().Add(time.Minute); until != nil && !until(); {
		if time.Now().After(deadline) {
			t.Fatal("what the kill waits for did not happen in a minute")
		}
		select {
		case line, ok := <-c.lines:
			if !ok {
				t.Fatal("the child stopped before it was killed")
			}
			read(line)
		case <-time.After(50 * time.Microsecond):
		}
	}
	time.Sleep(wait)
	if err := c.kill(); err != nil {
		t.Fatal(err)
	}
	for line := range c.lines {
		read(line)
	}
	return done
}
