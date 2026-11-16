package note

import (
	"strings"
	"testing"
)

// TestAThoughtsOpeningIsItsFirstSentenceInTheReadersOwnWords holds what is shown
// beside the text a thought answers: the first sentence of what the reader
// wrote, cut where a Chinese or Japanese stop or a Latin one ends it, never at a
// full stop inside a number, and never the line a draft opens with.
func TestAThoughtsOpeningIsItsFirstSentenceInTheReadersOwnWords(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"a Chinese sentence", "我原本以為 cancel() 之後就停了。後來才知道不是。\n", "我原本以為 cancel() 之後就停了。"},
		{"a Latin sentence", "I thought it had stopped. Then I read on.\n", "I thought it had stopped."},
		{"a stop inside a number is not a stop", "Go 1.26 already did this. It did not.\n", "Go 1.26 already did this."},
		{"a question mark ends it", "Who closes the channel? The sender.\n", "Who closes the channel?"},
		{"the draft's opening line is not the reader's", "看答案前，我的想法：\n\n以為 Wait 會先結束。後來才懂。\n", "以為 Wait 會先結束。"},
		{"the English draft line too", "My thoughts before opening the answer:\n\nIt blocks forever. Because nobody receives.\n", "It blocks forever."},
		{"the first line with words wins over blank ones", "\n\n\nFirst. Second.\n", "First."},
		{"markup is not the reader's words", "I **think** this is `wrong`. Maybe.\n", "I think this is wrong."},
		{"nothing written yet", "\n\n", ""},
		{"only the draft's line", "看答案前，我的想法：\n", ""},
		{"no stop at all", "an unfinished thought", "an unfinished thought"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := thoughtOpening(tc.body); got != tc.want {
				t.Errorf("thoughtOpening(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}

	long := strings.Repeat("字", maxOpeningRunes+30) + "。"
	got := thoughtOpening(long)
	if want := strings.Repeat("字", maxOpeningRunes) + "…"; got != want {
		t.Errorf("a sentence over the bound is %d runes ending %q, want it cut at %d and marked", len([]rune(got)), got[len(got)-6:], maxOpeningRunes)
	}
}
