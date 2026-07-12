package note_test

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
)

// newServer wires the reading page against a real (not faked)
// status.Service, with a nil contract (fail-closed). Good
// enough for tests whose point is that the page renders regardless of
// whether the write face is available (reading stays fail-open even when
// the write face is fail-closed) — NOT for exercising
// handler.go's NoFrontmatter/Transitions branch selection, since a
// fail-closed Service supplies a write diagnostic and note.templ's statusPanel
// switches on it first, before either of those ever matters. Use
// newServerWithContract for anything that needs to distinguish them.
func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	return newServerWithContract(t, root, nil)
}

// newServerWithContract is newServer with an explicit contract, so tests
// can put the write face in its non-fail-closed state (no write diagnostic)
// and actually observe which of NoFrontmatter / Transitions /
// "no legal transitions" handler.go's show() selected.
func newServerWithContract(t *testing.T, root string, contract *schema.Schema) *httptest.Server {
	t.Helper()
	return newServerWithProvenance(t, root, contract, func(context.Context, string) (string, error) { return "", nil })
}

func newServerWithProvenance(
	t *testing.T,
	root string,
	contract *schema.Schema,
	provenance func(context.Context, string) (string, error),
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	log := slog.New(slog.DiscardHandler)
	var roles schema.NavigationRoles
	var policy schema.ArtifactPolicy
	if contract != nil {
		roles = contract.NavigationRoles()
		policy = contract.ArtifactPolicy()
	}
	svc := status.NewService(root, contract, policy)
	store := snapshot.New(root, log, roles, policy)
	// Build the slot index when the temp vault has a System/slots dir; a test
	// without one leaves Slots nil (the legal "no slot machines" state).
	var slots lesson.SlotIndex
	var err error
	if slotsDir := filepath.Join(root, "System", "slots"); dirExists(slotsDir) {
		slots, err = lesson.BuildSlotIndex(slotsDir)
		if err != nil {
			t.Fatalf("lesson.BuildSlotIndex(%q) = %v", slotsDir, err)
		}
	}
	concepts, err := lesson.BuildConceptIndex(root)
	if err != nil {
		t.Fatalf("lesson.BuildConceptIndex(%q) = %v", root, err)
	}
	h := note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, store.Resolver()),
		Status:     svc,
		Snapshot:   store.Current,
		Provenance: provenance,
		Log:        log,
		Slots:      slots,
		Concepts:   concepts,
	})
	h.Register(mux)
	status.NewHandler(svc, log).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

const (
	loudLessonSlug     = "jp-template-loud"
	loudLessonSentinel = "LOUD LESSON RAW SENTINEL"
)

func writeLoudLessonFixture(t *testing.T, root, rel string) {
	t.Helper()
	conceptDir := filepath.Join(root, "Concepts", "japanese")
	if err := os.MkdirAll(conceptDir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"), []byte("---\ntitle: は\ntype: writing\n---\n\nConcept body.\n"), 0o644); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir lesson directory: %v", err)
	}
	body := "---\ntitle: Loud lesson template\ntype: lesson\nstatus: ready\nslug: " + loudLessonSlug + "\n---\n\n" +
		"| A | B |\n|---|---|\n| x | y |\n\n" +
		"<ruby>今日<rt>きょう</rt></ruby>は晴れ。 [[は]] " + loudLessonSentinel + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write loud lesson: %v", err)
	}

	slotsDir := filepath.Join(root, "System", "slots")
	if err := os.MkdirAll(slotsDir, 0o750); err != nil {
		t.Fatalf("mkdir slots: %v", err)
	}
	sidecar := "lesson: Template\nslug: " + loudLessonSlug + "\ntitle: loud\npatterns:\n" +
		"  - id: p1\n    template: \"{A}は {B}です\"\n    gloss_zh: \"A is B\"\n    slots:\n" +
		"      A: {label_zh: \"主題\", color: topic, fills: [{jp: わたし, reading: わたし, zh: 我}]}\n" +
		"      B: {label_zh: \"述語\", color: pred, fills: [{jp: 学生, reading: がくせい, zh: 學生}]}\n"
	if err := os.WriteFile(filepath.Join(slotsDir, "template.yaml"), []byte(sidecar), 0o644); err != nil {
		t.Fatalf("write slot sidecar: %v", err)
	}
}

func TestShowNonInstanceLessonHasNoGovernanceOrLessonEnhancements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const templateRel = "System/templates/Loud lesson.md"
	writeLoudLessonFixture(t, root, templateRel)

	provenanceCalls := 0
	srv := newServerWithProvenance(t, root, loadContract(t), func(context.Context, string) (string, error) {
		provenanceCalls++
		return "should-not-render", nil
	})
	code, page := get(t, srv.URL+"/notes/System/templates/Loud%20lesson.md")
	if code != http.StatusOK {
		t.Fatalf("GET template lesson status = %d, want 200", code)
	}
	main := noteMain(t, page)
	for _, want := range []string{
		loudLessonSentinel,
		`href="/notes/Concepts/japanese/%E3%81%AF.md" class="wikilink"`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("non-instance lesson main is missing %q", want)
		}
	}
	for _, want := range []string{
		`data-status-state="non-instance"`,
		"not a governable artifact",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("non-instance lesson page is missing %q", want)
		}
	}
	for _, absent := range []string{
		`action="/status"`,
		`data-tts=`,
		"y-slotmachine",
		`data-concept=`,
		`data-concept-sheet`,
		"actor · koopa",
		"sealed by koopa",
		"git · commit",
		"ui-status--ready",
	} {
		if strings.Contains(page, absent) {
			t.Errorf("non-instance lesson page unexpectedly contains %q", absent)
		}
	}
	if provenanceCalls != 0 {
		t.Errorf("non-instance lesson provenance reads = %d, want 0", provenanceCalls)
	}
}

