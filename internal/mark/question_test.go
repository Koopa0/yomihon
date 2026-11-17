package mark_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

func aQuestion() *mark.Question {
	return &mark.Question{
		RelPath: "Notes/source.md",
		Anchor:  "section-one",
		At:      time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC),
	}
}

func TestQuestionTogglesOnlyTheChosenPathAndAnchor(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	empty, err := file.Questions()
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("missing file = %v, %v; want empty non-nil list", empty, err)
	}
	first := aQuestion()
	second := *first
	second.Anchor = "section-two"
	third := *first
	third.RelPath = "Notes/other.md"
	for _, kept := range []*mark.Question{first, &second, &third} {
		if added, toggleErr := file.ToggleQuestion(kept, admitAny); toggleErr != nil || !added {
			t.Fatalf("add %v = %v, %v", kept, added, toggleErr)
		}
	}
	// Time is not identity: revisiting the first location removes it.
	first.At = first.At.Add(time.Hour)
	if added, toggleErr := file.ToggleQuestion(first, admitAny); toggleErr != nil || added {
		t.Fatalf("remove first = %v, %v", added, toggleErr)
	}
	held, err := file.Questions()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Question{second, third}, held); diff != "" {
		t.Fatalf("unrelated marks changed (-want +got):\n%s", diff)
	}
	for _, kept := range []*mark.Question{&second, &third} {
		if added, toggleErr := file.ToggleQuestion(kept, admitAny); toggleErr != nil || added {
			t.Fatalf("remove %v = %v, %v", kept, added, toggleErr)
		}
	}
	data, err := os.ReadFile(file.QuestionPath())
	if err != nil || string(data) != "[]\n" {
		t.Fatalf("empty file = %q, %v; want []", data, err)
	}
}

