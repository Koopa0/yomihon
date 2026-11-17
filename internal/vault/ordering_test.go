package vault

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A reader who numbered their notes expects to find them in that order. Sorting
// by code point does not give it to them: 三 is U+4E09 and 二 is U+4E8C, so the
// third lesson lands above the second, and a semester of them makes the sidebar
// something to search rather than read.
func TestPathsSortTheWayTheirNumbersRead(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{
			name:  "Chinese numerals run in lesson order, not code-point order",
			paths: []string{"第三課 水.md", "第二課 空氣.md", "第一課 物質.md"},
			want:  []string{"第一課 物質.md", "第二課 空氣.md", "第三課 水.md"},
		},
		{
			name:  "ten and above compose rather than comparing by their first character",
			paths: []string{"第二課.md", "第十二課.md", "第十課.md", "第九課.md", "第二十課.md"},
			want:  []string{"第二課.md", "第九課.md", "第十課.md", "第十二課.md", "第二十課.md"},
		},
		{
			name:  "a hundred and five is read as one number, not as one-zero-five",
			paths: []string{"第一百零五課.md", "第一百課.md", "第九十九課.md"},
			want:  []string{"第九十九課.md", "第一百課.md", "第一百零五課.md"},
		},
		{
			name:  "Arabic numerals do not sort as text either",
			paths: []string{"第10課.md", "第2課.md", "第1課.md"},
			want:  []string{"第1課.md", "第2課.md", "第10課.md"},
		},
		{
			name:  "dates keep working, since their fields are already fixed width",
			paths: []string{"2026-07-30.md", "2026-07-28.md", "2026-07-29.md"},
			want:  []string{"2026-07-28.md", "2026-07-29.md", "2026-07-30.md"},
		},
		{
			name:  "paths carrying no number follow folded letter order",
			paths: []string{"運弓筆記.md", "曲目/巴哈.md", "Notes/query-planner.md"},
			want:  []string{"Notes/query-planner.md", "曲目/巴哈.md", "運弓筆記.md"},
		},
		{
			name:  "a numeral inside an ordinary word is a character, not a number",
			paths: []string{"一年之計.md", "Intro.md", "一、緒論.md"},
			want:  []string{"一、緒論.md", "Intro.md", "一年之計.md"},
		},
		{
			// 零 opens a number only where it numbers something. Opening 零值設計
			// it does not, so the word sorts by its code points, after the Latin
			// names beside it, and not above them as though it were note zero.
			name:  "a numeral opening an ordinary word does not jump the notes above it",
			paths: []string{"Go 零值設計.md", "Go Array.md", "Go CGO.md"},
			want:  []string{"Go Array.md", "Go CGO.md", "Go 零值設計.md"},
		},
		{
			// 第 is itself a character, so Zed comes first; after it the numeral
			// is a number whatever follows, and leads the text beside it.
			name:  "a numeral after 第 is a number whatever follows it",
			paths: []string{"第甲.md", "第十名.md", "Zed.md", "第二名.md"},
			want:  []string{"Zed.md", "第二名.md", "第十名.md", "第甲.md"},
		},
		{
			name:  "a numeral followed by a separator is a number",
			paths: []string{"Zed.md", "六 f.md", "三、c.md", "五_e.md", "一．a.md", "四-d.md", "二. b.md"},
			want:  []string{"一．a.md", "二. b.md", "三、c.md", "四-d.md", "五_e.md", "六 f.md", "Zed.md"},
		},
		{
			name: "a numeral followed by a counter is a number",
			paths: []string{
				"Zed.md", "十二章.md", "三節.md", "二課.md", "五篇.md", "四回.md",
				"七話.md", "八卷.md", "九部.md", "六期.md", "十講.md", "一單元.md",
			},
			want: []string{
				"一單元.md", "二課.md", "三節.md", "四回.md", "五篇.md", "六期.md",
				"七話.md", "八卷.md", "九部.md", "十講.md", "十二章.md", "Zed.md",
			},
		},
		{
			name:  "a numeral that ends the name is a number",
			paths: []string{"abc.md", "二.md", "一.md"},
			want:  []string{"一.md", "二.md", "abc.md"},
		},
		{
			name:  "numerals inside words sort by code point among the words",
			paths: []string{"零值設計.md", "Zed.md", "三國演義.md", "一年之計.md", "Alpha.md"},
			want:  []string{"Alpha.md", "Zed.md", "一年之計.md", "三國演義.md", "零值設計.md"},
		},
		{
			name:  "numbered notes lead the words around them",
			paths: []string{"README.md", "10-appendix.md", "Notes.md", "02-setup.md", "01-intro.md"},
			want:  []string{"01-intro.md", "02-setup.md", "10-appendix.md", "Notes.md", "README.md"},
		},
		{
			name:  "a date sorts before the words, for the same reason",
			paths: []string{"README.md", "2026-07-30.md", "Notes.md"},
			want:  []string{"2026-07-30.md", "Notes.md", "README.md"},
		},
		{
			name:  "a number after a shared prefix leads the words there too",
			paths: []string{"Chapter Intro.md", "Chapter 10.md", "Chapter 2.md"},
			want:  []string{"Chapter 2.md", "Chapter 10.md", "Chapter Intro.md"},
		},
		{
			name:  "folders are ordered by their leading number before their files are",
			paths: []string{"notes/a.md", "10 Archive/z.md", "2 Areas/b.md"},
			want:  []string{"2 Areas/b.md", "10 Archive/z.md", "notes/a.md"},
		},
		{
			// A digit belonging to another script is text here. Reading it as
			// a number would mean deciding what ２ and ٢ are worth, and the
			// arithmetic that used to run on them produced 65250 and 1586 —
			// values nobody wrote, which then decided where the path sorted.
			// The vault numbers its notes with these ten digits and with the
			// Chinese numerals; the rest is a character like any other.
			name:  "a digit from another script sorts as the character it looks like",
			paths: []string{"２.md", "٢.md", "1.md", "a.md"},
			want:  []string{"1.md", "a.md", "٢.md", "２.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := slices.Clone(tt.paths)
			slices.SortFunc(got, ComparePaths)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sorted %v mismatch (-want +got):\n%s", tt.paths, diff)
			}
		})
	}
}

