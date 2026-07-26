package note_test

import (
	"context"
	"crypto/sha256"
	"errors"
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
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

func openStatusLifecycle(
	t *testing.T,
	source *vault.Reader,
	contract *schema.Contract,
	governance schema.Governance,
) *status.Lifecycle {
	t.Helper()
	lifecycle, err := status.Open(source, contract, governance)
	if err != nil {
		t.Fatalf("status.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lifecycle.Close(); err != nil {
			t.Errorf("Lifecycle.Close() error = %v", err)
		}
	})
	return lifecycle
}

// newServer wires the reading page against a real (not faked)
// status.Lifecycle, with a nil contract (fail-closed). Good
// enough for tests whose point is that the page renders regardless of
// whether the write face is available (reading stays fail-open even when
// the write face is fail-closed) — NOT for exercising
// handler.go's NoFrontmatter/Transitions branch selection, since a
// fail-closed Lifecycle supplies a write diagnostic and note.templ's statusPanel
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
func newServerWithContract(t *testing.T, root string, contract *schema.Contract) *httptest.Server {
	t.Helper()
	return newServerWithProvenance(t, root, contract, func(context.Context, string, [sha256.Size]byte) (string, error) {
		return "", nil
	})
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
	return newServerWithProvenanceAndGovernance(
		t, root, contract, governance,
		func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
	)
}

func newServerWithProvenance(
	t *testing.T,
	root string,
	contract *schema.Contract,
	provenance func(context.Context, string, [sha256.Size]byte) (string, error),
) *httptest.Server {
	t.Helper()
	return newServerWithProvenanceAndGovernance(t, root, contract, contract.Governance(), provenance)
}

func newServerWithProvenanceAndGovernance(
	t *testing.T,
	root string,
	contract *schema.Contract,
	governance schema.Governance,
	provenance func(context.Context, string, [sha256.Size]byte) (string, error),
) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, governance)
	lifecycle := openStatusLifecycle(t, source, contract, governance)
	h := note.New(&note.Dependencies{
		Source:     source,
		Status:     lifecycle.View,
		Snapshot:   store.Current,
		Provenance: provenance,
		WriteBlock: lifecycle.WriteBlockReason,
		Log:        log,
	})
	h.Register(mux)
	status.NewHandler(lifecycle, func() pages.Shell { return pages.Shell{} }, log).Register(mux)
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
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"), []byte("---\ntitle: は\ntype: writing\n---\n\nConcept body.\n"), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir lesson directory: %v", err)
	}
	body := "---\ntitle: Loud lesson template\ntype: lesson\nstatus: ready\nslug: " + loudLessonSlug + "\n---\n\n" +
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

	provenanceCalls := 0
	srv := newServerWithProvenance(t, root, loadContract(t), func(context.Context, string, [sha256.Size]byte) (string, error) {
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
		"不屬於生命週期治理範圍",
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
		"操作者 · koopa",
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
	lifecycle := openStatusLifecycle(t, source, contract, contract.Governance())
	statusCaptures := 0
	statusProvider := func() status.View {
		statusCaptures++
		return lifecycle.View()
	}

	mux := http.NewServeMux()
	handler := note.New(&note.Dependencies{
		Source:     source,
		Status:     statusProvider,
		Snapshot:   store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
		Log:        log,
	})
	handler.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
	if code != http.StatusOK {
		t.Fatalf("GET lesson status = %d, want 200", code)
	}
	for _, want := range []string{`action="/status"`, `data-concept=`, "y-slotmachine"} {
		if !strings.Contains(page, want) {
			t.Errorf("captured-open request is missing %q", want)
		}
	}
	if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("change contract between requests: %v", writeErr)
	}

	code, page = get(t, srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
	if code != http.StatusOK {
		t.Fatalf("second GET lesson status = %d, want 200", code)
	}
	const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
	if !strings.Contains(page, diagnostic) {
		t.Error("next reading request is missing the latched authority diagnostic")
	}
	for _, leaked := range []string{
		`action="/status"`,
		`data-tts=`,
		`data-concept=`,
		`data-concept-sheet`,
		`data-advanceable-chip`,
	} {
		if strings.Contains(page, leaked) {
			t.Errorf("next reading request retained %q after authority drift", leaked)
		}
	}
	if statusCaptures != 2 {
		t.Errorf("two reading requests captured status %d times, want exactly once per request", statusCaptures)
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
			lifecycle := openStatusLifecycle(t, source, contract, contract.Governance())

			var statusView status.View
			var captured *snapshot.View
			if snapshotFirst {
				captured = store.Current().Capture()
			} else {
				statusView = lifecycle.View()
			}
			if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
				t.Fatalf("change contract between captures: %v", writeErr)
			}
			if snapshotFirst {
				statusView = lifecycle.View()
			} else {
				captured = store.Current().Capture()
			}

			mux := http.NewServeMux()
			note.New(&note.Dependencies{
				Source:     source,
				Status:     func() status.View { return statusView },
				Snapshot:   func() *snapshot.View { return captured },
				Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
				Log:        log,
			}).Register(mux)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
			if code != http.StatusOK {
				t.Fatalf("GET lesson status = %d, want 200", code)
			}
			const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
			if !strings.Contains(page, diagnostic) {
				t.Error("reading page is missing the authority diagnostic")
			}
			if !strings.Contains(page, `data-status-state="unavailable"`) {
				t.Error("reading page did not mark the status face unavailable")
			}
			for _, leaked := range []string{
				`action="/status"`,
				`data-tts=`,
				`data-concept=`,
				`data-concept-sheet`,
				"y-slotmachine",
				"操作者 · koopa",
			} {
				if strings.Contains(page, leaked) {
					t.Errorf("reading page retained %q across torn authority captures", leaked)
				}
			}
		})
	}
}

