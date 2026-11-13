package note_test

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func openReadingVault(t *testing.T, root string) *vault.Reader {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("Reader.Close() error = %v", err)
		}
	})
	return reader
}

func newSnapshotStore(
	t *testing.T,
	root string,
	log *slog.Logger,
	contract *schema.Contract,
	governance schema.Governance,
) (*snapshot.Store, *vault.Reader) {
	t.Helper()
	source := openReadingVault(t, root)
	store, err := snapshot.New(t.Context(), source, log, contract, governance)
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store, source
}

// newServer wires the reading page with no contract, the shape of any folder
// yomihon is pointed at before it is given one.
func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	return newServerWithContract(t, root, nil)
}

// newServerWithContract is newServer with an explicit contract.
func newServerWithContract(t *testing.T, root string, contract *schema.Contract) *httptest.Server {
	t.Helper()
	governance := schema.Ungoverned()
	if contract != nil {
		governance = contract.Governance()
	}
	return newServerWithGovernance(t, root, contract, governance)
}

// newServerWithGovernance is newServerWithContract for the cases where the
// contract is absent and which absence it is matters: a folder that never
// carried one, or one whose contract exists and could not be read.
func newServerWithGovernance(
	t *testing.T,
	root string,
	contract *schema.Contract,
	governance schema.Governance,
) *httptest.Server {
	t.Helper()
	return newServerWithMark(t, root, contract, governance, noMark)
}

// noMark is the reading state of a reader who has kept no place, which is
// every test but the ones about the desk's row back to one.
func noMark() (mark.Continuation, bool) { return mark.Continuation{}, false }

// newServerWithMark is newServerWithGovernance with a place the reader kept,
// so the desk's row back to it can be driven without a file on disk.
func newServerWithMark(
	t *testing.T,
	root string,
	contract *schema.Contract,
	governance schema.Governance,
	kept func() (mark.Continuation, bool),
) *httptest.Server {
	t.Helper()
	return newServerKeepingPlaces(t, root, contract, governance, kept, "")
}

// newServerKeepingPlaces is newServerWithMark for a process that also keeps
// places: markAddress is where the control for keeping one posts, and the
// control is drawn only where that is not empty.
func newServerKeepingPlaces(
	t *testing.T,
	root string,
	contract *schema.Contract,
	governance schema.Governance,
	kept func() (mark.Continuation, bool),
	markAddress string,
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, governance)
	h := note.New(&note.Sources{
		Source:       source,
		Contract:     contract,
		VaultName:    shell.VaultName(source.Name()),
		Authority:    func() schema.Authority { return schema.CaptureAuthority(contract, governance) },
		Snapshot:     store.Current,
		Continuation: kept,
		MarkAddress:  markAddress,
		Log:          log,
	})
	h.Register(mux)
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
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"), []byte("---\ntype: writing\n---\n# は\n\nConcept body.\n"), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir lesson directory: %v", err)
	}
	body := "---\ntype: lesson\nslug: " + loudLessonSlug + "\n---\n\n" +
		"| A | B |\n|---|---|\n| x | y |\n\n" +
		"<ruby>今日<rt>きょう</rt></ruby>は晴れ。 [[は]] " + loudLessonSentinel + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(slotsDir, "template.yaml"), []byte(sidecar), 0o600); err != nil {
		t.Fatalf("write slot sidecar: %v", err)
	}
}

func TestShowNonInstanceLessonHasNoGovernanceOrLessonEnhancements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const templateRel = "System/templates/Loud lesson.md"
	writeLoudLessonFixture(t, root, templateRel)

	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.Client(), srv.URL+"/notes/System/templates/Loud%20lesson.md")
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
	for _, absent := range []string{
		`data-tts=`,
		"y-slotmachine",
		`data-concept=`,
		`data-concept-sheet`,
	} {
		if strings.Contains(page, absent) {
			t.Errorf("non-instance lesson page unexpectedly contains %q", absent)
		}
	}
}

func TestShowUsesOneAuthorityViewAndClosesTheNextRequestAfterDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const lessonRel = "Writing/lessons/japanese/Loud lesson.md"
	writeLoudLessonFixture(t, root, lessonRel)
	contractBytes, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, contractBytes, 0o600); err != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("write mutable contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	capture := func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) }
	captures := 0
	counted := func() schema.Authority {
		captures++
		return capture()
	}

	mux := http.NewServeMux()
	handler := note.New(&note.Sources{
		Continuation: noMark,
		Source:       source,
		Authority:    counted,
		Snapshot:     store.Current,
		Log:          log,
	})
	handler.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
	if code != http.StatusOK {
		t.Fatalf("GET lesson status = %d, want 200", code)
	}
	for _, want := range []string{`data-concept=`, "y-slotmachine"} {
		if !strings.Contains(page, want) {
			t.Errorf("captured-open request is missing %q", want)
		}
	}
	if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("change contract between requests: %v", writeErr)
	}

	code, page = get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
	if code != http.StatusOK {
		t.Fatalf("second GET lesson status = %d, want 200", code)
	}
	const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if !strings.Contains(page, diagnostic) {
		t.Error("next reading request is missing the latched authority diagnostic")
	}
	for _, leaked := range []string{
		`data-tts=`,
		`data-concept=`,
		`data-concept-sheet`,
	} {
		if strings.Contains(page, leaked) {
			t.Errorf("next reading request retained %q after authority drift", leaked)
		}
	}
	if captures != 2 {
		t.Errorf("two reading requests captured the authority %d times, want exactly once per request", captures)
	}
}

func TestShowClosesInstanceProjectionsForEitherAuthorityCaptureOrder(t *testing.T) {
	t.Parallel()

	for _, snapshotFirst := range []bool{true, false} {
		name := "status-first"
		if snapshotFirst {
			name = "snapshot-first"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const lessonRel = "Writing/lessons/japanese/Loud lesson.md"
			writeLoudLessonFixture(t, root, lessonRel)
			contractBytes, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
			if err != nil {
				t.Fatalf("read contract fixture: %v", err)
			}
			contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
			if writeErr := os.WriteFile(contractPath, contractBytes, 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
				t.Fatalf("write mutable contract: %v", writeErr)
			}
			contract, err := schema.LoadFile(contractPath)
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			log := slog.New(slog.DiscardHandler)
			store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
			capture := func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) }

			var authority schema.Authority
			var captured *snapshot.Generation
			if snapshotFirst {
				captured = store.Current().Capture()
			} else {
				authority = capture()
			}
			if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
				t.Fatalf("change contract between captures: %v", writeErr)
			}
			if snapshotFirst {
				authority = capture()
			} else {
				captured = store.Current().Capture()
			}

			mux := http.NewServeMux()
			note.New(&note.Sources{
				Continuation: noMark,
				Source:       source,
				Authority:    func() schema.Authority { return authority },
				Snapshot:     func() *snapshot.Generation { return captured },
				Log:          log,
			}).Register(mux)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
			if code != http.StatusOK {
				t.Fatalf("GET lesson status = %d, want 200", code)
			}
			const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
			if !strings.Contains(page, diagnostic) {
				t.Error("reading page is missing the authority diagnostic")
			}
			for _, leaked := range []string{
				`data-tts=`,
				`data-concept=`,
				`data-concept-sheet`,
				"y-slotmachine",
			} {
				if strings.Contains(page, leaked) {
					t.Errorf("reading page retained %q across torn authority captures", leaked)
				}
			}
		})
	}
}

// TestShowCarriesTheNavigationFaultOnANotePage pins the path the reader
// actually takes to a note: the command palette opens one directly and never
// renders Home. A contract whose navigation declaration was rejected closes
// paths and maps while leaving the recent list working, so
// the rail is the only place that fault can appear — and if it appears nowhere
// on this route, a reader who lives in the palette never learns their contract
// is wrong.
func TestShowCarriesTheNavigationFaultOnANotePage(t *testing.T) {
	t.Parallel()

	const rejectedNavigation = "[types.study-path]\nrole = \"missing-role\"\n"
	const validArtifact = "templates = [\"System/templates\"]\n"
	contract := loadHomeContractWithSections(t, rejectedNavigation, validArtifact)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Note.md"), []byte("---\ntype: writing\n---\n# Note\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, contract)
	code, body := get(t, srv.Client(), srv.URL+"/notes/Note.md")
	if code != http.StatusOK {
		t.Fatalf("GET note status = %d, want 200", code)
	}
	page := html.UnescapeString(body)

	if strings.Contains(page, "data-home-block") {
		t.Fatal("the fixture rendered Home, not a note page")
	}
	rail := pageSection(page, `<section class="y-navdiag"`)
	if rail == "" {
		t.Fatalf("the note page carries no capability fault at all; page = %q", page)
	}
	if !strings.Contains(rail, "missing-role") {
		t.Errorf("the rail does not name the rejected declaration: %q", rail)
	}
	if strings.Contains(rail, "artifact policy") {
		t.Errorf("the rail reports an artifact failure that did not happen: %q", rail)
	}
}

func TestShowFileCapturesStatusOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.txt"), []byte("guide\n"), 0o600); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	capture := func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) }
	statusCaptures := 0
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Continuation: noMark,
		Source:       source,
		Authority: func() schema.Authority {
			statusCaptures++
			return capture()
		},
		Snapshot: store.Current,
		Log:      log,
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, _ := get(t, srv.Client(), srv.URL+"/notes/guide.txt")
	if code != http.StatusOK {
		t.Fatalf("GET source status = %d, want 200", code)
	}
	if statusCaptures != 1 {
		t.Errorf("source request captured status %d times, want exactly once", statusCaptures)
	}
}

