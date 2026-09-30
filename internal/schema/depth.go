package schema

import "fmt"

// maxContractDepth bounds how deep arrays and inline tables may nest, and
// maxContractPath and maxContractPathBytes how long the decoder's key path may
// grow, in parts and in bytes: the table header, each key that opens an inline
// table on the way down, and the key being read. A contract written for this
// schema reaches two levels and a path of four parts and a few dozen bytes;
// the bounds leave room for any honest contract many times over.
//
// They exist because the TOML decoder copies the whole key path for every key
// it reads, so its cost grows with the length of that path times the number
// of keys. A contract of some twenty kilobytes that nests a few thousand
// levels deep, or chains a few thousand key parts, costs the decoder close to
// a gigabyte, and one long key part above a few thousand short keys costs it
// several; the contract is the first file read when yomihon starts, so a
// vault that arrives by clone or sync could exhaust memory before anything
// else runs. Depth, part count and bytes are bounded together, as one path,
// because any one of them alone can be kept small while the others grow. A
// contract past a bound is refused before the decoder sees the bytes.
const (
	maxContractDepth     = 32
	maxContractPath      = 32
	maxContractPathBytes = 256
)

// maxContractBytes bounds the size of a contract the decoder is handed. The
// path bounds keep each key cheap; this keeps the number of keys finite. A
// contract for this schema is a few kilobytes.
const maxContractBytes = 256 << 10

// checkContractDepth walks data once and refuses it when it is larger than
// maxContractBytes, nests arrays and inline tables deeper than
// maxContractDepth, or builds a key path longer than maxContractPath parts or
// maxContractPathBytes bytes. Strings and comments are passed over by the
// same rules the decoder reads them with, so a bracket or dot written inside
// one counts for nothing, and a string cannot hide the structure around it.
func checkContractDepth(data []byte) error {
	if len(data) > maxContractBytes {
		return fmt.Errorf("contract is larger than %d bytes", maxContractBytes)
	}
	var walk depthWalk
	for i := 0; i < len(data); {
		if next, passed := walk.passOver(data, i); passed {
			i = next
			continue
		}
		if !walk.step(data[i]) {
			return fmt.Errorf("contract nests deeper than %d levels, or builds a key path longer than %d parts or %d bytes", maxContractDepth, maxContractPath, maxContractPathBytes)
		}
		i++
	}
	return nil
}

// span is the length of a key path in the two measures the decoder's cost
// follows: its parts, and the bytes they are written in. A quoted part counts
// its quotes and escapes, so the count never falls short of what the decoder
// keeps.
type span struct {
	parts, bytes int
}

func (s span) add(t span) span {
	return span{parts: s.parts + t.parts, bytes: s.bytes + t.bytes}
}

// fits reports whether a path of this span is within both bounds.
func (s span) fits() bool {
	return s.parts <= maxContractPath && s.bytes <= maxContractPathBytes
}

// frame is one open array or inline table: the key path its contents are read
// under, and whether it is an array, whose contents are values, not keys.
type frame struct {
	path  span
	array bool
}

// depthWalk is where the walk stands: the open arrays and inline tables, the
// table header above them, and the key being read.
type depthWalk struct {
	open     []frame
	header   span // the current table header
	inHeader int  // brackets open in a table header being read; 0 outside one
	dots     int  // dots in the key being read
	keyBytes int  // bytes of the key being read
	opener   span // the key whose value follows its "="
	value    bool // what follows is a value, not a key
}

// key is the span of the key being read: one part, and one more for each dot.
func (w *depthWalk) key() span {
	return span{parts: w.dots + 1, bytes: w.keyBytes}
}

// path is the key path the next key is read under.
func (w *depthWalk) path() span {
	if n := len(w.open); n > 0 {
		return w.open[n-1].path
	}
	return w.header
}

// inArray reports whether the innermost open container is an array.
func (w *depthWalk) inArray() bool {
	n := len(w.open)
	return n > 0 && w.open[n-1].array
}

