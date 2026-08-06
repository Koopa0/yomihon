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
				want := []byte("status: " + to)
				if bytes.HasSuffix(beforeLines[i], []byte("\r")) {
					want = append(want, '\r')
				}
				if !bytes.Equal(afterLines[i], want) {
					t.Errorf("rewriteStatusLine(%q, %q) status line = %q, want %q", data, to, afterLines[i], want)
				}
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
	})
}