func TestQuestionStoresOnlyPathAnchorAndTimeOutsideBookmark(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	if err := file.SetBookmark(aPlace()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(file.Path())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.ToggleQuestion(aQuestion(), admitAny); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(file.Path())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("question changed bookmark bytes: %v", err)
	}
	data, err := os.ReadFile(file.QuestionPath())
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
	// The unchanged bookmark writer stands in for an older binary.
	if err = file.SetBookmark(aPlace()); err != nil {
		t.Fatal(err)
	}
	after, err = os.ReadFile(file.QuestionPath())
	if err != nil || !bytes.Equal(data, after) {
		t.Fatalf("bookmark writer changed question bytes: %v", err)
	}
	info, err := os.Stat(file.QuestionPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("question permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestCorruptQuestionsAreNeverReplaced(t *testing.T) {
	t.Parallel()
	const entry = `{"path":"Notes/source.md","anchor":"one","time":"2026-09-22T10:00:00Z"}`
	for _, damaged := range []string{
		"", "[", "null", "{}", "[null]", "[] []", "[] trailing",
		`[{"path":"Notes/source.md"}]`,
		`[{"path":"../source.md","anchor":"one","time":"2026-09-22T10:00:00Z"}]`,
		`[{"path":"Notes/source.md","anchor":"one","time":"invalid"}]`,
		`[{"path":"Notes/source.md","anchor":"one","time":"2026-09-22T10:00:00Z","content":"private"}]`,
		"[" + entry + "," + entry + "]",
		`[{"path":"Source.md\n2026-12-31  thought   Fake.md  Forged  (started)","anchor":"","time":"2026-09-22T10:00:00Z"}]`,
		`[{"path":"Notes/\u001b]0;pwned\u0007.md","anchor":"","time":"2026-09-22T10:00:00Z"}]`,
	} {
		t.Run(damaged, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			if err := os.MkdirAll(filepath.Dir(file.QuestionPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.QuestionPath(), []byte(damaged), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Questions(); !errors.Is(err, mark.ErrUnreadableQuestions) {
				t.Fatalf("read damaged file = %v; want unreadable", err)
			}
			if _, err := file.ToggleQuestion(aQuestion(), admitAny); !errors.Is(err, mark.ErrUnreadableQuestions) || errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("toggle damaged file = %v; want unreadable", err)
			}
			data, err := os.ReadFile(file.QuestionPath())
			if err != nil || string(data) != damaged {
				t.Fatalf("damaged file changed to %q: %v", data, err)
			}
		})
	}
}

func TestQuestionReadFailureDoesNotLookEmpty(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	if err := os.MkdirAll(file.QuestionPath(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Questions(); !errors.Is(err, mark.ErrUnreadableQuestions) {
		t.Fatalf("read directory = %v; want unreadable", err)
	}
	if _, err := file.ToggleQuestion(aQuestion(), admitAny); !errors.Is(err, mark.ErrUnreadableQuestions) {
		t.Fatalf("toggle over directory = %v; want unreadable", err)
	}
}

func TestInvalidQuestionIsRefusedBeforeReadingStorage(t *testing.T) {
	t.Parallel()
	if _, err := newFile(t).ToggleQuestion(nil, admitAny); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("nil toggle = %v; want invalid submission", err)
	}
	for _, tt := range []struct {
		name  string
		spoil func(*mark.Question)
	}{
		{"empty path", func(u *mark.Question) { u.RelPath = "" }},
		{"absolute path", func(u *mark.Question) { u.RelPath = "/tmp/source.md" }},
		{"traversal", func(u *mark.Question) { u.RelPath = "../source.md" }},
		{"dot", func(u *mark.Question) { u.RelPath = "." }},
		{"long path", func(u *mark.Question) { u.RelPath = strings.Repeat("a", 1025) }},
		{"decomposed path", func(u *mark.Question) { u.RelPath = "Notes/\u30cf\u309a.md" }},
		{"long anchor", func(u *mark.Question) { u.Anchor = strings.Repeat("a", 257) }},
		{"fragment in anchor", func(u *mark.Question) { u.Anchor = "one#two" }},
		{"control in anchor", func(u *mark.Question) { u.Anchor = "one\ntwo" }},
		{"line break in path", func(u *mark.Question) { u.RelPath = "Notes/a\nb.md" }},
		{"escape in path", func(u *mark.Question) { u.RelPath = "Notes/\x1b[2J.md" }},
		{"no time", func(u *mark.Question) { u.At = time.Time{} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			// A directory guarantees storage reads would fail with another error.
			if err := os.MkdirAll(file.QuestionPath(), 0o700); err != nil {
				t.Fatal(err)
			}
			kept := aQuestion()
			tt.spoil(kept)
			if _, err := file.ToggleQuestion(kept, admitAny); !errors.Is(err, mark.ErrInvalid) {
				t.Fatalf("invalid toggle = %v; want invalid submission", err)
			}
		})
	}
}

func TestQuestionsSurviveRestartAndRemainVaultLocal(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	file := newFileFor(t, config, "/vaults/one")
	kept := aQuestion()
	kept.Anchor = ""
	if _, err := file.ToggleQuestion(kept, admitAny); err != nil {
		t.Fatal(err)
	}
	reopened := newFileFor(t, config, "/vaults/one")
	held, err := reopened.Questions()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]mark.Question{*kept}, held); diff != "" {
		t.Fatalf("reopened marks (-want +got):\n%s", diff)
	}
	other := newFileFor(t, config, "/vaults/two")
	if marks, readErr := other.Questions(); readErr != nil || len(marks) != 0 {
		t.Fatalf("other vault sees %v, %v", marks, readErr)
	}
	if err = os.Remove(file.QuestionPath()); err != nil {
		t.Fatal(err)
	}
	if marks, readErr := reopened.Questions(); readErr != nil || len(marks) != 0 {
		t.Fatalf("deleted file still reports %v, %v", marks, readErr)
	}
}

func TestConcurrentQuestionTogglesKeepEveryDistinctLocation(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	var writers sync.WaitGroup
	for _, anchor := range []string{"one", "two", "three", "four"} {
		writers.Go(func() {
			kept := aQuestion()
			kept.Anchor = anchor
			if _, err := file.ToggleQuestion(kept, admitAny); err != nil {
				t.Errorf("toggle %s: %v", anchor, err)
			}
		})
	}
	writers.Wait()
	held, err := file.Questions()
	if err != nil || len(held) != 4 {
		t.Fatalf("concurrent writes retained %v, %v; want all four", held, err)
	}
}

func admitAny(*mark.Question) bool { return true }

func TestToggleQuestionRefusesInvalidUTF8ThenStaysReadable(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	// JSON stores an invalid byte as U+FFFD, so two identical requests used to
	// write the same pair twice and make every later read fail as a duplicate.
	for range 2 {
		kept := aQuestion()
		kept.Anchor = "x\xff"
		if _, err := file.ToggleQuestion(kept, admitAny); !errors.Is(err, mark.ErrInvalid) {
			t.Fatalf("ToggleQuestion(invalid UTF-8 anchor) error = %v, want ErrInvalid", err)
		}
	}
	held, err := file.Questions()
	if err != nil || len(held) != 0 {
		t.Fatalf("Questions() = %v, %v; want a readable empty list", held, err)
	}
	if _, err = file.ToggleQuestion(aQuestion(), admitAny); err != nil {
		t.Fatalf("a later valid toggle failed: %v", err)
	}
}

func TestToggleQuestionRefusesBeyondTheCapButStillClears(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	for i := range mark.MaxQuestions {
		kept := aQuestion()
		kept.Anchor = "a" + strconv.Itoa(i)
		if added, err := file.ToggleQuestion(kept, admitAny); err != nil || !added {
			t.Fatalf("mark %d = %v, %v", i, added, err)
		}
	}
	over := aQuestion()
	over.Anchor = "over"
	if _, err := file.ToggleQuestion(over, admitAny); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("mark past the cap error = %v, want ErrInvalid", err)
	}
	held, err := file.Questions()
	if err != nil || len(held) != mark.MaxQuestions {
		t.Fatalf("after the refusal: %d marks, %v", len(held), err)
	}
	first := aQuestion()
	first.Anchor = "a0"
	if added, err := file.ToggleQuestion(first, admitAny); err != nil || added {
		t.Fatalf("clearing at the cap = %v, %v", added, err)
	}
	if added, err := file.ToggleQuestion(over, admitAny); err != nil || !added {
		t.Fatalf("adding after room was made = %v, %v", added, err)
	}
}