func TestShowUnavailableArtifactPolicyDoesNotAssumeLessonInstance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		section string
		want    string
	}{
		{name: "invalid", section: "templates = [\".\"]\n", want: `invalid artifact policy: templates contains "."`},
		{name: "blank", section: "templates = [\"\"]\n", want: `invalid artifact policy: templates contains ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const rel = "Writing/lessons/japanese/Loud lesson.md"
			writeLoudLessonFixture(t, root, rel)

			contract := loadHomeContractWithArtifactSection(t, tt.section)
			srv := newServerWithContract(t, root, contract)
			code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
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
			// The page still names the declaration that could not be
			// honoured, so a reader is not left with a plain page and no reason.
			if !strings.Contains(html.UnescapeString(page), tt.want) {
				t.Errorf("lesson page does not state why the artifact policy is unavailable: %q", tt.want)
			}
			for _, absent := range []string{
				`data-tts=`,
				"y-slotmachine",
				`data-concept=`,
				`data-concept-sheet`,
			} {
				if strings.Contains(page, absent) {
					t.Errorf("lesson with unavailable artifact policy unexpectedly contains %q", absent)
				}
			}
		})
	}
}

func noteMain(t *testing.T, page string) string {
	t.Helper()
	openAt := strings.Index(page, `<main id="_y-main" tabindex="-1" class="y-main"`)
	if openAt < 0 {
		t.Fatal("note page has no main reading surface")
	}
	closeAt := strings.Index(page[openAt:], "</main>")
	if closeAt < 0 {
		t.Fatal("note page main has no closing tag")
	}
	return page[openAt : openAt+closeAt+len("</main>")]
}

// loadContract is a loader fixture, not a second schema: it reuses
// schema.LoadFile as-is against a lesson-only slice of the real contract
// shape (testdata/contract.toml).
func loadContract(t *testing.T) *schema.Contract {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

// loadHomeContract reuses the complete schema loader fixture, which declares
// every note type the desk and the folder index read, not only the lesson.
func loadHomeContract(t *testing.T) *schema.Contract {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(schema test contract) = %v", err)
	}
	return s
}

func loadHomeContractWithArtifactSection(t *testing.T, artifactSection string) *schema.Contract {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	const validSection = "templates = [\"System/templates\"]\n"
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

// loadHomeContractWithSections is the shared contract with the course role's
// table replaced by navigationSection and its templates line by artifactSection.
func loadHomeContractWithSections(t *testing.T, navigationSection, artifactSection string) *schema.Contract {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	const validNavigation = "[types.study-path]\nrole = \"course\"\n"
	const validArtifact = "templates = [\"System/templates\"]\n"
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

func get(t *testing.T, client *http.Client, urlStr string) (code int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", urlStr, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", urlStr, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

// TestShowUsesContractDeclaredArticleLanguage holds the whole path from a
// note's frontmatter to the language its article is served with. Only a value
// the contract gave authority to and the tag grammar accepts reaches the
// element; every other case leaves the article with no language of its own and
// the page's own language answering for it.
//
// So each case asserts two things at once, and the second is what makes the
// first mean anything: the article's opening tag in full, and that the page
// still declares the language the article is now leaning on. An article that
// states nothing inside a page that states nothing either would satisfy a
// check written only against the article, while reading as a note with no
// language anywhere.
func TestShowUsesContractDeclaredArticleLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		langLine    string
		contract    func(*testing.T) *schema.Contract
		wantArticle string
	}{
		{name: "canonicalizes declared tag", langLine: "lang: zh-hant\n", contract: loadContract, wantArticle: `<article class="y-article" lang="zh-Hant">`},
		{name: "invalid tag states no language", langLine: "lang: not_a_tag\n", contract: loadContract, wantArticle: `<article class="y-article">`},
		{name: "missing value states no language", contract: loadContract, wantArticle: `<article class="y-article">`},
		{name: "undeclared field has no authority", langLine: "lang: ja\n", contract: loadContractWithoutArticleLanguage, wantArticle: `<article class="y-article">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			rel := "Writing/日本語.md"
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatalf("mkdir note: %v", err)
			}
			body := "---\ntype: writing\n" + tt.langLine + "---\n\n本文\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract(t))
			code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/%E6%97%A5%E6%9C%AC%E8%AA%9E.md")
			if code != http.StatusOK {
				t.Fatalf("GET note = %d, want 200; body = %q", code, page)
			}
			if !strings.Contains(page, tt.wantArticle) {
				t.Errorf("article opening tag = missing %q in %q", tt.wantArticle, page)
			}
			const chrome = `<html lang="zh-Hant"`
			if !strings.Contains(page, chrome) {
				t.Errorf("the page no longer declares the language an article without one falls back to: want %q in %q", chrome, page)
			}
		})
	}
}

func loadContractWithoutArticleLanguage(t *testing.T) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	const declared = `"draft", "lang", "created"`
	without := strings.Replace(string(data), declared, `"draft", "created"`, 1)
	if without == string(data) || strings.Contains(without, declared) {
		t.Fatal("test contract language declaration was not removed exactly once")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(without), 0o600); writeErr != nil { // #nosec G703 -- fixed basename under this test's TempDir
		t.Fatalf("write contract without lang: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q): %v", path, err)
	}
	return contract
}

// TestReadingPageRejectsPathTraversal fires traversal-shaped requests at the
// reading route and asserts none escapes the vault root: no request is served
// (never 200) and no byte of a file outside the vault ever reaches the body.
// The defense is layered — the mux cleans dot segments, servable rejects any
// non-local or hidden segment, and the request snapshot admits only entries the
// rooted Reader enumerated — so this pins the observable contract rather than
// one layer. A regression in any layer surfaces as a 200 or leaked sentinel.
func TestReadingPageRejectsPathTraversal(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "vault")
	if err := os.MkdirAll(filepath.Join(root, "Notes"), 0o750); err != nil {
		t.Fatalf("mkdir vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Notes", "real.md"), []byte("a real note body\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	// A decoy one level above the vault root, reachable by traversal only if a
	// layer fails. Its sentinel must never appear in any response body.
	const sentinel = "yomihon-outside-vault-sentinel-never-serve-this"
	if err := os.WriteFile(filepath.Join(parent, "secret.md"), []byte(sentinel+"\n"), 0o600); err != nil {
		t.Fatalf("write decoy: %v", err)
	}
	srv := newServer(t, root)

	// Positive control: a genuine note is served, so a blanket 404 is not what
	// makes the traversal cases pass.
	if code, body := get(t, srv.Client(), srv.URL+"/notes/Notes/real.md"); code != http.StatusOK || !strings.Contains(body, "a real note") {
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
			code, body := get(t, srv.Client(), srv.URL+"/notes/"+p.path)
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
	lessonMD := "---\ntype: lesson\n---\n# L00 テスト課\n\n<ruby>今日<rt>きょう</rt></ruby>は<ruby>晴<rt>は</rt></ruby>れ。\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L00 テスト課.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServer(t, root)

	code, body := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L00 テスト課.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		"L00 テスト課",
		"<ruby>今日<rt>きょう</rt></ruby>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
	// Nothing governs this folder, so the page carries no apology for a
	// contract that was never promised. Reading is whole either way.
	for _, absent := range []string{
		"fail-closed",
	} {
		if strings.Contains(body, absent) {
			t.Errorf("ungoverned note page unexpectedly contains %q", absent)
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
		[]byte("---\ntype: lesson\n---\n# L00\n\n"+body), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S00.md"),
		[]byte("---\ntype: source-note\n---\n# S00\n\n"+body), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))

	_, lessonBody := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L00.md")
	if !strings.Contains(lessonBody, `data-tts="今日は晴れ。"`) {
		t.Errorf("lesson page missing the TTS button with reading-stripped data-tts; body = %q", lessonBody)
	}

	_, sourceBody := get(t, srv.Client(), srv.URL+"/notes/Sources/S00.md")
	if strings.Contains(sourceBody, "data-tts") || strings.Contains(sourceBody, "y-tts") {
		t.Errorf("non-lesson (source-note) page leaked a TTS button — the lesson gate failed; body = %q", sourceBody)
	}
	if !strings.Contains(sourceBody, "<ruby>今日<rt>きょう</rt></ruby>") {
		t.Errorf("non-lesson page lost its ruby (render must stay generic); body = %q", sourceBody)
	}
}

// TestReadingPageNeverExecutesANonNote pins what the reading page does with a
// file that is not a note.
// Every file the browse tree lists now opens, so a non-note resource no longer
// meets a 404 here. The guarantee that 404 was protecting is unchanged and is
// what this pins: a resource's markup never becomes live markup in this
// first-party, yomihon-origin page. A note receives only the renderer's inert
// authored-markup subset; every other kind is escaped into a source view, and
// its bytes reach the browser only through the sandboxed raw endpoint. The
// .html below carries a script tag precisely so that its inertness can be
// observed.
func TestReadingPageNeverExecutesANonNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	const liveTag = `<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>`
	write("Notes/keep.md", "# kept\n\nbody\n")
	write("Notes/live.html", `<!doctype html>`+liveTag+`<body>hi</body>`)
	write("Diagrams/x.canvas", "{\"nodes\":[]}\n")
	srv := newServer(t, root)

	if code, _ := get(t, srv.Client(), srv.URL+"/notes/Notes/keep.md"); code != http.StatusOK {
		t.Errorf("a .md note must still be served; GET keep.md = %d, want 200", code)
	}
	for _, rel := range []string{
		"Notes/live.html",
		"Diagrams/x.canvas",
	} {
		code, body := get(t, srv.Client(), srv.URL+"/notes/"+rel)
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
	if _, body := get(t, srv.Client(), srv.URL+"/notes/Notes/live.html"); !strings.Contains(body, "cdn.jsdelivr") {
		t.Error("the .html source view does not show the file's own text")
	}
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
	body := "---\ntype: lesson\nslug: jp-test-l01\n---\n# L01\n\n" +
		"| pattern | meaning |\n|---|---|\n| AはBです | A is B |\n\nAFTERTABLEBODY\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "L01.md"), []byte(body), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(slotsDir, "S1.yaml"), []byte(sidecar), 0o600); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
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
	// A lesson with a slug but no System/slots dir at all (Slots stays empty).
	body := "---\ntype: lesson\nslug: jp-orphan\n---\n# L02\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "L02.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))
	_, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L02.md")
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
		[]byte("---\ntype: concept\n---\n# は (主題助詞)\n\nMarks the topic of the sentence.\n"), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	lessonDir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lessonDir, "L01.md"),
		[]byte("---\ntype: lesson\n---\n# L01\n\nThe particle [[は]] marks the topic.\n"), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
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
	for _, want := range []string{`<template id="_y-concept-`, `Marks the topic of the sentence.`, `data-concept-sheet`, `data-concept-body`} {
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
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"), []byte("topic particle\n"), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}
	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S.md"),
		[]byte("---\ntype: source-note\n---\n# S\n\nMentions [[は]] in passing.\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv := newServerWithContract(t, root, loadContract(t))
	_, page := get(t, srv.Client(), srv.URL+"/notes/Sources/S.md")
	if strings.Contains(page, "data-concept") || strings.Contains(page, "data-concept-sheet") {
		t.Errorf("non-lesson note grew a concept trigger/sheet — the gate failed; body = %q", page)
	}
}

func TestShowNotFound(t *testing.T) {
	t.Parallel()
	srv := newServer(t, t.TempDir())

	code, _ := get(t, srv.Client(), srv.URL+"/notes/nope.md")
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// TestHome pins the landing page's observable contract: a direct 200 in the
// shared shell, the blocks this folder can fill, and the link to the vault
// README under them. The site markers are asserted by name rather than by their
// position in the document, so rearranging the desk cannot blame a correct
// page for violating a declaration-order accident.
//
// This folder holds one README and declares nothing, so no block on the page
// has anything to put in it and none of them ever will. They used to render as
// bordered boxes holding one sentence of apology each, which cost the top of
// every screen and pushed the reader's own writing most of the way down the
// first one. One line takes their place, and it says what the folder has.
func TestHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nREADME body sentinel.\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// Refuse redirects: a followed README redirect can produce the same body
	// while violating Home's direct-render contract.
	client := *srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()

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
		"search":                            `data-home-block="search"`,
		"link to the folder's introduction": `data-home-readme`,
		"topbar":                            `class="y-header"`,
		"command palette":                   `data-search`,
	} {
		if !strings.Contains(pageHTML, marker) {
			t.Errorf("GET / is missing the %s marker %q", name, marker)
		}
	}
	// The reader's own file is on the first screen of a folder that declares
	// nothing. Nothing here is a course, a map or a report, but the file is in
	// the folder, so the way in through the folders offers it.
	if folders := deskBlockMarkup(t, pageHTML, "folders"); !strings.Contains(folders, `href="/notes/README.md"`) {
		t.Errorf("the desk does not offer this folder's own file; folders block = %q", folders)
	}
	// This folder carries no contract, so nothing on the page reports a
	// capability that was never claimed. Before
	// the grant split this page opened with seven such notices and the reader's
	// own README started below the fold.
	for name, marker := range map[string]string{
		"recent block":      `data-home-block="recent"`,
		"capability faults": `data-home-block="faults"`,
		"fault row":         "data-home-fault",
	} {
		if strings.Contains(pageHTML, marker) {
			t.Errorf("the ungoverned desk renders the %s marker %q", name, marker)
		}
	}
	// The desk is where a reader chooses what to read, so there is nothing yet
	// being read for a rail to be. The reading surfaces behind it mount one.
	for _, absent := range []string{`class="y-rail-left"`, `id="_y-nav-rail"`} {
		if strings.Contains(pageHTML, absent) {
			t.Errorf("the desk mounts a left rail (%s); the rail is the thing being read", absent)
		}
	}
	// The introduction is a note with a page of its own. Home names it; it
	// does not reprint it, which used to be most of this screen.
	if strings.Contains(pageHTML, "README body sentinel.") {
		t.Error("Home reprints the folder's introduction instead of linking to it")
	}
}

