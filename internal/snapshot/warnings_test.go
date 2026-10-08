package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestSnapshotWarningsUnchanged(t *testing.T) {
	root := t.TempDir()
	huge := strings.Repeat("x", render.MaxSourceBytes+1)
	writeNote(t, root, "huge.md", huge)
	writeNote(t, root, "citing.md", "See [[huge]].\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &recordingSource{ObservedSource: reader, reads: map[string]int{}, fail: map[string]int{}}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	store, err := New(t.Context(), source, logger, contract, contract.Governance())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Log("producer-hit: actual New returned its captured generation")
	now := time.Date(2026, time.October, 6, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	previous := store.Current()
	assertWarningSkip(t, previous, int64(len(huge)))
	for generation := 1; generation <= 2; generation++ {
		path := fmt.Sprintf("later-%d.md", generation)
		needle := fmt.Sprintf("freshwarningbody%d", generation)
		writeNote(t, root, path, needle+"\n")
		now = now.Add(time.Second)
		store.rescan(t.Context())
		current := store.Current()
		if current == previous || !current.Freshness().Complete {
			t.Fatalf("rescan %d did not publish a distinct complete generation", generation)
		}
		results := snapshotSearch(t, current.Search(), needle)
		if len(results) != 1 || results[0].RelPath != path {
			t.Fatalf("Search(%q) = %+v, want %s alone", needle, results, path)
		}
		assertWarningSkip(t, current, int64(len(huge)))
		previous = current
	}
	if got := source.reads["huge.md"]; got != 0 {
		t.Fatalf("over-bound ReadFile calls = %d, want 0", got)
	}
	t.Log("producer-hit: New and two complete rescans retained the over-bound citation stub")
	got := warningRecords(t, logs.Bytes(), "vault note skipped: larger than the source size bound")
	want := []map[string]any{{"level": "WARN", "msg": "vault note skipped: larger than the source size bound", "path": "huge.md", "bytes": float64(len(huge))}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("caught: repeated oversize warning (-want +got):\n%s", diff)
	}
}

func assertWarningSkip(t *testing.T, gen *Generation, size int64) {
	t.Helper()
	want := []Skipped{{Path: "huge.md", Reason: "over the source bound", Size: size}}
	if diff := cmp.Diff(want, gen.Skipped()); diff != "" {
		t.Fatalf("caught: size skip projection lost (-want +got):\n%s", diff)
	}
	if _, ok := gen.Note("huge.md"); ok {
		t.Fatal("over-bound note was retained")
	}
	if _, ok := gen.Entry("huge.md"); !ok {
		t.Fatal("over-bound note disappeared from captured scan")
	}
	resolved := gen.Graph().Resolve("huge")
	if resolved.Kind != graph.KindUnique || resolved.RelPath != "huge.md" {
		t.Fatalf("Resolve(huge) = %+v, want unique huge.md", resolved)
	}
	note, ok := gen.Note("citing.md")
	if !ok {
		t.Fatal("citing note unavailable")
	}
	if html := gen.Render("citing.md", note.Body, wording.ZhHant).HTML; !strings.Contains(html, "/notes/huge.md") {
		t.Fatalf("rendered citation = %q, want huge.md destination", html)
	}
}

func warningRecords(t *testing.T, data []byte, message string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range bytes.SplitSeq(bytes.TrimSpace(data), []byte{'\n'}) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode logger record: %v", err)
		}
		if record["level"] == "WARN" && record["msg"] == message {
			delete(record, "time")
			out = append(out, record)
		}
	}
	return out
}

type warningSource struct {
	reader      *vault.Reader
	refuse      map[string]error
	refuseHead  map[string]error
	beforeRead  func(context.Context, vault.Entry)
	reads       map[string]int
	rootRefusal error
	scanRefusal error
}

func (s *warningSource) ObserveRoot() (vault.RootIdentity, error) {
	if s.rootRefusal != nil {
		return vault.RootIdentity{}, s.rootRefusal
	}
	return s.reader.ObserveRoot()
}

func (s *warningSource) ScanAvailable(ctx context.Context) (vault.Scan, error) {
	if s.scanRefusal != nil {
		return vault.Scan{}, s.scanRefusal
	}
	return s.reader.ScanAvailable(ctx)
}

