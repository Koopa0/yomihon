package note

import (
	"strings"

	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// noticeWordings is what each reason says: a title and a sentence. A reason
// with no row here would draw an empty heading, which is why the set the
// snapshot lists is checked against this table in both languages.
var noticeWordings = map[snapshot.NoticeReason]struct{ title, summary wording.Phrase }{
	snapshot.NoticeNamesCollide: {wording.NoticeNamesCollideTitle, wording.NoticeNamesCollide},
}

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

// noticeWords is what a reason says, as a title and a sentence.
func noticeWords(reason snapshot.NoticeReason, lang wording.Lang) (title, summary string) {
	words := noticeWordings[reason]
	return words.title.In(lang), words.summary.In(lang)
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