// TestHomeClosesTheLifecycleBlockForEitherAuthorityCaptureOrder is the landing
// page's twin of the reading page's reconciliation. Home derives its lifecycle
// block from the captured write authority and its counts from the snapshot's
// own artifact capture; the two are sampled at different instants, so whichever
// was taken first, a block that one authority still allows must close when the
// other has already stopped answering. Without the second check the page
// renders a status list counted against exclusions it can no longer read.
func TestHomeClosesTheLifecycleBlockForEitherAuthorityCaptureOrder(t *testing.T) {
	t.Parallel()

	for _, snapshotFirst := range []bool{true, false} {
		name := "status-first"
		if snapshotFirst {
			name = "snapshot-first"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
				t.Fatalf("write README: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, "Note.md"), []byte("---\ntitle: Note\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
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
			lifecycle := openStatusLifecycle(t, source, contract, contract.Governance())

			var statusView status.View
			var captured *snapshot.View
			if snapshotFirst {
				captured = store.Current().Capture()
			} else {
				statusView = lifecycle.View()
			}
			if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
				t.Fatalf("change contract between captures: %v", writeErr)
			}
			if snapshotFirst {
				statusView = lifecycle.View()
			} else {
				captured = store.Current().Capture()
			}

			mux := http.NewServeMux()
			note.New(&note.Dependencies{
				Source:     source,
				Status:     func() status.View { return statusView },
				Snapshot:   func() *snapshot.View { return captured },
				Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
				Log:        log,
			}).Register(mux)
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			code, body := get(t, srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}
			page := html.UnescapeString(body)
			if strings.Contains(page, `data-home-block="lifecycle"`) {
				t.Errorf("Home rendered a lifecycle block across torn authority captures; page = %q", page)
			}
			const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
			assertCauseStatedAtMostOncePerRegion(t, page, diagnostic)
			if strings.Contains(page, "data-advanceable-chip") {
				t.Error("Home kept the advanceable chip across torn authority captures")
			}
		})
	}
}

// TestShowKeepsTheWriteFaceClosedOnAGovernedFolderThatIsNoRepository pins the
// second producer of a closed write face. The contract here is complete and
// valid, so the vault is governed and the status face belongs on the page — but
// every transition is recorded as a commit, and this folder has no git working
// tree to record one in. The page must say so and offer nothing, rather than
// naming the operator beside a control that can only fail.
func TestShowKeepsTheWriteFaceClosedOnAGovernedFolderThatIsNoRepository(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Note.md"), []byte("---\ntitle: Note\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, page := get(t, srv.URL+"/notes/Note.md")
	if code != http.StatusOK {
		t.Fatalf("GET note status = %d, want 200", code)
	}
	page = html.UnescapeString(page)

	surfaces := statusSurfaces(t, page)
	if len(surfaces) == 0 {
		t.Fatalf("a governed vault rendered no status face at all; page = %q", page)
	}
	for _, surface := range surfaces {
		if !strings.Contains(surface.body, `data-status-state="unavailable"`) {
			t.Errorf("%s did not mark the write face unavailable", surface.name)
		}
		if !strings.Contains(surface.body, status.GitBlockReason) {
			t.Errorf("%s does not say why the transition would be refused", surface.name)
		}
		for _, leaked := range []string{"操作者 · koopa", "目前沒有合法的狀態轉換", `action="/status"`} {
			if strings.Contains(surface.body, leaked) {
				t.Errorf("%s asserts write authority that was refused: %q", surface.name, leaked)
			}
		}
	}
}