// passOver reports whether a comment or a string opens at i and, if one does,
// the index just past it. A quoted key part is still part of the key around
// it: passing over one adds its bytes to the key and leaves its dots alone.
func (w *depthWalk) passOver(data []byte, i int) (int, bool) {
	switch data[i] {
	case '#':
		for i < len(data) && data[i] != '\n' {
			i++
		}
		return i, true
	case '"', '\'':
		next := skipTOMLString(data, i)
		if !w.value {
			w.keyBytes += next - i
		}
		return next, true
	}
	return i, false
}

// step folds one byte outside any string or comment into the walk and reports
// whether the contract is still within the bounds.
func (w *depthWalk) step(c byte) bool {
	switch {
	case c == '[' && w.readingHeader(), c == ']' && w.inHeader > 0:
		return w.headerBracket(c)
	case c == '[' || c == '{':
		return w.push(c == '[')
	case c == ']' || c == '}':
		w.pop()
	case c == '.' || c == '=':
		return w.keyMark(c)
	case c == ',' || c == '\n':
		// A comma or a line break ends a key or a value.
		w.endKey(w.inArray())
	case keyByte(c) && !w.value:
		w.keyBytes++
	}
	// Anything else — a byte of a value, or a colon or sign inside one — leaves
	// the walk where it stands.
	return true
}

// readingHeader reports whether a "[" read now belongs to a table header: one
// already being read, or a bracket at the top that no "=" introduced.
func (w *depthWalk) readingHeader() bool {
	return w.inHeader > 0 || len(w.open) == 0 && !w.value
}

// headerBracket folds a bracket of a table header into the walk. A header's
// own path starts over from nothing, and is judged once its brackets close.
func (w *depthWalk) headerBracket(c byte) bool {
	if c == '[' {
		if w.inHeader == 0 {
			w.header, w.dots, w.keyBytes = span{}, 0, 0
		}
		w.inHeader++
		return true
	}
	w.inHeader--
	if w.inHeader > 0 {
		return true
	}
	w.header, w.dots, w.keyBytes = w.key(), 0, 0
	return w.header.fits()
}

// keyMark folds a dot or an "=" into the key being read and reports whether
// the key, read under the current path, is still within the bounds.
func (w *depthWalk) keyMark(c byte) bool {
	if c == '=' {
		w.opener = w.key()
		w.dots, w.keyBytes, w.value = 0, 0, true
		return w.path().add(w.opener).fits()
	}
	if w.value {
		// A dot inside a value is a fraction or a time, not a key part.
		return true
	}
	w.dots++
	return w.path().add(w.key()).fits()
}

// endKey forgets the key being read and the key that opened the value, and
// says whether a value or a key follows.
func (w *depthWalk) endKey(value bool) {
	w.dots, w.keyBytes, w.opener, w.value = 0, 0, span{}, value
}

// push opens an array or inline table. Its contents are read under the path
// it was opened under, lengthened by the key that opened it when an "="
// introduced it; an element of an array adds nothing, since the array's own
// key already did.
func (w *depthWalk) push(array bool) bool {
	path := w.path()
	if w.value && !w.inArray() {
		path = path.add(w.opener)
	}
	w.open = append(w.open, frame{path: path, array: array})
	w.endKey(array)
	return len(w.open) <= maxContractDepth && path.fits()
}

// pop closes the innermost open array or inline table.
func (w *depthWalk) pop() {
	if n := len(w.open); n > 0 {
		w.open = w.open[:n-1]
	}
	w.endKey(false)
}

// keyByte reports whether c is a byte of a bare key part: a bare-key
// character, or any byte of a non-ASCII character, which is counted as part of
// the key rather than trusted to be anything else. The spaces a key may carry
// around its dots are no part of it, and leave the walk where it stands.
func keyByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '_' || c == '-' || c >= 0x80
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