// assertCauseStatedOnce pins how a capability fault reaches the reader: once,
// in the fault block below the seam. It used to be stated twice, in the rail
// every page carried and again in the content column, because the rail was the
// only region a reader who never opened the desk would meet. The rail is now
// the book being read and carries no vault-wide news, so a second copy would be
// a second account of one hole — which is the six-deep column of repeats this
// assertion was written against, in a smaller form.
func assertCauseStatedOnce(t *testing.T, page, cause string) {
	t.Helper()
	content := pageSection(page, `data-home-block="faults"`)
	if content == "" {
		t.Errorf("no region states the cause; page = %q", page)
	} else if got := strings.Count(content, cause); got != 1 {
		t.Errorf("the fault block states the cause %d times, want once: %q", got, content)
	}
	if got := strings.Count(page, cause); got != 1 {
		t.Errorf("the cause appears %d times on the page, want exactly once", got)
	}
	if got := strings.Count(page, "data-home-fault"); got != 1 {
		t.Errorf("fault rows = %d, want exactly 1", got)
	}
}

// pageSection returns the one <section> that starts with opener, so an
// assertion can say which region of the page states a thing rather than only
// that the page states it somewhere.
func pageSection(page, opener string) string {
	const closer = "</section>"
	start := strings.Index(page, opener)
	if start < 0 {
		return ""
	}
	rest := page[start:]
	end := strings.Index(rest, closer)
	if end < 0 {
		return rest
	}
	return rest[:end+len(closer)]
}

func TestHomeReportsAnUnreadableContractExactlyOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nREADME body sentinel.\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServerWithGovernance(t, root, nil, schema.Unreadable(
		errors.New("toml: line 42: expected a key separator"),
	))
	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	page := html.UnescapeString(body)

	assertCauseStatedOnce(t, page, "toml: line 42")
	// The ways in whose projections closed list nothing rather than asserting
	// an emptiness they cannot vouch for.
	for _, mode := range []string{"courses", "maps"} {
		block := homeSection(t, page, `data-home-block="`+mode+`"`)
		if strings.Contains(block, "data-desk-item") {
			t.Errorf("the %s block lists items under an unreadable contract; block = %q", mode, block)
		}
	}
	// Plain reading stays beside the fault: the folder tree, the search box
	// and the README link never depended on a contract.
	for _, want := range []string{`data-home-block="folders"`, `data-home-block="search"`, "data-home-readme"} {
		if !strings.Contains(page, want) {
			t.Errorf("the desk lost %q to a contract failure; reading never depends on one", want)
		}
	}
	// The shelf keeps its own reading: a contract that could not be read closes
	// the distribution and leaves the recent list, which never depended on one.
	shelfCode, shelfBody := get(t, srv.Client(), srv.URL+"/folders")
	if shelfCode != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", shelfCode)
	}
	shelf := html.UnescapeString(shelfBody)
	if !strings.Contains(shelf, `data-home-block="recent"`) {
		t.Error("the shelf lost the recent list to a contract failure; reading never depends on one")
	}
}

