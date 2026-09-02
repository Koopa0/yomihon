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
			name:  "paths carrying no number keep the order they always had",
			paths: []string{"運弓筆記.md", "曲目/巴哈.md", "Notes/query-planner.md"},
			want:  []string{"Notes/query-planner.md", "曲目/巴哈.md", "運弓筆記.md"},
		},
		{
			name:  "a numeral inside an ordinary word is still just a number",
			paths: []string{"一期一會.md", "一年之計.md"},
			want:  []string{"一年之計.md", "一期一會.md"},
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
// order consistently rather than comparing equal.
func TestComparisonIsTotal(t *testing.T) {
	t.Parallel()
	paths := []string{"第一課.md", "第1課.md", "第01課.md", "第一課.md", "abc", "", "十", "十十", "百"}
	for _, a := range paths {
		for _, b := range paths {
			ab, ba := ComparePaths(a, b), ComparePaths(b, a)
			if ab != -ba {
				t.Errorf("compare(%q,%q) = %d but compare(%q,%q) = %d; not antisymmetric", a, b, ab, b, a, ba)
			}
			if (a == b) != (ab == 0) {
				t.Errorf("compare(%q,%q) = %d; equal only when the strings are", a, b, ab)
			}
		}
	}
}
