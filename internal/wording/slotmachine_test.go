package wording

import "testing"

// The card does not speak when a word is swapped, and it is not going to: a
// sentence read aloud on every swap would talk over a learner trying three
// words to see which one fits, each attempt cut off by the one before it. The
// instruction is therefore pinned to what the card actually does, because the
// version that promised speech shipped and was believed.
func TestSentencePracticeLedePromisesOnlyWhatSwappingDoes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		lang Lang
		want string
	}{
		{ZhHant, "替換詞語，句子會重新組合；朗讀鍵會唸出來。"},
		{En, "Swap a word and the sentence rebuilds; the speaker button reads it aloud."},
	} {
		t.Run(string(tc.lang), func(t *testing.T) {
			t.Parallel()
			if got := SlotMachineLede.In(tc.lang); got != tc.want {
				t.Errorf("SlotMachineLede.In(%s) = %q, want %q", tc.lang, got, tc.want)
			}
		})
	}
}
