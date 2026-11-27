package sealedline

import (
	"bytes"
	"slices"
	"testing"
)

// FuzzSeal seals any words as a line and reads it back: it is whole, every part
// of it short of the whole is not, and neither is the line with any one byte
// changed. A file of lines reads the same with carriage returns as without.
func FuzzSeal(f *testing.F) {
	f.Add("words", 3, 7)
	f.Add("\"quoted\" \\ and 字 ,\"h\":\"0123456789abcdef\"}", 0, 0)
	f.Add("\xff\x00\u2028", 1000, 1000)
	f.Fuzz(func(t *testing.T, words string, cut, flip int) {
		line, err := Seal(struct {
			Words string `json:"words"`
		}{words})
		if err != nil {
			t.Fatal(err)
		}
		if bytes.IndexByte(line, '\n') >= 0 || Check(line) != Whole {
			t.Fatalf("Seal(%q) = %q, which does not read back whole on one line", words, line)
		}
		if n := abs(cut) % len(line); n > 0 && Check(line[:n]) == Whole {
			t.Fatalf("the first %d bytes of %q read as a whole line", n, line)
		}
		changed := bytes.Clone(line)
		at := abs(flip) % len(changed)
		changed[at] ^= 1 << (abs(flip) % 8)
		if Check(changed) == Whole {
			t.Fatalf("%q with byte %d changed reads as a whole line", line, at)
		}
		file := append(append(bytes.Clone(line), '\n'), line...)
		crlf := bytes.ReplaceAll(file, []byte("\n"), []byte("\r\n"))
		if got := Lines(crlf); !slices.EqualFunc(got, Lines(file), bytes.Equal) || len(got) != 2 {
			t.Fatalf("the lines of %q read differently with carriage returns", file)
		}
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
