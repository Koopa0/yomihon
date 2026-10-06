package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestByteOrderMarkDoesNotHideTheFirstSectionFromTheJudge(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"", "---\ntitle: Note\n---\n", "---\r\ntitle: Note\r\n---\r\n"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			got := ruleTargets(fragmentRun(t, map[string]string{
				"N.md": "\xef\xbb\xbf" + prefix + "# BOM heading\n\nbody\n",
				"C.md": "[[N#BOM heading]]\n\n[[N#Absent]]\n",
			}))
			want := []string{"link.section_missing N#Absent"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("fragmentFindings(BOM heading) mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
