package status

import (
	"bytes"
	"errors"
	pathpkg "path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// FuzzNormalizeRelPath keeps the write face's first trust boundary local and
// canonical. Any accepted path must already have a stable slash form and OS
// form; normalizing that result again cannot change either representation.
func FuzzNormalizeRelPath(f *testing.F) {
	f.Add("Writing/lessons/japanese/L01.md")
	f.Add("Writing/./lessons/../notes/note.md")
	f.Add("../outside.md")
	f.Add("/absolute.md")
	f.Add(`Writing\outside.md`)
	f.Add("")
	f.Add("Writing/lessons/japanese/L01.md\x00")
	f.Add("Writing/lessons/japanese/L01.md\n\nnot the subject line")

	f.Fuzz(func(t *testing.T, rel string) {
		relSlash, osPath, err := normalizeRelPath(rel)
		if err != nil {
			if !errors.Is(err, ErrInvalidPath) {
				t.Errorf("normalizeRelPath(%q) error = %v, want ErrInvalidPath", rel, err)
			}
			return
		}

		if relSlash != pathpkg.Clean(rel) {
			t.Errorf("normalizeRelPath(%q) slash path = %q, want %q", rel, relSlash, pathpkg.Clean(rel))
		}
		if osPath != filepath.FromSlash(relSlash) {
			t.Errorf("normalizeRelPath(%q) OS path = %q, want %q", rel, osPath, filepath.FromSlash(relSlash))
		}
		if relSlash == "." || relSlash == ".." || strings.HasPrefix(relSlash, "../") ||
			pathpkg.IsAbs(relSlash) || strings.Contains(relSlash, `\`) || !filepath.IsLocal(osPath) {
			t.Errorf("normalizeRelPath(%q) accepted non-local result (%q, %q)", rel, relSlash, osPath)
		}

		againSlash, againOS, againErr := normalizeRelPath(relSlash)
		if againErr != nil || againSlash != relSlash || againOS != osPath {
			t.Errorf(
				"normalizeRelPath(%q) second pass = (%q, %q, %v), want (%q, %q, nil)",
				relSlash,
				againSlash,
				againOS,
				againErr,
				relSlash,
				osPath,
			)
		}
	})
}

// FuzzRewriteStatusLine proves the surgical rewrite owns exactly one line:
// every other frontmatter byte and every body byte remain unchanged. Status
// values are selected from contract-shaped literals because schema validation
// precedes this function in the production path.
func FuzzRewriteStatusLine(f *testing.F) {
	f.Add([]byte("---\ntitle: L01\nstatus: draft\n---\nbody\n"), uint8(0))
	f.Add([]byte("---\nstatus: draft\r\nother: value\n---\nbody\n"), uint8(1))
	f.Add([]byte("---\nstatus: draft\nstatus: ready\n---\nbody\n"), uint8(2))
	f.Add([]byte("status: draft\nbody\n"), uint8(0))
	f.Add([]byte("---\ntitle: x\n---\nstatus: body text\n"), uint8(1))

	statuses := [...]string{"ready", "draft", "archived"}
	f.Fuzz(func(t *testing.T, data []byte, choice uint8) {
		to := statuses[int(choice)%len(statuses)]
		got, err := rewriteStatusLine(data, to)
		if err != nil {
			if !errors.Is(err, ErrStatusLine) {
				t.Errorf("rewriteStatusLine(%q, %q) error = %v, want ErrStatusLine", data, to, err)
			}
			return
		}

		before, beforeFound := vault.SplitFrontmatter(data)
		after, afterFound := vault.SplitFrontmatter(got)
		if !beforeFound || !afterFound {
			t.Fatalf("rewriteStatusLine(%q, %q) succeeded without a complete frontmatter block", data, to)
		}
		if !bytes.Equal(before.Body, after.Body) {
			t.Errorf("rewriteStatusLine(%q, %q) changed body from %q to %q", data, to, before.Body, after.Body)
		}

		beforeLines := bytes.Split(before.Content, []byte("\n"))
		afterLines := bytes.Split(after.Content, []byte("\n"))
		if len(afterLines) != len(beforeLines) {
			t.Fatalf("rewriteStatusLine(%q, %q) line count = %d, want %d", data, to, len(afterLines), len(beforeLines))
		}

		changed := 0
		statusLines := 0
		for i := range beforeLines {
			if bytes.HasPrefix(beforeLines[i], []byte("status:")) {
				statusLines++
				checkOneRunBecame(t, data, to, beforeLines[i], afterLines[i])
			} else if !bytes.Equal(afterLines[i], beforeLines[i]) {
				t.Errorf("rewriteStatusLine(%q, %q) changed non-status line %d from %q to %q", data, to, i, beforeLines[i], afterLines[i])
			}
			if !bytes.Equal(afterLines[i], beforeLines[i]) {
				changed++
			}
		}
		if statusLines != 1 {
			t.Fatalf("rewriteStatusLine(%q, %q) succeeded with %d status lines", data, to, statusLines)
		}
		if changed > 1 {
			t.Errorf("rewriteStatusLine(%q, %q) changed %d lines, want at most the one status line", data, to, changed)
		}

		again, againErr := rewriteStatusLine(got, to)
		if againErr != nil || !bytes.Equal(again, got) {
			t.Errorf("rewriteStatusLine result is not idempotent: second = %q, %v; first = %q", again, againErr, got)
		}

		// The content identity spans everything but the one status line, so a
		// successful rewrite must never move it: a page rendered before a flip
		// keeps binding the flips that follow it.
		if vault.ContentIdentity(got) != vault.ContentIdentity(data) {
			t.Errorf("rewriteStatusLine(%q, %q) moved the content identity", data, to)
		}
	})
}

// checkOneRunBecame states what a surgical status write does to the one line
// it touches: the line after is the line before with a single contiguous run
// replaced by the target value, and the run that was replaced was a value and
// nothing else. Everything the author put either side of it — the key, the
// spacing they chose, a reason in a trailing comment, the quotes around the
// value, a carriage return — is outside the run and so must survive unchanged.
//
// It is derived from the two lines rather than from the span the writer used,
// so it is an oracle and not an echo: a writer that moved the wrong bytes
// fails here even if it moved them consistently. The earlier form asserted the
// line became "status: <to>" exactly, which was the whole-line rewrite written
// down as a law and could not tell that rewrite from a correct one.
//
// Every split is tried rather than the longest common prefix and suffix being
// trusted, because those are ambiguous exactly where this line is interesting:
// replacing "y" with "ready" leaves a "y" at the end of both lines, and a
// greedy suffix claims it for the surroundings and reports the value as
// "read". The lines are a few dozen bytes, so the exhaustive answer is free
// and it is the one that is actually true.
func checkOneRunBecame(t *testing.T, data []byte, to string, before, after []byte) {
	t.Helper()
	var firstReplaced []byte
	found := false
	for i := 0; i <= len(before); i++ {
		for j := i; j <= len(before); j++ {
			candidate := make([]byte, 0, i+len(to)+len(before)-j)
			candidate = append(candidate, before[:i]...)
			candidate = append(candidate, to...)
			candidate = append(candidate, before[j:]...)
			if !bytes.Equal(candidate, after) {
				continue
			}
			replaced := before[i:j]
			if !found {
				firstReplaced, found = replaced, true
			}
			if valueRunIsClean(before, i, j) {
				return
			}
		}
	}
	if !found {
		t.Errorf("rewriteStatusLine(%q, %q) turned %q into %q, which is not that line with one run replaced by the target", data, to, before, after)
		return
	}
	// A split exists but every one of them reaches past the value: the write
	// swallowed spacing or a comment that was not its to move.
	t.Errorf("rewriteStatusLine(%q, %q) replaced %q in %q, which reaches past the value", data, to, firstReplaced, before)
}

// valueRunIsClean reports whether line[i:j] could have been a status value's
// own text, judged with the bytes around it rather than the run alone —
// because what counts as the value depends on them.
//
// Quotes settle it: between a matching pair, the value is whatever they hold,
// spaces and "#" included, and "status: \' \'" really does carry one space. A
// run with no quotes around it is a plain scalar, and then spacing at its
// edges, a comment inside it, or a leading quote all mean the write reached
// past the value and took bytes the author chose. Only the leading position
// counts: a quote opens a quoted value there and is an ordinary character
// anywhere else, so "0000\"" is one plain token. A comment is likewise a "#"
// that follows whitespace, the only "#" YAML reads as one, which leaves
// "0#00" a single token that is right to replace whole.
func valueRunIsClean(line []byte, i, j int) bool {
	run := line[i:j]
	if bytes.Contains(run, []byte("\r")) {
		return false
	}
	if i > 0 && j < len(line) && (line[i-1] == '"' || line[i-1] == '\'') && line[j] == line[i-1] {
		return true
	}
	// Space and tab, which is all YAML counts as white space here. Trimming by
	// the Unicode definition instead would call a no-break space a blank and
	// reject a correct write of the one-character value "\u00a0".
	if !bytes.Equal(bytes.Trim(run, " \t"), run) {
		return false
	}
	if len(run) > 0 && (run[0] == '"' || run[0] == '\'') {
		return false
	}
	return !bytes.Contains(run, []byte(" #")) && !bytes.Contains(run, []byte("\t#"))
}