func TestShowUnavailableArtifactPolicyDoesNotAssumeLessonInstance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		section string
		want    string
	}{
		{name: "missing", section: "", want: "contract declares no artifact policy; instance projections disabled until it does"},
		{name: "invalid", section: "[artifacts]\nnon_instance_dirs = [\".\"]\n", want: `invalid artifact policy: non_instance_dirs contains "."`},
		{name: "incomplete", section: "[artifacts]\n", want: `invalid artifact policy: missing required key "non_instance_dirs"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const rel = "Writing/lessons/japanese/Loud lesson.md"
			writeLoudLessonFixture(t, root, rel)

			provenanceCalls := 0
			contract := loadHomeContractWithArtifactSection(t, tt.section)
			srv := newServerWithProvenance(t, root, contract, func(context.Context, string) (string, error) {
				provenanceCalls++
				return "should-not-render", nil
			})
			code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md?sealed=1")
			if code != http.StatusOK {
				t.Fatalf("GET lesson with unavailable artifact policy status = %d, want 200", code)
			}
			main := noteMain(t, page)
			for _, want := range []string{
				loudLessonSentinel,
				`href="/notes/Concepts/japanese/%E3%81%AF.md" class="wikilink"`,
			} {
				if !strings.Contains(main, want) {
					t.Errorf("lesson main is missing readable content %q", want)
				}
			}
			for _, surface := range statusSurfaces(t, html.UnescapeString(page)) {
				for _, want := range []string{`data-status-state="unavailable"`, tt.want} {
					if !strings.Contains(surface.body, want) {
						t.Errorf("%s is missing %q", surface.name, want)
					}
				}
			}
			for _, absent := range []string{
				`action="/status"`,
				"actor · koopa",
				`data-tts=`,
				"y-slotmachine",
				`data-concept=`,
				`data-concept-sheet`,
				"sealed by koopa",
				"git · commit",
				"y-toast",
			} {
				if strings.Contains(page, absent) {
					t.Errorf("lesson with unavailable artifact policy unexpectedly contains %q", absent)
				}
			}
			if provenanceCalls != 0 {
				t.Errorf("lesson provenance reads = %d, want 0 without artifact policy", provenanceCalls)
			}
		})
	}
}

func TestShowWriteClosureDiagnosticsRemainDistinct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		contract   func(*testing.T) *schema.Schema
		want       string
		wantAbsent string
	}{
		{
			name:       "core contract unavailable",
			contract:   func(*testing.T) *schema.Schema { return nil },
			want:       "Contract unavailable — the write face is closed (fail-closed).",
			wantAbsent: "contract declares no artifact policy; instance projections disabled until it does",
		},
		{
			name: "artifact policy missing",
			contract: func(t *testing.T) *schema.Schema {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "")
			},
			want:       "contract declares no artifact policy; instance projections disabled until it does",
			wantAbsent: "Contract unavailable — the write face is closed (fail-closed).",
		},
		{
			name: "artifact policy invalid",
			contract: func(t *testing.T) *schema.Schema {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\".\"]\n")
			},
			want:       `invalid artifact policy: non_instance_dirs contains "."`,
			wantAbsent: "Contract unavailable — the write face is closed (fail-closed).",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "Note.md"), []byte("---\ntitle: Note\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract(t))
			code, page := get(t, srv.URL+"/notes/Note.md")
			if code != http.StatusOK {
				t.Fatalf("GET note status = %d, want 200", code)
			}
			page = html.UnescapeString(page)
			for _, surface := range statusSurfaces(t, page) {
				for _, want := range []string{`data-status-state="unavailable"`, tt.want} {
					if !strings.Contains(surface.body, want) {
						t.Errorf("%s is missing %q", surface.name, want)
					}
				}
				for _, absent := range []string{tt.wantAbsent, `action="/status"`} {
					if strings.Contains(surface.body, absent) {
						t.Errorf("%s contains forbidden closure output %q", surface.name, absent)
					}
				}
			}
		})
	}
}

func noteMain(t *testing.T, page string) string {
	t.Helper()
	openAt := strings.Index(page, `<main class="y-main">`)
	if openAt < 0 {
		t.Fatal("note page has no main reading surface")
	}
	closeAt := strings.Index(page[openAt:], "</main>")
	if closeAt < 0 {
		t.Fatal("note page main has no closing tag")
	}
	return page[openAt : openAt+closeAt+len("</main>")]
}

type renderedStatusSurface struct {
	name string
	body string
}

func statusSurfaces(t *testing.T, page string) []renderedStatusSurface {
	t.Helper()
	definitions := []struct {
		name, open, close string
	}{
		{name: "status panel", open: `<section class="y-statuspanel`, close: "</section>"},
		{name: "seal bar", open: `<div class="y-sealbar`, close: "</div>"},
	}
	surfaces := make([]renderedStatusSurface, 0, len(definitions))
	for _, definition := range definitions {
		openAt := strings.Index(page, definition.open)
		if openAt < 0 {
			t.Fatalf("note page has no %s", definition.name)
		}
		closeAt := strings.Index(page[openAt:], definition.close)
		if closeAt < 0 {
			t.Fatalf("note page %s has no closing tag", definition.name)
		}
		surfaces = append(surfaces, renderedStatusSurface{
			name: definition.name,
			body: page[openAt : openAt+closeAt+len(definition.close)],
		})
	}
	return surfaces
}

// loadContract is a loader fixture, not a second schema: it reuses
// schema.LoadFile as-is against a lesson-only slice of the real contract
// shape (testdata/contract.toml), mirroring internal/status/status_test.go.
func loadContract(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

// loadHomeContract reuses the complete schema loader fixture because Home's
// lifecycle strip needs the default note-status group, not the lesson-only
// group exercised by the reading-page tests above.
func loadHomeContract(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(schema test contract) = %v", err)
	}
	return s
}