// A comparison used for sorting has to be a total order, or the sort is free to
// produce anything. Two paths that spell the same number differently must still
// order consistently rather than comparing equal, and no three of them may close
// a cycle: slices.SortFunc reads a cycle as licence to return any order at all,
// so the same folder scanned in a different order would list differently and
// nothing would report it.
//
// The cycle needs three kinds of path present at once — one opening with a Latin
// letter, one opening with a Chinese numeral worth less than the Arabic one, and
// one opening with an Arabic digit — because it closes between those three and
// nowhere else. Drop any of the three and the triple loop below runs over a
// corpus that cannot make it fail.
func TestComparisonIsTotal(t *testing.T) {
	t.Parallel()
	paths := []string{
		"第一課.md", "第1課.md", "第01課.md", "第一課.md", "abc", "", "十", "十十", "百",
		"一期一會.md", "2.md", "A/z.md", "a/b.md", "K.md", "k.md", "K.md",
		"Σ.md", "ς.md", "σ.md", "Part01/z.md", "Part1/b.md", "a/", "a", "a\xff", "a\uFFFD",
		"一、緒論.md", "零值設計.md", "第三課.md", "三國.md", "一單元.md", "一單位.md", "Go 零.md", "Go 零值.md",
	}
	for _, a := range paths {
		for _, b := range paths {
			ab, ba := ComparePaths(a, b), ComparePaths(b, a)
			if ab != -ba {
				t.Errorf("compare(%q,%q) = %d but compare(%q,%q) = %d; not antisymmetric", a, b, ab, b, a, ba)
			}
			if (a == b) != (ab == 0) {
				t.Errorf("compare(%q,%q) = %d; equal only when the strings are", a, b, ab)
			}
			if ab >= 0 {
				continue
			}
			for _, c := range paths {
				if ac := ComparePaths(a, c); ComparePaths(b, c) < 0 && ac >= 0 {
					t.Errorf("compare(%q,%q) < 0 and compare(%q,%q) < 0, but compare(%q,%q) = %d; the three form a cycle", a, b, b, c, a, c, ac)
				}
			}
		}
	}
}

