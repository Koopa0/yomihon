package vault

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"unicode"
	"unicode/utf8"
)

// EndsALine reports whether r can end a line or a control sequence: a C0 or C1
// control, DELETE, or one of the two Unicode separators that end a line without
// being a newline. It is the one definition of that set.
func EndsALine(r rune) bool {
	return unicode.IsControl(r) || r == 0x2028 || r == 0x2029
}

// CheckStoredPath says p is a path the reader's own files may hold: valid
// UTF-8, not empty and at most maxBytes long, a local vault-relative slash path
// other than ".", one line of text, and in NFC, the spelling every face names a
// note by. fs.ValidPath lets a NUL or a line break through, and a path that
// holds one is not one line to put on a page; a file whose own name carries one
// is refused here, which is accepted because such names are very rare.
func CheckStoredPath(p string, maxBytes int) error {
	switch {
	case !utf8.ValidString(p):
		return errors.New("the path is not valid UTF-8")
	case p == "" || len(p) > maxBytes:
		return errors.New("the path is empty or too long")
	case !fs.ValidPath(p) || p == ".":
		return fmt.Errorf("%q is not a local vault-relative path", p)
	case strings.ContainsFunc(p, EndsALine):
		return fmt.Errorf("%q carries a control character", p)
	case p != NormalizeNFC(p):
		return fmt.Errorf("%q is not written in NFC", p)
	}
	return nil
}
