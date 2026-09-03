package pages

import (
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheReadAloudBarsWordsTravelWithThePage covers a bar the browser builds
// and the server therefore never sees. Its sentences used to live in the
// script, which is one file for every reader: an English reader pressed a
// sentence and was answered in Traditional Chinese, on a page that was
// otherwise theirs.
func TestTheReadAloudBarsWordsTravelWithThePage(t *testing.T) {
	t.Parallel()

	spoken := &NoteView{BodyHTML: `<div class="y-reading" lang="ja"><button data-tts="あさ。"></button></div>`}

	t.Run("a page with nothing to read aloud carries none of them", func(t *testing.T) {
		t.Parallel()
		if got := readAloudAttrs(&NoteView{BodyHTML: "<p>plain</p>"}, wording.ZhHant); got != nil {
			t.Errorf("readAloudAttrs() = %v, want nothing on a page that grows no bar", got)
		}
	})

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run("a page that can speak carries them in "+string(lang), func(t *testing.T) {
			t.Parallel()
			attrs := readAloudAttrs(spoken, lang)
			for name, phrase := range map[string]wording.Phrase{
				"data-readaloud-controls":    wording.ReadAloudControls,
				"data-readaloud-speed":       wording.ReadAloudSpeed,
				"data-readaloud-rate":        wording.ReadAloudRateFmt,
				"data-readaloud-stop":        wording.ReadAloudStop,
				"data-readaloud-stopthis":    wording.ReadAloudStopThis,
				"data-readaloud-stopped":     wording.ReadAloudStopped,
				"data-readaloud-playing":     wording.ReadAloudPlaying,
				"data-readaloud-finished":    wording.ReadAloudFinished,
				"data-readaloud-unavailable": wording.ReadAloudUnavailable,
			} {
				if got, want := attrs[name], phrase.In(lang); got != want {
					t.Errorf("%s = %v, want %q", name, got, want)
				}
			}
			if len(attrs) != 9 {
				t.Errorf("readAloudAttrs() carries %d attributes; every one of them has to be named above", len(attrs))
			}
		})
	}
}
