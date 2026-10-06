package vault

import (
	"cmp"
	"path"
	"strings"
	"unicode"
)

// chineseDigits maps the numeral characters that can open or continue a number.
// 零 and 〇 both appear in written vault paths for the same zero.
var chineseDigits = map[rune]int{
	'〇': 0, '零': 0, '一': 1, '二': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

// chineseUnits maps the multipliers a Chinese numeral is built from. Nothing
// larger than a thousand appears in a lesson number, so the rest stays text.
var chineseUnits = map[rune]int{'十': 10, '百': 100, '千': 1000}

// ComparePaths orders every list of vault paths by each segment's stem, then
// extension. Case is folded with Unicode simple folding, and runs of digits
// retain their numeric reading. Numbers sort after text: mixing code-point
// order with numeric order would let Latin letters, Chinese numerals and ASCII
// digits close a cycle. The original bytes settle a tie only after every
// segment agrees, so case and leading zeros cannot outrank a later filename.
func ComparePaths(a, b string) int {
	left, right := a, b
	for {
		as, at, amore := strings.Cut(left, "/")
		bs, bt, bmore := strings.Cut(right, "/")
		aext, bext := path.Ext(as), path.Ext(bs)
		if c := comparePathPart(strings.TrimSuffix(as, aext), strings.TrimSuffix(bs, bext)); c != 0 {
			return c
		}
		if c := comparePathPart(aext, bext); c != 0 {
			return c
		}
		if amore != bmore {
			if amore {
				return 1
			}
			return -1
		}
		if !amore {
			return strings.Compare(a, b)
		}
		left, right = at, bt
	}
}

// comparePathPart leaves equal numeric spellings and folded letters tied until
// the whole path has been read. Resolving that tie here would let Part01/z
// precede Part1/b on the spelling of the folder rather than its note's name.
func comparePathPart(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		an, aw := numberAt(ar, i)
		bn, bw := numberAt(br, j)
		if aw > 0 && bw > 0 {
			if c := cmp.Compare(an, bn); c != 0 {
				return c
			}
			i += aw
			j += bw
			continue
		}
		// Whether a number opens here is itself the answer, and outranks the
		// code points: no code-point ranking of the two kinds agrees with the
		// number reading, and one that disagrees closes a cycle.
		if (aw > 0) != (bw > 0) {
			if aw > 0 {
				return 1
			}
			return -1
		}
		if c := cmp.Compare(foldPathRune(ar[i]), foldPathRune(br[j])); c != 0 {
			return c
		}
		i++
		j++
	}
	// One is a prefix of the other, or they matched rune for rune.
	if c := cmp.Compare(len(ar)-i, len(br)-j); c != 0 {
		return c
	}
	return 0
}

// foldPathRune chooses one rune for the whole simple-fold orbit. A single
// lowercase conversion leaves final sigma separate from ordinary sigma.
func foldPathRune(r rune) rune {
	least := r
	for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
		least = min(least, next)
	}
	return least
}

// numberAt reads the number beginning at rs[i], returning its value and how
// many runes it spans. A width of zero means no number starts there.
func numberAt(rs []rune, i int) (value, width int) {
	if i >= len(rs) {
		return 0, 0
	}
	if isASCIIDigit(rs[i]) {
		return asciiNumberAt(rs, i)
	}
	return chineseNumberAt(rs, i)
}

// isASCIIDigit reports whether r is one of the ten digits a number is spelled
// with here. A digit belonging to another script is left as ordinary text: the
// vault numbers its notes with these ten and with the Chinese numerals, and the
// value read off any other script came out of subtracting '0' from a code point
// far away from it, which put a path where no number anybody wrote would.
func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// asciiNumberAt reads a run of decimal digits. Leading zeros carry no value, so
// 007 and 7 compare equal and the whole-path byte fallback settles them.
func asciiNumberAt(rs []rune, i int) (value, width int) {
	n := 0
	for i+n < len(rs) && isASCIIDigit(rs[i+n]) {
		// A path long enough to overflow this is not a numbered note.
		if value < 1<<40 {
			value = value*10 + int(rs[i+n]-'0')
		}
		n++
	}
	return value, n
}

// chineseNumberAt reads one Chinese numeral, composing units as written:
// 十二 is twelve, 二十 is twenty, 二十三 is twenty-three, 一百零五 is a hundred
// and five. A bare 十 opening the number means ten, which is how 十二課 is read.
func chineseNumberAt(rs []rune, i int) (value, width int) {
	total, section, seen := 0, 0, false
	n := 0
	for i+n < len(rs) {
		r := rs[i+n]
		if d, ok := chineseDigits[r]; ok {
			section = d
			seen = true
			n++
			continue
		}
		unit, ok := chineseUnits[r]
		if !ok {
			break
		}
		if section == 0 {
			// 十二 — the unit stands alone, so it counts once.
			section = 1
		}
		total += section * unit
		section = 0
		seen = true
		n++
	}
	if !seen {
		return 0, 0
	}
	return total + section, n
}