// A Chinese numeral numbers something only where it is written as numbering:
// after 第, before a separator or a counter, or ending the name. Anywhere else
// it is a character in a word, and numberAt says so by reading no number there.
// Arabic digits are numbers wherever they stand.
func TestChineseNumeralIsANumberOnlyWhereItLooksLikeNumbering(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		text         string
		at           int
		value, width int
	}{
		{"after 第", "第三課", 1, 3, 1},
		{"after 第 and ending the name", "第十二", 1, 12, 2},
		{"after 第 with a plain word following", "第二名", 1, 2, 1},
		{"before 、", "一、緒論", 0, 1, 1},
		{"before a full stop", "二. 方法", 0, 2, 1},
		{"before a full-width stop", "三．結語", 0, 3, 1},
		{"before a hyphen", "四-附錄", 0, 4, 1},
		{"before an underscore", "五_附錄", 0, 5, 1},
		{"before a space", "六 附錄", 0, 6, 1},
		{"before a counter", "二十三章", 0, 23, 3},
		{"before 單元", "一單元", 0, 1, 1},
		{"ending the name", "筆記二", 2, 2, 1},
		{"opening an ordinary word", "零值設計", 0, 0, 0},
		{"inside an ordinary word", "Go 零值設計", 3, 0, 0},
		{"as the first character of an idiom", "一年之計", 0, 0, 0},
		{"before a character that is not a counter", "三國", 0, 0, 0},
		{"before 單 that is not 單元", "一單位", 0, 0, 0},
		{"before a Latin letter", "一a", 0, 0, 0},
		{"a later numeral of the same run is no rescue", "甲二十三乙", 1, 0, 0},
		{"Arabic digits need nothing", "3rd", 0, 3, 1},
		{"Arabic digits after 第 or anywhere", "Go 12 notes", 3, 12, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value, width := numberAt([]rune(tt.text), tt.at)
			if value != tt.value || width != tt.width {
				t.Errorf("numberAt(%q, %d) = (%d, %d), want (%d, %d)", tt.text, tt.at, value, width, tt.value, tt.width)
			}
		})
	}
}

// A sort is only as good as the order under it: if three names could close a
// cycle, the same folder listed in another order would sort differently and
// nothing would report it. The set mixes every kind of name the numbering rule
// can tell apart — Arabic numbers, numerals that number, numerals that are
// characters in a word, and plain words — and every one of its orderings has to
// sort to the same list, which is also the list a reader expects.
func TestEveryPermutationOfAMixedSetSortsTheSame(t *testing.T) {
	t.Parallel()
	want := []string{
		"一、緒論.md", "2.md", "10-appendix.md",
		"Go Array.md", "Go 零值設計.md", "Notes.md", "一年之計.md", "第三課 水.md",
	}
	perm := slices.Clone(want)
	slices.Reverse(perm)
	seen := 0
	var generate func(k int)
	generate = func(k int) {
		if k == 1 {
			seen++
			got := slices.Clone(perm)
			slices.SortFunc(got, ComparePaths)
			if !slices.Equal(got, want) {
				t.Fatalf("sorting %q gave %q, want %q", perm, got, want)
			}
			return
		}
		generate(k - 1)
		for i := range k - 1 {
			// Heap's algorithm: each swap yields an arrangement not yet seen.
			if k%2 == 0 {
				perm[i], perm[k-1] = perm[k-1], perm[i]
			} else {
				perm[0], perm[k-1] = perm[k-1], perm[0]
			}
			generate(k - 1)
		}
	}
	generate(len(perm))
	if seen != 40320 {
		t.Errorf("visited %d orderings of %d names, want 40320", seen, len(want))
	}
}