func loadHomeContractWithArtifactSection(t *testing.T, artifactSection string) *schema.Schema {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	const validSection = "[artifacts]\nnon_instance_dirs = [\"System/templates\"]\n"
	contractText := strings.Replace(string(base), validSection, artifactSection, 1)
	if contractText == string(base) {
		t.Fatal("schema test contract artifact section was not replaced")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	err = os.WriteFile(path, []byte(contractText), 0o600) // #nosec G703 -- path is a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}

func loadHomeContractWithSections(t *testing.T, navigationSection, artifactSection string) *schema.Schema {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	const validNavigation = "[navigation]\npath_types = [\"study-path\"]\nmap_types = [\"moc\", \"source-map\", \"topic-map\"]\n"
	const validArtifact = "[artifacts]\nnon_instance_dirs = [\"System/templates\"]\n"
	contractText := strings.Replace(string(base), validNavigation, navigationSection, 1)
	contractText = strings.Replace(contractText, validArtifact, artifactSection, 1)
	if contractText == string(base) {
		t.Fatal("contract section replacements did not apply")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	err = os.WriteFile(path, []byte(contractText), 0o600) // #nosec G703 -- path is a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(t.Context(), "git", cmdArgs...) // #nosec G204 -- test-controlled arguments are passed directly, never through a shell
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func hiddenValue(t *testing.T, form, name string) string {
	t.Helper()
	marker := `name="` + name + `" value="`
	start := strings.Index(form, marker)
	if start < 0 {
		t.Fatalf("form has no hidden %q value: %q", name, form)
	}
	start += len(marker)
	end := strings.IndexByte(form[start:], '"')
	if end < 0 {
		t.Fatalf("form has an unterminated hidden %q value: %q", name, form)
	}
	return html.UnescapeString(form[start : start+end])
}

func get(t *testing.T, urlStr string) (code int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", urlStr, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", urlStr, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

// TestReadingPageRejectsPathTraversal fires traversal-shaped requests at the
// reading route and asserts none escapes the vault root: no request is served
// (never 200) and no byte of a file outside the vault ever reaches the body.
// The defense is layered — the mux cleans dot segments, the handler serves only
// .md, and vault.ReadNote rejects any non-local path — so this pins the
// observable contract rather than one layer; a regression in any of them that
// let an escape through would surface here as a 200 or a leaked sentinel.
func TestReadingPageRejectsPathTraversal(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	if err := os.MkdirAll(filepath.Join(root, "Notes"), 0o750); err != nil {
		t.Fatalf("mkdir vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Notes", "real.md"), []byte("a real note body\n"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	// A decoy one level above the vault root, reachable by traversal only if a
	// layer fails. Its sentinel must never appear in any response body.
	const sentinel = "yomihon-outside-vault-sentinel-never-serve-this"
	if err := os.WriteFile(filepath.Join(parent, "secret.md"), []byte(sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("write decoy: %v", err)
	}
	srv := newServer(t, root)

	// Positive control: a genuine note is served, so a blanket 404 is not what
	// makes the traversal cases pass.
	if code, body := get(t, srv.URL+"/notes/Notes/real.md"); code != http.StatusOK || !strings.Contains(body, "a real note") {
		t.Fatalf("GET a genuine note = %d, want 200 with the note body", code)
	}

	payloads := []struct {
		name string
		path string
	}{
		{name: "encoded dot-dot", path: "%2e%2e%2fsecret.md"},
		{name: "encoded dot-dot twice", path: "%2e%2e%2f%2e%2e%2fsecret.md"},
		{name: "mixed dot-dot", path: "..%2fsecret.md"},
		{name: "raw dot-dot", path: "../secret.md"},
		{name: "double-encoded dot-dot", path: "%252e%252e%2fsecret.md"},
		{name: "slash-injected absolute", path: "%2fetc%2fhostname"},
		{name: "nul byte", path: "real%00.md"},
	}
	for _, p := range payloads {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.URL+"/notes/"+p.path)
			if code == http.StatusOK {
				t.Errorf("GET /notes/%s = 200, want the request refused", p.path)
			}
			if strings.Contains(body, sentinel) {
				t.Errorf("GET /notes/%s leaked a file from outside the vault root", p.path)
			}
		})
	}
}

func TestShow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessonMD := "---\ntitle: L00 テスト課\ntype: lesson\nstatus: draft\n---\n\n<ruby>今日<rt>きょう</rt></ruby>は<ruby>晴<rt>は</rt></ruby>れ。\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L00 テスト課.md"), []byte(lessonMD), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServer(t, root)

	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L00 テスト課.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		"L00 テスト課",
		"ui-status--draft", // the note's status, rendered as its badge
		"<ruby>今日<rt>きょう</rt></ruby>",
		// The status service is wired but fail-closed (nil contract):
		// the page must still render, with the write face's own notice
		// instead of any transition form (asymmetric fault tolerance —
		// a missing contract never breaks reading).
		"fail-closed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

// TestShowTTSGatedToLessons is the landmine guard for the TTS lesson gate: an
// explicitly marked lesson paragraph gets a speak button, but a non-lesson
// note containing the same marker and ruby does not. render.HTML is generic and
// the type gate lives in the handler, so TTS must never leak into other types.
func TestShowTTSGatedToLessons(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "<!-- read-aloud: ja -->\n<ruby>今日<rt>きょう</rt></ruby>は晴れ。\n"

	lessonDir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lessonDir, "L00.md"),
		[]byte("---\ntitle: L00\ntype: lesson\nstatus: draft\n---\n\n"+body), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S00.md"),
		[]byte("---\ntitle: S00\ntype: source-note\nstatus: draft\n---\n\n"+body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))

	_, lessonBody := get(t, srv.URL+"/notes/Writing/lessons/japanese/L00.md")
	if !strings.Contains(lessonBody, `data-tts="今日は晴れ。"`) {
		t.Errorf("lesson page missing the TTS button with reading-stripped data-tts; body = %q", lessonBody)
	}

	_, sourceBody := get(t, srv.URL+"/notes/Sources/S00.md")
	if strings.Contains(sourceBody, "data-tts") || strings.Contains(sourceBody, "y-tts") {
		t.Errorf("non-lesson (source-note) page leaked a TTS button — the lesson gate failed; body = %q", sourceBody)
	}
	if !strings.Contains(sourceBody, "<ruby>今日<rt>きょう</rt></ruby>") {
		t.Errorf("non-lesson page lost its ruby (render must stay generic); body = %q", sourceBody)
	}
}

// dirExists reports whether path is an existing directory.
// TestReadingPageServesOnlyMarkdownNotes pins that /notes serves only .md notes.
// Every file the browse tree lists now opens, so a non-note resource no longer
// meets a 404 here. The guarantee that 404 was protecting is unchanged and is
// what this pins: a resource's markup never becomes live markup in this
// first-party, yomihon-origin page. A note's body passes through WithUnsafe,
// which hands raw HTML — including <script> — to the page verbatim; every other
// kind is escaped into a source view instead, and its bytes reach the browser
// only through the sandboxed raw endpoint. The .html below carries a script tag
// precisely so that its inertness can be observed.
func TestReadingPageNeverExecutesANonNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	const liveTag = `<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>`
	write("Notes/keep.md", "# kept\n\nbody\n")
	write("System/reports/daily-briefing/x.html", `<!doctype html>`+liveTag+`<body>hi</body>`)
	write("Diagrams/x.canvas", "{\"nodes\":[]}\n")
	srv := newServer(t, root)

	if code, _ := get(t, srv.URL+"/notes/Notes/keep.md"); code != http.StatusOK {
		t.Errorf("a .md note must still be served; GET keep.md = %d, want 200", code)
	}
	for _, rel := range []string{
		"System/reports/daily-briefing/x.html",
		"Diagrams/x.canvas",
	} {
		code, body := get(t, srv.URL+"/notes/"+rel)
		if code != http.StatusOK {
			t.Errorf("GET /notes/%s = %d, want 200 (every listed file opens)", rel, code)
		}
		if !strings.Contains(body, `<pre class="chroma"`) {
			t.Errorf("GET /notes/%s is not a source view", rel)
		}
		if strings.Contains(body, liveTag) {
			t.Errorf("GET /notes/%s put a live script tag into a first-party page", rel)
		}
	}

	// The script's own text is shown — escaped, as source — which is the
	// difference between reading a file and running it.
	if _, body := get(t, srv.URL+"/notes/System/reports/daily-briefing/x.html"); !strings.Contains(body, "cdn.jsdelivr") {
		t.Error("the .html source view does not show the file's own text")
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// TestShowSlotMachine is the slot-machine wiring guard: a lesson whose slug
// joins a slot sidecar gets its pattern machine spliced into the page,
// positioned right after the lesson's first table (the 文型骨架 skeleton) and
// before the body that follows it. A lesson with no matching sidecar gets no
// machine.
func TestShowSlotMachine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	lessonDir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	// A lesson with a table (the splice anchor) and a paragraph after it.
	body := "---\ntitle: L01\ntype: lesson\nstatus: draft\nslug: jp-test-l01\n---\n\n" +
		"| pattern | meaning |\n|---|---|\n| AはBです | A is B |\n\nAFTERTABLEBODY\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "L01.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	slotsDir := filepath.Join(root, "System", "slots")
	if err := os.MkdirAll(slotsDir, 0o750); err != nil {
		t.Fatalf("mkdir slots: %v", err)
	}
	// The sidecar joins by the slug INSIDE the file, not the filename:
	// filename S1.yaml, slug jp-test-l01 matching the lesson.
	sidecar := "lesson: L01\nslug: jp-test-l01\ntitle: t\npatterns:\n" +
		"  - id: p1\n    template: \"{A}は {B}です\"\n    gloss_zh: \"{A} 是 {B}\"\n    slots:\n" +
		"      A: {label_zh: \"主題\", color: topic, fills: [{jp: わたし, reading: わたし, zh: 我}]}\n" +
		"      B: {label_zh: \"述語\", color: pred, fills: [{jp: 学生, reading: がくせい, zh: 學生}]}\n"
	if err := os.WriteFile(filepath.Join(slotsDir, "S1.yaml"), []byte(sidecar), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// The machine rendered, with the first-fill sentence and a coloured slot.
	for _, want := range []string{`class="y-slotmachine"`, `class="y-slotcard"`, `y-slotdata`, `y-slot-topic`, `わたし`} {
		if !strings.Contains(page, want) {
			t.Errorf("lesson page missing slot-machine marker %q", want)
		}
	}
	// Placement: after the table, before the body that follows it.
	tbl := strings.Index(page, "</table>")
	machine := strings.Index(page, "y-slotmachine")
	after := strings.Index(page, "AFTERTABLEBODY")
	if tbl < 0 || tbl >= machine || machine >= after {
		t.Errorf("slot machine mis-positioned: </table>@%d, machine@%d, after-body@%d (want table < machine < after)", tbl, machine, after)
	}
}

// TestShowLessonWithoutSidecarHasNoMachine confirms the gate: a lesson whose
// slug matches no sidecar renders with no slot machine.
func TestShowLessonWithoutSidecarHasNoMachine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A lesson with a slug but no System/slots dir at all (Slots stays nil).
	body := "---\ntitle: L02\ntype: lesson\nstatus: draft\nslug: jp-orphan\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "L02.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))
	_, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L02.md")
	if strings.Contains(page, "y-slotmachine") {
		t.Errorf("lesson with no matching sidecar still rendered a slot machine; body = %q", page)
	}
}

