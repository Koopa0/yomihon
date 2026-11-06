package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

// reportFile is one captured Markdown file under System/reports/ with the
// parsed note a scan would have handed navigation.
func reportFile(path, markdown string) capturedFile {
	return capturedFile{path: path, note: vault.Parse(path, []byte(markdown))}
}

// TestAReportIsCalledByWhatItSays pins the order a report is named in: its
// frontmatter title first, then the first level-one heading its body opens on,
// and only then its file; a report that says nothing still has words.
func TestAReportIsCalledByWhatItSays(t *testing.T) {
	t.Parallel()

	const dir = "System/reports/"
	tests := []struct {
		name string
		file capturedFile
		want string
	}{
		{
			name: "a frontmatter title is what a written report is called",
			file: reportFile(dir+"audit.md", "---\ntitle: Vault audit\n---\n# A different heading\n"),
			want: "Vault audit",
		},
		{
			name: "with no title the first level-one heading is the name",
			file: reportFile(dir+"2026-09-22 review.md", "# What the receiver leaves behind\n\nBody.\n"),
			want: "What the receiver leaves behind",
		},
		{
			name: "a heading after frontmatter without a title still names it",
			file: reportFile(dir+"review.md", "---\ncreated: 2026-09-22\n---\n\n# After the frontmatter\n"),
			want: "After the frontmatter",
		},
		{
			name: "lesser headings before it do not stand in for it",
			file: reportFile(dir+"review.md", "## Summary\n\ntext\n\n# The title\n"),
			want: "The title",
		},
		{
			name: "a heading inside a code fence is code, not a title",
			file: reportFile(dir+"review.md", "```sh\n# not a title\n```\n\n# The real one\n"),
			want: "The real one",
		},
		{
			name: "an underlined heading is a level-one heading",
			file: reportFile(dir+"review.md", "Underlined title\n================\n\nBody.\n"),
			want: "Underlined title",
		},
		{
			name: "a heading's words travel as typed, closing marks aside",
			file: reportFile(dir+"review.md", "#   Spaced   **words**   ##\n"),
			want: "Spaced **words**",
		},
		{
			name: "a heading with no words is passed over",
			file: reportFile(dir+"review.md", "# \n\n# Second\n"),
			want: "Second",
		},
		{
			name: "a report with no title and no heading is called by its file",
			file: reportFile(dir+"vault-check.md", "Just prose.\n"),
			want: "vault-check",
		},
		{
			name: "a report that could not be read is called by its file",
			file: capturedFile{path: dir + "unreadable.md"},
			want: "unreadable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildReports([]capturedFile{tt.file}, testContract(t).AuthoredDate())
			if len(got) != 1 {
				t.Fatalf("buildReports() = %d reports, want 1", len(got))
			}
			if got[0].Title != tt.want {
				t.Errorf("Title = %q, want %q", got[0].Title, tt.want)
			}
		})
	}
}

// TestNoteTitleStaysTheFrontmatterTitleOrTheFile keeps the report's reading of a
// heading out of the note model. A note is called by its frontmatter title or
// its file everywhere else in the product, and only a report's shelf also
// reads the heading.
func TestNoteTitleStaysTheFrontmatterTitleOrTheFile(t *testing.T) {
	t.Parallel()

	note := vault.Parse("System/reports/2026-09-22 review.md", []byte("# A heading the note model does not read\n"))
	if got, want := note.Title(), "2026-09-22 review"; got != want {
		t.Errorf("Note.Title() = %q, want %q", got, want)
	}
}