// The image-routing property this used to cover now lives where the body is
// rendered: Home links to the folder's introduction instead of reprinting it,
// so there is no image on Home to route. A note's own images are locked by
// TestHTMLResolvesImagesAgainstTheNotesOwnDirectory and by the note page.
// TestReadingRoutesKeepCapturedViewWhenCurrentSwaps is the coherence guard
// for a snapshot swap during a request. The provider atomically installs a
// different current View while returning the previously current one; every
// projection in the response must still come from that one captured value.
func TestReadingRoutesKeepCapturedViewWhenCurrentSwaps(t *testing.T) {
	t.Parallel()
	firstRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(firstRoot, "README.md"), []byte("# First\n\nSee [[Target]].\n"), 0o600); err != nil {
		t.Fatalf("write first README: %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	firstTarget := filepath.Join(firstRoot, "Concepts", "Target.md")
	if err := os.MkdirAll(filepath.Dir(firstTarget), 0o750); err != nil {
		t.Fatalf("mkdir first target parent: %v", err)
	}
	if err := os.WriteFile(firstTarget, []byte("# First target\n"), 0o600); err != nil {
		t.Fatalf("write first target: %v", err)
	}

	secondRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(secondRoot, "README.md"), []byte("# Second\n\nSee [[Target]].\n"), 0o600); err != nil {
		t.Fatalf("write second README: %v", err)
	}
	secondTarget := filepath.Join(secondRoot, "Writing", "Target.md")
	if err := os.MkdirAll(filepath.Dir(secondTarget), 0o750); err != nil {
		t.Fatalf("mkdir second target parent: %v", err)
	}
	if err := os.WriteFile(secondTarget, []byte("# Second target\n"), 0o600); err != nil {
		t.Fatalf("write second target: %v", err)
	}

	firstStore, firstSource := newSnapshotStore(t, firstRoot, log, nil, schema.Ungoverned())
	secondStore, _ := newSnapshotStore(t, secondRoot, log, nil, schema.Ungoverned())
	capture := func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) }

	for _, tt := range []struct {
		name string
		path string
		want string
	}{
		// The desk lists ways in rather than notes, so the needle for a
		// captured generation is a link the shelf's own recent list put there
		// rather than a wikilink resolved inside a body.
		{name: "shelf", path: "/folders", want: `href="/notes/Concepts/Target.md"`},
		{name: "note", path: "/notes/README.md", want: `<a href="/notes/Concepts/Target.md" class="wikilink">Target</a>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var current atomic.Pointer[snapshot.Generation]
			current.Store(firstStore.Current())
			calls := 0
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Continuation: noMark,
				Source:       firstSource,
				Authority:    capture,
				Snapshot: func() *snapshot.Generation {
					calls++
					return current.Swap(secondStore.Current())
				},
				Log: log,
			}).Register(mux)
			rr := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, rr.Code, http.StatusOK)
			}
			if !strings.Contains(rr.Body.String(), tt.want) {
				t.Errorf("GET %s did not resolve against the captured View; want %q in body", tt.path, tt.want)
			}
			if strings.Contains(rr.Body.String(), `/notes/Writing/Target.md`) {
				t.Errorf("GET %s mixed in the newly current View", tt.path)
			}
			if calls != 1 {
				t.Errorf("GET %s snapshot provider calls = %d, want 1", tt.path, calls)
			}
			if current.Load() != secondStore.Current() {
				t.Errorf("GET %s did not install the second View during the request", tt.path)
			}
		})
	}
}

// TestMissingPageKeepsCapturedGenerationWhenCurrentSwaps is the 404-path
// coherence lock. show (or showFile, or folder) captures generation A, decides
// the path is missing against A, and used to load the pointer again to build
// the not-found sidebar. A rebuild that lands between those two reads can put
// the asked note — or a different journal — into the rail of a page that says
// it is not here. The provider swaps current on the first read; every
// projection on the 404 must still come from the value that decided it.
func TestMissingPageKeepsCapturedGenerationWhenCurrentSwaps(t *testing.T) {
	t.Parallel()
	firstRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(firstRoot, "Diary"), 0o750); err != nil {
		t.Fatalf("mkdir first Diary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstRoot, "Diary", "Alpha.md"), []byte("# Alpha\n"), 0o600); err != nil {
		t.Fatalf("write first journal: %v", err)
	}

	secondRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(secondRoot, "Diary"), 0o750); err != nil {
		t.Fatalf("mkdir second Diary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondRoot, "Diary", "Beta.md"), []byte("# Beta\n"), 0o600); err != nil {
		t.Fatalf("write second journal: %v", err)
	}

	log := slog.New(slog.DiscardHandler)
	contract := loadHomeContract(t)
	firstStore, firstSource := newSnapshotStore(t, firstRoot, log, contract, contract.Governance())
	secondStore, _ := newSnapshotStore(t, secondRoot, log, contract, contract.Governance())
	capture := func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) }

	const (
		fromFirst  = `href="/notes/Diary/Alpha.md"`
		fromSecond = `href="/notes/Diary/Beta.md"`
	)
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "missing note", path: "/notes/Diary/Beta.md"},
		{name: "missing file", path: "/notes/ghost.txt"},
		{name: "missing folder", path: "/folders/Ghost"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var current atomic.Pointer[snapshot.Generation]
			current.Store(firstStore.Current())
			calls := 0
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Continuation: noMark,
				Source:       firstSource,
				Authority:    capture,
				Snapshot: func() *snapshot.Generation {
					calls++
					return current.Swap(secondStore.Current())
				},
				Log: log,
			}).Register(mux)
			rr := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want %d", tt.path, rr.Code, http.StatusNotFound)
			}
			body := rr.Body.String()
			if !strings.Contains(body, fromFirst) {
				t.Errorf("GET %s sidebar left the captured generation; want %q in body", tt.path, fromFirst)
			}
			if strings.Contains(body, fromSecond) {
				t.Errorf("GET %s mixed the newly current generation into the 404 sidebar", tt.path)
			}
			if calls != 1 {
				t.Errorf("GET %s snapshot provider calls = %d, want 1", tt.path, calls)
			}
			if current.Load() != secondStore.Current() {
				t.Errorf("GET %s did not install the second generation during the request", tt.path)
			}
		})
	}
}

func TestReadingFacesReadOneRequestSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nHome body.\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("plain source\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	capture := func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) }

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
			note.New(&note.Sources{
				Continuation: noMark,
				Source:       source,
				Authority:    capture,
				Snapshot: func() *snapshot.Generation {
					calls++
					return store.Current()
				},
				Log: log,
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

// TestEveryBlockIsBuiltFromTheSnapshot pins three blocks beyond their site
// markers. Recently changed is read from the folder index: the newest seven
// typed notes in mtime order, with no block of notes by status beside it.
// Study paths is read from the desk,
// where it states how large a course is rather than how much of it is done. The
// recent section is scoped by its own section marker, never by whichever block
// happens to follow it.
func TestEveryBlockIsBuiltFromTheSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n\nDashboard README sentinel.\n"), 0o600); err != nil {
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
		content := fmt.Sprintf("---\ntype: concept\n---\n# %s\n\nbody\n", name)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
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
	for _, name := range []string{"Open", "Sealed"} {
		content := fmt.Sprintf("---\ntype: lesson\n---\n# %s\n\nbody\n", name)
		full := filepath.Join(lessonDir, name+".md")
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
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
	pathBody := "---\ntype: study-path\n---\n# Test path\n\n## Part {sequence=primary}\n\n- [[Open]]\n- [[Sealed]]\n"
	pathFile := filepath.Join(mapDir, "path.md")
	if err := os.WriteFile(pathFile, []byte(pathBody), 0o600); err != nil {
		t.Fatalf("write study path: %v", err)
	}
	if err := os.Chtimes(pathFile, base.Add(-24*time.Hour), base.Add(-24*time.Hour)); err != nil {
		t.Fatalf("set study path mtime: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	deskCode, desk := get(t, srv.Client(), srv.URL+"/")
	if deskCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", deskCode)
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

	if strings.Contains(body, `data-home-block="lifecycle"`) {
		t.Error("the folder index still draws a block of notes by status")
	}
	paths := homeSection(t, desk, `data-home-block="courses"`)
	// The block says how big a course is, not how much of it is done. The
	// figure that used to sit here counted lessons awaiting a human's final
	// review, and publishing one moved it out of that status — so the number
	// fell as the work finished.
	if strings.Contains(paths, "已完成") {
		t.Errorf("the study-path block reports completion again; section = %q", paths)
	}
	for _, marker := range []string{"Test path", "2 課", "/courses/Maps/path.md"} {
		if !strings.Contains(paths, marker) {
			t.Errorf("Study paths block is missing %q", marker)
		}
	}
	if !strings.Contains(desk, "data-home-readme") {
		t.Error("the desk is missing the link to the folder's introduction")
	}
}

func TestTheFolderIndexUsesOneAuthorityViewAndClosesTheNextRequestAfterDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	notePath := filepath.Join(root, "Concepts", "golang", "A.md")
	if err := os.MkdirAll(filepath.Dir(notePath), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	const noteText = `---
type: concept
draft: true
created: 2026-07-16
based_on: source
---
# A

body
`
	if err := os.WriteFile(notePath, []byte(noteText), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	contractBytes, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, contractBytes, 0o600); err != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	log := slog.New(slog.DiscardHandler)
	captures := 0
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	capture := func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) }
	counted := func() schema.Authority {
		captures++
		return capture()
	}
	mux := http.NewServeMux()
	handler := note.New(&note.Sources{
		Continuation: noMark,
		Source:       source,
		Authority:    counted,
		Snapshot:     store.Current,
		Log:          log,
	})
	handler.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	section := html.UnescapeString(homeSection(t, body, `data-home-block="recent"`))
	if !strings.Contains(section, ">A<") {
		t.Errorf("captured-open recent block lost its instance projection: %q", section)
	}
	if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("change contract between requests: %v", writeErr)
	}

	code, body = get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("second GET /folders status = %d, want 200", code)
	}
	const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
	page := html.UnescapeString(body)
	assertCauseStatedOnce(t, page, diagnostic)
	// The recent list is plain reading — names and scan times — and stays,
	// but it stops citing the knowledge layer: that citation is the drifted
	// contract's own claim.
	recent := homeSection(t, page, `data-home-block="recent"`)
	if !strings.Contains(recent, ">A<") {
		t.Errorf("the recent list closed with the drifted authority; section = %q", recent)
	}
	if strings.Contains(recent, "知識層") {
		t.Errorf("the recent block cites a knowledge layer after authority drift; section = %q", recent)
	}
	// The row keeps the author's own draft flag: it needs no vocabulary, so
	// a drifted contract has nothing to withhold it on.
	if !strings.Contains(recent, wording.Draft.In(wording.ZhHant)) {
		t.Errorf("the note's draft mark is hidden after drift; section = %q", recent)
	}
	// The folder listing still offers the way down to the note: ordinary
	// browsing never depended on the artifact policy, and the level holding it
	// is one row away.
	if !strings.Contains(page, `href="/folders/Concepts"`) {
		t.Error("ordinary folder browsing closed with the artifact authority")
	}
	if captures != 2 {
		t.Errorf("two home requests captured the authority %d times, want exactly once per request", captures)
	}
}

func TestTheArtifactPolicyDegradesInstanceProjections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract *schema.Contract
		want     string
	}{
		{name: "invalid", contract: loadHomeContractWithArtifactSection(t, "templates = [\".\"]\n"), want: `invalid artifact policy: templates contains "."`},
		{name: "blank", contract: loadHomeContractWithArtifactSection(t, "templates = [\"\"]\n"), want: `invalid artifact policy: templates contains ""`},
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
			if err := os.WriteFile(filepath.Join(root, "Writing", "Draft.md"), []byte("---\ntype: writing\n---\n# Draft\n\nbody\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract)
			code, page := get(t, srv.Client(), srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}
			body := html.UnescapeString(page)
			// The instance-derived way in closes on one cause, so the cause is
			// stated once and the block lists nothing at all.
			if block := homeSection(t, body, `data-home-block="courses"`); strings.Contains(block, "data-desk-item") {
				t.Errorf("the desk still lists courses without a usable artifact policy; block = %q", block)
			}
			assertCauseStatedOnce(t, body, tt.want)
			shelfCode, shelfPage := get(t, srv.Client(), srv.URL+"/folders")
			if shelfCode != http.StatusOK {
				t.Fatalf("GET /folders status = %d, want 200", shelfCode)
			}
			shelf := html.UnescapeString(shelfPage)
			// The recent list is plain reading and stays, without the
			// knowledge-layer citation: the citation belongs to the same
			// contract whose artifact declaration was refused.
			recent := homeSection(t, shelf, `data-home-block="recent"`)
			if !strings.Contains(recent, "Draft") {
				t.Errorf("the recent list closed with the artifact policy; section = %q", recent)
			}
			if strings.Contains(recent, "知識層") {
				t.Errorf("the recent block cites a knowledge layer beside a refused declaration; section = %q", recent)
			}
			if !strings.Contains(body, `data-home-block="search"`) {
				t.Error("Home search block disappeared during artifact degradation")
			}
		})
	}
}

func TestANavigationFailureLeavesArtifactAggregatesOperational(t *testing.T) {
	t.Parallel()
	const invalidNavigation = "[types.study-path]\nrole = \"missing-role\"\n"
	const validArtifact = "templates = [\"System/templates\"]\n"
	contract := loadHomeContractWithSections(t, invalidNavigation, validArtifact)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
		t.Fatalf("mkdir Writing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Writing", "Draft.md"), []byte("---\ntype: writing\n---\n# Aggregate sentinel\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, contract)
	code, page := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	shelfCode, shelfPage := get(t, srv.Client(), srv.URL+"/folders")
	if shelfCode != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", shelfCode)
	}
	recent := homeSection(t, shelfPage, `data-home-block="recent"`)
	if !strings.Contains(recent, "Aggregate sentinel") || strings.Contains(recent, "data-home-recent-diagnostic") {
		t.Errorf("Recent degraded with navigation roles; section = %q", recent)
	}
	body := html.UnescapeString(page)
	if block := homeSection(t, body, `data-home-block="courses"`); strings.Contains(block, "data-desk-item") {
		t.Errorf("Study Paths listed without usable navigation roles; block = %q", block)
	}
	// A rejected navigation declaration closes only the study paths, and the
	// write authority knows nothing about it. The desk is the only surface that
	// states it, so a cause that never reached the desk would drop a way in
	// with no explanation anywhere.
	assertCauseStatedOnce(t, body,
		`invalid navigation roles: type "study-path" has the unknown role "missing-role"; Paths and Maps disabled`)
	if strings.Contains(body, "contract declares no artifact policy; instance projections disabled until it does") {
		t.Errorf("Home falsely reports an artifact failure: %q", body)
	}
}

func TestHomeStudyPathsReportsBothCapabilityFailures(t *testing.T) {
	t.Parallel()
	const invalidNavigation = "[types.study-path]\nrole = \"missing-role\"\n"
	const invalidArtifact = "templates = [\".\"]\n"
	contract := loadHomeContractWithSections(t, invalidNavigation, invalidArtifact)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServerWithContract(t, root, contract)
	code, page := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	body := html.UnescapeString(page)
	// Two declarations were rejected independently, and they close different
	// things: the artifact one closed the write authority, the navigation one
	// closed paths and maps. Both have to reach the reader, in the one region
	// that states a capability fault, and neither may be repeated there.
	content := pageSection(body, `data-home-block="faults"`)
	const artifactCause = `invalid artifact policy: templates contains "."`
	for _, cause := range []string{"missing-role", artifactCause} {
		if got := strings.Count(content, cause); got != 1 {
			t.Errorf("the fault block states %q %d times, want once: %q", cause, got, content)
		}
		if got := strings.Count(body, cause); got != 1 {
			t.Errorf("capability cause %q appears %d times on the page, want once", cause, got)
		}
	}
	if block := homeSection(t, body, `data-home-block="courses"`); strings.Contains(block, "data-desk-item") {
		t.Error("Study Paths listed under two rejected declarations")
	}
}

func TestAValidPolicyExcludesNonInstancesFromRecent(t *testing.T) {
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
		content := fmt.Sprintf("---\ntype: writing\n---\n# %s\n\nbody\n", title)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, page := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	recent := homeSection(t, page, `data-home-block="recent"`)
	if !strings.Contains(recent, "Governed draft sentinel") {
		t.Error("Recent is missing the governed instance")
	}
	if strings.Contains(recent, "LOUD TEMPLATE DRAFT SENTINEL") {
		t.Error("Recent includes a non-instance template")
	}
}

// TestHomeWithoutAnIntroductionStaysReadOnly pins first-use recovery: the desk
// still draws its four ways in and its search, an absent introduction costs
// only the link to it, and neither route creates the missing vault file. This
// vault declares a lifecycle and holds no notes yet, so every one of the four
// ways in is empty and says so.
func TestHomeWithoutAnIntroductionStaysReadOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// The contract declares a usable egress policy, because this test is about
	// a vault with nothing in it rather than a vault whose contract left an
	// input out — the second has something to say and would fail the emptiness
	// assertions below for a reason that has nothing to do with them.
	srv := newServerWithContract(t, root, contractWithPrivacySection(t, "[privacy]\nnever_egress_dirs = []\n"))

	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / without README status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(body, `data-home-block="search"`) {
		t.Error(`GET / without README is missing the search block`)
	}
	// The four ways in stay on the page even when every one of them is empty.
	// They are the only route to the mode pages — no header or rail carries
	// one — so a vault with nothing in it yet would otherwise strand a reader
	// on a page they cannot leave except by searching.
	for _, present := range []string{
		`data-home-block="courses"`,
		`data-home-block="maps"`,
		`data-home-block="reports"`,
		`data-home-block="folders"`,
	} {
		if !strings.Contains(body, present) {
			t.Errorf("GET / without README dropped %q, leaving no way to that mode's page", present)
		}
	}
	for _, absent := range []string{
		`data-home-block="recent"`,
	} {
		if strings.Contains(body, absent) {
			t.Errorf("GET / without README still renders %q, which belongs to the folder mode", absent)
		}
	}

	noteCode, _ := get(t, srv.Client(), srv.URL+"/notes/README.md")
	if noteCode != http.StatusNotFound {
		t.Errorf("GET /notes/README.md without README status = %d, want %d", noteCode, http.StatusNotFound)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Errorf("README.md after recovery requests: os.Stat error = %v, want not-exist", err)
	}
}

// TestTheDeskOffersTheFilesAFolderHoldsWhenNoneIsANote covers a folder nothing
// classifies: three files, no markdown, no contract. Every file is in the
// folder, so the way in through the folders lists them. The measure counts
// notes only, so the block says 0 篇 above the three hrefs the desk can open.
func TestTheDeskOffersTheFilesAFolderHoldsWhenNoneIsANote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for name, content := range map[string]string{
		"reading.html": "<h1>Saved</h1>\n",
		"older.txt":    "older body\n",
	} {
		full := filepath.Join(root, name)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(full, base, base); err != nil {
			t.Fatalf("set %s mtime: %v", name, err)
		}
	}
	newest := filepath.Join(root, "todo.txt")
	if err := os.WriteFile(newest, []byte("buy coffee\n"), 0o600); err != nil {
		t.Fatalf("write todo.txt: %v", err)
	}
	changed := base.Add(48 * time.Hour)
	if err := os.Chtimes(newest, changed, changed); err != nil {
		t.Fatalf("set todo.txt mtime: %v", err)
	}

	srv := newServer(t, root)
	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	folders := deskBlockMarkup(t, body, "folders")
	for _, want := range []string{
		"0 篇",
		`href="/notes/todo.txt"`,
		`href="/notes/older.txt"`,
		`href="/notes/reading.html"`,
	} {
		if !strings.Contains(folders, want) {
			t.Errorf("the folders block is missing %q; block = %q", want, folders)
		}
	}
	if strings.Contains(folders, "3 篇") {
		t.Errorf("the folders block counted non-notes as 篇; block = %q", folders)
	}
	if code, _ = get(t, srv.Client(), srv.URL+"/notes/todo.txt"); code != http.StatusOK {
		t.Errorf("GET /notes/todo.txt status = %d, want %d: the desk offers a row that does not answer", code, http.StatusOK)
	}
}

// TestTheRailHeadingOfARootNoteAnswers covers the one way out the rail offers a
// reader who is in the vault root. The root is not a folder under the tree —
// the route for one level refuses an empty path — so a path that pointed there
// sent the reader to a page that does not exist, and since the rail stopped
// carrying the whole vault that path is the only way out it has.
func TestTheRailHeadingOfARootNoteAnswers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Root\n"), 0o600); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	srv := newServer(t, root)
	code, body := get(t, srv.Client(), srv.URL+"/notes/README.md")
	if code != http.StatusOK {
		t.Fatalf("GET /notes/README.md status = %d, want 200", code)
	}
	const opener = `<a class="y-where__path" href="`
	_, rest, found := strings.Cut(body, opener)
	if !found {
		t.Fatal("the rail of a root note carries no path for the folder it is in")
	}
	href, _, closed := strings.Cut(rest, `"`)
	if !closed {
		t.Fatal("the rail path's address is never closed")
	}
	if code, _ = get(t, srv.Client(), srv.URL+href); code != http.StatusOK {
		t.Errorf("the rail path of a root note leads to %s, which answers %d, want %d", href, code, http.StatusOK)
	}
}

// TestTheDeskDrawsItsWaysInForAFolderHoldingNothing covers the first minute of
// a vault: a folder with nothing in it at all. All four ways in are drawn, each
// stating its own measure and its own sentence, and none of them offers a row.
// An empty folder is where a reader starts, not a failure to report, and the
// four sentences are the whole of what the desk has to say about it.
func TestTheDeskDrawsItsWaysInForAFolderHoldingNothing(t *testing.T) {
	t.Parallel()

	srv := newServer(t, t.TempDir())
	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	for _, mode := range []string{"courses", "maps", "reports", "folders"} {
		if !strings.Contains(body, `data-home-block="`+mode+`"`) {
			t.Errorf("the desk lost its %s block over a folder holding nothing", mode)
		}
	}
	if strings.Contains(body, "data-desk-item") {
		t.Error("the desk offers a way on for a folder holding nothing")
	}
}

func homeSection(t *testing.T, body, marker string) string {
	t.Helper()
	markerAt := strings.Index(body, marker)
	if markerAt < 0 {
		t.Fatalf("page body is missing section marker %q", marker)
	}
	openAt := strings.LastIndex(body[:markerAt], "<section")
	if openAt < 0 {
		t.Fatalf("marker %q is not inside a section", marker)
	}
	closeAt := strings.Index(body[markerAt:], "</section>")
	if closeAt < 0 {
		t.Fatalf("section %q has no closing tag", marker)
	}
	return body[openAt : markerAt+closeAt+len("</section>")]
}

// loadContractRequiringFrontmatter is the testdata contract with its scan
// declaration turned against a note that carries no frontmatter block.
func loadContractRequiringFrontmatter(t *testing.T) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	const legal = "frontmatter_optional = true"
	modified := strings.Replace(string(data), legal, "frontmatter_optional = false", 1)
	if modified == string(data) {
		t.Fatal("frontmatter declaration replacement did not apply")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(modified), 0o600); writeErr != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatalf("write modified contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	if !contract.RequiresFrontmatter() {
		t.Fatal("the modified contract does not require frontmatter")
	}
	return contract
}

