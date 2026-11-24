package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestHereLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dir  string
		want string
	}{
		{"Concepts/golang", "golang"},
		{"Inbox", "Inbox"},
		{"a/b/c", "c"},
		{"", "書庫根目錄"},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			t.Parallel()
			if got := hereLabel(tt.dir, wording.ZhHant); got != tt.want {
				t.Errorf("hereLabel(%q, wording.ZhHant) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

// buildModel writes a small vault to disk, captures it through vault.Reader,
// and builds the real graph and navigation projections from that generation.
func buildModel(t *testing.T) *nav.Model {
	t.Helper()
	_, model := buildVault(t)
	return model
}

// buildVault is buildModel with the folder it wrote kept, for the recordings
// that read a note's own bytes as well as the shape navigation made of them.
func buildVault(t *testing.T) (root string, model *nav.Model) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		// A study-path that lists L01 twice — once deep under Decode > Bytes, once
		// directly under Review — so the reverse index yields two placements. Its
		// opening is two paragraphs above the first heading: what the course is
		// and who it is for, which is what a course page prints under its title.
		"Maps/Go path.md": "---\ntype: study-path\n---\n" +
			"這條路徑講的是 Go 的讀法：從位元開始，走到讀得懂一份原始碼。\n\n" +
			"寫給已經會另一種語言、想把 Go 讀進去的人。\n\n" +
			"## decode | Decode | 解碼\n\n" +
			"### bytes | Bytes | 位元 {sequence=primary}\n\n" +
			"- [[L01]]\n- [[L02]]\n- [[Template target]]\n- [[Unwritten Lesson]]\n- [[Repeat|Ambiguous Lesson]]\n\n" +
			"## review | Review | 複習 {sequence=primary}\n\n" +
			"- [[L01]]\n",
		// A newly added topic map lists a concept. No application registration
		// names this file; its type and content are enough to grow the sidebar.
		"Maps/Reading map.md": "---\ntype: topic-map\n---\n" +
			"## Themes\n\n- [[C01]]\n- [[Ghost]]\n",
		"Writing/lessons/go/L01.md": "---\ntype: lesson\ndraft: true\n---\nbody\n",
		"Writing/lessons/go/L02.md": "---\ntype: lesson\ndraft: true\n---\nbody\n",
		// A concept note not listed by any study-path.
		"Concepts/go/C01.md": "---\ntype: concept\n---\nbody\n",
		"Concepts/go/C02.md": "---\ntype: concept\n---\nbody\n",
		"A/Repeat.md":        "body\n",
		"B/Repeat.md":        "body\n",
		"System/templates/Template map.md": "---\ntype: topic-map\n---\n# TEMPLATE MAP SENTINEL\n\n" +
			"## Shelf\n- [[L01]]\n",
		"System/templates/Template target.md": "---\ntype: concept\n---\n# TEMPLATE TARGET SENTINEL\n\nbody\n",
		// A Sources note with no frontmatter at all (a legal shape).
		"Sources/articles/Other.md": "just prose, no frontmatter\n",
		"Sources/articles/Raw.md":   "raw clipping, no frontmatter\n",
		// Journal entries deliberately have no frontmatter. There are more of
		// them than the rail's drawer shows, so a recording of that drawer is a
		// recording of it narrowing rather than of a journal that happens to be
		// short.
		"Diary/2026-07-02.md": "# Two\n",
		"Diary/2026-07-03.md": "# Three\n",
		"Diary/2026-07-04.md": "# Four\n",
		"Diary/2026-07-05.md": "# Five\n",
		"Diary/2026-07-06.md": "# Six\n",
		"Diary/2026-07-09.md": "# Earlier\n",
		"Diary/2026-07-10.md": "# Latest\n",
		// A file at the vault root, belonging to no folder: the folder shelf
		// lists these after the folders, and without one here no recording
		// would ever draw that kind of row.
		"Reading list.md": "# Reading list\n",
		// A report, so the reports mode has something to narrow. With none,
		// every check that holds a block to its page compared nothing for it.
		"System/reports/2026-07-10 vault audit.md": "# Vault audit\n\nbody\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	// A date a recording draws must not depend on the day the test runs or the
	// zone it runs in, so every file starts at local midday on one day.
	mtimes := map[string]time.Time{}
	for rel := range files {
		mtimes[rel] = time.Date(2026, time.July, 1, 12, 0, 0, 0, time.Local)
	}
	mtimes["Diary/2026-07-09.md"] = time.Date(2026, time.July, 9, 8, 0, 0, 0, time.UTC)
	mtimes["Diary/2026-07-10.md"] = time.Date(2026, time.July, 10, 8, 0, 0, 0, time.UTC)
	for rel, modified := range mtimes {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.Chtimes(full, modified, modified); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", rel, err)
		}
	}
	return root, modelOf(t, root)
}

// modelOf reads the folder at root the way a running server does and answers
// with the navigation model built from it.
func modelOf(t *testing.T, root string) *nav.Model {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	notes := make(map[string]*vault.Note)
	noteList := make([]*vault.Note, 0, len(scan.Files()))
	resources := make([]string, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		if !strings.HasSuffix(entry.Path(), ".md") {
			resources = append(resources, entry.Path())
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	contract, err := schema.LoadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	return nav.New(
		scan.Files(),
		notes,
		graph.New(noteList, resources),
		contract.NavigationRoles(),
		contract.KnowledgeScope(),
		contract.ArtifactPolicy(),
		contract.JournalDir(),
		contract.ArticleLanguage(), contract.AuthoredDate())
}

// TestSidebarMarksDisclosureStateForTheScript locks the HTML contract the
// disclosure state owner reads: every course branch carries a stable data-key,
// the current note's wayfinding chain also carries data-chain (forced open,
// never persisted), and the pre-paint restore script and the filter's no-match
// notice ride with the sidebar. Rendered attributes come from a map, so the
// marker pair appears in alphabetical order (data-chain before data-key).
func TestSidebarMarksDisclosureStateForTheScript(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	sb := NewNoteSidebar(nav.Shell{Nav: model}, "Writing/lessons/go/L01.md", "golang")

	var buf bytes.Buffer
	if err := sidebar(sb, layouts.Chrome{Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		`data-chain data-key="bookbranch:`,
		`data-filter-empty`,
		`yomihon.nav`,
		`<script nonce="response-nonce">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered sidebar is missing %q", want)
		}
	}
}

// TestSidebarRendersNavigationCapabilityDiagnostics pins the one signal a
// reader gets outside Home: a note opened straight from the command palette
// never renders Home, so a contract whose declarations could not be read has to
// say so in the rail or nowhere.
func TestSidebarRendersNavigationCapabilityDiagnostics(t *testing.T) {
	t.Parallel()

	contract := rejectedCapabilityContract(t)
	model := nav.New(
		nil, nil, graph.BuildFromNotes(nil, nil),
		contract.NavigationRoles(),
		contract.KnowledgeScope(),
		contract.ArtifactPolicy(),
		contract.JournalDir(),
		contract.ArticleLanguage(), contract.AuthoredDate())
	if model.NavigationClosure().Diagnostic() == "" || model.ArtifactClosure().Diagnostic() == "" {
		t.Fatalf("fixture produced no capability fault: navigation %q artifact %q",
			model.NavigationClosure().Diagnostic(), model.ArtifactClosure().Diagnostic())
	}
	var buf bytes.Buffer
	if err := sidebar(NewLibrarySidebar(nav.Shell{Nav: model}, LibraryNone, ""), layouts.Chrome{Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-sidebar-group="navigation-diagnostics"`,
		"課程與索引",
		"筆記清單目前無法使用",
		// The sentences themselves, HTML-escaped exactly as the page writes them.
		htmlEscape(model.NavigationClosure().Diagnostic()),
		htmlEscape(model.ArtifactClosure().Diagnostic()),
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered degraded sidebar is missing %q", want)
		}
	}
}

// TestSidebarRendersRejectedJournalDirDiagnostic is the journal half of the
// capability-fault lock: a contract that wrote folders.journal and could not honour
// it must say so on the rail. Gating the drawer on an empty projection alone
// made a typo look like a vault that never kept a journal.
func TestSidebarRendersRejectedJournalDirDiagnostic(t *testing.T) {
	t.Parallel()

	contract := rejectedJournalDirContract(t)
	model := nav.New(
		nil, nil, graph.BuildFromNotes(nil, nil),
		contract.NavigationRoles(),
		contract.KnowledgeScope(),
		contract.ArtifactPolicy(),
		contract.JournalDir(),
		contract.ArticleLanguage(), contract.AuthoredDate())
	if !model.JournalClosure().Closed() || model.JournalClosure().Diagnostic() == "" {
		t.Fatalf("fixture produced no journal fault: closed=%t diagnostic=%q",
			model.JournalClosure().Closed(), model.JournalClosure().Diagnostic())
	}
	var buf bytes.Buffer
	if err := sidebar(NewLibrarySidebar(nav.Shell{Nav: model}, LibraryNone, ""), layouts.Chrome{Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-sidebar-group="journal-diagnostics"`,
		"日記目前無法使用。",
		htmlEscape(model.JournalClosure().Diagnostic()),
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered rejected-journal sidebar is missing %q", want)
		}
	}
}

// TestSidebarSaysNothingForAnUngovernedFolder is the other half: a folder that
// carries no contract declared nothing, so it has no paths and no maps, and the
// rail must not apologise on every page for the ordinary shape of a directory.
func TestSidebarSaysNothingForAnUngovernedFolder(t *testing.T) {
	t.Parallel()

	model := nav.New(
		nil, nil, graph.BuildFromNotes(nil, nil),
		schema.NavigationRoles{},
		schema.KnowledgeScope{},
		schema.ArtifactPolicy{},
		schema.JournalDir{},
		schema.ArticleLanguage{},
		schema.AuthoredDate{},
	)
	var buf bytes.Buffer
	if err := sidebar(NewLibrarySidebar(nav.Shell{Nav: model}, LibraryNone, ""), layouts.Chrome{Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, unwanted := range []string{
		`data-sidebar-group="navigation-diagnostics"`,
		`data-sidebar-group="journal-diagnostics"`,
		"課程與索引目前無法使用",
		"筆記清單目前無法使用",
		"日記目前無法使用",
	} {
		if strings.Contains(html, unwanted) {
			t.Errorf("ungoverned sidebar renders %q; nothing ever claimed governance here", unwanted)
		}
	}
}

// htmlEscape mirrors the text escaping the renderer applies, so a test can look
// for the exact sentence the page wrote, quotes and all. It is spelled out
// rather than imported so the package name does not collide with the "html"
// variable every render assertion in this file already uses.
func htmlEscape(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(s)
}

// rejectedCapabilityContract loads a contract that claims governance and then
// gives a type a role the contract does not know, and a template directory that
// escapes the vault. Both declarations are made and rejected.
func rejectedCapabilityContract(t *testing.T) *schema.Contract {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := `[folders]
checked = ["Concepts"]
templates = ["../escape"]

[fields]
common = ["type", "based_on"]
required = ["type"]

[types.concept]

[types.moc]
role = "map"

[types.broken]
role = "cours"

[rules]
concept_requires_provenance = ["based_on"]
`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	return contract
}

// rejectedJournalDirContract writes a journal directory the capability refuses, so
// the journal projection closes while paths and maps stay open.
func rejectedJournalDirContract(t *testing.T) *schema.Contract {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := `[folders]
checked = ["Concepts"]
templates = ["System/templates"]
journal = "../escape"

[fields]
common = ["type", "based_on"]
required = ["type"]

[types.concept]

[types.moc]
role = "map"

[types.study-path]
role = "course"

[rules]
concept_requires_provenance = ["based_on"]
`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	return contract
}

// TestSidebarLeavesTheCurrentNotesDraftMarkToThePage holds which copy of a
// draft's mark speaks. A reader standing on a note is already told it is a
// draft by the note's own head, so the rail's badge on that row adds no fact.
// Every other row keeps its badge.
//
// The current row's badge is drawn but reserved, which the stylesheet hides
// while keeping its room. Leaving it out instead made the row's name wrap in a
// different place on the note being read than on the lessons beside it.
func TestSidebarLeavesTheCurrentNotesDraftMarkToThePage(t *testing.T) {
	t.Parallel()
	const current = "Writing/lessons/japanese/L01.md"
	const other = "Writing/lessons/japanese/L02.md"

	rows := map[string]func(string) string{
		"book row": func(rel string) string {
			t.Helper()
			var buf bytes.Buffer
			entry := PathEntryView{Kind: nav.EntryResolved, RelPath: rel, Href: "/notes/" + rel, Name: "L", Draft: true, Here: rel == current}
			if err := bookRailEntry(layouts.Chrome{}, entry, nil).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s: %v", rel, err)
			}
			return buf.String()
		},
	}

	badge := `<span class="ui-navitem__count">` + wording.Draft.In(wording.ZhHant) + `</span>`
	reserved := `<span class="ui-navitem__count ui-navitem__count--reserved">` + wording.Draft.In(wording.ZhHant) + `</span>`
	for name, render := range rows {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			currentRow := render(current)
			if !strings.Contains(currentRow, reserved) {
				t.Errorf("the current note's row does not carry its badge as the reserved, hidden one, so the rail either repeats the mark beside the note's own head or lets the name wrap differently here:\n%s", currentRow)
			}
			if strings.Contains(currentRow, badge) {
				t.Errorf("the current note's row shows a badge the stylesheet does not hide:\n%s", currentRow)
			}
			otherRow := render(other)
			if !strings.Contains(otherRow, badge) {
				t.Errorf("a row that is not the current note lost its draft mark:\n%s", otherRow)
			}
			if strings.Contains(otherRow, "--reserved") {
				t.Errorf("a row that is not the current note has its badge hidden:\n%s", otherRow)
			}
			if !strings.Contains(currentRow, "ui-navitem") {
				t.Error("the current note's row stopped rendering entirely")
			}
		})
	}
}

// TestTheSidebarListsAReportByWhatItCallsItself pins the reports beside a
// report. A report is listed by its heading, never by file name, and a title two
// reports share arrives with the day that tells them apart.
func TestTheSidebarListsAReportByWhatItCallsItself(t *testing.T) {
	t.Parallel()

	const (
		newer   = "System/reports/2026-09-22 review.md"
		older   = "System/reports/2026-09-21 review.md"
		written = "System/reports/audit-run.md"
	)
	root := t.TempDir()
	for rel, content := range map[string]string{
		newer:   "# Weekly review\n\nbody\n",
		older:   "# Weekly review\n\nbody\n",
		written: "# What the audit found\n\nbody\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	shell := nav.Shell{Nav: modelOf(t, root)}

	var reading bytes.Buffer
	if err := sidebar(NewNoteSidebar(shell, written, ""), layouts.Chrome{Nonce: "response-nonce"}).Render(t.Context(), &reading); err != nil {
		t.Fatalf("render sidebar: %v", err)
	}

	// The reports are read off the links themselves, as text, in the order the
	// shelf puts them: the file names are in the addresses, so only what a
	// reader sees is asked.
	want := []string{"Weekly review · 2026-09-22", "Weekly review · 2026-09-21", "What the audit found"}
	got := railLinkTexts(t, reading.String(), `class="y-where__path"`, "</nav>")
	if len(got) == 0 {
		t.Fatalf("the sidebar lists no links beside the report:\n%s", reading.String())
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("reports (-want +got):\n%s", diff)
	}
}

// railLinkTexts is the words of each link between from and to in a rendered
// page, tags dropped and whitespace collapsed.
func railLinkTexts(t *testing.T, html, from, to string) []string {
	t.Helper()
	_, after, found := strings.Cut(html, from)
	if !found {
		t.Fatalf("the page has no %q", from)
	}
	section, _, found := strings.Cut(after, to)
	if !found {
		t.Fatalf("the page has no %q after %q", to, from)
	}
	var texts []string
	for _, link := range regexp.MustCompile(`(?s)<a [^>]*>(.*?)</a>`).FindAllStringSubmatch(section, -1) {
		texts = append(texts, strings.Join(strings.Fields(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(link[1], " ")), " "))
	}
	return texts
}
