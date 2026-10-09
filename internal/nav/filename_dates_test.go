package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestFilenameDatesRejectImpossibleCalendarDays(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cases := map[string]string{
		"2026-13-01": "",
		"2026-00-01": "",
		"2026-01-00": "",
		"2026-01-32": "",
		"2026-02-30": "",
		"2026-02-29": "",
		"1900-02-29": "",
		"2026-04-31": "",
		"2024-02-29": "2024-02-29",
		"2000-02-29": "2000-02-29",
		"2026-12-31": "2026-12-31",
	}
	want := map[string]string{}
	for stem, day := range cases {
		for _, pair := range [][2]string{{"Diary/", ".md"}, {"System/reports/", ".md"}, {"System/reports/daily-briefing/", ".html"}} {
			rel := pair[0] + stem + pair[1]
			writeNavFixture(t, root, rel, "synthetic content\n")
			want[rel] = day
		}
	}
	writeNavFixture(t, root, "Diary/2026-13-01 authored.md", "---\ncreated: 2026-07-10\n---\nbody\n")
	want["Diary/2026-13-01 authored.md"] = "2026-07-10"
	writeNavFixture(t, root, "System/reports/2026-13-01 authored.md", "---\ncreated: 2026-07-10\n---\nbody\n")
	want["System/reports/2026-13-01 authored.md"] = "2026-07-10"
	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, schema.KnowledgeScope{}, policy, nil)
	got := map[string]string{}
	for _, entry := range model.Journal() {
		got[entry.RelPath] = entry.Date
	}
	for _, entry := range model.Reports() {
		got[entry.RelPath] = entry.Date
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("filename dates admitted an impossible calendar day (-want +got):\n%s", diff)
	}
}