// TestAFenceThatNeverClosesIsSaidOnThePage drives the shape the lost dash
// makes through the real handler. The closing line of the block is gone, so no
// reader finds a block, and the page used to answer that the note has no
// frontmatter, which is legal. It says instead that the fence never closes, in
// either language, on a note the contract judges and on one outside the layer it
// judges, under a contract that allows a note without a block and one that
// does not.
//
// The dedicated fence explanation already names this fault, so the schema
// finding is not repeated as a missing or unreadable frontmatter block.
func TestAFenceThatNeverClosesIsSaidOnThePage(t *testing.T) {
	t.Parallel()

	const content = "---\ntitle: Unclosed\ntype: writing\ndraft: true\n--\n\n# Body\n\nbody\n"
	tests := []struct {
		name     string
		rel      string
		contract func(t *testing.T) *schema.Contract
	}{
		{name: "a note the contract judges, contract allows no block", rel: "Writing/Unclosed.md", contract: loadContract},
		{name: "a note the contract judges, contract wants a block", rel: "Writing/Unclosed.md", contract: loadContractRequiringFrontmatter},
		{name: "a note outside the layer the contract judges", rel: "Drills/Unclosed.md", contract: loadContract},
	}
	for _, tt := range tests {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(tt.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				full := filepath.Join(root, filepath.FromSlash(tt.rel))
				if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
					t.Fatalf("write: %v", err)
				}
				srv := newServerWithContract(t, root, tt.contract(t))

				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/notes/"+tt.rel, http.NoBody)
				if err != nil {
					t.Fatalf("new request: %v", err)
				}
				req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
				resp, err := srv.Client().Do(req)
				if err != nil {
					t.Fatalf("GET note: %v", err)
				}
				defer func() {
					if closeErr := resp.Body.Close(); closeErr != nil {
						t.Errorf("close response body: %v", closeErr)
					}
				}()
				raw, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				body := string(raw)
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", resp.StatusCode)
				}

				if want := wording.FrontmatterNeverCloses.In(lang); !strings.Contains(body, want) {
					t.Errorf("the page does not say %q", want)
				}
				if strings.Contains(body, "schema.frontmatter") {
					t.Errorf("the page repeats the schema rule beside the dedicated notice that names the unclosed fence")
				}
			})
		}
	}
}

// markObsidianVault gives root the .obsidian folder that makes it a vault
// Obsidian has opened, the only evidence the page takes that a link into
// Obsidian can be answered.
func markObsidianVault(t *testing.T, root string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(root, ".obsidian"), 0o750); err != nil {
		t.Fatalf("mkdir .obsidian: %v", err)
	}
}

