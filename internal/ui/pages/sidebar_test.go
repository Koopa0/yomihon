package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestAncestorDirs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		relPath string
		want    []string
	}{
		{"Concepts/golang/Foo.md", []string{"Concepts", "Concepts/golang"}},
		{"Inbox/note.md", []string{"Inbox"}},
		{"a/b/c/d.md", []string{"a", "a/b", "a/b/c"}},
		{"README.md", nil},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, ancestorDirs(tt.relPath)); diff != "" {
				t.Errorf("ancestorDirs(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

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
	root := t.TempDir()
	files := map[string]string{
		// A study-path that lists L01 twice — once deep under Decode > Bytes, once
		// directly under Review — so the reverse index yields two placements.
		"Maps/Go path.md": "---\ntype: study-path\n---\n" +
			"## decode | Decode | 解碼\n\n" +
			"### bytes | Bytes | 位元 {sequence=primary}\n\n" +
			"- [[L01]]\n- [[L02]]\n- [[Template target]]\n- [[Unwritten Lesson]]\n- [[Repeat|Ambiguous Lesson]]\n\n" +
			"## review | Review | 複習 {sequence=primary}\n\n" +
			"- [[L01]]\n",
		// A newly added topic map lists a concept. No application registration
		// names this file; its type and content are enough to grow the sidebar.
		"Maps/Reading map.md": "---\ntitle: Reading map\ntype: topic-map\ndomain: humanities\n---\n" +
			"## Themes\n\n- [[C01]]\n- [[Ghost]]\n",
		"Writing/lessons/go/L01.md": "---\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/lessons/go/L02.md": "---\ntype: lesson\nstatus: draft\n---\nbody\n",
		// A concept note not listed by any study-path.
		"Concepts/go/C01.md": "---\ntype: concept\n---\nbody\n",
		"Concepts/go/C02.md": "---\ntype: concept\n---\nbody\n",
		"A/Repeat.md":        "body\n",
		"B/Repeat.md":        "body\n",
		"System/templates/Template map.md": "---\ntitle: TEMPLATE MAP SENTINEL\ntype: topic-map\n---\n" +
			"## Shelf\n- [[L01]]\n",
		"System/templates/Template target.md": "---\ntitle: TEMPLATE TARGET SENTINEL\ntype: concept\nstatus: ready\n---\nbody\n",
		// A Sources note with no frontmatter at all (a legal shape).
		"Sources/articles/Other.md": "just prose, no frontmatter\n",
		"Sources/articles/Raw.md":   "raw clipping, no frontmatter\n",
		// Journal entries deliberately have no frontmatter.
		"Diary/2026-07-09.md": "# Earlier\n",
		"Diary/2026-07-10.md": "# Latest\n",
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
	mtimes := map[string]time.Time{
		"Diary/2026-07-09.md": time.Date(2026, time.July, 9, 8, 0, 0, 0, time.UTC),
		"Diary/2026-07-10.md": time.Date(2026, time.July, 10, 8, 0, 0, 0, time.UTC),
	}
	for rel, modified := range mtimes {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.Chtimes(full, modified, modified); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", rel, err)
		}
	}
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
	model := nav.New(
		scan.Files(),
		notes,
		graph.New(noteList, resources),
		contract.NavigationRoles(),
		contract.KnowledgeScope(),
		contract.ArtifactPolicy(),
	)
	return model
}

// TestNewSidebarWayfinding checks the resolved navigation for a note that lives
// in a folder and is listed by a study-path: its siblings surface, the study-path
// and every section on the path to it open, its folder ancestors expand, and it
// is the one entry marked current. A lesson listed by two sections opens both.
func TestNewSidebarWayfinding(t *testing.T) {
	t.Parallel()
	model := buildModel(t)

	const (
		goPath  = "Maps/Go path.md"
		current = "Writing/lessons/go/L01.md"
		sibling = "Writing/lessons/go/L02.md"
	)
	sb := NewSidebar(model, current, wording.ZhHant)

	if !sb.mapOpen(goPath) {
		t.Errorf("mapOpen(%q) = false, want true", goPath)
	}
	for _, chain := range [][]string{{"Decode"}, {"Decode", "Bytes"}, {"Review"}} {
		if !sb.branchOpen(goPath, chain) {
			t.Errorf("branchOpen(%q, %v) = false, want true", goPath, chain)
		}
	}
	for _, dir := range []string{"Writing", "Writing/lessons", "Writing/lessons/go"} {
		if !sb.folderOpen(dir) {
			t.Errorf("folderOpen(%q) = false, want true", dir)
		}
	}
	if sb.folderOpen("Concepts") {
		t.Error("folderOpen(\"Concepts\") = true, want false (not an ancestor of the current note)")
	}
	if !sb.folderTreeOpen() {
		t.Error("folderTreeOpen() = false, want true (the current note lives in a folder)")
	}
	if !sb.current(current) {
		t.Errorf("current(%q) = false, want true", current)
	}
	if sb.current(sibling) {
		t.Errorf("current(%q) = true, want false", sibling)
	}
	if sb.HereDir != "Writing/lessons/go" {
		t.Errorf("HereDir = %q, want %q", sb.HereDir, "Writing/lessons/go")
	}
	wantHere := []nav.NoteRef{
		{Name: "L01", RelPath: current},
		{Name: "L02", RelPath: sibling},
	}
	if diff := cmp.Diff(wantHere, sb.Here); diff != "" {
		t.Errorf("Here mismatch (-want +got):\n%s", diff)
	}

	// A note listed only under Decode > Bytes must not open the Review section:
	// the open set is per note, not per study-path.
	sb2 := NewSidebar(model, sibling, wording.ZhHant)
	if sb2.branchOpen(goPath, []string{"Review"}) {
		t.Errorf("branchOpen(%q, [Review]) = true for a note absent from Review, want false", goPath)
	}
	if !sb2.branchOpen(goPath, []string{"Decode", "Bytes"}) {
		t.Errorf("branchOpen(%q, [Decode Bytes]) = false, want true", goPath)
	}
}

// TestNewSidebarNonLessonFixtures checks two other representative shapes: a
// concept listed by a topic map and a frontmatter-less Sources note listed by
// no map. Siblings and folder ancestors surface for both; the topic map opens
// only for the concept it contains.
func TestNewSidebarNonLessonFixtures(t *testing.T) {
	t.Parallel()
	model := buildModel(t)

	tests := []struct {
		name     string
		current  string
		wantDir  string
		wantHere []nav.NoteRef
		wantDirs []string
		wantMap  bool
	}{
		{
			name:    "concept note",
			current: "Concepts/go/C01.md",
			wantDir: "Concepts/go",
			wantHere: []nav.NoteRef{
				{Name: "C01", RelPath: "Concepts/go/C01.md"},
				{Name: "C02", RelPath: "Concepts/go/C02.md"},
			},
			wantDirs: []string{"Concepts", "Concepts/go"},
			wantMap:  true,
		},
		{
			name:    "no-frontmatter Sources note",
			current: "Sources/articles/Raw.md",
			wantDir: "Sources/articles",
			wantHere: []nav.NoteRef{
				{Name: "Other", RelPath: "Sources/articles/Other.md"},
				{Name: "Raw", RelPath: "Sources/articles/Raw.md"},
			},
			wantDirs: []string{"Sources", "Sources/articles"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sb := NewSidebar(model, tt.current, wording.ZhHant)
			if sb.mapOpen("Maps/Go path.md") {
				t.Error("mapOpen = true for a note no study-path lists, want false")
			}
			if got := sb.mapOpen("Maps/Reading map.md"); got != tt.wantMap {
				t.Errorf("mapOpen(Reading map) = %t, want %t", got, tt.wantMap)
			}
			if got := sb.branchOpen("Maps/Reading map.md", []string{"Themes"}); got != tt.wantMap {
				t.Errorf("branchOpen(Reading map, Themes) = %t, want %t", got, tt.wantMap)
			}
			if sb.HereDir != tt.wantDir {
				t.Errorf("HereDir = %q, want %q", sb.HereDir, tt.wantDir)
			}
			if diff := cmp.Diff(tt.wantHere, sb.Here); diff != "" {
				t.Errorf("Here mismatch (-want +got):\n%s", diff)
			}
			for _, dir := range tt.wantDirs {
				if !sb.folderOpen(dir) {
					t.Errorf("folderOpen(%q) = false, want true", dir)
				}
			}
			if !sb.current(tt.current) {
				t.Errorf("current(%q) = false, want true", tt.current)
			}
		})
	}
}

// TestSidebarMarksDisclosureStateForTheScript locks the HTML contract the
// disclosure state owner reads: every sidebar disclosure carries a stable
// data-key, the current note's wayfinding chain also carries data-chain
// (forced open, never persisted), discretionary branches carry no chain
// marker, and the pre-paint restore script and the filter's no-match notice
// ride with the sidebar. Rendered attributes come from a map, so the marker
// pair appears in alphabetical order (data-chain before data-key).
func TestSidebarMarksDisclosureStateForTheScript(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	sb := NewSidebar(model, "Writing/lessons/go/L01.md", wording.ZhHant)

	var buf bytes.Buffer
	if err := sidebar(sb, "response-nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		`data-chain data-key="paths"`,
		`data-chain data-key="map:Maps/Go path.md"`,
		`data-chain data-key="dir:Writing/lessons/go"`,
		`data-chain data-key="folders"`,
		`data-key="dir:Concepts"`,
		`data-filter-empty`,
		`yomihon.nav`,
		`<script nonce="response-nonce">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered sidebar is missing %q", want)
		}
	}
	if strings.Contains(html, `data-chain data-key="dir:Concepts"`) {
		t.Error("dir:Concepts carries data-chain, but it is not an ancestor of the current note")
	}
}

func TestSidebarContentGrouping(t *testing.T) {
	t.Parallel()
	model := buildModel(t)

	var buf bytes.Buffer
	if err := sidebar(NewSidebar(model, "", wording.ZhHant), "response-nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	pathsAt := strings.Index(html, `data-sidebar-group="paths"`)
	mapsAt := strings.Index(html, `data-sidebar-group="maps"`)
	journalAt := strings.Index(html, `data-sidebar-group="journal"`)
	if pathsAt < 0 || mapsAt < 0 || journalAt < 0 || pathsAt >= mapsAt || mapsAt >= journalAt {
		t.Errorf("sidebar group order = paths:%d maps:%d journal:%d, want Paths then Maps then Journal", pathsAt, mapsAt, journalAt)
	}
	for _, want := range []string{
		`data-map-tree="Maps/Reading map.md"`,
		`href="/notes/Maps/Reading%20map.md"`,
		`>開啟地圖</a>`,
		`>C01</a>`,
		`data-key="journal"`,
		`data-sidebar-journal-entry>2026-07-10</a>`,
		`data-sidebar-journal-entry>2026-07-09</a>`,
		`data-resolution="unresolved"`,
		`data-resolution="ambiguous"`,
		"Unwritten Lesson",
		"Ambiguous Lesson",
		"y-navmark--warn",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered sidebar is missing %q", want)
		}
	}
	if strings.Contains(html, "Ghost") {
		t.Error("rendered sidebar contains unresolved map row Ghost")
	}
	if strings.Contains(html, `href="/notes/"`) {
		t.Error("rendered sidebar fabricates an empty note href for a broken study-path row")
	}
	unwrittenAt := strings.Index(html, "Unwritten Lesson")
	ambiguousAt := strings.Index(html, "Ambiguous Lesson")
	if unwrittenAt < 0 || ambiguousAt < 0 || unwrittenAt >= ambiguousAt {
		t.Errorf("broken study-path row order = unresolved:%d ambiguous:%d, want document order", unwrittenAt, ambiguousAt)
	}
	if strings.Contains(html, ">Lifecycle<") {
		t.Error("rendered sidebar still contains the Lifecycle section")
	}
	if got := strings.Count(html, `data-map-tree=`); got != 2 {
		t.Errorf("rendered sidebar map disclosures = %d, want 2 (one Path and one Map)", got)
	}
	for _, tt := range []struct {
		name string
		key  string
	}{
		{name: "map", key: "map:Maps/Reading map.md"},
		{name: "journal", key: "journal"},
	} {
		t.Run(tt.name+" starts closed", func(t *testing.T) {
			t.Parallel()
			tag := detailsTagByKey(t, html, tt.key)
			if strings.Contains(tag, " open") {
				t.Errorf("detailsTagByKey(%q) = %q, want no open attribute", tt.key, tag)
			}
		})
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
	)
	if model.NavigationDiagnostic() == "" || model.ArtifactDiagnostic() == "" {
		t.Fatalf("fixture produced no capability fault: navigation %q artifact %q",
			model.NavigationDiagnostic(), model.ArtifactDiagnostic())
	}
	var buf bytes.Buffer
	if err := sidebar(NewSidebar(model, "", wording.ZhHant), "response-nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-sidebar-group="navigation-diagnostics"`,
		"路徑與地圖",
		"治理項目投影目前無法使用",
		// The sentences themselves, HTML-escaped exactly as the page writes them.
		htmlEscape(model.NavigationDiagnostic()),
		htmlEscape(model.ArtifactDiagnostic()),
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered degraded sidebar is missing %q", want)
		}
	}
	if strings.Contains(html, `data-map-tree=`) {
		t.Error("rendered degraded sidebar contains a map disclosure")
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
	)
	var buf bytes.Buffer
	if err := sidebar(NewSidebar(model, "", wording.ZhHant), "response-nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, unwanted := range []string{
		`data-sidebar-group="navigation-diagnostics"`,
		"路徑與地圖目前無法使用",
		"治理項目投影目前無法使用",
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
// declares a path type its own enums.type does not contain, and an artifact
// directory that escapes the vault. Both declarations are made and rejected.
func rejectedCapabilityContract(t *testing.T) *schema.Contract {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := `schema_version = "1"

[enums]
type = ["concept", "moc"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[scan]
knowledge_dirs = ["Concepts"]

[navigation]
path_types = ["undeclared-type"]
map_types = ["moc"]

[artifacts]
non_instance_dirs = ["../escape"]

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
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

func TestSidebarKeepsNonInstanceStudyPathWarningsOutOfNavigationLinks(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	var buf bytes.Buffer
	if err := sidebar(NewSidebar(model, "", wording.ZhHant), "response-nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, `data-map-tree="System/templates/Template map.md"`) {
		t.Error("sidebar Maps contains the non-instance template map document")
	}
	pathsAt := strings.Index(html, `data-sidebar-group="paths"`)
	mapsAt := strings.Index(html, `data-sidebar-group="maps"`)
	if pathsAt < 0 || mapsAt < 0 || pathsAt >= mapsAt {
		t.Fatalf("sidebar path/map markers = %d/%d, want ordered groups", pathsAt, mapsAt)
	}
	paths := html[pathsAt:mapsAt]
	for _, want := range []string{`data-resolution="non-instance"`, "Template target", ">非治理項目</span>"} {
		if !strings.Contains(paths, want) {
			t.Errorf("sidebar Paths is missing non-instance warning output %q", want)
		}
	}
	if strings.Contains(paths, `href="/notes/System/templates/Template%20target.md"`) || strings.Contains(paths, "ui-status--ready") {
		t.Error("sidebar Paths turns a non-instance warning into a linked or status-bearing entry")
	}
	for _, want := range []string{
		`href="/notes/System/templates/Template%20map.md">Template map</a>`,
		`href="/notes/System/templates/Template%20target.md">Template target</a>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("sidebar Folders is missing readable non-instance artifact %q", want)
		}
	}
}