// TestShowConceptSheet is the concept-sheet wiring guard: a lesson whose
// wikilink resolves to a concept note gets that link marked as a trigger, the
// concept pre-rendered into a hidden <template>, and the shared <dialog>
// emitted. The trigger stays a real navigable <a> (the no-JS fallback).
func TestShowConceptSheet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// A concept note the lesson will link to.
	conceptDir := filepath.Join(root, "Concepts", "japanese")
	if err := os.MkdirAll(conceptDir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"),
		[]byte("---\ntitle: は (主題助詞)\ntype: concept\n---\n\nMarks the topic of the sentence.\n"), 0o644); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	lessonDir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lessonDir, "L01.md"),
		[]byte("---\ntitle: L01\ntype: lesson\nstatus: draft\n---\n\nThe particle [[は]] marks the topic.\n"), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// The wikilink became a trigger but stayed a real navigable link.
	if !strings.Contains(page, `data-concept=`) || !strings.Contains(page, `class="wikilink concept-link"`) {
		t.Errorf("concept wikilink not marked as a trigger; body = %q", page)
	}
	if !strings.Contains(page, `href="/notes/Concepts/japanese/`) {
		t.Errorf("concept trigger lost its navigable href (no-JS fallback); body = %q", page)
	}
	// The concept was pre-rendered into a hidden template + the shared dialog.
	for _, want := range []string{`<template id="concept-`, `Marks the topic of the sentence.`, `data-concept-sheet`, `data-concept-body`} {
		if !strings.Contains(page, want) {
			t.Errorf("concept sheet missing %q", want)
		}
	}
}

// TestShowNonLessonNoConceptTriggers confirms the gate: a non-lesson note that
// links to a concept navigates as usual — no trigger, no sheet.
func TestShowNonLessonNoConceptTriggers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conceptDir := filepath.Join(root, "Concepts", "japanese")
	if err := os.MkdirAll(conceptDir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"), []byte("topic particle\n"), 0o644); err != nil {
		t.Fatalf("write concept: %v", err)
	}
	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S.md"),
		[]byte("---\ntitle: S\ntype: source-note\nstatus: draft\n---\n\nMentions [[は]] in passing.\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	_, page := get(t, srv.URL+"/notes/Sources/S.md")
	if strings.Contains(page, "data-concept") || strings.Contains(page, "data-concept-sheet") {
		t.Errorf("non-lesson note grew a concept trigger/sheet — the gate failed; body = %q", page)
	}
}

