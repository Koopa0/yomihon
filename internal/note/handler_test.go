package note_test

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

func openReadingVault(t *testing.T, root string) *vaultfs.Reader {
	t.Helper()
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
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
) (*snapshot.Store, *vaultfs.Reader) {
	t.Helper()
	source := openReadingVault(t, root)
	store, err := snapshot.New(t.Context(), source, log, contract, governance)
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store, source
}

func openStatusWriter(
	t *testing.T,
	source *vaultfs.Reader,
	contract *schema.Contract,
	governance schema.Governance,
) *status.Writer {
	t.Helper()
	writer, err := status.Open(source, contract, governance, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("status.Open(, slog.New(slog.DiscardHandler)) error = %v", err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("Writer.Close() error = %v", err)
		}
	})
	return writer
}

// newServer wires the reading page against a real (not faked)
// status.Writer, with a nil contract (fail-closed). Good
// enough for tests whose point is that the page renders regardless of
// whether the write face is available (reading stays fail-open even when
// the write face is fail-closed) — NOT for exercising
// handler.go's NoFrontmatter/Transitions branch selection, since a
// fail-closed Writer supplies a write diagnostic and note.templ's statusPanel
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
	mux := http.NewServeMux()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, governance)
	writer := openStatusWriter(t, source, contract, governance)
	h := note.New(&note.Sources{
		Source:         source,
		Status:         writer.Authority,
		Snapshot:       store.Current,
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Log:            log,
	})
	h.Register(mux)
	status.NewHandler(writer, func() nav.Shell { return nav.Shell{} }, log).Register(mux)
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

	srv := newServerWithContract(t, root, loadContract(t))
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
		wording.NonInstanceReason.In(wording.ZhHant),
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
		"ui-status--ready",
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
	writer := openStatusWriter(t, source, contract, contract.Governance())
	statusCaptures := 0
	statusProvider := func() status.Authority {
		statusCaptures++
		return writer.Authority()
	}

	mux := http.NewServeMux()
	handler := note.New(&note.Sources{
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Source:         source,
		Status:         statusProvider,
		Snapshot:       store.Current,
		Log:            log,
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
			writer := openStatusWriter(t, source, contract, contract.Governance())

			var authority status.Authority
			var captured *snapshot.Generation
			if snapshotFirst {
				captured = store.Current().Capture()
			} else {
				authority = writer.Authority()
			}
			if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
				t.Fatalf("change contract between captures: %v", writeErr)
			}
			if snapshotFirst {
				authority = writer.Authority()
			} else {
				captured = store.Current().Capture()
			}

			mux := http.NewServeMux()
			note.New(&note.Sources{
				ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt,
				Source:         source,
				Status:         func() status.Authority { return authority },
				Snapshot:       func() *snapshot.Generation { return captured },
				Log:            log,
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
			writer := openStatusWriter(t, source, contract, contract.Governance())

			var authority status.Authority
			var captured *snapshot.Generation
			if snapshotFirst {
				captured = store.Current().Capture()
			} else {
				authority = writer.Authority()
			}
			if writeErr := os.WriteFile(contractPath, append(contractBytes, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
				t.Fatalf("change contract between captures: %v", writeErr)
			}
			if snapshotFirst {
				authority = writer.Authority()
			} else {
				captured = store.Current().Capture()
			}

			mux := http.NewServeMux()
			note.New(&note.Sources{
				ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt,
				Source:         source,
				Status:         func() status.Authority { return authority },
				Snapshot:       func() *snapshot.Generation { return captured },
				Log:            log,
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
		})
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
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())
	statusCaptures := 0
	mux := http.NewServeMux()
	note.New(&note.Sources{
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Source:         source,
		Status: func() status.Authority {
			statusCaptures++
			return writer.Authority()
		},
		Snapshot: store.Current,
		Log:      log,
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

			contract := loadHomeContractWithArtifactSection(t, tt.section)
			srv := newServerWithContract(t, root, contract)
			code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/Loud%20lesson.md")
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
			wantAbsent: wording.ContractUnavailable.In(wording.ZhHant),
		},
		{
			name: "artifact policy invalid",
			contract: func(t *testing.T) *schema.Contract {
				t.Helper()
				return loadHomeContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\".\"]\n")
			},
			want:       `invalid artifact policy: non_instance_dirs contains "."`,
			wantAbsent: wording.ContractUnavailable.In(wording.ZhHant),
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
	openAt := strings.Index(page, `<main id="main-content" tabindex="-1" class="y-main"`)
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
			body := "---\ntitle: 日本語\ntype: writing\ndomain: japanese\nstatus: draft\n" + tt.langLine + "---\n\n本文\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write note: %v", err)
			}
			srv := newServerWithContract(t, root, tt.contract(t))
			code, page := get(t, srv.URL+"/notes/Writing/%E6%97%A5%E6%9C%AC%E8%AA%9E.md")
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
// shared shell, the blocks this folder can fill, and the vault README rendered
// beneath them. The site markers are asserted by name rather than by their
// position in the document, so rearranging the dashboard cannot blame a correct
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
		"search":                            `data-home-block="search"`,
		"recent block":                      `data-home-block="recent"`,
		"link to the folder's introduction": `data-home-readme`,
		"topbar":                            `class="y-header"`,
		"command palette":                   `data-search`,
	} {
		if !strings.Contains(pageHTML, marker) {
			t.Errorf("GET / is missing the %s marker %q", name, marker)
		}
	}
	// The reader's own file is on the first screen of a folder that declares
	// nothing. It used to be filtered out for carrying no type — a field this
	// folder has never heard of — which left the block empty and the page
	// standing in for it.
	if !strings.Contains(pageHTML, `href="/notes/README.md" data-home-recent-note`) {
		t.Errorf("the recent block does not carry this folder's own file; body = %q", pageHTML)
	}
	// This folder carries no contract. It has no lifecycle vocabulary, so the
	// block that would name one is absent rather than empty or apologetic, and
	// nothing on the page reports a capability that was never claimed. Before
	// the grant split this page opened with seven such notices and the reader's
	// own README started below the fold.
	for name, marker := range map[string]string{
		"lifecycle block":   `data-home-block="lifecycle"`,
		"study-path block":  `data-home-block="study-paths"`,
		"empty-box notice":  "y-homeempty",
		"capability faults": `data-home-block="faults"`,
		"fault row":         "data-home-fault",
		"sidebar fault":     `data-sidebar-group="navigation-diagnostics"`,
	} {
		if strings.Contains(pageHTML, marker) {
			t.Errorf("ungoverned Home renders the %s marker %q", name, marker)
		}
	}
	// The introduction is a note with a page of its own. Home names it; it
	// does not reprint it, which used to be most of this screen.
	if strings.Contains(pageHTML, "README body sentinel.") {
		t.Error("Home reprints the folder's introduction instead of linking to it")
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
		"lifecycle":   `data-home-block="lifecycle"`,
		"study paths": `data-home-block="study-paths"`,
	} {
		if strings.Contains(page, marker) {
			t.Errorf("Home still renders the %s block under an unreadable contract", name)
		}
	}
	// Plain reading stays beside the fault: the recent list, the search box
	// and the README link never depended on a contract.
	for _, want := range []string{`data-home-block="recent"`, `data-home-block="search"`, "data-home-readme"} {
		if !strings.Contains(page, want) {
			t.Errorf("Home lost %q to a contract failure; reading never depends on one", want)
		}
	}
	// Withheld is not the same as empty. The stand-in line states a cheerful
	// fact about the folder, and beside a sentence explaining that projections
	// closed it would read as a second, contradictory account of the same hole.
	if strings.Contains(page, "data-home-standin") {
		t.Error("Home states what the folder holds beside the reason it cannot say what is in it")
	}
	// The subtitle names exactly the one content block on the page.
	if got := homeSubtitle(t, page); got != "查看最近變更。" {
		t.Errorf("subtitle = %q, want it to name the recent block and nothing else", got)
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
	writer := openStatusWriter(t, firstSource, nil, schema.Ungoverned())

	for _, tt := range []struct {
		name string
		path string
		want string
	}{
		// Home no longer reprints the introduction, so its needle is the link
		// the captured generation put in the navigation rather than a wikilink
		// resolved inside a body.
		{name: "home", path: "/", want: `href="/notes/Concepts/Target.md"`},
		{name: "note", path: "/notes/README.md", want: `<a href="/notes/Concepts/Target.md" class="wikilink">Target</a>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var current atomic.Pointer[snapshot.Generation]
			current.Store(firstStore.Current())
			calls := 0
			mux := http.NewServeMux()
			note.New(&note.Sources{
				ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt,
				Source:         firstSource,
				Status:         writer.Authority,
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
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())

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
				ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt,
				Source:         source,
				Status:         writer.Authority,
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
	pathBody := "---\ntitle: Test path\ntype: study-path\n---\n\n## Part {sequence=primary}\n\n- [[Open]]\n- [[Sealed]]\n"
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

	// The lifecycle block states the distribution: every status these notes
	// actually carry, with its live count, and nothing about who owns what.
	lifecycleBlock := homeSection(t, body, `data-home-block="lifecycle"`)
	for status, want := range map[string]int{"draft": 9, "ready": 1} {
		row := homeLifecycleRow(t, lifecycleBlock, status)
		if marker := `>` + strconv.Itoa(want) + `<`; !strings.Contains(row, marker) {
			t.Errorf("the %q row does not state %d; row = %q", status, want, row)
		}
	}
	if got := chipCounts(t, lifecycleBlock); len(got) != 2 {
		t.Errorf("the block lists %d statuses and the notes carry 2; counts = %v", len(got), got)
	}
	paths := homeSection(t, body, `data-home-block="study-paths"`)
	// The block says how big a course is, not how much of it is done. The
	// figure that used to sit here counted lessons awaiting a human's final
	// review, and publishing one moved it out of that status — so the number
	// fell as the work finished.
	if strings.Contains(paths, "已完成") {
		t.Errorf("the study-path block reports completion again; section = %q", paths)
	}
	for _, marker := range []string{"Test path", "2 課", "/syllabus/Maps/path.md"} {
		if !strings.Contains(paths, marker) {
			t.Errorf("Study paths block is missing %q", marker)
		}
	}
	if !strings.Contains(body, "data-home-readme") {
		t.Error("Home is missing the link to the folder's introduction")
	}
	if got := homeSubtitle(t, body); got != "查看最近變更、狀態分布，以及接下來的學習路徑。" {
		t.Errorf("subtitle = %q, want the sentence naming exactly the three blocks below it", got)
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
	writer := openStatusWriter(t, source, contract, contract.Governance())
	requestStatus := func() status.Authority {
		statusCaptures++
		return writer.Authority()
	}
	mux := http.NewServeMux()
	handler := note.New(&note.Sources{
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Source:         source,
		Status:         requestStatus,
		Snapshot:       store.Current,
		Log:            log,
	})
	handler.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	// The lifecycle block names only statuses with somewhere to go, and this
	// fixture's concept has nowhere to go but retirement, so the block is
	// legitimately empty here. Recent carries the same captured generation and
	// is what this test is about.
	section := html.UnescapeString(homeSection(t, body, `data-home-block="recent"`))
	if !strings.Contains(section, ">A<") {
		t.Errorf("captured-open recent block lost its instance projection: %q", section)
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
	// The adjudication surfaces close: the lifecycle distribution speaks for
	// the drifted authority and may not.
	if strings.Contains(page, `data-home-block="lifecycle"`) {
		t.Error("lifecycle block survived authority drift; a projection it cannot vouch for must be withheld")
	}
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
	// The row keeps the note's own frontmatter value, unjudged: hiding it
	// would show less than the file says, and judging it needs the vocabulary
	// that drifted.
	if !strings.Contains(recent, "seedling") {
		t.Errorf("the note's own status text is hidden after drift; section = %q", recent)
	}
	if strings.Contains(recent, "不在 schema 允許清單中") {
		t.Errorf("a drifted vocabulary flagged a value; section = %q", recent)
	}
	// The folder tree still lists the note: ordinary browsing never depended on
	// the artifact policy.
	if !strings.Contains(page, "/notes/Concepts/golang/A.md") {
		t.Error("ordinary folder browsing closed with the artifact authority")
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
			// The instance-derived blocks close on one cause, so the cause is
			// stated once and each closed block renders nothing at all.
			for _, marker := range []string{
				`data-home-block="lifecycle"`,
				`data-home-block="study-paths"`,
			} {
				if strings.Contains(body, marker) {
					t.Errorf("Home still renders %s without a usable artifact policy", marker)
				}
			}
			assertCauseStatedOncePerRegion(t, body, tt.want)
			// The recent list is plain reading and stays, without the
			// knowledge-layer citation: the citation belongs to the same
			// contract whose artifact declaration was refused.
			recent := homeSection(t, body, `data-home-block="recent"`)
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

func TestHomeValidPolicyExcludesNonInstancesFromRecent(t *testing.T) {
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
}

// TestHomeWithoutReadmeKeepsDashboardReadOnly pins first-use recovery: Home is
// still the dashboard, only the absent README body is replaced, and neither
// route creates the missing vault file. This vault declares a lifecycle and
// holds no notes yet, so nothing waits and no content block renders; the
// stand-in line and the search block carry the page.
func TestHomeWithoutReadmeKeepsDashboardReadOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// The contract declares a usable egress policy, because this test is about
	// a vault with nothing in it rather than a vault whose contract left an
	// input out — the second has something to say and would fail the emptiness
	// assertions below for a reason that has nothing to do with them.
	srv := newServerWithContract(t, root, contractWithPrivacySection(t, "[privacy]\nnever_egress_dirs = []\n"))

	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / without README status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(body, `data-home-block="search"`) {
		t.Error(`GET / without README is missing the search block`)
	}
	for _, absent := range []string{
		`data-home-block="recent"`,
		`data-home-block="lifecycle"`,
		`data-home-block="study-paths"`,
		"y-homeempty",
	} {
		if strings.Contains(body, absent) {
			t.Errorf("GET / without README still renders %q over a vault that has nothing to put in it", absent)
		}
	}

	if got := homeSubtitle(t, body); got != "" {
		t.Errorf("subtitle = %q; it must not promise blocks the page does not carry", got)
	}

	noteCode, _ := get(t, srv.URL+"/notes/README.md")
	if noteCode != http.StatusNotFound {
		t.Errorf("GET /notes/README.md without README status = %d, want %d", noteCode, http.StatusNotFound)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Errorf("README.md after recovery requests: os.Stat error = %v, want not-exist", err)
	}
}

// homeSubtitle returns the line under Home's title, or "" when the page carries
// none. It reads the header rather than the whole document so a sentence
// elsewhere on the page cannot answer for it.
func homeSubtitle(t *testing.T, body string) string {
	t.Helper()
	const opener = `<header class="y-home__head">`
	start := strings.Index(body, opener)
	if start < 0 {
		t.Fatalf("Home body has no header; body = %q", body)
	}
	header := body[start:]
	end := strings.Index(header, "</header>")
	if end < 0 {
		t.Fatalf("Home header is never closed; body = %q", body)
	}
	header = header[:end]
	open := strings.Index(header, "<p>")
	if open < 0 {
		return ""
	}
	end = strings.Index(header[open:], "</p>")
	if end < 0 {
		t.Fatalf("Home subtitle is never closed; header = %q", header)
	}
	return header[open+len("<p>") : open+end]
}

// homeStandInLine returns the stand-in line's markup. It is a paragraph rather
// than a block, which is the whole point of it, so the section reader cannot
// find it.
func homeStandInLine(t *testing.T, body string) string {
	t.Helper()
	const opener = `<p class="y-homestandin"`
	start := strings.Index(body, opener)
	if start < 0 {
		t.Fatalf("Home body carries no stand-in line; body = %q", body)
	}
	line := body[start:]
	end := strings.Index(line, "</p>")
	if end < 0 {
		t.Fatalf("the stand-in line is never closed; body = %q", body)
	}
	return line[:end+len("</p>")]
}

// TestHomeStandInNamesTheNewestFileNotTheNewestNote is the stand-in line's own
// lock. The line exists because a folder with no contract fills none of the
// dashboard blocks and never will, and it earns its place by doing in one row
// what the recently-changed block was built to do. It therefore has to answer
// over everything the folder holds: a plain folder's newest thing is often not
// a note, and a line that quietly skipped to the newest note would point past
// the file the reader just saved.
// TestHomeStandInNamesTheNewestFileWhenThereAreNoNotes covers the one folder
// shape that has nothing for any block to show: files, but no markdown. The
// stand-in used to cover a much commoner case — a folder whose notes carried no
// type field — and that case now fills the recent block with the notes
// themselves, which is what the reader came for.
func TestHomeStandInNamesTheNewestFileWhenThereAreNoNotes(t *testing.T) {
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
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	standIn := homeStandInLine(t, body)
	for _, want := range []string{
		"這個資料夾有 3 個檔案",
		`href="/notes/todo.txt"`,
		">todo.txt</a>",
		"2026-07-03",
	} {
		if !strings.Contains(standIn, want) {
			t.Errorf("the stand-in line is missing %q; line = %q", want, standIn)
		}
	}
	if strings.Contains(standIn, "older.txt") || strings.Contains(standIn, "reading.html") {
		t.Errorf("the stand-in line names something other than the newest file; line = %q", standIn)
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

// distributionContract loads a contract whose lifecycle covers the shapes the
// distribution block must be indifferent to: the owner names differ from row
// to row, ready moves only to published, and a guide moves through a status
// group the default vocabulary never mentions. The owner lists are inert
// vault-side data; nothing below reads them.
func distributionContract(t *testing.T) *schema.Contract {
	t.Helper()
	text := `schema_version = "1"

[enums]
type = ["doc", "guide"]

[enums.status]
note = ["inbox", "draft", "ready", "published"]
system = ["proposed", "active"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[fields.status_group]
system = ["guide"]

[scan]
knowledge_dirs = ["Writing"]
skip_basenames = ["README.md"]

[artifacts]
non_instance_dirs = ["System/templates"]

[[lifecycle]]
status = "inbox"
applies_to = ["doc"]
from = []
owner = ["agent"]

[[lifecycle]]
status = "draft"
applies_to = ["doc"]
from = ["inbox"]
owner = ["agent"]

[[lifecycle]]
status = "ready"
applies_to = ["doc"]
from = ["draft"]
owner = ["reviewer"]

[[lifecycle]]
status = "published"
applies_to = ["doc"]
from = ["ready"]
owner = ["reviewer"]

[[lifecycle]]
status = "proposed"
applies_to = ["guide"]
from = []
owner = ["agent"]

[[lifecycle]]
status = "active"
applies_to = ["guide"]
from = ["proposed"]
owner = ["reviewer"]
`
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	err := os.WriteFile(path, []byte(text), 0o600) // #nosec G703 -- path is a fixed basename under this test's TempDir
	if err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}

// TestHomeLifecycleCountsStatusDistribution pins the home block to its one
// source: the snapshot's per-status note counts. Every status at least one
// note carries gets its chip — whoever the contract says owns the onward
// step, and terminal values included, because a distribution that hides a
// bucket disagrees with its own total. A note carrying no status value sits
// in no bucket, and a vault whose notes carry none renders no block at all.
func TestHomeLifecycleCountsStatusDistribution(t *testing.T) {
	t.Parallel()

	type note struct{ path, front string }
	doc := func(name, status string) note {
		return note{
			path:  "Writing/" + name + ".md",
			front: "title: " + name + "\ntype: doc\nstatus: " + status,
		}
	}
	tests := []struct {
		name     string
		notes    []note
		wantRows map[string]int
	}{
		{
			name: "an agent-owned onward step does not hide a status",
			notes: []note{
				doc("D1", "inbox"),
				doc("D2", "draft"), doc("D3", "draft"),
			},
			wantRows: map[string]int{"inbox": 1, "draft": 2},
		},
		{
			name: "terminal statuses show what exists",
			notes: []note{
				doc("D1", "ready"),
				doc("D2", "published"),
			},
			wantRows: map[string]int{"ready": 1, "published": 1},
		},
		{
			name: "a status only another group declares",
			notes: []note{
				doc("D1", "draft"),
				{path: "System/agent-guides/G1.md", front: "title: G1\ntype: guide\nstatus: proposed"},
			},
			wantRows: map[string]int{"draft": 1, "proposed": 1},
		},
		{
			name: "a note without a status is accounted for outside the statuses",
			notes: []note{
				doc("D1", "draft"),
				{path: "Writing/D2.md", front: "title: D2\ntype: doc"},
			},
			wantRows: map[string]int{"draft": 1},
		},
		{
			name: "no status values render no block",
			notes: []note{
				{path: "Writing/D1.md", front: "title: D1\ntype: doc"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
				t.Fatalf("write README: %v", err)
			}
			for _, n := range tt.notes {
				full := filepath.Join(root, filepath.FromSlash(n.path))
				if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				body := "---\n" + n.front + "\n---\n\nbody\n"
				if err := os.WriteFile(full, []byte(body), 0o600); err != nil { // #nosec G703 -- a fixed fixture path under this test's TempDir
					t.Fatalf("write %s: %v", n.path, err)
				}
			}
			srv := newServerWithContract(t, root, distributionContract(t))

			code, body := get(t, srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}

			if len(tt.wantRows) == 0 {
				if strings.Contains(body, `data-home-block="lifecycle"`) {
					t.Error("Home rendered a distribution block with nothing to count")
				}
				// The subtitle must not promise a block that is not below it.
				if strings.Contains(body, "狀態分布") {
					t.Error("the subtitle names the distribution block while the page carries none")
				}
				return
			}
			block := homeSection(t, body, `data-home-block="lifecycle"`)
			counts := chipCounts(t, block)
			if len(counts) == 0 {
				t.Fatal("the fixture produced nothing to count, so this proves nothing")
			}
			for _, n := range counts {
				if n == 0 {
					t.Errorf("the block lists a status no note carries; counts = %v", counts)
				}
			}
			if len(counts) != len(tt.wantRows) {
				t.Errorf("the block lists %d statuses and the notes carry %d; counts = %v, want %v", len(counts), len(tt.wantRows), counts, tt.wantRows)
			}
			for status, want := range tt.wantRows {
				row := homeLifecycleRow(t, block, status)
				if marker := `>` + strconv.Itoa(want) + `<`; !strings.Contains(row, marker) {
					t.Errorf("the %q row does not state %d; row = %q", status, want, row)
				}
			}
			if marker := `href="/search?q=` + url.QueryEscape("status:") + `"`; strings.Contains(block, marker) {
				t.Errorf("the block carries a chip for the absent status value; block = %q", block)
			}
		})
	}
}

// homeLifecycleRow returns the one chip anchor for statusName inside the
// distribution block's markup.
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

// chipCounts reads every per-status figure the block states.
func chipCounts(t *testing.T, block string) []int {
	t.Helper()
	var out []int
	const marker = `class="y-homechip__count"`
	for rest := block; ; {
		at := strings.Index(rest, marker)
		if at < 0 {
			return out
		}
		rest = rest[at:]
		open := strings.IndexByte(rest, '>')
		end := strings.Index(rest, "</span>")
		if open < 0 || end < 0 || end < open {
			t.Fatalf("a chip count is malformed: %q", rest[:80])
		}
		n, err := strconv.Atoi(strings.TrimSpace(rest[open+1 : end]))
		if err != nil {
			t.Fatalf("a chip count is not a number: %v", err)
		}
		out = append(out, n)
		rest = rest[end:]
	}
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
	if strings.Contains(body, wording.ContractUnavailable.In(wording.ZhHant)) || strings.Contains(body, "fail-closed") {
		t.Errorf("page shows the fail-closed notice even though the contract loaded; body = %q", body)
	}
}

// TestShowTransitions exercises handler.go's default branch (view.Transitions
// = authority.Transitions(n.RelPath, n.Type(), n.Status())) with a loaded contract.
// Getting the argument order backwards (Transitions(current, noteType)) or
// swapping the switch's case order would silently render the wrong panel —
// this test is the only one in the repo that would catch either.
func TestShowTransitions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	contract := loadContract(t)
	srv := newServerWithContract(t, root, contract)

	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	// draft -> [ready, archived] per testdata/contract.toml's lifecycle table
	// (cross-checked by hand, mirroring the status package's TestTransitions).
	transitionSource := openReadingVault(t, root)
	transitions := openStatusWriter(t, transitionSource, contract, contract.Governance()).Authority().Transitions("Writing/lessons/japanese/L01.md", "lesson", "draft")
	if len(transitions) != 2 {
		t.Fatalf("Transitions() = %v, want two targets", transitions)
	}
	if !slices.Contains(transitions, schema.SealStatus) {
		t.Fatalf("Transitions() = %v; fixture must keep a draft edge to %q for this test", transitions, schema.SealStatus)
	}
	for _, target := range transitions {
		want := `value="` + target + `"`
		if !strings.Contains(body, want) {
			t.Errorf("page missing transition key %s; body = %q", want, body)
		}
	}
	// Drive the page's own ready-target form: the transition POSTed below
	// carries exactly the hidden fields the page rendered, so a drift between
	// the two would fail here rather than on a hand-built request.
	beforeTarget, _, found := strings.Cut(body, `name="to" value="`+schema.SealStatus+`"`)
	if !found {
		t.Fatalf("page missing a transition form for contract target %q", schema.SealStatus)
	}
	start := strings.LastIndex(beforeTarget, "<form")
	if start < 0 {
		t.Fatalf("transition target %q is not inside a form; body = %q", schema.SealStatus, body)
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatalf("transition form is unterminated; body = %q", body[start:])
	}
	readyForm := body[start : start+end]
	form := url.Values{
		"path":             {hiddenValue(t, readyForm, "path")},
		"from":             {hiddenValue(t, readyForm, "from")},
		"to":               {hiddenValue(t, readyForm, "to")},
		"content_identity": {hiddenValue(t, readyForm, "content_identity")},
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
	// The target names the status the note just left, which is what the
	// reading page states the change from.
	if got, want := resp.Header.Get("Location"), "/notes/Writing/lessons/japanese/L01.md?from=draft"; got != want {
		t.Errorf("POST /status Location = %q, want %q", got, want)
	}
	// Follow the redirect the way a browser does and read what the reader is
	// actually told. Asserting the receipt on a hand-built NoteView proves the
	// template can render it; only this proves the page is handed the value.
	landingCode, landing := get(t, srv.URL+resp.Header.Get("Location"))
	if landingCode != http.StatusOK {
		t.Fatalf("GET the redirect target = %d, want 200", landingCode)
	}
	for _, want := range []string{`role="status"`, "狀態已從", ">draft<", ">" + schema.SealStatus + "<"} {
		if !strings.Contains(landing, want) {
			t.Errorf("the page a successful flip lands on does not state the change (%q missing)", want)
		}
	}
	// A plain reading of the same note states nothing.
	_, plain := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if strings.Contains(plain, "狀態已從") {
		t.Errorf("an ordinary reading carries a transition receipt")
	}

	// A hand-typed origin cannot manufacture one. The value arrives in the URL
	// where anything can put anything, so the page checks it against the same
	// contract the write face obeys before repeating it: a status the contract
	// never declared, and one it declares but legalises no move from, each buy
	// nothing. Without this the address bar could announce a transition that
	// never happened — including to published, which this write face refuses
	// to perform at all.
	for _, forged := range []string{"?from=nonsense", "?from=" + schema.PublishedStatus, "?from=archived"} {
		_, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md"+forged)
		if strings.Contains(page, "狀態已從") {
			t.Errorf("%s made the page announce a transition that never happened", forged)
		}
	}

	got, err := os.ReadFile(filepath.Join(dir, "L01.md")) // #nosec G304 -- dir is under t.TempDir and the filename is fixed by this test
	if err != nil {
		t.Fatalf("read flipped lesson: %v", err)
	}
	want := strings.Replace(lessonMD, "status: draft", "status: "+schema.SealStatus, 1)
	if string(got) != want {
		t.Errorf("lesson after POST differs outside the one status line:\ngot:  %q\nwant: %q", got, want)
	}
	if strings.Contains(body, wording.ContractUnavailable.In(wording.ZhHant)) || strings.Contains(body, "fail-closed") || strings.Contains(body, "沒有 frontmatter") {
		t.Errorf("page shows the wrong status-panel branch; body = %q", body)
	}
}

// transitionFormFor cuts the transition form whose hidden "to" field carries
// target out of a rendered page body.
func transitionFormFor(t *testing.T, body, target string) string {
	t.Helper()
	before, _, found := strings.Cut(body, `name="to" value="`+target+`"`)
	if !found {
		t.Fatalf("page has no transition form for target %q", target)
	}
	start := strings.LastIndex(before, "<form")
	if start < 0 {
		t.Fatalf("transition target %q is not inside a form", target)
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatalf("transition form for %q is unterminated", target)
	}
	return body[start : start+end]
}

// flipViaPage drives the page's own transition form for target and returns
// the redirect Location, so the POST carries exactly the hidden fields the
// page rendered.
func flipViaPage(t *testing.T, srv *httptest.Server, pageBody, target string) string {
	t.Helper()
	form := transitionFormFor(t, pageBody, target)
	values := url.Values{
		"path":             {hiddenValue(t, form, "path")},
		"from":             {hiddenValue(t, form, "from")},
		"to":               {hiddenValue(t, form, "to")},
		"content_identity": {hiddenValue(t, form, "content_identity")},
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/status", strings.NewReader(values.Encode()))
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
	return resp.Header.Get("Location")
}

// TestFlipReceiptRequiresTheWriteItReports holds the receipt to the one
// reading a successful flip redirects to. The origin arrives in the URL,
// where anything can put anything: before this lock, a note born ready — no
// POST ever made — printed "the status changed from draft" on every load of
// ?from=draft, because the sentence was gated only on the contract admitting
// such a move. The receipt must be spent by the write that mints it: shown on
// the arrival that follows the redirect, gone on refresh, and never available
// to a hand-typed address.
func TestFlipReceiptRequiresTheWriteItReports(t *testing.T) {
	t.Parallel()
	const receiptMarker = "狀態已從"

	t.Run("a hand typed origin on a note never flipped prints nothing", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeLesson(t, root, lessonWithStatus(schema.SealStatus))
		srv := newServerWithContract(t, root, loadContract(t))

		// draft -> ready is a move the contract admits, which used to be the
		// whole gate.
		_, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md?from=draft")
		if strings.Contains(page, receiptMarker) {
			t.Errorf("a hand typed ?from=draft printed a receipt for a write that never happened")
		}
	})

	t.Run("the redirected reading shows the receipt once", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeLesson(t, root, lessonWithStatus("draft"))
		srv := newServerWithContract(t, root, loadContract(t))

		_, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
		location := flipViaPage(t, srv, page, schema.SealStatus)
		landingCode, landing := get(t, srv.URL+location)
		if landingCode != http.StatusOK {
			t.Fatalf("GET the redirect target = %d, want 200", landingCode)
		}
		if !strings.Contains(landing, receiptMarker) {
			t.Fatalf("the reading a flip redirects to does not state the change")
		}
		_, refreshed := get(t, srv.URL+location)
		if strings.Contains(refreshed, receiptMarker) {
			t.Errorf("reloading the same address printed the receipt again")
		}
	})

	t.Run("two flips of one note each buy exactly one receipt", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeLesson(t, root, lessonWithStatus("draft"))
		srv := newServerWithContract(t, root, loadContract(t))

		_, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
		firstLocation := flipViaPage(t, srv, page, schema.SealStatus)
		_, firstLanding := get(t, srv.URL+firstLocation)
		if !strings.Contains(firstLanding, receiptMarker) {
			t.Fatalf("the first flip's landing does not state the change")
		}

		secondLocation := flipViaPage(t, srv, firstLanding, "archived")
		_, secondLanding := get(t, srv.URL+secondLocation)
		if !strings.Contains(secondLanding, receiptMarker) {
			t.Fatalf("the second flip's landing does not state the change")
		}
		for name, address := range map[string]string{
			"the first flip's address":  firstLocation,
			"the second flip's address": secondLocation,
		} {
			_, page := get(t, srv.URL+address)
			if strings.Contains(page, receiptMarker) {
				t.Errorf("revisiting %s printed a receipt again", name)
			}
		}
	})
}

// writeLesson writes the one lesson fixture these tests read, L01.md under
// the contract's knowledge directory.
func writeLesson(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// lessonWithStatus is a minimal legal lesson note carrying one status line.
func lessonWithStatus(noteStatus string) string {
	return "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: " + noteStatus +
		"\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
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
			name:  "status view",
			clear: func(d *note.Sources) { d.Status = nil },
			want:  "note: New requires a non-nil Status",
		},
		{
			name:  "snapshot provider",
			clear: func(d *note.Sources) { d.Snapshot = nil },
			want:  "note: New requires a non-nil Snapshot provider",
		},
		{
			name:  "observed status provider",
			clear: func(d *note.Sources) { d.ObservedStatus = nil },
			want:  "note: New requires a non-nil ObservedStatus provider",
		},
		{
			name:  "consume receipt provider",
			clear: func(d *note.Sources) { d.ConsumeReceipt = nil },
			want:  "note: New requires a non-nil ConsumeReceipt provider",
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
			writer := openStatusWriter(t, source, nil, schema.Ungoverned())
			deps := note.Sources{
				ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt,
				Source:         source,
				Status:         writer.Authority,
				Snapshot:       store.Current,
				Log:            log,
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
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())
	deps := note.Sources{
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Source:         source,
		Status:         writer.Authority,
		Snapshot:       store.Current,
		Log:            log,
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
	lessonMD := "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	syllabus := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字 {sequence=primary}\n\n- [[Slices]]\n"
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

// TestReadingPrecedesTheRulingInTheRail locks the right rail's order. The rail
// is its own scroll container, so whatever renders first decides what a reader
// sees there without scrolling, and what they came for is the note: its own
// shape first, then what leads to it from elsewhere in the vault. The ruling is
// a small verb beside the reading rather than the frame around it, so it takes
// the last position and is reached by scrolling the rail like everything else
// in it. The panel opened the rail before this, which put the smallest control
// on the page above both of the things the reader was reading.
func TestReadingPrecedesTheRulingInTheRail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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
	citedAt := strings.Index(rail, "y-citedby")
	if panelAt < 0 {
		t.Fatalf("right rail has no status panel; rail = %q", rail)
	}
	if outlineAt < 0 {
		t.Fatalf("this fixture must render an outline for the ordering to mean anything; rail = %q", rail)
	}
	// The rail draws only what a note has, and this note cites nobody, so the
	// middle block is held where it appears rather than demanded here. The
	// browser probe drives a note that has one.
	type block struct {
		what string
		at   int
	}
	var order []block
	for _, b := range []block{
		{"the note's own shape", outlineAt},
		{"what leads to this note", citedAt},
		{"the ruling", panelAt},
	} {
		if b.at >= 0 {
			order = append(order, b)
		}
	}
	for i := 1; i < len(order); i++ {
		if order[i-1].at > order[i].at {
			t.Errorf("%s renders after %s in the rail (%d against %d); reading comes first there and the verb comes last",
				order[i-1].what, order[i].what, order[i-1].at, order[i].at)
		}
	}
}

// TestReadingPageShowsTheStatusTheFileCarriesNow covers the moment the write
// face exists for: the reader presses a transition and lands on the note. The
// scan behind every other part of the page is up to a couple of seconds old, so
// a status taken from it is the value the reader has just moved away from — and
// the control offered beside it transitions from a state the note has left,
// which the write path then refuses. The file is written directly here rather
// than through a POST because what is under test is the read.
func TestReadingPageShowsTheStatusTheFileCarriesNow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := "Writing/lessons/japanese/L01.md"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	notePath := filepath.Join(dir, "L01.md")
	const header = "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: "
	const footer = "\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	if err := os.WriteFile(notePath, []byte(header+"draft"+footer), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	if code, page := get(t, srv.URL+"/notes/"+rel); code != http.StatusOK {
		t.Fatalf("first read status = %d, want 200", code)
	} else if !strings.Contains(page, `value="draft"`) {
		t.Fatalf("the note starts at draft but the page does not say so; body = %q", page)
	}

	// Change the file behind the scan's back, exactly as a completed write does,
	// and read again well inside the scan interval.
	if err := os.WriteFile(notePath, []byte(header+"ready"+footer), 0o600); err != nil {
		t.Fatalf("rewrite status: %v", err)
	}
	code, page := get(t, srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("second read status = %d, want 200", code)
	}
	if strings.Contains(page, `value="draft"`) {
		t.Errorf("the page still offers a transition from draft, which the note has left; the write path would refuse it")
	}
	if !strings.Contains(page, `value="ready"`) {
		t.Errorf("the page does not carry the status the file now holds; body = %q", page)
	}
	// Which transitions are offered is the other half of the same read. The
	// contract lets draft reach ready and archived, and ready reach archived
	// alone, so a page still offering ready is one computing the menu from the
	// state the note left.
	if strings.Contains(page, `name="to" value="ready"`) {
		t.Errorf("the page still offers a transition into ready, which the note is already in")
	}
	if !strings.Contains(page, `name="to" value="archived"`) {
		t.Errorf("the page offers no transition at all from ready; body = %q", page)
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
		Source:         source,
		Status:         func() status.Authority { return status.Authority{} },
		Snapshot:       store.Current,
		ObservedStatus: func(context.Context, string) (string, error) { return "", nil },
		ConsumeReceipt: func(string, string) bool { return false },
		Log:            slog.New(slog.DiscardHandler),
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, f := range files {
		t.Run(f.rel, func(t *testing.T) {
			t.Parallel()
			code, page := get(t, srv.URL+"/notes/"+f.rel)
			if code != http.StatusOK {
				t.Fatalf("GET /notes/%s status = %d, want 200", f.rel, code)
			}
			shown := strings.Contains(page, `class="y-prose y-source"`)
			if shown != f.wantText {
				t.Errorf("GET /notes/%s renders its characters = %v, want %v", f.rel, shown, f.wantText)
			}
			results, _, err := store.Current().Search().SearchN(lexical.Parse(f.term), -1)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", f.term, err)
			}
			found := len(results) == 1 && results[0].RelPath == f.rel
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
			code, body := get(t, srv.URL+"/")
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

// TestFuriganaControlAppearsOnlyWhereThereIsFurigana covers a control for a
// capability the page does not have. The button switches readings off; a folder
// that holds no Japanese has none to switch, and shipping it anyway is where
// the tool's own history shows through its chrome to a reader who has no idea
// what 振 means.
func TestFuriganaControlAppearsOnlyWhereThereIsFurigana(t *testing.T) {
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
		name string
		path string
		want bool
	}{
		{name: "a note carrying readings", path: "/notes/japanese.md", want: true},
		{name: "a note carrying none", path: "/notes/plain.md", want: false},
		{name: "home", path: "/", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.URL+tt.path)
			if code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tt.path, code)
			}
			if got := strings.Contains(body, "data-ruby-toggle"); got != tt.want {
				t.Errorf("GET %s carries the furigana control = %t, want %t", tt.path, got, tt.want)
			}
			if strings.Contains(body, "<ruby") != tt.want {
				t.Errorf("GET %s: the fixture does not match what the test assumes", tt.path)
			}
		})
	}
}

// TestAFileTooLargeToSearchSaysSoOnItsOwnPage covers the only thing that makes
// a bound honest. A note past the cap still renders — every file in the folder
// stays readable — but nothing reaches it by search, and a search that answers
// "no results" for a phrase sitting in a note the reader is looking at is a
// false statement about the folder. The cap already existed for every file kind
// except the one held three times over.
func TestAFileTooLargeToSearchSaysSoOnItsOwnPage(t *testing.T) {
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

	// It renders, and it says why a search will not find it.
	code, page := get(t, srv.URL+"/notes/huge.md")
	if code != http.StatusOK {
		t.Fatalf("GET the oversize note = %d, want 200 — reading is never withheld", code)
	}
	if !strings.Contains(page, "sits here too") {
		t.Error("the oversize note did not render its own body")
	}
	if !strings.Contains(page, "data-note-unsearchable") {
		t.Error("the oversize note is absent from the index and its page does not say so")
	}

	// The note that is indexed says nothing of the kind.
	if _, small := get(t, srv.URL+"/notes/small.md"); strings.Contains(small, "data-note-unsearchable") {
		t.Error("a note that is searchable was told it is not")
	}

	// That the index really leaves it out is asserted where the index lives:
	// TestAnOversizeNoteRendersAndStaysOutOfTheIndex in internal/snapshot.
}

// TestFolderWithoutARepositoryStillOffersTransitions locks the reading page to
// the same rule as the write path: a governed folder that is no git repository
// offers its legal transitions like any other, because a flip is a plain file
// rewrite and needs no repository.
func TestFolderWithoutARepositoryStillOffersTransitions(t *testing.T) {
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
		t.Fatalf("status = %d, want 200", code)
	}
	if n := strings.Count(page, `action="/status"`); n == 0 {
		t.Errorf("page offers no transition control in a plain folder; a flip needs no repository")
	}
	if !strings.Contains(page, `name="to" value="`+schema.SealStatus+`"`) {
		t.Errorf("page is missing the draft note's onward transition; body = %q", page)
	}
}

// TestTransitionsArePlainControls locks the write face's plain shape: every
// legal transition renders as an ordinary one-click form with no hold
// affordance, no operator line, and no confirmation toast, and a note at the
// accented ready status shows the same plain chip as any other value rather
// than a stamp or a certification line.
func TestTransitionsArePlainControls(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status string
		want   []string
	}{
		{
			name:   "draft note offers one-click forms",
			status: "draft",
			want: []string{
				`name="to" value="` + schema.SealStatus + `"`,
				`name="to" value="archived"`,
				"ui-status--draft",
			},
		},
		{
			name:   "ready note keeps the plain chip and its onward form",
			status: schema.SealStatus,
			want: []string{
				"ui-status--" + schema.SealStatus,
				`name="to" value="archived"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: " + tt.status + "\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
			dir := filepath.Join(root, "Writing", "lessons", "japanese")
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			srv := newServerWithContract(t, root, loadContract(t))
			code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			for _, want := range tt.want {
				if !strings.Contains(page, want) {
					t.Errorf("page is missing %q", want)
				}
			}
			for _, absent := range []string{
				"data-seal",
				"y-sealbtn",
				"y-sealfill",
				"y-sealpoem",
				"按住",
				"操作者",
				"y-sealed",
				"鈐印",
				"済",
			} {
				if strings.Contains(page, absent) {
					t.Errorf("page still carries retired markup %q", absent)
				}
			}
		})
	}
}

// TestShowFlagsAStatusOutsideTheSchema locks the reading page's answer to a
// status value the contract never declared. An empty transition set is true
// of such a value but misleading: the schema did not define nothing onward —
// it never defined the value at all. The panel states that fact instead,
// wall-4 voiced: yomihon reports, a human edits the file.
func TestShowFlagsAStatusOutsideTheSchema(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: 這是草稿\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		wording.StatusValuePrefix.In(wording.ZhHant) + "<code>這是草稿</code> " + wording.StatusOutsideList.In(wording.ZhHant),
		"ui-status--這是草稿",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	for _, absent := range []string{
		wording.NoLegalTransitions.In(wording.ZhHant),
		`action="/status"`,
	} {
		if strings.Contains(page, absent) {
			t.Errorf("page unexpectedly contains %q", absent)
		}
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
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))
	code, page := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
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

// TestShowOffersTransitionsWhateverTheOwnerLists locks the reading page to
// the same demotion: the buttons come from the from-lists alone, so a
// contract whose onward stages name only another owner still renders each of
// them as a plain control, and the page never claims the schema defines
// nothing onward.
func TestShowOffersTransitionsWhateverTheOwnerLists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// The note sits inside the knowledge layer the contract declares, which is
	// where the lifecycle reaches a note at all.
	if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
		t.Fatalf("make the knowledge directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Writing", "Note.md"), []byte("---\ntitle: Note\ntype: writing\nstatus: draft\n---\n\nbody\n"), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	// Every lifecycle edge moves to a different owner, so from draft the
	// contract still defines onward transitions — ready and archived — while
	// naming nobody this tool ever wrote as.
	rewritten := strings.ReplaceAll(string(base), `owner = ["claude", "koopa"]`, `owner = ["alice"]`)
	rewritten = strings.ReplaceAll(rewritten, `owner = ["koopa"]`, `owner = ["alice"]`)
	if strings.Contains(rewritten, "koopa") {
		t.Fatal("the owner rewrite left an edge behind; the fixture contract changed shape")
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(contractPath, []byte(rewritten), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under t.TempDir
		t.Fatalf("write rewritten contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("LoadFile(rewritten contract) = %v", err)
	}

	srv := newServerWithContract(t, root, contract)
	code, body := get(t, srv.URL+"/notes/Writing/Note.md")
	if code != http.StatusOK {
		t.Fatalf("GET note status = %d, want 200", code)
	}
	page := html.UnescapeString(body)

	for _, want := range []string{`name="to" value="ready"`, `name="to" value="archived"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing the from-list transition %q", want)
		}
	}
	if strings.Contains(page, "接下來的狀態轉換由其他 owner 持有") {
		t.Error("page still words an owner boundary that no longer exists")
	}
}

// TestShowNamesTheLayerThatWithheldTheControls holds the reading page to the
// truth about an empty transition set. A note outside the knowledge layer the
// contract declares is offered nothing, and the page has to say that this is
// why: the sentence for a schema that defines nothing onward would be false
// here, because the contract does define moves from draft for this note's
// type. Both status faces carry the sentence, counted rather than found,
// because the bar is the only face at the widths that drop the rail. The same
// bytes inside the layer keep their forms, so the path is the whole difference.
func TestShowNamesTheLayerThatWithheldTheControls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := "---\ntitle: L05\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	const outside = "System/agent-guides/L05.md"
	const inside = "Writing/lessons/japanese/L05.md"
	for _, rel := range []string{outside, inside} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", full, err)
		}
	}
	srv := newServerWithContract(t, root, loadContract(t))
	layer := wording.OutsideKnowledgeScope.In(wording.ZhHant)

	code, page := get(t, srv.URL+"/notes/"+outside)
	if code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", outside, code)
	}
	page = html.UnescapeString(page)
	if got := strings.Count(page, layer); got != 2 {
		t.Errorf("the layer sentence appears %d times on the page outside the layer, want 2: one per status face", got)
	}
	if strings.Contains(page, wording.NoLegalTransitions.In(wording.ZhHant)) {
		t.Error("the page claims the schema defines nothing onward, for a note the layer withheld")
	}
	if strings.Contains(page, `name="to"`) {
		t.Error("the page offers a transition form outside the knowledge layer")
	}

	code, page = get(t, srv.URL+"/notes/"+inside)
	if code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", inside, code)
	}
	page = html.UnescapeString(page)
	if !strings.Contains(page, `name="to" value="ready"`) {
		t.Error("the same note inside the layer lost its transition form")
	}
	if strings.Contains(page, layer) {
		t.Error("the page inside the layer names the layer as withholding anything")
	}
}

// TestHomeLifecycleBlockDropsTheQueueHeading pins the block's voice: it
// counts statuses, so the heading that claimed a queue may not return. A
// lesson at draft is the plainest fixture — one status, one note, one chip.
func TestHomeLifecycleBlockDropsTheQueueHeading(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Vault\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	block := homeSection(t, body, `data-home-block="lifecycle"`)
	if row := homeLifecycleRow(t, block, "draft"); !strings.Contains(row, ">1<") {
		t.Errorf("the draft row does not state its count; row = %q", row)
	}
	if !strings.Contains(block, "依狀態分組") {
		t.Error("the block does not carry the distribution heading")
	}
	for _, phrase := range []string{"等你處理", "待判讀內容"} {
		if strings.Contains(body, phrase) {
			t.Errorf("the page still claims a queue: %q", phrase)
		}
	}
}

// TestHomeLifecycleAccountsForEveryIndexedNote is the structural claim the
// block makes once it carries cells for notes holding no status: every note
// the folder counts is somewhere in it. A note that left the block entirely is
// how a distribution came to disagree with the number of notes it was a
// distribution of, and no per-status assertion can see that — only the sum can.
//
// The four shapes are the ones that divide here: a note with a declared
// status, one whose frontmatter cannot be parsed, one carrying no frontmatter
// at all, and one whose frontmatter reads and states no status. The last two
// are legal and the second is not, which is why they are not one cell.
func TestHomeLifecycleAccountsForEveryIndexedNote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, n := range []struct{ path, body string }{
		{"Writing/Declared.md", "---\ntitle: Declared\ntype: doc\nstatus: draft\n---\n\nbody\n"},
		{"Writing/Unparsable.md", "---\ntitle: Unparsable\ntype: doc\nstatus: draft\nstatus: ready\n---\n\nbody\n"},
		{"Writing/Bare.md", "no frontmatter at all\n"},
		{"Writing/Silent.md", "---\ntitle: Silent\ntype: doc\n---\n\nbody\n"},
	} {
		full := filepath.Join(root, filepath.FromSlash(n.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", n.path, err)
		}
		if err := os.WriteFile(full, []byte(n.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", n.path, err)
		}
	}
	srv := newServerWithContract(t, root, distributionContract(t))

	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	block := homeSection(t, body, `data-home-block="lifecycle"`)

	total := 0
	for _, n := range chipCounts(t, block) {
		total += n
	}
	unstated := unstatedCounts(t, block)
	if len(unstated) != 2 {
		t.Fatalf("the block carries %d cells for notes with no status; want one for unparsable frontmatter and one for none stated", len(unstated))
	}
	// The division is the point, not just the total: a note carrying no
	// frontmatter declared nothing, which is legal, and calling it unreadable
	// would send a reader to repair a file with nothing wrong with it. Only
	// the note whose frontmatter was there and did not parse belongs in the
	// first cell.
	if want := []int{1, 2}; unstated[0] != want[0] || unstated[1] != want[1] {
		t.Errorf("the cells hold %v; want %v — one unparsable, and the bare and silent notes together", unstated, want)
	}
	for _, n := range unstated {
		total += n
	}
	if want := 4; total != want {
		t.Errorf("the block accounts for %d notes and the folder holds %d; a note that appears nowhere is the defect this counts against", total, want)
	}
}

// unstatedCounts reads the counts from the cells that stand for notes with no
// status. They carry their own markup rather than the status chip's, so this
// cannot pick up a status by accident — which is also what stops the block
// from dressing them as statuses.
func unstatedCounts(t *testing.T, block string) []int {
	t.Helper()
	var out []int
	const marker = `class="y-homeunstated__count"`
	for rest := block; ; {
		at := strings.Index(rest, marker)
		if at < 0 {
			return out
		}
		rest = rest[at:]
		open := strings.IndexByte(rest, '>')
		shut := strings.Index(rest, "</span>")
		if open < 0 || shut < 0 || shut < open {
			t.Fatalf("an unstated cell's count is not readable: %q", rest[:min(len(rest), 120)])
		}
		n, err := strconv.Atoi(strings.TrimSpace(rest[open+1 : shut]))
		if err != nil {
			t.Fatalf("an unstated cell states no number: %v", err)
		}
		out = append(out, n)
		rest = rest[shut:]
	}
}

// TestNoteMetarowCarriesADate holds the reading page's one date: the author's
// declared update when the frontmatter carries a readable one, and the file's
// own recorded change time when it does not. The two are different claims and
// each carries its own label — a fresh checkout stamps every file with one
// moment, and calling that moment the author's update would put words in their
// mouth. The declared date renders as the date it is; the file time keeps its
// clock in the machine-readable value, because that is what was observed.
func TestNoteMetarowCarriesADate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, content := range map[string]string{
		"Writing/Declared.md":   "---\ntitle: Declared\ntype: writing\nstatus: draft\nupdated: 2026-07-12\n---\n\nbody\n",
		"Writing/Undeclared.md": "---\ntitle: Undeclared\ntype: writing\nstatus: draft\n---\n\nbody\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	undeclared := filepath.Join(root, "Writing", "Undeclared.md")
	stamp := time.Date(2026, time.June, 3, 8, 30, 0, 0, time.UTC)
	if err := os.Chtimes(undeclared, stamp, stamp); err != nil {
		t.Fatalf("set mtime: %v", err)
	}
	// The recorded time is read back rather than trusted: a filesystem may
	// round what it was handed, and the page prints what the scan observed.
	info, err := os.Stat(undeclared)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	srv := newServer(t, root)

	code, body := get(t, srv.URL+"/notes/Writing/Declared.md")
	if code != http.StatusOK {
		t.Fatalf("GET declared note status = %d, want 200", code)
	}
	if want := `更新於 <time datetime="2026-07-12">2026-07-12</time>`; !strings.Contains(body, want) {
		t.Errorf("a note declaring its update does not show it; want %q in page", want)
	}
	if strings.Contains(body, "檔案變更於") {
		t.Error("a note declaring its update is dated by the file as well")
	}

	code, body = get(t, srv.URL+"/notes/Writing/Undeclared.md")
	if code != http.StatusOK {
		t.Fatalf("GET undeclared note status = %d, want 200", code)
	}
	mod := info.ModTime()
	want := `檔案變更於 <time datetime="` + mod.Format(time.RFC3339) + `">` + mod.Format(time.DateOnly) + `</time>`
	if !strings.Contains(body, want) {
		t.Errorf("a note declaring no update is not dated by the file; want %q in page", want)
	}
	if strings.Contains(body, "更新於 <time") {
		t.Error("a note declaring no update still claims an authored one")
	}
}

// TestNoteMetarowDateSpeaksTheInterfaceLanguage holds the date's label in the
// second language the chrome speaks. The words come from the dictionary, so an
// English page must not fall back to the default half of the pair.
func TestNoteMetarowDateSpeaksTheInterfaceLanguage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	full := filepath.Join(root, "Writing", "Declared.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntitle: Declared\ntype: writing\nstatus: draft\nupdated: 2026-07-12\n---\n\nbody\n"
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServer(t, root)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/notes/Writing/Declared.md", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(wording.En))
	resp, err := http.DefaultClient.Do(req)
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
	if want := `Updated <time datetime="2026-07-12">2026-07-12</time>`; !strings.Contains(body, want) {
		t.Errorf("the English page does not label the date in English; want %q", want)
	}
	if strings.Contains(body, "更新於") {
		t.Error("the English page carries the default-language date label")
	}
}

// TestHomeRecentStatesItsKnowledgeScope holds the two dashboard blocks to
// their own scopes, stated where the reader compares them. The recent list
// shows the declared knowledge folders; the distribution counts every indexed
// note. On a vault holding one draft inside the layer and one outside, the
// page used to show one recent note beside "draft 2" with nothing explaining
// the difference — both numbers true, and the pair reading as a contradiction.
// Each block now says what it counts, and neither is forced onto the other's
// set.
func TestHomeRecentStatesItsKnowledgeScope(t *testing.T) {
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
		content := "---\ntitle: " + title + "\ntype: writing\nstatus: draft\n---\n\nbody\n"
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, at, at); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}

	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, "Inside") {
		t.Fatalf("the recent block does not list the knowledge-layer note; section = %q", recent)
	}
	if strings.Contains(recent, "Outside") {
		t.Errorf("the recent block lists a note outside the knowledge layer; section = %q", recent)
	}
	if !strings.Contains(recent, "知識層資料夾中最近改動過的筆記") {
		t.Errorf("the recent block does not state its scope; section = %q", recent)
	}
	if strings.Contains(recent, "時間戳一模一樣") {
		t.Errorf("distinct times still read as a tie; section = %q", recent)
	}

	lifecycleBlock := homeSection(t, body, `data-home-block="lifecycle"`)
	row := homeLifecycleRow(t, lifecycleBlock, "draft")
	if !strings.Contains(row, ">2<") {
		t.Errorf("the distribution does not count both drafts; row = %q", row)
	}
	if !strings.Contains(lifecycleBlock, "書庫中每篇已索引筆記落在哪裡") {
		t.Errorf("the distribution does not state its own scope; section = %q", lifecycleBlock)
	}
}

// TestHomeSingleNoteClaimsNoTimestampTie holds the tie notice to vaults where
// it is true. With one note there is one recorded time and nothing for it to
// equal, yet the page said "these files carry identical timestamps" — a
// sentence about files that do not exist. One note is trivially the most
// recently changed thing, so the ordinary heading stands and the tie sentence
// stays off the page.
func TestHomeSingleNoteClaimsNoTimestampTie(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "---\ntitle: Only\ntype: writing\nstatus: draft\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(root, "Only.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServer(t, root)
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if strings.Contains(recent, "時間戳一模一樣") || strings.Contains(recent, "identical timestamps") {
		t.Errorf("a single-note vault claims a timestamp tie; section = %q", recent)
	}
	if !strings.Contains(recent, `<h2 id="home-recent-title">最近變更</h2>`) {
		t.Errorf("a single-note vault does not carry the ordinary recency heading; section = %q", recent)
	}
}

// TestHomeTiedTimesStillSaySoWithinTheirScope pins the tie notice itself: on
// a knowledge-scoped vault whose files all carry one moment, the block still
// says the times separate nothing — and now names the scope it lists, since
// the tie changes nothing about which files these are.
func TestHomeTiedTimesStillSaySoWithinTheirScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stamp := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for _, rel := range []string{"Writing/A.md", "Writing/B.md"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		title := strings.TrimSuffix(filepath.Base(rel), ".md")
		content := "---\ntitle: " + title + "\ntype: writing\nstatus: draft\n---\n\nbody\n"
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, stamp, stamp); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, "知識層資料夾中的筆記。這些檔案的時間戳一模一樣") {
		t.Errorf("tied times in a scoped list do not say both facts; section = %q", recent)
	}
}

// TestHomeUnscopedVaultClaimsNoKnowledgeLayer holds the scope phrase to
// vaults that declared one. A folder without a contract lists everything, and
// a lede naming a knowledge layer there would invent a rule its owner never
// wrote.
func TestHomeUnscopedVaultClaimsNoKnowledgeLayer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for rel, at := range map[string]time.Time{"A.md": base, "B.md": base.Add(time.Hour)} {
		content := "---\ntitle: " + strings.TrimSuffix(rel, ".md") + "\ntype: writing\n---\n\nbody\n"
		full := filepath.Join(root, rel)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, at, at); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServer(t, root)
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if strings.Contains(recent, "知識層") {
		t.Errorf("an unscoped vault's recent block names a knowledge layer; section = %q", recent)
	}
	if !strings.Contains(recent, "最近改動過的筆記") {
		t.Errorf("the unscoped lede is gone; section = %q", recent)
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
		"Maps/broken.md":  "---\ntitle: Broken course\ntype: study-path\n---\n\n## Part\n\n- [[Open]]\n",
		"Maps/prose.md":   "---\ntitle: Prose only\ntype: study-path\n---\n\nNothing here lists a lesson.\n",
		"Maps/working.md": "---\ntitle: Working course\ntype: study-path\n---\n\n## Part {sequence=primary}\n\n- [[Open]]\n",
		"Writing/Open.md": "---\ntitle: Open\ntype: lesson\nstatus: draft\n---\n\nbody\n",
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
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	paths := homeSection(t, body, `data-home-block="study-paths"`)
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

// TestHomeUnscopedTieKeepsThePlainNotice completes the lede's four states: a
// folder that declared no knowledge layer and whose files carry one moment
// gets the tie notice exactly as before, with no scope phrase — every clause
// of the sentence true of the page it sits on.
func TestHomeUnscopedTieKeepsThePlainNotice(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stamp := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	for _, rel := range []string{"A.md", "B.md"} {
		content := "---\ntitle: " + strings.TrimSuffix(rel, ".md") + "\ntype: writing\n---\n\nbody\n"
		full := filepath.Join(root, rel)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		if err := os.Chtimes(full, stamp, stamp); err != nil {
			t.Fatalf("set %s mtime: %v", rel, err)
		}
	}
	srv := newServer(t, root)
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, body, `data-home-block="recent"`)
	if !strings.Contains(recent, "<p>這些檔案的時間戳一模一樣") {
		t.Errorf("an unscoped tie lost its plain notice; section = %q", recent)
	}
	if strings.Contains(recent, "知識層") {
		t.Errorf("an unscoped tie names a knowledge layer; section = %q", recent)
	}
}

// TestHomeUnreadableContractKeepsTheRecentListUnscoped pins the degradation
// direction for a contract that exists and cannot be parsed. A folder with no
// contract at all lists everything it holds; a folder whose contract broke
// used to list nothing — so the vault most in need of repair was the one the
// page refused to show, and the operator mending the toml lost the reading
// surface they were mending it with. The broken contract still closes what a
// contract answers for — the lifecycle distribution and the study paths — and
// the parse error stays on the page; the recent list is plain reading and
// stays, with the plain lede, because a knowledge layer this contract declared
// is a claim nothing can vouch for now.
func TestHomeUnreadableContractKeepsTheRecentListUnscoped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	files := map[string]struct {
		content string
		at      time.Time
	}{
		"README.md":                    {content: "# Vault\n", at: base},
		"Concepts/Legal.md":            {content: "---\ntitle: Legal concept\ntype: concept\nstatus: draft\n---\n\nbody\n", at: base.Add(2 * time.Hour)},
		"System/templates/Template.md": {content: "---\ntitle: Template note\ntype: concept\nstatus: draft\n---\n\nbody\n", at: base.Add(time.Hour)},
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
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	page := html.UnescapeString(body)

	// The parse error is still the page's one loud sentence.
	assertCauseStatedOncePerRegion(t, page, "toml: line 42")
	// What a contract answers for stays closed.
	for name, marker := range map[string]string{
		"lifecycle":   `data-home-block="lifecycle"`,
		"study paths": `data-home-block="study-paths"`,
	} {
		if strings.Contains(page, marker) {
			t.Errorf("Home still renders the %s block under an unreadable contract", name)
		}
	}
	// Plain reading stays: the recent list, over everything readable, because
	// the exclusions the contract would have declared are unknown rather than
	// empty — and unknown may not silently hide files either.
	recent := homeSection(t, page, `data-home-block="recent"`)
	for _, want := range []string{"Legal concept", "Template note"} {
		if !strings.Contains(recent, want) {
			t.Errorf("recent block is missing %q; section = %q", want, recent)
		}
	}
	// The plain lede, not the scoped one: the knowledge layer is this
	// contract's own declaration, and this contract cannot vouch for it.
	if !strings.Contains(recent, "最近改動過的筆記") {
		t.Errorf("the plain lede is gone; section = %q", recent)
	}
	if strings.Contains(recent, "知識層") {
		t.Errorf("an unreadable contract's recent block cites a knowledge layer; section = %q", recent)
	}
	// The raw status value is shown without judgment: the folder is governed,
	// so the value is not hidden, and the vocabulary that would judge it
	// cannot be read, so nothing is flagged.
	if !strings.Contains(recent, "draft") {
		t.Errorf("the note's own status text is hidden; section = %q", recent)
	}
	if strings.Contains(recent, "不在 schema 允許清單中") {
		t.Errorf("a vocabulary nobody could read flagged a value; section = %q", recent)
	}
}

// TestHomeRecentFlagsAStatusOutsideTheSchema is the positive control for the
// out-of-enum chip in a recent row: under a contract that loaded, a value the
// note's type never declared carries the same phrase the search hit and the
// distribution chip carry. The bad-contract tests above assert this phrase is
// absent; this test is what proves that phrase is the one the row would print,
// so their absence checks cannot pass by hunting for words nothing emits.
func TestHomeRecentFlagsAStatusOutsideTheSchema(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Writing"), 0o750); err != nil {
		t.Fatalf("mkdir Writing: %v", err)
	}
	const content = "---\ntitle: Odd status\ntype: writing\nstatus: meditating\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(root, "Writing", "Odd.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	recent := homeSection(t, html.UnescapeString(body), `data-home-block="recent"`)
	if !strings.Contains(recent, "meditating") {
		t.Fatalf("the row does not name the value; section = %q", recent)
	}
	if !strings.Contains(recent, "不在 schema 允許清單中") {
		t.Errorf("an undeclared value carries no flag; section = %q", recent)
	}
}

// TestHomeBadContractWithNoNotesKeepsTheStandInAway pins the stand-in's other
// gate. A folder holding no markdown at all fills no content block, and the
// stand-in line would normally state what the folder has — but under a broken
// contract the lifecycle and study-path projections were withheld, the reason
// is on the page, and a cheerful fact beside that reason would be a second,
// contradictory account of the same hole. The recent block cannot carry this
// case: there are no notes, so nothing else stands between the two sentences.
func TestHomeBadContractWithNoNotesKeepsTheStandInAway(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("not markdown\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	srv := newServerWithGovernance(t, root, nil, schema.Unreadable(
		errors.New("toml: line 42: expected a key separator"),
	))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", code)
	}
	page := html.UnescapeString(body)
	if !strings.Contains(page, "toml: line 42") {
		t.Fatalf("the parse error is missing; page = %q", page)
	}
	if strings.Contains(page, "data-home-standin") {
		t.Error("Home states what the folder holds beside the reason it cannot say what is in it")
	}
	if strings.Contains(page, `data-home-block="recent"`) {
		t.Error("Home renders a recent block for a folder with no markdown")
	}
}
