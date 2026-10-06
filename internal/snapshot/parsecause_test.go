package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

type parseCauseReadFailure struct{ ObservedSource }

func (s parseCauseReadFailure) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	if entry.Path() == panicPoisoned {
		return nil, errors.New("panic while parsing this note: a read error")
	}
	return s.ObservedSource.ReadFile(ctx, entry)
}

func TestBlockedCauseComesFromTheParserBoundary(t *testing.T) {
	for stage := range stagePanics {
		t.Run(stagePanics[stage].name, func(t *testing.T) {
			root := t.TempDir()
			writePanicVault(t, root)
			breakStage(t, stage)
			store, err := New(t.Context(), panicSource(t, root), discardLogger(), nil, schema.Governance{})
			if err != nil {
				t.Fatal(err)
			}
			assertParseCause(t, store.Current().Freshness().Blocked, true, "panic while parsing this note: "+panicValue)
		})
	}
	t.Run("empty panic", func(t *testing.T) {
		root := t.TempDir()
		writePanicVault(t, root)
		genuine := parsers
		t.Cleanup(func() { parsers = genuine })
		parsers.note = func(rel string, data []byte) *vault.Note {
			if strings.Contains(string(data), poisonMark) {
				panic("")
			}
			return genuine.note(rel, data)
		}
		store, err := New(t.Context(), panicSource(t, root), discardLogger(), nil, schema.Governance{})
		if err != nil {
			t.Fatal(err)
		}
		assertParseCause(t, store.Current().Freshness().Blocked, true, "panic while parsing this note: ")
	})
	t.Run("read error resembles panic", func(t *testing.T) {
		root := t.TempDir()
		writePanicVault(t, root)
		store, err := New(t.Context(), parseCauseReadFailure{panicSource(t, root)}, discardLogger(), nil, schema.Governance{})
		if err != nil {
			t.Fatal(err)
		}
		assertParseCause(t, store.Current().Freshness().Blocked, false, "panic while parsing this note: a read error")
	})
}

func assertParseCause(t *testing.T, blocked []BlockedSource, want bool, reason string) {
	t.Helper()
	data, err := json.Marshal(blocked)
	if err != nil {
		t.Fatal(err)
	}
	var observed []struct {
		Path       string
		Reason     string
		ParsePanic bool
	}
	if err := json.Unmarshal(data, &observed); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 1 {
		t.Fatalf("caught: blocked set = %s", data)
	}
	got := observed[0]
	if got.Path != panicPoisoned || got.Reason != reason || got.ParsePanic != want {
		t.Errorf("caught: blocked cause = %+v, want path %q reason %q parse=%t", got, panicPoisoned, reason, want)
	}
}

// TestRetainedReadingNamesTheFailedParseAttempt observes the live failure on
// the same generation still serving its previous readable body.
func TestRetainedReadingNamesTheFailedParseAttempt(t *testing.T) {
	root := t.TempDir()
	writePanicVault(t, root)
	store, err := New(t.Context(), panicSource(t, root), discardLogger(), nil, schema.Governance{})
	if err != nil {
		t.Fatal(err)
	}
	whole := store.Current()
	breakStage(t, 0)
	writeNote(t, root, panicPoisoned, "---\ntitle: Poison\n---\npoisoned newly saved content\n")
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }
	store.rescan(t.Context())
	if store.Current() != whole {
		t.Fatal("caught: grace did not retain the whole generation")
	}
	held, ok := whole.Note(panicPoisoned)
	if !ok || held.Stale || !strings.Contains(held.Body, "poisoned as first read") {
		t.Fatalf("caught: retained reading = %+v", held)
	}
	assertParseCause(t, whole.Freshness().Blocked, true, "panic while parsing this note: "+panicValue)
	for tick := 1; tick <= 3; tick++ {
		clock = base.Add(time.Duration(tick) * scanInterval)
		store.rescan(t.Context())
	}
	carried, ok := store.Current().Note(panicPoisoned)
	if !ok || !carried.Stale || !strings.Contains(carried.Body, "poisoned as first read") {
		t.Fatalf("caught: published carried reading = %+v", carried)
	}
	assertParseCause(t, store.Current().Freshness().Blocked, true, "panic while parsing this note: "+panicValue)
}