func TestShowNotFound(t *testing.T) {
	t.Parallel()
	srv := newServer(t, t.TempDir())

	code, _ := get(t, srv.URL+"/notes/nope.md")
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// TestHome pins the landing page's observable contract: a direct 200 in the
// shared shell, one site marker for each dashboard block, and the vault README
// rendered beneath them. The site markers are asserted by name rather than by
// their position in the document, so rearranging the dashboard cannot blame a
// correct page for violating a declaration-order accident.
func TestHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nREADME body sentinel.\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// Refuse redirects: a followed README redirect can produce the same body
	// while violating Home's direct-render contract.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Location"); got != "" {
		t.Errorf("GET / Location = %q, want no redirect", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	pageHTML := string(body)
	for name, marker := range map[string]string{
		"recently changed": `data-home-block="recent"`,
		"lifecycle":        `data-home-block="lifecycle"`,
		"study paths":      `data-home-block="study-paths"`,
		"search":           `data-home-block="search"`,
		"vault README":     "README body sentinel.",
		"topbar":           `class="y-header"`,
		"command palette":  `data-search`,
	} {
		if !strings.Contains(pageHTML, marker) {
			t.Errorf("GET / is missing the %s marker %q", name, marker)
		}
	}
}

// TestReadingRoutesRenderAgainstRequestSnapshot is the coherence guard for a
// scanner swap during a request. The long-lived Renderer below is deliberately
// bound to a stale Store whose graph does not contain Target, while the request
// Snapshot does. Both Home and the ordinary note page must resolve the README's
// wikilink from the request snapshot, alongside that same snapshot's navigation
// and counts, rather than consulting the stale live resolver independently.
func TestReadingRoutesRenderAgainstRequestSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nSee [[Target]].\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	log := slog.New(slog.DiscardHandler)
	staleStore := snapshot.New(root, log, schema.NavigationRoles{}, schema.ArtifactPolicy{})
	renderer := render.New(root, staleStore.Resolver())

	conceptDir := filepath.Join(root, "Concepts")
	if err := os.MkdirAll(conceptDir, 0o750); err != nil {
		t.Fatalf("mkdir Concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conceptDir, "Target.md"), []byte("# Target\n"), 0o644); err != nil {
		t.Fatalf("write Target: %v", err)
	}
	currentStore := snapshot.New(root, log, schema.NavigationRoles{}, schema.ArtifactPolicy{})

	mux := http.NewServeMux()
	h := note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   renderer,
		Status:     status.NewService(root, nil, schema.ArtifactPolicy{}),
		Snapshot:   currentStore.Current,
		Provenance: func(context.Context, string) (string, error) { return "", nil },
		Log:        log,
	})
	h.Register(mux)

	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "home", path: "/"},
		{name: "note", path: "/notes/README.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, rr.Code, http.StatusOK)
			}
			want := `<a href="/notes/Concepts/Target.md" class="wikilink">Target</a>`
			if !strings.Contains(rr.Body.String(), want) {
				t.Errorf("GET %s did not resolve against the request snapshot; want %q in body", tt.path, want)
			}
		})
	}
}

func TestReadingFacesReadOneRequestSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nHome body.\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("plain source\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	log := slog.New(slog.DiscardHandler)
	store := snapshot.New(root, log, schema.NavigationRoles{}, schema.ArtifactPolicy{})

	for _, tt := range []struct {
		name, path string
	}{
		{name: "home", path: "/"},
		{name: "note", path: "/notes/README.md"},
		{name: "file", path: "/notes/plain.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			mux := http.NewServeMux()
			note.NewHandler(note.Deps{
				Root:     root,
				Renderer: render.New(root, store.Current().Graph),
				Status:   status.NewService(root, nil, schema.ArtifactPolicy{}),
				Snapshot: func() *snapshot.Snapshot {
					calls++
					return store.Current()
				},
				Provenance: func(context.Context, string) (string, error) { return "", nil },
				Log:        log,
			}).Register(mux)
			rr := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, rr.Code, http.StatusOK)
			}
			if calls != 1 {
				t.Errorf("GET %s snapshot reads = %d, want 1", tt.path, calls)
			}
		})
	}
}