// TestNewPanicsOnAMissingDependency covers every guard New has, one row per
// guard. A wiring bug has to fail at construction with the field named, not
// three calls deep inside the first request, so each row asserts the exact
// diagnostic rather than merely that something panicked. A guard added to New
// without a row here is a guard nothing has watched fail.
func TestNewPanicsOnAMissingDependency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// nilDeps passes a nil *Sources instead of clearing one field.
		nilDeps bool
		clear   func(*note.Sources)
		want    string
	}{
		{
			name:    "no dependencies at all",
			nilDeps: true,
			want:    "note: New requires a non-nil Sources",
		},
		{
			name:  "source",
			clear: func(d *note.Sources) { d.Source = nil },
			want:  "note: New requires a non-nil Source",
		},
		{
			name:  "authority",
			clear: func(d *note.Sources) { d.Authority = nil },
			want:  "note: New requires a non-nil Authority",
		},
		{
			name:  "snapshot provider",
			clear: func(d *note.Sources) { d.Snapshot = nil },
			want:  "note: New requires a non-nil Snapshot provider",
		},
		{
			name:  "continuation provider",
			clear: func(d *note.Sources) { d.Continuation = nil },
			want:  "note: New requires a non-nil Continuation provider",
		},
		{
			name:  "log",
			clear: func(d *note.Sources) { d.Log = nil },
			want:  "note: New requires a non-nil Log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				got := recover()
				if got != tt.want {
					t.Fatalf("New() panic = %v, want %q", got, tt.want)
				}
			}()
			if tt.nilDeps {
				note.New(nil)
				return
			}
			root := t.TempDir()
			log := slog.New(slog.DiscardHandler)
			store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
			capture := func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) }
			deps := note.Sources{
				Continuation: noMark,
				Source:       source,
				Authority:    capture,
				Snapshot:     store.Current,
				Log:          log,
			}
			tt.clear(&deps)
			note.New(&deps)
		})
	}
}

func TestNewCopiesItsSources(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "constructor ownership sentinel\n"
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte(body), 0o600); err != nil {
		t.Fatalf("write raw fixture: %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	capture := func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) }
	deps := note.Sources{
		Continuation: noMark,
		Source:       source,
		Authority:    capture,
		Snapshot:     store.Current,
		Log:          log,
	}
	handler := note.New(&deps)
	deps.Source = openReadingVault(t, t.TempDir())

	mux := http.NewServeMux()
	handler.Register(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/raw/plain.txt", http.NoBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /raw/plain.txt status = %d, want 200 after caller rewired its copy", recorder.Code)
	}
	if got := recorder.Body.String(); got != body {
		t.Errorf("GET /raw/plain.txt body = %q, want %q", got, body)
	}
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
	lessonMD := "---\ntype: lesson\ndraft: true\n---\n# Slices\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	syllabus := "---\ntype: study-path\n---\n# Go path\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字 {sequence=primary}\n\n- [[Slices]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(syllabus), 0o600); err != nil {
		t.Fatalf("write syllabus: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, body := get(t, srv.Client(), srv.URL+"/notes/Maps/Go path.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`class="y-rail-left"`,
		`data-reading-rail="book"`,
		"Go path",
		"Data",
		`href="/notes/Writing/lessons/golang/Slices.md"`,
		`<span class="ui-navitem__count">` + wording.Draft.In(wording.ZhHant) + `</span>`,
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
	mapNote := "---\ntype: topic-map\n---\n# Reading map\n\n## Themes\n\n- [[Ghost Essay]]\n"
	if err := os.WriteFile(filepath.Join(mapDir, "Reading map.md"), []byte(mapNote), 0o600); err != nil {
		t.Fatalf("write map: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+"/notes/Maps/Reading map.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`data-reading-rail="folder"`,
		// The reason reaches a pointer through the title and everyone else
		// through text carried out of sight, so both are named here: a
		// state told by cursor alone is told to no one on a phone.
		`class="wikilink-broken" title="還沒有「Ghost Essay」這篇筆記">Ghost Essay`,
		`<span class="y-offscreen">（還沒有「Ghost Essay」這篇筆記）</span>`,
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

// TestFilePageAndSearchAgreeOnWhatIsText is the drift lock between two faces
// that answer the same question. A file's page decides whether to show its
// characters; the text index decides whether to read them. One rule holds
// across both — if yomihon shows it to you as text, you can find it — and a
// reader who is shown a file and then cannot search it learns that the tool is
// unreliable rather than that the file is unusual.
//
// Both sides run for real here: the page comes from an HTTP request and the
// answer comes from the generation's own index, so an agreement asserted over
// two re-implementations of the rule is not what this proves.
func TestFilePageAndSearchAgreeOnWhatIsText(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := []struct {
		rel      string
		body     string
		term     string
		wantText bool
	}{
		{rel: "todo.txt", body: "renew the flamingoterm passport", term: "flamingoterm", wantText: true},
		{rel: "page.html", body: "<p>kingfisherterm</p>", term: "kingfisherterm", wantText: true},
		{rel: "Makefile", body: "build:\n\tgo build # heronterm\n", term: "heronterm", wantText: true},
		{rel: "drawing.svg", body: `<svg xmlns="http://www.w3.org/2000/svg"><text>pelicanterm</text></svg>`, term: "pelicanterm"},
		{rel: "blob.bin", body: "storkterm\x00opaque", term: "storkterm"},
		{
			rel:  "huge.txt",
			body: strings.Repeat("egretterm ", (render.MaxSourceBytes/10)+1),
			term: "egretterm",
		},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f.rel), []byte(f.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", f.rel, err)
		}
	}
	store, source := newSnapshotStore(t, root, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source:       source,
		Authority:    func() schema.Authority { return schema.Authority{} },
		Snapshot:     store.Current,
		Continuation: noMark,
		Log:          slog.New(slog.DiscardHandler),
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, f := range files {
		t.Run(f.rel, func(t *testing.T) {
			t.Parallel()
			code, page := get(t, srv.Client(), srv.URL+"/notes/"+f.rel)
			if code != http.StatusOK {
				t.Fatalf("GET /notes/%s status = %d, want 200", f.rel, code)
			}
			shown := strings.Contains(page, `class="y-prose y-source"`)
			if shown != f.wantText {
				t.Errorf("GET /notes/%s renders its characters = %v, want %v", f.rel, shown, f.wantText)
			}
			answer, err := store.Current().Search().Search(lexical.Parse(f.term), -1)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", f.term, err)
			}
			found := len(answer.Results) == 1 && answer.Results[0].RelPath == f.rel
			if found != shown {
				t.Errorf("/notes/%s shows its characters = %v but search finds it = %v; one face contradicts the other",
					f.rel, shown, found)
			}
		})
	}
}

// TestHomeDoesNotSpendItsScreenTalkingAboutItself locks the reason rather than
// the layout. A test asserting "this block is absent" pins an arrangement and
// has to be rewritten the next time something moves; what matters is the share
// of the first screen the tool spends on itself.
//
// Measured before this changed: on a governed vault, the inlined introduction
// was 72% of everything Home said, and its content is doctrine addressed to
// agents. On a folder without one, 46% of a 191-character screen was an
// instruction to go author a file in another program — a reading tool opening
// with homework.
func TestHomeDoesNotSpendItsScreenTalkingAboutItself(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{
			name: "a folder with an introduction",
			setup: func(t *testing.T, root string) {
				t.Helper()
				long := "# Vault\n\n" + strings.Repeat("這是寫給代理程式看的 vault 條文。", 120) + "\n"
				if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(long), 0o600); err != nil {
					t.Fatalf("write README: %v", err)
				}
			},
		},
		{
			name:  "a folder without one",
			setup: func(t *testing.T, _ string) { t.Helper() },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("# Note\n\nreader body\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			tt.setup(t, root)
			srv := newServer(t, root)
			code, body := get(t, srv.Client(), srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}
			for _, forbidden := range []string{
				"請使用外部編輯器",
				"yomihon 不會建立或修改這個檔案",
			} {
				if strings.Contains(body, forbidden) {
					t.Errorf("the first screen instructs the reader to go and author a file: %q", forbidden)
				}
			}
			if strings.Contains(body, "這是寫給代理程式看的 vault 條文") {
				t.Error("the introduction is reprinted on Home instead of linked")
			}
		})
	}
}

// TestTheFuriganaControlFollowsTheReadings holds the header's switch to the
// pages that carry readings, and the way back to the pages that do not. The
// preference is one cookie for the whole site, so a reader who switches
// readings off on a Japanese note has them off everywhere; the switch then
// lives on the settings page as well, which every page leads to, and that is
// what lets the header draw it only where it does something.
//
// The three pages below differ in whether they contain readings at all, which is
// what stops this passing for the wrong reason: it would be satisfied by three
// pages that all happened to carry Japanese.
func TestTheFuriganaControlFollowsTheReadings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain.md"),
		[]byte("# Plain\n\nordinary prose, no readings.\n"), 0o600); err != nil {
		t.Fatalf("write plain note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "japanese.md"),
		[]byte("# Japanese\n\n<ruby>今日<rt>きょう</rt></ruby>は晴れ。\n"), 0o600); err != nil {
		t.Fatalf("write japanese note: %v", err)
	}
	srv := newServer(t, root)

	for _, tt := range []struct {
		name        string
		path        string
		hasReadings bool
	}{
		{name: "a note carrying readings", path: "/notes/japanese.md", hasReadings: true},
		{name: "a note carrying none", path: "/notes/plain.md", hasReadings: false},
		{name: "home", path: "/", hasReadings: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.Client(), srv.URL+tt.path)
			if code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tt.path, code)
			}
			if strings.Contains(body, "<ruby") != tt.hasReadings {
				t.Fatalf("GET %s: the fixture does not match what the test assumes", tt.path)
			}
			if got := strings.Contains(body, "data-ruby-toggle"); got != tt.hasReadings {
				t.Errorf("GET %s draws the furigana switch = %v, want %v", tt.path, got, tt.hasReadings)
			}
			if !strings.Contains(body, `href="/preferences`) {
				t.Errorf("GET %s has no way to the settings page, where a reader who switched readings off switches them back on", tt.path)
			}
		})
	}
}

// TestAnOverCapNotePageIsTheFilePage is the page-side of the bound. A note
// past MaxSourceBytes is the same file-information page an over-cap non-note
// already got: size named, raw-bytes link, body not carried. It is not a 404
// that promises the body after a reload. The under-cap note still reads. The
// generation-side lock is TestAnOverCapNoteIsNotRetained in internal/snapshot.
func TestAnOverCapNotePageIsTheFilePage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	const needle = "rarespelunker"
	small := "# Small\n\n" + needle + " sits here.\n"
	if err := os.WriteFile(filepath.Join(root, "small.md"), []byte(small), 0o600); err != nil {
		t.Fatalf("write small note: %v", err)
	}
	huge := "# Huge\n\n" + needle + " sits here too.\n" + strings.Repeat("padding padding padding\n", 60000)
	if len(huge) <= render.MaxSourceBytes {
		t.Fatalf("the oversize fixture is %d bytes, which is under the cap; this would prove nothing", len(huge))
	}
	if err := os.WriteFile(filepath.Join(root, "huge.md"), []byte(huge), 0o600); err != nil {
		t.Fatalf("write huge note: %v", err)
	}
	srv := newServer(t, root)

	code, page := get(t, srv.Client(), srv.URL+"/notes/huge.md")
	assertOverCapNoteFilePage(t, code, page, "huge.md", huge)

	code, smallPage := get(t, srv.Client(), srv.URL+"/notes/small.md")
	if code != http.StatusOK {
		t.Fatalf("GET the under-cap note = %d, want 200", code)
	}
	if !strings.Contains(smallPage, "sits here") {
		t.Error("the under-cap note did not render its own body")
	}
}