// TestShowCarriesTheNavigationFaultOnANotePage pins the path the reader
// actually takes to a note: the command palette opens one directly and never
// renders Home. A contract whose navigation declaration was rejected closes
// paths and maps while leaving the write face and the recent list working, so
// the rail is the only place that fault can appear — and if it appears nowhere
// on this route, a reader who lives in the palette never learns their contract
// is wrong.
func TestShowCarriesTheNavigationFaultOnANotePage(t *testing.T) {
	t.Parallel()

	const rejectedNavigation = "[navigation]\npath_types = [\"missing-type\"]\nmap_types = []\n"
	const validArtifact = "[artifacts]\nnon_instance_dirs = [\"System/templates\"]\n"
	contract := loadHomeContractWithSections(t, rejectedNavigation, validArtifact)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Note.md"), []byte("---\ntitle: Note\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, contract)
	code, body := get(t, srv.URL+"/notes/Note.md")
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
	if !strings.Contains(rail, "missing-type") {
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
	lifecycle := openStatusLifecycle(t, source, nil, schema.Ungoverned())
	statusCaptures := 0
	mux := http.NewServeMux()
	note.New(&note.Dependencies{
		Source: source,
		Status: func() status.View {
			statusCaptures++
			return lifecycle.View()
		},
		Snapshot:   store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
		Log:        log,
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, _ := get(t, srv.URL+"/notes/guide.txt")
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
			srv := newServerWithProvenance(t, root, contract, func(context.Context, string, [sha256.Size]byte) (string, error) {
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
				"操作者 · koopa",
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
		contract   func(*testing.T) *schema.Contract
		governance schema.Governance
		want       string
		wantAbsent string
	}{
		{
			// A contract that exists and cannot be read. This row used to pass a
			// nil contract, which is also what a folder carrying no contract
			// produces — so the fault and the ordinary case were the same test.
			name: "core contract unreadable",
			contract: func(t *testing.T) *schema.Contract {
				t.Helper()
				return nil
			},
			governance: schema.Unreadable(errors.New("toml: line 42: expected a key separator")),
			want:       "toml: line 42",
			wantAbsent: "contract declares no artifact policy; instance projections disabled until it does",
		},
		{
			name: "artifact policy missing",
			contract: func(t *testing.T) *schema.Contract {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "")
			},
			want:       "contract declares no artifact policy; instance projections disabled until it does",
			wantAbsent: status.CoreUnavailableDiagnostic,
		},
		{
			name: "artifact policy invalid",
			contract: func(t *testing.T) *schema.Contract {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\".\"]\n")
			},
			want:       `invalid artifact policy: non_instance_dirs contains "."`,
			wantAbsent: status.CoreUnavailableDiagnostic,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "Note.md"), []byte("---\ntitle: Note\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			contract := tt.contract(t)
			governance := tt.governance
			if contract != nil {
				governance = contract.Governance()
			}
			srv := newServerWithGovernance(t, root, contract, governance)
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
	openAt := strings.Index(page, `<main id="main-content" tabindex="-1" class="y-main">`)
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
func loadContract(t *testing.T) *schema.Contract {
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

func loadHomeContractWithSections(t *testing.T, navigationSection, artifactSection string) *schema.Contract {
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

func TestShowUsesContractDeclaredArticleLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		langLine    string
		contract    func(*testing.T) *schema.Contract
		wantArticle string
	}{
		{name: "canonicalizes declared tag", langLine: "lang: zh-hant\n", contract: loadContract, wantArticle: `lang="zh-Hant"`},
		{name: "invalid tag is undetermined", langLine: "lang: not_a_tag\n", contract: loadContract, wantArticle: `lang="und"`},
		{name: "missing value is undetermined", contract: loadContract, wantArticle: `lang="und"`},
		{name: "undeclared field has no authority", langLine: "lang: ja\n", contract: loadContractWithoutArticleLanguage, wantArticle: `lang="und"`},
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
			body := "---\ntitle: 日本語\ntype: writing\ndomain: japanese\nstatus: draft\n" + tt.langLine + "---\n\n本文\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract(t))
			code, page := get(t, srv.URL+"/notes/Writing/%E6%97%A5%E6%9C%AC%E8%AA%9E.md")
			if code != http.StatusOK {
				t.Fatalf("GET note = %d, want 200; body = %q", code, page)
			}
			want := `<article class="y-article" ` + tt.wantArticle + `>`
			if !strings.Contains(page, want) {
				t.Errorf("article language = %q, want markup %q", page, want)
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
	const declared = `, "lang"]`
	without := strings.Replace(string(data), declared, `]`, 1)
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
	if err := os.WriteFile(filepath.Join(dir, "L00 テスト課.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServer(t, root)

	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L00 テスト課.md")
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
	// Nothing governs this folder, so the page carries no write face at all —
	// not the forms, and not an apology for a contract that was never promised.
	// Reading is whole either way; that asymmetry is the point.
	for _, absent := range []string{
		"y-statuspanel",
		"y-sealbar",
		"操作者 · koopa",
		"生命週期寫入目前無法使用",
		"fail-closed",
		`action="/status"`,
		"ui-status--draft",
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
		[]byte("---\ntitle: L00\ntype: lesson\nstatus: draft\n---\n\n"+body), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S00.md"),
		[]byte("---\ntitle: S00\ntype: source-note\nstatus: draft\n---\n\n"+body), 0o600); err != nil {
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
	// A lesson with a slug but no System/slots dir at all (Slots stays empty).
	body := "---\ntitle: L02\ntype: lesson\nstatus: draft\nslug: jp-orphan\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "L02.md"), []byte(body), 0o600); err != nil {
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
		[]byte("---\ntitle: は (主題助詞)\ntype: concept\n---\n\nMarks the topic of the sentence.\n"), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	lessonDir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lessonDir, "L01.md"),
		[]byte("---\ntitle: L01\ntype: lesson\nstatus: draft\n---\n\nThe particle [[は]] marks the topic.\n"), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(conceptDir, "は.md"), []byte("topic particle\n"), 0o600); err != nil {
		t.Fatalf("write concept: %v", err)
	}
	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S.md"),
		[]byte("---\ntitle: S\ntype: source-note\nstatus: draft\n---\n\nMentions [[は]] in passing.\n"), 0o600); err != nil {
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
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
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
		"recently changed": `data-home-block="recent"`,
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
	// This folder carries no contract. It has no lifecycle vocabulary, so the
	// block that would name one is absent rather than empty or apologetic, and
	// nothing on the page reports a capability that was never claimed. Before
	// the grant split this page opened with seven such notices and the reader's
	// own README started below the fold.
	for name, marker := range map[string]string{
		"lifecycle block":   `data-home-block="lifecycle"`,
		"capability faults": `data-home-block="faults"`,
		"fault row":         "data-home-fault",
		"advanceable chip":  "data-advanceable-chip",
		"sidebar fault":     `data-sidebar-group="navigation-diagnostics"`,
	} {
		if strings.Contains(pageHTML, marker) {
			t.Errorf("ungoverned Home renders the %s marker %q", name, marker)
		}
	}
	if !strings.Contains(pageHTML, "README body sentinel.") {
		t.Error("the reader's own content is missing")
	}
}

// TestHomeRecentOmitsTheStatusChipForAnUngovernedFolder pins the same
// separation on Home that the search results page pins on a hit. An ungoverned
// folder's notes may still carry a "status:" line — frontmatter is the author's,
// not the contract's — and the recent list still shows them. What it must not do
// is render that raw word as a lifecycle chip, which would name a value from a
// vocabulary nothing here ever declared.
func TestHomeRecentOmitsTheStatusChipForAnUngovernedFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "Note.md"),
		[]byte("---\ntitle: Author's own note\ntype: writing\nstatus: draft\n---\n\nbody\n"),
		0o600,
	); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServer(t, root)
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)

	if !strings.Contains(recent, "Author&#39;s own note") && !strings.Contains(recent, "Author's own note") {
		t.Fatalf("recent block does not list the note at all; section = %q", recent)
	}
	for _, leaked := range []string{"ui-status--draft", "ui-status"} {
		if strings.Contains(recent, leaked) {
			t.Errorf("ungoverned recent list renders %q; no vocabulary was ever declared here", leaked)
		}
	}
}

// TestHomeReportsAnUnreadableContractExactlyOnce is the other side of TestHome.
// The same absence of a usable contract, but here the folder claimed one and
// could not deliver it, so the page says so — once, in a node of its own,
// rather than repeating the cause in each block it closed.

// assertCauseStatedOncePerRegion pins how a capability fault reaches the reader:
// once in the navigation rail, which every page carries, and once in Home's own
// content column, which stays visible when the rail is collapsed behind its
// toggle. Two regions, one sentence each — not the six-deep column of repeats
// that used to push the reader's own content below the fold.
func assertCauseStatedOncePerRegion(t *testing.T, page, cause string) {
	t.Helper()
	rail := pageSection(page, `<section class="y-navdiag"`)
	content := pageSection(page, `data-home-block="faults"`)
	if rail == "" {
		t.Errorf("the navigation rail states nothing; a note opened from the palette never renders Home")
	} else if got := strings.Count(rail, cause); got != 1 {
		t.Errorf("the rail states the cause %d times, want once: %q", got, rail)
	}
	if content == "" {
		t.Errorf("Home's content column states nothing; at narrow widths the rail is behind a toggle")
	} else if got := strings.Count(content, cause); got != 1 {
		t.Errorf("Home's content column states the cause %d times, want once: %q", got, content)
	}
	if got := strings.Count(page, cause); got != 2 {
		t.Errorf("the cause appears %d times on the page, want exactly one per region", got)
	}
	if got := strings.Count(page, "data-home-fault"); got != 1 {
		t.Errorf("Home fault rows = %d, want exactly 1", got)
	}
}

// assertCauseStatedAtMostOncePerRegion is the weaker sibling, for pages where
// which region knows the cause depends on which authority was sampled first:
// with the write view captured before a drift and the snapshot after it, the
// write face still believes it is open and only the rail has anything to say.
// The cause must still reach the reader, and must not repeat inside a region.
func assertCauseStatedAtMostOncePerRegion(t *testing.T, page, cause string) {
	t.Helper()
	rail := strings.Count(pageSection(page, `<section class="y-navdiag"`), cause)
	content := strings.Count(pageSection(page, `data-home-block="faults"`), cause)
	if rail > 1 || content > 1 {
		t.Errorf("the cause repeats inside a region: rail=%d content=%d", rail, content)
	}
	if rail+content == 0 {
		t.Errorf("no region states the cause; page = %q", page)
	}
	if got := strings.Count(page, cause); got != rail+content {
		t.Errorf("the cause appears %d times but only %d are accounted for by the two regions", got, rail+content)
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
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	page := html.UnescapeString(body)

	assertCauseStatedOncePerRegion(t, page, "toml: line 42")
	// The blocks whose projections closed say nothing rather than asserting an
	// emptiness they cannot vouch for.
	for name, marker := range map[string]string{
		"recent":      `data-home-block="recent"`,
		"lifecycle":   `data-home-block="lifecycle"`,
		"study paths": `data-home-block="study-paths"`,
	} {
		if strings.Contains(page, marker) {
			t.Errorf("Home still renders the %s block under an unreadable contract", name)
		}
	}
	for _, want := range []string{`data-home-block="search"`, "README body sentinel."} {
		if !strings.Contains(page, want) {
			t.Errorf("Home lost %q to a contract failure; reading never depends on one", want)
		}
	}
}

// TestHomeReadmeImagesAddressTheBytes covers the landing page separately from
// the reading page, because Home renders a body through its own call and a fix
// applied at the reading page alone leaves the first screen anyone sees showing
// a broken image.
func TestHomeReadmeImagesAddressTheBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Assets"), 0o750); err != nil {
		t.Fatalf("make Assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Assets", "cover.png"), []byte("png"), 0o600); err != nil {
		t.Fatalf("write cover: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"),
		[]byte("# Vault\n\n![cover](./Assets/cover.png)\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	srv := newServer(t, root)

	code, pageHTML := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", code, http.StatusOK)
	}
	const want = `src="/raw/Assets/cover.png"`
	if !strings.Contains(pageHTML, want) {
		t.Errorf("GET / README image is not routed to the bytes; want %q in:\n%s", want, pageHTML)
	}
}

// TestReadingRoutesKeepCapturedViewWhenCurrentSwaps is the coherence guard for
// publication during a request. The provider atomically installs a different
// current View while returning the previously current one; every projection in
// the response must still come from that one captured value.
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
	lifecycle := openStatusLifecycle(t, firstSource, nil, schema.Ungoverned())

	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "home", path: "/"},
		{name: "note", path: "/notes/README.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var current atomic.Pointer[snapshot.View]
			current.Store(firstStore.Current())
			calls := 0
			mux := http.NewServeMux()
			note.New(&note.Dependencies{
				Source: firstSource,
				Status: lifecycle.View,
				Snapshot: func() *snapshot.View {
					calls++
					return current.Swap(secondStore.Current())
				},
				Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
				Log:        log,
			}).Register(mux)
			rr := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, rr.Code, http.StatusOK)
			}
			const firstLink = `<a href="/notes/Concepts/Target.md" class="wikilink">Target</a>`
			if !strings.Contains(rr.Body.String(), firstLink) {
				t.Errorf("GET %s did not resolve against the captured View; want %q in body", tt.path, firstLink)
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
	lifecycle := openStatusLifecycle(t, source, nil, schema.Ungoverned())

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
			note.New(&note.Dependencies{
				Source: source,
				Status: lifecycle.View,
				Snapshot: func() *snapshot.View {
					calls++
					return store.Current()
				},
				Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
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
		content := fmt.Sprintf("---\ntitle: %s\ntype: concept\nstatus: draft\n---\n\nbody\n", name)
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
	for name, statusName := range map[string]string{"Open": "draft", "Sealed": schema.SealStatus} {
		content := fmt.Sprintf("---\ntitle: %s\ntype: lesson\nstatus: %s\n---\n\nbody\n", name, statusName)
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
	pathBody := "---\ntitle: Test path\ntype: study-path\n---\n\n## Part\n\n- [[Open]]\n- [[Sealed]]\n"
	pathFile := filepath.Join(mapDir, "path.md")
	if err := os.WriteFile(pathFile, []byte(pathBody), 0o600); err != nil {
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
	for _, marker := range []string{"Test path", "1 / 2 已完成", "/syllabus/Maps/path.md"} {
		if !strings.Contains(paths, marker) {
			t.Errorf("Study paths block is missing %q", marker)
		}
	}
	if !strings.Contains(body, "Dashboard README sentinel.") {
		t.Error("Home is missing the rendered vault README body")
	}
	if !strings.Contains(body, `aria-label="1 篇筆記可進入下一個合法狀態"`) {
		t.Error("Home topbar is missing the snapshot-derived advanceable chip")
	}
}

// TestHomeWithholdsTheLifecycleBlockAndNamesTheCause covers a vault something
// governs whose artifact declaration yomihon could not honour. The lifecycle
// block cannot be counted honestly, so it is withheld rather than shown empty,
// and the reason is stated once in the page's own fault node.
func TestHomeWithholdsTheLifecycleBlockAndNamesTheCause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contract func(*testing.T) *schema.Contract
		want     string
	}{
		{
			name: "artifact policy left out of an existing contract",
			contract: func(t *testing.T) *schema.Contract {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "")
			},
			want: "contract declares no artifact policy; instance projections disabled until it does",
		},
		{
			name: "artifact policy rejected",
			contract: func(t *testing.T) *schema.Contract {
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
			page := html.UnescapeString(body)

			if strings.Contains(page, `data-home-block="lifecycle"`) {
				t.Error("Home rendered a lifecycle block it cannot count")
			}
			assertCauseStatedOncePerRegion(t, page, tt.want)
		})
	}
}

func TestHomeUsesOneAuthorityViewAndClosesTheNextRequestAfterDrift(t *testing.T) {
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
title: A
type: concept
domain: golang
status: seedling
created: 2026-07-16
updated: 2026-07-16
based_on: source
---

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
	statusCaptures := 0
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	lifecycle := openStatusLifecycle(t, source, contract, contract.Governance())
	requestStatus := func() status.View {
		statusCaptures++
		return lifecycle.View()
	}
	mux := http.NewServeMux()
	handler := note.New(&note.Dependencies{
		Source:     source,
		Status:     requestStatus,
		Snapshot:   store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
		Log:        log,
	})
	handler.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	for _, block := range []string{"recent", "lifecycle"} {
		section := html.UnescapeString(homeSection(t, body, `data-home-block="`+block+`"`))
		if !strings.Contains(section, ">A<") && !strings.Contains(section, "seedling") {
			t.Errorf("captured-open %s block lost its instance projection: %q", block, section)
		}
	}
	if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("change contract between requests: %v", writeErr)
	}

	code, body = get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("second GET / status = %d, want 200", code)
	}
	const diagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"
	page := html.UnescapeString(body)
	assertCauseStatedOncePerRegion(t, page, diagnostic)
	for _, block := range []string{"recent", "lifecycle"} {
		if strings.Contains(page, `data-home-block="`+block+`"`) {
			t.Errorf("%s block survived authority drift; a projection it cannot vouch for must be withheld", block)
		}
	}
	// The folder tree still lists the note: ordinary browsing never depended on
	// the artifact policy. What must be gone is the instance-derived status.
	if !strings.Contains(page, "/notes/Concepts/golang/A.md") {
		t.Error("ordinary folder browsing closed with the artifact authority")
	}
	if strings.Contains(page, "seedling") {
		t.Errorf("Home retained an instance projection after authority drift: %q", page)
	}
	if statusCaptures != 2 {
		t.Errorf("two home requests captured status %d times, want exactly once per request", statusCaptures)
	}
}