// TestHomeDashboardUsesSnapshotData pins the four blocks beyond their site
// markers. Recently changed is the newest seven typed notes in mtime order;
// Lifecycle links the contract-provided statuses; Study paths reports the same
// ready/total tally as its full page. The recent section is scoped by its own
// section marker, never by whichever block happens to follow it.
func TestHomeDashboardUsesSnapshotData(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nDashboard README sentinel.\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	conceptDir := filepath.Join(root, "Concepts")
	if err := os.MkdirAll(conceptDir, 0o750); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for i := range 8 {
		name := fmt.Sprintf("Note %d", i)
		full := filepath.Join(conceptDir, fmt.Sprintf("note-%d.md", i))
		content := fmt.Sprintf("---\ntitle: %s\ntype: concept\nstatus: draft\n---\n\nbody\n", name)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		modified := base.Add(time.Duration(i) * 24 * time.Hour)
		if err := os.Chtimes(full, modified, modified); err != nil {
			t.Fatalf("set %s mtime: %v", name, err)
		}
	}

	lessonDir := filepath.Join(root, "Writing", "lessons")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lessons: %v", err)
	}
	for name, statusName := range map[string]string{"Open": "draft", "Sealed": schema.SealStatus} {
		content := fmt.Sprintf("---\ntitle: %s\ntype: lesson\nstatus: %s\n---\n\nbody\n", name, statusName)
		full := filepath.Join(lessonDir, name+".md")
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write lesson %s: %v", name, err)
		}
		if err := os.Chtimes(full, base.Add(-24*time.Hour), base.Add(-24*time.Hour)); err != nil {
			t.Fatalf("set lesson %s mtime: %v", name, err)
		}
	}
	mapDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapDir, 0o750); err != nil {
		t.Fatalf("mkdir maps: %v", err)
	}
	pathBody := "---\ntitle: Test path\ntype: study-path\n---\n\n## Part\n\n- [[Open]]\n- [[Sealed]]\n"
	pathFile := filepath.Join(mapDir, "path.md")
	if err := os.WriteFile(pathFile, []byte(pathBody), 0o644); err != nil {
		t.Fatalf("write study path: %v", err)
	}
	if err := os.Chtimes(pathFile, base.Add(-24*time.Hour), base.Add(-24*time.Hour)); err != nil {
		t.Fatalf("set study path mtime: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}

	recent := homeSection(t, body, `data-home-block="recent"`)
	if got := strings.Count(recent, "data-home-recent-note"); got != 7 {
		t.Errorf("recent note rows = %d, want 7", got)
	}
	previous := -1
	for i := 7; i >= 1; i-- {
		marker := fmt.Sprintf("Note %d", i)
		at := strings.Index(recent, marker)
		if at < 0 {
			t.Errorf("recent section is missing %q", marker)
			continue
		}
		if previous >= 0 && at <= previous {
			t.Errorf("recent section order places %q at %d after the prior newer note at %d", marker, at, previous)
		}
		previous = at
	}
	if strings.Contains(recent, "Note 0") {
		t.Error("recent section includes the eighth-newest note, want the newest seven")
	}

	lifecycle := homeSection(t, body, `data-home-block="lifecycle"`)
	for _, marker := range []string{"status%3Adraft", "status%3Aready", "draft", schema.SealStatus} {
		if !strings.Contains(lifecycle, marker) {
			t.Errorf("Lifecycle block is missing %q", marker)
		}
	}
	paths := homeSection(t, body, `data-home-block="study-paths"`)
	for _, marker := range []string{"Test path", "1 / 2 ready", "/syllabus/Maps/path.md"} {
		if !strings.Contains(paths, marker) {
			t.Errorf("Study paths block is missing %q", marker)
		}
	}
	if !strings.Contains(body, "Dashboard README sentinel.") {
		t.Error("Home is missing the rendered vault README body")
	}
	if !strings.Contains(body, `aria-label="1 notes have a legal next status"`) {
		t.Error("Home topbar is missing the snapshot-derived pending chip")
	}
}

func TestHomeLifecycleDiagnostic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contract func(*testing.T) *schema.Schema
		want     string
	}{
		{
			name:     "closed core contract",
			contract: func(*testing.T) *schema.Schema { return nil },
			want:     "Lifecycle is unavailable while the contract is closed.",
		},
		{
			name: "missing artifact policy",
			contract: func(t *testing.T) *schema.Schema {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "")
			},
			want: "contract declares no artifact policy; instance projections disabled until it does",
		},
		{
			name: "invalid artifact policy",
			contract: func(t *testing.T) *schema.Schema {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\".\"]\n")
			},
			want: `invalid artifact policy: non_instance_dirs contains "."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
				t.Fatalf("write README: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract(t))
			code, body := get(t, srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}
			lifecycle := html.UnescapeString(homeSection(t, body, `data-home-block="lifecycle"`))
			for _, want := range []string{"data-home-lifecycle-diagnostic", tt.want} {
				if !strings.Contains(lifecycle, want) {
					t.Errorf("Lifecycle diagnostic missing %q; section = %q", want, lifecycle)
				}
			}
		})
	}
}

func TestHomeArtifactPolicyDegradesInstanceProjections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract *schema.Schema
		want     string
	}{
		{name: "missing", contract: loadHomeContractWithArtifactSection(t, ""), want: "contract declares no artifact policy; instance projections disabled until it does"},
		{name: "invalid", contract: loadHomeContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\".\"]\n"), want: `invalid artifact policy: non_instance_dirs contains "."`},
		{name: "incomplete", contract: loadHomeContractWithArtifactSection(t, "[artifacts]\n"), want: `invalid artifact policy: missing required key "non_instance_dirs"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
				t.Fatalf("write README: %v", err)
			}
			if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
				t.Fatalf("mkdir Writing: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "Writing", "Draft.md"), []byte("---\ntitle: Draft\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract)
			code, page := get(t, srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}
			diagnostic := tt.want
			for _, block := range []struct {
				marker, diagnosticMarker string
			}{
				{marker: `data-home-block="recent"`, diagnosticMarker: `data-home-recent-diagnostic`},
				{marker: `data-home-block="lifecycle"`, diagnosticMarker: `data-home-lifecycle-diagnostic`},
				{marker: `data-home-block="study-paths"`, diagnosticMarker: `data-home-paths-diagnostic`},
			} {
				section := html.UnescapeString(homeSection(t, page, block.marker))
				for _, want := range []string{block.diagnosticMarker, diagnostic} {
					if !strings.Contains(section, want) {
						t.Errorf("Home block %s is missing %q; section = %q", block.marker, want, section)
					}
				}
			}
			if strings.Contains(page, `data-advanceable-chip`) {
				t.Error("Home pending chip remained available without artifact metadata")
			}
			if !strings.Contains(page, `data-home-block="search"`) {
				t.Error("Home search block disappeared during artifact degradation")
			}
		})
	}
}

func TestHomeNavigationFailureLeavesArtifactAggregatesOperational(t *testing.T) {
	t.Parallel()
	const invalidNavigation = "[navigation]\npath_types = [\"missing-type\"]\nmap_types = []\n"
	const validArtifact = "[artifacts]\nnon_instance_dirs = [\"System/templates\"]\n"
	contract := loadHomeContractWithSections(t, invalidNavigation, validArtifact)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
		t.Fatalf("mkdir Writing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Writing", "Draft.md"), []byte("---\ntitle: Aggregate sentinel\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, contract)
	code, page := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, page, `data-home-block="recent"`)
	if !strings.Contains(recent, "Aggregate sentinel") || strings.Contains(recent, "data-home-recent-diagnostic") {
		t.Errorf("Recent degraded with navigation roles; section = %q", recent)
	}
	lifecycle := homeSection(t, page, `data-home-block="lifecycle"`)
	if strings.Contains(lifecycle, "data-home-lifecycle-diagnostic") {
		t.Errorf("Lifecycle degraded with navigation roles; section = %q", lifecycle)
	}
	draftRow := homeLifecycleRow(t, lifecycle, "draft")
	if !strings.Contains(draftRow, `aria-label="1 notes">1</span>`) {
		t.Errorf("Lifecycle draft count degraded with navigation roles; row = %q", draftRow)
	}
	paths := html.UnescapeString(homeSection(t, page, `data-home-block="study-paths"`))
	for _, want := range []string{`data-home-paths-diagnostic`, `missing-type`} {
		if !strings.Contains(paths, want) {
			t.Errorf("Study Paths is missing navigation diagnostic %q; section = %q", want, paths)
		}
	}
	if strings.Contains(paths, "contract declares no artifact policy; instance projections disabled until it does") {
		t.Errorf("Study Paths falsely reports artifact failure: %q", paths)
	}
	if !strings.Contains(page, `aria-label="1 notes have a legal next status"`) {
		t.Error("pending chip was suppressed by navigation-only failure")
	}
}