func TestSidebarZeroEntryMapKeepsDisclosureAndOpenLink(t *testing.T) {
	t.Parallel()

	model := nav.Map{Title: "Empty map", RelPath: "Maps/Empty.md"}
	var buf bytes.Buffer
	if err := mapTree(NewSidebar(nil, "", wording.ZhHant), model, notesHref(model.RelPath)).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	tag := detailsTagByKey(t, html, "map:Maps/Empty.md")
	if !strings.Contains(tag, `data-map-tree="Maps/Empty.md"`) {
		t.Errorf("zero-entry map details tag = %q, want selected map marker", tag)
	}
	for _, want := range []string{
		"Empty map",
		`href="/notes/Maps/Empty.md">開啟地圖</a>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered zero-entry map is missing %q", want)
		}
	}
}

func detailsTagByKey(t *testing.T, html, key string) string {
	t.Helper()
	marker := `data-key="` + key + `"`
	markerAt := strings.Index(html, marker)
	if markerAt < 0 {
		t.Fatalf("rendered sidebar has no details tag with %s", marker)
	}
	start := strings.LastIndex(html[:markerAt], "<details")
	if start < 0 {
		t.Fatalf("rendered sidebar has %s outside a details tag", marker)
	}
	endFromMarker := strings.IndexByte(html[markerAt:], '>')
	if endFromMarker < 0 {
		t.Fatalf("rendered sidebar details tag with %s has no closing bracket", marker)
	}
	return html[start : markerAt+endFromMarker+1]
}

// TestNewSidebarNoCurrentNote is the report-page shape: with no current note,
// every branch stays closed and unmarked, and the "here" block is empty.
func TestNewSidebarNoCurrentNote(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	sb := NewSidebar(model, "", wording.ZhHant)

	if sb.mapOpen("Maps/Go path.md") {
		t.Error("mapOpen = true with no current note, want false")
	}
	if sb.folderTreeOpen() {
		t.Error("folderTreeOpen = true with no current note, want false")
	}
	if len(sb.Here) != 0 {
		t.Errorf("Here = %v with no current note, want empty", sb.Here)
	}
}

// TestSidebarLeavesTheCurrentNotesStatusToThePage holds which of a page's
// status faces is allowed to speak. The panel and the bottom bar read the
// note's status line from disk when the page is built; the rail's per-lesson
// badge comes from the captured generation, which the scanner republishes on
// its own cadence. Right after a transition the redirect is served inside that
// window, so the same response printed the new status twice and the old one
// once, for one note, with nothing to say which was current.
//
// The repair is to drop the third copy rather than to synchronise it. A reader
// standing on a note is already told its status by the faces that read it
// live; a rail badge repeating an older answer beside them adds no fact and
// can only disagree. Every other row keeps its badge — there the generation's
// answer is the only one on offer and contradicts nothing.
func TestSidebarLeavesTheCurrentNotesStatusToThePage(t *testing.T) {
	t.Parallel()
	const current = "Writing/lessons/japanese/L01.md"
	const other = "Writing/lessons/japanese/L02.md"
	sb := Sidebar{CurrentPath: current}

	// Both row renderers, because the rail draws the same badge twice: study
	// path rows and map branch rows are separate templates over separate
	// types, and fixing one leaves the other saying the thing this test forbids.
	rows := map[string]func(string) string{
		"study path row": func(rel string) string {
			t.Helper()
			var buf bytes.Buffer
			entry := nav.PathEntry{Kind: nav.EntryResolved, RelPath: rel, Text: "L", Status: "draft"}
			if err := pathEntryLink(sb, &entry).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s: %v", rel, err)
			}
			return buf.String()
		},
		"map branch row": func(rel string) string {
			t.Helper()
			var buf bytes.Buffer
			entry := nav.Entry{Kind: nav.EntryResolved, RelPath: rel, Text: "L", Status: "draft"}
			if err := entryLink(sb, entry).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s: %v", rel, err)
			}
			return buf.String()
		},
	}

	for name, render := range rows {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := render(current); strings.Contains(got, "draft") {
				t.Errorf("the rail repeats the current note's status beside the faces that read it live:\n%s", got)
			}
			otherRow := render(other)
			if !strings.Contains(otherRow, "draft") {
				t.Errorf("a row that is not the current note lost the only status answer it had:\n%s", otherRow)
			}
			if !strings.Contains(render(current), "ui-navitem") {
				t.Error("the current note's row stopped rendering entirely")
			}
		})
	}
}
