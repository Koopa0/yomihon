package note

import (
	"strings"

	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// folderNotices words the standing notices the snapshot holds, in the reader's
// language, for the surfaces that draw them. Nothing is dropped: a surface that
// received a notice and did not draw it would be the silence this exists to
// end.
func folderNotices(notices []snapshot.Notice, lang wording.Lang) []pages.FolderNotice {
	if len(notices) == 0 {
		return nil
	}
	out := make([]pages.FolderNotice, 0, len(notices))
	for _, notice := range notices {
		title, summary := noticeWords(notice.Reason, lang)
		out = append(out, pages.FolderNotice{Title: title, Summary: summary, Detail: noticeFiles(notice.Paths)})
	}
	return out
}

// noticeWords is what a reason says, as a title and a sentence. The switch has
// no default arm, so a reason added to the snapshot's set without words here is
// a gap the exhaustive linter reports rather than a notice with an empty
// heading; the test over the set's own list is the second lock on the same gap.
func noticeWords(reason snapshot.NoticeReason, lang wording.Lang) (title, summary string) {
	//nolint:gocritic // singleCaseSwitch: the set has one reason today, and a switch is the form the exhaustive linter reads; a second reason adds a case here
	switch reason {
	case snapshot.NoticeNamesCollide:
		return wording.NoticeNamesCollideTitle.In(lang), wording.NoticeNamesCollide.In(lang)
	}
	return "", ""
}

// noticeFiles names the files a notice is about as the filesystem spells them.
// Two names that fold to one path print alike, so each is written with its
// escapes beside it, which is the only place the pair differs.
func noticeFiles(paths []string) string {
	spelled := make([]string, 0, len(paths))
	for _, raw := range paths {
		spelled = append(spelled, vault.Spelled(raw))
	}
	return strings.Join(spelled, "; ")
}
