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

// chineseUnits maps the multipliers a Chinese numeral is built from. Anything
// larger than a thousand does not appear in a lesson or chapter number, and
// treating it as ordinary text is better than guessing at it.
var chineseUnits = map[rune]int{'十': 10, '百': 100, '千': 1000}

// ComparePaths orders two vault paths the way their reader would, and is the
// one order every list of them is sorted by: a course in the rail, a results
// page, a backlinks panel and a folder's uncited notes all answer with the
// same lessons in the same sequence.
//
// Paths are sorted for a reader, not for a machine, and a reader who numbered
// their notes expects them in that order. Comparing code points puts 第三課
// before 第二課, because 三 is U+4E09 and 二 is U+4E8C — an order with no meaning
// on screen, and one that gets worse with every lesson added. Arabic numerals
// fare no better: 第10課 lands between 第1課 and 第2課. So a run of digits is
// compared as the number it spells, and everything else by its code points,
// which leaves every path carrying no number exactly where it was.
//
// It falls back to a code-point comparison whenever the numbers are equal, so
// the order stays total and two paths never compare equal unless they are.
// Names derived from a path — the label a list shows for one — read the same
// way, so they are compared through here too.
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
	// One is a prefix of the other, or they matched rune for rune; the shorter
	// sorts first, and identical strings compare equal.
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
		// A path long enough to overflow this is not a numbered note; stop
		// growing the value and let the remaining digits compare as text.
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