func (s *warningSource) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	s.reads[entry.Path()]++
	if s.beforeRead != nil {
		s.beforeRead(ctx, entry)
	}
	if err := s.refuse[entry.Path()]; err != nil {
		return nil, err
	}
	return s.reader.ReadFile(ctx, entry)
}

func (s *warningSource) ReadPrefix(ctx context.Context, entry vault.Entry, n int64) ([]byte, error) {
	if err := s.refuseHead[entry.Path()]; err != nil {
		return nil, err
	}
	return s.reader.ReadPrefix(ctx, entry, n)
}

type warningFixture struct {
	root   string
	store  *Store
	source *warningSource
	logs   *bytes.Buffer
	clock  time.Time
	steps  int
}

func newWarningFixture(t *testing.T, root string, configure func(*warningSource)) *warningFixture {
	t.Helper()
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &warningSource{reader: reader, refuse: map[string]error{}, refuseHead: map[string]error{}, reads: map[string]int{}}
	if configure != nil {
		configure(source)
	}
	logs := new(bytes.Buffer)
	store, err := New(t.Context(), source, slog.New(slog.NewJSONHandler(logs, nil)), contract, contract.Governance())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Log("producer-hit: actual New returned its captured generation")
	f := &warningFixture{root: root, store: store, source: source, logs: logs, clock: store.Current().Freshness().BuiltAt}
	store.now = func() time.Time { return f.clock }
	return f
}

func (f *warningFixture) rescan(t *testing.T) {
	t.Helper()
	f.steps++
	path := fmt.Sprintf("unrelated-%d.md", f.steps)
	writeNote(t, f.root, path, fmt.Sprintf("independentwarningbody%d\n", f.steps))
	f.clock = f.clock.Add(2 * time.Hour)
	f.store.rescan(t.Context())
}

func assertWarningCount(t *testing.T, f *warningFixture, message string, want int, caught string) {
	t.Helper()
	got := warningRecords(t, f.logs.Bytes(), message)
	if len(got) != want {
		t.Fatalf("caught: %s: logger records = %+v, want %d", caught, got, want)
	}
}

func setWarningTime(t *testing.T, root, path string, modified time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(path)), modified, modified); err != nil {
		t.Fatalf("Chtimes(%q): %v", path, err)
	}
}

func TestSnapshotWarningIdentity(t *testing.T) {
	const message = "vault source unavailable in snapshot generation"
	const briefing = "System/reports/daily-briefing/2026-10-06.html"
	for _, name := range []string{"path", "size", "mtime", "message", "kind"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := "fault.txt"
			if name == "kind" {
				path = briefing
			}
			writeNote(t, root, path, "first body\n")
			modified := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
			setWarningTime(t, root, path, modified)
			if name == "path" {
				writeNote(t, root, "other.txt", "first body\n")
				setWarningTime(t, root, "other.txt", modified)
			}
			f := newWarningFixture(t, root, func(s *warningSource) {
				s.refuse[path] = errors.New("same failure")
				if name == "path" {
					s.refuse["other.txt"] = errors.New("same failure")
				}
				if name == "kind" {
					s.refuseHead[path] = errors.New("same failure")
				}
			})
			if !f.store.Current().Freshness().Complete {
				t.Fatal("ordinary file refusal made startup incomplete")
			}
			switch name {
			case "size":
				writeNote(t, root, path, "a longer body\n")
				setWarningTime(t, root, path, modified)
			case "mtime":
				setWarningTime(t, root, path, modified.Add(time.Nanosecond))
			case "message":
				f.source.refuse[path] = errors.New("changed failure")
			}
			f.rescan(t)
			if !f.store.Current().Freshness().Complete {
				t.Fatal("ordinary file refusal prevented a complete rebuild")
			}
			if got := snapshotSearch(t, f.store.Current().Search(), "independentwarningbody1"); len(got) != 1 || got[0].RelPath != "unrelated-1.md" {
				t.Fatalf("healthy body after refusal = %+v", got)
			}
			t.Log("producer-hit: actual source refusal retained independent publication")
			if name == "kind" {
				assertWarningCount(t, f, "briefing head unreadable; it is shown under its file name", 1, "warning kinds were collapsed")
				assertWarningCount(t, f, message, 1, "warning kinds were collapsed")
				return
			}
			assertWarningCount(t, f, message, 2, "warning "+name+" was collapsed")
			f.rescan(t)
			assertWarningCount(t, f, message, 2, "unchanged identity was reported again")
		})
	}
}