func TestHomeStudyPathsReportsBothCapabilityFailures(t *testing.T) {
	t.Parallel()
	const invalidNavigation = "[navigation]\npath_types = [\"missing-type\"]\nmap_types = []\n"
	const invalidArtifact = "[artifacts]\nnon_instance_dirs = [\".\"]\n"
	contract := loadHomeContractWithSections(t, invalidNavigation, invalidArtifact)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServerWithContract(t, root, contract)
	code, page := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	paths := html.UnescapeString(homeSection(t, page, `data-home-block="study-paths"`))
	for _, want := range []string{
		`data-home-paths-diagnostic`,
		`missing-type`,
		`invalid artifact policy: non_instance_dirs contains "."`,
	} {
		if !strings.Contains(paths, want) {
			t.Errorf("Study Paths is missing concurrent capability diagnostic %q; section = %q", want, paths)
		}
	}
	if got := strings.Count(paths, `data-home-paths-diagnostic`); got != 2 {
		t.Errorf("Study Paths diagnostic rows = %d, want 2", got)
	}
}

func TestHomeValidPolicyExcludesNonInstancesFromRecentAndCounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	for rel, title := range map[string]string{
		"Writing/Instance.md":          "Governed draft sentinel",
		"System/templates/Template.md": "LOUD TEMPLATE DRAFT SENTINEL",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		content := fmt.Sprintf("---\ntitle: %s\ntype: writing\nstatus: draft\n---\n\nbody\n", title)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, page := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, page, `data-home-block="recent"`)
	if !strings.Contains(recent, "Governed draft sentinel") {
		t.Error("Recent is missing the governed instance")
	}
	if strings.Contains(recent, "LOUD TEMPLATE DRAFT SENTINEL") {
		t.Error("Recent includes a non-instance template")
	}
	lifecycle := homeSection(t, page, `data-home-block="lifecycle"`)
	draftRow := homeLifecycleRow(t, lifecycle, "draft")
	if !strings.Contains(draftRow, `aria-label="1 notes">1</span>`) {
		t.Errorf("draft lifecycle count includes the template or misses the instance; row = %q", draftRow)
	}
	if !strings.Contains(page, `aria-label="1 notes have a legal next status"`) {
		t.Error("pending count includes the template or misses the instance")
	}
}

// TestHomeWithoutReadmeIsNotReady ensures a missing README is an honest 404,
// not a blank dashboard 200 that the readiness poll could mistake for Home.
func TestHomeWithoutReadmeIsNotReady(t *testing.T) {
	t.Parallel()
	srv := newServer(t, t.TempDir())
	code, _ := get(t, srv.URL+"/")
	if code != http.StatusNotFound {
		t.Errorf("GET / without README status = %d, want %d", code, http.StatusNotFound)
	}
}

func homeSection(t *testing.T, body, marker string) string {
	t.Helper()
	markerAt := strings.Index(body, marker)
	if markerAt < 0 {
		t.Fatalf("Home body is missing section marker %q", marker)
	}
	openAt := strings.LastIndex(body[:markerAt], "<section")
	if openAt < 0 {
		t.Fatalf("Home marker %q is not inside a section", marker)
	}
	closeAt := strings.Index(body[markerAt:], "</section>")
	if closeAt < 0 {
		t.Fatalf("Home section %q has no closing tag", marker)
	}
	return body[openAt : markerAt+closeAt+len("</section>")]
}

func homeLifecycleRow(t *testing.T, section, statusName string) string {
	t.Helper()
	marker := `href="/search?q=` + url.QueryEscape("status:"+statusName) + `"`
	markerAt := strings.Index(section, marker)
	if markerAt < 0 {
		t.Fatalf("Lifecycle has no %q row", statusName)
	}
	openAt := strings.LastIndex(section[:markerAt], "<a")
	if openAt < 0 {
		t.Fatalf("Lifecycle %q marker is not inside a row", statusName)
	}
	closeAt := strings.Index(section[markerAt:], "</a>")
	if closeAt < 0 {
		t.Fatalf("Lifecycle %q row has no closing tag", statusName)
	}
	return section[openAt : markerAt+closeAt+len("</a>")]
}

// TestShowNoFrontmatter exercises handler.go's NoFrontmatter branch with a
// loaded (non-nil) contract, so no write diagnostic masks note.templ's
// statusPanel cannot fall into the "Contract unavailable" fail-closed case first —
// the only way to actually observe that show() set view.NoFrontmatter
// instead of leaving it false.
func TestShowNoFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Drills")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No frontmatter block at all — legal per the contract's
	// no_frontmatter_is_legal (e.g. drills).
	if err := os.WriteFile(filepath.Join(dir, "d1.md"), []byte("just a drill body\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, body := get(t, srv.URL+"/notes/Drills/d1.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "No frontmatter") {
		t.Errorf("page missing the no-frontmatter notice; body = %q", body)
	}
	if strings.Contains(body, "Contract unavailable") || strings.Contains(body, "fail-closed") {
		t.Errorf("page shows the fail-closed notice even though the contract loaded; body = %q", body)
	}
}

