package mark_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

func anUncertainty() *mark.Uncertainty {
	return &mark.Uncertainty{
		RelPath: "Notes/source.md",
		Anchor:  "section-one",
		At:      time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

func TestUncertaintyTogglesOnlyTheChosenPathAndAnchor(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	empty, err := file.Uncertainties()
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("missing file = %v, %v; want empty non-nil list", empty, err)
	}
	first := anUncertainty()
	second := *first
	second.Anchor = "section-two"
	third := *first
	third.RelPath = "Notes/other.md"
	for _, kept := range []*mark.Uncertainty{first, &second, &third} {
		if added, toggleErr := file.ToggleUncertainty(kept); toggleErr != nil || !added {
			t.Fatalf("add %v = %v, %v", kept, added, toggleErr)
		}
	}
	// Time is not identity: revisiting the first location removes it.
	first.At = first.At.Add(time.Hour)
	if added, toggleErr := file.ToggleUncertainty(first); toggleErr != nil || added {
		t.Fatalf("remove first = %v, %v", added, toggleErr)
	}
	held, err := file.Uncertainties()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Uncertainty{second, third}, held); diff != "" {
		t.Fatalf("unrelated marks changed (-want +got):\n%s", diff)
	}
	for _, kept := range []*mark.Uncertainty{&second, &third} {
		if added, toggleErr := file.ToggleUncertainty(kept); toggleErr != nil || added {
			t.Fatalf("remove %v = %v, %v", kept, added, toggleErr)
		}
	}
	data, err := os.ReadFile(file.UncertaintyPath())
	if err != nil || string(data) != "[]\n" {
		t.Fatalf("empty file = %q, %v; want []", data, err)
	}
}

