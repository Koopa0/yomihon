package judge

import (
	"slices"
	"strings"
	"testing"
)

// parsePanicValue is what the injected parse panics with, so a test can look for
// it in what the run reports.
const parsePanicValue = "injected parser panic"

// panicsParsing is the hook that makes the parse of one note panic while every
// other note parses as it always does. It stands in for a third-party parser
// that cannot survive some input, which no test can produce on demand.
func panicsParsing(path string) actionHooks {
	return actionHooks{parseNote: func(rel string, data []byte, marks plannedMarks) note {
		if rel == path {
			panic(parsePanicValue)
		}
		return parseNoteWithMarks(rel, data, marks)
	}}
}

// mustNotPanic runs step and turns a panic that escapes it into the test
// failure it is, rather than a crashed test binary.
func mustNotPanic(tb testing.TB, step string, run func()) {
	tb.Helper()
	defer func() {
		if rec := recover(); rec != nil {
			tb.Fatalf("%s: a panic parsing one note escaped the run: %v", step, rec)
		}
	}()
	run()
}

// TestAPanicParsingOneNoteIsReportedUnreadableLikeAFileNothingCouldOpen is the
// lock on the check command surviving a note its parser cannot. The note is
// reported as the same finding a file nothing could be read from is, at the same
// weight, and the run goes on to judge the rest of the vault with the same rules
// held back — because the note nobody parsed may hold what they conclude is
// nowhere.
//
// The fixture is the one the sealed-file tests use, with the note they seal left
// readable on disk: the hole here is made by the parse, so this holds as root,
// where a file cannot be sealed.
func TestAPanicParsingOneNoteIsReportedUnreadableLikeAFileNothingCouldOpen(t *testing.T) {
	t.Parallel()

	root := judgeFixtureRoot(t, unreadableFixture)
	whole, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check(whole vault) error = %v; the run below proves nothing without it", err)
	}

	var findings []Finding
	mustNotPanic(t, "check", func() {
		a, openErr := openAction(t.Context(), root, panicsParsing(sealedNote))
		if openErr != nil {
			t.Fatalf("openAction() error = %v, want a judgement of the notes that parse", openErr)
		}
		var checkErr error
		findings, checkErr = checkAction(a, nil, false)
		if checkErr != nil {
			t.Fatalf("checkAction() error = %v", checkErr)
		}
		if finishErr := a.finish(); finishErr != nil {
			t.Fatalf("finish() error = %v", finishErr)
		}
	})

	var notices []Finding
	for i := range findings {
		if findings[i].RuleID == unreadableRule {
			notices = append(notices, findings[i])
		}
	}
	if len(notices) != 1 {
		t.Fatalf("%d %q findings, want exactly one for the note whose parse panicked: %+v", len(notices), unreadableRule, notices)
	}
	notice := notices[0]
	if notice.Path != sealedNote || notice.Severity != SeverityError {
		t.Errorf("notice = {Path:%q Severity:%v}, want the panicked note at error weight", notice.Path, notice.Severity)
	}
	if !strings.Contains(notice.Evidence, parsePanicValue) {
		t.Errorf("notice evidence = %q, want the panic value %q in it", notice.Evidence, parsePanicValue)
	}

	wholeRules, panickedRules := ruleSet(whole, ""), ruleSet(findings, "")
	for _, rule := range withheldOnPartialCorpus {
		if !wholeRules[rule] {
			t.Errorf("the fixture never trips %q, so this test cannot tell whether withholding it does anything", rule)
		}
		if panickedRules[rule] {
			t.Errorf("%q answered over a vault one note of which was never parsed, and that note may hold what it says is nowhere", rule)
		}
	}
	for rule := range ruleSet(whole, sealedNote) {
		if slices.Contains(withheldOnPartialCorpus, rule) || panickedRules[rule] {
			continue
		}
		t.Errorf("%q was reported over the whole vault and vanished over the one with a note that did not parse, though it reads one note rather than the vault", rule)
	}
}

// TestAPanicParsingOneNoteExitsAsAnUnreadableFileDoes pins the exit code of the
// same run: a finding gates by name like every other, so the panicked note exits
// 0 unless the caller denied it, and 1 when they did.
func TestAPanicParsingOneNoteExitsAsAnUnreadableFileDoes(t *testing.T) {
	t.Parallel()

	root := judgeFixtureRoot(t, unreadableFixture)
	tests := []struct {
		name string
		deny []string
		want int
	}{
		{name: "nothing denied", want: 0},
		{name: "the rule denied by name", deny: []string{string(unreadableRule)}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var prepared preparedCommand
			mustNotPanic(t, "check", func() {
				var err error
				prepared, err = prepareCheckWithHooks(t.Context(),
					&CheckOptions{Root: root, Deny: tt.deny, Format: FormatJSON}, panicsParsing(sealedNote))
				if err != nil {
					t.Fatalf("prepareCheckWithHooks() error = %v, want a judgement of what parsed", err)
				}
				if err = prepared.finish(); err != nil {
					t.Fatalf("finish() error = %v", err)
				}
			})
			if prepared.exit != tt.want {
				t.Errorf("exit = %d, want %d", prepared.exit, tt.want)
			}
		})
	}
}

// TestAPanicParsingOneNoteRefusesTheCommandsThatCannotAnswerOverAHole holds the
// other two commands to what they do for a file nothing could open: a census
// and an answer that no note carries a name are conclusions about the whole
// vault, so each says which note it could not use instead of answering.
func TestAPanicParsingOneNoteRefusesTheCommandsThatCannotAnswerOverAHole(t *testing.T) {
	t.Parallel()

	root := judgeFixtureRoot(t, unreadableFixture)
	for _, command := range []string{"coverage", "exists"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			var err error
			mustNotPanic(t, command, func() {
				_, err = runPrepared(t.Context(), t, command, root, "a name no note answers to", panicsParsing(sealedNote), nil)
			})
			if err == nil {
				t.Fatalf("%s answered over a vault one note of which was never parsed", command)
			}
			for _, want := range []string{sealedNote, parsePanicValue} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%s refusal = %q, want %q named in it", command, err, want)
				}
			}
		})
	}
}
