package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

// scannedEntry is the entry a scan observes for rel, written with content and
// modified at modified, in a folder of its own.
func scannedEntry(t *testing.T, rel, content string, modified time.Time) vault.Entry {
	t.Helper()
	root := t.TempDir()
	writeNote(t, root, rel, content)
	if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(rel)), modified, modified); err != nil {
		t.Fatalf("Chtimes(%q): %v", rel, err)
	}
	scan, err := openReader(t, root).ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("ScanAvailable: %v", err)
	}
	entry, ok := scan.Entry(rel)
	if !ok {
		t.Fatalf("the scan holds no %q", rel)
	}
	return entry
}

// TestAWarningRepeatsOnlyWhenWhatItIsAboutChanges holds the key a warning is
// remembered by: a change to any part of it is news, and nothing else is.
func TestAWarningRepeatsOnlyWhenWhatItIsAboutChanges(t *testing.T) {
	t.Parallel()
	modified := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	seen := scannedEntry(t, "a.md", "body\n", modified)
	tests := []struct {
		name    string
		entry   vault.Entry
		kind    string
		message string
		want    bool
	}{
		{name: "nothing changed", entry: seen, kind: warningRead, message: "refused"},
		{name: "another path", entry: scannedEntry(t, "b.md", "body\n", modified), kind: warningRead, message: "refused", want: true},
		{name: "another size", entry: scannedEntry(t, "a.md", "a longer body\n", modified), kind: warningRead, message: "refused", want: true},
		{name: "another mtime", entry: scannedEntry(t, "a.md", "body\n", modified.Add(time.Second)), kind: warningRead, message: "refused", want: true},
		{name: "another message", entry: seen, kind: warningRead, message: "refused again", want: true},
		{name: "another kind", entry: seen, kind: warningSidecar, message: "refused", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reported := make(map[warningKey]struct{})
			first := newWarningAttempt(reported)
			if !first.record(seen, warningRead, "refused") {
				t.Fatal("the first warning was not reported")
			}
			first.complete()
			if got := newWarningAttempt(reported).record(tt.entry, tt.kind, tt.message); got != tt.want {
				t.Errorf("record() after the attempt before reported (a.md, %s, refused) = %t, want %t", warningRead, got, tt.want)
			}
		})
	}
}

// TestAWarningThatClearsIsReportedWhenItReturns holds a finished attempt to
// what it saw: a warning it did not meet is over, so its return is news.
func TestAWarningThatClearsIsReportedWhenItReturns(t *testing.T) {
	t.Parallel()
	entry := scannedEntry(t, "a.md", "body\n", time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	reported := make(map[warningKey]struct{})
	first := newWarningAttempt(reported)
	first.record(entry, warningRead, "refused")
	first.complete()

	newWarningAttempt(reported).complete()

	if !newWarningAttempt(reported).record(entry, warningRead, "refused") {
		t.Error("a warning that cleared and came back was not reported")
	}
}

// TestAnAbandonedAttemptKeepsWhatWasReported holds an attempt that never
// finished to nothing it did not see through: it forgets no warning it did not
// reach, and a warning it reported is not reported again.
func TestAnAbandonedAttemptKeepsWhatWasReported(t *testing.T) {
	t.Parallel()
	modified := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	earlier := scannedEntry(t, "a.md", "body\n", modified)
	fresh := scannedEntry(t, "b.md", "body\n", modified)
	reported := make(map[warningKey]struct{})
	first := newWarningAttempt(reported)
	first.record(earlier, warningRead, "refused")
	first.complete()

	newWarningAttempt(reported).record(fresh, warningRead, "refused")

	next := newWarningAttempt(reported)
	if next.record(earlier, warningRead, "refused") {
		t.Error("an abandoned attempt forgot a warning it never reached")
	}
	if next.record(fresh, warningRead, "refused") {
		t.Error("a warning an abandoned attempt reported was reported again")
	}
}

// cancelingSource cancels a build once it has read the file at, so the build
// finishes its reads under a canceled context.
type cancelingSource struct {
	Source

	at     string
	cancel context.CancelFunc
}

func (s *cancelingSource) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	data, err := s.Source.ReadFile(ctx, entry)
	if s.cancel != nil && entry.Path() == s.at {
		s.cancel()
	}
	return data, err
}

// TestTheStoreLogsARefusalOncePerEpisode holds the build to its attempt's
// record: a refusal every build meets is logged once, a canceled build that
// saw the file read leaves that alone, and a refusal that a finished build saw
// clear is logged again when it returns.
func TestTheStoreLogsARefusalOncePerEpisode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A figure is read but holds nothing back, so every build below publishes.
	// The note is read after it, and editing the note makes each rescan build.
	const figure, note = "a.html", "z.md"
	writeNote(t, root, figure, "<p>figure</p>\n")
	writeNote(t, root, note, "0\n")
	recording := newRecordingSource(t, root)
	recording.fail[figure] = 1 << 30
	source := &cancelingSource{Source: recording}
	log, logged := newRecordingLogger()
	store := buildStore(t, source, log, nil)
	edits := 0
	rebuild := func(ctx context.Context) {
		edits++
		writeNote(t, root, note, strings.Repeat("x", edits)+"\n")
		store.rescan(ctx)
	}
	refusals := func() int { return len(linesWith(logged(), logSourceUnavailable)) }

	rebuild(t.Context())
	if got := refusals(); got != 1 {
		t.Fatalf("refusals logged by two builds that met the same one = %d, want 1", got)
	}

	recording.fail[figure] = 0
	ctx, cancel := context.WithCancel(t.Context())
	source.at, source.cancel = note, cancel
	rebuild(ctx)
	source.cancel = nil
	recording.fail[figure] = 1 << 30
	rebuild(t.Context())
	if got := refusals(); got != 1 {
		t.Errorf("refusals logged after a canceled build saw the figure read = %d, want 1", got)
	}

	recording.fail[figure] = 0
	rebuild(t.Context())
	recording.fail[figure] = 1 << 30
	rebuild(t.Context())
	if got := refusals(); got != 2 {
		t.Errorf("refusals logged after a finished build saw the figure read = %d, want 2", got)
	}
}
