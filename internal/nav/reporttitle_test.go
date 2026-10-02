package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

// reportFile is one captured file under System/reports/ with the bytes a
// scan would have handed navigation: a parsed note for Markdown, and the title
// the file's own HTML gave for a briefing.
func reportFile(path, markdown, htmlTitle string) capturedFile {
	file := capturedFile{path: path, htmlTitle: htmlTitle}
	if markdown != "" {
		file.note = vault.Parse(path, []byte(markdown))
	}
	return file
}

// TestAReportIsCalledByWhatItSays pins the order a report is named in. A
// briefing is called by what its own HTML says, and a written report by its
// frontmatter title first, then by the first level-one heading its body opens
// on, and only then by its file; a report that says nothing still has words.
func TestAReportIsCalledByWhatItSays(t *testing.T) {
	t.Parallel()

	const dir = "System/reports/"
	tests := []struct {
		name string
		file capturedFile
		want string
	}{
		{
			name: "a briefing is called by the title its HTML gave",
			file: reportFile(dir+"daily-briefing/latest.html", "", "接收者離開後 — Go 並行回顧"),
			want: "接收者離開後 — Go 並行回顧",
		},
		{
			name: "a briefing that gave none is called by its file",
			file: reportFile(dir+"daily-briefing/2026-07-02 briefing.html", "", ""),
			want: "2026-07-02 briefing.html",
		},
		{
			name: "a frontmatter title is what a written report is called",
			file: reportFile(dir+"audit.md", "---\ntitle: Vault audit\n---\n# A different heading\n", ""),
			want: "Vault audit",
		},
		{
			name: "with no title the first level-one heading is the name",
			file: reportFile(dir+"2026-09-22 review.md", "# What the receiver leaves behind\n\nBody.\n", ""),
			want: "What the receiver leaves behind",
		},
		{
			name: "a heading after frontmatter without a title still names it",
			file: reportFile(dir+"review.md", "---\ncreated: 2026-09-22\n---\n\n# After the frontmatter\n", ""),
			want: "After the frontmatter",
		},
		{
			name: "lesser headings before it do not stand in for it",
			file: reportFile(dir+"review.md", "## Summary\n\ntext\n\n# The title\n", ""),
			want: "The title",
		},
		{
			name: "a heading inside a code fence is code, not a title",
			file: reportFile(dir+"review.md", "```sh\n# not a title\n```\n\n# The real one\n", ""),
			want: "The real one",
		},
		{
			name: "an underlined heading is a level-one heading",
			file: reportFile(dir+"review.md", "Underlined title\n================\n\nBody.\n", ""),
			want: "Underlined title",
		},
		{
			name: "a heading's words travel as typed, closing marks aside",
			file: reportFile(dir+"review.md", "#   Spaced   **words**   ##\n", ""),
			want: "Spaced **words**",
		},
		{
			name: "a heading with no words is passed over",
			file: reportFile(dir+"review.md", "# \n\n# Second\n", ""),
			want: "Second",
		},
		{
			name: "a report with no title and no heading is called by its file",
			file: reportFile(dir+"vault-check.md", "Just prose.\n", ""),
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
// title and its file name when it is not, and the latest briefing needs none
// because it is a copy of the newest and wears its own mark.
func TestSharedReportTitlesAreToldApart(t *testing.T) {
	t.Parallel()

	const (
		briefings = "System/reports/daily-briefing/"
		reports   = "System/reports/"
	)
	tests := []struct {
		name  string
		files []capturedFile
		// want is each report's qualifier, by file name.
		want map[string]string
	}{
		{
			name: "the latest briefing and its dated twin",
			files: []capturedFile{
				reportFile(briefings+"latest.html", "", "Daily briefing"),
				reportFile(briefings+"2026-09-22.html", "", "Daily briefing"),
			},
			want: map[string]string{"latest.html": "", "2026-09-22.html": "2026-09-22"},
		},
		{
			name: "two reports on one day are told apart by their files",
			files: []capturedFile{
				reportFile(briefings+"2026-09-22-am.html", "", "Daily briefing"),
				reportFile(briefings+"2026-09-22-pm.html", "", "Daily briefing"),
			},
			want: map[string]string{"2026-09-22-am.html": "2026-09-22-am.html", "2026-09-22-pm.html": "2026-09-22-pm.html"},
		},
		{
			name: "two reports with no day are told apart by their files",
			files: []capturedFile{
				reportFile(briefings+"alpha.html", "", "Daily briefing"),
				reportFile(reports+"beta.md", "# Daily briefing\n", ""),
			},
			want: map[string]string{"alpha.html": "alpha.html", "beta.md": "beta.md"},
		},
		{
			name: "a day that is unique among the title's reports is enough",
			files: []capturedFile{
				reportFile(briefings+"2026-07-01.html", "", "Daily briefing"),
				reportFile(briefings+"2026-07-02-am.html", "", "Daily briefing"),
				reportFile(briefings+"2026-07-02-pm.html", "", "Daily briefing"),
			},
			want: map[string]string{
				"2026-07-01.html":    "2026-07-01",
				"2026-07-02-am.html": "2026-07-02-am.html",
				"2026-07-02-pm.html": "2026-07-02-pm.html",
			},
		},
		{
			name: "a title no other report carries has no qualifier",
			files: []capturedFile{
				reportFile(briefings+"2026-07-01.html", "", "One"),
				reportFile(briefings+"2026-07-02.html", "", "Two"),
			},
			want: map[string]string{"2026-07-01.html": "", "2026-07-02.html": ""},
		},
		{
			name: "titles that differ only in how a letter is composed are one title",
			files: []capturedFile{
				reportFile(briefings+"2026-07-01.html", "", "Café"),
				reportFile(briefings+"2026-07-02.html", "", "Café"),
			},
			want: map[string]string{"2026-07-01.html": "2026-07-01", "2026-07-02.html": "2026-07-02"},
		},
		{
			name: "titles that differ in case are two titles",
			files: []capturedFile{
				reportFile(briefings+"2026-07-01.html", "", "Review"),
				reportFile(briefings+"2026-07-02.html", "", "review"),
			},
			want: map[string]string{"2026-07-01.html": "", "2026-07-02.html": ""},
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
		reportFile("System/reports/daily-briefing/latest.html", "", "Daily briefing"),
		reportFile("System/reports/daily-briefing/2026-09-22.html", "", "Daily briefing"),
	}
	got := buildReports(files, testContract(t).AuthoredDate())
	if len(got) != 2 {
		t.Fatalf("buildReports() = %d reports, want both", len(got))
	}
	for _, report := range got {
		if report.Title != "Daily briefing" {
			t.Errorf("%s: Title = %q, want the title it carries", report.Name, report.Title)
		}
	}
}

// TestNewNamesABriefingByTheTitleItsCallerRead pins that the title read from a
// briefing's head reaches the shelf through New, keyed by the briefing's path,
// and that a briefing the caller read nothing for keeps its file name.
func TestNewNamesABriefingByTheTitleItsCallerRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const (
		titled = "System/reports/daily-briefing/2026-09-22.html"
		bare   = "System/reports/daily-briefing/2026-09-21.html"
	)
	writeNavFixture(t, root, titled, "<title>ignored here</title>")
	writeNavFixture(t, root, bare, "<p>no name</p>")

	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close: %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete: %v", err)
	}
	roles, policy := testCapabilities(t)
	contract := testContract(t)
	model := New(
		scan.Files(), nil, resolver(t),
		roles, contract.KnowledgeScope(), policy, contract.JournalDir(),
		contract.ArticleLanguage(), contract.AuthoredDate(), contract.Settlement(),
		map[string]string{titled: "What the receiver leaves behind"},
	)

	got := map[string]string{}
	for _, report := range model.Reports() {
		got[report.RelPath] = report.Title
	}
	want := map[string]string{
		titled: "What the receiver leaves behind",
		bare:   "2026-09-21.html",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("report titles (-want +got):\n%s", diff)
	}
}