func TestUncertaintyStoresOnlyPathAnchorAndTimeOutsideContinuation(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	if err := file.SetContinuation(aPlace()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(file.Path())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.ToggleUncertainty(anUncertainty()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(file.Path())
	if err != nil || string(before) != string(after) {
		t.Fatalf("uncertainty changed continuation bytes: %v", err)
	}
	data, err := os.ReadFile(file.UncertaintyPath())
	if err != nil {
		t.Fatal(err)
	}
	var held []map[string]json.RawMessage
	if err = json.Unmarshal(data, &held); err != nil || len(held) != 1 {
		t.Fatalf("stored array = %s, %v", data, err)
	}
	if len(held[0]) != 3 || held[0]["path"] == nil || held[0]["anchor"] == nil || held[0]["time"] == nil {
		t.Fatalf("stored fields = %s; want only path, anchor, time", data)
	}
	// The unchanged continuation writer stands in for an older binary.
	if err = file.SetContinuation(aPlace()); err != nil {
		t.Fatal(err)
	}
	after, err = os.ReadFile(file.UncertaintyPath())
	if err != nil || string(data) != string(after) {
		t.Fatalf("continuation writer changed uncertainty bytes: %v", err)
	}
	info, err := os.Stat(file.UncertaintyPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("uncertainty permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestCorruptUncertaintiesAreNeverReplaced(t *testing.T) {
	t.Parallel()
	const entry = `{"path":"Notes/source.md","anchor":"one","time":"2026-09-22T10:00:00Z"}`
	for _, damaged := range []string{
		"", "[", "null", "{}", "[null]", "[] []", "[] trailing",
		`[{"path":"Notes/source.md"}]`,
		`[{"path":"../source.md","anchor":"one","time":"2026-09-22T10:00:00Z"}]`,
		`[{"path":"Notes/source.md","anchor":"one","time":"invalid"}]`,
		`[{"path":"Notes/source.md","anchor":"one","time":"2026-09-22T10:00:00Z","content":"private"}]`,
		"[" + entry + "," + entry + "]",
	} {
		t.Run(damaged, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			if err := os.MkdirAll(filepath.Dir(file.UncertaintyPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.UncertaintyPath(), []byte(damaged), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Uncertainties(); !errors.Is(err, mark.ErrUnreadableUncertainties) {
				t.Fatalf("read damaged file = %v; want unreadable", err)
			}
			if _, err := file.ToggleUncertainty(anUncertainty()); !errors.Is(err, mark.ErrUnreadableUncertainties) || errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("toggle damaged file = %v; want unreadable", err)
			}
			data, err := os.ReadFile(file.UncertaintyPath())
			if err != nil || string(data) != damaged {
				t.Fatalf("damaged file changed to %q: %v", data, err)
			}
		})
	}
}

func TestUncertaintyReadFailureDoesNotLookEmpty(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	if err := os.MkdirAll(file.UncertaintyPath(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Uncertainties(); !errors.Is(err, mark.ErrUnreadableUncertainties) {
		t.Fatalf("read directory = %v; want unreadable", err)
	}
	if _, err := file.ToggleUncertainty(anUncertainty()); !errors.Is(err, mark.ErrUnreadableUncertainties) {
		t.Fatalf("toggle over directory = %v; want unreadable", err)
	}
}

func TestInvalidUncertaintyIsRefusedBeforeReadingStorage(t *testing.T) {
	t.Parallel()
	if _, err := newFile(t).ToggleUncertainty(nil); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("nil toggle = %v; want invalid submission", err)
	}
	for _, tt := range []struct {
		name  string
		spoil func(*mark.Uncertainty)
	}{
		{"empty path", func(u *mark.Uncertainty) { u.RelPath = "" }},
		{"absolute path", func(u *mark.Uncertainty) { u.RelPath = "/tmp/source.md" }},
		{"traversal", func(u *mark.Uncertainty) { u.RelPath = "../source.md" }},
		{"dot", func(u *mark.Uncertainty) { u.RelPath = "." }},
		{"long path", func(u *mark.Uncertainty) { u.RelPath = strings.Repeat("a", 1025) }},
		{"decomposed path", func(u *mark.Uncertainty) { u.RelPath = "Notes/\u30cf\u309a.md" }},
		{"long anchor", func(u *mark.Uncertainty) { u.Anchor = strings.Repeat("a", 257) }},
		{"fragment in anchor", func(u *mark.Uncertainty) { u.Anchor = "one#two" }},
		{"control in anchor", func(u *mark.Uncertainty) { u.Anchor = "one\ntwo" }},
		{"no time", func(u *mark.Uncertainty) { u.At = time.Time{} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			// A directory guarantees storage reads would fail with another error.
			if err := os.MkdirAll(file.UncertaintyPath(), 0o700); err != nil {
				t.Fatal(err)
			}
			kept := anUncertainty()
			tt.spoil(kept)
			if _, err := file.ToggleUncertainty(kept); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("invalid toggle = %v; want invalid submission", err)
			}
		})
	}
}

func TestUncertaintiesSurviveRestartAndRemainVaultLocal(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	file := newFileFor(t, config, "/vaults/one")
	kept := anUncertainty()
	kept.Anchor = ""
	if _, err := file.ToggleUncertainty(kept); err != nil {
		t.Fatal(err)
	}
	reopened := newFileFor(t, config, "/vaults/one")
	held, err := reopened.Uncertainties()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Uncertainty{*kept}, held); diff != "" {
		t.Fatalf("reopened marks (-want +got):\n%s", diff)
	}
	other := newFileFor(t, config, "/vaults/two")
	if marks, readErr := other.Uncertainties(); readErr != nil || len(marks) != 0 {
		t.Fatalf("other vault sees %v, %v", marks, readErr)
	}
	if err = os.Remove(file.UncertaintyPath()); err != nil {
		t.Fatal(err)
	}
	if marks, readErr := reopened.Uncertainties(); readErr != nil || len(marks) != 0 {
		t.Fatalf("deleted file still reports %v, %v", marks, readErr)
	}
}

func TestConcurrentUncertaintyTogglesKeepEveryDistinctLocation(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	var writers sync.WaitGroup
	for _, anchor := range []string{"one", "two", "three", "four"} {
		writers.Go(func() {
			kept := anUncertainty()
			kept.Anchor = anchor
			if _, err := file.ToggleUncertainty(kept); err != nil {
				t.Errorf("toggle %s: %v", anchor, err)
			}
		})
	}
	writers.Wait()
	held, err := file.Uncertainties()
	if err != nil || len(held) != 4 {
		t.Fatalf("concurrent writes retained %v, %v; want all four", held, err)
	}
}