func TestSnapshotWarningRecovery(t *testing.T) {
	const message = "vault note skipped: larger than the source size bound"
	for _, name := range []string{"touch", "shrink", "delete"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			huge := strings.Repeat("x", render.MaxSourceBytes+1)
			writeNote(t, root, "huge.md", huge)
			writeNote(t, root, "citing.md", "See [[huge]].\n")
			modified := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
			setWarningTime(t, root, "huge.md", modified)
			f := newWarningFixture(t, root, nil)
			assertWarningSkip(t, f.store.Current(), int64(len(huge)))
			if name == "touch" {
				assertWarningCount(t, f, message, 1, "initial oversized warning missing")
				modified = modified.Add(time.Second)
				setWarningTime(t, root, "huge.md", modified)
				f.rescan(t)
				assertWarningSkip(t, f.store.Current(), int64(len(huge)))
				entry, ok := f.store.Current().Entry("huge.md")
				if !ok || !entry.ModTime().Equal(modified) {
					t.Fatalf("not-applied: captured oversized mtime = %v, present %t, want %v", entry.ModTime(), ok, modified)
				}
				assertWarningCount(t, f, message, 2, "changed oversized observation was suppressed")
				f.rescan(t)
				assertWarningSkip(t, f.store.Current(), int64(len(huge)))
				if got := f.source.reads["huge.md"]; got != 0 {
					t.Fatalf("over-bound ReadFile calls = %d, want 0", got)
				}
				t.Log("producer-hit: actual oversized touch and unchanged unrelated rescan")
				assertWarningCount(t, f, message, 2, "unchanged touched oversized warning was reported again")
				return
			}
			if name == "shrink" {
				writeNote(t, root, "huge.md", "recovered body\n")
			} else if err := os.Remove(filepath.Join(root, "huge.md")); err != nil {
				t.Fatalf("remove: %v", err)
			}
			f.rescan(t)
			if len(f.store.Current().Skipped()) != 0 {
				t.Fatal("completed recovery retained the old skip")
			}
			if name == "shrink" {
				if note, ok := f.store.Current().Note("huge.md"); !ok || note.Body != "recovered body\n" {
					t.Fatalf("recovered note = %+v, present %t", note, ok)
				}
			}
			writeNote(t, root, "huge.md", huge)
			setWarningTime(t, root, "huge.md", modified)
			f.rescan(t)
			assertWarningSkip(t, f.store.Current(), int64(len(huge)))
			t.Log("producer-hit: completed recovery and exact-observation relapse")
			assertWarningCount(t, f, message, 2, "recovered warning was not rearmed")
			restarted := newWarningFixture(t, root, nil)
			assertWarningSkip(t, restarted.store.Current(), int64(len(huge)))
			assertWarningCount(t, restarted, message, 1, "restart retained prior warning history")
		})
	}
}

