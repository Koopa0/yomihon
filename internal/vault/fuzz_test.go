package vault

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// FuzzParse keeps the actual read-face parser — not a diagnostics copy — on
// arbitrary note bytes. It proves splitting is lossless when a block exists,
// Parse owns its result, and repeated parses classify the same bytes the same
// way.
func FuzzParse(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("plain body\n"))
	f.Add([]byte("---\ntitle: Note\nstatus: draft\n---\nbody\n"))
	f.Add([]byte("---\ntitle: \"unterminated\n---\nbody\n"))
	f.Add([]byte("---\na: 1\na: 2\n---\nbody\n"))
	f.Add([]byte("---\nbase: &base {title: Note}\nnote: {<<: *base}\n---\nbody\n"))
	f.Add([]byte("---\ntitle: Note\n"))

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 256<<10 {
			t.Skip()
		}
		data := bytes.Clone(input)
		block, found := SplitFrontmatter(data)
		blockAgain, foundAgain := SplitFrontmatter(data)
		if found != foundAgain || block.ContentStart != blockAgain.ContentStart || block.BodyStartLine != blockAgain.BodyStartLine ||
			!bytes.Equal(block.Content, blockAgain.Content) || !bytes.Equal(block.Body, blockAgain.Body) {
			t.Fatalf("SplitFrontmatter(%q) is not deterministic", data)
		}
		checkFrontmatterSplit(t, data, block, found)

		const rel = "Writing/fuzz.md"
		wantOwned := Parse(rel, bytes.Clone(data))
		first := Parse(rel, data)
		second := Parse(rel, data)
		if diff := cmp.Diff(first, second); diff != "" {
			t.Errorf("Parse(%q) is not deterministic (-first +second):\n%s", data, diff)
		}
		if first.RelPath != rel || first.Body != string(block.Body) {
			t.Errorf("Parse(%q) = path %q body %q, want path %q body %q", data, first.RelPath, first.Body, rel, block.Body)
		}
		if utf8.Valid(data) && !utf8.ValidString(first.Body) {
			t.Errorf("Parse(valid UTF-8 %q) returned invalid UTF-8 body %x", data, first.Body)
		}

		// Parse must not retain a mutable view of the caller's byte slice.
		if len(data) > 0 {
			data[0] ^= 0xff
			if diff := cmp.Diff(wantOwned, first); diff != "" {
				t.Errorf("Parse retained caller bytes (-before mutation +after):\n%s", diff)
			}
		}

		// These accessors are part of the read path and must remain total even
		// when YAML decoded a non-map or malformed frontmatter.
		_ = first.Title()
		_ = first.Status()
		_ = first.Type()
		_ = first.Domain()
		_ = first.Slug()
	})
}

func checkFrontmatterSplit(t *testing.T, data []byte, block FrontmatterSplit, found bool) {
	t.Helper()
	if !found {
		if !bytes.Equal(block.Body, data) {
			t.Errorf("SplitFrontmatter(%q) body = %q, want whole input", data, block.Body)
		}
		return
	}

	contentEnd := block.ContentStart + len(block.Content)
	if block.ContentStart < 0 || contentEnd > len(data) || !bytes.Equal(data[block.ContentStart:contentEnd], block.Content) {
		t.Errorf("SplitFrontmatter(%q) returned content outside its source", data)
	}
	if !bytes.HasSuffix(data, block.Body) {
		t.Errorf("SplitFrontmatter(%q) body %q is not a source suffix", data, block.Body)
	}
}
