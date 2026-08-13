package judge

import (
	"strings"
	"testing"
)

// TestMembershipIsTheAcceptedOccurrence holds what "the course lists this note"
// means. A course lists a note by naming it as a lesson — one exact occurrence
// in the source, not "something on that lesson's line". A row can carry an
// embed, a same-file anchor, or a contextual mention beside its lesson link,
// and joining by line number hands all of them to the rules that reconcile a
// course against disk.
func TestMembershipIsTheAcceptedOccurrence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		// want is whether the archived note is reported as something the
		// course still points at.
		want bool
		// alsoWant is a rule the same run has to report, for a case whose
		// answer would otherwise be right for a reason it does not state.
		alsoWant string
	}{
		{
			// The lesson link opens the row and is the course's member; the
			// embed beside it shows a picture of a retired note, which is not
			// the course pointing at it.
			name: "an embed beside the lesson link",
			body: "## Main {sequence=primary}\n\n- [[Lesson]] ![[Retired]]\n",
			want: false,
		},
		{
			// The embed comes first, and an embed is something a reader sees,
			// so the lesson link no longer opens the row: the grammar refuses
			// the row and the course lists nothing here. The retired note is
			// not a member either way, and this holds the reason it is not.
			name:     "an embed before the lesson link refuses the row",
			body:     "## Main {sequence=primary}\n\n- ![[Retired]] [[Lesson]]\n",
			want:     false,
			alsoWant: "path.entry_noncanonical",
		},
		{
			// The row names two notes, so the grammar refuses it whole. Neither
			// is a member — picking the first would be the guess the contract
			// forbids.
			name: "a second live link makes the row refuse",
			body: "## Main {sequence=primary}\n\n- [[Lesson]] 參見 [[Retired]]\n",
			want: false,
		},
		{
			// The lesson itself is the retired note: that is the course
			// pointing at it, and the author has to hear about it.
			name: "the lesson link is the archived note",
			body: "## Main {sequence=primary}\n\n- [[Retired]]\n",
			want: true,
		},
		{
			// A same-file anchor names no note at all; it must not drag the
			// row's own lesson into anything, nor become a member itself.
			name: "a same-file anchor beside the lesson link",
			body: "## Main {sequence=primary}\n\n- [[Lesson]] [[#section]]\n\n## section\n\n說明。\n",
			want: false,
		},
		{
			// The row's only link is the retired note, but it does not open the
			// row, so the grammar refused the row and the course lists nothing
			// here. A refused row keeps its target readable, which must not be
			// mistaken for membership.
			name: "a refused row whose link is the archived note",
			body: "## Main {sequence=primary}\n\n- 補充 [[Retired]]\n",
			want: false,
		},
		{
			// A branch declared out of the course carries the same shape and
			// still lists nothing.
			name: "an embed beside a lesson in a branch declared none",
			body: "## Main {sequence=primary}\n\n- [[Lesson]]\n\n## Aside {sequence=none}\n\n- [[Other]] ![[Retired]]\n",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeSupersessionContract(t, root)
			writeJudgeNote(t, root, "Maps/Course.md",
				"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+tt.body)
			writeJudgeNote(t, root, "Writing/Lesson.md",
				"---\ntitle: Lesson\ntype: lesson\nstatus: ready\n---\nbody\n")
			writeJudgeNote(t, root, "Writing/Other.md",
				"---\ntitle: Other\ntype: lesson\nstatus: ready\n---\nbody\n")
			writeJudgeNote(t, root, "Writing/Retired.md",
				"---\ntitle: Retired\ntype: lesson\nstatus: archived\n---\nbody\n")

			out := string(runCheck(t, root))
			if got := strings.Contains(out, archivedNavigationRule); got != tt.want {
				t.Errorf("%s reported = %v, want %v for %q; findings:\n%s",
					archivedNavigationRule, got, tt.want, tt.body, out)
			}
			if tt.alsoWant != "" && !strings.Contains(out, tt.alsoWant) {
				t.Errorf("%s was not reported, so the row was left out for some other reason:\n%s",
					tt.alsoWant, out)
			}
		})
	}
}

// TestTheSameTargetTwiceKeepsTwoOccurrences holds that one name written twice
// is two occurrences. A course can refuse one row and accept another that names
// the same note, and the refused row must not inherit the accepted one's
// membership.
func TestTheSameTargetTwiceKeepsTwoOccurrences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSupersessionContract(t, root)
	// The archived note is written twice: once in a row the grammar refuses
	// because it names two notes, and once as a lesson of its own. Only the
	// second is the course pointing at it, so the author hears about it once.
	// Joining by name would make both occurrences members and say it twice.
	writeJudgeNote(t, root, "Maps/Course.md",
		"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+
			"## Main {sequence=primary}\n\n"+
			"- [[Retired]] 與 [[Lesson]]\n"+
			"- [[Retired]]\n")
	writeJudgeNote(t, root, "Writing/Lesson.md",
		"---\ntitle: Lesson\ntype: lesson\nstatus: ready\n---\nbody\n")
	writeJudgeNote(t, root, "Writing/Retired.md",
		"---\ntitle: Retired\ntype: lesson\nstatus: archived\n---\nbody\n")

	out := string(runCheck(t, root))
	if n := strings.Count(out, archivedNavigationRule); n != 1 {
		t.Errorf("%s reported %d times, want 1 — one of the two writings is not a lesson:\n%s",
			archivedNavigationRule, n, out)
	}
	if !strings.Contains(out, "path.entry_multi_target") {
		t.Errorf("the refused row was not reported at all:\n%s", out)
	}
}

