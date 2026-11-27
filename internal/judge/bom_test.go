package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestByteOrderMarkDoesNotHideTheFirstSectionFromTheJudge(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, prefix string }{
		{name: "no frontmatter", prefix: ""},
		{name: "frontmatter", prefix: "---\ntitle: Note\n---\n"},
		{name: "crlf frontmatter", prefix: "---\r\ntitle: Note\r\n---\r\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findingKeys(fragmentRun(t, map[string]string{
				"N.md": "\xef\xbb\xbf" + tt.prefix + "# BOM heading\n\nbody\n",
				"C.md": "[[N#BOM heading]]\n\n[[N#Absent]]\n",
			}))
			want := []string{"link.section_missing N#Absent"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("fragmentFindings(BOM heading) mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
