package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFragmentMarksRemainDistinct(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, heading, missing string }{
		{"Hindi spacing mark", "की", "कि"},
		{"Thai nonspacing mark", "ปู่", "ปู"},
		{"Kana combining mark", "か\u309a", "か"},
		{"supplementary variation selector", "葛\U000e0100城", "葛城"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, syntax := range []struct{ prefix, rule string }{
				{"", "link.section_missing"},
				{"!", "embed.section_missing"},
			} {
				got := ruleTargets(fragmentRun(t, map[string]string{
					"Notes/Target.md": "## " + tt.heading + "\n\nPassage.\n",
					"Notes/Citer.md":  syntax.prefix + "[[Target#" + tt.heading + "]]\n" + syntax.prefix + "[[Target#" + tt.missing + "]]\n",
				}))
				want := []string{syntax.rule + " Target#" + tt.missing}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("%s fragment findings mismatch (-want +got):\n%s", syntax.rule, diff)
				}
			}
		})
	}
}