// TestSharedReportTitlesAreToldApart pins what keeps two reports that carry
// one title from reading alike. Both stay on the shelf; each gets a qualifier,
// which is its day when that day is its own among the reports sharing the
// title and its file name when it is not.
func TestSharedReportTitlesAreToldApart(t *testing.T) {
	t.Parallel()

	const dir = "System/reports/"
	tests := []struct {
		name  string
		files []capturedFile
		// want is each report's qualifier, by file name.
		want map[string]string
	}{
		{
			name: "two reports on one day are told apart by their files",
			files: []capturedFile{
				reportFile(dir+"2026-09-22 am.md", "# Review\n"),
				reportFile(dir+"2026-09-22 pm.md", "# Review\n"),
			},
			want: map[string]string{"2026-09-22 am.md": "2026-09-22 am.md", "2026-09-22 pm.md": "2026-09-22 pm.md"},
		},
		{
			name: "two reports with no day are told apart by their files",
			files: []capturedFile{
				reportFile(dir+"alpha.md", "# Review\n"),
				reportFile(dir+"beta.md", "---\ntitle: Review\n---\n"),
			},
			want: map[string]string{"alpha.md": "alpha.md", "beta.md": "beta.md"},
		},
		{
			name: "a day that is unique among the title's reports is enough",
			files: []capturedFile{
				reportFile(dir+"2026-07-01.md", "# Review\n"),
				reportFile(dir+"2026-07-02 am.md", "# Review\n"),
				reportFile(dir+"2026-07-02 pm.md", "# Review\n"),
			},
			want: map[string]string{
				"2026-07-01.md":    "2026-07-01",
				"2026-07-02 am.md": "2026-07-02 am.md",
				"2026-07-02 pm.md": "2026-07-02 pm.md",
			},
		},
		{
			name: "a title no other report carries has no qualifier",
			files: []capturedFile{
				reportFile(dir+"2026-07-01.md", "# One\n"),
				reportFile(dir+"2026-07-02.md", "# Two\n"),
			},
			want: map[string]string{"2026-07-01.md": "", "2026-07-02.md": ""},
		},
		{
			name: "titles that differ only in how a letter is composed are one title",
			files: []capturedFile{
				reportFile(dir+"2026-07-01.md", "# Caf\u00e9\n"),
				reportFile(dir+"2026-07-02.md", "# Cafe\u0301\n"),
			},
			want: map[string]string{"2026-07-01.md": "2026-07-01", "2026-07-02.md": "2026-07-02"},
		},
		{
			name: "titles that differ in case are two titles",
			files: []capturedFile{
				reportFile(dir+"2026-07-01.md", "# Review\n"),
				reportFile(dir+"2026-07-02.md", "# review\n"),
			},
			want: map[string]string{"2026-07-01.md": "", "2026-07-02.md": ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := map[string]string{}
			for _, report := range buildReports(tt.files, testContract(t).AuthoredDate()) {
				got[report.Name] = report.Qualifier
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("qualifiers (-want +got):\n%s", diff)
			}
		})
	}
}

// TestSharedReportTitlesStayOnTheShelf keeps the other half of the rule:
// telling two reports apart never drops one, and never changes the title
// either of them is called by.
func TestSharedReportTitlesStayOnTheShelf(t *testing.T) {
	t.Parallel()

	files := []capturedFile{
		reportFile("System/reports/2026-09-21.md", "# Review\n"),
		reportFile("System/reports/2026-09-22.md", "# Review\n"),
	}
	got := buildReports(files, testContract(t).AuthoredDate())
	if len(got) != 2 {
		t.Fatalf("buildReports() = %d reports, want both", len(got))
	}
	for _, report := range got {
		if report.Title != "Review" {
			t.Errorf("%s: Title = %q, want the title it carries", report.Name, report.Title)
		}
	}
}

// TestOnlyMarkdownDirectlyInTheFolderIsAReport pins the shelf's whole
// membership rule. An HTML file — one left in the daily-briefing folder a
// generator once wrote to included — a file in any folder below, and a file of
// any other kind stay in the folder tree and are not reports.
func TestOnlyMarkdownDirectlyInTheFolderIsAReport(t *testing.T) {
	t.Parallel()

	files := []capturedFile{
		reportFile("System/reports/2026-09-22 review.md", "# Review\n"),
		{path: "System/reports/daily-briefing/latest.html"},
		{path: "System/reports/daily-briefing/2026-09-22.html"},
		{path: "System/reports/stray.html"},
		reportFile("System/reports/archive/2026-01-01.md", "# Old\n"),
		{path: "System/reports/notes.txt"},
		reportFile("System/not-reports/2026-09-22.md", "# Elsewhere\n"),
	}
	var got []string
	for _, report := range buildReports(files, testContract(t).AuthoredDate()) {
		got = append(got, report.RelPath)
	}
	if diff := cmp.Diff([]string{"System/reports/2026-09-22 review.md"}, got); diff != "" {
		t.Errorf("reports (-want +got):\n%s", diff)
	}
}
