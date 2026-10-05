package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDuplicateHeadingAllocationGrowth(t *testing.T) {
	inputs := [2]string{
		strings.Repeat("<h2>Repeat</h2>\n", 128),
		strings.Repeat("<h2>Repeat</h2>\n", 512),
	}
	var allocs [2]float64
	for i, input := range inputs {
		allocs[i] = testing.AllocsPerRun(3, func() {
			_, toc := assignHeadingIDs(input, "")
			if len(toc) != 128<<(2*i) {
				t.Fatal("allocation fixture did not render every heading")
			}
		})
	}
	growth := allocs[1] / allocs[0]
	t.Logf("128 headings: %.0f allocations; 512 headings: %.0f; growth: %.3f", allocs[0], allocs[1], growth)
	if growth >= 6 {
		t.Errorf("duplicate heading allocations grow by %.3f; want below 6 for four times the headings", growth)
	}
}

func TestDuplicateHeadingSuffixCollisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		reserved string
		want     string
		toc      []TOCEntry
	}{
		{
			name:  "authored suffix before repeats",
			input: "<h2>h-2</h2><h2>h</h2><h2>h</h2><h2>h-3</h2><h2>h</h2>",
			want:  `<h3 id="h-2" data-level="2">h-2</h3><h3 id="h" data-level="2">h</h3><h3 id="h-3" data-level="2">h</h3><h3 id="h-3-2" data-level="2">h-3</h3><h3 id="h-4" data-level="2">h</h3>`,
			toc:   []TOCEntry{{2, "h-2", "h-2"}, {2, "h", "h"}, {2, "h", "h-3"}, {2, "h-3", "h-3-2"}, {2, "h", "h-4"}},
		},
		{
			name:     "reserved title and authored suffix after repeats",
			input:    "<h2>h</h2><h2>h</h2><h2>h-2</h2><h2>h</h2>",
			reserved: "h",
			want:     `<h3 id="h-2" data-level="2">h</h3><h3 id="h-3" data-level="2">h</h3><h3 id="h-2-2" data-level="2">h-2</h3><h3 id="h-4" data-level="2">h</h3>`,
			toc:      []TOCEntry{{2, "h", "h-2"}, {2, "h", "h-3"}, {2, "h-2", "h-2-2"}, {2, "h", "h-4"}},
		},
		{
			name:     "reserved numeric suffix",
			input:    "<h2>h</h2><h2>h</h2><h2>h</h2>",
			reserved: "h-2",
			want:     `<h3 id="h" data-level="2">h</h3><h3 id="h-3" data-level="2">h</h3><h3 id="h-4" data-level="2">h</h3>`,
			toc:      []TOCEntry{{2, "h", "h"}, {2, "h", "h-3"}, {2, "h", "h-4"}},
		},
		{
			name:  "independent bases interleaved",
			input: "<h2>h</h2><h2>j</h2><h2>h</h2><h2>j</h2><h2>h</h2>",
			want:  `<h3 id="h" data-level="2">h</h3><h3 id="j" data-level="2">j</h3><h3 id="h-2" data-level="2">h</h3><h3 id="j-2" data-level="2">j</h3><h3 id="h-3" data-level="2">h</h3>`,
			toc:   []TOCEntry{{2, "h", "h"}, {2, "j", "j"}, {2, "h", "h-2"}, {2, "j", "j-2"}, {2, "h", "h-3"}},
		},
		{
			name:  "transclusion consumes suffix without contents entry",
			input: `<h2>h</h2><div class="embed"><h2>h</h2></div><h2>h</h2>`,
			want:  `<h3 id="h" data-level="2">h</h3><div class="embed"><h3 id="h-2" data-level="2">h</h3></div><h3 id="h-3" data-level="2">h</h3>`,
			toc:   []TOCEntry{{2, "h", "h"}, {2, "h", "h-3"}},
		},
		{
			name:  "nested heading remains untouched without claiming an id",
			input: "<h2>h<h3>h</h3></h2><h2>h</h2><h2>h</h2>",
			want:  `<h2>h<h3>h</h3></h2><h3 id="h" data-level="2">h</h3><h3 id="h-2" data-level="2">h</h3>`,
			toc:   []TOCEntry{{2, "h", "h"}, {2, "h", "h-2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, toc := assignHeadingIDs(tt.input, tt.reserved)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("heading HTML changed (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.toc, toc); diff != "" {
				t.Errorf("heading contents changed (-want +got):\n%s", diff)
			}
		})
	}
}

func BenchmarkDuplicateHeadingIDs(b *testing.B) {
	for _, n := range []int{128, 512, 1024} {
		input := strings.Repeat("<h2>Repeat</h2>\n", n)
		b.Run(fmt.Sprintf("headings-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, toc := assignHeadingIDs(input, "")
				if len(toc) != n {
					b.Fatal("benchmark did not render every heading")
				}
			}
		})
	}
}
