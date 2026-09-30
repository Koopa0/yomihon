package schema

import "fmt"

// maxContractDepth bounds how deep a contract may nest. A contract written for
// this schema reaches two levels — an array of tables, a table inside one —
// and a key of two or three parts; the bound leaves room for any honest
// contract many times over.
//
// It exists because the TOML decoder's cost grows with the square of the
// depth: each level copies the key path of every level above it. A contract of
// some twenty kilobytes nesting a few thousand levels deep costs the decoder
// close to a gigabyte, and the contract is the first file read when yomihon
// starts, so a vault that arrives by clone or sync could exhaust memory before
// anything else runs. A depth past the bound is refused before the decoder
// sees the bytes.
const maxContractDepth = 32

// checkContractDepth walks data once and refuses it when an array or inline
// table nests deeper than maxContractDepth, or a key names more parts than
// that. Strings and comments are passed over by the same rules the decoder
// reads them with, so a bracket or dot written inside one counts for nothing,
// and a string cannot hide the structure around it.
func checkContractDepth(data []byte) error {
	var walk depthWalk
	for i := 0; i < len(data); {
		if next, passed := passOver(data, i); passed {
			i = next
			continue
		}
		if !walk.step(data[i]) {
			return fmt.Errorf("contract nests deeper than %d levels", maxContractDepth)
		}
		i++
	}
	return nil
}

// passOver reports whether a comment or a string opens at i and, if one does,
// the index just past it. A quoted key part is still part of the key around
// it, so passing over one leaves the count of the key's dots alone.
func passOver(data []byte, i int) (int, bool) {
	switch data[i] {
	case '#':
		for i < len(data) && data[i] != '\n' {
			i++
		}
		return i, true
	case '"', '\'':
		return skipTOMLString(data, i), true
	}
	return i, false
}

// depthWalk is how deep the walk stands in arrays and inline tables, and how
// many dots the key it is reading has carried so far.
type depthWalk struct {
	depth, dots int
}

// step folds one byte outside any string or comment into the walk and reports
// whether the contract is still within the bound.
func (w *depthWalk) step(c byte) bool {
	switch {
	case c == '[' || c == '{':
		w.depth++
		w.dots = 0
		return w.depth <= maxContractDepth
	case c == ']' || c == '}':
		if w.depth > 0 {
			w.depth--
		}
		w.dots = 0
	case c == '.':
		w.dots++
		return w.dots <= maxContractDepth
	case keyByte(c):
	default:
		// Anything else ends a key: "=", a comma, a line break. A number
		// with a fraction carries one dot, so it never nears the bound.
		w.dots = 0
	}
	return true
}

// keyByte reports whether c can stand inside a dotted key without ending it:
// a bare-key character, the spaces a key may carry around its dots, or any
// byte of a non-ASCII character, which is counted as part of the key rather
// than trusted to end it.
func keyByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '_' || c == '-' || c == ' ' || c == '\t' || c >= 0x80
}

// skipTOMLString returns the index just past the string that opens at i, which
// holds a double or single quote. Three quotes open a multi-line string; any
// other opening is a single-line one. A double-quoted string honours backslash
// escapes and a single-quoted one does not.
func skipTOMLString(data []byte, i int) int {
	quote := data[i]
	if i+2 < len(data) && data[i+1] == quote && data[i+2] == quote {
		return skipMultiLine(data, i+3, quote)
	}
	return skipSingleLine(data, i+1, quote)
}

// skipMultiLine returns the index just past a multi-line string whose content
// starts at j. It closes at the last three of a run of up to five quotes, as
// the decoder reads it, and one that never closes runs to the end.
func skipMultiLine(data []byte, j int, quote byte) int {
	for j < len(data) {
		switch {
		case quote == '"' && data[j] == '\\':
			j += 2
		case data[j] == quote && j+2 < len(data) && data[j+1] == quote && data[j+2] == quote:
			end := j + 3
			for extra := 0; extra < 2 && end < len(data) && data[end] == quote; extra++ {
				end++
			}
			return end
		default:
			j++
		}
	}
	return len(data)
}

// skipSingleLine returns the index just past a single-line string whose
// content starts at j. One that meets a line break has ended, as far as this
// walk is concerned — the decoder refuses it — and one that never closes runs
// to the end.
func skipSingleLine(data []byte, j int, quote byte) int {
	for j < len(data) {
		switch data[j] {
		case quote:
			return j + 1
		case '\n':
			return j
		case '\\':
			if quote == '"' {
				j++
			}
		}
		j++
	}
	return len(data)
}
