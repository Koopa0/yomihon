package vault

import (
	"cmp"
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

// ComparePaths orders two vault paths the way their reader would, and is the
// one order every list of them sorts by. A run of digits compares as the number
// it spells, because comparing code points puts 第三課 before 第二課 and 第10課
// between 第1課 and 第2課; everything else compares by code point, so a path
// carrying no number stays where it was. Equal numbers fall back to a code-point
// comparison, which keeps the order total.
func ComparePaths(a, b string) int {
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
		if c := cmp.Compare(ar[i], br[j]); c != 0 {
			return c
		}
		i++
		j++
	}
	// One is a prefix of the other, or they matched rune for rune.
	if c := cmp.Compare(len(ar)-i, len(br)-j); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

// numberAt reads the number beginning at rs[i], returning its value and how
// many runes it spans. A width of zero means no number starts there.
func numberAt(rs []rune, i int) (value, width int) {
	if i >= len(rs) {
		return 0, 0
	}
	if unicode.IsDigit(rs[i]) {
		return asciiNumberAt(rs, i)
	}
	return chineseNumberAt(rs, i)
}

// asciiNumberAt reads a run of decimal digits. Leading zeros carry no value, so
// 007 and 7 compare equal and the code-point fallback settles them.
func asciiNumberAt(rs []rune, i int) (value, width int) {
	n := 0
	for i+n < len(rs) && unicode.IsDigit(rs[i+n]) {
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