func TestSnapshotWarningProducers(t *testing.T) {
	const head = "System/reports/daily-briefing/2026-10-06.html"
	for _, name := range []string{"source", "head", "sidecar"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeNote(t, root, "healthy.md", "healthybody\n")
			var message string
			want := 1
			switch name {
			case "source":
				writeNote(t, root, "fault.txt", "unreadablefilebody\n")
				message = "vault source unavailable in snapshot generation"
			case "head":
				writeNote(t, root, head, "<title>Readable title</title>")
				message = "briefing head unreadable; it is shown under its file name"
			case "sidecar":
				writeNote(t, root, "System/slots/bad.yaml", "slug: bad\npatterns:\n  - id: p\n    template: '{A}'\n    gloss_zh: '{B}'\n")
				writeNote(t, root, "System/slots/good.yaml", "slug: good\n")
				message = "slot sidecar unusable in snapshot generation"
				want = 2
			}
			f := newWarningFixture(t, root, func(s *warningSource) {
				if name == "source" {
					s.refuse["fault.txt"] = errors.New("refused source")
				}
				if name == "head" {
					s.refuseHead[head] = errors.New("refused head")
				}
			})
			for generation := range 3 {
				if generation != 0 {
					f.rescan(t)
				}
				gen := f.store.Current()
				if !gen.Freshness().Complete {
					t.Fatal("per-file warning held back the complete generation")
				}
				if got := snapshotSearch(t, gen.Search(), "healthybody"); len(got) != 1 || got[0].RelPath != "healthy.md" {
					t.Fatalf("healthy search = %+v", got)
				}
				switch name {
				case "source":
					if _, ok := gen.Entry("fault.txt"); !ok {
						t.Fatal("refused source disappeared from scan")
					}
					if got := snapshotSearch(t, gen.Search(), "unreadablefilebody"); len(got) != 0 {
						t.Fatalf("unread source reached search: %+v", got)
					}
				case "head":
					reports := gen.Navigation().Reports()
					if len(reports) != 1 || reports[0].Title != "2026-10-06.html" {
						t.Fatalf("briefing fallback = %+v", reports)
					}
				case "sidecar":
					if _, ok := gen.Slots().Lookup("bad"); ok {
						t.Fatal("unusable sidecar gained a practice panel")
					}
					if _, ok := gen.Slots().Lookup("good"); !ok || gen.Slots().Len() != 1 {
						t.Fatal("neighboring valid sidecar was lost")
					}
				}
			}
			t.Log("producer-hit: actual " + name + " producer and downstream projections")
			assertWarningCount(t, f, message, want, name+" warning was reported again")
			if name == "sidecar" {
				got := warningRecords(t, f.logs.Bytes(), message)
				wantRecords := []map[string]any{
					{"level": "WARN", "msg": message, "path": "System/slots/bad.yaml", "problem": "pattern \"p\": template key {A} has no slot"},
					{"level": "WARN", "msg": message, "path": "System/slots/bad.yaml", "problem": "pattern \"p\": gloss key {B} is not in the template"},
				}
				if diff := cmp.Diff(wantRecords, got); diff != "" {
					t.Fatalf("distinct sidecar records (-want +got):\n%s", diff)
				}
				return
			}
			path, detail := "fault.txt", "refused source"
			if name == "head" {
				path, detail = head, "refused head"
			}
			wantRecords := []map[string]any{{"level": "WARN", "msg": message, "path": path, "error": detail}}
			if diff := cmp.Diff(wantRecords, warningRecords(t, f.logs.Bytes(), message)); diff != "" {
				t.Fatalf("original source diagnostic attrs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSnapshotWarningVerdict(t *testing.T) {
	// Contract loading refuses the bad slug pattern before a real generation
	// can reach this producer. This fixture holds its bounded reporting route.
	root := t.TempDir()
	writeNote(t, root, "note.md", "ordinary body\n")
	f := newWarningFixture(t, root, nil)
	entry, ok := f.store.Current().Entry("note.md")
	if !ok {
		t.Fatal("captured note entry unavailable")
	}
	reported := make(map[warningKey]struct{})
	logger := slog.New(slog.NewJSONHandler(f.logs, nil))
	result := judge.FrontmatterResult{Findings: []judge.Finding{{RuleID: "schema.enum"}}, EnumNoteType: "system"}
	for range 3 {
		g := newGeneration(1)
		g.warnings = newWarningAttempt(reported)
		g.recordVerdict(entry, result, errors.New("unavailable pattern"), logger)
		if len(g.results) != 0 {
			t.Fatal("failed verdict gained findings")
		}
		g.warnings.complete()
	}
	t.Log("producer-hit: bounded actual recordVerdict route; New caller failure unreachable")
	assertWarningCount(t, f, "schema verdict unavailable for a note", 1, "verdict warning was reported again")
	g := newGeneration(1)
	g.warnings = newWarningAttempt(reported)
	g.recordVerdict(entry, result, nil, logger)
	if diff := cmp.Diff(result, g.results["note.md"]); diff != "" {
		t.Fatalf("successful verdict lost its bundle (-want +got):\n%s", diff)
	}
	g.warnings.complete()
	g = newGeneration(1)
	g.warnings = newWarningAttempt(reported)
	g.recordVerdict(entry, result, errors.New("unavailable pattern"), logger)
	assertWarningCount(t, f, "schema verdict unavailable for a note", 2, "recovered verdict was not rearmed")
}

func TestSnapshotWarningsCanceled(t *testing.T) {
	const message = "vault source unavailable in snapshot generation"
	for _, name := range []string{"new report", "partial recovery"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeNote(t, root, "a-fault.txt", "faultybody\n")
			writeNote(t, root, "zz.md", "lastbody\n")
			f := newWarningFixture(t, root, func(s *warningSource) {
				if name == "partial recovery" {
					s.refuse["a-fault.txt"] = errors.New("same refusal")
				}
			})
			previous := f.store.Current()
			fresh := previous.Freshness()
			lastRebuild := f.store.lastRebuild
			if name == "new report" {
				f.source.refuse["a-fault.txt"] = errors.New("same refusal")
			} else {
				delete(f.source.refuse, "a-fault.txt")
			}
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			visited := false
			f.source.beforeRead = func(_ context.Context, entry vault.Entry) {
				if entry.Path() == "a-fault.txt" {
					visited = true
				}
				if entry.Path() == "zz.md" {
					cancel()
				}
			}
			writeNote(t, root, "zz.md", "changed lastbody\n")
			f.clock = f.clock.Add(2 * time.Hour)
			f.store.rescan(ctx)
			if !visited || ctx.Err() == nil || f.store.Current() != previous {
				t.Fatal("source callback did not abandon the actual build after visiting the fault")
			}
			if diff := cmp.Diff(fresh, previous.Freshness()); diff != "" {
				t.Fatalf("canceled attempt changed freshness (-want +got):\n%s", diff)
			}
			if f.store.lastRebuild != lastRebuild {
				t.Fatal("canceled attempt advanced the rebuild clock")
			}
			f.source.beforeRead = nil
			f.source.refuse["a-fault.txt"] = errors.New("same refusal")
			f.rescan(t)
			if f.store.Current() == previous || !f.store.Current().Freshness().Complete {
				t.Fatal("noncanceled control did not publish")
			}
			t.Log("producer-hit: actual canceled build and unchanged noncanceled refusal")
			caught := "canceled warning was reported again"
			if name == "partial recovery" {
				caught = "cancellation manufactured warning recovery"
			}
			assertWarningCount(t, f, message, 1, caught)
		})
	}
}

func TestSnapshotWarningsIncomplete(t *testing.T) {
	const message = "vault note skipped: larger than the source size bound"
	root := t.TempDir()
	huge := strings.Repeat("x", render.MaxSourceBytes+1)
	writeNote(t, root, "huge.md", huge)
	writeNote(t, root, "citing.md", "See [[huge]].\n")
	writeNote(t, root, "bad.md", "previousgoodbody\n")
	modified := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	setWarningTime(t, root, "huge.md", modified)
	f := newWarningFixture(t, root, nil)
	previous := f.store.Current()
	f.source.refuse["bad.md"] = errors.New("permanent read refusal")
	writeNote(t, root, "huge.md", "recovered body\n")
	f.rescan(t)
	if f.store.Current() != previous {
		t.Fatal("first incomplete candidate was published before its retention threshold")
	}
	wantBlocked := []BlockedSource{{Path: "bad.md", Reason: "permanent read refusal"}}
	if diff := cmp.Diff(wantBlocked, previous.Freshness().Blocked); diff != "" {
		t.Fatalf("retained blocked sources (-want +got):\n%s", diff)
	}
	if previous.Freshness().FailedRetries != 1 || f.store.nextRetry != f.clock.Add(2*time.Second) {
		t.Fatal("first incomplete attempt changed the retry schedule")
	}
	writeNote(t, root, "huge.md", huge)
	setWarningTime(t, root, "huge.md", modified)
	f.rescan(t)
	if f.store.Current() != previous {
		t.Fatal("second incomplete candidate was published before its retention threshold")
	}
	t.Log("producer-hit: complete retained recovery followed by exact-observation relapse")
	assertWarningCount(t, f, message, 2, "retained attempt did not rearm warning")
	f.rescan(t)
	degraded := f.store.Current()
	if degraded == previous || degraded.Freshness().Complete {
		t.Fatal("third incomplete attempt did not publish degraded reading")
	}
	if diff := cmp.Diff(wantBlocked, degraded.Freshness().Blocked); diff != "" {
		t.Fatalf("degraded blocked sources (-want +got):\n%s", diff)
	}
	if note, ok := degraded.Note("bad.md"); !ok || !note.Stale || note.Body != "previousgoodbody\n" {
		t.Fatalf("degraded reading = %+v, present %t", note, ok)
	}
	assertWarningSkip(t, degraded, int64(len(huge)))
	if got := snapshotSearch(t, degraded.Search(), "independentwarningbody3"); len(got) != 1 || got[0].RelPath != "unrelated-3.md" {
		t.Fatalf("degraded healthy search = %+v", got)
	}
	assertWarningCount(t, f, message, 2, "degraded attempt repeated oversize warning")
	assertWarningCount(t, f, "vault source unavailable in snapshot generation", 1, "retained source warning was reported again")
	assertWarningCount(t, f, "vault snapshot incomplete; retaining previous generation", 3, "process warning was suppressed")
	delete(f.source.refuse, "bad.md")
	f.rescan(t)
	if !f.store.Current().Freshness().Complete || len(f.store.Current().Freshness().Blocked) != 0 {
		t.Fatal("fixed source did not restore complete reading")
	}
}

func TestSnapshotWarningPanic(t *testing.T) {
	const message = "vault note parse still panics on bytes already reported; treating the note as unreadable"
	for stage := range stagePanics {
		t.Run(stagePanics[stage].name, func(t *testing.T) {
			root := t.TempDir()
			writePanicVault(t, root)
			f := newWarningFixture(t, root, nil)
			restore := breakStage(t, stage)
			for range 3 {
				f.rescan(t)
			}
			assertTheRestIsServed(t, f.store.Current())
			fresh := f.store.Current().Freshness()
			if fresh.Complete || len(fresh.Blocked) != 1 || !fresh.Blocked[0].ParsePanic {
				t.Fatalf("parse-panic blocked facts = %+v", fresh)
			}
			t.Log("producer-hit: real parser stage, first ERROR, two repeats and containment")
			assertWarningCount(t, f, message, 1, "panic warning was reported again")
			assertWarningPanicError(t, f.logs.Bytes(), 1)
			restore()
			f.rescan(t)
			if !f.store.Current().Freshness().Complete {
				t.Fatal("genuine parser recovery remained incomplete")
			}
			breakStage(t, stage)
			f.rescan(t)
			f.rescan(t)
			assertWarningCount(t, f, message, 2, "recovered panic warning was not rearmed")
			assertWarningPanicError(t, f.logs.Bytes(), 2)
		})
	}
}

func assertWarningPanicError(t *testing.T, data []byte, want int) {
	t.Helper()
	count := 0
	for line := range bytes.SplitSeq(bytes.TrimSpace(data), []byte{'\n'}) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode ERROR: %v", err)
		}
		if record["level"] != "ERROR" {
			continue
		}
		count++
		if record["path"] != panicPoisoned || record["panic"] != panicValue || record["stack"] == "" || record["stack"] == nil {
			t.Fatalf("first parse panic lost its full report: %+v", record)
		}
	}
	if count != want {
		t.Fatalf("caught: panic ERROR history changed: ERROR count = %d, want %d", count, want)
	}
}