// assertOverCapNoteFilePage is what an over-cap note and an over-cap non-note
// share: 200, the information page, the size, the raw-bytes link, and no body.
func assertOverCapNoteFilePage(t *testing.T, code int, page, rel, body string) {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("GET /notes/%s = %d, want 200 — the file page, not a withheld read", rel, code)
	}
	if !strings.Contains(page, wording.FileInfoLabel.In(wording.ZhHant)) {
		t.Error("the over-cap note's page is not the file-information page")
	}
	if !strings.Contains(page, wording.FileSize.In(wording.ZhHant)) {
		t.Error("the over-cap note's page does not name the size")
	}
	digits := strconv.Itoa(len(body))
	var grouped strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}
	if !strings.Contains(page, grouped.String()) {
		t.Errorf("the over-cap note's page does not name the byte size %s", grouped.String())
	}
	if !strings.Contains(page, wording.OpenRawBytes.In(wording.ZhHant)) {
		t.Error("the over-cap note's page offers no raw-bytes link")
	}
	if !strings.Contains(page, "/raw/"+rel) {
		t.Errorf("the over-cap note's page has no raw-bytes href for %s", rel)
	}
	if strings.Contains(page, "sits here too") {
		t.Error("the over-cap note's page retained its body")
	}
	if strings.Contains(page, wording.NotReadableLede.In(wording.ZhHant)) {
		t.Error("the over-cap note's page is the unreadable page, which tells the reader to wait for a reload that never shows it")
	}
	if strings.Contains(page, wording.NothingHere.In(wording.ZhHant)) {
		t.Error("the over-cap note's page is the plain not-found page")
	}
}

// TestShowOffersAnObsidianDoor locks the reading page's hand-off to the
// editor: the metarow beside the note's path carries an obsidian://open link
// whose query names the note's absolute path, segment-escaped. Every repair
// yomihon reports is a hand edit, so the page offers the one-click way to
// where editing happens.
func TestShowOffersAnObsidianDoor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessonMD := "---\ntype: lesson\ncreated: 2026-06-01\n---\n# L01\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))
	code, plain := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if strings.Contains(plain, "obsidian://") {
		t.Error("a folder without .obsidian was offered a link into Obsidian")
	}
	markObsidianVault(t, root)
	code, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(page, wording.OpenInObsidian.In(wording.ZhHant)) {
		t.Error("page offers no Obsidian link")
	}
	if !strings.Contains(page, `href="obsidian://open?path=`) {
		t.Error("page carries no obsidian://open href")
	}
	if !strings.Contains(page, "Writing/lessons/japanese/L01.md") {
		t.Error("the Obsidian href does not name the note")
	}
}

// TestTheFactsDateIsTheAuthorsCheckedDayAndNeverTheFilesTime pins the one date
// the reading page shows: the day the author wrote as the one they checked the
// note, labelled as theirs. The file's own change time, which a checkout stamps
// on every file at once, and the other dates the frontmatter holds do not stand
// in for it, and a note that wrote none shows no date at all.
func TestTheFactsDateIsTheAuthorsCheckedDayAndNeverTheFilesTime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		stamp := time.Date(2026, time.June, 3, 8, 30, 0, 0, time.UTC)
		if err := os.Chtimes(full, stamp, stamp); err != nil {
			t.Fatalf("set mtime: %v", err)
		}
	}
	write("Writing/Checked.md", "---\ntype: writing\ncreated: 2026-07-01\nupdated: 2026-07-12\nverified_on: 2026-07-20\n---\n# Checked\n\nbody\n")
	write("Writing/Quoted.md", "---\ntype: writing\nverified_on: \"2026-08-02\"\n---\n# Quoted\n\nbody\n")
	write("Writing/Silent.md", "---\ntype: writing\ncreated: 2026-07-01\n---\n# Silent\n\nbody\n")
	srv := newServer(t, root)

	for _, tc := range []struct {
		lang  wording.Lang
		label string
	}{
		{wording.ZhHant, `<dt lang="zh-Hant">作者寫的查證日</dt>`},
		{wording.En, `<dt lang="en">Checked on, as the author wrote it</dt>`},
	} {
		page := fetchAs(t, srv, "/notes/Writing/Checked.md", tc.lang)
		if want := tc.label + `<dd><time datetime="2026-07-20">2026-07-20</time></dd>`; !strings.Contains(page, want) {
			t.Errorf("%s: the facts do not carry the author's day as %q", tc.lang.Tag(), want)
		}
		for _, other := range []string{"2026-06-03", "2026-07-01", "2026-07-12", "檔案變更於", "File changed"} {
			if strings.Contains(page, other) {
				t.Errorf("%s: %q reached the page, though only the author's checked day is the note's date", tc.lang.Tag(), other)
			}
		}
		if quoted := fetchAs(t, srv, "/notes/Writing/Quoted.md", tc.lang); !strings.Contains(quoted, `<time datetime="2026-08-02">2026-08-02</time>`) {
			t.Errorf("%s: a day written as quoted text is not read as a day", tc.lang.Tag())
		}
		if silent := fetchAs(t, srv, "/notes/Writing/Silent.md", tc.lang); strings.Contains(silent, "<time") || strings.Contains(silent, "2026-06-03") {
			t.Errorf("%s: a note that wrote no checked day is still dated:\n%s", tc.lang.Tag(), silent)
		}
	}
}

// TestPrerequisitesAreLinksToTheNotesTheyNameAndTextWhereTheyNameNone holds the
// reading of prerequisites the way based_on is read: a value that places exactly
// one note is that note's name and a link to it, and anything else stays the
// author's own words with no address.
func TestPrerequisitesAreLinksToTheNotesTheyNameAndTextWhereTheyNameNone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, content := range map[string]string{
		"Writing/Needs.md":  "---\ntype: writing\nprerequisites:\n  - \"[[Writing/Basics]]\"\n  - \"[[Writing/Basics|again]]\"\n  - \"not written yet\"\n---\n# Needs\n\nbody\n",
		"Writing/Basics.md": "---\ntype: writing\nlang: ja\n---\n# Basics\n\nbody\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	srv := newServer(t, root)

	page := fetchAs(t, srv, "/notes/Writing/Needs.md", wording.ZhHant)
	if want := `<dt lang="zh-Hant">先備知識</dt><dd><ul class="y-notefacts__links" role="list"><li><a href="/notes/Writing/Basics.md">Basics</a></li><li><span>not written yet</span></li></ul></dd>`; !strings.Contains(page, want) {
		t.Errorf("the prerequisites are not one link to the note they name, once, and the unwritten one as text; want %q", want)
	}
}

// fetchAs reads one page in the interface language a reader chose.
func fetchAs(t *testing.T, srv *httptest.Server, path string, lang wording.Lang) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(raw)
}

// TestTheRecentListStatesItsKnowledgeScope holds the folder index's recent
// list to its own scope, stated where the reader reads it: the declared
// knowledge folders, so the list does not read as the whole folder.
func TestTheRecentListStatesItsKnowledgeScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for rel, at := range map[string]time.Time{
		"Writing/Inside.md": base.Add(24 * time.Hour),
		"Attic/Outside.md":  base,
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		title := strings.TrimSuffix(filepath.Base(rel), ".md")
		content := "---\ntype: writing\n---\n# " + title + "\n\nbody\n"
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, at, at); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}

	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, "Inside") {
		t.Fatalf("the recent block does not list the knowledge-layer note; section = %q", recent)
	}
	if strings.Contains(recent, "Outside") {
		t.Errorf("the recent block lists a note outside the knowledge layer; section = %q", recent)
	}
	if !strings.Contains(recent, "知識層資料夾中") {
		t.Errorf("the recent block does not state its scope; section = %q", recent)
	}
	if strings.Contains(recent, wording.FolderTiedTitle.In(wording.ZhHant)) {
		t.Errorf("distinct times still read as a tie; section = %q", recent)
	}
}

// TestASingleNoteClaimsNoTimestampTie holds the tie notice to vaults where
// it is true. With one note there is one recorded time and nothing for it to
// equal, yet the page said "these files carry identical timestamps" — a
// sentence about files that do not exist. One note is trivially the most
// recently changed thing, so the ordinary heading stands and the tie sentence
// stays off the page.
func TestASingleNoteClaimsNoTimestampTie(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "---\ntype: writing\n---\n# Only\n\nbody\n"
	if err := os.WriteFile(filepath.Join(root, "Only.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServer(t, root)
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if strings.Contains(recent, "時間戳相同") || strings.Contains(recent, wording.FolderTiedTitle.In(wording.ZhHant)) {
		t.Errorf("a single-note vault claims a timestamp tie; section = %q", recent)
	}
	if !strings.Contains(recent, `<h2 id="folders-recent-title">`+wording.FolderRecentTitle.In(wording.ZhHant)+`</h2>`) {
		t.Errorf("a single-note vault does not carry the ordinary recency heading; section = %q", recent)
	}
}

// TestTiedTimesStillSaySoWithinTheirScope pins the tie notice itself: on
// a knowledge-scoped vault whose files all carry one moment, the block still
// says the times separate nothing — and now names the scope it lists, since
// the tie changes nothing about which files these are.
func TestTiedTimesStillSaySoWithinTheirScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stamp := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for _, rel := range []string{"Writing/A.md", "Writing/B.md"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		title := strings.TrimSuffix(filepath.Base(rel), ".md")
		content := "---\ntype: writing\n---\n# " + title + "\n\nbody\n"
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, stamp, stamp); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, wording.FolderTiedTitle.In(wording.ZhHant)) ||
		!strings.Contains(recent, wording.FolderTiedLedeScoped.In(wording.ZhHant)) {
		t.Errorf("tied times in a scoped list do not name the tie and its scope; section = %q", recent)
	}
}