func TestHomeArtifactPolicyDegradesInstanceProjections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract *schema.Contract
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
			body := html.UnescapeString(page)
			// Every instance-derived block closes on one cause, so the cause is
			// stated once and each closed block renders nothing at all.
			for _, marker := range []string{
				`data-home-block="recent"`,
				`data-home-block="lifecycle"`,
				`data-home-block="study-paths"`,
			} {
				if strings.Contains(body, marker) {
					t.Errorf("Home still renders %s without a usable artifact policy", marker)
				}
			}
			assertCauseStatedOncePerRegion(t, body, tt.want)
			if strings.Contains(body, `data-advanceable-chip`) {
				t.Error("Home advanceable chip remained available without artifact metadata")
			}
			if !strings.Contains(body, `data-home-block="search"`) {
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
	if !strings.Contains(draftRow, `aria-label="1 篇筆記">1</span>`) {
		t.Errorf("Lifecycle draft count degraded with navigation roles; row = %q", draftRow)
	}
	body := html.UnescapeString(page)
	if strings.Contains(body, `data-home-block="study-paths"`) {
		t.Error("Study Paths rendered without usable navigation roles")
	}
	// A rejected navigation declaration closes only the study paths, and the
	// write authority knows nothing about it. Stating the cause in the rail
	// alone would drop that block with no explanation for any reader whose
	// window is narrow enough to fold the rail behind its toggle.
	assertCauseStatedOncePerRegion(t, body,
		`invalid navigation roles: path type "missing-type" is not declared in enums.type`)
	if strings.Contains(body, "contract declares no artifact policy; instance projections disabled until it does") {
		t.Errorf("Home falsely reports an artifact failure: %q", body)
	}
	if !strings.Contains(page, `aria-label="1 篇筆記可進入下一個合法狀態"`) {
		t.Error("advanceable chip was suppressed by navigation-only failure")
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
	body := html.UnescapeString(page)
	// Two declarations were rejected independently. The artifact one closed the
	// write authority, so it reaches Home's content column; the navigation one
	// closed only paths and maps, so the rail is its home. Each is stated once
	// where it belongs, and neither is repeated.
	rail := pageSection(body, `<section class="y-navdiag"`)
	content := pageSection(body, `data-home-block="faults"`)
	if got := strings.Count(rail, "missing-type"); got != 1 {
		t.Errorf("the rail states the navigation cause %d times, want once: %q", got, rail)
	}
	const artifactCause = `invalid artifact policy: non_instance_dirs contains "."`
	if got := strings.Count(content, artifactCause); got != 1 {
		t.Errorf("Home's content column states the artifact cause %d times, want once: %q", got, content)
	}
	for _, cause := range []string{"missing-type", artifactCause} {
		if got := strings.Count(body, cause); got > 2 {
			t.Errorf("capability cause %q appears %d times, want at most one per region", cause, got)
		}
	}
	if strings.Contains(body, `data-home-block="study-paths"`) {
		t.Error("Study Paths rendered under two rejected declarations")
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
	if !strings.Contains(draftRow, `aria-label="1 篇筆記">1</span>`) {
		t.Errorf("draft lifecycle count includes the template or misses the instance; row = %q", draftRow)
	}
	if !strings.Contains(page, `aria-label="1 篇筆記可進入下一個合法狀態"`) {
		t.Error("advanceable count includes the template or misses the instance")
	}
}

// TestHomeWithoutReadmeKeepsDashboardReadOnly pins first-use recovery: Home is
// still the complete snapshot dashboard, only the absent README body is
// replaced, and neither route creates the missing vault file.
func TestHomeWithoutReadmeKeepsDashboardReadOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	srv := newServerWithContract(t, root, loadContract(t))

	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / without README status = %d, want %d", code, http.StatusOK)
	}
	for _, marker := range []string{
		`data-home-block="recent"`,
		`data-home-block="lifecycle"`,
		`data-home-block="study-paths"`,
		`data-home-block="search"`,
		`data-home-readme-recovery`,
		`請使用外部編輯器或檔案工具，在 vault 根目錄建立 README.md，然後重新載入此頁。`,
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("GET / without README is missing %q", marker)
		}
	}

	noteCode, _ := get(t, srv.URL+"/notes/README.md")
	if noteCode != http.StatusNotFound {
		t.Errorf("GET /notes/README.md without README status = %d, want %d", noteCode, http.StatusNotFound)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Errorf("README.md after recovery requests: os.Stat error = %v, want not-exist", err)
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
	if err := os.WriteFile(filepath.Join(dir, "d1.md"), []byte("just a drill body\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, body := get(t, srv.URL+"/notes/Drills/d1.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "沒有 frontmatter") {
		t.Errorf("page missing the no-frontmatter notice; body = %q", body)
	}
	if strings.Contains(body, status.CoreUnavailableDiagnostic) || strings.Contains(body, "fail-closed") {
		t.Errorf("page shows the fail-closed notice even though the contract loaded; body = %q", body)
	}
}

// TestShowTransitions exercises handler.go's default branch (view.Transitions
// = statusView.Transitions(n.RelPath, n.Type(), n.Status())) with a loaded contract.
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
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
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
	transitionSource := openReadingVault(t, root)
	transitions := openStatusLifecycle(t, transitionSource, contract, contract.Governance()).View().Transitions("Writing/lessons/japanese/L01.md", "lesson", "draft")
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusSeeOther {
		b, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			t.Fatalf("read failed status response: %v", readErr)
		}
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
	if strings.Contains(body, status.CoreUnavailableDiagnostic) || strings.Contains(body, "fail-closed") || strings.Contains(body, "沒有 frontmatter") {
		t.Errorf("page shows the wrong status-panel branch; body = %q", body)
	}
}