// TestShowTransitions exercises handler.go's default branch (view.Transitions
// = h.statusSvc.Transitions(n.RelPath, n.Type(), n.Status())) with a loaded contract.
// Getting the argument order backwards (Transitions(current, noteType)) or
// swapping the switch's case order would silently render the wrong panel —
// this test is the only one in the repo that would catch either.
func TestShowTransitions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Test Vault")
	runGit(t, root, "config", "user.email", "test-vault@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, root, "add", "Writing/lessons/japanese/L01.md")
	runGit(t, root, "commit", "-m", "seed lesson")
	before := len(strings.Split(strings.TrimSpace(runGit(t, root, "log", "--oneline")), "\n"))
	contract := loadContract(t)
	srv := newServerWithContract(t, root, contract)

	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	// draft -> [ready, archived] for actor "koopa" per
	// testdata/contract.toml's lifecycle table (cross-checked by hand,
	// mirroring internal/status/status_test.go's TestTransitions).
	transitions := status.NewService(root, contract, contract.ArtifactPolicy()).Transitions("Writing/lessons/japanese/L01.md", "lesson", "draft")
	if len(transitions) != 2 {
		t.Fatalf("Transitions() = %v, want two targets", transitions)
	}
	if !slices.Contains(transitions, schema.SealStatus) {
		t.Fatalf("Transitions() = %v; fixture must keep a draft→seal edge to %q for this test", transitions, schema.SealStatus)
	}
	for _, target := range transitions {
		want := `value="` + target + `"`
		if !strings.Contains(body, want) {
			t.Errorf("page missing transition key %s; body = %q", want, body)
		}
	}
	start := strings.Index(body, "<form class=\"y-statusform\" method=\"post\" action=\"/status\" data-seal>")
	if start < 0 {
		t.Fatalf("page missing the primary seal form for contract target %q", schema.SealStatus)
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatalf("primary seal form is unterminated; body = %q", body[start:])
	}
	sealForm := body[start : start+end]
	if want := `name="to" value="` + schema.SealStatus + `"`; !strings.Contains(sealForm, want) {
		t.Errorf("primary seal form is missing the schema seal target %q; form = %q", want, sealForm)
	}
	form := url.Values{
		"path": {hiddenValue(t, sealForm, "path")},
		"from": {hiddenValue(t, sealForm, "from")},
		"to":   {hiddenValue(t, sealForm, "to")},
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/status", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new status request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /status = %d, want %d; body = %q", resp.StatusCode, http.StatusSeeOther, b)
	}
	if got, want := resp.Header.Get("Location"), "/notes/Writing/lessons/japanese/L01.md?sealed=1"; got != want {
		t.Errorf("POST /status Location = %q, want %q", got, want)
	}
	got, err := os.ReadFile(filepath.Join(dir, "L01.md")) // #nosec G304 -- dir is under t.TempDir and the filename is fixed by this test
	if err != nil {
		t.Fatalf("read flipped lesson: %v", err)
	}
	want := strings.Replace(lessonMD, "status: draft", "status: "+schema.SealStatus, 1)
	if string(got) != want {
		t.Errorf("lesson after POST differs outside the one status line:\ngot:  %q\nwant: %q", got, want)
	}
	after := len(strings.Split(strings.TrimSpace(runGit(t, root, "log", "--oneline")), "\n"))
	if after != before+1 {
		t.Errorf("commit count = %d, want %d (exactly one new commit)", after, before+1)
	}
	if strings.Contains(body, "Contract unavailable") || strings.Contains(body, "fail-closed") || strings.Contains(body, "No frontmatter") {
		t.Errorf("page shows the wrong status-panel branch; body = %q", body)
	}
}

// TestNewHandlerPanicsOnNilStatusPolicy mirrors
// internal/status/handler_test.go's coverage of status.NewHandler's own
// nil-dependency panic: a fail-closed *status.Service is a valid
// StatusPolicy (Closed() reports true), but a literal nil is not — a
// future caller passing one must fail at wiring time, not three calls deep
// inside the first GET /notes/... request.
func TestNewHandlerPanicsOnNilStatusPolicy(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewHandler(nil StatusPolicy) did not panic")
		}
	}()
	root := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	store := snapshot.New(root, log, schema.NavigationRoles{}, schema.ArtifactPolicy{})
	note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, store.Resolver()),
		Status:     nil, // the nil under test
		Snapshot:   store.Current,
		Provenance: func(context.Context, string) (string, error) { return "", nil },
		Log:        log,
	})
}

// TestNewHandlerPanicsOnNilSnapshot mirrors the StatusPolicy check: a provider
// returning an empty-but-valid snapshot is legal, but a nil provider is a
// wiring bug that must fail at construction rather than inside the first read.
func TestNewHandlerPanicsOnNilSnapshot(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewHandler(nil Snapshot provider) did not panic")
		}
	}()
	root := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	store := snapshot.New(root, log, schema.NavigationRoles{}, schema.ArtifactPolicy{})
	note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, store.Resolver()),
		Status:     status.NewService(root, nil, schema.ArtifactPolicy{}),
		Snapshot:   nil, // the nil under test
		Provenance: func(context.Context, string) (string, error) { return "", nil },
		Log:        log,
	})
}

// TestShowIncludesSidebar is the navigation-face regression: the reading
// page must still render AND now carry the plain sidebar — a known
// lifecycle folder, a study-path title, and a resolvable lesson link with
// its status badge. It builds a tiny vault with one syllabus and one lesson
// so the assertions are hand-derived, not tautological.
func TestShowIncludesSidebar(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lessonMD := "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lessonMD), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	syllabus := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字\n\n- [[Slices]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(syllabus), 0o644); err != nil {
		t.Fatalf("write syllabus: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, body := get(t, srv.URL+"/notes/Maps/Go path.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`class="y-rail-left"`, // the nav rail rendered at all
		"Writing",             // a lifecycle folder in the collapsed Folders tree
		"Go path",             // the study-path title
		"Data",                // the pipe-format H2's English label
		`href="/notes/Writing/lessons/golang/Slices.md"`, // the resolved lesson link
		"draft", // the lesson's status badge
	} {
		if !strings.Contains(body, want) {
			t.Errorf("reading page sidebar missing %q; body = %q", want, body)
		}
	}
}

func TestShowKeepsUnresolvedGeneralMapRowOnNotePageOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mapDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mapNote := "---\ntitle: Reading map\ntype: topic-map\ndomain: humanities\n---\n\n## Themes\n\n- [[Ghost Essay]]\n"
	if err := os.WriteFile(filepath.Join(mapDir, "Reading map.md"), []byte(mapNote), 0o644); err != nil {
		t.Fatalf("write map: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/notes/Maps/Reading map.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`data-map-tree="Maps/Reading map.md"`,
		`class="wikilink-broken">Ghost Essay</span>`,
		`wikilink &#34;Ghost Essay&#34; does not resolve to any note or file`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("general map note page is missing %q", want)
		}
	}
	asideStart := strings.Index(body, `<aside class="y-rail-left"`)
	if asideStart < 0 {
		t.Fatal("response has no left sidebar")
	}
	asideEnd := strings.Index(body[asideStart:], `</aside>`)
	if asideEnd < 0 {
		t.Fatalf("response has no complete left sidebar")
	}
	sidebar := body[asideStart : asideStart+asideEnd]
	if strings.Contains(sidebar, "Ghost Essay") {
		t.Error("general map unresolved row appears in sidebar navigation")
	}
}