// The writer never keeps more than MaxQuestions, so a file that holds more,
// or more bytes than that many marks can take, was not written by it and is
// read as damaged rather than handed on whole.
func TestStoredQuestionsBeyondTheCapAreDamaged(t *testing.T) {
	t.Parallel()
	entries := make([]string, mark.MaxQuestions+1)
	for i := range entries {
		entries[i] = `{"path":"Notes/source.md","anchor":"a` + strconv.Itoa(i) + `","time":"2026-09-22T10:00:00Z"}`
	}
	for _, tt := range []struct {
		name, data string
	}{
		{name: "one mark past the cap", data: "[" + strings.Join(entries, ",") + "]"},
		{name: "more bytes than the cap can take", data: "[" + strings.Repeat(" ", 8<<20) + "]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file := newFile(t)
			if err := os.MkdirAll(filepath.Dir(file.QuestionPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.QuestionPath(), []byte(tt.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if held, err := file.Questions(); !errors.Is(err, mark.ErrUnreadableQuestions) {
				t.Errorf("caught: %s read as %d marks, %v; want unreadable", tt.name, len(held), err)
			}
		})
	}
	// The cap itself is still a file the writer could have written.
	file := newFile(t)
	if err := os.MkdirAll(filepath.Dir(file.QuestionPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.QuestionPath(), []byte("["+strings.Join(entries[:mark.MaxQuestions], ",")+"]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if held, err := file.Questions(); err != nil || len(held) != mark.MaxQuestions {
		t.Errorf("a file at the cap read as %d marks, %v", len(held), err)
	}
}

func TestToggleQuestionAdmitsOnlyWhenAddingAndWithoutAdmitterRefuses(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	if _, err := file.ToggleQuestion(aQuestion(), nil); !errors.Is(err, mark.ErrInvalid) {
		t.Fatalf("no admitter error = %v, want ErrInvalid", err)
	}
	if _, err := file.ToggleQuestion(aQuestion(), admitAny); err != nil {
		t.Fatal(err)
	}
	// The place has since gone; clearing the mark must still work.
	refuse := func(*mark.Question) bool { return false }
	if added, err := file.ToggleQuestion(aQuestion(), refuse); err != nil || added {
		t.Fatalf("clearing a stale mark = %v, %v", added, err)
	}
}