func TestNewPanicsOnNilDependencies(t *testing.T) {
	t.Parallel()
	defer func() {
		if got := recover(); got != "note: New requires non-nil Dependencies" {
			t.Fatalf("New(nil Dependencies) panic = %v, want explicit wiring diagnostic", got)
		}
	}()
	note.New(nil)
}

func TestNewCopiesDependencies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "constructor ownership sentinel\n"
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte(body), 0o600); err != nil {
		t.Fatalf("write raw fixture: %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	lifecycle := openStatusLifecycle(t, source, nil, schema.Ungoverned())
	deps := note.Dependencies{
		Source:     source,
		Status:     lifecycle.View,
		Snapshot:   store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
		Log:        log,
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

func TestNewPanicsOnNilSource(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("New(nil Source) did not panic")
		}
	}()
	root := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	lifecycle := openStatusLifecycle(t, source, nil, schema.Ungoverned())
	note.New(&note.Dependencies{
		Source:     nil,
		Status:     lifecycle.View,
		Snapshot:   store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
		Log:        log,
	})
}

// TestNewPanicsOnNilStatusProvider mirrors
// internal/status/handler_test.go's coverage of status.NewHandler's own
// nil-dependency panic: a fail-closed lifecycle still has a valid View method,
// but a literal nil provider is not — a
// future caller passing one must fail at wiring time, not three calls deep
// inside the first GET /notes/... request.
func TestNewPanicsOnNilStatusProvider(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("New(nil Status provider) did not panic")
		}
	}()
	root := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	note.New(&note.Dependencies{
		Source:     source,
		Status:     nil, // the nil under test
		Snapshot:   store.Current,
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
		Log:        log,
	})
}