// TestMembershipStillHoldsWhatTheCourseLists is the other direction, and the
// one that keeps the rest of this file from passing for the wrong reason.
//
// Every membership assertion above is satisfied by a join that matches nothing
// at all. A join that has drifted by a byte would leave the course listing
// nothing, and the checks that read membership would go quiet or start
// reporting the whole course as a disagreement with disk. These hold that a
// listed lesson is found: present on disk it is silent, missing it is reported
// once by the rule that owns a course's rows, and prose beside it is still
// ordinary prose.
func TestMembershipStillHoldsWhatTheCourseLists(t *testing.T) {
	t.Parallel()

	t.Run("a listed lesson that exists is not a disagreement with disk", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSupersessionContract(t, root)
		writeJudgeNote(t, root, "Maps/Course.md",
			"---\ntitle: Course\ntype: study-path\nstatus: ready\ndomain: golang\n---\n\n"+
				"## Main {sequence=primary}\n\n- [[Lesson]] ![[Picture]]\n")
		writeJudgeNote(t, root, "Writing/Lesson.md",
			"---\ntitle: Lesson\ntype: lesson\nstatus: ready\ndomain: golang\n---\nbody\n")
		writeJudgeNote(t, root, "Writing/Picture.md",
			"---\ntitle: Picture\ntype: concept\nstatus: ready\n---\nbody\n")

		out := string(runCheck(t, root))
		if strings.Contains(out, "map.disk_mismatch") {
			t.Errorf("a lesson the course lists and disk holds was reported missing — the join found nothing:\n%s", out)
		}
		if strings.Contains(out, "map.disk_unlisted") {
			t.Errorf("a lesson the course lists was reported unlisted — the join found nothing:\n%s", out)
		}
	})

	// A row can open with something that is not the link's first byte —
	// emphasis around the lesson's name is the ordinary case — and then the row
	// begins in one place and its target in another. A join anchored on where
	// the row begins finds no link at all there, and the course silently lists
	// nothing.
	t.Run("a lesson whose link is wrapped in emphasis is still listed", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSupersessionContract(t, root)
		writeJudgeNote(t, root, "Maps/Course.md",
			"---\ntitle: Course\ntype: study-path\nstatus: ready\ndomain: golang\n---\n\n"+
				"## Main {sequence=primary}\n\n- **[[Lesson]]** ![[Retired]]\n")
		writeJudgeNote(t, root, "Writing/Lesson.md",
			"---\ntitle: Lesson\ntype: lesson\nstatus: ready\ndomain: golang\n---\nbody\n")
		writeJudgeNote(t, root, "Writing/Retired.md",
			"---\ntitle: Retired\ntype: lesson\nstatus: archived\n---\nbody\n")

		out := string(runCheck(t, root))
		if strings.Contains(out, "map.disk_unlisted") {
			t.Errorf("the course lists this lesson but the join did not find it:\n%s", out)
		}
		if strings.Contains(out, archivedNavigationRule) {
			t.Errorf("the embed beside the lesson was read as the course pointing at it:\n%s", out)
		}
	})

	t.Run("a listed lesson that is missing is reported once by the course rule", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSupersessionContract(t, root)
		writeJudgeNote(t, root, "Maps/Course.md",
			"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+
				"## Main {sequence=primary}\n\n- [[Unwritten]] ![[Picture]]\n")
		writeJudgeNote(t, root, "Writing/Picture.md",
			"---\ntitle: Picture\ntype: concept\nstatus: ready\n---\nbody\n")

		out := string(runCheck(t, root))
		if n := strings.Count(out, "map.disk_mismatch"); n != 1 {
			t.Errorf("map.disk_mismatch reported %d times, want 1 — the course rule owns this row:\n%s", n, out)
		}
		if strings.Contains(out, `"rule_id":"link.broken"`) {
			t.Errorf("a course's own lesson row was also reported as an ordinary broken link:\n%s", out)
		}
	})

	// The prose names the same note the course lists. Membership is the
	// occurrence, so the row belongs to the course and the sentence does not,
	// and each is answered by the rule that owns it. Were membership the name,
	// the sentence would inherit the row's and fall silent.
	t.Run("prose naming a listed lesson keeps ordinary link health", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSupersessionContract(t, root)
		writeJudgeNote(t, root, "Maps/Course.md",
			"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+
				"關於 [[Unwritten]] 的說明。\n\n## Main {sequence=primary}\n\n- [[Unwritten]]\n")

		out := string(runCheck(t, root))
		if n := strings.Count(out, `"rule_id":"link.broken"`); n != 1 {
			t.Errorf("link.broken reported %d times, want 1 for the sentence:\n%s", n, out)
		}
		if n := strings.Count(out, "map.disk_mismatch"); n != 1 {
			t.Errorf("map.disk_mismatch reported %d times, want 1 for the row:\n%s", n, out)
		}
	})
}

