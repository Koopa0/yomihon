package snapshot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

type parseCauseReadFailure struct{ Source }

func (s parseCauseReadFailure) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	if entry.Path() == panicPoisoned {
		return nil, errors.New("panic while parsing this note: a read error")
	}
	return s.Source.ReadFile(ctx, entry)
}

func TestBlockedCauseComesFromTheParserBoundary(t *testing.T) {
	t.Parallel()
	for stage := range stagePanics {
		t.Run(stagePanics[stage].name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writePanicVault(t, root)
			store := newStore(openReader(t, root), discardLogger(), nil, schema.Ungoverned())
			breakStage(store, stage)
			if err := store.buildFirst(t.Context(), nil); err != nil {
				t.Fatal(err)
			}
			assertParseCause(t, store.Current().Freshness().Blocked, true, "panic while parsing this note: "+panicValue)
		})
	}
	t.Run("empty panic", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writePanicVault(t, root)
		store := newStore(openReader(t, root), discardLogger(), nil, schema.Ungoverned())
		genuine := store.parsers.note
		store.parsers.note = func(rel string, data []byte) *vault.Note {
			if strings.Contains(string(data), poisonMark) {
				panic("")
			}
			return genuine(rel, data)
		}
		if err := store.buildFirst(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		assertParseCause(t, store.Current().Freshness().Blocked, true, "panic while parsing this note: ")
	})
	t.Run("read error resembles panic", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writePanicVault(t, root)
		store := buildStore(t, parseCauseReadFailure{openReader(t, root)}, discardLogger(), nil)
		assertParseCause(t, store.Current().Freshness().Blocked, false, "panic while parsing this note: a read error")
	})
}

func assertParseCause(t *testing.T, blocked []BlockedSource, parsePanic bool, reason string) {
	t.Helper()
	want := []BlockedSource{{Path: panicPoisoned, Reason: reason, ParsePanic: parsePanic}}
	if diff := cmp.Diff(want, blocked); diff != "" {
		t.Errorf("blocked sources mismatch (-want +got):\n%s", diff)
	}
}
