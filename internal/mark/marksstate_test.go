package mark_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// mkNoteAnswer is an answer with the margin note the page drew for it.
type mkNoteAnswer struct {
	mkAnswer

	Note string `json:"note"`
}

// A mark is one thing in three states: turned from one into another it keeps its
// id, its words and the time it was made, and a resolution it had survives a
// stay in another state.
func TestMkPatchTurnsOneMarkIntoEachStateAndKeepsItsID(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	made := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkNote+`","anchor":"section-one","quote":"the topic particle marks"}`))

	patch := func(prev mkAnswer, body string) mkAnswer {
		t.Helper()
		response := r.do(http.MethodPatch, "/mark/"+made.ID, `{"seen":`+mkSeenJSON(t, prev.Seen)+","+body+`}`)
		mkStatus(t, response, http.StatusOK)
		return mkDecode[mkAnswer](t, response)
	}

	thought := patch(made, `"kind":"thought","body":"it marks the topic"`)
	asked := patch(thought, `"kind":"question"`)
	resolved := patch(asked, `"resolved":true`)
	back := patch(resolved, `"kind":"thought"`)
	again := patch(back, `"kind":"question"`)
	bare := patch(again, `"kind":"highlight","body":""`)

	type shape struct {
		Kind     machinemarks.Kind
		Body     string
		Resolved bool
	}
	for i, tt := range []struct {
		got  mkAnswer
		want shape
	}{
		{thought, shape{machinemarks.Thought, "it marks the topic", false}},
		{asked, shape{machinemarks.Question, "it marks the topic", false}},
		{resolved, shape{machinemarks.Question, "it marks the topic", true}},
		{back, shape{machinemarks.Thought, "it marks the topic", true}},
		{again, shape{machinemarks.Question, "it marks the topic", true}},
		{bare, shape{machinemarks.Highlight, "", true}},
	} {
		got := shape{tt.got.Kind, tt.got.Body, !tt.got.Resolved.IsZero()}
		if tt.got.ID != made.ID || !tt.got.At.Equal(made.At) || tt.got.Quote != "the topic particle marks" {
			t.Errorf("step %d lost who the mark is: %+v", i, tt.got.Mark)
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("step %d (-want +got):\n%s", i, diff)
		}
	}
	if !again.Resolved.Equal(resolved.Resolved) {
		t.Errorf("a resolution moved from %v to %v across two changes of state", resolved.Resolved, again.Resolved)
	}
	if got := len(r.mkLines()); got != 1 {
		t.Errorf("the vault holds %d lines, want the one a change replaces in place", got)
	}
}

// What a state does not hold is refused, and nothing is written: a highlight is
// a passage and says nothing of its own, a thought says something, and a whole
// section is never a highlight.
func TestMkPatchRefusesAStateTheMarkCannotBeIn(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	highlight := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkNote+`","quote":"the topic particle marks"}`))
	section := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","anchor":"section-one","body":"about all of it"}`))
	question := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","quote":"the topic particle marks","body":"why?"}`))

	for _, tt := range []struct {
		name string
		from mkAnswer
		body string
	}{
		{"a highlight made a thought with no words", highlight, `"kind":"thought"`},
		{"a section made a highlight", section, `"kind":"highlight","body":""`},
		{"a question with words made a highlight that keeps them", question, `"kind":"highlight"`},
		{"a state nobody has", highlight, `"kind":"card"`},
		{"a highlight resolved by turning it into one", question, `"kind":"highlight","body":"","resolved":true`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := tree(t, r.root)
			response := r.do(http.MethodPatch, "/mark/"+tt.from.ID, `{"seen":`+mkSeenJSON(t, tt.from.Seen)+","+tt.body+`}`)
			mkStatus(t, response, http.StatusUnprocessableEntity)
			if diff := cmp.Diff(before, tree(t, r.root)); diff != "" {
				t.Errorf("a refused change wrote:\n%s", diff)
			}
		})
	}
}

// A request sent again after the mark was changed is answered with the mark as it
// is now, and never writes the words it carried over the newer ones.
func TestMkAMarkSentAgainAfterItChangedIsAnsweredAsItIsNow(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	const id = "00112233445566778899aabbccddeeff"
	thought := `{"id":"` + id + `","kind":"thought","path":"` + mkNote + `","anchor":"section-one","body":"first words"}`
	created := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", thought))
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+id, `{"seen":`+mkSeenJSON(t, created.Seen)+`,"kind":"thought","body":"newer words"}`), http.StatusOK)
	before := len(r.mkLines())

	again := r.do(http.MethodPost, "/mark", thought)
	mkStatus(t, again, http.StatusOK)
	if got := mkDecode[mkAnswer](t, again); got.Body != "newer words" {
		t.Errorf("the request sent again was answered with %q, want the words the mark has now, %q", got.Body, "newer words")
	}
	if got := len(r.mkLines()); got != before {
		t.Errorf("sending it again wrote %d lines, want none", got-before)
	}
}