// TestAnUnscopedVaultClaimsNoKnowledgeLayer holds the scope phrase to
// vaults that declared one. A folder without a contract lists everything, and
// a lede naming a knowledge layer there would invent a rule its owner never
// wrote. The heading stands alone: a second sentence that restates it is not
// written.
func TestAnUnscopedVaultClaimsNoKnowledgeLayer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for rel, at := range map[string]time.Time{"A.md": base, "B.md": base.Add(time.Hour)} {
		content := "---\ntype: writing\n---\n# " + strings.TrimSuffix(rel, ".md") + "\n\nbody\n"
		full := filepath.Join(root, rel)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, at, at); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServer(t, root)
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, wording.FolderRecentTitle.In(wording.ZhHant)) {
		t.Errorf("an unscoped vault lost the recency heading; section = %q", recent)
	}
	if strings.Contains(recent, "知識層") {
		t.Errorf("an unscoped vault's recent block names a knowledge layer; section = %q", recent)
	}
	if strings.Contains(recent, "最近改動過的筆記") {
		t.Errorf("an unscoped vault still paraphrases its own heading; section = %q", recent)
	}
}

// TestHomeStudyPathCardSeparatesTheTwoZeroes tells apart the two courses that
// plan nothing. A path whose note lists lessons the grammar could not read is
// marked — the zero is a fault to repair, and the card used to look exactly
// like an empty course. A path whose note simply declares no course stays
// unmarked: its zero is the author's answer, and a flag on it would send them
// hunting for a fault that does not exist. A course that reads fine keeps its
// count and no mark.
func TestHomeStudyPathCardSeparatesTheTwoZeroes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"Maps/broken.md":  "---\ntype: study-path\n---\n# Broken course\n\n## Part\n\n- [[Open]]\n",
		"Maps/prose.md":   "---\ntype: study-path\n---\n# Prose only\n\nNothing here lists a lesson.\n",
		"Maps/working.md": "---\ntype: study-path\n---\n# Working course\n\n## Part {sequence=primary}\n\n- [[Open]]\n",
		"Writing/Open.md": "---\ntype: lesson\n---\n# Open\n\nbody\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	paths := homeSection(t, body, `data-home-block="courses"`)
	card := func(title string) string {
		t.Helper()
		at := strings.Index(paths, title)
		if at < 0 {
			t.Fatalf("no card for %q; section = %q", title, paths)
		}
		end := strings.Index(paths[at:], "</a>")
		if end < 0 {
			t.Fatalf("unterminated card for %q", title)
		}
		return paths[at : at+end]
	}

	broken := card("Broken course")
	if !strings.Contains(broken, "0 課") {
		t.Errorf("the unreadable course does not show its zero; card = %q", broken)
	}
	if !strings.Contains(broken, "未讀到課程結構") {
		t.Errorf("the unreadable course's zero is not marked; card = %q", broken)
	}

	prose := card("Prose only")
	if !strings.Contains(prose, "0 課") {
		t.Errorf("the courseless note does not show its zero; card = %q", prose)
	}
	if strings.Contains(prose, "未讀到課程結構") {
		t.Errorf("a note that declares no course is marked as unreadable; card = %q", prose)
	}

	working := card("Working course")
	if !strings.Contains(working, "1 課") {
		t.Errorf("the working course lost its count; card = %q", working)
	}
	if strings.Contains(working, "未讀到課程結構") {
		t.Errorf("a working course is marked as unreadable; card = %q", working)
	}
}

// TestAnUnscopedTieKeepsThePlainNotice completes the lede's four states: a
// folder that declared no knowledge layer and whose files carry one moment
// gets the tie notice exactly as before, with no scope phrase — every clause
// of the sentence true of the page it sits on.
func TestAnUnscopedTieKeepsThePlainNotice(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stamp := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for _, rel := range []string{"A.md", "B.md"} {
		content := "---\ntype: writing\n---\n# " + strings.TrimSuffix(rel, ".md") + "\n\nbody\n"
		full := filepath.Join(root, rel)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, stamp, stamp); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServer(t, root)
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, "<p>"+wording.FolderTiedLede.In(wording.ZhHant)) {
		t.Errorf("an unscoped tie lost its consequence notice; section = %q", recent)
	}
	if strings.Contains(recent, "知識層") {
		t.Errorf("an unscoped tie names a knowledge layer; section = %q", recent)
	}
}

// TestAnUnreadableContractKeepsTheRecentListUnscoped pins the degradation
// direction for a contract that exists and cannot be parsed. A folder with no
// contract at all lists everything it holds; a folder whose contract broke
// used to list nothing — so the vault most in need of repair was the one the
// page refused to show, and the operator mending the toml lost the reading
// surface they were mending it with. The broken contract still closes what a
// contract answers for — the study paths — and
// the parse error stays on the page; the recent list is plain reading and
// stays, with the plain lede, because a knowledge layer this contract declared
// is a claim nothing can vouch for now.
func TestAnUnreadableContractKeepsTheRecentListUnscoped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	files := map[string]struct {
		content string
		at      time.Time
	}{
		"README.md":                    {content: "# Vault\n", at: base},
		"Concepts/Legal.md":            {content: "---\ntype: concept\ndraft: true\n---\n# Legal concept\n\nbody\n", at: base.Add(2 * time.Hour)},
		"System/templates/Template.md": {content: "---\ntype: concept\n---\n# Template note\n\nbody\n", at: base.Add(time.Hour)},
	}
	for rel, file := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(file.content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, file.at, file.at); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServerWithGovernance(t, root, nil, schema.Unreadable(
		errors.New("toml: line 42: expected a key separator"),
	))
	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	page := html.UnescapeString(body)

	// The parse error is still the page's one loud sentence.
	assertCauseStatedOnce(t, page, "toml: line 42")
	// What a contract answers for stays closed.
	if block := homeSection(t, page, `data-home-block="courses"`); strings.Contains(block, "data-desk-item") {
		t.Errorf("the desk still lists courses under an unreadable contract; block = %q", block)
	}
	shelfCode, shelfBody := get(t, srv.Client(), srv.URL+"/folders")
	if shelfCode != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", shelfCode)
	}
	shelf := html.UnescapeString(shelfBody)
	// Plain reading stays: the recent list, over everything readable, because
	// the exclusions the contract would have declared are unknown rather than
	// empty — and unknown may not silently hide files either.
	recent := homeSection(t, shelf, `data-home-block="recent"`)
	for _, want := range []string{"Legal concept", "Template note"} {
		if !strings.Contains(recent, want) {
			t.Errorf("recent block is missing %q; section = %q", want, recent)
		}
	}
	// No scoped lede: the knowledge layer is this contract's own
	// declaration, and this contract cannot vouch for it. The heading
	// already says what the list is.
	if strings.Contains(recent, "最近改動過的筆記") {
		t.Errorf("an unreadable contract's recent block still paraphrases its heading; section = %q", recent)
	}
	if strings.Contains(recent, "知識層") {
		t.Errorf("an unreadable contract's recent block cites a knowledge layer; section = %q", recent)
	}
	// The author's draft flag is the note's own and needs no vocabulary, so
	// an unreadable contract does not hide it.
	if !strings.Contains(recent, wording.Draft.In(wording.ZhHant)) {
		t.Errorf("the note's draft mark is hidden; section = %q", recent)
	}
}

// TestABrokenContractOverAFolderWithNoNotesDrawsNoRecentBlock covers the two
// answers that shape together: the reason the contract could not be read is on
// the page, and the recent list is absent because there is no note to list —
// not because the failure took it away. A block drawn empty beside that reason
// would read as a second account of the same hole.
func TestABrokenContractOverAFolderWithNoNotesDrawsNoRecentBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("not markdown\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	srv := newServerWithGovernance(t, root, nil, schema.Unreadable(
		errors.New("toml: line 42: expected a key separator"),
	))
	code, body := get(t, srv.Client(), srv.URL+"/folders")
	if code != http.StatusOK {
		t.Fatalf("GET /folders status = %d, want 200", code)
	}
	page := html.UnescapeString(body)
	if !strings.Contains(page, "toml: line 42") {
		t.Fatalf("the parse error is missing; page = %q", page)
	}
	if strings.Contains(page, `data-home-block="recent"`) {
		t.Error("the folder index renders a recent block for a folder with no markdown")
	}
}

// TestTheFolderBlockDoesNotDenyTheFilesItCounts covers a vault whose files all
// sit at the root. The folder mode's rows are the folders at the top, and it
// has none; its measure is every file under them, and it has two. A block that
// took its own rowlessness for emptiness printed a count of the files and, an
// inch below it, the sentence for a vault holding none.
func TestTheFolderBlockDoesNotDenyTheFilesItCounts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"one.md", "two.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("# "+name+"\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	srv := newServerWithContract(t, root, contractWithPrivacySection(t, "[privacy]\nnever_egress_dirs = []\n"))

	code, body := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", code, http.StatusOK)
	}
	block := deskBlockMarkup(t, body, "folders")
	if !strings.Contains(block, "2") {
		t.Errorf("the folders block does not count the two files at the root: %q", block)
	}
	if strings.Contains(block, `data-nothing="shelf"`) {
		t.Errorf("the folders block counts files and calls itself empty in the same breath: %q", block)
	}
}

// deskBlockMarkup slices one way in out of the desk, from its own marker to the
// end of the section it opens.
func deskBlockMarkup(t *testing.T, page, mode string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, `data-home-block="`+mode+`"`)
	if !found {
		t.Fatalf("the desk carries no %s block", mode)
	}
	block, _, closed := strings.Cut(rest, "</section>")
	if !closed {
		t.Fatalf("the %s block is never closed", mode)
	}
	return block
}

// TestTheContentsListOmitsAClosedCalloutsHeadings pins what a reader sees on
// the page: a heading inside a callout closed by default is not named in the
// contents list, since naming it would give away the answer, yet the heading
// keeps its id in the body so a link can still land on it.
func TestTheContentsListOmitsAClosedCalloutsHeadings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessonMD := "---\ntype: lesson\ncreated: 2026-06-01\n---\n# L01\n\n" +
		"\n## Before\n\ntext\n\n> [!question]-\n> ## Hidden Answer\n> the answer\n\n> [!tip]+\n> ## Open Answer\n> shown\n\n## After\n\ntext\n"
	rel := "Writing/lessons/japanese/L01.md"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, id := range []string{"before", "open-answer", "after"} {
		if !strings.Contains(page, `href="#`+id+`"`) {
			t.Errorf("the contents list does not name %q; page = %q", id, page)
		}
	}
	if strings.Contains(page, `href="#hidden-answer"`) {
		t.Error("the contents list names a heading inside a closed callout")
	}
	if !strings.Contains(page, `id="hidden-answer"`) {
		t.Error("the closed callout's heading lost its id in the body")
	}
}