func TestSnapshotWarningProducerRecovery(t *testing.T) {
	const head = "System/reports/daily-briefing/2026-10-06.html"
	for _, name := range []string{"source", "head", "sidecar"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path, body, message := "fault.txt", "ordinarybody\n", "vault source unavailable in snapshot generation"
			wantChanged, wantRelapsed := 2, 3
			if name == "head" {
				path, body, message = head, "<title>Readable title</title>", "briefing head unreadable; it is shown under its file name"
			}
			if name == "sidecar" {
				path, body, message = "System/slots/bad.yaml", "slug: bad\npatterns:\n  - id: p\n    template: '{A}'\n    gloss_zh: '{B}'\n", "slot sidecar unusable in snapshot generation"
				wantChanged, wantRelapsed = 4, 6
			}
			writeNote(t, root, path, body)
			modified := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
			setWarningTime(t, root, path, modified)
			f := newWarningFixture(t, root, func(s *warningSource) {
				if name == "source" {
					s.refuse[path] = errors.New("first refusal")
				}
				if name == "head" {
					s.refuseHead[path] = errors.New("first refusal")
				}
			})
			switch name {
			case "source":
				f.source.refuse[path] = errors.New("second refusal")
			case "head":
				f.source.refuseHead[path] = errors.New("second refusal")
			case "sidecar":
				writeNote(t, root, path, strings.Replace(body, "id: p", "id: q", 1))
				setWarningTime(t, root, path, modified)
			}
			f.rescan(t)
			assertWarningCount(t, f, message, wantChanged, name+" changed message was suppressed")
			delete(f.source.refuse, path)
			delete(f.source.refuseHead, path)
			if name == "sidecar" {
				writeNote(t, root, path, "slug: bad\n")
			}
			f.rescan(t)
			gen := f.store.Current()
			if !gen.Freshness().Complete {
				t.Fatal("recovered producer left an incomplete generation")
			}
			switch name {
			case "source":
				if got := snapshotSearch(t, gen.Search(), "ordinarybody"); len(got) != 1 || got[0].RelPath != path {
					t.Fatalf("recovered source search = %+v", got)
				}
			case "head":
				reports := gen.Navigation().Reports()
				if len(reports) != 1 || reports[0].Title != "Readable title" {
					t.Fatalf("recovered briefing title = %+v", reports)
				}
			case "sidecar":
				if _, ok := gen.Slots().Lookup("bad"); !ok {
					t.Fatal("recovered sidecar remained unavailable")
				}
			}
			switch name {
			case "source":
				f.source.refuse[path] = errors.New("first refusal")
			case "head":
				f.source.refuseHead[path] = errors.New("first refusal")
			case "sidecar":
				writeNote(t, root, path, body)
				setWarningTime(t, root, path, modified)
			}
			f.rescan(t)
			t.Log("producer-hit: changed diagnostic, complete recovery and same-observation relapse for " + name)
			assertWarningCount(t, f, message, wantRelapsed, name+" recovered warning was not rearmed")
		})
	}
}