// TestNewPanicsOnNilSnapshot mirrors the Status provider check: a provider
// returning an empty-but-valid snapshot is legal, but a nil provider is a
// wiring bug that must fail at construction rather than inside the first read.
func TestNewPanicsOnNilSnapshot(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("New(nil Snapshot provider) did not panic")
		}
	}()
	root := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	_, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	lifecycle := openStatusLifecycle(t, source, nil, schema.Ungoverned())
	note.New(&note.Dependencies{
		Source:     source,
		Status:     lifecycle.View,
		Snapshot:   nil, // the nil under test
		Provenance: func(context.Context, string, [sha256.Size]byte) (string, error) { return "", nil },
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
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	syllabus := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字\n\n- [[Slices]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(syllabus), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(mapDir, "Reading map.md"), []byte(mapNote), 0o600); err != nil {
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

// TestWriteBlockShownBeforeThePress locks the wiring that makes the reading
// page state a refusal the write path would otherwise only reveal after the
// operator commits to the action. The transition controls are derived from the
// contract, which cannot see the working tree, so a note carrying an
// uncommitted edit used to render a fully live seal button that then failed.
// Both directions are asserted: a committed note must stay quiet, or the notice
// is permanent furniture rather than a signal.
func TestWriteBlockShownBeforeThePress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Test Vault")
	runGit(t, root, "config", "user.email", "test-vault@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	rel := "Writing/lessons/japanese/L01.md"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	notePath := filepath.Join(dir, "L01.md")
	if err := os.WriteFile(notePath, []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, root, "add", rel)
	runGit(t, root, "commit", "-m", "seed lesson")
	srv := newServerWithContract(t, root, loadContract(t))

	code, clean := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(clean, `name="to" value="`+schema.SealStatus+`"`) {
		t.Fatalf("committed note is missing the seal control this test depends on; body = %q", clean)
	}
	if strings.Contains(clean, status.DirtyBlockReason) {
		t.Errorf("committed note shows the uncommitted-changes notice; it would then never mean anything")
	}

	if err := os.WriteFile(notePath, []byte(lessonMD+"an edit nobody committed\n"), 0o600); err != nil {
		t.Fatalf("dirty the note: %v", err)
	}

	code, dirty := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("status after the edit = %d, want 200", code)
	}
	if !strings.Contains(dirty, status.DirtyBlockReason) {
		t.Errorf("note with an uncommitted edit does not state why a transition would be refused; body = %q", dirty)
	}
	if !strings.Contains(dirty, `name="to" value="`+schema.SealStatus+`"`) {
		t.Errorf("the notice removed the transition control; it is advisory and the operator may clear the edit and retry")
	}
}

// TestStatusPanelPrecedesOutline locks the right rail's order. The rail is its
// own scroll container, so whatever renders first decides what the reader sees
// without scrolling — and an outline of any length used to push the transition
// controls past the bottom of the window on most notes, where scrolling the
// article moved them not at all.
func TestStatusPanelPrecedesOutline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Test Vault")
	runGit(t, root, "config", "user.email", "test-vault@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	// A long outline is the case that used to bury the panel.
	var body strings.Builder
	for i := range 30 {
		fmt.Fprintf(&body, "\n## 章節 %d\n\ntext\n", i)
	}
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n" + body.String()
	rel := "Writing/lessons/japanese/L01.md"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, root, "add", rel)
	runGit(t, root, "commit", "-m", "seed lesson")
	srv := newServerWithContract(t, root, loadContract(t))

	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	railStart := strings.Index(page, `<aside class="y-rail-right"`)
	if railStart < 0 {
		t.Fatalf("page has no right rail; body = %q", page)
	}
	railEnd := strings.Index(page[railStart:], "</aside>")
	if railEnd < 0 {
		t.Fatalf("right rail is unterminated")
	}
	rail := page[railStart : railStart+railEnd]

	panelAt := strings.Index(rail, "y-statuspanel")
	outlineAt := strings.Index(rail, `<nav aria-label="本頁內容">`)
	if panelAt < 0 {
		t.Fatalf("right rail has no status panel; rail = %q", rail)
	}
	if outlineAt < 0 {
		t.Fatalf("this fixture must render an outline for the ordering to mean anything; rail = %q", rail)
	}
	if panelAt > outlineAt {
		t.Errorf("outline renders before the status panel (panel at %d, outline at %d); a long outline then pushes the transition controls out of view",
			panelAt, outlineAt)
	}
}

// TestFolderWithoutGitOffersNoTransition locks the fail-closed treatment of a
// folder that is no git repository. Every accepted transition is recorded as a
// commit, so a control offered there could only ever fail — and unlike an
// uncommitted edit, the reader cannot clear this from inside yomihon.
func TestFolderWithoutGitOffersNoTransition(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // deliberately not a git repository
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	rel := "Writing/lessons/japanese/L01.md"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — reading must keep working without git", code)
	}
	if n := strings.Count(page, `action="/status"`); n != 0 {
		t.Errorf("page offers %d transition control(s) in a folder with no git repository; each one can only fail", n)
	}
	if !strings.Contains(page, status.GitBlockReason) {
		t.Errorf("page does not say why the write face is closed; body = %q", page)
	}
}
