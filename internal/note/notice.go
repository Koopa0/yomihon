package note

import (
	"strings"

	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// folderNotices words what the pages owe their reader about the folder as a
// whole, in the reader's language, for the surfaces that draw it. Today that is
// one fact: two names fold to one path, so the scan is refused and the pages
// have stopped updating. A surface that was handed it and did not draw it would
// be the silence this exists to end, so a pair is never dropped.
func folderNotices(collision []string, lang wording.Lang) []pages.FolderNotice {
	if len(collision) == 0 {
		return nil
	}
	return []pages.FolderNotice{{
		Title:   wording.NoticeNamesCollideTitle.In(lang),
		Summary: wording.NoticeNamesCollide.In(lang),
		Detail:  noticeFiles(collision),
	}}
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