func TestSnapshotWarningScope(t *testing.T) {
	const message = "vault source unavailable in snapshot generation"
	for _, name := range []string{"root refusal", "scan refusal", "metadata fast path", "retry backoff"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := "fault.txt"
			if name == "retry backoff" {
				path = "fault.md"
			}
			writeNote(t, root, path, "faultybody\n")
			f := newWarningFixture(t, root, func(s *warningSource) { s.refuse[path] = errors.New("unchanged refusal") })
			if name == "retry backoff" {
				f.store.rescan(t.Context())
				if !f.clock.Before(f.store.nextRetry) {
					t.Fatal("completed refused rescan did not establish a retry deadline")
				}
			}
			previous := f.store.Current()
			reads := f.source.reads[path]
			delete(f.source.refuse, path)
			switch name {
			case "root refusal":
				f.source.rootRefusal = errors.New("selected root unavailable")
			case "scan refusal":
				f.source.scanRefusal = errors.New("scan unavailable")
			}
			f.store.rescan(t.Context())
			if f.store.Current() != previous || f.source.reads[path] != reads {
				t.Fatal("bypass route unexpectedly built a recovery candidate")
			}
			if name == "root refusal" && previous.Freshness().Root == nil {
				t.Fatal("independent selected-root episode was suppressed")
			}
			if name == "scan refusal" {
				assertWarningCount(t, f, "vault scan unavailable; retaining previous snapshot", 1, "independent scan warning was suppressed")
			}
			f.source.rootRefusal, f.source.scanRefusal = nil, nil
			f.source.refuse[path] = errors.New("unchanged refusal")
			f.rescan(t)
			t.Log("producer-hit: real bypass followed by unchanged refusal for " + name)
			assertWarningCount(t, f, message, 1, "bypass manufactured warning recovery")
		})
	}
}