// A page that makes the id of a mark can send the same request again, when no
// answer reached it, and never keeps the mark twice: not for a thought, which
// is always new, and not for requests that arrive together.
func TestMkAMarkSentAgainWithItsIDIsKeptOnce(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	const id = "0123456789abcdef0123456789abcdef"
	thought := `{"id":"` + id + `","kind":"thought","path":"` + mkNote + `","anchor":"section-one","body":"once"}`

	first := r.do(http.MethodPost, "/mark", thought)
	mkStatus(t, first, http.StatusCreated)
	again := r.do(http.MethodPost, "/mark", thought)
	mkStatus(t, again, http.StatusOK)
	if got := mkDecode[mkAnswer](t, again); got.ID != id || got.Body != "once" {
		t.Fatalf("the same thought sent again was answered %+v, want the one kept", got.Mark)
	}
	if got := len(r.mkLines()); got != 1 {
		t.Fatalf("a thought sent twice wrote %d lines, want 1", got)
	}

	const other = "fedcba9876543210fedcba9876543210"
	crowd := `{"id":"` + other + `","kind":"thought","path":"` + mkNote + `","anchor":"section-one","body":"together"}`
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Go(func() { codes[i] = r.do(http.MethodPost, "/mark", crowd).Code })
	}
	wg.Wait()
	created := 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusOK:
		default:
			t.Errorf("a request sent with the same id together was answered %d, want 200 or 201", code)
		}
	}
	if created != 1 {
		t.Errorf("%d of the requests created the mark, want 1", created)
	}
	if got := len(r.mkLines()); got != 2 {
		t.Errorf("the vault holds %d lines, want 2: one for each mark", got)
	}

	for _, bad := range []string{"short", strings.ToUpper(id), id + "0"} {
		mkStatus(t, r.do(http.MethodPost, "/mark", `{"id":"`+bad+`","kind":"thought","path":"`+mkNote+`","body":"x"}`), http.StatusUnprocessableEntity)
	}
}

// The answer to a change carries the note the page draws for the mark, in the
// language asked for, so a page that has just made a mark draws what a page
// that arrives with it draws; a deleted mark has none.
func TestMkAnswersCarryTheNoteThePageDraws(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	response := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","anchor":"section-one","body":"x"}`, "en")
	mkStatus(t, response, http.StatusCreated)
	made := mkDecode[mkNoteAnswer](t, response)
	if want := `<aside data-kind="thought" data-placement="section" lang="en"></aside>`; made.Note != want {
		t.Errorf("note = %q, want %q", made.Note, want)
	}
	deleted := mkDecode[mkNoteAnswer](t, r.do(http.MethodDelete, "/mark/"+made.ID, `{"seen":`+mkSeenJSON(t, made.Seen)+`}`))
	if deleted.Note != "" || !deleted.Deleted {
		t.Errorf("a deleted mark was answered %+v, want it deleted and with no note", deleted)
	}
}

// Live is the marks of one note that are not deleted, the newest value of each,
// in the order they were made.
func TestKeptLiveListsTheMarksOfOneNoteThatAreNotDeleted(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	first := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkNote+`","quote":"the topic particle marks"}`))
	second := r.mkThought("words")
	gone := r.mkThought("deleted")
	mkStatus(t, r.do(http.MethodDelete, "/mark/"+gone.ID, `{"seen":`+mkSeenJSON(t, gone.Seen)+`}`), http.StatusOK)
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+second.ID, `{"seen":`+mkSeenJSON(t, second.Seen)+`,"body":"newer"}`), http.StatusOK)
	elsewhere := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkSecret+`","quote":"the topic particle marks"}`))

	live, err := r.kept.Live(mkNote)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := range live {
		got = append(got, live[i].ID+":"+live[i].Body)
	}
	want := []string{first.ID + ":", second.ID + ":newer"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Live() (-want +got):\n%s", diff)
	}
	if elsewhere.ID == "" {
		t.Fatal("a mark of another note was not kept")
	}
}

// A reader who has marked nothing since the upgrade still has the marks of the
// earlier file, and the page of a note lists those of that note which are not
// deleted, before the first write takes them in.
func TestKeptLiveListsTheEarlierMarksOfThatNoteToo(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	writeFile(t, marksPath(r.store), aMarksFile(
		`{"id":"`+markID(1)+`","kind":"thought","path":"`+mkNote+`","body":"earlier thought","at":"2026-09-22T11:00:00Z"}`,
		`{"id":"`+markID(3)+`","kind":"highlight","path":"`+mkSecret+`","quote":"a passage elsewhere","at":"2026-09-22T12:00:00Z"}`,
	))

	live, err := r.kept.Live(mkNote)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := range live {
		got = append(got, live[i].ID+":"+live[i].Body)
	}
	if diff := cmp.Diff([]string{markID(1) + ":earlier thought"}, got); diff != "" {
		t.Errorf("Live() over the earlier file (-want +got):\n%s", diff)
	}
}