// TestGeneralMapAnswersForEveryLink holds that narrowing a course to its
// accepted rows did not narrow a general map. A map declares no sequence, so
// every link it carries is still the map pointing somewhere — including one
// written beside another on the same line.
func TestGeneralMapAnswersForEveryLink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSupersessionContract(t, root)
	writeJudgeNote(t, root, "Maps/Topic.md",
		"---\ntitle: Topic\ntype: topic-map\nstatus: ready\n---\n\n- [[Lesson]] 與 [[Retired]]\n")
	writeJudgeNote(t, root, "Writing/Lesson.md",
		"---\ntitle: Lesson\ntype: lesson\nstatus: ready\n---\nbody\n")
	writeJudgeNote(t, root, "Writing/Retired.md",
		"---\ntitle: Retired\ntype: lesson\nstatus: archived\n---\nbody\n")

	out := string(runCheck(t, root))
	if !strings.Contains(out, archivedNavigationRule) {
		t.Errorf("a general map stopped answering for a link it carries:\n%s", out)
	}
}

// TestGapHeadingStillSoftensAMissingLesson holds the severity a planned section
// carries, and with it the recipe the authoring contract hands an author whose
// lesson is listed but unwritten. A lesson under a heading the author marked as
// a gap is a promise, not a fault, and reports as information rather than a
// warning. The link's own context has to survive the join that decides
// membership, and each mark the contract prints has to be one the tool reads.
func TestGapHeadingStillSoftensAMissingLesson(t *testing.T) {
	t.Parallel()
	// The marks the authoring contract tells an author to write. A mark that
	// stopped being read here is a contract promising something the tool no
	// longer does.
	gapMarks := []string{"缺口", "待補", "待寫", "待整理", "待建"}

	for _, mark := range gapMarks {
		t.Run("a branch marked "+mark+" informs", func(t *testing.T) {
			t.Parallel()
			assertMissingLessonSeverity(t,
				"## "+mark+" {sequence=primary}\n\n- [[Unwritten]]\n", `"severity":"info"`)
		})
	}

	t.Run("an unmarked branch warns", func(t *testing.T) {
		t.Parallel()
		assertMissingLessonSeverity(t,
			"## Main {sequence=primary}\n\n- [[Unwritten]]\n", `"severity":"warn"`)
	})

	t.Run("a following heading of the same level closes the gap", func(t *testing.T) {
		t.Parallel()
		assertMissingLessonSeverity(t,
			"## 缺口 {sequence=primary}\n\n- [[Planned]]\n\n## Main {sequence=primary}\n\n- [[Unwritten]]\n",
			`"severity":"warn"`)
	})

	t.Run("a concept planned elsewhere does not soften a course's own row", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeSupersessionContract(t, root)
		writeJudgeNote(t, root, "Concepts/Index.md",
			"---\ntitle: Index\ntype: concept\nstatus: ready\n---\n\n## 缺口\n\n- Unwritten\n")
		writeJudgeNote(t, root, "Maps/Course.md",
			"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+
				"## Main {sequence=primary}\n\n- [[Unwritten]]\n")
		assertSeverityOf(t, string(runCheck(t, root)), `"severity":"warn"`)
	})
}

// assertMissingLessonSeverity runs a course whose only lesson is unwritten and
// holds the severity its disk disagreement reports at.
func assertMissingLessonSeverity(t *testing.T, body, want string) {
	t.Helper()
	root := t.TempDir()
	writeSupersessionContract(t, root)
	writeJudgeNote(t, root, "Maps/Course.md",
		"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+body)
	assertSeverityOf(t, string(runCheck(t, root)), want)
}

// assertSeverityOf holds the severity of the disk disagreement reported for the
// lesson named Unwritten.
func assertSeverityOf(t *testing.T, out, want string) {
	t.Helper()
	var line string
	for l := range strings.SplitSeq(out, "\n") {
		if strings.Contains(l, "map.disk_mismatch") && strings.Contains(l, "Unwritten") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("the missing lesson was not reported at all:\n%s", out)
	}
	if !strings.Contains(line, want) {
		t.Errorf("severity = %s, want %s", line, want)
	}
}
