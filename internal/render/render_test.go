package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/render"
)

func TestHTML(t *testing.T) {
	t.Parallel()
	r := render.New()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "ruby markup passes through untouched",
			body: "<ruby>今日<rt>きょう</rt></ruby>は晴れ。",
			want: "<ruby>今日<rt>きょう</rt></ruby>",
		},
		{
			name: "explicit br passes through",
			body: "一行目<br>二行目",
			want: "<br>",
		},
		{
			name: "gfm table renders",
			body: "| a | b |\n|---|---|\n| 1 | 2 |\n",
			want: "<table>",
		},
		{
			name: "gfm task list renders",
			body: "- [ ] todo\n- [x] done\n",
			want: "type=\"checkbox\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := r.HTML(tt.body)
			if err != nil {
				t.Fatalf("HTML(%q) = %v", tt.body, err)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("HTML(%q) missing %q:\n%s", tt.body, tt.want, got)
			}
		})
	}
}
